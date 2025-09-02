// Package events defines domain events for the event sourcing system.
// Domain events capture important business occurrences and are the source of truth
// for aggregate state changes in an event-sourced architecture.
package events

import (
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/google/uuid"
)

// DomainEvent is the base interface for all domain events.
// Events are immutable facts that have happened in the past.
type DomainEvent interface {
	// GetEventID returns the unique identifier of the event
	GetEventID() string
	
	// GetEventType returns the type name of the event
	GetEventType() string
	
	// GetAggregateID returns the ID of the aggregate this event belongs to
	GetAggregateID() string
	
	// GetAggregateType returns the type of the aggregate
	GetAggregateType() string
	
	// GetTimestamp returns when the event occurred
	GetTimestamp() time.Time
	
	// GetVersion returns the version of the aggregate after this event
	GetVersion() int64
	
	// GetMetadata returns event metadata (user, correlation ID, etc.)
	GetMetadata() EventMetadata
	
	// Marshal serializes the event for storage
	Marshal() ([]byte, error)
}

// EventMetadata contains contextual information about an event
type EventMetadata struct {
	// CorrelationID links related events across aggregates
	CorrelationID string `json:"correlation_id"`
	
	// CausationID is the ID of the event that caused this event
	CausationID string `json:"causation_id"`
	
	// UserID is the user who triggered the event
	UserID string `json:"user_id"`
	
	// IPAddress of the request origin
	IPAddress string `json:"ip_address,omitempty"`
	
	// UserAgent of the client
	UserAgent string `json:"user_agent,omitempty"`
	
	// Custom allows for additional metadata
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// BaseEvent provides common functionality for all domain events
type BaseEvent struct {
	EventID       string        `json:"event_id"`
	EventType     string        `json:"event_type"`
	AggregateID   string        `json:"aggregate_id"`
	AggregateType string        `json:"aggregate_type"`
	Timestamp     time.Time     `json:"timestamp"`
	Version       int64         `json:"version"`
	Metadata      EventMetadata `json:"metadata"`
}

// NewBaseEvent creates a new base event with generated ID and timestamp
func NewBaseEvent(aggregateID, aggregateType, eventType string, version int64) *BaseEvent {
	return &BaseEvent{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Timestamp:     time.Now().UTC(),
		Version:       version,
		Metadata:      EventMetadata{},
	}
}

// GetEventID returns the event ID
func (e *BaseEvent) GetEventID() string {
	return e.EventID
}

// GetEventType returns the event type
func (e *BaseEvent) GetEventType() string {
	return e.EventType
}

// GetAggregateID returns the aggregate ID
func (e *BaseEvent) GetAggregateID() string {
	return e.AggregateID
}

// GetAggregateType returns the aggregate type
func (e *BaseEvent) GetAggregateType() string {
	return e.AggregateType
}

// GetTimestamp returns when the event occurred
func (e *BaseEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetVersion returns the aggregate version
func (e *BaseEvent) GetVersion() int64 {
	return e.Version
}

// GetMetadata returns the event metadata
func (e *BaseEvent) GetMetadata() EventMetadata {
	return e.Metadata
}

// WithMetadata sets the event metadata
func (e *BaseEvent) WithMetadata(metadata EventMetadata) *BaseEvent {
	e.Metadata = metadata
	return e
}

// Marshal serializes the event to JSON
func (e *BaseEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// EventStore defines the interface for persisting and retrieving events
type EventStore interface {
	// SaveEvents persists new events for an aggregate
	SaveEvents(aggregateID string, events []DomainEvent, expectedVersion int64) error
	
	// GetEvents retrieves all events for an aggregate
	GetEvents(aggregateID string) ([]DomainEvent, error)
	
	// GetEventsAfterVersion retrieves events after a specific version
	GetEventsAfterVersion(aggregateID string, version int64) ([]DomainEvent, error)
	
	// GetEventsByType retrieves events of a specific type
	GetEventsByType(eventType string, limit int) ([]DomainEvent, error)
	
	// GetEventStream creates a stream of events for real-time processing
	GetEventStream(fromPosition int64) (<-chan DomainEvent, error)
}

// EventBus publishes events to interested subscribers
type EventBus interface {
	// Publish sends an event to all subscribers
	Publish(event DomainEvent) error
	
	// PublishBatch sends multiple events
	PublishBatch(events []DomainEvent) error
	
	// Subscribe registers a handler for specific event types
	Subscribe(eventType string, handler EventHandler) error
	
	// SubscribeAll registers a handler for all events
	SubscribeAll(handler EventHandler) error
	
	// Unsubscribe removes a handler
	Unsubscribe(eventType string, handler EventHandler) error
}

// EventHandler processes domain events
type EventHandler interface {
	// Handle processes an event
	Handle(event DomainEvent) error
	
	// HandlerName returns the name of the handler for logging
	HandlerName() string
}

// EventHandlerFunc is a function adapter for EventHandler
type EventHandlerFunc func(DomainEvent) error

// Handle calls the function
func (f EventHandlerFunc) Handle(event DomainEvent) error {
	return f(event)
}

// HandlerName returns a generic name
func (f EventHandlerFunc) HandlerName() string {
	return "EventHandlerFunc"
}

// EventSerializer handles event serialization/deserialization
type EventSerializer interface {
	// Serialize converts an event to bytes
	Serialize(event DomainEvent) ([]byte, error)
	
	// Deserialize converts bytes back to an event
	Deserialize(data []byte, eventType string) (DomainEvent, error)
	
	// RegisterEventType registers a type for deserialization
	RegisterEventType(eventType string, factory func() DomainEvent)
}

// EventUpgrader handles event schema migrations
type EventUpgrader interface {
	// CanUpgrade checks if this upgrader can handle the event
	CanUpgrade(eventType string, version int) bool
	
	// Upgrade transforms an old event to the current schema
	Upgrade(oldEvent json.RawMessage) (DomainEvent, error)
}

// EventProjector projects events to read models
type EventProjector interface {
	// Project applies an event to update read models
	Project(event DomainEvent) error
	
	// GetProjectionName returns the name of this projection
	GetProjectionName() string
	
	// Reset clears and rebuilds the projection from events
	Reset() error
}

// EventSourcingError represents errors in the event sourcing system
type EventSourcingError struct {
	Type    string
	Message string
	Cause   error
}

// Error implements the error interface
func (e *EventSourcingError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewEventSourcingError creates a new event sourcing error
func NewEventSourcingError(errType, message string, cause error) *EventSourcingError {
	return &EventSourcingError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// Common error types
var (
	ErrConcurrencyConflict = NewEventSourcingError("CONCURRENCY_CONFLICT", "Version mismatch detected", nil)
	ErrEventNotFound       = NewEventSourcingError("EVENT_NOT_FOUND", "Event not found", nil)
	ErrInvalidEvent        = NewEventSourcingError("INVALID_EVENT", "Event validation failed", nil)
	ErrSerializationFailed = NewEventSourcingError("SERIALIZATION_FAILED", "Failed to serialize event", nil)
)