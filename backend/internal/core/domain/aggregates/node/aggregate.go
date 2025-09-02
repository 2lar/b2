// Package node implements the Node aggregate root with event sourcing.
// The Node aggregate represents a memory or thought in the knowledge graph.
package node

import (
	"fmt"
	"time"
	
	"brain2-backend/internal/core/domain/aggregates"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
)

// Aggregate is the Node aggregate root that manages node state through events
type Aggregate struct {
	*aggregates.BaseAggregate
	
	// Current state (derived from events)
	userID          valueobjects.UserID
	content         valueobjects.Content
	title           valueobjects.Title
	keywords        valueobjects.Keywords
	tags            valueobjects.Tags
	categories      []string
	archived        bool
	metadata        map[string]interface{}
	connectionCount int
}

// NewAggregate creates a new Node aggregate
func NewAggregate(
	nodeID valueobjects.NodeID,
	userID valueobjects.UserID,
	content valueobjects.Content,
	title valueobjects.Title,
	tags valueobjects.Tags,
) (*Aggregate, error) {
	// Validate inputs
	if err := content.Validate(); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	
	if err := title.Validate(); err != nil {
		return nil, fmt.Errorf("invalid title: %w", err)
	}
	
	aggregate := &Aggregate{
		BaseAggregate:   aggregates.NewBaseAggregate(nodeID.String()),
		userID:          userID,
		content:         content,
		title:           title,
		tags:            tags,
		keywords:        content.ExtractKeywords(),
		categories:      []string{},
		archived:        false,
		metadata:        make(map[string]interface{}),
		connectionCount: 0,
	}
	
	// Create and apply the creation event
	event := NewNodeCreatedEvent(
		nodeID.String(),
		userID.String(),
		content.String(),
		title.String(),
		aggregate.keywords.ToSlice(),
		tags.ToSlice(),
		1, // Version 1 for new aggregate
	)
	
	aggregate.AddEvent(event)
	
	return aggregate, nil
}

// LoadFromHistory recreates the aggregate from historical events
func LoadFromHistory(id string, history []events.DomainEvent) (*Aggregate, error) {
	aggregate := &Aggregate{
		BaseAggregate: aggregates.NewBaseAggregate(id),
		metadata:      make(map[string]interface{}),
		categories:    []string{},
	}
	
	for _, event := range history {
		if err := aggregate.When(event); err != nil {
			return nil, fmt.Errorf("failed to apply event %s: %w", event.GetEventID(), err)
		}
		aggregate.Version++
	}
	
	return aggregate, nil
}

// When applies an event to update the aggregate state
func (a *Aggregate) When(event events.DomainEvent) error {
	switch e := event.(type) {
	case *NodeCreatedEvent:
		return a.whenNodeCreated(e)
	case *NodeUpdatedEvent:
		return a.whenNodeUpdated(e)
	case *NodeArchivedEvent:
		return a.whenNodeArchived(e)
	case *NodeRestoredEvent:
		return a.whenNodeRestored(e)
	case *NodeTaggedEvent:
		return a.whenNodeTagged(e)
	case *NodeCategorizedEvent:
		return a.whenNodeCategorized(e)
	case *NodeConnectedEvent:
		return a.whenNodeConnected(e)
	case *NodeDisconnectedEvent:
		return a.whenNodeDisconnected(e)
	default:
		return fmt.Errorf("unknown event type: %T", event)
	}
}

// Event handlers
func (a *Aggregate) whenNodeCreated(e *NodeCreatedEvent) error {
	a.userID = valueobjects.NewUserID(e.UserID)
	a.content = valueobjects.NewContent(e.Content)
	a.title = valueobjects.NewTitle(e.Title)
	a.keywords = valueobjects.NewKeywords(e.Keywords)
	a.tags = valueobjects.NewTags(e.Tags)
	a.archived = false
	return nil
}

func (a *Aggregate) whenNodeUpdated(e *NodeUpdatedEvent) error {
	a.content = valueobjects.NewContent(e.NewContent)
	a.title = valueobjects.NewTitle(e.NewTitle)
	a.keywords = a.content.ExtractKeywords()
	return nil
}

func (a *Aggregate) whenNodeArchived(e *NodeArchivedEvent) error {
	a.archived = true
	return nil
}

func (a *Aggregate) whenNodeRestored(e *NodeRestoredEvent) error {
	a.archived = false
	return nil
}

func (a *Aggregate) whenNodeTagged(e *NodeTaggedEvent) error {
	a.tags = a.tags.Add(e.Tags...)
	return nil
}

func (a *Aggregate) whenNodeCategorized(e *NodeCategorizedEvent) error {
	// Check if category already exists
	for _, cat := range a.categories {
		if cat == e.CategoryID {
			return nil // Already categorized
		}
	}
	a.categories = append(a.categories, e.CategoryID)
	return nil
}

func (a *Aggregate) whenNodeConnected(e *NodeConnectedEvent) error {
	a.connectionCount++
	return nil
}

func (a *Aggregate) whenNodeDisconnected(e *NodeDisconnectedEvent) error {
	if a.connectionCount > 0 {
		a.connectionCount--
	}
	return nil
}

// Command methods that generate events

// UpdateContent updates the node's content and title
func (a *Aggregate) UpdateContent(newContent valueobjects.Content, newTitle valueobjects.Title) error {
	if a.archived {
		return fmt.Errorf("cannot update archived node")
	}
	
	if err := newContent.Validate(); err != nil {
		return fmt.Errorf("invalid content: %w", err)
	}
	
	if err := newTitle.Validate(); err != nil {
		return fmt.Errorf("invalid title: %w", err)
	}
	
	// Only create event if content actually changed
	if a.content.Equals(newContent) && a.title.Equals(newTitle) {
		return nil
	}
	
	event := NewNodeUpdatedEvent(
		a.ID,
		a.content.String(),
		newContent.String(),
		a.title.String(),
		newTitle.String(),
		a.Version+1,
	)
	
	a.AddEvent(event)
	a.content = newContent
	a.title = newTitle
	a.keywords = newContent.ExtractKeywords()
	
	return nil
}

// Archive marks the node as archived
func (a *Aggregate) Archive() error {
	if a.archived {
		return nil // Already archived
	}
	
	event := NewNodeArchivedEvent(a.ID, a.Version+1)
	a.AddEvent(event)
	a.archived = true
	
	return nil
}

// Restore restores an archived node
func (a *Aggregate) Restore() error {
	if !a.archived {
		return nil // Not archived
	}
	
	event := NewNodeRestoredEvent(a.ID, a.Version+1)
	a.AddEvent(event)
	a.archived = false
	
	return nil
}

// AddTags adds new tags to the node
func (a *Aggregate) AddTags(tags ...string) error {
	if a.archived {
		return fmt.Errorf("cannot tag archived node")
	}
	
	newTags := []string{}
	for _, tag := range tags {
		if !a.tags.Contains(tag) {
			newTags = append(newTags, tag)
		}
	}
	
	if len(newTags) == 0 {
		return nil // No new tags
	}
	
	event := NewNodeTaggedEvent(a.ID, newTags, a.Version+1)
	a.AddEvent(event)
	a.tags = a.tags.Add(newTags...)
	
	return nil
}

// Categorize adds the node to a category
func (a *Aggregate) Categorize(categoryID string) error {
	if a.archived {
		return fmt.Errorf("cannot categorize archived node")
	}
	
	// Check if already in this category
	for _, cat := range a.categories {
		if cat == categoryID {
			return nil
		}
	}
	
	event := NewNodeCategorizedEvent(a.ID, categoryID, a.Version+1)
	a.AddEvent(event)
	a.categories = append(a.categories, categoryID)
	
	return nil
}

// Connect records a connection to another node
func (a *Aggregate) Connect(targetNodeID string, strength float64) error {
	if a.archived {
		return fmt.Errorf("cannot connect archived node")
	}
	
	event := NewNodeConnectedEvent(a.ID, targetNodeID, strength, a.Version+1)
	a.AddEvent(event)
	a.connectionCount++
	
	return nil
}

// Disconnect removes a connection to another node
func (a *Aggregate) Disconnect(targetNodeID string) error {
	event := NewNodeDisconnectedEvent(a.ID, targetNodeID, a.Version+1)
	a.AddEvent(event)
	if a.connectionCount > 0 {
		a.connectionCount--
	}
	
	return nil
}

// Query methods

// GetUserID returns the user who owns this node
func (a *Aggregate) GetUserID() string {
	return a.userID.String()
}

// GetContent returns the node's content
func (a *Aggregate) GetContent() string {
	return a.content.String()
}

// GetTitle returns the node's title
func (a *Aggregate) GetTitle() string {
	return a.title.String()
}

// GetKeywords returns the extracted keywords
func (a *Aggregate) GetKeywords() []string {
	return a.keywords.ToSlice()
}

// GetTags returns the node's tags
func (a *Aggregate) GetTags() []string {
	return a.tags.ToSlice()
}

// GetCategories returns the categories this node belongs to
func (a *Aggregate) GetCategories() []string {
	return a.categories
}

// IsArchived returns whether the node is archived
func (a *Aggregate) IsArchived() bool {
	return a.archived
}

// GetCategoryIDs returns the node's category IDs
func (a *Aggregate) GetCategoryIDs() []string {
	return a.categories
}

// GetCreatedAt returns when the node was created
func (a *Aggregate) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns when the node was last updated
func (a *Aggregate) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// GetConnectionCount returns the number of connections
func (a *Aggregate) GetConnectionCount() int {
	return a.connectionCount
}

// GetMetadata returns the node's metadata
func (a *Aggregate) GetMetadata() map[string]interface{} {
	return a.metadata
}

// CreateSnapshot creates a snapshot of the current aggregate state
func (a *Aggregate) CreateSnapshot() (*aggregates.Snapshot, error) {
	data := map[string]interface{}{
		"user_id":          a.userID.String(),
		"content":          a.content.String(),
		"title":            a.title.String(),
		"keywords":         a.keywords.ToSlice(),
		"tags":             a.tags.ToSlice(),
		"categories":       a.categories,
		"archived":         a.archived,
		"connection_count": a.connectionCount,
		"metadata":         a.metadata,
	}
	
	return &aggregates.Snapshot{
		AggregateID: a.ID,
		Version:     a.Version,
		Data:        data,
		Timestamp:   time.Now(),
	}, nil
}

// RestoreFromSnapshot restores the aggregate state from a snapshot
func (a *Aggregate) RestoreFromSnapshot(snapshot *aggregates.Snapshot) error {
	data, ok := snapshot.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid snapshot data format")
	}
	
	// Restore state from snapshot
	if userID, ok := data["user_id"].(string); ok {
		a.userID = valueobjects.NewUserID(userID)
	}
	if content, ok := data["content"].(string); ok {
		a.content = valueobjects.NewContent(content)
	}
	if title, ok := data["title"].(string); ok {
		a.title = valueobjects.NewTitle(title)
	}
	if keywords, ok := data["keywords"].([]string); ok {
		a.keywords = valueobjects.NewKeywords(keywords)
	}
	if tags, ok := data["tags"].([]string); ok {
		a.tags = valueobjects.NewTags(tags)
	}
	if categories, ok := data["categories"].([]string); ok {
		a.categories = categories
	}
	if archived, ok := data["archived"].(bool); ok {
		a.archived = archived
	}
	if count, ok := data["connection_count"].(int); ok {
		a.connectionCount = count
	}
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		a.metadata = metadata
	}
	
	a.Version = snapshot.Version
	
	return nil
}

// NewAggregateWithData creates a node aggregate with all data (for reconstruction from persistence)
func NewAggregateWithData(
	id valueobjects.NodeID,
	userID valueobjects.UserID,
	content valueobjects.Content,
	title valueobjects.Title,
	tags valueobjects.Tags,
	keywords valueobjects.Keywords,
	categoryIDs []string,
	isArchived bool,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
	metadata map[string]interface{},
) *Aggregate {
	return &Aggregate{
		BaseAggregate: &aggregates.BaseAggregate{
			ID:        id.String(),
			Version:   version,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		userID:      userID,
		content:     content,
		title:       title,
		tags:        tags,
		keywords:    keywords,
		categories:  categoryIDs,
		archived:    isArchived,
		metadata:    metadata,
	}
}