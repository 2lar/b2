package domain

import (
	"time"
)

// DomainEvent represents an important business occurrence in the domain
type DomainEvent interface {
	// EventID returns a unique identifier for this event instance
	EventID() string
	
	// EventType returns the type of event (e.g., "NodeCreated", "NodeUpdated")
	EventType() string
	
	// AggregateID returns the ID of the aggregate that generated this event
	AggregateID() string
	
	// UserID returns the ID of the user who triggered this event
	UserID() string
	
	// Timestamp returns when the event occurred
	Timestamp() time.Time
	
	// Version returns the version of the aggregate when the event occurred
	Version() int
	
	// EventData returns the event-specific data
	EventData() map[string]interface{}
}

// BaseEvent provides common functionality for all domain events
type BaseEvent struct {
	eventID     string
	eventType   string
	aggregateID string
	userID      string
	timestamp   time.Time
	version     int
}

// EventID returns the unique event identifier
func (e BaseEvent) EventID() string {
	return e.eventID
}

// EventType returns the type of event
func (e BaseEvent) EventType() string {
	return e.eventType
}

// AggregateID returns the aggregate identifier
func (e BaseEvent) AggregateID() string {
	return e.aggregateID
}

// UserID returns the user identifier
func (e BaseEvent) UserID() string {
	return e.userID
}

// Timestamp returns the event timestamp
func (e BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// Version returns the aggregate version
func (e BaseEvent) Version() int {
	return e.version
}

// newBaseEvent creates a new base event with common fields
func newBaseEvent(eventType, aggregateID, userID string, version int) BaseEvent {
	return BaseEvent{
		eventID:     NewNodeID().String(), // Reuse NodeID generator for simplicity
		eventType:   eventType,
		aggregateID: aggregateID,
		userID:      userID,
		timestamp:   time.Now(),
		version:     version,
	}
}

// Node Events

// NodeCreatedEvent is fired when a new node is created
type NodeCreatedEvent struct {
	BaseEvent
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
}

// NewNodeCreatedEvent creates a new NodeCreatedEvent
func NewNodeCreatedEvent(nodeID NodeID, userID UserID, content Content, keywords Keywords, tags Tags, version Version) *NodeCreatedEvent {
	return &NodeCreatedEvent{
		BaseEvent: newBaseEvent("NodeCreated", nodeID.String(), userID.String(), version.Int()),
		Content:   content.String(),
		Keywords:  keywords.ToSlice(),
		Tags:      tags.ToSlice(),
	}
}

// EventData returns the event-specific data
func (e *NodeCreatedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"content":  e.Content,
		"keywords": e.Keywords,
		"tags":     e.Tags,
	}
}

// NodeContentUpdatedEvent is fired when node content is updated
type NodeContentUpdatedEvent struct {
	BaseEvent
	OldContent string   `json:"old_content"`
	NewContent string   `json:"new_content"`
	OldKeywords []string `json:"old_keywords"`
	NewKeywords []string `json:"new_keywords"`
}

// NewNodeContentUpdatedEvent creates a new NodeContentUpdatedEvent
func NewNodeContentUpdatedEvent(nodeID NodeID, userID UserID, oldContent, newContent Content, oldKeywords, newKeywords Keywords, version Version) *NodeContentUpdatedEvent {
	return &NodeContentUpdatedEvent{
		BaseEvent:   newBaseEvent("NodeContentUpdated", nodeID.String(), userID.String(), version.Int()),
		OldContent:  oldContent.String(),
		NewContent:  newContent.String(),
		OldKeywords: oldKeywords.ToSlice(),
		NewKeywords: newKeywords.ToSlice(),
	}
}

// EventData returns the event-specific data
func (e *NodeContentUpdatedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"old_content":  e.OldContent,
		"new_content":  e.NewContent,
		"old_keywords": e.OldKeywords,
		"new_keywords": e.NewKeywords,
	}
}

// NodeTagsUpdatedEvent is fired when node tags are updated
type NodeTagsUpdatedEvent struct {
	BaseEvent
	OldTags []string `json:"old_tags"`
	NewTags []string `json:"new_tags"`
}

// NewNodeTagsUpdatedEvent creates a new NodeTagsUpdatedEvent
func NewNodeTagsUpdatedEvent(nodeID NodeID, userID UserID, oldTags, newTags Tags, version Version) *NodeTagsUpdatedEvent {
	return &NodeTagsUpdatedEvent{
		BaseEvent: newBaseEvent("NodeTagsUpdated", nodeID.String(), userID.String(), version.Int()),
		OldTags:   oldTags.ToSlice(),
		NewTags:   newTags.ToSlice(),
	}
}

// EventData returns the event-specific data
func (e *NodeTagsUpdatedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"old_tags": e.OldTags,
		"new_tags": e.NewTags,
	}
}

// NodeDeletedEvent is fired when a node is deleted
type NodeDeletedEvent struct {
	BaseEvent
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
}

// NewNodeDeletedEvent creates a new NodeDeletedEvent
func NewNodeDeletedEvent(nodeID NodeID, userID UserID, content Content, keywords Keywords, tags Tags, version Version) *NodeDeletedEvent {
	return &NodeDeletedEvent{
		BaseEvent: newBaseEvent("NodeDeleted", nodeID.String(), userID.String(), version.Int()),
		Content:   content.String(),
		Keywords:  keywords.ToSlice(),
		Tags:      tags.ToSlice(),
	}
}

// EventData returns the event-specific data
func (e *NodeDeletedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"content":  e.Content,
		"keywords": e.Keywords,
		"tags":     e.Tags,
	}
}

// NodeArchivedEvent is fired when a node is archived
type NodeArchivedEvent struct {
	BaseEvent
	Reason string `json:"reason"`
}

// NewNodeArchivedEvent creates a new NodeArchivedEvent
func NewNodeArchivedEvent(nodeID NodeID, userID UserID, reason string, version Version) *NodeArchivedEvent {
	return &NodeArchivedEvent{
		BaseEvent: newBaseEvent("NodeArchived", nodeID.String(), userID.String(), version.Int()),
		Reason:    reason,
	}
}

// EventData returns the event-specific data
func (e *NodeArchivedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"reason": e.Reason,
	}
}

// Edge Events

// EdgeCreatedEvent is fired when a new edge is created
type EdgeCreatedEvent struct {
	BaseEvent
	SourceNodeID string  `json:"source_node_id"`
	TargetNodeID string  `json:"target_node_id"`
	Weight       float64 `json:"weight"`
}

// NewEdgeCreatedEvent creates a new EdgeCreatedEvent
func NewEdgeCreatedEvent(edgeID, sourceNodeID, targetNodeID NodeID, userID UserID, weight float64, version Version) *EdgeCreatedEvent {
	return &EdgeCreatedEvent{
		BaseEvent:    newBaseEvent("EdgeCreated", edgeID.String(), userID.String(), version.Int()),
		SourceNodeID: sourceNodeID.String(),
		TargetNodeID: targetNodeID.String(),
		Weight:       weight,
	}
}

// EventData returns the event-specific data
func (e *EdgeCreatedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"source_node_id": e.SourceNodeID,
		"target_node_id": e.TargetNodeID,
		"weight":         e.Weight,
	}
}

// EdgeDeletedEvent is fired when an edge is deleted
type EdgeDeletedEvent struct {
	BaseEvent
	SourceNodeID string  `json:"source_node_id"`
	TargetNodeID string  `json:"target_node_id"`
	Weight       float64 `json:"weight"`
}

// NewEdgeDeletedEvent creates a new EdgeDeletedEvent
func NewEdgeDeletedEvent(edgeID, sourceNodeID, targetNodeID NodeID, userID UserID, weight float64, version Version) *EdgeDeletedEvent {
	return &EdgeDeletedEvent{
		BaseEvent:    newBaseEvent("EdgeDeleted", edgeID.String(), userID.String(), version.Int()),
		SourceNodeID: sourceNodeID.String(),
		TargetNodeID: targetNodeID.String(),
		Weight:       weight,
	}
}

// EventData returns the event-specific data
func (e *EdgeDeletedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"source_node_id": e.SourceNodeID,
		"target_node_id": e.TargetNodeID,
		"weight":         e.Weight,
	}
}

// Connection Events

// PotentialConnectionFoundEvent is fired when the system finds a potential connection between nodes
type PotentialConnectionFoundEvent struct {
	BaseEvent
	SourceNodeID     string  `json:"source_node_id"`
	TargetNodeID     string  `json:"target_node_id"`
	SimilarityScore  float64 `json:"similarity_score"`
	MatchingKeywords []string `json:"matching_keywords"`
}

// NewPotentialConnectionFoundEvent creates a new PotentialConnectionFoundEvent
func NewPotentialConnectionFoundEvent(sourceNodeID, targetNodeID NodeID, userID UserID, similarityScore float64, matchingKeywords []string) *PotentialConnectionFoundEvent {
	return &PotentialConnectionFoundEvent{
		BaseEvent:        newBaseEvent("PotentialConnectionFound", sourceNodeID.String(), userID.String(), 0),
		SourceNodeID:     sourceNodeID.String(),
		TargetNodeID:     targetNodeID.String(),
		SimilarityScore:  similarityScore,
		MatchingKeywords: matchingKeywords,
	}
}

// EventData returns the event-specific data
func (e *PotentialConnectionFoundEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"source_node_id":     e.SourceNodeID,
		"target_node_id":     e.TargetNodeID,
		"similarity_score":   e.SimilarityScore,
		"matching_keywords":  e.MatchingKeywords,
	}
}

// EventAggregate interface for entities that can generate domain events
type EventAggregate interface {
	// GetUncommittedEvents returns events that haven't been persisted yet
	GetUncommittedEvents() []DomainEvent
	
	// MarkEventsAsCommitted clears the uncommitted events after persistence
	MarkEventsAsCommitted()
}

// EventStore interface for persisting and retrieving domain events
type EventStore interface {
	// SaveEvents saves events to the store
	SaveEvents(aggregateID string, events []DomainEvent, expectedVersion int) error
	
	// GetEvents retrieves events for an aggregate
	GetEvents(aggregateID string, fromVersion int) ([]DomainEvent, error)
	
	// GetAllEvents retrieves all events of a specific type
	GetAllEvents(eventType string, fromTimestamp time.Time) ([]DomainEvent, error)
}