// Package dynamodb provides DynamoDB persistence with batch optimization.
package dynamodb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// BatchWriter implements efficient batch writing to DynamoDB.
// It accumulates write requests and flushes them in batches to optimize performance.
type BatchWriter struct {
	client    *dynamodb.Client
	tableName string
	logger    *zap.Logger
	
	// Batch configuration
	batchSize     int
	flushInterval time.Duration
	maxRetries    int
	
	// Internal state
	mu       sync.Mutex
	buffer   []types.WriteRequest
	flushCh  chan struct{}
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewBatchWriter creates a new batch writer for DynamoDB.
func NewBatchWriter(
	client *dynamodb.Client,
	tableName string,
	logger *zap.Logger,
	batchSize int,
	flushInterval time.Duration,
) *BatchWriter {
	bw := &BatchWriter{
		client:        client,
		tableName:     tableName,
		logger:        logger,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		maxRetries:    3,
		buffer:        make([]types.WriteRequest, 0, batchSize),
		flushCh:       make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}
	
	// Start background flusher
	bw.wg.Add(1)
	go bw.backgroundFlusher()
	
	return bw
}

// Write adds an item to the batch buffer.
func (bw *BatchWriter) Write(ctx context.Context, item map[string]types.AttributeValue) error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	
	// Add to buffer
	bw.buffer = append(bw.buffer, types.WriteRequest{
		PutRequest: &types.PutRequest{
			Item: item,
		},
	})
	
	// Check if we should flush
	if len(bw.buffer) >= bw.batchSize {
		select {
		case bw.flushCh <- struct{}{}:
		default:
			// Flush already signaled
		}
	}
	
	return nil
}

// Delete adds a delete request to the batch buffer.
func (bw *BatchWriter) Delete(ctx context.Context, key map[string]types.AttributeValue) error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	
	// Add to buffer
	bw.buffer = append(bw.buffer, types.WriteRequest{
		DeleteRequest: &types.DeleteRequest{
			Key: key,
		},
	})
	
	// Check if we should flush
	if len(bw.buffer) >= bw.batchSize {
		select {
		case bw.flushCh <- struct{}{}:
		default:
			// Flush already signaled
		}
	}
	
	return nil
}

// Flush manually triggers a flush of the buffer.
func (bw *BatchWriter) Flush(ctx context.Context) error {
	bw.mu.Lock()
	buffer := bw.buffer
	bw.buffer = make([]types.WriteRequest, 0, bw.batchSize)
	bw.mu.Unlock()
	
	if len(buffer) == 0 {
		return nil
	}
	
	return bw.flushBatch(ctx, buffer)
}

// flushBatch writes a batch of items to DynamoDB.
func (bw *BatchWriter) flushBatch(ctx context.Context, batch []types.WriteRequest) error {
	if len(batch) == 0 {
		return nil
	}
	
	// DynamoDB limits batch writes to 25 items
	const maxBatchSize = 25
	
	for i := 0; i < len(batch); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(batch) {
			end = len(batch)
		}
		
		chunk := batch[i:end]
		
		// Retry logic with exponential backoff
		var lastErr error
		for attempt := 0; attempt < bw.maxRetries; attempt++ {
			if err := bw.writeBatchWithRetry(ctx, chunk, attempt); err != nil {
				lastErr = err
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			lastErr = nil
			break
		}
		
		if lastErr != nil {
			bw.logger.Error("Failed to write batch after retries",
				zap.Error(lastErr),
				zap.Int("items", len(chunk)),
			)
			return lastErr
		}
	}
	
	bw.logger.Debug("Successfully flushed batch",
		zap.Int("items", len(batch)),
	)
	
	return nil
}

// writeBatchWithRetry performs a single batch write with retry handling.
func (bw *BatchWriter) writeBatchWithRetry(ctx context.Context, batch []types.WriteRequest, attempt int) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			bw.tableName: batch,
		},
	}
	
	output, err := bw.client.BatchWriteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("batch write failed: %w", err)
	}
	
	// Handle unprocessed items
	if len(output.UnprocessedItems) > 0 {
		unprocessed := output.UnprocessedItems[bw.tableName]
		if len(unprocessed) > 0 {
			if attempt < bw.maxRetries-1 {
				// Retry unprocessed items
				return bw.writeBatchWithRetry(ctx, unprocessed, attempt+1)
			}
			return fmt.Errorf("%d items were not processed", len(unprocessed))
		}
	}
	
	return nil
}

// backgroundFlusher runs in the background to flush batches periodically.
func (bw *BatchWriter) backgroundFlusher() {
	defer bw.wg.Done()
	
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-bw.stopCh:
			// Final flush before stopping
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			bw.Flush(ctx)
			cancel()
			return
			
		case <-ticker.C:
			// Periodic flush
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			bw.Flush(ctx)
			cancel()
			
		case <-bw.flushCh:
			// Triggered flush
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			bw.Flush(ctx)
			cancel()
		}
	}
}

// Close stops the batch writer and flushes remaining items.
func (bw *BatchWriter) Close() error {
	close(bw.stopCh)
	bw.wg.Wait()
	return nil
}

// BatchReader implements efficient batch reading from DynamoDB.
type BatchReader struct {
	client    *dynamodb.Client
	tableName string
	logger    *zap.Logger
}

// NewBatchReader creates a new batch reader for DynamoDB.
func NewBatchReader(
	client *dynamodb.Client,
	tableName string,
	logger *zap.Logger,
) *BatchReader {
	return &BatchReader{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// BatchGet retrieves multiple items by their keys.
func (br *BatchReader) BatchGet(ctx context.Context, keys []map[string]types.AttributeValue) ([]map[string]types.AttributeValue, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	
	// DynamoDB limits batch gets to 100 items
	const maxBatchSize = 100
	var allItems []map[string]types.AttributeValue
	
	for i := 0; i < len(keys); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		
		chunk := keys[i:end]
		
		input := &dynamodb.BatchGetItemInput{
			RequestItems: map[string]types.KeysAndAttributes{
				br.tableName: {
					Keys: chunk,
				},
			},
		}
		
		output, err := br.client.BatchGetItem(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("batch get failed: %w", err)
		}
		
		// Collect items
		if items, ok := output.Responses[br.tableName]; ok {
			allItems = append(allItems, items...)
		}
		
		// Handle unprocessed keys with retry
		if len(output.UnprocessedKeys) > 0 {
			if unprocessed, ok := output.UnprocessedKeys[br.tableName]; ok && len(unprocessed.Keys) > 0 {
				// Retry unprocessed keys
				retryItems, err := br.BatchGet(ctx, unprocessed.Keys)
				if err != nil {
					br.logger.Warn("Failed to retry unprocessed keys",
						zap.Error(err),
						zap.Int("count", len(unprocessed.Keys)),
					)
				} else {
					allItems = append(allItems, retryItems...)
				}
			}
		}
	}
	
	br.logger.Debug("Batch read completed",
		zap.Int("requested", len(keys)),
		zap.Int("retrieved", len(allItems)),
	)
	
	return allItems, nil
}

// ParallelQuery performs parallel queries for better performance.
func (br *BatchReader) ParallelQuery(
	ctx context.Context,
	partitionKeys []string,
	queryBuilder func(pk string) *dynamodb.QueryInput,
) ([]map[string]types.AttributeValue, error) {
	
	// Use goroutines for parallel queries
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	
	var mu sync.Mutex
	var allItems []map[string]types.AttributeValue
	var wg sync.WaitGroup
	
	for _, pk := range partitionKeys {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore
		
		go func(partitionKey string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore
			
			input := queryBuilder(partitionKey)
			
			// Execute query with pagination
			var items []map[string]types.AttributeValue
			paginator := dynamodb.NewQueryPaginator(br.client, input)
			
			for paginator.HasMorePages() {
				output, err := paginator.NextPage(ctx)
				if err != nil {
					br.logger.Error("Query failed",
						zap.Error(err),
						zap.String("partition_key", partitionKey),
					)
					return
				}
				items = append(items, output.Items...)
			}
			
			// Add to results
			mu.Lock()
			allItems = append(allItems, items...)
			mu.Unlock()
		}(pk)
	}
	
	wg.Wait()
	
	br.logger.Debug("Parallel query completed",
		zap.Int("partitions", len(partitionKeys)),
		zap.Int("items", len(allItems)),
	)
	
	return allItems, nil
}