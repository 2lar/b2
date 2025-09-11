package events

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"backend/domain/events"
	"go.uber.org/zap"
)

// EventHandler is the interface that all event handlers must implement
type EventHandler interface {
	// Handle processes a domain event
	Handle(ctx context.Context, event events.DomainEvent) error
	
	// SupportsEvent checks if this handler supports the given event type
	SupportsEvent(eventType string) bool
	
	// Priority returns the handler's priority (lower numbers = higher priority)
	Priority() int
	
	// Name returns the handler's name for logging
	Name() string
}

// HandlerRegistry manages event handler registration and dispatching
type HandlerRegistry struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewHandlerRegistry creates a new event handler registry
func NewHandlerRegistry(logger *zap.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string][]EventHandler),
		logger:   logger,
	}
}

// Register adds a handler for specific event types
func (r *HandlerRegistry) Register(eventTypes []string, handler EventHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	for _, eventType := range eventTypes {
		if eventType == "" {
			return fmt.Errorf("event type cannot be empty")
		}

		// Check if handler supports the event type
		if !handler.SupportsEvent(eventType) {
			return fmt.Errorf("handler %s does not support event type %s", handler.Name(), eventType)
		}

		// Add handler to the list for this event type
		r.handlers[eventType] = append(r.handlers[eventType], handler)
		
		// Sort handlers by priority
		r.sortHandlersByPriority(eventType)
		
		r.logger.Info("Registered event handler",
			zap.String("handler", handler.Name()),
			zap.String("eventType", eventType),
			zap.Int("priority", handler.Priority()),
		)
	}

	return nil
}

// Unregister removes a handler for specific event types
func (r *HandlerRegistry) Unregister(eventTypes []string, handler EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, eventType := range eventTypes {
		handlers := r.handlers[eventType]
		filtered := []EventHandler{}
		
		for _, h := range handlers {
			if h != handler {
				filtered = append(filtered, h)
			}
		}
		
		if len(filtered) > 0 {
			r.handlers[eventType] = filtered
		} else {
			delete(r.handlers, eventType)
		}
		
		r.logger.Info("Unregistered event handler",
			zap.String("handler", handler.Name()),
			zap.String("eventType", eventType),
		)
	}
}

// Dispatch sends an event to all registered handlers
func (r *HandlerRegistry) Dispatch(ctx context.Context, event events.DomainEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	eventType := r.getEventType(event)
	
	r.mu.RLock()
	handlers := r.handlers[eventType]
	// Make a copy to avoid holding the lock during handler execution
	handlersCopy := make([]EventHandler, len(handlers))
	copy(handlersCopy, handlers)
	r.mu.RUnlock()

	if len(handlersCopy) == 0 {
		r.logger.Debug("No handlers registered for event type",
			zap.String("eventType", eventType),
		)
		return nil
	}

	var lastError error
	successCount := 0
	failureCount := 0

	for _, handler := range handlersCopy {
		start := time.Now()
		
		// Execute handler with timeout
		handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := handler.Handle(handlerCtx, event)
		cancel()
		
		duration := time.Since(start)
		
		if err != nil {
			failureCount++
			lastError = err
			r.logger.Error("Event handler failed",
				zap.String("handler", handler.Name()),
				zap.String("eventType", eventType),
				zap.Error(err),
				zap.Duration("duration", duration),
			)
		} else {
			successCount++
			r.logger.Debug("Event handler succeeded",
				zap.String("handler", handler.Name()),
				zap.String("eventType", eventType),
				zap.Duration("duration", duration),
			)
		}
	}

	r.logger.Info("Event dispatched",
		zap.String("eventType", eventType),
		zap.Int("handlers", len(handlersCopy)),
		zap.Int("succeeded", successCount),
		zap.Int("failed", failureCount),
	)

	// Return error if all handlers failed
	if failureCount > 0 && successCount == 0 {
		return fmt.Errorf("all handlers failed for event %s: %w", eventType, lastError)
	}

	return nil
}

// DispatchBatch sends multiple events to handlers
func (r *HandlerRegistry) DispatchBatch(ctx context.Context, events []events.DomainEvent) error {
	var lastError error
	successCount := 0
	failureCount := 0

	for _, event := range events {
		if err := r.Dispatch(ctx, event); err != nil {
			failureCount++
			lastError = err
		} else {
			successCount++
		}
	}

	if failureCount > 0 {
		r.logger.Warn("Batch dispatch completed with errors",
			zap.Int("total", len(events)),
			zap.Int("succeeded", successCount),
			zap.Int("failed", failureCount),
		)
		return fmt.Errorf("batch dispatch had %d failures: %w", failureCount, lastError)
	}

	return nil
}

// GetHandlers returns all handlers for a specific event type
func (r *HandlerRegistry) GetHandlers(eventType string) []EventHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	handlers := r.handlers[eventType]
	result := make([]EventHandler, len(handlers))
	copy(result, handlers)
	
	return result
}

// GetAllHandlers returns all registered handlers
func (r *HandlerRegistry) GetAllHandlers() map[string][]EventHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make(map[string][]EventHandler)
	for eventType, handlers := range r.handlers {
		handlersCopy := make([]EventHandler, len(handlers))
		copy(handlersCopy, handlers)
		result[eventType] = handlersCopy
	}
	
	return result
}

// Clear removes all registered handlers
func (r *HandlerRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.handlers = make(map[string][]EventHandler)
	r.logger.Info("Cleared all event handlers")
}

// Private helper methods

func (r *HandlerRegistry) getEventType(event events.DomainEvent) string {
	// Get the type name of the event
	eventType := reflect.TypeOf(event).Name()
	if eventType == "" {
		// Handle pointer types
		eventType = reflect.TypeOf(event).Elem().Name()
	}
	return eventType
}

func (r *HandlerRegistry) sortHandlersByPriority(eventType string) {
	handlers := r.handlers[eventType]
	
	// Simple bubble sort for small lists
	n := len(handlers)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if handlers[j].Priority() > handlers[j+1].Priority() {
				handlers[j], handlers[j+1] = handlers[j+1], handlers[j]
			}
		}
	}
}

// HandlerStats provides statistics about registered handlers
type HandlerStats struct {
	TotalHandlers     int            `json:"total_handlers"`
	EventTypes        int            `json:"event_types"`
	HandlersPerEvent  map[string]int `json:"handlers_per_event"`
	AveragePriority   float64        `json:"average_priority"`
}

// GetStats returns statistics about registered handlers
func (r *HandlerRegistry) GetStats() HandlerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := HandlerStats{
		HandlersPerEvent: make(map[string]int),
	}
	
	totalPriority := 0
	totalHandlers := 0
	
	for eventType, handlers := range r.handlers {
		stats.HandlersPerEvent[eventType] = len(handlers)
		stats.EventTypes++
		totalHandlers += len(handlers)
		
		for _, handler := range handlers {
			totalPriority += handler.Priority()
		}
	}
	
	stats.TotalHandlers = totalHandlers
	if totalHandlers > 0 {
		stats.AveragePriority = float64(totalPriority) / float64(totalHandlers)
	}
	
	return stats
}

// BaseEventHandler provides a base implementation for event handlers
type BaseEventHandler struct {
	name         string
	priority     int
	supportedTypes []string
}

// NewBaseEventHandler creates a new base event handler
func NewBaseEventHandler(name string, priority int, supportedTypes []string) BaseEventHandler {
	return BaseEventHandler{
		name:         name,
		priority:     priority,
		supportedTypes: supportedTypes,
	}
}

// Name returns the handler's name
func (h BaseEventHandler) Name() string {
	return h.name
}

// Priority returns the handler's priority
func (h BaseEventHandler) Priority() int {
	return h.priority
}

// SupportsEvent checks if this handler supports the given event type
func (h BaseEventHandler) SupportsEvent(eventType string) bool {
	for _, supported := range h.supportedTypes {
		if supported == eventType || supported == "*" {
			return true
		}
	}
	return false
}