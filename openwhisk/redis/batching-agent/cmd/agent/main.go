package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/serverless-benchmarks/redis-batching-agent/pkg/batching"
)

// Set up logging to stderr to ensure we see output even if stdout is buffered
func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Redis Batching Agent initialized")
}

// Configuration holds the agent's configuration
type Configuration struct {
	Port            int
	BatchingEnabled bool
	BatchWindow     time.Duration
	MaxBatchSize    int
	DebugMode       bool
	RedisHost       string
	RedisPort       string
	RedisPassword   string
	RedisPoolSize   int
}

// BatchingAgent handles Redis requests and batches them
type BatchingAgent struct {
	config   Configuration
	batcher  *batching.RedisBatcher
	server   *http.Server
	router   *mux.Router
	mu       sync.Mutex
}

// NewBatchingAgent creates a new batching agent
func NewBatchingAgent(config Configuration) (*BatchingAgent, error) {
	// Debug: Print configuration
	log.Printf("Creating Redis batching agent with configuration:")
	log.Printf("  Redis Host: %s", config.RedisHost)
	log.Printf("  Redis Port: %s", config.RedisPort)
	log.Printf("  Batching Enabled: %v", config.BatchingEnabled)
	log.Printf("  Batch Window: %v", config.BatchWindow)
	log.Printf("  Max Batch Size: %d", config.MaxBatchSize)

	// Create router and server
	router := mux.NewRouter()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: router,
	}

	// Create agent
	agent := &BatchingAgent{
		config:  config,
		router:  router,
		server:  server,
	}

	// Initialize batcher
	agent.batcher = batching.NewRedisBatcher(
		config.RedisHost,
		config.RedisPort,
		config.RedisPassword,
		config.RedisPoolSize,
		config.BatchingEnabled,
		config.BatchWindow,
		config.MaxBatchSize,
	)

	// Set up routes
	agent.setupRoutes()

	return agent, nil
}

// setupRoutes configures the HTTP routes
func (a *BatchingAgent) setupRoutes() {
	// Health check
	a.router.HandleFunc("/health", a.handleHealth).Methods("GET")

	// Redis API endpoints
	a.router.HandleFunc("/redis/get", a.handleGet).Methods("GET")
	a.router.HandleFunc("/redis/set", a.handleSet).Methods("POST")
	a.router.HandleFunc("/redis/del", a.handleDel).Methods("DELETE")
	a.router.HandleFunc("/redis/exists", a.handleExists).Methods("GET")

	// Debug endpoints
	if a.config.DebugMode {
		a.router.HandleFunc("/debug/config", a.handleDebugConfig).Methods("GET")
	}
}

// Start starts the HTTP server
func (a *BatchingAgent) Start() {
	go func() {
		log.Printf("Starting Redis batching agent on port %d", a.config.Port)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
}

// Shutdown gracefully shuts down the agent
func (a *BatchingAgent) Shutdown(ctx context.Context) {
	a.batcher.Shutdown()
	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}
}

// Handler functions

func (a *BatchingAgent) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (a *BatchingAgent) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing required parameter: key", http.StatusBadRequest)
		return
	}

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeGet,
		Key:        key,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(string)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Marshal the response
		jsonData, err := json.Marshal(map[string]string{"value": resp})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to get value: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleSet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing required parameter: key", http.StatusBadRequest)
		return
	}

	value := r.URL.Query().Get("value")
	if value == "" {
		http.Error(w, "Missing required parameter: value", http.StatusBadRequest)
		return
	}

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeSet,
		Key:        key,
		Value:      value,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(string)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Marshal the response
		jsonData, err := json.Marshal(map[string]string{"result": resp})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to set value: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleDel(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing required parameter: key", http.StatusBadRequest)
		return
	}

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeDel,
		Key:        key,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(int64)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Marshal the response
		jsonData, err := json.Marshal(map[string]int64{"deleted": resp})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to delete key: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleExists(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing required parameter: key", http.StatusBadRequest)
		return
	}

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeExists,
		Key:        key,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(int64)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		exists := resp > 0
		// Marshal the response
		jsonData, err := json.Marshal(map[string]bool{"exists": exists})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to check if key exists: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleDebugConfig(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Marshal the configuration
	jsonData, err := json.Marshal(a.config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal configuration: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func parseEnvDuration(name string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(name); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func parseEnvBool(name string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(name); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func parseEnvInt(name string, defaultVal int) int {
	if val, ok := os.LookupEnv(name); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8080, "Port to listen on")
	batchingEnabled := flag.Bool("batching", true, "Enable request batching")
	batchWindow := flag.Duration("batch-window", 100*time.Millisecond, "Batch window duration")
	maxBatchSize := flag.Int("max-batch-size", 10, "Maximum batch size")
	debugMode := flag.Bool("debug", false, "Enable debug mode")
	redisHost := flag.String("redis-host", "localhost", "Redis host")
	redisPort := flag.String("redis-port", "6379", "Redis port")
	redisPassword := flag.String("redis-password", "", "Redis password")
	redisPoolSize := flag.Int("redis-pool-size", 10, "Redis connection pool size")
	flag.Parse()

	// Override with environment variables if set
	if os.Getenv("PORT") != "" {
		*port = parseEnvInt("PORT", *port)
	}
	if os.Getenv("BATCHING_ENABLED") != "" {
		*batchingEnabled = parseEnvBool("BATCHING_ENABLED", *batchingEnabled)
	}
	if os.Getenv("BATCH_WINDOW") != "" {
		*batchWindow = parseEnvDuration("BATCH_WINDOW", *batchWindow)
	}
	if os.Getenv("MAX_BATCH_SIZE") != "" {
		*maxBatchSize = parseEnvInt("MAX_BATCH_SIZE", *maxBatchSize)
	}
	if os.Getenv("DEBUG_MODE") != "" {
		*debugMode = parseEnvBool("DEBUG_MODE", *debugMode)
	}
	if os.Getenv("REDIS_HOST") != "" {
		*redisHost = os.Getenv("REDIS_HOST")
	}
	if os.Getenv("REDIS_PORT") != "" {
		*redisPort = os.Getenv("REDIS_PORT")
	}
	if os.Getenv("REDIS_PASSWORD") != "" {
		*redisPassword = os.Getenv("REDIS_PASSWORD")
	}
	if os.Getenv("REDIS_POOL_SIZE") != "" {
		*redisPoolSize = parseEnvInt("REDIS_POOL_SIZE", *redisPoolSize)
	}

	log.Printf("Command line flags and environment variables parsed")
	log.Printf("  Port: %d", *port)
	log.Printf("  Redis Host: %s", *redisHost)
	log.Printf("  Redis Port: %s", *redisPort)
	log.Printf("  Batching Enabled: %v", *batchingEnabled)
	log.Printf("  Batch Window: %v", *batchWindow)
	log.Printf("  Max Batch Size: %d", *maxBatchSize)

	// Create configuration
	config := Configuration{
		Port:            *port,
		BatchingEnabled: *batchingEnabled,
		BatchWindow:     *batchWindow,
		MaxBatchSize:    *maxBatchSize,
		DebugMode:       *debugMode,
		RedisHost:       *redisHost,
		RedisPort:       *redisPort,
		RedisPassword:   *redisPassword,
		RedisPoolSize:   *redisPoolSize,
	}

	// Create and start agent
	agent, err := NewBatchingAgent(config)
	if err != nil {
		log.Fatalf("Failed to create Redis batching agent: %v", err)
	}

	agent.Start()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigChan
	log.Println("Shutting down...")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the agent
	agent.Shutdown(ctx)
	log.Println("Shutdown complete")
} 