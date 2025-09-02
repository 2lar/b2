// Package builders provides test builders for creating domain objects in tests
package builders

import (
	"time"

	"brain2-backend/internal/core/domain/events"
	"github.com/google/uuid"
)

// EventBuilder provides a fluent interface for building test events
type EventBuilder struct {
	eventType     string
	aggregateID   string
	version       int64
	timestamp     time.Time
	userID        string
	correlationID string
	causationID   string
	source        string
	data          map[string]interface{}
}

// NewEventBuilder creates a new event builder with sensible defaults
func NewEventBuilder() *EventBuilder {
	return &EventBuilder{
		eventType:     "TestEvent",
		aggregateID:   uuid.New().String(),
		version:       1,
		timestamp:     time.Now(),
		userID:        "test-user-" + uuid.New().String(),
		correlationID: uuid.New().String(),
		causationID:   uuid.New().String(),
		source:        "test",
		data:          make(map[string]interface{}),
	}
}

// WithType sets the event type
func (b *EventBuilder) WithType(eventType string) *EventBuilder {
	b.eventType = eventType
	return b
}

// WithAggregateID sets the aggregate ID
func (b *EventBuilder) WithAggregateID(id string) *EventBuilder {
	b.aggregateID = id
	return b
}

// WithVersion sets the event version
func (b *EventBuilder) WithVersion(version int64) *EventBuilder {
	b.version = version
	return b
}

// WithTimestamp sets the event timestamp
func (b *EventBuilder) WithTimestamp(t time.Time) *EventBuilder {
	b.timestamp = t
	return b
}

// WithUserID sets the user ID in metadata
func (b *EventBuilder) WithUserID(userID string) *EventBuilder {
	b.userID = userID
	return b
}

// WithCorrelationID sets the correlation ID
func (b *EventBuilder) WithCorrelationID(id string) *EventBuilder {
	b.correlationID = id
	return b
}

// WithCausationID sets the causation ID
func (b *EventBuilder) WithCausationID(id string) *EventBuilder {
	b.causationID = id
	return b
}

// WithSource sets the event source
func (b *EventBuilder) WithSource(source string) *EventBuilder {
	b.source = source
	return b
}

// WithData adds data fields to the event
func (b *EventBuilder) WithData(key string, value interface{}) *EventBuilder {
	b.data[key] = value
	return b
}

// BuildNodeCreated creates a NodeCreatedEvent
func (b *EventBuilder) BuildNodeCreated(content, title string, tags []string) *events.NodeCreatedEvent {
	return &events.NodeCreatedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			Version:     b.version,
			Timestamp:   b.timestamp,
			Metadata: events.EventMetadata{
				UserID:        b.userID,
				CorrelationID: b.correlationID,
				CausationID:   b.causationID,
			},
		},
		UserID:   b.userID,
		Content:  content,
		Title:    title,
		Tags:     tags,
		Keywords: []string{},
		Metadata: b.data,
	}
}

// BuildNodeUpdated creates a NodeUpdatedEvent
func (b *EventBuilder) BuildNodeUpdated(content, title string, tags []string) *events.NodeUpdatedEvent {
	return &events.NodeUpdatedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			Version:     b.version,
			Timestamp:   b.timestamp,
			Metadata: events.EventMetadata{
				UserID:        b.userID,
				CorrelationID: b.correlationID,
				CausationID:   b.causationID,
			},
		},
		Content:  content,
		Title:    title,
		Tags:     tags,
		Keywords: []string{},
	}
}

// BuildNodeArchived creates a NodeArchivedEvent
func (b *EventBuilder) BuildNodeArchived(reason string) *events.NodeArchivedEvent {
	return &events.NodeArchivedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			Version:     b.version,
			Timestamp:   b.timestamp,
			Metadata: events.EventMetadata{
				UserID:        b.userID,
				CorrelationID: b.correlationID,
				CausationID:   b.causationID,
			},
		},
		Reason: reason,
	}
}

// BuildNodeConnected creates a NodeConnectedEvent
func (b *EventBuilder) BuildNodeConnected(targetNodeID string, weight float64) *events.NodeConnectedEvent {
	return &events.NodeConnectedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			Version:     b.version,
			Timestamp:   b.timestamp,
			Metadata: events.EventMetadata{
				UserID:        b.userID,
				CorrelationID: b.correlationID,
				CausationID:   b.causationID,
			},
		},
		TargetNodeID: targetNodeID,
		Weight:       weight,
		Metadata:     b.data,
	}
}

// BuildNodeDisconnected creates a NodeDisconnectedEvent
func (b *EventBuilder) BuildNodeDisconnected(targetNodeID string) *events.NodeDisconnectedEvent {
	return &events.NodeDisconnectedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			Version:     b.version,
			Timestamp:   b.timestamp,
			Metadata: events.EventMetadata{
				UserID:        b.userID,
				CorrelationID: b.correlationID,
				CausationID:   b.causationID,
			},
		},
		TargetNodeID: targetNodeID,
	}
}

// EventBuilderPresets provides common event configurations
type EventBuilderPresets struct{}

// NewEventBuilderPresets creates a new presets helper
func NewEventBuilderPresets() *EventBuilderPresets {
	return &EventBuilderPresets{}
}

// NodeCreatedEvent creates a standard node created event
func (p *EventBuilderPresets) NodeCreatedEvent(nodeID, userID string) *events.NodeCreatedEvent {
	return NewEventBuilder().
		WithAggregateID(nodeID).
		WithUserID(userID).
		BuildNodeCreated(
			"Test content",
			"Test Node",
			[]string{"test"},
		)
}

// NodeUpdatedEvent creates a standard node updated event
func (p *EventBuilderPresets) NodeUpdatedEvent(nodeID, userID string) *events.NodeUpdatedEvent {
	return NewEventBuilder().
		WithAggregateID(nodeID).
		WithUserID(userID).
		WithVersion(2).
		BuildNodeUpdated(
			"Updated content",
			"Updated Node",
			[]string{"updated"},
		)
}

// NodeArchivedEvent creates a standard node archived event
func (p *EventBuilderPresets) NodeArchivedEvent(nodeID, userID string) *events.NodeArchivedEvent {
	return NewEventBuilder().
		WithAggregateID(nodeID).
		WithUserID(userID).
		WithVersion(3).
		BuildNodeArchived("Test archive")
}

// EventSequence creates a sequence of events for a node lifecycle
func (p *EventBuilderPresets) EventSequence(nodeID, userID string) []events.DomainEvent {
	builder := NewEventBuilder().
		WithAggregateID(nodeID).
		WithUserID(userID)

	return []events.DomainEvent{
		builder.WithVersion(1).BuildNodeCreated("Initial content", "Initial Title", []string{"new"}),
		builder.WithVersion(2).BuildNodeUpdated("Modified content", "Modified Title", []string{"modified"}),
		builder.WithVersion(3).BuildNodeConnected("target-node-1", 0.7),
		builder.WithVersion(4).BuildNodeConnected("target-node-2", 0.5),
		builder.WithVersion(5).BuildNodeDisconnected("target-node-1"),
	}
}