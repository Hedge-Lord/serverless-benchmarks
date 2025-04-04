package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/gorilla/mux"
	"github.com/serverless-benchmarks/openwhisk/batching-agent/pkg/batching"
)

// Set up logging to stderr to ensure we see output even if stdout is buffered
func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Package initialized")
}

// Configuration holds the agent's configuration
type Configuration struct {
	Port              int
	BatchingEnabled   bool
	BatchWindow       time.Duration
	MaxBatchSize      int
	DebugMode         bool
	AwsRegion         string
	DefaultBucketName string
}

// BatchingAgent handles S3 requests and optionally batches them
type BatchingAgent struct {
	config  Configuration
	s3Client *s3.Client
	batcher  *batching.S3Batcher
	server   *http.Server
	router   *mux.Router
	mu       sync.Mutex
}

// NewBatchingAgent creates a new batching agent
func NewBatchingAgent(config Configuration) (*BatchingAgent, error) {
	// Debug: Print environment variables
	log.Printf("Checking AWS environment variables...")
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey != "" {
		log.Printf("AWS_ACCESS_KEY_ID is set (length: %d, prefix: %s)", len(accessKey), accessKey[:4])
	} else {
		log.Printf("AWS_ACCESS_KEY_ID is not set")
	}
	if secretKey != "" {
		log.Printf("AWS_SECRET_ACCESS_KEY is set (length: %d, prefix: %s)", len(secretKey), secretKey[:4])
	} else {
		log.Printf("AWS_SECRET_ACCESS_KEY is not set")
	}

	// Configure AWS SDK with explicit credentials
	creds := credentials.NewStaticCredentialsProvider(
		accessKey,
		secretKey,
		"",
	)

	// Test credentials before creating config
	credsValue, err := creds.Retrieve(context.Background())
	if err != nil {
		log.Printf("Failed to retrieve credentials: %v", err)
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}
	log.Printf("Successfully retrieved credentials - Access Key: %s, Secret Key: %s", 
		credsValue.AccessKeyID[:4], 
		credsValue.SecretAccessKey[:4])

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), 
		awsconfig.WithRegion(config.AwsRegion),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		log.Printf("Failed to load AWS configuration: %v", err)
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Debug: Print AWS configuration details
	log.Printf("AWS Configuration loaded - Region: %s, Credentials: %v", config.AwsRegion, cfg.Credentials != nil)
	if cfg.Credentials != nil {
		log.Printf("Credentials provider type: %T", cfg.Credentials)
	}

	s3Client := s3.NewFromConfig(cfg)

	// Create router and server
	router := mux.NewRouter()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: router,
	}

	// Create agent
	agent := &BatchingAgent{
		config:  config,
		s3Client: s3Client,
		router:  router,
		server:  server,
	}

	// Initialize batcher
	agent.batcher = batching.NewS3Batcher(
		s3Client,
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

	// S3 API endpoints
	a.router.HandleFunc("/s3/listBuckets", a.handleListBuckets).Methods("GET")
	a.router.HandleFunc("/s3/listObjects", a.handleListObjects).Methods("GET")
	a.router.HandleFunc("/s3/getObject", a.handleGetObject).Methods("GET")

	// Debug endpoints
	if a.config.DebugMode {
		a.router.HandleFunc("/debug/config", a.handleDebugConfig).Methods("GET")
	}
}

// Start starts the HTTP server
func (a *BatchingAgent) Start() {
	go func() {
		log.Printf("Starting batching agent on port %d", a.config.Port)
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

func (a *BatchingAgent) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received ListBuckets request")
	
	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeListBuckets,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)
	log.Printf("Submitted batch request")

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(*s3.ListBucketsOutput)
		if !ok {
			log.Printf("Invalid response type received: %T", result)
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Marshal the response
		jsonData, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		log.Printf("Error from batcher: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list buckets: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		bucket = a.config.DefaultBucketName
	}

	prefix := r.URL.Query().Get("prefix")
	maxKeys := int32(1000) // Default to 1000

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeListObjects,
		BucketName: bucket,
		Prefix:     prefix,
		MaxKeys:    maxKeys,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(*s3.ListObjectsV2Output)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Marshal the response
		jsonData, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
	}
}

func (a *BatchingAgent) handleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		bucket = a.config.DefaultBucketName
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing required parameter: key", http.StatusBadRequest)
		return
	}

	resultChan := make(chan any, 1)
	errorChan := make(chan error, 1)

	// Create a batch request
	request := &batching.BatchRequest{
		Type:       batching.TypeGetObject,
		BucketName: bucket,
		Key:        key,
		ResultChan: resultChan,
		ErrorChan:  errorChan,
	}

	// Submit the request
	a.batcher.Submit(request)

	// Wait for the result
	select {
	case result := <-resultChan:
		resp, ok := result.(*s3.GetObjectOutput)
		if !ok {
			http.Error(w, "Invalid response type", http.StatusInternalServerError)
			return
		}

		// Set headers
		w.Header().Set("Content-Type", *resp.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))

		// Copy the body to the response
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("Error copying response body: %v", err)
		}
		resp.Body.Close()

	case err := <-errorChan:
		http.Error(w, fmt.Sprintf("Failed to get object: %v", err), http.StatusInternalServerError)
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

func main() {
	// Add early logging
	log.Printf("Starting batching agent initialization...")
	
	// Parse command-line flags
	port := flag.Int("port", 8080, "Port to listen on")
	batchingEnabled := flag.Bool("batching", true, "Enable request batching")
	batchWindow := flag.Duration("batch-window", 100*time.Millisecond, "Batch window duration")
	maxBatchSize := flag.Int("max-batch-size", 10, "Maximum batch size")
	debugMode := flag.Bool("debug", false, "Enable debug mode")
	awsRegion := flag.String("aws-region", "us-east-1", "AWS region")
	defaultBucketName := flag.String("default-bucket", "", "Default S3 bucket name")
	flag.Parse()

	log.Printf("Command line flags parsed - Port: %d, Region: %s, Bucket: %s", *port, *awsRegion, *defaultBucketName)

	// Create configuration
	config := Configuration{
		Port:              *port,
		BatchingEnabled:   *batchingEnabled,
		BatchWindow:       *batchWindow,
		MaxBatchSize:      *maxBatchSize,
		DebugMode:         *debugMode,
		AwsRegion:         *awsRegion,
		DefaultBucketName: *defaultBucketName,
	}

	log.Printf("Configuration created, attempting to create agent...")

	// Create and start agent
	agent, err := NewBatchingAgent(config)
	if err != nil {
		log.Fatalf("Failed to create batching agent: %v", err)
	}

	log.Printf("Agent created successfully, starting server...")
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