package projections

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/domain/events"
	"go.uber.org/zap"
)

// ProjectionRegistry manages all projection handlers and routes events to them.
// This is the core component that implements the event-to-projection routing in CQRS.
type ProjectionRegistry struct {
	mu             sync.RWMutex
	projections    map[string]ProjectionHandler
	eventMap       map[string][]ProjectionHandler // event type -> handlers
	checkpointStore CheckpointStore
	logger         *zap.Logger
	stats          map[string]*ProjectionStats
	errorHandler   func(error)
}

// NewProjectionRegistry creates a new projection registry
func NewProjectionRegistry(checkpointStore CheckpointStore, logger *zap.Logger) *ProjectionRegistry {
	return &ProjectionRegistry{
		projections:     make(map[string]ProjectionHandler),
		eventMap:        make(map[string][]ProjectionHandler),
		checkpointStore: checkpointStore,
		logger:          logger,
		stats:           make(map[string]*ProjectionStats),
		errorHandler:    func(err error) { logger.Error("Projection error", zap.Error(err)) },
	}
}

// Register adds a projection handler to the registry
func (r *ProjectionRegistry) Register(projection ProjectionHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	name := projection.GetProjectionName()
	if _, exists := r.projections[name]; exists {
		return fmt.Errorf("projection '%s' already registered", name)
	}
	
	// Register the projection
	r.projections[name] = projection
	
	// Initialize stats
	r.stats[name] = &ProjectionStats{
		ProjectionName: name,
	}
	
	// Map event types to this projection
	for _, eventType := range projection.GetEventTypes() {
		r.eventMap[eventType] = append(r.eventMap[eventType], projection)
	}
	
	r.logger.Info("Registered projection",
		zap.String("name", name),
		zap.Strings("eventTypes", projection.GetEventTypes()))
	
	return nil
}

// Dispatch routes an event to all interested projections
func (r *ProjectionRegistry) Dispatch(ctx context.Context, event events.DomainEvent) error {
	eventType := GetEventType(event)
	if eventType == "" {
		return fmt.Errorf("unable to determine event type")
	}
	
	r.mu.RLock()
	handlers, exists := r.eventMap[eventType]
	r.mu.RUnlock()
	
	if !exists || len(handlers) == 0 {
		r.logger.Debug("No projections registered for event type",
			zap.String("eventType", eventType))
		return nil
	}
	
	// Process event with all registered handlers
	var errs []error
	for _, handler := range handlers {
		if err := r.processEvent(ctx, handler, event, eventType); err != nil {
			errs = append(errs, err)
			r.errorHandler(err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("projection dispatch had %d errors", len(errs))
	}
	
	return nil
}

// processEvent handles a single event with a single projection
func (r *ProjectionRegistry) processEvent(ctx context.Context, handler ProjectionHandler, event events.DomainEvent, eventType string) error {
	projectionName := handler.GetProjectionName()
	startTime := time.Now()
	
	// Update stats
	defer func() {
		r.mu.Lock()
		stats := r.stats[projectionName]
		stats.EventsProcessed++
		stats.LastEventTime = time.Now().Unix()
		latency := time.Since(startTime).Milliseconds()
		// Simple moving average for latency
		stats.AverageLatencyMs = (stats.AverageLatencyMs*float64(stats.EventsProcessed-1) + float64(latency)) / float64(stats.EventsProcessed)
		r.mu.Unlock()
	}()
	
	// Handle the event
	if err := handler.Handle(ctx, event); err != nil {
		r.mu.Lock()
		r.stats[projectionName].ErrorCount++
		r.mu.Unlock()
		
		return &ProjectionError{
			ProjectionName: projectionName,
			EventID:        event.GetAggregateID(),
			EventType:      eventType,
			Err:            err,
		}
	}
	
	// Save checkpoint if checkpoint store is available
	if r.checkpointStore != nil {
		position := &ProjectionPosition{
			ProjectionName: projectionName,
			LastEventID:    event.GetAggregateID(),
			UpdatedAt:      time.Now().Unix(),
		}
		
		if err := r.checkpointStore.SavePosition(ctx, position); err != nil {
			r.logger.Warn("Failed to save projection checkpoint",
				zap.String("projection", projectionName),
				zap.String("eventID", event.GetAggregateID()),
				zap.Error(err))
		}
	}
	
	r.logger.Debug("Projection processed event",
		zap.String("projection", projectionName),
		zap.String("eventType", eventType),
		zap.String("eventID", event.GetAggregateID()))
	
	return nil
}

// ResetProjection resets a specific projection
func (r *ProjectionRegistry) ResetProjection(ctx context.Context, projectionName string) error {
	r.mu.RLock()
	projection, exists := r.projections[projectionName]
	r.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("projection '%s' not found", projectionName)
	}
	
	// Reset the projection
	if err := projection.Reset(ctx); err != nil {
		return fmt.Errorf("failed to reset projection '%s': %w", projectionName, err)
	}
	
	// Clear checkpoint
	if r.checkpointStore != nil {
		if err := r.checkpointStore.DeletePosition(ctx, projectionName); err != nil {
			r.logger.Warn("Failed to delete projection checkpoint",
				zap.String("projection", projectionName),
				zap.Error(err))
		}
	}
	
	// Reset stats
	r.mu.Lock()
	r.stats[projectionName] = &ProjectionStats{
		ProjectionName: projectionName,
	}
	r.mu.Unlock()
	
	r.logger.Info("Reset projection", zap.String("projection", projectionName))
	return nil
}

// ResetAll resets all registered projections
func (r *ProjectionRegistry) ResetAll(ctx context.Context) error {
	r.mu.RLock()
	projectionNames := make([]string, 0, len(r.projections))
	for name := range r.projections {
		projectionNames = append(projectionNames, name)
	}
	r.mu.RUnlock()
	
	var errs []error
	for _, name := range projectionNames {
		if err := r.ResetProjection(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("failed to reset %d projections", len(errs))
	}
	
	return nil
}

// GetStats returns statistics for all projections
func (r *ProjectionRegistry) GetStats() map[string]*ProjectionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	statsCopy := make(map[string]*ProjectionStats)
	for name, stats := range r.stats {
		statsCopy[name] = &ProjectionStats{
			ProjectionName:   stats.ProjectionName,
			EventsProcessed:  stats.EventsProcessed,
			LastEventTime:    stats.LastEventTime,
			AverageLatencyMs: stats.AverageLatencyMs,
			ErrorCount:       stats.ErrorCount,
		}
	}
	
	return statsCopy
}

// SetErrorHandler sets a custom error handler for projection errors
func (r *ProjectionRegistry) SetErrorHandler(handler func(error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorHandler = handler
}

// GetProjection retrieves a registered projection by name
func (r *ProjectionRegistry) GetProjection(name string) (ProjectionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	projection, exists := r.projections[name]
	return projection, exists
}