// Package persistence - Retry decorator for resilient repository operations.
package persistence

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	
	"go.uber.org/zap"
)

// ============================================================================
// RETRY DECORATOR - Adds automatic retry logic with exponential backoff
// ============================================================================

// RetryConfig configures retry behavior for repository operations.
//
// Key Concepts:
//   - Exponential Backoff: Delays increase exponentially between retries
//   - Jitter: Random variation to prevent thundering herd
//   - Selective Retry: Only retry on transient errors
//   - Context Awareness: Respects context cancellation
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialDelay   time.Duration // Initial delay before first retry
	MaxDelay       time.Duration // Maximum delay between retries
	BackoffFactor  float64       // Multiplier for exponential backoff (e.g., 2.0)
	JitterFactor   float64       // Random jitter factor (0.0 to 1.0)
	
	// Retry conditions
	RetryableErrors []error      // Specific errors to retry
	RetryOnTimeout  bool         // Retry on context timeout
	RetryOn5xx      bool         // Retry on 5xx-like errors
	
	// Advanced options
	OnRetry         func(attempt int, err error) // Callback on each retry
	CircuitBreaker  *CircuitBreakerConfig       // Optional circuit breaker integration
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:      3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffFactor:   2.0,
		JitterFactor:    0.1,
		RetryOnTimeout:  true,
		RetryOn5xx:      true,
	}
}

// RetryNodeRepository adds retry logic to NodeRepository operations.
//
// This decorator demonstrates:
//   1. Resilience Pattern: Automatic retry on transient failures
//   2. Exponential Backoff: Progressive delay increases
//   3. Jitter: Randomization to prevent synchronized retries
//   4. Smart Retry: Only retries operations that are idempotent
//   5. Context Integration: Respects cancellation and deadlines
type RetryNodeRepository struct {
	inner  repository.NodeRepository
	config RetryConfig
	logger *zap.Logger
	rand   *rand.Rand
}

// NewRetryNodeRepository creates a new retry decorator for NodeRepository.
func NewRetryNodeRepository(
	inner repository.NodeRepository,
	config RetryConfig,
) repository.NodeRepository {
	return &RetryNodeRepository{
		inner:  inner,
		config: config,
		logger: zap.L().Named("retry_node_repository"),
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateNodeAndKeywords retries node creation on transient failures.
// Note: This operation is NOT idempotent, so we're careful about retries.
func (r *RetryNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	// Create operations are generally not safe to retry
	// Only retry if we're certain the previous attempt didn't succeed
	return r.executeWithRetry(ctx, "CreateNodeAndKeywords", func() error {
		return r.inner.CreateNodeAndKeywords(ctx, node)
	}, false) // false = not idempotent
}

// FindNodeByID retries node lookup on transient failures.
// This is a safe read operation that can be retried.
func (r *RetryNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	var result *node.Node
	err := r.executeWithRetry(ctx, "FindNodeByID", func() error {
		var err error
		result, err = r.inner.FindNodeByID(ctx, userID, nodeID)
		return err
	}, true) // true = idempotent
	return result, err
}

// FindNodes retries node search on transient failures.
func (r *RetryNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	var result []*node.Node
	err := r.executeWithRetry(ctx, "FindNodes", func() error {
		var err error
		result, err = r.inner.FindNodes(ctx, query)
		return err
	}, true)
	return result, err
}

// DeleteNode retries deletion with caution.
// Delete is idempotent (deleting twice has same effect as deleting once).
func (r *RetryNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.executeWithRetry(ctx, "DeleteNode", func() error {
		return r.inner.DeleteNode(ctx, userID, nodeID)
	}, true)
}

// GetNodesPage retries paginated queries on transient failures.
func (r *RetryNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	var result *repository.NodePage
	err := r.executeWithRetry(ctx, "GetNodesPage", func() error {
		var err error
		result, err = r.inner.GetNodesPage(ctx, query, pagination)
		return err
	}, true)
	return result, err
}

// GetNodeNeighborhood retries graph queries on transient failures.
func (r *RetryNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	var result *shared.Graph
	err := r.executeWithRetry(ctx, "GetNodeNeighborhood", func() error {
		var err error
		result, err = r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
		return err
	}, true)
	return result, err
}

// CountNodes retries count operations on transient failures.
func (r *RetryNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	var result int
	err := r.executeWithRetry(ctx, "CountNodes", func() error {
		var err error
		result, err = r.inner.CountNodes(ctx, userID)
		return err
	}, true)
	return result, err
}

// FindNodesWithOptions retries enhanced queries with options.
func (r *RetryNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	var result []*node.Node
	err := r.executeWithRetry(ctx, "FindNodesWithOptions", func() error {
		var err error
		result, err = r.inner.FindNodesWithOptions(ctx, query, opts...)
		return err
	}, true)
	return result, err
}

// FindNodesPageWithOptions retries enhanced paginated queries.
func (r *RetryNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	var result *repository.NodePage
	err := r.executeWithRetry(ctx, "FindNodesPageWithOptions", func() error {
		var err error
		result, err = r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
		return err
	}, true)
	return result, err
}

// executeWithRetry implements the core retry logic with exponential backoff.
func (r *RetryNodeRepository) executeWithRetry(
	ctx context.Context,
	operation string,
	fn func() error,
	idempotent bool,
) error {
	var lastErr error
	
	// If operation is not idempotent and we have no way to verify success,
	// we should be very conservative about retries
	maxRetries := r.config.MaxRetries
	if !idempotent {
		maxRetries = min(maxRetries, 1) // At most one retry for non-idempotent ops
	}
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}
		
		// Execute the operation
		err := fn()
		
		// Success - return immediately
		if err == nil {
			if attempt > 0 {
				r.logger.Info("operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt),
				)
			}
			return nil
		}
		
		lastErr = err
		
		// Check if we should retry
		if attempt >= maxRetries {
			break // No more retries
		}
		
		if !r.shouldRetry(err, idempotent) {
			break // Error is not retryable
		}
		
		// Calculate delay with exponential backoff and jitter
		delay := r.calculateDelay(attempt)
		
		// Call retry callback if configured
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err)
		}
		
		r.logger.Warn("retrying operation",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Duration("delay", delay),
			zap.Error(err),
		)
		
		// Wait before retrying
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry delay: %w", ctx.Err())
		}
	}
	
	// All retries exhausted
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, lastErr)
}

// shouldRetry determines if an error is retryable.
func (r *RetryNodeRepository) shouldRetry(err error, idempotent bool) bool {
	if err == nil {
		return false
	}
	
	// Don't retry non-idempotent operations unless we're certain it's safe
	if !idempotent {
		// Only retry network-level errors where we know the request didn't reach the server
		return isNetworkError(err)
	}
	
	// Check for specific retryable errors
	for _, retryableErr := range r.config.RetryableErrors {
		if err == retryableErr {
			return true
		}
	}
	
	// Check for timeout errors
	if r.config.RetryOnTimeout && isTimeoutError(err) {
		return true
	}
	
	// Check for server errors (5xx equivalent)
	if r.config.RetryOn5xx && isServerError(err) {
		return true
	}
	
	// Check for throttling errors
	if isThrottlingError(err) {
		return true
	}
	
	// Default: don't retry
	return false
}

// calculateDelay calculates the delay before the next retry attempt.
func (r *RetryNodeRepository) calculateDelay(attempt int) time.Duration {
	// Base delay with exponential backoff
	baseDelay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt))
	
	// Apply maximum delay cap
	if baseDelay > float64(r.config.MaxDelay) {
		baseDelay = float64(r.config.MaxDelay)
	}
	
	// Add jitter to prevent thundering herd
	jitter := r.config.JitterFactor * baseDelay * (r.rand.Float64()*2 - 1) // -jitter to +jitter
	finalDelay := baseDelay + jitter
	
	// Ensure delay is not negative
	if finalDelay < 0 {
		finalDelay = 0
	}
	
	return time.Duration(finalDelay)
}

// ============================================================================
// RETRY EDGE REPOSITORY
// ============================================================================

// RetryEdgeRepository adds retry logic to EdgeRepository operations.
type RetryEdgeRepository struct {
	inner  repository.EdgeRepository
	config RetryConfig
	logger *zap.Logger
	rand   *rand.Rand
}

// NewRetryEdgeRepository creates a new retry decorator for EdgeRepository.
func NewRetryEdgeRepository(
	inner repository.EdgeRepository,
	config RetryConfig,
) repository.EdgeRepository {
	return &RetryEdgeRepository{
		inner:  inner,
		config: config,
		logger: zap.L().Named("retry_edge_repository"),
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateEdges retries edge creation with caution.
func (r *RetryEdgeRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	return r.executeWithRetry(ctx, "CreateEdges", func() error {
		return r.inner.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
	}, false) // Not idempotent unless we check for existing edges
}

// CreateEdge retries single edge creation.
func (r *RetryEdgeRepository) CreateEdge(ctx context.Context, edge *edge.Edge) error {
	return r.executeWithRetry(ctx, "CreateEdge", func() error {
		return r.inner.CreateEdge(ctx, edge)
	}, false)
}

// FindEdges retries edge queries.
func (r *RetryEdgeRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	var result []*edge.Edge
	err := r.executeWithRetry(ctx, "FindEdges", func() error {
		var err error
		result, err = r.inner.FindEdges(ctx, query)
		return err
	}, true)
	return result, err
}

// GetEdgesPage retries paginated edge queries.
func (r *RetryEdgeRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	var result *repository.EdgePage
	err := r.executeWithRetry(ctx, "GetEdgesPage", func() error {
		var err error
		result, err = r.inner.GetEdgesPage(ctx, query, pagination)
		return err
	}, true)
	return result, err
}

// FindEdgesWithOptions retries enhanced edge queries.
func (r *RetryEdgeRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	var result []*edge.Edge
	err := r.executeWithRetry(ctx, "FindEdgesWithOptions", func() error {
		var err error
		result, err = r.inner.FindEdgesWithOptions(ctx, query, opts...)
		return err
	}, true)
	return result, err
}

// executeWithRetry implements the retry logic (same as NodeRepository).
func (r *RetryEdgeRepository) executeWithRetry(
	ctx context.Context,
	operation string,
	fn func() error,
	idempotent bool,
) error {
	// Implementation is identical to RetryNodeRepository.executeWithRetry
	// In production, this would be extracted to a shared helper
	var lastErr error
	maxRetries := r.config.MaxRetries
	if !idempotent {
		maxRetries = min(maxRetries, 1)
	}
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}
		
		err := fn()
		if err == nil {
			if attempt > 0 {
				r.logger.Info("operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt),
				)
			}
			return nil
		}
		
		lastErr = err
		
		if attempt >= maxRetries || !shouldRetryError(err, idempotent, &r.config) {
			break
		}
		
		delay := calculateRetryDelay(attempt, &r.config, r.rand)
		
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, err)
		}
		
		r.logger.Warn("retrying operation",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Duration("delay", delay),
			zap.Error(err),
		)
		
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry delay: %w", ctx.Err())
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries+1, lastErr)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// Helper functions to determine error types
func isNetworkError(err error) bool {
	// Check for network-related errors
	// In production, would check for specific network error types
	return false
}

func isTimeoutError(err error) bool {
	// Check for timeout errors
	// Would check for context.DeadlineExceeded, etc.
	return false
}

func isServerError(err error) bool {
	// Check for server errors (5xx equivalent)
	return false
}

func isThrottlingError(err error) bool {
	// Check for rate limiting / throttling errors
	return false
}

func shouldRetryError(err error, idempotent bool, config *RetryConfig) bool {
	if err == nil {
		return false
	}
	
	if !idempotent {
		return isNetworkError(err)
	}
	
	for _, retryableErr := range config.RetryableErrors {
		if err == retryableErr {
			return true
		}
	}
	
	return (config.RetryOnTimeout && isTimeoutError(err)) ||
		(config.RetryOn5xx && isServerError(err)) ||
		isThrottlingError(err)
}

func calculateRetryDelay(attempt int, config *RetryConfig, rnd *rand.Rand) time.Duration {
	baseDelay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt))
	
	if baseDelay > float64(config.MaxDelay) {
		baseDelay = float64(config.MaxDelay)
	}
	
	jitter := config.JitterFactor * baseDelay * (rnd.Float64()*2 - 1)
	finalDelay := baseDelay + jitter
	
	if finalDelay < 0 {
		finalDelay = 0
	}
	
	return time.Duration(finalDelay)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}