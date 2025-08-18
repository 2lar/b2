// Package persistence - Circuit Breaker decorator for preventing cascading failures.
package persistence

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	
	"go.uber.org/zap"
)

// ============================================================================
// CIRCUIT BREAKER DECORATOR - Prevents cascading failures
// ============================================================================

// CircuitBreakerConfig configures circuit breaker behavior.
//
// The Circuit Breaker pattern prevents cascading failures by:
//   1. Monitoring failure rates
//   2. Opening the circuit when failures exceed threshold
//   3. Rejecting requests while circuit is open
//   4. Periodically testing if service has recovered
//   5. Closing circuit when service is healthy again
//
// States:
//   - Closed: Normal operation, requests pass through
//   - Open: Failures exceeded threshold, requests are rejected
//   - Half-Open: Testing if service has recovered
type CircuitBreakerConfig struct {
	// Failure detection
	FailureThreshold   float64       // Failure rate to open circuit (e.g., 0.5 = 50%)
	SuccessThreshold   float64       // Success rate to close circuit from half-open
	MinimumRequests    int           // Minimum requests before evaluating threshold
	WindowSize         time.Duration // Time window for tracking requests
	
	// Circuit behavior
	OpenDuration       time.Duration // How long to keep circuit open
	HalfOpenRequests   int           // Max requests in half-open state
	
	// Advanced options
	OnStateChange      func(from, to CircuitState) // Callback on state changes
	OnRequestRejected  func()                       // Callback when request is rejected
	FallbackFunc       func() error                 // Fallback function when circuit is open
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 0.5,           // Open at 50% failure rate
		SuccessThreshold: 0.8,           // Close at 80% success rate
		MinimumRequests:  10,            // Need at least 10 requests
		WindowSize:       10 * time.Second,
		OpenDuration:     30 * time.Second,
		HalfOpenRequests: 3,
	}
}

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker tracks request statistics and manages circuit state.
type CircuitBreaker struct {
	config CircuitBreakerConfig
	logger *zap.Logger
	
	// State management
	state           atomic.Value // CircuitState
	lastStateChange time.Time
	stateMutex      sync.RWMutex
	
	// Request tracking
	requestWindow   *slidingWindow
	halfOpenCounter int32
	
	// Metrics
	totalRequests   int64
	totalFailures   int64
	consecutiveFails int32
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		config:        config,
		logger:        logger,
		requestWindow: newSlidingWindow(config.WindowSize),
	}
	cb.state.Store(StateClosed)
	return cb
}

// Execute runs a function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check circuit state
	state := cb.getState()
	
	switch state {
	case StateOpen:
		// Check if we should transition to half-open
		if cb.shouldAttemptReset() {
			cb.transitionTo(StateHalfOpen)
			return cb.executeInHalfOpen(fn)
		}
		
		// Circuit is open, reject request
		atomic.AddInt64(&cb.totalRequests, 1)
		if cb.config.OnRequestRejected != nil {
			cb.config.OnRequestRejected()
		}
		
		// Try fallback if configured
		if cb.config.FallbackFunc != nil {
			return cb.config.FallbackFunc()
		}
		
		return ErrCircuitBreakerOpen
		
	case StateHalfOpen:
		return cb.executeInHalfOpen(fn)
		
	case StateClosed:
		return cb.executeInClosed(fn)
		
	default:
		return fmt.Errorf("unknown circuit state: %v", state)
	}
}

// executeInClosed executes function when circuit is closed (normal operation).
func (cb *CircuitBreaker) executeInClosed(fn func() error) error {
	atomic.AddInt64(&cb.totalRequests, 1)
	
	err := fn()
	cb.recordResult(err == nil)
	
	if err != nil {
		atomic.AddInt64(&cb.totalFailures, 1)
		atomic.AddInt32(&cb.consecutiveFails, 1)
		
		// Check if we should open the circuit
		if cb.shouldOpen() {
			cb.transitionTo(StateOpen)
			cb.logger.Warn("circuit breaker opened due to high failure rate",
				zap.Float64("failure_rate", cb.getFailureRate()),
				zap.Int64("total_requests", atomic.LoadInt64(&cb.totalRequests)),
			)
		}
	} else {
		atomic.StoreInt32(&cb.consecutiveFails, 0)
	}
	
	return err
}

// executeInHalfOpen executes function when circuit is half-open (testing recovery).
func (cb *CircuitBreaker) executeInHalfOpen(fn func() error) error {
	// Check if we've exceeded half-open request limit
	count := atomic.AddInt32(&cb.halfOpenCounter, 1)
	if count > int32(cb.config.HalfOpenRequests) {
		// Too many requests in half-open state
		return ErrCircuitBreakerOpen
	}
	
	atomic.AddInt64(&cb.totalRequests, 1)
	
	err := fn()
	cb.recordResult(err == nil)
	
	if err != nil {
		atomic.AddInt64(&cb.totalFailures, 1)
		// Failure in half-open state, reopen circuit
		cb.transitionTo(StateOpen)
		cb.logger.Info("circuit breaker reopened due to failure in half-open state")
	} else {
		// Check if we have enough successes to close the circuit
		if cb.shouldClose() {
			cb.transitionTo(StateClosed)
			cb.logger.Info("circuit breaker closed after successful recovery")
		}
	}
	
	return err
}

// shouldOpen determines if the circuit should open based on failure rate.
func (cb *CircuitBreaker) shouldOpen() bool {
	stats := cb.requestWindow.getStats()
	
	// Need minimum requests before evaluating
	if stats.total < cb.config.MinimumRequests {
		return false
	}
	
	failureRate := float64(stats.failures) / float64(stats.total)
	return failureRate >= cb.config.FailureThreshold
}

// shouldClose determines if the circuit should close from half-open state.
func (cb *CircuitBreaker) shouldClose() bool {
	stats := cb.requestWindow.getStats()
	
	// In half-open state, we look at recent success rate
	if stats.total == 0 {
		return false
	}
	
	successRate := float64(stats.successes) / float64(stats.total)
	return successRate >= cb.config.SuccessThreshold
}

// shouldAttemptReset determines if we should try transitioning from open to half-open.
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	cb.stateMutex.RLock()
	defer cb.stateMutex.RUnlock()
	
	return time.Since(cb.lastStateChange) >= cb.config.OpenDuration
}

// transitionTo changes the circuit breaker state.
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	cb.stateMutex.Lock()
	defer cb.stateMutex.Unlock()
	
	oldState := cb.state.Load().(CircuitState)
	if oldState == newState {
		return // No change
	}
	
	cb.state.Store(newState)
	cb.lastStateChange = time.Now()
	
	// Reset half-open counter when entering half-open state
	if newState == StateHalfOpen {
		atomic.StoreInt32(&cb.halfOpenCounter, 0)
	}
	
	// Call state change callback if configured
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}
	
	cb.logger.Info("circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", newState.String()),
	)
}

// getState returns the current circuit state.
func (cb *CircuitBreaker) getState() CircuitState {
	return cb.state.Load().(CircuitState)
}

// recordResult records the result of a request.
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.requestWindow.record(success)
}

// getFailureRate returns the current failure rate.
func (cb *CircuitBreaker) getFailureRate() float64 {
	stats := cb.requestWindow.getStats()
	if stats.total == 0 {
		return 0
	}
	return float64(stats.failures) / float64(stats.total)
}

// ============================================================================
// CIRCUIT BREAKER NODE REPOSITORY
// ============================================================================

// CircuitBreakerNodeRepository adds circuit breaker protection to NodeRepository.
//
// This decorator demonstrates:
//   1. Fault Tolerance: Prevents cascading failures
//   2. Fast Fail: Returns errors quickly when service is down
//   3. Automatic Recovery: Tests and recovers when service is back
//   4. Fallback Support: Can provide degraded service when circuit is open
type CircuitBreakerNodeRepository struct {
	inner          repository.NodeRepository
	circuitBreaker *CircuitBreaker
	logger         *zap.Logger
}

// NewCircuitBreakerNodeRepository creates a new circuit breaker decorator.
func NewCircuitBreakerNodeRepository(
	inner repository.NodeRepository,
	config CircuitBreakerConfig,
) repository.NodeRepository {
	logger := zap.L().Named("cb_node_repository")
	return &CircuitBreakerNodeRepository{
		inner:          inner,
		circuitBreaker: NewCircuitBreaker(config, logger),
		logger:         logger,
	}
}

// CreateNodeAndKeywords wraps create operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	return r.circuitBreaker.Execute(func() error {
		return r.inner.CreateNodeAndKeywords(ctx, node)
	})
}

// FindNodeByID wraps find operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	var result *node.Node
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.FindNodeByID(ctx, userID, nodeID)
		return err
	})
	return result, err
}

// FindNodes wraps search operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	var result []*node.Node
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.FindNodes(ctx, query)
		return err
	})
	return result, err
}

// DeleteNode wraps delete operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.circuitBreaker.Execute(func() error {
		return r.inner.DeleteNode(ctx, userID, nodeID)
	})
}

// BatchDeleteNodes wraps batch delete operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	err = r.circuitBreaker.Execute(func() error {
		var innerErr error
		deleted, failed, innerErr = r.inner.BatchDeleteNodes(ctx, userID, nodeIDs)
		return innerErr
	})
	return deleted, failed, err
}

// GetNodesPage wraps paginated query with circuit breaker.
func (r *CircuitBreakerNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	var result *repository.NodePage
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.GetNodesPage(ctx, query, pagination)
		return err
	})
	return result, err
}

// GetNodeNeighborhood wraps graph query with circuit breaker.
func (r *CircuitBreakerNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	var result *shared.Graph
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
		return err
	})
	return result, err
}

// CountNodes wraps count operation with circuit breaker.
func (r *CircuitBreakerNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	var result int
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.CountNodes(ctx, userID)
		return err
	})
	return result, err
}

// FindNodesWithOptions wraps enhanced query with circuit breaker.
func (r *CircuitBreakerNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	var result []*node.Node
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.FindNodesWithOptions(ctx, query, opts...)
		return err
	})
	return result, err
}

// FindNodesPageWithOptions wraps enhanced paginated query with circuit breaker.
func (r *CircuitBreakerNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	var result *repository.NodePage
	err := r.circuitBreaker.Execute(func() error {
		var err error
		result, err = r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
		return err
	})
	return result, err
}

// ============================================================================
// SLIDING WINDOW - For tracking request statistics
// ============================================================================

// slidingWindow tracks requests over a time window.
type slidingWindow struct {
	windowSize time.Duration
	buckets    []bucket
	mutex      sync.RWMutex
}

type bucket struct {
	timestamp time.Time
	successes int
	failures  int
}

type windowStats struct {
	total     int
	successes int
	failures  int
}

func newSlidingWindow(windowSize time.Duration) *slidingWindow {
	return &slidingWindow{
		windowSize: windowSize,
		buckets:    make([]bucket, 0),
	}
}

func (w *slidingWindow) record(success bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	
	now := time.Now()
	w.cleanup(now)
	
	// Find or create current bucket (1-second granularity)
	bucketTime := now.Truncate(time.Second)
	
	var currentBucket *bucket
	for i := range w.buckets {
		if w.buckets[i].timestamp.Equal(bucketTime) {
			currentBucket = &w.buckets[i]
			break
		}
	}
	
	if currentBucket == nil {
		w.buckets = append(w.buckets, bucket{
			timestamp: bucketTime,
		})
		currentBucket = &w.buckets[len(w.buckets)-1]
	}
	
	if success {
		currentBucket.successes++
	} else {
		currentBucket.failures++
	}
}

func (w *slidingWindow) getStats() windowStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-w.windowSize)
	
	stats := windowStats{}
	for _, b := range w.buckets {
		if b.timestamp.After(cutoff) {
			stats.successes += b.successes
			stats.failures += b.failures
		}
	}
	
	stats.total = stats.successes + stats.failures
	return stats
}

func (w *slidingWindow) cleanup(now time.Time) {
	cutoff := now.Add(-w.windowSize)
	
	// Remove old buckets
	i := 0
	for i < len(w.buckets) && w.buckets[i].timestamp.Before(cutoff) {
		i++
	}
	
	if i > 0 {
		w.buckets = w.buckets[i:]
	}
}

// ============================================================================
// ERROR DEFINITIONS
// ============================================================================

// ErrCircuitBreakerOpen is returned when the circuit breaker is open.
var ErrCircuitBreakerOpen = fmt.Errorf("circuit breaker is open")