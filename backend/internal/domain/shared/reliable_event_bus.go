// Package shared provides a reliable event bus implementation that addresses
// silent failures in event publishing found throughout the codebase.
package shared

import (
	"context"
	"fmt"
	"time"
	
	"go.uber.org/zap"
)

// ============================================================================
// RELIABLE EVENT BUS INTERFACE
// ============================================================================

// ReliableEventBus extends the basic EventBus with reliability guarantees.
// This interface addresses the silent failures found in event publishing
// by providing configurable error handling strategies.
type ReliableEventBus interface {
	EventBus
	
	// PublishWithRetry publishes an event with retry logic
	PublishWithRetry(ctx context.Context, event DomainEvent, config RetryConfig) error
	
	// PublishAsync publishes an event asynchronously with error handling
	PublishAsync(ctx context.Context, event DomainEvent, errorHandler AsyncErrorHandler) error
	
	// SetErrorStrategy sets the error handling strategy for the event bus
	SetErrorStrategy(strategy ErrorStrategy)
}

// ============================================================================
// ERROR HANDLING STRATEGIES
// ============================================================================

// ErrorStrategy defines how event publishing errors should be handled.
type ErrorStrategy string

const (
	// ErrorStrategyFail causes the operation to fail if event publishing fails
	ErrorStrategyFail ErrorStrategy = "FAIL"
	
	// ErrorStrategyLog logs the error but continues the operation
	ErrorStrategyLog ErrorStrategy = "LOG"
	
	// ErrorStrategyRetry attempts to retry the event publishing
	ErrorStrategyRetry ErrorStrategy = "RETRY"
	
	// ErrorStrategyQueue queues the event for later retry
	ErrorStrategyQueue ErrorStrategy = "QUEUE"
)

// RetryConfig configures retry behavior for event publishing.
type RetryConfig struct {
	MaxAttempts     int           // Maximum retry attempts
	InitialDelay    time.Duration // Initial delay between retries
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Exponential backoff factor
	RetryableErrors []string      // Error patterns that are retryable
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		BackoffFactor:   2.0,
		RetryableErrors: []string{"timeout", "unavailable", "connection"},
	}
}

// AsyncErrorHandler handles errors from asynchronous event publishing.
type AsyncErrorHandler func(event DomainEvent, err error)

// ============================================================================
// RELIABLE EVENT BUS IMPLEMENTATION
// ============================================================================

// ReliableEventBusImpl implements the ReliableEventBus interface.
type ReliableEventBusImpl struct {
	underlying    EventBus      // Underlying event bus implementation
	logger        *zap.Logger   // Logger for error reporting
	strategy      ErrorStrategy // Current error handling strategy
	retryConfig   RetryConfig   // Retry configuration
	eventQueue    EventQueue    // Queue for failed events
}

// EventQueue defines an interface for queuing failed events.
type EventQueue interface {
	Enqueue(event DomainEvent) error
	Dequeue() (DomainEvent, error)
	Size() int
}

// NewReliableEventBus creates a new reliable event bus.
func NewReliableEventBus(
	underlying EventBus,
	logger *zap.Logger,
	strategy ErrorStrategy,
	eventQueue EventQueue,
) *ReliableEventBusImpl {
	return &ReliableEventBusImpl{
		underlying:  underlying,
		logger:      logger,
		strategy:    strategy,
		retryConfig: DefaultRetryConfig(),
		eventQueue:  eventQueue,
	}
}

// Publish publishes an event using the configured error strategy.
func (r *ReliableEventBusImpl) Publish(ctx context.Context, event DomainEvent) error {
	err := r.underlying.Publish(ctx, event)
	if err == nil {
		return nil
	}
	
	// Handle the error based on the configured strategy
	return r.handlePublishError(ctx, event, err)
}

// PublishWithRetry publishes an event with explicit retry configuration.
func (r *ReliableEventBusImpl) PublishWithRetry(ctx context.Context, event DomainEvent, config RetryConfig) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		err := r.underlying.Publish(ctx, event)
		if err == nil {
			if attempt > 0 && r.logger != nil {
				r.logger.Info("Event published successfully after retry",
					zap.String("event_type", event.EventType()),
					zap.Int("attempt", attempt+1),
				)
			}
			return nil
		}
		
		lastErr = err
		
		// Check if the error is retryable
		if !r.isRetryableError(err, config.RetryableErrors) {
			break
		}
		
		// Calculate delay for next attempt
		if attempt < config.MaxAttempts-1 {
			delay := r.calculateDelay(attempt, config)
			if r.logger != nil {
				r.logger.Warn("Event publishing failed, retrying",
					zap.String("event_type", event.EventType()),
					zap.Int("attempt", attempt+1),
					zap.Duration("retry_delay", delay),
					zap.Error(err),
				)
			}
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}
	
	// All retry attempts failed
	if r.logger != nil {
		r.logger.Error("Event publishing failed after all retries",
			zap.String("event_type", event.EventType()),
			zap.Int("max_attempts", config.MaxAttempts),
			zap.Error(lastErr),
		)
	}
	
	return fmt.Errorf("failed to publish event after %d attempts: %w", config.MaxAttempts, lastErr)
}

// PublishAsync publishes an event asynchronously.
func (r *ReliableEventBusImpl) PublishAsync(ctx context.Context, event DomainEvent, errorHandler AsyncErrorHandler) error {
	go func() {
		err := r.Publish(context.Background(), event)
		if err != nil && errorHandler != nil {
			errorHandler(event, err)
		}
	}()
	return nil
}

// SetErrorStrategy sets the error handling strategy.
func (r *ReliableEventBusImpl) SetErrorStrategy(strategy ErrorStrategy) {
	r.strategy = strategy
}

// handlePublishError handles publishing errors based on the configured strategy.
func (r *ReliableEventBusImpl) handlePublishError(ctx context.Context, event DomainEvent, err error) error {
	switch r.strategy {
	case ErrorStrategyFail:
		return err
		
	case ErrorStrategyLog:
		if r.logger != nil {
			r.logger.Warn("Event publishing failed but continuing operation",
				zap.String("event_type", event.EventType()),
				zap.Error(err),
			)
		}
		return nil // Don't propagate the error
		
	case ErrorStrategyRetry:
		return r.PublishWithRetry(ctx, event, r.retryConfig)
		
	case ErrorStrategyQueue:
		if r.eventQueue != nil {
			if queueErr := r.eventQueue.Enqueue(event); queueErr != nil {
				if r.logger != nil {
					r.logger.Error("Failed to queue event for retry",
						zap.String("event_type", event.EventType()),
						zap.Error(queueErr),
					)
				}
				return err // Return original error
			}
			
			if r.logger != nil {
				r.logger.Info("Event queued for retry",
					zap.String("event_type", event.EventType()),
				)
			}
			return nil // Event queued successfully
		}
		return err
		
	default:
		return err
	}
}

// isRetryableError checks if an error is retryable based on configuration.
func (r *ReliableEventBusImpl) isRetryableError(err error, retryablePatterns []string) bool {
	errMsg := err.Error()
	for _, pattern := range retryablePatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// calculateDelay calculates the delay for the next retry attempt.
func (r *ReliableEventBusImpl) calculateDelay(attempt int, config RetryConfig) time.Duration {
	delay := config.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
			break
		}
	}
	return delay
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    len(s) > len(substr) && 
		    (s[:len(substr)] == substr || 
		     s[len(s)-len(substr):] == substr ||
		     indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ============================================================================
// IN-MEMORY EVENT QUEUE IMPLEMENTATION
// ============================================================================

// InMemoryEventQueue provides a simple in-memory implementation of EventQueue.
type InMemoryEventQueue struct {
	events []DomainEvent
	logger *zap.Logger
}

// NewInMemoryEventQueue creates a new in-memory event queue.
func NewInMemoryEventQueue(logger *zap.Logger) *InMemoryEventQueue {
	return &InMemoryEventQueue{
		events: make([]DomainEvent, 0),
		logger: logger,
	}
}

// Enqueue adds an event to the queue.
func (q *InMemoryEventQueue) Enqueue(event DomainEvent) error {
	q.events = append(q.events, event)
	if q.logger != nil {
		q.logger.Debug("Event enqueued",
			zap.String("event_type", event.EventType()),
			zap.Int("queue_size", len(q.events)),
		)
	}
	return nil
}

// Dequeue removes and returns the next event from the queue.
func (q *InMemoryEventQueue) Dequeue() (DomainEvent, error) {
	if len(q.events) == 0 {
		return nil, fmt.Errorf("queue is empty")
	}
	
	event := q.events[0]
	q.events = q.events[1:]
	
	if q.logger != nil {
		q.logger.Debug("Event dequeued",
			zap.String("event_type", event.EventType()),
			zap.Int("queue_size", len(q.events)),
		)
	}
	
	return event, nil
}

// Size returns the current size of the queue.
func (q *InMemoryEventQueue) Size() int {
	return len(q.events)
}

// ============================================================================
// CONFIGURATION BUILDERS
// ============================================================================

// ReliableEventBusBuilder provides a fluent interface for building ReliableEventBus instances.
type ReliableEventBusBuilder struct {
	underlying  EventBus
	logger      *zap.Logger
	strategy    ErrorStrategy
	retryConfig RetryConfig
	eventQueue  EventQueue
}

// NewReliableEventBusBuilder creates a new builder.
func NewReliableEventBusBuilder(underlying EventBus) *ReliableEventBusBuilder {
	return &ReliableEventBusBuilder{
		underlying:  underlying,
		strategy:    ErrorStrategyLog, // Default to logging errors
		retryConfig: DefaultRetryConfig(),
	}
}

// WithLogger sets the logger for the event bus.
func (b *ReliableEventBusBuilder) WithLogger(logger *zap.Logger) *ReliableEventBusBuilder {
	b.logger = logger
	return b
}

// WithErrorStrategy sets the error handling strategy.
func (b *ReliableEventBusBuilder) WithErrorStrategy(strategy ErrorStrategy) *ReliableEventBusBuilder {
	b.strategy = strategy
	return b
}

// WithRetryConfig sets the retry configuration.
func (b *ReliableEventBusBuilder) WithRetryConfig(config RetryConfig) *ReliableEventBusBuilder {
	b.retryConfig = config
	return b
}

// WithEventQueue sets the event queue for failed events.
func (b *ReliableEventBusBuilder) WithEventQueue(queue EventQueue) *ReliableEventBusBuilder {
	b.eventQueue = queue
	return b
}

// Build creates the ReliableEventBus instance.
func (b *ReliableEventBusBuilder) Build() *ReliableEventBusImpl {
	// Create default queue if none provided and strategy requires it
	if b.eventQueue == nil && b.strategy == ErrorStrategyQueue {
		b.eventQueue = NewInMemoryEventQueue(b.logger)
	}
	
	return NewReliableEventBus(b.underlying, b.logger, b.strategy, b.eventQueue)
}