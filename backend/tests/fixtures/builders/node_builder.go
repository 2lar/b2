// Package builders provides test builders for creating domain objects in tests
package builders

import (
	"fmt"
	"time"

	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
	"github.com/google/uuid"
)

// NodeBuilder provides a fluent interface for building test nodes
type NodeBuilder struct {
	id          valueobjects.NodeID
	userID      valueobjects.UserID
	content     valueobjects.Content
	title       valueobjects.Title
	tags        valueobjects.Tags
	keywords    valueobjects.Keywords
	categoryIDs []valueobjects.CategoryID
	metadata    map[string]interface{}
	createdAt   time.Time
	updatedAt   time.Time
	version     int64
	isArchived  bool
}

// NewNodeBuilder creates a new node builder with sensible defaults
func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{
		id:        valueobjects.NewNodeID(uuid.New().String()),
		userID:    valueobjects.NewUserID("test-user-" + uuid.New().String()),
		content:   valueobjects.NewContent("Test content"),
		title:     valueobjects.NewTitle("Test Node"),
		tags:      valueobjects.NewTags([]string{"test"}),
		keywords:  valueobjects.NewKeywords([]string{"test", "node"}),
		createdAt: time.Now(),
		updatedAt: time.Now(),
		version:   1,
		metadata:  make(map[string]interface{}),
	}
}

// WithID sets the node ID
func (b *NodeBuilder) WithID(id string) *NodeBuilder {
	b.id = valueobjects.NewNodeID(id)
	return b
}

// WithUserID sets the user ID
func (b *NodeBuilder) WithUserID(userID string) *NodeBuilder {
	b.userID = valueobjects.NewUserID(userID)
	return b
}

// WithContent sets the node content
func (b *NodeBuilder) WithContent(content string) *NodeBuilder {
	b.content = valueobjects.NewContent(content)
	return b
}

// WithTitle sets the node title
func (b *NodeBuilder) WithTitle(title string) *NodeBuilder {
	b.title = valueobjects.NewTitle(title)
	return b
}

// WithTags sets the node tags
func (b *NodeBuilder) WithTags(tags ...string) *NodeBuilder {
	b.tags = valueobjects.NewTags(tags)
	return b
}

// WithKeywords sets the node keywords
func (b *NodeBuilder) WithKeywords(keywords ...string) *NodeBuilder {
	b.keywords = valueobjects.NewKeywords(keywords)
	return b
}

// WithCategories sets the node categories
func (b *NodeBuilder) WithCategories(categoryIDs ...string) *NodeBuilder {
	b.categoryIDs = make([]valueobjects.CategoryID, len(categoryIDs))
	for i, id := range categoryIDs {
		b.categoryIDs[i] = valueobjects.NewCategoryID(id)
	}
	return b
}

// WithMetadata sets metadata key-value pairs
func (b *NodeBuilder) WithMetadata(key string, value interface{}) *NodeBuilder {
	b.metadata[key] = value
	return b
}

// WithCreatedAt sets the creation timestamp
func (b *NodeBuilder) WithCreatedAt(t time.Time) *NodeBuilder {
	b.createdAt = t
	return b
}

// WithUpdatedAt sets the update timestamp
func (b *NodeBuilder) WithUpdatedAt(t time.Time) *NodeBuilder {
	b.updatedAt = t
	return b
}

// WithVersion sets the aggregate version
func (b *NodeBuilder) WithVersion(version int64) *NodeBuilder {
	b.version = version
	return b
}

// AsArchived marks the node as archived
func (b *NodeBuilder) AsArchived() *NodeBuilder {
	b.isArchived = true
	return b
}

// Build creates the node aggregate
func (b *NodeBuilder) Build() *node.Aggregate {
	// Create the aggregate using the proper constructor
	aggregate, err := node.NewAggregate(
		b.id,
		b.userID,
		b.content,
		b.title,
		b.tags,
	)
	if err != nil {
		// For tests, we'll panic on error
		panic(fmt.Sprintf("failed to create node: %v", err))
	}

	// Apply archived event if needed
	if b.isArchived {
		err = aggregate.Archive()
		if err != nil {
			panic(fmt.Sprintf("failed to archive node: %v", err))
		}
	}

	// Version is managed internally by the aggregate
	// No need to set it manually

	return aggregate
}

// BuildWithEvents creates a node aggregate and returns it with its events
func (b *NodeBuilder) BuildWithEvents() (*node.Aggregate, []events.DomainEvent) {
	aggregate := b.Build()
	return aggregate, aggregate.GetUncommittedEvents()
}

// getCategoryIDStrings converts category IDs to strings
func (b *NodeBuilder) getCategoryIDStrings() []string {
	result := make([]string, len(b.categoryIDs))
	for i, id := range b.categoryIDs {
		result[i] = id.String()
	}
	return result
}

// NodeBuilderPresets provides common node configurations
type NodeBuilderPresets struct{}

// NewNodeBuilderPresets creates a new presets helper
func NewNodeBuilderPresets() *NodeBuilderPresets {
	return &NodeBuilderPresets{}
}

// SimpleNode creates a basic node with minimal configuration
func (p *NodeBuilderPresets) SimpleNode(userID string) *node.Aggregate {
	return NewNodeBuilder().
		WithUserID(userID).
		Build()
}

// RichNode creates a node with all fields populated
func (p *NodeBuilderPresets) RichNode(userID string) *node.Aggregate {
	return NewNodeBuilder().
		WithUserID(userID).
		WithTitle("Rich Test Node").
		WithContent("This is a rich test node with extensive content for testing purposes.").
		WithTags("test", "rich", "example").
		WithKeywords("testing", "comprehensive", "example", "node").
		WithCategories("cat-1", "cat-2").
		WithMetadata("importance", "high").
		WithMetadata("source", "test-suite").
		Build()
}

// ArchivedNode creates an archived node
func (p *NodeBuilderPresets) ArchivedNode(userID string) *node.Aggregate {
	return NewNodeBuilder().
		WithUserID(userID).
		AsArchived().
		Build()
}