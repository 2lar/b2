package loaders

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BatchFunction is the function that performs the actual batch loading
type BatchFunction[K comparable, V any] func(context.Context, []K) (map[K]V, error)

// Result holds the result of a batch load operation
type Result[V any] struct {
	Value V
	Error error
}

// pendingRequest represents a pending load request
type pendingRequest[V any] struct {
	ctx    context.Context
	result chan Result[V]
}

// Batcher provides generic batching functionality without caching
type Batcher[K comparable, V any] struct {
	// Configuration
	batchFn      BatchFunction[K, V]
	batchWindow  time.Duration
	maxBatchSize int

	// State management
	pending map[K][]*pendingRequest[V]
	mu      sync.Mutex
	timer   *time.Timer

	// Metrics
	totalBatches   int64
	totalRequests  int64
	batchSizeSum   int64
	mu_metrics     sync.RWMutex

	logger *zap.Logger
}

// NewBatcher creates a new batcher
func NewBatcher[K comparable, V any](
	batchFn BatchFunction[K, V],
	batchWindow time.Duration,
	maxBatchSize int,
	logger *zap.Logger,
) *Batcher[K, V] {
	if batchWindow <= 0 {
		batchWindow = 10 * time.Millisecond
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 25
	}

	return &Batcher[K, V]{
		batchFn:      batchFn,
		batchWindow:  batchWindow,
		maxBatchSize: maxBatchSize,
		pending:      make(map[K][]*pendingRequest[V]),
		logger:       logger,
	}
}

// Load loads a single value, batching with other concurrent requests
func (b *Batcher[K, V]) Load(ctx context.Context, key K) (V, error) {
	b.mu.Lock()

	// Create result channel for this request
	resultChan := make(chan Result[V], 1)
	req := &pendingRequest[V]{
		ctx:    ctx,
		result: resultChan,
	}

	// Add to pending requests for this key
	b.pending[key] = append(b.pending[key], req)

	// Update metrics
	b.mu_metrics.Lock()
	b.totalRequests++
	b.mu_metrics.Unlock()

	// Check if we should dispatch immediately (batch size limit)
	shouldDispatch := len(b.pending) >= b.maxBatchSize

	// Start or reset timer if needed
	if b.timer == nil && !shouldDispatch {
		b.timer = time.AfterFunc(b.batchWindow, func() {
			b.dispatch()
		})
	} else if shouldDispatch {
		// Cancel existing timer and dispatch immediately
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		go b.dispatch()
	}

	b.mu.Unlock()

	// Wait for result
	select {
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	case result := <-resultChan:
		return result.Value, result.Error
	}
}

// LoadMany loads multiple values, batching them together
func (b *Batcher[K, V]) LoadMany(ctx context.Context, keys []K) (map[K]V, error) {
	results := make(map[K]V)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, key := range keys {
		wg.Add(1)
		go func(k K) {
			defer wg.Done()

			value, err := b.Load(ctx, k)
			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstErr == nil {
				firstErr = err
			} else if err == nil {
				results[k] = value
			}
		}(key)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return results, nil
}

// dispatch executes the batch function for all pending requests
func (b *Batcher[K, V]) dispatch() {
	b.mu.Lock()

	// Nothing to dispatch
	if len(b.pending) == 0 {
		b.timer = nil
		b.mu.Unlock()
		return
	}

	// Collect all unique keys
	keys := make([]K, 0, len(b.pending))
	requests := b.pending
	b.pending = make(map[K][]*pendingRequest[V])
	b.timer = nil

	for key := range requests {
		keys = append(keys, key)
	}

	// Update metrics
	b.mu_metrics.Lock()
	b.totalBatches++
	b.batchSizeSum += int64(len(keys))
	avgBatchSize := float64(b.batchSizeSum) / float64(b.totalBatches)
	b.mu_metrics.Unlock()

	b.mu.Unlock()

	// Log batch execution
	b.logger.Debug("Executing batch",
		zap.Int("batchSize", len(keys)),
		zap.Float64("avgBatchSize", avgBatchSize),
	)

	// Create a context that respects all pending request contexts
	ctx := context.Background()
	for _, reqs := range requests {
		for _, req := range reqs {
			if req.ctx.Err() == nil {
				ctx = req.ctx
				break
			}
		}
	}

	// Execute batch function
	startTime := time.Now()
	results, err := b.batchFn(ctx, keys)
	duration := time.Since(startTime)

	b.logger.Debug("Batch executed",
		zap.Int("requested", len(keys)),
		zap.Int("returned", len(results)),
		zap.Duration("duration", duration),
		zap.Error(err),
	)

	// Deliver results to waiting requests
	for key, reqs := range requests {
		var result Result[V]

		if err != nil {
			// Batch function returned an error
			result.Error = fmt.Errorf("batch load failed: %w", err)
		} else if value, ok := results[key]; ok {
			// Found value for this key
			result.Value = value
		} else {
			// Key not found in results
			var zero V
			result.Value = zero
			result.Error = fmt.Errorf("key not found in batch results")
		}

		// Send result to all requests for this key
		for _, req := range reqs {
			select {
			case req.result <- result:
			case <-req.ctx.Done():
				// Request was cancelled, skip
			}
		}
	}
}

// GetMetrics returns batching metrics
func (b *Batcher[K, V]) GetMetrics() BatcherMetrics {
	b.mu_metrics.RLock()
	defer b.mu_metrics.RUnlock()

	avgBatchSize := float64(0)
	if b.totalBatches > 0 {
		avgBatchSize = float64(b.batchSizeSum) / float64(b.totalBatches)
	}

	return BatcherMetrics{
		TotalBatches:  b.totalBatches,
		TotalRequests: b.totalRequests,
		AvgBatchSize:  avgBatchSize,
	}
}

// BatcherMetrics holds metrics for the batcher
type BatcherMetrics struct {
	TotalBatches  int64
	TotalRequests int64
	AvgBatchSize  float64
}