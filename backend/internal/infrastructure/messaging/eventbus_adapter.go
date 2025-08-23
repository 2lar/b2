// Package messaging provides event publishing infrastructure adapters.
package messaging

import (
	"context"

	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// EventBusAdapter adapts the repository.EventPublisher interface to the shared.EventBus interface.
// This allows the EventBridgePublisher to be used where shared.EventBus is expected.
type EventBusAdapter struct {
	publisher repository.EventPublisher
}

// NewEventBusAdapter creates a new adapter that wraps an EventPublisher.
func NewEventBusAdapter(publisher repository.EventPublisher) shared.EventBus {
	return &EventBusAdapter{
		publisher: publisher,
	}
}

// Publish implements the shared.EventBus interface by delegating to the EventPublisher.
// It converts the single event to an array as expected by EventPublisher.
func (a *EventBusAdapter) Publish(ctx context.Context, event shared.DomainEvent) error {
	// Publishing event to underlying publisher
	
	// EventPublisher expects an array of events
	events := []shared.DomainEvent{event}
	err := a.publisher.Publish(ctx, events)
	
	// Error handling is done at caller level
	
	return err
}