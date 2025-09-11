package messaging

import (
	"context"
	"fmt"
	"time"

	appevents "backend/application/events"
	"backend/domain/events"
	"go.uber.org/zap"
)

// EventDispatcher bridges between external event publishing and local event handling
// It ensures that events published to EventBridge are also handled locally
type EventDispatcher struct {
	registry *appevents.HandlerRegistry
	logger   *zap.Logger
}

// NewEventDispatcher creates a new event dispatcher
func NewEventDispatcher(registry *appevents.HandlerRegistry, logger *zap.Logger) *EventDispatcher {
	return &EventDispatcher{
		registry: registry,
		logger:   logger,
	}
}

// DispatchLocal dispatches events to local handlers
func (d *EventDispatcher) DispatchLocal(ctx context.Context, event events.DomainEvent) error {
	if d.registry == nil {
		d.logger.Debug("No event handler registry configured, skipping local dispatch")
		return nil
	}

	startTime := time.Now()
	
	// Dispatch to local handlers
	err := d.registry.Dispatch(ctx, event)
	
	duration := time.Since(startTime)
	
	if err != nil {
		d.logger.Error("Failed to dispatch event locally",
			zap.String("eventType", event.GetEventType()),
			zap.String("aggregateID", event.GetAggregateID()),
			zap.Error(err),
			zap.Duration("duration", duration))
		// Don't fail the whole operation if local dispatch fails
		// This ensures external systems still get the event
		return nil
	}
	
	d.logger.Debug("Event dispatched locally",
		zap.String("eventType", event.GetEventType()),
		zap.String("aggregateID", event.GetAggregateID()),
		zap.Duration("duration", duration))
	
	return nil
}

// DispatchBatchLocal dispatches multiple events to local handlers
func (d *EventDispatcher) DispatchBatchLocal(ctx context.Context, events []events.DomainEvent) error {
	if d.registry == nil {
		d.logger.Debug("No event handler registry configured, skipping local dispatch")
		return nil
	}

	if len(events) == 0 {
		return nil
	}

	startTime := time.Now()
	successCount := 0
	failureCount := 0
	
	for _, event := range events {
		if err := d.registry.Dispatch(ctx, event); err != nil {
			failureCount++
			d.logger.Warn("Failed to dispatch event locally",
				zap.String("eventType", event.GetEventType()),
				zap.String("aggregateID", event.GetAggregateID()),
				zap.Error(err))
		} else {
			successCount++
		}
	}
	
	duration := time.Since(startTime)
	
	d.logger.Info("Batch events dispatched locally",
		zap.Int("total", len(events)),
		zap.Int("success", successCount),
		zap.Int("failed", failureCount),
		zap.Duration("duration", duration))
	
	if failureCount > 0 {
		return fmt.Errorf("failed to dispatch %d of %d events locally", failureCount, len(events))
	}
	
	return nil
}