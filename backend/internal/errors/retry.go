// Package errors provides retry strategies for transient errors.
package errors

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryStrategy defines the retry behavior for operations.
type RetryStrategy struct {
	MaxAttempts     int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay between retries
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Multiplication factor for exponential backoff
	JitterEnabled   bool          // Whether to add random jitter to delays
	RetryableErrors []ErrorType   // Which error types should trigger retry
}

// DefaultRetryStrategy returns a sensible default retry strategy.
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		JitterEnabled: true,
		RetryableErrors: []ErrorType{
			ErrorTypeTimeout,
			ErrorTypeConnection,
			ErrorTypeRateLimit,
			ErrorTypeUnavailable,
		},
	}
}

// CalculateDelay calculates the delay for the given attempt number.
func (rs *RetryStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// Calculate exponential backoff
	delay := float64(rs.InitialDelay) * math.Pow(rs.BackoffFactor, float64(attempt-1))
	
	// Cap at max delay
	if delay > float64(rs.MaxDelay) {
		delay = float64(rs.MaxDelay)
	}
	
	// Add jitter if enabled (±25% of delay)
	if rs.JitterEnabled {
		jitter := (rand.Float64() - 0.5) * 0.5 * delay
		delay += jitter
	}
	
	return time.Duration(delay)
}

// ShouldRetry determines if an error should trigger a retry.
func (rs *RetryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= rs.MaxAttempts {
		return false
	}
	
	// Check if it's a UnifiedError
	unifiedErr, ok := err.(*UnifiedError)
	if !ok {
		// For non-UnifiedError, don't retry by default
		return false
	}
	
	// Check if the error is explicitly marked as retryable
	if !unifiedErr.Retryable {
		return false
	}
	
	// Check if the error type is in the retryable list
	for _, retryableType := range rs.RetryableErrors {
		if unifiedErr.Type == retryableType {
			return true
		}
	}
	
	return false
}

// RetryWithBackoff executes an operation with exponential backoff retry.
func RetryWithBackoff(ctx context.Context, strategy *RetryStrategy, operation func() error) error {
	var lastErr error
	
	for attempt := 1; attempt <= strategy.MaxAttempts; attempt++ {
		// Execute the operation
		err := operation()
		
		// Success - return immediately
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if we should retry
		if !strategy.ShouldRetry(err, attempt) {
			// Not retryable or max attempts reached
			break
		}
		
		// Check if it's the last attempt
		if attempt == strategy.MaxAttempts {
			break
		}
		
		// Calculate delay for next attempt
		delay := strategy.CalculateDelay(attempt)
		
		// Check for special retry-after header (for rate limiting)
		if unifiedErr, ok := err.(*UnifiedError); ok && unifiedErr.RetryAfter > 0 {
			delay = unifiedErr.RetryAfter
		}
		
		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			// Context cancelled - return context error wrapped with last error
			return Timeout("RETRY_CANCELLED", "Retry cancelled by context").
				WithCause(lastErr).
				WithDetails("Last error: " + lastErr.Error()).
				Build()
		}
		
		// Update retry count in error if it's a UnifiedError
		if unifiedErr, ok := lastErr.(*UnifiedError); ok {
			unifiedErr.RetryCount = attempt
			unifiedErr.MaxRetries = strategy.MaxAttempts
		}
	}
	
	// All retries exhausted - return the last error
	if unifiedErr, ok := lastErr.(*UnifiedError); ok {
		unifiedErr.RetryCount = strategy.MaxAttempts
		unifiedErr.MaxRetries = strategy.MaxAttempts
		unifiedErr.Retryable = false // No more retries possible
	}
	
	return lastErr
}

// RetryableOperation wraps an operation to make it retryable.
type RetryableOperation struct {
	Operation func() error
	Strategy  *RetryStrategy
	OnRetry   func(attempt int, err error, delay time.Duration) // Optional callback
}

// Execute runs the retryable operation.
func (ro *RetryableOperation) Execute(ctx context.Context) error {
	var lastErr error
	
	for attempt := 1; attempt <= ro.Strategy.MaxAttempts; attempt++ {
		// Execute the operation
		err := ro.Operation()
		
		// Success
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if we should retry
		if !ro.Strategy.ShouldRetry(err, attempt) || attempt == ro.Strategy.MaxAttempts {
			break
		}
		
		// Calculate delay
		delay := ro.Strategy.CalculateDelay(attempt)
		
		// Call retry callback if provided
		if ro.OnRetry != nil {
			ro.OnRetry(attempt, err, delay)
		}
		
		// Wait with cancellation support
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return EnrichWithContext(ctx, lastErr, "RetryableOperation", "Operation cancelled")
		}
	}
	
	return lastErr
}

// CircuitBreakerStrategy defines circuit breaker behavior for failing operations.
type CircuitBreakerStrategy struct {
	FailureThreshold   int           // Number of failures before opening circuit
	SuccessThreshold   int           // Number of successes before closing circuit
	Timeout            time.Duration // Timeout for half-open state
	consecutiveFailures int
	consecutiveSuccesses int
	state              CircuitState
	lastFailureTime    time.Time
}

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// ShouldAllow determines if a request should be allowed through the circuit breaker.
func (cb *CircuitBreakerStrategy) ShouldAllow() bool {
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.Timeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreakerStrategy) RecordSuccess() {
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses++
	
	if cb.state == CircuitHalfOpen && cb.consecutiveSuccesses >= cb.SuccessThreshold {
		cb.state = CircuitClosed
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreakerStrategy) RecordFailure() {
	cb.consecutiveSuccesses = 0
	cb.consecutiveFailures++
	cb.lastFailureTime = time.Now()
	
	if cb.consecutiveFailures >= cb.FailureThreshold {
		cb.state = CircuitOpen
	}
}