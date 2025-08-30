package concurrency

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"brain2-backend/internal/errors"
)

// BatchProcessor handles batch processing with environment-aware optimizations
type BatchProcessor struct {
	pool         *AdaptiveWorkerPool
	environment  RuntimeEnvironment
	batchSize    int
	timeout      time.Duration
	errorHandler ErrorHandler
}

// BatchItem represents an item to be processed in a batch
type BatchItem interface {
	GetID() string
}

// ProcessFunc is the function that processes a single item
type ProcessFunc func(ctx context.Context, item BatchItem) error

// BatchResult contains the results of batch processing
type BatchResult struct {
	SuccessCount int
	FailedCount  int
	TotalCount   int
	Errors       map[string]error
	Duration     time.Duration
}

// ErrorHandler handles errors during batch processing
type ErrorHandler func(itemID string, err error)

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(ctx context.Context, config *PoolConfig) *BatchProcessor {
	if config == nil {
		config = &PoolConfig{}
	}
	
	pool := NewAdaptiveWorkerPool(ctx, config)
	
	// Set timeout based on environment
	var timeout time.Duration
	switch config.Environment {
	case EnvironmentLambda:
		// Lambda: Leave buffer before function timeout
		// Assume 15 minute max, leave 30 seconds buffer
		timeout = 14*time.Minute + 30*time.Second
		
		// Check if custom timeout is set via env var
		if funcTimeout := getLambdaTimeout(); funcTimeout > 0 {
			// Leave 10% buffer
			timeout = time.Duration(float64(funcTimeout) * 0.9)
		}
		
	case EnvironmentECS:
		// ECS: Longer timeout acceptable
		timeout = 30 * time.Minute
		
	default:
		// Local: Reasonable timeout
		timeout = 10 * time.Minute
	}
	
	return &BatchProcessor{
		pool:        pool,
		environment: config.Environment,
		batchSize:   config.BatchSize,
		timeout:     timeout,
	}
}

// getLambdaTimeout gets the configured Lambda timeout from environment
func getLambdaTimeout() time.Duration {
	// In actual Lambda, this would come from context.Deadline()
	// For now, return 0 to use default
	return 0
}

// ProcessBatch processes a batch of items with optimal concurrency
func (p *BatchProcessor) ProcessBatch(
	ctx context.Context,
	items []BatchItem,
	processFunc ProcessFunc,
) (*BatchResult, error) {
	return p.ProcessBatchWithDeadline(ctx, items, processFunc)
}

// ProcessBatchWithDeadline processes a batch with Lambda deadline awareness
func (p *BatchProcessor) ProcessBatchWithDeadline(
	ctx context.Context,
	items []BatchItem,
	processFunc ProcessFunc,
) (*BatchResult, error) {
	
	if len(items) == 0 {
		return &BatchResult{
			TotalCount: 0,
		}, nil
	}
	
	startTime := time.Now()
	
	// Handle context deadline appropriately
	var cancel context.CancelFunc
	deadline, hasDeadline := ctx.Deadline()
	
	if hasDeadline && p.environment == EnvironmentLambda {
		// Get timeout buffer from config or use default
		timeoutBuffer := 10 * time.Second
		if p.pool != nil && p.pool.config.TimeoutBuffer > 0 {
			timeoutBuffer = time.Duration(p.pool.config.TimeoutBuffer) * time.Second
		}
		
		// Check if we need to adjust the deadline
		safeDeadline := deadline.Add(-timeoutBuffer)
		if time.Now().After(safeDeadline) {
			return nil, errors.Timeout("BATCH_TIMEOUT", "Insufficient time remaining for batch processing").
				WithOperation("ProcessBatch").
				WithResource("batch_processor").
				Build()
		}
		
		// Only create new context if we need to adjust the deadline
		if safeDeadline.Before(deadline) {
			ctx, cancel = context.WithDeadline(ctx, safeDeadline)
			defer cancel()
		}
		
		// Only log deadline info in debug mode
		if os.Getenv("DEBUG") != "" {
			log.Printf("Lambda deadline detected: %v remaining (buffer: %v)", time.Until(safeDeadline), timeoutBuffer)
		}
	} else if !hasDeadline {
		// Only add timeout if there's no existing deadline
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}
	
	// Start the worker pool only if we need to manage it
	// Don't stop pools we didn't create
	shouldManagePool := false
	if !p.pool.running {
		if err := p.pool.Start(); err != nil {
			return nil, errors.Internal("WORKER_POOL_START_FAILED", "Failed to start worker pool").
				WithOperation("ProcessBatch").
				WithResource("worker_pool").
				WithCause(err).
				Build()
		}
		shouldManagePool = true
		defer func() {
			if shouldManagePool {
				p.pool.Stop()
			}
		}()
	}
	
	// Results tracking - pre-allocate with expected capacity
	var mu sync.Mutex
	errors := make(map[string]error, len(items)/10) // Assume ~10% error rate
	successCount := 0
	failedCount := 0
	
	// Create wait group for tracking completion
	var wg sync.WaitGroup
	
	// Process items in chunks with deadline awareness
	chunks := p.createChunksWithDeadline(ctx, items)
	
	for _, chunk := range chunks {
		for _, item := range chunk {
			wg.Add(1)
			
			// Capture item in closure
			currentItem := item
			
			task := Task{
				ID: currentItem.GetID(),
				Execute: func(taskCtx context.Context) error {
					// Apply Lambda-specific optimizations
					if p.environment == EnvironmentLambda {
						// Add small timeout per item to prevent hanging
						itemCtx, itemCancel := context.WithTimeout(taskCtx, 30*time.Second)
						err := processFunc(itemCtx, currentItem)
						itemCancel() // Cancel immediately, not deferred
						return err
					}
					
					return processFunc(taskCtx, currentItem)
				},
				Callback: func(id string, err error) {
					defer wg.Done()
					
					mu.Lock()
					defer mu.Unlock()
					
					if err != nil {
						errors[id] = err
						failedCount++
						
						// Call error handler if set
						if p.errorHandler != nil {
							p.errorHandler(id, err)
						}
					} else {
						successCount++
					}
				},
			}
			
			// Submit task to pool
			if err := p.pool.Submit(task); err != nil {
				wg.Done()
				mu.Lock()
				errors[currentItem.GetID()] = fmt.Errorf("failed to submit task: %w", err)
				failedCount++
				mu.Unlock()
			}
		}
		
		// In Lambda, check remaining time between chunks
		if p.environment == EnvironmentLambda {
			if deadline, ok := ctx.Deadline(); ok {
				timeRemaining := time.Until(deadline)
				if timeRemaining < 5*time.Second {
					if os.Getenv("DEBUG") != "" {
						log.Printf("Stopping batch processing - approaching deadline (remaining: %v)", timeRemaining)
					}
					break
				}
			}
			
			select {
			case <-ctx.Done():
				// Context cancelled or deadline exceeded
				break
			default:
				// Continue to next chunk
			}
		}
	}
	
	// Wait for all tasks to complete or timeout
	done := make(chan struct{})
	go func() {
		// Monitor for context cancellation to prevent goroutine leak
		waitDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(waitDone)
		}()
		
		select {
		case <-waitDone:
			// All tasks completed normally
			close(done)
		case <-ctx.Done():
			// Context cancelled, exit to prevent leak
			// Note: tasks may still be running but we exit the wait goroutine
			close(done)
		}
	}()
	
	select {
	case <-done:
		// Check if it was due to context cancellation
		if ctx.Err() != nil {
			return &BatchResult{
				SuccessCount: successCount,
				FailedCount:  failedCount + (len(items) - successCount - failedCount),
				TotalCount:   len(items),
				Errors:       errors,
				Duration:     time.Since(startTime),
			}, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
		}
		// All tasks completed successfully
		
	case <-ctx.Done():
		// Timeout or cancellation
		return &BatchResult{
			SuccessCount: successCount,
			FailedCount:  failedCount + (len(items) - successCount - failedCount),
			TotalCount:   len(items),
			Errors:       errors,
			Duration:     time.Since(startTime),
		}, fmt.Errorf("batch processing timeout or cancelled")
	}
	
	return &BatchResult{
		SuccessCount: successCount,
		FailedCount:  failedCount,
		TotalCount:   len(items),
		Errors:       errors,
		Duration:     time.Since(startTime),
	}, nil
}

// createChunks divides items into environment-appropriate chunks
func (p *BatchProcessor) createChunks(items []BatchItem) [][]BatchItem {
	return p.createChunksWithDeadline(context.Background(), items)
}

// createChunksWithDeadline creates chunks based on remaining time
func (p *BatchProcessor) createChunksWithDeadline(ctx context.Context, items []BatchItem) [][]BatchItem {
	if len(items) == 0 {
		return nil
	}
	
	chunkSize := p.batchSize
	if chunkSize <= 0 {
		chunkSize = GetOptimalBatchSize(p.environment)
	}
	
	// Adjust batch size based on deadline if in Lambda
	if p.environment == EnvironmentLambda {
		if deadline, ok := ctx.Deadline(); ok {
			timeRemaining := time.Until(deadline)
			chunkSize = p.adaptBatchSizeToDeadline(timeRemaining, chunkSize)
			// Only log in debug mode
			if os.Getenv("DEBUG") != "" {
				log.Printf("Adaptive batch size: %d items (time remaining: %v)", chunkSize, timeRemaining)
			}
		}
		
		// DynamoDB BatchWriteItem limit is 25
		if chunkSize > 25 {
			chunkSize = 25
		}
	}
	
	return p.createChunksWithSize(items, chunkSize)
}

// adaptBatchSizeToDeadline adjusts batch size based on time remaining
func (p *BatchProcessor) adaptBatchSizeToDeadline(timeRemaining time.Duration, defaultSize int) int {
	switch {
	case timeRemaining < 30*time.Second:
		// Very limited time - process minimal batches
		return min(5, defaultSize)
	case timeRemaining < 1*time.Minute:
		// Limited time - smaller batches
		return min(10, defaultSize)
	case timeRemaining < 2*time.Minute:
		// Moderate time - medium batches
		return min(15, defaultSize)
	default:
		// Plenty of time - use default
		return defaultSize
	}
}

// createChunksWithSize creates chunks of specified size
func (p *BatchProcessor) createChunksWithSize(items []BatchItem, chunkSize int) [][]BatchItem {
	var chunks [][]BatchItem
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProcessBatchWithRetry processes a batch with retry logic
func (p *BatchProcessor) ProcessBatchWithRetry(
	ctx context.Context,
	items []BatchItem,
	processFunc ProcessFunc,
	maxRetries int,
) (*BatchResult, error) {
	
	// Lambda doesn't benefit from retries (stateless)
	if p.environment == EnvironmentLambda {
		maxRetries = 1
	}
	
	var lastResult *BatchResult
	var lastErr error
	
	remainingItems := items
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		result, err := p.ProcessBatch(ctx, remainingItems, processFunc)
		
		if err == nil && result.FailedCount == 0 {
			// Complete success
			return result, nil
		}
		
		lastResult = result
		lastErr = err
		
		// For Lambda, don't retry - return partial success
		if p.environment == EnvironmentLambda {
			break
		}
		
		// Prepare items for retry (only failed ones)
		if result != nil && len(result.Errors) > 0 {
			var retryItems []BatchItem
			for _, item := range remainingItems {
				if _, failed := result.Errors[item.GetID()]; failed {
					retryItems = append(retryItems, item)
				}
			}
			remainingItems = retryItems
			
			// Exponential backoff for retries
			if attempt < maxRetries-1 && len(retryItems) > 0 {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				if backoff > 10*time.Second {
					backoff = 10 * time.Second
				}
				time.Sleep(backoff)
			}
		} else {
			break
		}
	}
	
	return lastResult, lastErr
}

// SetErrorHandler sets the error handler for the batch processor
func (p *BatchProcessor) SetErrorHandler(handler ErrorHandler) {
	p.errorHandler = handler
}

// GetStats returns statistics about the batch processor
func (p *BatchProcessor) GetStats() map[string]interface{} {
	stats := p.pool.GetStats()
	stats["batch_size"] = p.batchSize
	stats["timeout"] = p.timeout.String()
	return stats
}