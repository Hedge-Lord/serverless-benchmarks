package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	"crypto/tls"

	"github.com/redis/go-redis/v9"
)

// Global variables for node IP caching
var (
	cachedNodeIP string
	nodeIPOnce   sync.Once
)

// Configuration holds the benchmark configuration
type Configuration struct {
	NumOps         int    `json:"num_ops"`
	OperationType  string `json:"operation_type"`
	UseBatching    bool   `json:"use_batching"`
	BatchingHost   string `json:"batching_host"`
	BatchingPort   string `json:"batching_port"`
	RedisHost      string `json:"redis_host"`
	RedisPort      string `json:"redis_port"`
	RedisPassword  string `json:"redis_password"`
	KeyPrefix      string `json:"key_prefix"`
	ParallelCalls  int    `json:"parallel_calls"`
}

// Response represents the benchmark results
type Response struct {
	StatusCode     int               `json:"statusCode"`
	ExecutionTimeMs float64          `json:"execution_time_ms"`
	NumOps         int               `json:"num_ops"`
	OperationType  string            `json:"operation_type"`
	ParallelCalls  int               `json:"parallel_calls"`
	UseBatching    bool              `json:"use_batching"`
	BatchingURL    string            `json:"batching_url,omitempty"`
	RedisHost      string            `json:"redis_host,omitempty"`
	SuccessCount   int               `json:"success_count"`
	Results        []OperationResult `json:"results"`
	Error          string            `json:"error,omitempty"`
}

// OperationResult represents the result of a single Redis operation
type OperationResult struct {
	Key           string  `json:"key"`
	Status        string  `json:"status"`
	Value         string  `json:"value,omitempty"`
	Error         string  `json:"error,omitempty"`
	DurationMs    float64 `json:"duration_ms"`
}

// getNodeIP retrieves and caches the node IP for the current pod
func getNodeIP() (string, error) {
	var err error
	nodeIPOnce.Do(func() {
		// First check if batching_agent_host was provided as a parameter
		batchingHost := os.Getenv("BATCHING_AGENT_HOST")
		if batchingHost != "" {
			log.Printf("Using BATCHING_AGENT_HOST environment variable: %s", batchingHost)
			cachedNodeIP = batchingHost
			return
		}
		
		// Then try to get the node IP using the Kubernetes API
		var ip string
		ip, err = fetchNodeIPFromKubernetesAPI()
		if err == nil && ip != "" {
			log.Printf("Successfully retrieved node IP from Kubernetes API: %s", ip)
			cachedNodeIP = ip
			return
		}
		log.Printf("Failed to get node IP from Kubernetes API: %v, trying fallbacks", err)
		
		// Fallback: check for environment variables
		ip = os.Getenv("NODE_IP")
		if ip != "" {
			log.Printf("Using NODE_IP environment variable: %s", ip)
			cachedNodeIP = ip
			return
		}
		
		// Try other common environment variables
		for _, envVar := range []string{"KUBERNETES_NODE_IP", "HOST_IP", "HOSTNAME"} {
			ip = os.Getenv(envVar)
			if ip != "" {
				log.Printf("Using %s environment variable: %s", envVar, ip)
				cachedNodeIP = ip
				return
			}
		}
		
		// Final fallback: use a default hostname for the node
		cachedNodeIP = "localhost"
		log.Printf("No node IP could be determined. Using default: %s", cachedNodeIP)
	})
	
	if cachedNodeIP == "" {
		return "", fmt.Errorf("failed to determine node IP")
	}
	
	return cachedNodeIP, err
}

// fetchNodeIPFromKubernetesAPI retrieves the node IP using the Kubernetes API
func fetchNodeIPFromKubernetesAPI() (string, error) {
	// Get pod name from hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %v", err)
	}
	
	// Check if service account token exists
	tokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return "", fmt.Errorf("service account token not found")
	}
	
	// Read the service account token
	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read service account token: %v", err)
	}
	
	// Get Kubernetes API server address
	kubeHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubePort := os.Getenv("KUBERNETES_SERVICE_PORT")
	if kubeHost == "" || kubePort == "" {
		return "", fmt.Errorf("Kubernetes service host or port not found")
	}
	
	// Read namespace
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	namespace, err := ioutil.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace: %v", err)
	}
	
	// Create request to Kubernetes API
	url := fmt.Sprintf("https://%s:%s/api/v1/namespaces/%s/pods/%s", 
		kubeHost, kubePort, string(namespace), hostname)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	
	// Configure TLS to skip verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	
	log.Printf("Attempting to query Kubernetes API at: %s", url)
	
	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get pod info: status %s, body: %s", resp.Status, string(bodyBytes))
	}
	
	// Parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	
	// Parse JSON
	var podInfo map[string]interface{}
	if err := json.Unmarshal(body, &podInfo); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	
	// Extract hostIP from status
	status, ok := podInfo["status"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("status not found in pod info")
	}
	
	hostIP, ok := status["hostIP"].(string)
	if !ok {
		return "", fmt.Errorf("hostIP not found in pod status")
	}
	
	return hostIP, nil
}

// directRedisOperation performs a Redis operation directly
func directRedisOperation(ctx context.Context, redisClient *redis.Client, opType, key, value string) (string, error) {
	switch opType {
	case "get":
		return redisClient.Get(ctx, key).Result()
	case "set":
		return redisClient.Set(ctx, key, value, 0).Result()
	case "del":
		result, err := redisClient.Del(ctx, key).Result()
		return strconv.FormatInt(result, 10), err
	case "exists":
		result, err := redisClient.Exists(ctx, key).Result()
		return strconv.FormatInt(result, 10), err
	default:
		return "", fmt.Errorf("unsupported operation type: %s", opType)
	}
}

// batchedRedisOperation performs a Redis operation through the batching agent
func batchedRedisOperation(batchingURL, opType, key, value string) (string, error) {
	var url string
	var method string
	
	switch opType {
	case "get":
		url = fmt.Sprintf("%s/redis/get?key=%s", batchingURL, key)
		method = "GET"
	case "set":
		url = fmt.Sprintf("%s/redis/set?key=%s&value=%s", batchingURL, key, value)
		method = "POST"
	case "del":
		url = fmt.Sprintf("%s/redis/del?key=%s", batchingURL, key)
		method = "DELETE"
	case "exists":
		url = fmt.Sprintf("%s/redis/exists?key=%s", batchingURL, key)
		method = "GET"
	default:
		return "", fmt.Errorf("unsupported operation type: %s", opType)
	}
	
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status: %s", resp.Status)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	
	// Parse JSON response
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}
	
	// Return different fields based on operation type
	switch opType {
	case "get":
		return result["value"], nil
	case "set":
		return result["result"], nil
	case "del":
		return result["deleted"], nil
	case "exists":
		return result["exists"], nil
	default:
		return "", fmt.Errorf("unsupported operation type: %s", opType)
	}
}

// runBenchmark runs the Redis benchmark
func runBenchmark(config Configuration) Response {
	startTime := time.Now()
	ctx := context.Background()
	
	response := Response{
		StatusCode:     200,
		NumOps:         config.NumOps,
		OperationType:  config.OperationType,
		ParallelCalls:  config.ParallelCalls,
		UseBatching:    config.UseBatching,
		Results:        make([]OperationResult, 0, config.NumOps),
	}
	
	// Use default values if not provided
	if config.OperationType == "" {
		config.OperationType = "get"
	}
	if config.NumOps <= 0 {
		config.NumOps = 1
	}
	if config.ParallelCalls <= 0 {
		config.ParallelCalls = 1
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "test_key"
	}
	
	// Set up for direct Redis access
	var redisClient *redis.Client
	var batchingURL string
	
	if config.UseBatching {
		// Use batching agent
		batchingHost := config.BatchingHost
		batchingPort := config.BatchingPort
		
		// If host not provided, detect node IP
		if batchingHost == "" {
			log.Printf("No batching agent host provided, attempting to auto-detect")
			var err error
			batchingHost, err = getNodeIP()
			if err != nil {
				response.StatusCode = 500
				response.Error = fmt.Sprintf("Failed to get node IP: %v", err)
				return response
			}
			log.Printf("Auto-detected batching agent host: %s", batchingHost)
		} else {
			log.Printf("Using provided batching agent host: %s", batchingHost)
		}
		
		// Use default port if not provided
		if batchingPort == "" {
			batchingPort = "8080"
		}
		
		batchingURL = fmt.Sprintf("http://%s:%s", batchingHost, batchingPort)
		response.BatchingURL = batchingURL
		log.Printf("Using Redis batching agent at %s", batchingURL)
		
		// Test the connection to the batching agent
		testURL := fmt.Sprintf("%s/health", batchingURL)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(testURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to batching agent health endpoint: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("Warning: Batching agent health check returned non-OK status: %d", resp.StatusCode)
			} else {
				log.Printf("Successfully connected to batching agent at %s", batchingURL)
			}
		}
	} else {
		// Direct Redis access
		redisHost := config.RedisHost
		redisPort := config.RedisPort
		
		if redisHost == "" {
			redisHost = "localhost"
		}
		if redisPort == "" {
			redisPort = "6379"
		}
		
		redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: config.RedisPassword,
			DB:       0,
		})
		
		// Test the connection
		_, err := redisClient.Ping(ctx).Result()
		if err != nil {
			response.StatusCode = 500
			response.Error = fmt.Sprintf("Failed to connect to Redis: %v", err)
			return response
		}
		
		response.RedisHost = redisHost
		log.Printf("Connected to Redis at %s", redisAddr)
	}
	
	// Run the benchmark operations
	successCount := 0
	resultsChan := make(chan OperationResult, config.NumOps)
	
	// Create a wait group to synchronize goroutines
	var wg sync.WaitGroup
	
	// Create worker functions
	workerFunc := func(start, end int) {
		defer wg.Done()
		
		for i := start; i < end; i++ {
			key := fmt.Sprintf("%s_%d", config.KeyPrefix, i)
			value := fmt.Sprintf("value_%d", i)
			
			var result OperationResult
			result.Key = key
			
			opStart := time.Now()
			
			if config.UseBatching {
				// Use batching agent
				val, err := batchedRedisOperation(batchingURL, config.OperationType, key, value)
				if err != nil {
					result.Status = "error"
					result.Error = err.Error()
				} else {
					result.Status = "success"
					result.Value = val
					successCount++
				}
			} else {
				// Direct Redis access
				val, err := directRedisOperation(ctx, redisClient, config.OperationType, key, value)
				if err != nil && err != redis.Nil {
					result.Status = "error"
					result.Error = err.Error()
				} else {
					result.Status = "success"
					if err == redis.Nil {
						result.Value = ""
					} else {
						result.Value = val
					}
					successCount++
				}
			}
			
			result.DurationMs = float64(time.Since(opStart)) / float64(time.Millisecond)
			resultsChan <- result
		}
	}
	
	// Distribute work among workers
	opsPerWorker := config.NumOps / config.ParallelCalls
	if opsPerWorker == 0 {
		opsPerWorker = 1
		config.ParallelCalls = config.NumOps
	}
	
	wg.Add(config.ParallelCalls)
	for i := 0; i < config.ParallelCalls; i++ {
		start := i * opsPerWorker
		end := (i + 1) * opsPerWorker
		if i == config.ParallelCalls-1 {
			end = config.NumOps // Ensure all ops are processed
		}
		go workerFunc(start, end)
	}
	
	// Close results channel once all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		response.Results = append(response.Results, result)
	}
	
	response.SuccessCount = successCount
	response.ExecutionTimeMs = float64(time.Since(startTime)) / float64(time.Millisecond)
	
	// Close Redis client
	if redisClient != nil {
		redisClient.Close()
	}
	
	return response
}

// main is the entry point for the OpenWhisk action
func main() {
	// Read and parse input
	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
	
	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}
	
	// Convert params to configuration
	config := Configuration{}
	
	// Convert num_ops
	if val, ok := params["num_ops"]; ok {
		if numOps, ok := val.(float64); ok {
			config.NumOps = int(numOps)
		} else if numOpsStr, ok := val.(string); ok {
			if numOps, err := strconv.Atoi(numOpsStr); err == nil {
				config.NumOps = numOps
			}
		}
	}
	
	// Convert operation_type
	if val, ok := params["operation_type"]; ok {
		if opType, ok := val.(string); ok {
			config.OperationType = opType
		}
	}
	
	// Convert use_batching
	if val, ok := params["use_batching"]; ok {
		if useBatching, ok := val.(bool); ok {
			config.UseBatching = useBatching
		} else if useBatchingStr, ok := val.(string); ok {
			if useBatching, err := strconv.ParseBool(useBatchingStr); err == nil {
				config.UseBatching = useBatching
			}
		}
	}
	
	// Get batching_agent_host from params
	if val, ok := params["batching_agent_host"]; ok {
		if host, ok := val.(string); ok {
			config.BatchingHost = host
			// Set env var for getNodeIP to use
			os.Setenv("BATCHING_AGENT_HOST", host)
		}
	} else if val, ok := params["BATCHING_AGENT_HOST"]; ok {
		// Backward compatibility with environment variable style
		if host, ok := val.(string); ok {
			config.BatchingHost = host
			// Set env var for getNodeIP to use
			os.Setenv("BATCHING_AGENT_HOST", host)
		}
	}
	
	// Get batching_agent_port from params
	if val, ok := params["batching_agent_port"]; ok {
		if port, ok := val.(string); ok {
			config.BatchingPort = port
		} else if portNum, ok := val.(float64); ok {
			config.BatchingPort = strconv.Itoa(int(portNum))
		}
	} else if val, ok := params["BATCHING_AGENT_PORT"]; ok {
		// Backward compatibility with environment variable style
		if port, ok := val.(string); ok {
			config.BatchingPort = port
		} else if portNum, ok := val.(float64); ok {
			config.BatchingPort = strconv.Itoa(int(portNum))
		}
	}
	
	// Get REDIS_HOST from params
	if val, ok := params["REDIS_HOST"]; ok {
		if host, ok := val.(string); ok {
			config.RedisHost = host
		}
	}
	
	// Get REDIS_PORT from params
	if val, ok := params["REDIS_PORT"]; ok {
		if port, ok := val.(string); ok {
			config.RedisPort = port
		} else if portNum, ok := val.(float64); ok {
			config.RedisPort = strconv.Itoa(int(portNum))
		}
	}
	
	// Get REDIS_PASSWORD from params
	if val, ok := params["REDIS_PASSWORD"]; ok {
		if password, ok := val.(string); ok {
			config.RedisPassword = password
		}
	}
	
	// Get key_prefix
	if val, ok := params["key_prefix"]; ok {
		if keyPrefix, ok := val.(string); ok {
			config.KeyPrefix = keyPrefix
		}
	}
	
	// Get parallel_calls
	if val, ok := params["parallel_calls"]; ok {
		if parallelCalls, ok := val.(float64); ok {
			config.ParallelCalls = int(parallelCalls)
		} else if parallelCallsStr, ok := val.(string); ok {
			if parallelCalls, err := strconv.Atoi(parallelCallsStr); err == nil {
				config.ParallelCalls = parallelCalls
			}
		}
	}
	
	// Run the benchmark
	response := runBenchmark(config)
	
	// Output the response
	output, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding response: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(string(output))
} 