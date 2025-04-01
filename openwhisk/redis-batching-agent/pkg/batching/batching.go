package batching

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Operation types
const (
	TypeGet    = "Get"
	TypeSet    = "Set"
	TypeDel    = "Del"
	TypeExists = "Exists"
)

// BatchRequest represents a generic Redis request that can be batched
type BatchRequest struct {
	Type        string      // Type of request (Get, Set, Del, etc.)
	Key         string      // Redis key
	Value       string      // For Set requests
	ResultChan  chan any    // Channel to deliver result
	ErrorChan   chan error  // Channel to deliver errors
}

// RedisBatcher handles batching Redis requests
type RedisBatcher struct {
	client          *redis.Client
	enabled         bool
	batchWindow     time.Duration
	maxBatchSize    int
	pendingRequests chan *BatchRequest
	mu              sync.Mutex
	wg              sync.WaitGroup
}

// NewRedisBatcher creates a new Redis batcher
func NewRedisBatcher(redisHost string, redisPort string, redisPassword string, poolSize int, enabled bool, batchWindow time.Duration, maxBatchSize int) *RedisBatcher {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: redisPassword,
		PoolSize: poolSize,
	})

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	} else {
		log.Printf("Redis connection successful to %s:%s", redisHost, redisPort)
	}

	batcher := &RedisBatcher{
		client:          client,
		enabled:         enabled,
		batchWindow:     batchWindow,
		maxBatchSize:    maxBatchSize,
		pendingRequests: make(chan *BatchRequest, maxBatchSize*10), // Buffer to handle spikes
	}

	if enabled {
		batcher.wg.Add(1)
		go batcher.processRequestsLoop()
	}

	return batcher
}

// Submit adds a request to the batching queue
func (b *RedisBatcher) Submit(request *BatchRequest) {
	if !b.enabled {
		// If batching is disabled, execute the request immediately
		b.executeRequest(context.Background(), request)
		return
	}

	// Submit to the batching queue
	b.pendingRequests <- request
}

// Shutdown stops the batcher and waits for all requests to finish
func (b *RedisBatcher) Shutdown() {
	if b.enabled {
		close(b.pendingRequests)
		b.wg.Wait()
	}
	if b.client != nil {
		b.client.Close()
	}
}

// executeRequest executes a single request without batching
func (b *RedisBatcher) executeRequest(ctx context.Context, request *BatchRequest) {
	switch request.Type {
	case TypeGet:
		result, err := b.client.Get(ctx, request.Key).Result()
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}
	case TypeSet:
		result, err := b.client.Set(ctx, request.Key, request.Value, 0).Result()
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}
	case TypeDel:
		result, err := b.client.Del(ctx, request.Key).Result()
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}
	case TypeExists:
		result, err := b.client.Exists(ctx, request.Key).Result()
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}
	default:
		request.ErrorChan <- fmt.Errorf("unsupported request type: %s", request.Type)
	}
}

// processRequestsLoop processes batches of requests
func (b *RedisBatcher) processRequestsLoop() {
	defer b.wg.Done()

	for {
		// Create a new batch
		batch := make([]*BatchRequest, 0, b.maxBatchSize)
		
		// Wait for first request or exit if channel is closed
		request, ok := <-b.pendingRequests
		if !ok {
			// Channel closed, exit
			return
		}
		
		batch = append(batch, request)
		
		// Set timer for batch window
		timer := time.NewTimer(b.batchWindow)

		// Collect requests until batch is full or window expires
	collectLoop:
		for len(batch) < b.maxBatchSize {
			select {
			case request, ok := <-b.pendingRequests:
				if !ok {
					// Channel closed
					break collectLoop
				}
				batch = append(batch, request)
			case <-timer.C:
				// Batch window expired
				break collectLoop
			}
		}

		// Stop the timer if it hasn't expired
		if !timer.Stop() {
			// Try to drain the channel
			select {
			case <-timer.C:
			default:
			}
		}

		// Process the batch
		b.processBatch(batch)
	}
}

// processBatch processes a batch of requests
func (b *RedisBatcher) processBatch(batch []*BatchRequest) {
	if len(batch) == 0 {
		return
	}

	// Group requests by type and parameters for better batching
	getRequests := make(map[string][]*BatchRequest)
	setRequests := make(map[string][]*BatchRequest)
	delRequests := make(map[string][]*BatchRequest)
	existsRequests := make(map[string][]*BatchRequest)

	log.Printf("Processing batch of %d requests", len(batch))

	for _, request := range batch {
		switch request.Type {
		case TypeGet:
			key := request.Key
			getRequests[key] = append(getRequests[key], request)
		case TypeSet:
			key := fmt.Sprintf("%s:%s", request.Key, request.Value)
			setRequests[key] = append(setRequests[key], request)
		case TypeDel:
			key := request.Key
			delRequests[key] = append(delRequests[key], request)
		case TypeExists:
			key := request.Key
			existsRequests[key] = append(existsRequests[key], request)
		default:
			request.ErrorChan <- fmt.Errorf("unsupported request type: %s", request.Type)
		}
	}

	ctx := context.Background()
	
	// Process all requests using Redis pipelines
	pipe := b.client.Pipeline()
	
	// Process GET requests
	type GetResult struct {
		Cmd      *redis.StringCmd
		Requests []*BatchRequest
	}
	getResults := make([]GetResult, 0)
	for key, requests := range getRequests {
		cmd := pipe.Get(ctx, key)
		getResults = append(getResults, GetResult{
			Cmd:      cmd,
			Requests: requests,
		})
	}
	
	// Process SET requests
	type SetResult struct {
		Cmd      *redis.StatusCmd
		Requests []*BatchRequest
	}
	setResults := make([]SetResult, 0)
	for keyValue, requests := range setRequests {
		parts := splitKeyValue(keyValue)
		if len(parts) == 2 {
			cmd := pipe.Set(ctx, parts[0], parts[1], 0)
			setResults = append(setResults, SetResult{
				Cmd:      cmd,
				Requests: requests,
			})
		}
	}
	
	// Process DEL requests
	type DelResult struct {
		Cmd      *redis.IntCmd
		Requests []*BatchRequest
	}
	delResults := make([]DelResult, 0)
	for key, requests := range delRequests {
		cmd := pipe.Del(ctx, key)
		delResults = append(delResults, DelResult{
			Cmd:      cmd,
			Requests: requests,
		})
	}
	
	// Process EXISTS requests
	type ExistsResult struct {
		Cmd      *redis.IntCmd
		Requests []*BatchRequest
	}
	existsResults := make([]ExistsResult, 0)
	for key, requests := range existsRequests {
		cmd := pipe.Exists(ctx, key)
		existsResults = append(existsResults, ExistsResult{
			Cmd:      cmd,
			Requests: requests,
		})
	}
	
	// Execute the pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		// If pipeline fails, send error to all requests
		log.Printf("Redis pipeline execution failed: %v", err)
		for _, batch := range []interface{}{getResults, setResults, delResults, existsResults} {
			switch b := batch.(type) {
			case []GetResult:
				for _, result := range b {
					for _, request := range result.Requests {
						request.ErrorChan <- err
					}
				}
			case []SetResult:
				for _, result := range b {
					for _, request := range result.Requests {
						request.ErrorChan <- err
					}
				}
			case []DelResult:
				for _, result := range b {
					for _, request := range result.Requests {
						request.ErrorChan <- err
					}
				}
			case []ExistsResult:
				for _, result := range b {
					for _, request := range result.Requests {
						request.ErrorChan <- err
					}
				}
			}
		}
		return
	}
	
	// Process results and send responses
	for _, result := range getResults {
		val, err := result.Cmd.Result()
		for _, request := range result.Requests {
			if err != nil {
				request.ErrorChan <- err
			} else {
				request.ResultChan <- val
			}
		}
	}
	
	for _, result := range setResults {
		val, err := result.Cmd.Result()
		for _, request := range result.Requests {
			if err != nil {
				request.ErrorChan <- err
			} else {
				request.ResultChan <- val
			}
		}
	}
	
	for _, result := range delResults {
		val, err := result.Cmd.Result()
		for _, request := range result.Requests {
			if err != nil {
				request.ErrorChan <- err
			} else {
				request.ResultChan <- val
			}
		}
	}
	
	for _, result := range existsResults {
		val, err := result.Cmd.Result()
		for _, request := range result.Requests {
			if err != nil {
				request.ErrorChan <- err
			} else {
				request.ResultChan <- val
			}
		}
	}
	
	log.Printf("Batch processing completed for %d requests", len(batch))
}

// Helper function to split key:value format
func splitKeyValue(keyValue string) []string {
	var parts []string
	inKey := true
	var key, value string
	
	for i := 0; i < len(keyValue); i++ {
		if keyValue[i] == ':' && inKey {
			inKey = false
			continue
		}
		
		if inKey {
			key += string(keyValue[i])
		} else {
			value += string(keyValue[i])
		}
	}
	
	return []string{key, value}
} 