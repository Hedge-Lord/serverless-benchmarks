package batching

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Request types
const (
	TypeGetObject = "GetObject"
	TypeListObjects = "ListObjects"
	TypeListBuckets = "ListBuckets"
)

// BatchRequest represents a generic S3 request that can be batched
type BatchRequest struct {
	Type        string      // Type of request (GetObject, ListObjects, etc.)
	BucketName  string      // S3 bucket name
	Key         string      // For GetObject requests
	Prefix      string      // For ListObjects requests
	MaxKeys     int32       // For ListObjects requests
	ResultChan  chan any    // Channel to deliver result
	ErrorChan   chan error  // Channel to deliver errors
}

// S3Batcher handles batching S3 requests
type S3Batcher struct {
	client          *s3.Client
	enabled         bool
	batchWindow     time.Duration
	maxBatchSize    int
	batchWindowChan chan struct{}
	pendingRequests chan *BatchRequest
	mu              sync.Mutex
	wg              sync.WaitGroup
}

// NewS3Batcher creates a new S3 batcher
func NewS3Batcher(client *s3.Client, enabled bool, batchWindow time.Duration, maxBatchSize int) *S3Batcher {
	batcher := &S3Batcher{
		client:          client,
		enabled:         enabled,
		batchWindow:     batchWindow,
		maxBatchSize:    maxBatchSize,
		batchWindowChan: make(chan struct{}),
		pendingRequests: make(chan *BatchRequest, maxBatchSize*10), // Buffer to handle spikes
	}

	if enabled {
		batcher.wg.Add(1)
		go batcher.processRequestsLoop()
	}

	return batcher
}

// Submit adds a request to the batching queue
func (b *S3Batcher) Submit(request *BatchRequest) {
	if !b.enabled {
		// If batching is disabled, execute the request immediately
		b.executeRequest(context.Background(), request)
		return
	}

	// Submit to the batching queue
	b.pendingRequests <- request
}

// Shutdown stops the batcher and waits for all requests to finish
func (b *S3Batcher) Shutdown() {
	if b.enabled {
		close(b.pendingRequests)
		b.wg.Wait()
	}
}

// processRequestsLoop processes batches of requests
func (b *S3Batcher) processRequestsLoop() {
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
func (b *S3Batcher) processBatch(batch []*BatchRequest) {
	if len(batch) == 0 {
		return
	}

	ctx := context.Background()

	// Group requests by type and parameters
	getObjectRequests := make(map[string][]*BatchRequest)
	listObjectsRequests := make(map[string][]*BatchRequest)
	listBucketsRequests := make([]*BatchRequest, 0)

	for _, request := range batch {
		switch request.Type {
		case TypeGetObject:
			key := fmt.Sprintf("%s:%s", request.BucketName, request.Key)
			getObjectRequests[key] = append(getObjectRequests[key], request)
		case TypeListObjects:
			key := fmt.Sprintf("%s:%s:%d", request.BucketName, request.Prefix, request.MaxKeys)
			listObjectsRequests[key] = append(listObjectsRequests[key], request)
		case TypeListBuckets:
			listBucketsRequests = append(listBucketsRequests, request)
		default:
			// Unknown request type, execute immediately
			b.executeRequest(ctx, request)
		}
	}

	// Process grouped GetObject requests
	for key, requests := range getObjectRequests {
		// Execute the first request
		b.executeRequest(ctx, requests[0])
		
		// Copy the result to all other requests in the group
		for i := 1; i < len(requests); i++ {
			select {
			case result := <-requests[0].ResultChan:
				requests[i].ResultChan <- result
			case err := <-requests[0].ErrorChan:
				requests[i].ErrorChan <- err
			}
		}
	}

	// Process grouped ListObjects requests
	for key, requests := range listObjectsRequests {
		// Execute the first request
		b.executeRequest(ctx, requests[0])
		
		// Copy the result to all other requests in the group
		for i := 1; i < len(requests); i++ {
			select {
			case result := <-requests[0].ResultChan:
				requests[i].ResultChan <- result
			case err := <-requests[0].ErrorChan:
				requests[i].ErrorChan <- err
			}
		}
	}
	
	// Process ListBuckets requests (if any)
	if len(listBucketsRequests) > 0 {
		// Execute only the first request
		b.executeRequest(ctx, listBucketsRequests[0])
		
		// Copy the result to all other requests
		for i := 1; i < len(listBucketsRequests); i++ {
			select {
			case result := <-listBucketsRequests[0].ResultChan:
				listBucketsRequests[i].ResultChan <- result
			case err := <-listBucketsRequests[0].ErrorChan:
				listBucketsRequests[i].ErrorChan <- err
			}
		}
	}
}

// executeRequest executes a single request
func (b *S3Batcher) executeRequest(ctx context.Context, request *BatchRequest) {
	switch request.Type {
	case TypeGetObject:
		input := &s3.GetObjectInput{
			Bucket: &request.BucketName,
			Key:    &request.Key,
		}
		
		result, err := b.client.GetObject(ctx, input)
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}

	case TypeListObjects:
		input := &s3.ListObjectsV2Input{
			Bucket:  &request.BucketName,
			Prefix:  &request.Prefix,
			MaxKeys: request.MaxKeys,
		}
		
		result, err := b.client.ListObjectsV2(ctx, input)
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}
		
	case TypeListBuckets:
		input := &s3.ListBucketsInput{}
		
		result, err := b.client.ListBuckets(ctx, input)
		if err != nil {
			request.ErrorChan <- err
		} else {
			request.ResultChan <- result
		}

	default:
		err := fmt.Errorf("unsupported request type: %s", request.Type)
		log.Printf("Error: %v", err)
		request.ErrorChan <- err
	}
} 