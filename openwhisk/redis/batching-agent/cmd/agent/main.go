package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
)

// Configuration for the agent
type Configuration struct {
	Port           int
	BatchWindow    time.Duration
	MaxBatchSize   int
	RedisHost      string
	RedisPort      string
	RedisPassword  string
	RedisPoolSize  int
}

// Operation types
type OpType string
const (
	TypeGet OpType = "get"
	TypeSet OpType = "set"
	TypeDel OpType = "del"
)

// Request represents a Redis operation request
type Request struct {
	Type     OpType
	Key      string
	Value    string
	ResultCh chan Result
}

// Result represents the result of a Redis operation
type Result struct {
	Value string
	Error error
}

// Batcher handles batching Redis operations
type Batcher struct {
	client       *redis.Client
	requests     chan *Request
	batchWindow  time.Duration
	maxBatchSize int
	wg           sync.WaitGroup
	shutdown     chan struct{}
}

// NewBatcher creates a new Redis batcher
func NewBatcher(config Configuration) (*Batcher, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
		Password: config.RedisPassword,
		PoolSize: config.RedisPoolSize,
	})

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis connection failed: %v", err)
	}

	log.Printf("Redis connection successful to %s:%s", config.RedisHost, config.RedisPort)

	batcher := &Batcher{
		client:       client,
		requests:     make(chan *Request, config.MaxBatchSize*10), // Buffer to handle spikes
		batchWindow:  config.BatchWindow,
		maxBatchSize: config.MaxBatchSize,
		shutdown:     make(chan struct{}),
	}

	// Start the processing goroutine
	batcher.wg.Add(1)
	go batcher.processRequests()

	return batcher, nil
}

// Submit adds a request to the batching queue
func (b *Batcher) Submit(req *Request) {
	select {
	case b.requests <- req:
		// Request successfully added to queue
	case <-b.shutdown:
		// Batcher is shutting down
		req.ResultCh <- Result{Error: fmt.Errorf("batcher is shutting down")}
	}
}

// Shutdown stops the batcher gracefully
func (b *Batcher) Shutdown() error {
	close(b.shutdown)
	b.wg.Wait()
	return b.client.Close()
}

// processRequests processes batches of requests
func (b *Batcher) processRequests() {
	defer b.wg.Done()

	for {
		select {
		case <-b.shutdown:
			return
		default:
			b.processBatch()
		}
	}
}

// processBatch collects and executes a batch of requests
func (b *Batcher) processBatch() {
	ctx := context.Background()
	batch := make([]*Request, 0, b.maxBatchSize)
	timer := time.NewTimer(b.batchWindow)

	// Wait for first request or exit if shutdown is signaled
	select {
	case req := <-b.requests:
		batch = append(batch, req)
		timer.Reset(b.batchWindow)
	case <-b.shutdown:
		timer.Stop()
		return
	}

	// Collect requests until batch is full or window expires
collectLoop:
	for len(batch) < b.maxBatchSize {
		select {
		case req := <-b.requests:
			batch = append(batch, req)
		case <-timer.C:
			break collectLoop
		case <-b.shutdown:
			timer.Stop()
			for _, req := range batch {
				req.ResultCh <- Result{Error: fmt.Errorf("batcher is shutting down")}
			}
			return
		}
	}

	timer.Stop()

	// Process the batch with pipelining
	if len(batch) > 0 {
		log.Printf("Processing batch of %d requests", len(batch))
		
		// Create a pipeline
		pipe := b.client.Pipeline()
		
		// Group requests by type for tracking
		getRequests := make(map[int]*Request)
		setRequests := make(map[int]*Request)
		delRequests := make(map[int]*Request)
		
		// Add commands to pipeline
		for i, req := range batch {
			switch req.Type {
			case TypeGet:
				getRequests[i] = req
				pipe.Get(ctx, req.Key)
			case TypeSet:
				setRequests[i] = req
				pipe.Set(ctx, req.Key, req.Value, 0)
			case TypeDel:
				delRequests[i] = req
				pipe.Del(ctx, req.Key)
			}
		}
		
		// Execute pipeline
		results, err := pipe.Exec(ctx)
		
		// If there was a global error, return it to all requesters
		if err != nil && err != redis.Nil {
			log.Printf("Pipeline execution error: %v", err)
			for _, req := range batch {
				req.ResultCh <- Result{Error: err}
			}
			return
		}
		
		// Process results
		for i, result := range results {
			switch {
			case i < len(getRequests):
				req := getRequests[i]
				if result.Err() != nil && result.Err() != redis.Nil {
					req.ResultCh <- Result{Error: result.Err()}
				} else {
					value, _ := result.(*redis.StringCmd).Result()
					req.ResultCh <- Result{Value: value}
				}
			case i < len(getRequests) + len(setRequests):
				req := setRequests[i-len(getRequests)]
				if result.Err() != nil {
					req.ResultCh <- Result{Error: result.Err()}
				} else {
					req.ResultCh <- Result{Value: "OK"}
				}
			case i < len(getRequests) + len(setRequests) + len(delRequests):
				req := delRequests[i-len(getRequests)-len(setRequests)]
				if result.Err() != nil {
					req.ResultCh <- Result{Error: result.Err()}
				} else {
					count, _ := result.(*redis.IntCmd).Result()
					req.ResultCh <- Result{Value: strconv.FormatInt(count, 10)}
				}
			}
		}
	}
}

// Server handles HTTP requests
type Server struct {
	batcher *Batcher
	server  *fasthttp.Server
}

// NewServer creates a new HTTP server
func NewServer(batcher *Batcher, port int) *Server {
	server := &Server{
		batcher: batcher,
	}

	// Create fasthttp server
	server.server = &fasthttp.Server{
		Handler: server.handleRequest,
		Name:    "Redis Batching Agent",
	}

	return server
}

// Start starts the HTTP server
func (s *Server) Start(port int) error {
	log.Printf("Starting server on port %d", port)
	return s.server.ListenAndServe(fmt.Sprintf(":%d", port))
}

// Shutdown stops the HTTP server
func (s *Server) Shutdown() error {
	return s.server.Shutdown()
}

// handleRequest routes incoming HTTP requests
func (s *Server) handleRequest(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	method := string(ctx.Method())

	switch {
	case path == "/health" && method == "GET":
		s.handleHealth(ctx)
	case path == "/redis/get" && method == "GET":
		s.handleGet(ctx)
	case path == "/redis/set" && method == "POST":
		s.handleSet(ctx)
	case path == "/redis/del" && method == "DELETE":
		s.handleDel(ctx)
	default:
		ctx.Error("Not Found", fasthttp.StatusNotFound)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBodyString("OK")
}

// handleGet handles GET requests
func (s *Server) handleGet(ctx *fasthttp.RequestCtx) {
	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		ctx.Error("Missing required parameter: key", fasthttp.StatusBadRequest)
		return
	}

	resultCh := make(chan Result, 1)
	req := &Request{
		Type:     TypeGet,
		Key:      key,
		ResultCh: resultCh,
	}

	s.batcher.Submit(req)
	result := <-resultCh

	if result.Error != nil {
		ctx.Error(fmt.Sprintf("Failed to get value: %v", result.Error), fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]string{"value": result.Value}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to marshal response: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetBody(jsonResponse)
}

// handleSet handles SET requests
func (s *Server) handleSet(ctx *fasthttp.RequestCtx) {
	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		ctx.Error("Missing required parameter: key", fasthttp.StatusBadRequest)
		return
	}

	value := string(ctx.QueryArgs().Peek("value"))
	if value == "" {
		ctx.Error("Missing required parameter: value", fasthttp.StatusBadRequest)
		return
	}

	resultCh := make(chan Result, 1)
	req := &Request{
		Type:     TypeSet,
		Key:      key,
		Value:    value,
		ResultCh: resultCh,
	}

	s.batcher.Submit(req)
	result := <-resultCh

	if result.Error != nil {
		ctx.Error(fmt.Sprintf("Failed to set value: %v", result.Error), fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]string{"result": result.Value}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to marshal response: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetBody(jsonResponse)
}

// handleDel handles DEL requests
func (s *Server) handleDel(ctx *fasthttp.RequestCtx) {
	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		ctx.Error("Missing required parameter: key", fasthttp.StatusBadRequest)
		return
	}

	resultCh := make(chan Result, 1)
	req := &Request{
		Type:     TypeDel,
		Key:      key,
		ResultCh: resultCh,
	}

	s.batcher.Submit(req)
	result := <-resultCh

	if result.Error != nil {
		ctx.Error(fmt.Sprintf("Failed to delete key: %v", result.Error), fasthttp.StatusInternalServerError)
		return
	}

	response := map[string]string{"deleted": result.Value}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		ctx.Error(fmt.Sprintf("Failed to marshal response: %v", err), fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetContentType("application/json")
	ctx.SetBody(jsonResponse)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Parse command-line flags
	portFlag := flag.Int("port", 8080, "HTTP server port")
	redisHostFlag := flag.String("redis-host", "", "Redis host")
	redisPortFlag := flag.String("redis-port", "6379", "Redis port")
	redisPasswordFlag := flag.String("redis-password", "", "Redis password")
	redisPoolSizeFlag := flag.Int("redis-pool-size", 10, "Redis connection pool size")
	batchWindowFlag := flag.Duration("batch-window", 100*time.Millisecond, "Batch collection window")
	maxBatchSizeFlag := flag.Int("max-batch-size", 10, "Maximum batch size")
	flag.Parse()

	// Override with environment variables if set
	port, _ := strconv.Atoi(getEnvOrDefault("PORT", strconv.Itoa(*portFlag)))
	redisHost := getEnvOrDefault("REDIS_HOST", *redisHostFlag)
	redisPort := getEnvOrDefault("REDIS_PORT", *redisPortFlag)
	redisPassword := getEnvOrDefault("REDIS_PASSWORD", *redisPasswordFlag)
	redisPoolSize, _ := strconv.Atoi(getEnvOrDefault("REDIS_POOL_SIZE", strconv.Itoa(*redisPoolSizeFlag)))
	
	batchWindowStr := getEnvOrDefault("BATCH_WINDOW", "")
	batchWindow := *batchWindowFlag
	if batchWindowStr != "" {
		if parsedWindow, err := time.ParseDuration(batchWindowStr); err == nil {
			batchWindow = parsedWindow
		}
	}
	
	maxBatchSize, _ := strconv.Atoi(getEnvOrDefault("MAX_BATCH_SIZE", strconv.Itoa(*maxBatchSizeFlag)))

	// Validate Redis host
	if redisHost == "" {
		log.Fatal("Redis host is required")
	}

	// Create configuration
	config := Configuration{
		Port:           port,
		BatchWindow:    batchWindow,
		MaxBatchSize:   maxBatchSize,
		RedisHost:      redisHost,
		RedisPort:      redisPort,
		RedisPassword:  redisPassword,
		RedisPoolSize:  redisPoolSize,
	}

	// Print configuration
	log.Printf("Starting Redis batching agent with configuration:")
	log.Printf("  Port: %d", config.Port)
	log.Printf("  Redis Host: %s", config.RedisHost)
	log.Printf("  Redis Port: %s", config.RedisPort)
	log.Printf("  Batch Window: %v", config.BatchWindow)
	log.Printf("  Max Batch Size: %d", config.MaxBatchSize)
	log.Printf("  Redis Pool Size: %d", config.RedisPoolSize)

	// Create batcher
	batcher, err := NewBatcher(config)
	if err != nil {
		log.Fatalf("Failed to create batcher: %v", err)
	}
	defer batcher.Shutdown()

	// Create server
	server := NewServer(batcher, config.Port)

	// Set up signal handling for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(config.Port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-signals
	log.Println("Shutting down...")

	// Shut down server
	if err := server.Shutdown(); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}

	// Shut down batcher
	if err := batcher.Shutdown(); err != nil {
		log.Printf("Error shutting down batcher: %v", err)
	}
} 