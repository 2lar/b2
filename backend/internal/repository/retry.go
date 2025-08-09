package repository

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// RetryConfig defines retry behavior configuration
type RetryConfig struct {
	MaxAttempts   int           // Maximum number of retry attempts
	BaseDelay     time.Duration // Base delay between retries
	MaxDelay      time.Duration // Maximum delay between retries
	BackoffFactor float64       // Exponential backoff multiplier
	JitterFactor  float64       // Jitter factor to prevent thundering herd
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
	}
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err       error
	Retryable bool
	Temporary bool
}

func (e RetryableError) Error() string {
	return e.Err.Error()
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for our custom retryable error
	if retryErr, ok := err.(RetryableError); ok {
		return retryErr.Retryable
	}

	// Check for AWS DynamoDB specific retryable errors
	return isAWSRetryableError(err)
}

// isAWSRetryableError checks if an AWS error is retryable
func isAWSRetryableError(err error) bool {
	switch err.(type) {
	case *types.ProvisionedThroughputExceededException:
		return true
	case *types.RequestLimitExceeded:
		return true
	case *types.InternalServerError:
		return true
	case *types.ItemCollectionSizeLimitExceededException:
		return true
	case *types.LimitExceededException:
		return true
		// Note: ThrottlingException is handled by other cases above
	}

	// Check for transient network errors
	if awsErr, ok := err.(interface{ ErrorCode() string }); ok {
		switch awsErr.ErrorCode() {
		case "ServiceUnavailable", "Throttling", "RequestTimeout", "RequestLimitExceeded":
			return true
		}
	}

	return false
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// RetryWithBackoff executes an operation with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config RetryConfig, operation RetryableOperation) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			return err // Non-retryable error
		}

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := config.calculateDelay(attempt)

		// Wait before next attempt
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay for the given attempt number
func (c RetryConfig) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	backoff := float64(c.BaseDelay) * math.Pow(c.BackoffFactor, float64(attempt))

	// Apply jitter to prevent thundering herd
	jitter := backoff * c.JitterFactor * (rand.Float64() - 0.5) * 2
	delay := time.Duration(backoff + jitter)

	// Cap at maximum delay
	if delay > c.MaxDelay {
		delay = c.MaxDelay
	}

	return delay
}

// Circuit breaker states
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config       CircuitConfig
	state        CircuitState
	failures     int
	lastFailTime time.Time
	successCount int
}

// CircuitConfig defines circuit breaker configuration
type CircuitConfig struct {
	MaxFailures      int           // Maximum failures before opening circuit
	ResetTimeout     time.Duration // Time to wait before attempting reset
	HalfOpenMaxCalls int           // Maximum calls in half-open state
}

// DefaultCircuitConfig returns default circuit breaker configuration
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation RetryableOperation) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is open, operation rejected")
	}

	err := operation()
	cb.recordResult(err)
	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() bool {
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailTime) > cb.config.ResetTimeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.successCount < cb.config.HalfOpenMaxCalls
	}
	return false
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error) {
	if err == nil {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess handles successful operations
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitClosed:
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.HalfOpenMaxCalls {
			cb.state = CircuitClosed
			cb.failures = 0
		}
	}
}

// onFailure handles failed operations
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.state
}
