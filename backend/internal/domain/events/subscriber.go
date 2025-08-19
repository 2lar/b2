// Package events provides event-driven architecture components.
// This implementation completes the Observer pattern for domain events.
package events

import (
	"context"
	"sync"
	
	"brain2-backend/internal/domain/shared"
	"go.uber.org/zap"
)

// EventHandler processes domain events
type EventHandler interface {
	Handle(ctx context.Context, event shared.DomainEvent) error
	CanHandle(eventType string) bool
}

// EventBus manages event subscriptions and publishing.
// This is a simple in-memory implementation suitable for single-instance deployments.
type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewEventBus creates a new event bus instance
func NewEventBus(logger *zap.Logger) *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
		logger:   logger,
	}
}

// Subscribe registers a handler for a specific event type
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	eb.logger.Info("Event handler subscribed",
		zap.String("event_type", eventType),
		zap.Int("total_handlers", len(eb.handlers[eventType])))
}

// Publish sends an event to all registered handlers.
// Errors from handlers are logged but don't stop other handlers from executing.
func (eb *EventBus) Publish(ctx context.Context, event shared.DomainEvent) error {
	eb.mu.RLock()
	handlers, exists := eb.handlers[event.EventType()]
	eb.mu.RUnlock()
	
	if !exists || len(handlers) == 0 {
		// No handlers registered, that's fine
		return nil
	}
	
	// Execute handlers asynchronously to avoid blocking
	for _, handler := range handlers {
		if handler.CanHandle(event.EventType()) {
			// Run each handler in a goroutine for async processing
			go eb.executeHandler(ctx, handler, event)
		}
	}
	
	return nil
}

// PublishSync sends an event to all registered handlers synchronously.
// Use this when you need to ensure all handlers complete before continuing.
func (eb *EventBus) PublishSync(ctx context.Context, event shared.DomainEvent) error {
	eb.mu.RLock()
	handlers, exists := eb.handlers[event.EventType()]
	eb.mu.RUnlock()
	
	if !exists || len(handlers) == 0 {
		return nil
	}
	
	var wg sync.WaitGroup
	for _, handler := range handlers {
		if handler.CanHandle(event.EventType()) {
			wg.Add(1)
			handler := handler // Capture for goroutine
			go func() {
				defer wg.Done()
				eb.executeHandler(ctx, handler, event)
			}()
		}
	}
	
	wg.Wait()
	return nil
}

// executeHandler runs a single handler with error logging
func (eb *EventBus) executeHandler(ctx context.Context, handler EventHandler, event shared.DomainEvent) {
	if err := handler.Handle(ctx, event); err != nil {
		// Log error but don't fail - events are processed asynchronously
		eb.logger.Error("Event handler failed",
			zap.String("event_type", event.EventType()),
			zap.String("aggregate_id", event.AggregateID()),
			zap.Error(err))
	}
}

// GetHandlerCount returns the number of handlers for a given event type
func (eb *EventBus) GetHandlerCount(eventType string) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	return len(eb.handlers[eventType])
}

// Clear removes all registered handlers (useful for testing)
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.handlers = make(map[string][]EventHandler)
}