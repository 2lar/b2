package errors

import (
	"context"
	"time"
)

// CompensationFunc represents a function that can compensate for an error
type CompensationFunc func(ctx context.Context) error

// ErrorRecoveryStrategy defines how to recover from specific errors
type ErrorRecoveryStrategy interface {
	// CanRecover determines if this strategy can handle the error
	CanRecover(err error) bool
	
	// Recover attempts to recover from the error
	Recover(ctx context.Context, err error) error
	
	// Name returns the strategy name for logging
	Name() string
	
	// Priority returns the priority (higher = try first)
	Priority() int
}

// RecoveryManager manages multiple recovery strategies
type RecoveryManager struct {
	strategies []ErrorRecoveryStrategy
	maxRetries int
	timeout    time.Duration
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager() *RecoveryManager {
	return &RecoveryManager{
		strategies: []ErrorRecoveryStrategy{},
		maxRetries: 3,
		timeout:    30 * time.Second,
	}
}

// AddStrategy adds a recovery strategy
func (m *RecoveryManager) AddStrategy(strategy ErrorRecoveryStrategy) {
	m.strategies = append(m.strategies, strategy)
	// Sort by priority
	m.sortStrategies()
}

// sortStrategies sorts strategies by priority (highest first)
func (m *RecoveryManager) sortStrategies() {
	// Simple bubble sort for small number of strategies
	n := len(m.strategies)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if m.strategies[j].Priority() < m.strategies[j+1].Priority() {
				m.strategies[j], m.strategies[j+1] = m.strategies[j+1], m.strategies[j]
			}
		}
	}
}

// AttemptRecovery tries to recover from an error using available strategies
func (m *RecoveryManager) AttemptRecovery(ctx context.Context, err error) error {
	// Create context with timeout
	recoveryCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	
	// Try each strategy in priority order
	for _, strategy := range m.strategies {
		if !strategy.CanRecover(err) {
			continue
		}
		
		// Attempt recovery with this strategy
		if recoveryErr := strategy.Recover(recoveryCtx, err); recoveryErr == nil {
			return nil // Recovery successful
		}
	}
	
	// No strategy could recover
	return err
}

// Built-in Recovery Strategies

// RetryRecoveryStrategy recovers by retrying the operation
type RetryRecoveryStrategy struct {
	retryDelay time.Duration
	maxRetries int
}

func NewRetryRecoveryStrategy(delay time.Duration, maxRetries int) *RetryRecoveryStrategy {
	return &RetryRecoveryStrategy{
		retryDelay: delay,
		maxRetries: maxRetries,
	}
}

func (s *RetryRecoveryStrategy) CanRecover(err error) bool {
	// Check if error is retryable
	if unifiedErr, ok := err.(*UnifiedError); ok {
		return unifiedErr.Retryable && unifiedErr.RetryCount < s.maxRetries
	}
	return false
}

func (s *RetryRecoveryStrategy) Recover(ctx context.Context, err error) error {
	unifiedErr, ok := err.(*UnifiedError)
	if !ok {
		return err
	}
	
	// Wait for retry delay (or use RetryAfter if specified)
	delay := s.retryDelay
	if unifiedErr.RetryAfter > 0 {
		delay = unifiedErr.RetryAfter
	}
	
	select {
	case <-time.After(delay):
		// Ready to retry
	case <-ctx.Done():
		return ctx.Err()
	}
	
	// Update retry count
	unifiedErr.RetryCount++
	
	// Recovery strategy doesn't actually retry the operation,
	// it just prepares for retry. The caller should retry.
	return nil
}

func (s *RetryRecoveryStrategy) Name() string {
	return "RetryRecovery"
}

func (s *RetryRecoveryStrategy) Priority() int {
	return 100 // High priority
}

// CircuitBreakerRecoveryStrategy recovers by opening a circuit breaker
type CircuitBreakerRecoveryStrategy struct {
	failureThreshold int
	resetTimeout     time.Duration
}

func NewCircuitBreakerRecoveryStrategy(threshold int, resetTimeout time.Duration) *CircuitBreakerRecoveryStrategy {
	return &CircuitBreakerRecoveryStrategy{
		failureThreshold: threshold,
		resetTimeout:     resetTimeout,
	}
}

func (s *CircuitBreakerRecoveryStrategy) CanRecover(err error) bool {
	// Can recover from connection or timeout errors
	if unifiedErr, ok := err.(*UnifiedError); ok {
		return unifiedErr.Type == ErrorTypeConnection || 
		       unifiedErr.Type == ErrorTypeTimeout ||
		       unifiedErr.Type == ErrorTypeUnavailable
	}
	return false
}

func (s *CircuitBreakerRecoveryStrategy) Recover(ctx context.Context, err error) error {
	// Circuit breaker recovery would typically:
	// 1. Open the circuit to prevent cascading failures
	// 2. Route to fallback service
	// 3. Return cached data
	// This is a simplified implementation
	
	unifiedErr, ok := err.(*UnifiedError)
	if !ok {
		return err
	}
	
	// Set recovery strategy
	unifiedErr.RecoveryStrategy = "CircuitBreakerOpen"
	unifiedErr.RecoveryMetadata = map[string]interface{}{
		"resetAfter": time.Now().Add(s.resetTimeout),
		"threshold":  s.failureThreshold,
	}
	
	return nil
}

func (s *CircuitBreakerRecoveryStrategy) Name() string {
	return "CircuitBreakerRecovery"
}

func (s *CircuitBreakerRecoveryStrategy) Priority() int {
	return 90
}

// CompensationRecoveryStrategy recovers by executing compensation
type CompensationRecoveryStrategy struct{}

func NewCompensationRecoveryStrategy() *CompensationRecoveryStrategy {
	return &CompensationRecoveryStrategy{}
}

func (s *CompensationRecoveryStrategy) CanRecover(err error) bool {
	// Can recover if error has compensation function
	if unifiedErr, ok := err.(*UnifiedError); ok {
		return unifiedErr.CompensationFunc != nil
	}
	return false
}

func (s *CompensationRecoveryStrategy) Recover(ctx context.Context, err error) error {
	unifiedErr, ok := err.(*UnifiedError)
	if !ok || unifiedErr.CompensationFunc == nil {
		return err
	}
	
	// Execute compensation
	if compensationErr := unifiedErr.CompensationFunc(ctx); compensationErr != nil {
		// Compensation failed, wrap both errors
		return NewError(ErrorTypeInternal, "COMPENSATION_FAILED", "Compensation failed").
			WithDetails("Original error: " + err.Error() + ", Compensation error: " + compensationErr.Error()).
			WithCause(err).
			Build()
	}
	
	// Compensation successful
	unifiedErr.RecoveryStrategy = "Compensated"
	return nil
}

func (s *CompensationRecoveryStrategy) Name() string {
	return "CompensationRecovery"
}

func (s *CompensationRecoveryStrategy) Priority() int {
	return 80
}

// FallbackRecoveryStrategy recovers by using fallback values
type FallbackRecoveryStrategy struct {
	fallbackProvider func(ctx context.Context, err error) (interface{}, error)
}

func NewFallbackRecoveryStrategy(provider func(context.Context, error) (interface{}, error)) *FallbackRecoveryStrategy {
	return &FallbackRecoveryStrategy{
		fallbackProvider: provider,
	}
}

func (s *FallbackRecoveryStrategy) CanRecover(err error) bool {
	// Can provide fallback for non-critical errors
	if unifiedErr, ok := err.(*UnifiedError); ok {
		return unifiedErr.Severity != SeverityCritical
	}
	return false
}

func (s *FallbackRecoveryStrategy) Recover(ctx context.Context, err error) error {
	if s.fallbackProvider == nil {
		return err
	}
	
	fallbackValue, fallbackErr := s.fallbackProvider(ctx, err)
	if fallbackErr != nil {
		return fallbackErr
	}
	
	// Store fallback value in recovery metadata
	if unifiedErr, ok := err.(*UnifiedError); ok {
		unifiedErr.RecoveryStrategy = "Fallback"
		unifiedErr.RecoveryMetadata = map[string]interface{}{
			"fallbackValue": fallbackValue,
			"originalError": err.Error(),
		}
	}
	
	return nil
}

func (s *FallbackRecoveryStrategy) Name() string {
	return "FallbackRecovery"
}

func (s *FallbackRecoveryStrategy) Priority() int {
	return 70
}

// Helper functions for error recovery

// WithRecoveryManager attaches a recovery manager to the context
func WithRecoveryManager(ctx context.Context, manager *RecoveryManager) context.Context {
	return context.WithValue(ctx, "recoveryManager", manager)
}

// GetRecoveryManager retrieves the recovery manager from context
func GetRecoveryManager(ctx context.Context) *RecoveryManager {
	if manager, ok := ctx.Value("recoveryManager").(*RecoveryManager); ok {
		return manager
	}
	return nil
}

// AttemptRecoveryWithContext attempts recovery using context's recovery manager
func AttemptRecoveryWithContext(ctx context.Context, err error) error {
	manager := GetRecoveryManager(ctx)
	if manager == nil {
		return err
	}
	return manager.AttemptRecovery(ctx, err)
}