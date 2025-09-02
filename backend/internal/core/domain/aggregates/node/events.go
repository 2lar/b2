// Package node contains domain events for the Node aggregate
package node

import (
	"encoding/json"
	"fmt"
	
	"brain2-backend/internal/core/domain/events"
)

const (
	// Event type constants
	EventTypeNodeCreated      = "NodeCreated"
	EventTypeNodeUpdated      = "NodeUpdated"
	EventTypeNodeArchived     = "NodeArchived"
	EventTypeNodeRestored     = "NodeRestored"
	EventTypeNodeTagged       = "NodeTagged"
	EventTypeNodeCategorized  = "NodeCategorized"
	EventTypeNodeConnected    = "NodeConnected"
	EventTypeNodeDisconnected = "NodeDisconnected"
	
	// Aggregate type
	AggregateTypeNode = "Node"
)

// NodeCreatedEvent is raised when a new node is created
type NodeCreatedEvent struct {
	*events.BaseEvent
	UserID   string   `json:"user_id"`
	Content  string   `json:"content"`
	Title    string   `json:"title"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
}

// NewNodeCreatedEvent creates a new NodeCreatedEvent
func NewNodeCreatedEvent(nodeID, userID, content, title string, keywords, tags []string, version int64) *NodeCreatedEvent {
	return &NodeCreatedEvent{
		BaseEvent: events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeCreated, version),
		UserID:    userID,
		Content:   content,
		Title:     title,
		Keywords:  keywords,
		Tags:      tags,
	}
}

// Marshal serializes the event
func (e *NodeCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeUpdatedEvent is raised when a node's content is updated
type NodeUpdatedEvent struct {
	*events.BaseEvent
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
	OldTitle   string `json:"old_title"`
	NewTitle   string `json:"new_title"`
}

// NewNodeUpdatedEvent creates a new NodeUpdatedEvent
func NewNodeUpdatedEvent(nodeID, oldContent, newContent, oldTitle, newTitle string, version int64) *NodeUpdatedEvent {
	return &NodeUpdatedEvent{
		BaseEvent:  events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeUpdated, version),
		OldContent: oldContent,
		NewContent: newContent,
		OldTitle:   oldTitle,
		NewTitle:   newTitle,
	}
}

// Marshal serializes the event
func (e *NodeUpdatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeArchivedEvent is raised when a node is archived
type NodeArchivedEvent struct {
	*events.BaseEvent
	Reason string `json:"reason,omitempty"`
}

// NewNodeArchivedEvent creates a new NodeArchivedEvent
func NewNodeArchivedEvent(nodeID string, version int64) *NodeArchivedEvent {
	return &NodeArchivedEvent{
		BaseEvent: events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeArchived, version),
	}
}

// Marshal serializes the event
func (e *NodeArchivedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeRestoredEvent is raised when an archived node is restored
type NodeRestoredEvent struct {
	*events.BaseEvent
}

// NewNodeRestoredEvent creates a new NodeRestoredEvent
func NewNodeRestoredEvent(nodeID string, version int64) *NodeRestoredEvent {
	return &NodeRestoredEvent{
		BaseEvent: events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeRestored, version),
	}
}

// Marshal serializes the event
func (e *NodeRestoredEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeTaggedEvent is raised when tags are added to a node
type NodeTaggedEvent struct {
	*events.BaseEvent
	Tags []string `json:"tags"`
}

// NewNodeTaggedEvent creates a new NodeTaggedEvent
func NewNodeTaggedEvent(nodeID string, tags []string, version int64) *NodeTaggedEvent {
	return &NodeTaggedEvent{
		BaseEvent: events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeTagged, version),
		Tags:      tags,
	}
}

// Marshal serializes the event
func (e *NodeTaggedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeCategorizedEvent is raised when a node is added to a category
type NodeCategorizedEvent struct {
	*events.BaseEvent
	CategoryID string `json:"category_id"`
}

// NewNodeCategorizedEvent creates a new NodeCategorizedEvent
func NewNodeCategorizedEvent(nodeID, categoryID string, version int64) *NodeCategorizedEvent {
	return &NodeCategorizedEvent{
		BaseEvent:  events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeCategorized, version),
		CategoryID: categoryID,
	}
}

// Marshal serializes the event
func (e *NodeCategorizedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeConnectedEvent is raised when a node is connected to another
type NodeConnectedEvent struct {
	*events.BaseEvent
	TargetNodeID string  `json:"target_node_id"`
	Strength     float64 `json:"strength"`
}

// NewNodeConnectedEvent creates a new NodeConnectedEvent
func NewNodeConnectedEvent(nodeID, targetNodeID string, strength float64, version int64) *NodeConnectedEvent {
	return &NodeConnectedEvent{
		BaseEvent:    events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeConnected, version),
		TargetNodeID: targetNodeID,
		Strength:     strength,
	}
}

// Marshal serializes the event
func (e *NodeConnectedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// NodeDisconnectedEvent is raised when a node connection is removed
type NodeDisconnectedEvent struct {
	*events.BaseEvent
	TargetNodeID string `json:"target_node_id"`
}

// NewNodeDisconnectedEvent creates a new NodeDisconnectedEvent
func NewNodeDisconnectedEvent(nodeID, targetNodeID string, version int64) *NodeDisconnectedEvent {
	return &NodeDisconnectedEvent{
		BaseEvent:    events.NewBaseEvent(nodeID, AggregateTypeNode, EventTypeNodeDisconnected, version),
		TargetNodeID: targetNodeID,
	}
}

// Marshal serializes the event
func (e *NodeDisconnectedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// EventFactory creates events from raw data (for deserialization)
type EventFactory struct{}

// CreateEvent creates the appropriate event type from raw data
func (f *EventFactory) CreateEvent(eventType string, data []byte) (events.DomainEvent, error) {
	var event events.DomainEvent
	
	switch eventType {
	case EventTypeNodeCreated:
		event = &NodeCreatedEvent{}
	case EventTypeNodeUpdated:
		event = &NodeUpdatedEvent{}
	case EventTypeNodeArchived:
		event = &NodeArchivedEvent{}
	case EventTypeNodeRestored:
		event = &NodeRestoredEvent{}
	case EventTypeNodeTagged:
		event = &NodeTaggedEvent{}
	case EventTypeNodeCategorized:
		event = &NodeCategorizedEvent{}
	case EventTypeNodeConnected:
		event = &NodeConnectedEvent{}
	case EventTypeNodeDisconnected:
		event = &NodeDisconnectedEvent{}
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
	
	if err := json.Unmarshal(data, event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}
	
	return event, nil
}