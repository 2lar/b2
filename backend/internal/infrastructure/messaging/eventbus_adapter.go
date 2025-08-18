// Package messaging provides event publishing infrastructure adapters.
package messaging

import (
	"context"
	"log"

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
	// Log event publishing for debugging
	log.Printf("DEBUG: EventBusAdapter.Publish - Type: %s, AggregateID: %s, EventID: %s", 
		event.EventType(), event.AggregateID(), event.EventID())
	
	// EventPublisher expects an array of events
	events := []shared.DomainEvent{event}
	err := a.publisher.Publish(ctx, events)
	
	if err != nil {
		log.Printf("ERROR: EventBusAdapter failed to publish event: %v", err)
	} else {
		log.Printf("DEBUG: EventBusAdapter successfully published event %s", event.EventID())
	}
	
	return err
}