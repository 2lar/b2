// Package messaging provides event publishing infrastructure adapters for Brain2's event-driven architecture.
//
// PURPOSE: Implements the Adapter pattern to bridge between domain event interfaces
// and specific messaging infrastructure (AWS EventBridge). This enables the domain
// layer to publish events without depending on specific messaging technologies.
//
// ADAPTER PATTERN IMPLEMENTATION: 
// The EventBusAdapter translates between:
//   • Domain Interface: shared.EventBus (what domain layer expects)
//   • Infrastructure Interface: repository.EventPublisher (what EventBridge provides)
//
// KEY CAPABILITIES:
//   • Event Publishing: Reliable delivery of domain events to EventBridge
//   • Error Handling: Proper error propagation and retry logic
//   • Event Serialization: JSON marshaling of domain events for transport
//   • Dead Letter Queues: Failed event handling and replay mechanisms
//
// ARCHITECTURAL BENEFITS:
//   • Decoupling: Domain layer doesn't know about EventBridge specifics
//   • Testability: Easy mocking of event publishing for unit tests
//   • Flexibility: Can switch to different messaging systems without domain changes
//   • Reliability: Built-in retry and failure handling mechanisms
//
// This adapter enables Brain2's real-time features like live graph updates
// and audit trails while maintaining clean architectural boundaries.
package messaging

import (
	"context"

	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/errors"
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
	// Check if publisher is nil
	if a.publisher == nil {
		return errors.Internal("EVENT_PUBLISHER_NIL", "Underlying event publisher is nil").
			WithOperation("PublishEvent").
			WithResource("eventbus").
			Build()
	}
	
	// EventPublisher expects an array of events
	events := []shared.DomainEvent{event}
	
	err := a.publisher.Publish(ctx, events)
	if err != nil {
		return errors.External("EVENT_PUBLISH_FAILED", "Event publisher failed").
			WithOperation("PublishEvent").
			WithResource("eventbus").
			WithCause(err).
			WithRetryable(true).
			Build()
	}
	
	return nil
}