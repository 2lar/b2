// Package events defines domain events for the event sourcing system.
package events

import (
	"encoding/json"
	"time"
)

// NodeCreatedEvent is raised when a new node is created
type NodeCreatedEvent struct {
	BaseEvent
	UserID      string                 `json:"user_id"`
	Content     string                 `json:"content"`
	Title       string                 `json:"title"`
	Tags        []string               `json:"tags,omitempty"`
	Keywords    []string               `json:"keywords,omitempty"`
	CategoryIDs []string               `json:"category_ids,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewNodeCreatedEvent creates a new node created event
func NewNodeCreatedEvent(nodeID, userID, content, title string) *NodeCreatedEvent {
	return &NodeCreatedEvent{
		BaseEvent: *NewBaseEvent(nodeID, "Node", "NodeCreated", 1),
		UserID:    userID,
		Content:   content,
		Title:     title,
		Tags:      []string{},
		Keywords:  []string{},
		Metadata:  make(map[string]interface{}),
	}
}

// NodeUpdatedEvent is raised when a node is updated
type NodeUpdatedEvent struct {
	BaseEvent
	Content     string                 `json:"content,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Keywords    []string               `json:"keywords,omitempty"`
	CategoryIDs []string               `json:"category_ids,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewNodeUpdatedEvent creates a new node updated event
func NewNodeUpdatedEvent(nodeID string, version int64) *NodeUpdatedEvent {
	return &NodeUpdatedEvent{
		BaseEvent: *NewBaseEvent(nodeID, "Node", "NodeUpdated", version),
	}
}

// NodeArchivedEvent is raised when a node is archived
type NodeArchivedEvent struct {
	BaseEvent
	Reason      string    `json:"reason"`
	ArchivedBy  string    `json:"archived_by"`
	ArchivedAt  time.Time `json:"archived_at"`
}

// NewNodeArchivedEvent creates a new node archived event
func NewNodeArchivedEvent(nodeID string, version int64, reason string) *NodeArchivedEvent {
	return &NodeArchivedEvent{
		BaseEvent:  *NewBaseEvent(nodeID, "Node", "NodeArchived", version),
		Reason:     reason,
		ArchivedAt: time.Now().UTC(),
	}
}

// NodeRestoredEvent is raised when an archived node is restored
type NodeRestoredEvent struct {
	BaseEvent
	RestoredBy string    `json:"restored_by"`
	RestoredAt time.Time `json:"restored_at"`
}

// NewNodeRestoredEvent creates a new node restored event
func NewNodeRestoredEvent(nodeID string, version int64) *NodeRestoredEvent {
	return &NodeRestoredEvent{
		BaseEvent:  *NewBaseEvent(nodeID, "Node", "NodeRestored", version),
		RestoredAt: time.Now().UTC(),
	}
}

// NodeDeletedEvent is raised when a node is permanently deleted
type NodeDeletedEvent struct {
	BaseEvent
	DeletedBy string    `json:"deleted_by"`
	DeletedAt time.Time `json:"deleted_at"`
}

// NewNodeDeletedEvent creates a new node deleted event
func NewNodeDeletedEvent(nodeID string, version int64) *NodeDeletedEvent {
	return &NodeDeletedEvent{
		BaseEvent:  *NewBaseEvent(nodeID, "Node", "NodeDeleted", version),
		DeletedAt: time.Now().UTC(),
	}
}

// NodeConnectedEvent is raised when a connection is created between nodes
type NodeConnectedEvent struct {
	BaseEvent
	TargetNodeID string                 `json:"target_node_id"`
	Weight       float64                `json:"weight"`
	EdgeType     string                 `json:"edge_type,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// NewNodeConnectedEvent creates a new node connected event
func NewNodeConnectedEvent(sourceNodeID, targetNodeID string, version int64, weight float64) *NodeConnectedEvent {
	return &NodeConnectedEvent{
		BaseEvent:    *NewBaseEvent(sourceNodeID, "Node", "NodeConnected", version),
		TargetNodeID: targetNodeID,
		Weight:       weight,
		Metadata:     make(map[string]interface{}),
	}
}

// NodeDisconnectedEvent is raised when a connection is removed between nodes
type NodeDisconnectedEvent struct {
	BaseEvent
	TargetNodeID string `json:"target_node_id"`
	Reason       string `json:"reason,omitempty"`
}

// NewNodeDisconnectedEvent creates a new node disconnected event
func NewNodeDisconnectedEvent(sourceNodeID, targetNodeID string, version int64) *NodeDisconnectedEvent {
	return &NodeDisconnectedEvent{
		BaseEvent:    *NewBaseEvent(sourceNodeID, "Node", "NodeDisconnected", version),
		TargetNodeID: targetNodeID,
	}
}

// NodeCategorizedEvent is raised when a node is assigned to categories
type NodeCategorizedEvent struct {
	BaseEvent
	CategoryIDs []string `json:"category_ids"`
	AddedIDs    []string `json:"added_ids"`
	RemovedIDs  []string `json:"removed_ids"`
}

// NewNodeCategorizedEvent creates a new node categorized event
func NewNodeCategorizedEvent(nodeID string, version int64, categoryIDs []string) *NodeCategorizedEvent {
	return &NodeCategorizedEvent{
		BaseEvent:   *NewBaseEvent(nodeID, "Node", "NodeCategorized", version),
		CategoryIDs: categoryIDs,
	}
}

// NodeTaggedEvent is raised when tags are updated on a node
type NodeTaggedEvent struct {
	BaseEvent
	Tags    []string `json:"tags"`
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// NewNodeTaggedEvent creates a new node tagged event
func NewNodeTaggedEvent(nodeID string, version int64, tags []string) *NodeTaggedEvent {
	return &NodeTaggedEvent{
		BaseEvent: *NewBaseEvent(nodeID, "Node", "NodeTagged", version),
		Tags:      tags,
	}
}

// NodeKeywordsExtractedEvent is raised when keywords are extracted from a node
type NodeKeywordsExtractedEvent struct {
	BaseEvent
	Keywords         []string          `json:"keywords"`
	ExtractionMethod string            `json:"extraction_method"`
	Confidence       float64           `json:"confidence"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// NewNodeKeywordsExtractedEvent creates a new keywords extracted event
func NewNodeKeywordsExtractedEvent(nodeID string, version int64, keywords []string) *NodeKeywordsExtractedEvent {
	return &NodeKeywordsExtractedEvent{
		BaseEvent:        *NewBaseEvent(nodeID, "Node", "NodeKeywordsExtracted", version),
		Keywords:         keywords,
		ExtractionMethod: "automatic",
		Confidence:       0.8,
	}
}

// Ensure all events implement DomainEvent interface
var (
	_ DomainEvent = (*NodeCreatedEvent)(nil)
	_ DomainEvent = (*NodeUpdatedEvent)(nil)
	_ DomainEvent = (*NodeArchivedEvent)(nil)
	_ DomainEvent = (*NodeRestoredEvent)(nil)
	_ DomainEvent = (*NodeDeletedEvent)(nil)
	_ DomainEvent = (*NodeConnectedEvent)(nil)
	_ DomainEvent = (*NodeDisconnectedEvent)(nil)
	_ DomainEvent = (*NodeCategorizedEvent)(nil)
	_ DomainEvent = (*NodeTaggedEvent)(nil)
	_ DomainEvent = (*NodeKeywordsExtractedEvent)(nil)
)

// Marshal implementations for each event type

func (e *NodeCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeUpdatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeArchivedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeRestoredEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeDeletedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeConnectedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeDisconnectedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeCategorizedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeTaggedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func (e *NodeKeywordsExtractedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}