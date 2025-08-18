package shared

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

// NewBaseEvent creates a new base event with common fields (exported for external packages)
func NewBaseEvent(eventType, aggregateID, userID string, version int) BaseEvent {
	return newBaseEvent(eventType, aggregateID, userID, version)
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

// Category Events

// CategoryCreatedEvent is fired when a new category is created
type CategoryCreatedEvent struct {
	BaseEvent
	Name        string `json:"name"`
	Description string `json:"description"`
	Level       int    `json:"level"`
}

// NewCategoryCreatedEvent creates a new CategoryCreatedEvent
func NewCategoryCreatedEvent(categoryID CategoryID, userID UserID, name, description string, level int) *CategoryCreatedEvent {
	return &CategoryCreatedEvent{
		BaseEvent:   newBaseEvent("CategoryCreated", string(categoryID), userID.String(), 1),
		Name:        name,
		Description: description,
		Level:       level,
	}
}

// EventData returns the event-specific data
func (e *CategoryCreatedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"name":        e.Name,
		"description": e.Description,
		"level":       e.Level,
	}
}

// CategoryDeletedEvent is fired when a category is deleted
type CategoryDeletedEvent struct {
	BaseEvent
	Name     string `json:"name"`
	Level    int    `json:"level"`
	NoteCount int   `json:"note_count"`
}

// NewCategoryDeletedEvent creates a new CategoryDeletedEvent
func NewCategoryDeletedEvent(categoryID CategoryID, userID UserID, name string, level, noteCount int) *CategoryDeletedEvent {
	return &CategoryDeletedEvent{
		BaseEvent: newBaseEvent("CategoryDeleted", string(categoryID), userID.String(), 1),
		Name:      name,
		Level:     level,
		NoteCount: noteCount,
	}
}

// EventData returns the event-specific data
func (e *CategoryDeletedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"name":       e.Name,
		"level":      e.Level,
		"note_count": e.NoteCount,
	}
}

// NodeRemovedFromCategoryEvent is fired when a node is removed from a category
type NodeRemovedFromCategoryEvent struct {
	BaseEvent
	NodeID     string `json:"node_id"`
	CategoryID string `json:"category_id"`
}

// NewNodeRemovedFromCategoryEvent creates a new NodeRemovedFromCategoryEvent
func NewNodeRemovedFromCategoryEvent(nodeID NodeID, categoryID CategoryID, userID UserID) *NodeRemovedFromCategoryEvent {
	return &NodeRemovedFromCategoryEvent{
		BaseEvent:  newBaseEvent("NodeRemovedFromCategory", nodeID.String(), userID.String(), 1),
		NodeID:     nodeID.String(),
		CategoryID: string(categoryID),
	}
}

// EventData returns the event-specific data
func (e *NodeRemovedFromCategoryEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"node_id":     e.NodeID,
		"category_id": e.CategoryID,
	}
}

// NodeAssignedToCategoryEvent is fired when a node is assigned to a category
type NodeAssignedToCategoryEvent struct {
	BaseEvent
	NodeID     string `json:"node_id"`
	CategoryID string `json:"category_id"`
}

// NewNodeAssignedToCategoryEvent creates a new NodeAssignedToCategoryEvent
func NewNodeAssignedToCategoryEvent(nodeID, categoryID, userID string, timestamp time.Time) *NodeAssignedToCategoryEvent {
	return &NodeAssignedToCategoryEvent{
		BaseEvent:  newBaseEvent("NodeAssignedToCategory", nodeID, userID, 1),
		NodeID:     nodeID,
		CategoryID: categoryID,
	}
}

// EventData returns the event-specific data
func (e *NodeAssignedToCategoryEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"node_id":     e.NodeID,
		"category_id": e.CategoryID,
	}
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