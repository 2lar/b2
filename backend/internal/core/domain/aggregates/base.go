// Package aggregates contains the domain aggregate roots implementing event sourcing.
// Aggregates are the transaction boundaries in Domain-Driven Design and maintain
// consistency through domain events.
package aggregates

import (
	"fmt"
	"time"
	
	"brain2-backend/internal/core/domain/events"
)

// AggregateRoot is the base interface for all aggregate roots in the domain.
// It provides event sourcing capabilities and version management for optimistic locking.
type AggregateRoot interface {
	// GetID returns the unique identifier of the aggregate
	GetID() string
	
	// GetVersion returns the current version for optimistic locking
	GetVersion() int64
	
	// GetUncommittedEvents returns events that haven't been persisted yet
	GetUncommittedEvents() []events.DomainEvent
	
	// MarkEventsAsCommitted clears the uncommitted events after persistence
	MarkEventsAsCommitted()
	
	// LoadFromHistory rebuilds the aggregate state from historical events
	LoadFromHistory(history []events.DomainEvent) error
	
	// Apply applies an event to update the aggregate state
	Apply(event events.DomainEvent) error
	
	// GetCreatedAt returns when the aggregate was created
	GetCreatedAt() time.Time
	
	// GetUpdatedAt returns when the aggregate was last updated
	GetUpdatedAt() time.Time
}

// BaseAggregate provides common functionality for all aggregates.
// It implements the event sourcing pattern with uncommitted events tracking.
type BaseAggregate struct {
	// ID is the unique identifier of the aggregate
	ID string
	
	// Version is used for optimistic locking
	Version int64
	
	// CreatedAt timestamp
	CreatedAt time.Time
	
	// UpdatedAt timestamp  
	UpdatedAt time.Time
	
	// uncommittedEvents holds events that haven't been persisted
	uncommittedEvents []events.DomainEvent
	
	// appliedEvents holds all events that have been applied
	appliedEvents []events.DomainEvent
}

// NewBaseAggregate creates a new base aggregate with the given ID
func NewBaseAggregate(id string) *BaseAggregate {
	return &BaseAggregate{
		ID:                id,
		Version:           0,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		uncommittedEvents: make([]events.DomainEvent, 0),
		appliedEvents:     make([]events.DomainEvent, 0),
	}
}

// GetID returns the aggregate ID
func (a *BaseAggregate) GetID() string {
	return a.ID
}

// GetVersion returns the current version
func (a *BaseAggregate) GetVersion() int64 {
	return a.Version
}

// GetCreatedAt returns the creation timestamp
func (a *BaseAggregate) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *BaseAggregate) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// GetUncommittedEvents returns events that haven't been persisted
func (a *BaseAggregate) GetUncommittedEvents() []events.DomainEvent {
	return a.uncommittedEvents
}

// MarkEventsAsCommitted clears uncommitted events after successful persistence
func (a *BaseAggregate) MarkEventsAsCommitted() {
	a.appliedEvents = append(a.appliedEvents, a.uncommittedEvents...)
	a.uncommittedEvents = []events.DomainEvent{}
}

// AddEvent adds a new event to the uncommitted list and applies it
func (a *BaseAggregate) AddEvent(event events.DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
	a.Version++
	a.UpdatedAt = event.GetTimestamp()
}

// addUncommittedEvent is an internal method to add uncommitted events
func (a *BaseAggregate) addUncommittedEvent(event events.DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// LoadFromHistory rebuilds aggregate state from historical events
func (a *BaseAggregate) LoadFromHistory(history []events.DomainEvent) error {
	for _, event := range history {
		if err := a.applyEvent(event); err != nil {
			return fmt.Errorf("failed to apply historical event: %w", err)
		}
		a.Version++
	}
	a.appliedEvents = history
	return nil
}

// applyEvent applies an event without adding it to uncommitted events
func (a *BaseAggregate) applyEvent(event events.DomainEvent) error {
	a.UpdatedAt = event.GetTimestamp()
	return nil
}

// Apply is meant to be overridden by concrete aggregates
func (a *BaseAggregate) Apply(event events.DomainEvent) error {
	return a.applyEvent(event)
}

// GetAppliedEvents returns all events that have been applied to this aggregate
func (a *BaseAggregate) GetAppliedEvents() []events.DomainEvent {
	return a.appliedEvents
}

// ClearEvents clears both committed and uncommitted events
// Useful for testing or when creating snapshots
func (a *BaseAggregate) ClearEvents() {
	a.uncommittedEvents = []events.DomainEvent{}
	a.appliedEvents = []events.DomainEvent{}
}

// HasUncommittedEvents checks if there are any uncommitted events
func (a *BaseAggregate) HasUncommittedEvents() bool {
	return len(a.uncommittedEvents) > 0
}

// GetEventCount returns the total number of events (committed + uncommitted)
func (a *BaseAggregate) GetEventCount() int {
	return len(a.appliedEvents) + len(a.uncommittedEvents)
}

// Snapshot represents a point-in-time state of an aggregate
type Snapshot struct {
	AggregateID string
	Version     int64
	Data        interface{}
	Timestamp   time.Time
}

// SnapshotCapable is implemented by aggregates that support snapshots
type SnapshotCapable interface {
	AggregateRoot
	
	// CreateSnapshot creates a snapshot of the current state
	CreateSnapshot() (*Snapshot, error)
	
	// RestoreFromSnapshot restores the aggregate state from a snapshot
	RestoreFromSnapshot(snapshot *Snapshot) error
}

// EventSourced provides a standard way to track which events an aggregate handles
type EventSourced interface {
	// When handles a specific event type and updates aggregate state
	When(event events.DomainEvent) error
}