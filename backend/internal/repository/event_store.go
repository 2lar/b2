package repository

import (
	"context"
	"time"

	"brain2-backend/internal/domain/shared"
)

// EventStore provides persistence for domain events, enabling event sourcing
// and audit logging capabilities.
type EventStore interface {
	// SaveEvents persists domain events atomically with the aggregate state
	SaveEvents(ctx context.Context, aggregateID string, events []shared.DomainEvent, expectedVersion int) error
	
	// GetEvents retrieves all events for an aggregate
	GetEvents(ctx context.Context, aggregateID string) ([]shared.DomainEvent, error)
	
	// GetEventsAfterVersion retrieves events after a specific version
	GetEventsAfterVersion(ctx context.Context, aggregateID string, version int) ([]shared.DomainEvent, error)
	
	// GetEventsByType retrieves events of a specific type
	GetEventsByType(ctx context.Context, eventType string, since time.Time) ([]shared.DomainEvent, error)
	
	// GetSnapshot retrieves the latest snapshot for an aggregate (optimization)
	GetSnapshot(ctx context.Context, aggregateID string) (*AggregateSnapshot, error)
	
	// SaveSnapshot saves a snapshot of aggregate state (optimization)
	SaveSnapshot(ctx context.Context, snapshot *AggregateSnapshot) error
}

// AggregateSnapshot represents a point-in-time state of an aggregate
// Used to optimize event replay by not having to replay all events from the beginning
type AggregateSnapshot struct {
	AggregateID   string
	AggregateType string
	Version       int
	Data          []byte // Serialized aggregate state
	CreatedAt     time.Time
}

// EventStream represents a stream of events for rebuilding aggregate state
type EventStream struct {
	AggregateID string
	Events      []shared.DomainEvent
	Version     int
}

// EventStoreRepository combines event store with regular repository operations
// This ensures events and state changes are persisted atomically
type EventStoreRepository interface {
	// SaveWithEvents saves the aggregate and its events in a single transaction
	SaveWithEvents(ctx context.Context, aggregate shared.AggregateRoot) error
	
	// LoadFromEvents rebuilds aggregate state from its event history
	LoadFromEvents(ctx context.Context, aggregateID string) (shared.AggregateRoot, error)
}