package domain

import "context"

// EventBus defines the interface for publishing domain events.
// This interface allows the domain layer to publish events without depending on infrastructure.
type EventBus interface {
	// Publish publishes a domain event to all registered subscribers
	Publish(ctx context.Context, event DomainEvent) error
}

// MockEventBus provides a simple in-memory event bus for testing and development.
type MockEventBus struct {
	events []DomainEvent
}

// NewMockEventBus creates a new mock event bus.
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]DomainEvent, 0),
	}
}

// Publish stores the event in memory (for testing/development).
func (b *MockEventBus) Publish(ctx context.Context, event DomainEvent) error {
	b.events = append(b.events, event)
	return nil
}

// GetEvents returns all published events (for testing).
func (b *MockEventBus) GetEvents() []DomainEvent {
	return b.events
}

// Clear removes all stored events (for testing).
func (b *MockEventBus) Clear() {
	b.events = make([]DomainEvent, 0)
}