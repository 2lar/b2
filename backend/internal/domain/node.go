package domain

import (
	"context"
	"time"
)

// Node represents a memory, thought, or piece of knowledge in a user's knowledge graph.
// This is a rich domain model that encapsulates all business logic related to nodes.
//
// Key Design Principles Demonstrated:
//   - Rich Domain Model: Contains behavior, not just data
//   - Encapsulation: Internal state is protected with private fields
//   - Value Objects: Uses strongly-typed value objects instead of primitives
//   - Domain Events: Tracks important business occurrences
//   - Business Invariants: Ensures the node is always in a valid state
//   - Factory Pattern: Uses factory methods for complex construction
type Node struct {
	// Private fields ensure encapsulation - external code must use methods
	id        NodeID    // Value object for type safety
	content   Content   // Value object with business rules
	keywords  Keywords  // Value object with keyword logic
	tags      Tags      // Value object for tag management
	userID    UserID    // Value object for user identification
	createdAt time.Time // When the node was created
	updatedAt time.Time // When the node was last updated
	version   Version   // For optimistic locking
	archived  bool      // Whether the node is archived

	// Public fields for compatibility with existing code
	ID        NodeID                 `json:"id"`
	UserID    UserID                 `json:"user_id"`
	Content   Content                `json:"content"`
	Tags      Tags                   `json:"tags"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Version   int                    `json:"version"`

	// Domain events that occurred during this aggregate's lifetime
	events []DomainEvent
}

// NewNode creates a new node with full business rule validation.
// This factory method ensures that all nodes are created in a valid state.
//
// Business Rules Enforced:
//   - UserID must be valid
//   - Content must pass validation (length, profanity, etc.)
//   - Tags are normalized and validated
//   - Keywords are automatically extracted from content
//   - Version always starts at 0 for new nodes
//   - Domain events are generated for the creation
func NewNode(userID UserID, content Content, tags Tags) (*Node, error) {
	// Validate content (already done in Content value object, but explicit check)
	if err := content.Validate(); err != nil {
		return nil, NewDomainError("invalid_content", "node content validation failed", err)
	}

	now := time.Now()
	nodeID := NewNodeID()
	keywords := content.ExtractKeywords()

	node := &Node{
		id:        nodeID,
		userID:    userID,
		content:   content,
		keywords:  keywords,
		tags:      tags,
		createdAt: now,
		updatedAt: now,
		version:   NewVersion(), // Always start at 0
		archived:  false,
		events:    []DomainEvent{},
		// Initialize public fields
		ID:        nodeID,
		UserID:    userID,
		Content:   content,
		Tags:      tags,
		Metadata:  make(map[string]interface{}),
		CreatedAt: now,
		UpdatedAt: now,
		Version:   0, // Start at version 0
	}

	// Generate domain event for node creation
	event := NewNodeCreatedEvent(nodeID, userID, content, keywords, tags, node.version)
	node.addEvent(event)

	return node, nil
}

// Factory method for reconstructing nodes from persistence (no events generated)
func ReconstructNode(id NodeID, userID UserID, content Content, keywords Keywords, tags Tags,
	createdAt, updatedAt time.Time, version Version, archived bool) *Node {
	return &Node{
		// Private fields
		id:        id,
		userID:    userID,
		content:   content,
		keywords:  keywords,
		tags:      tags,
		createdAt: createdAt,
		updatedAt: updatedAt,
		version:   version,
		archived:  archived,
		events:    []DomainEvent{},
		// Public fields (for compatibility)
		ID:        id,
		UserID:    userID,
		Content:   content,
		Tags:      tags,
		Metadata:  make(map[string]interface{}),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Version:   version.Int(),
	}
}

// ReconstructNodeFromPrimitives creates a node from primitive types (for repository layer)
func ReconstructNodeFromPrimitives(id, userID, content string, keywords, tags []string, createdAt time.Time, version int) (*Node, error) {
	nodeID, err := ParseNodeID(id)
	if err != nil {
		return nil, err
	}

	userIDVO, err := NewUserID(userID)
	if err != nil {
		return nil, err
	}

	contentVO, err := NewContent(content)
	if err != nil {
		return nil, err
	}

	keywordsVO := NewKeywords(keywords)
	tagsVO := NewTags(tags...)
	versionVO := ParseVersion(version)

	// Use ReconstructNode which now properly initializes both private and public fields
	return ReconstructNode(nodeID, userIDVO, contentVO, keywordsVO, tagsVO,
		createdAt, createdAt, versionVO, false), nil
}

// Getters (read-only access to internal state)

// Keywords returns the node's keywords
func (n *Node) Keywords() Keywords {
	return n.keywords
}

// IsArchived returns whether the node is archived
func (n *Node) IsArchived() bool {
	return n.archived
}

// Business Methods (encapsulated business logic)

// UpdateContent updates the node's content following business rules.
//
// Business Rules Enforced:
//   - Cannot update archived nodes
//   - Content must be valid
//   - Keywords are automatically re-extracted
//   - Version is incremented for optimistic locking
//   - Domain events are generated if content actually changes
func (n *Node) UpdateContent(newContent Content) error {
	if n.archived {
		return ErrCannotUpdateArchivedNode
	}

	if n.content.Equals(newContent) {
		return nil // No change needed
	}

	// Store old values for events
	oldContent := n.content
	oldKeywords := n.keywords

	// Apply changes
	n.content = newContent
	n.Content = newContent // Also update public field
	n.keywords = newContent.ExtractKeywords()
	n.updatedAt = time.Now()
	n.UpdatedAt = n.updatedAt // Also update public field
	n.version = n.version.Next()
	n.Version = n.version.Int() // Also update public field

	// Generate domain event
	event := NewNodeContentUpdatedEvent(n.id, n.userID, oldContent, newContent, oldKeywords, n.keywords, n.version)
	n.addEvent(event)

	return nil
}

// UpdateTags updates the node's tags following business rules.
//
// Business Rules Enforced:
//   - Cannot update archived nodes
//   - Tags are normalized and validated
//   - Version is incremented for optimistic locking
//   - Domain events are generated if tags actually change
func (n *Node) UpdateTags(newTags Tags) error {
	if n.archived {
		return ErrCannotUpdateArchivedNode
	}

	// Check if tags actually changed (simple equality check)
	if tagsEqual(n.tags, newTags) {
		return nil // No change needed
	}

	// Store old values for events
	oldTags := n.tags

	// Apply changes
	n.tags = newTags
	n.Tags = newTags // Also update public field
	n.updatedAt = time.Now()
	n.UpdatedAt = n.updatedAt // Also update public field
	n.version = n.version.Next()
	n.Version = n.version.Int() // Also update public field

	// Generate domain event
	event := NewNodeTagsUpdatedEvent(n.id, n.userID, oldTags, newTags, n.version)
	n.addEvent(event)

	return nil
}

// Archive marks the node as archived with a reason.
//
// Business Rules Enforced:
//   - Cannot archive already archived nodes
//   - Version is incremented
//   - Domain events are generated
func (n *Node) Archive(reason string) error {
	if n.archived {
		return NewBusinessRuleError("archive_archived_node", "Node", "cannot archive already archived node")
	}

	n.archived = true
	n.updatedAt = time.Now()
	n.version = n.version.Next()

	// Generate domain event
	event := NewNodeArchivedEvent(n.id, n.userID, reason, n.version)
	n.addEvent(event)

	return nil
}

// CanConnectTo checks if this node can connect to another node based on business rules.
//
// Business Rules Enforced:
//   - Cannot connect to self
//   - Cannot connect nodes from different users
//   - Cannot connect archived nodes
func (n *Node) CanConnectTo(target *Node) error {
	if n.id.Equals(target.id) {
		return ErrCannotConnectToSelf
	}

	if !n.userID.Equals(target.userID) {
		return ErrCrossUserConnection
	}

	if n.archived || target.archived {
		return ErrCannotConnectArchivedNodes
	}

	return nil
}

// CalculateSimilarityTo calculates similarity with another node based on content and tags
func (n *Node) CalculateSimilarityTo(other *Node) float64 {
	if n.id.Equals(other.id) {
		return 0 // Same node has no similarity for connection purposes
	}

	// Weighted combination of keyword and tag similarity
	keywordSimilarity := n.keywords.Overlap(other.keywords)
	tagSimilarity := n.tags.Overlap(other.tags)

	// Weight keywords more heavily than tags
	return keywordSimilarity*0.7 + tagSimilarity*0.3
}

// HasKeyword checks if the node contains a specific keyword
func (n *Node) HasKeyword(keyword string) bool {
	return n.keywords.Contains(keyword)
}

// HasTag checks if the node has a specific tag
func (n *Node) HasTag(tag string) bool {
	return n.tags.Contains(tag)
}

// WordCount returns the number of words in the content
func (n *Node) WordCount() int {
	return n.content.WordCount()
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (n *Node) GetUncommittedEvents() []DomainEvent {
	return n.events
}

// MarkEventsAsCommitted clears the events after persistence
func (n *Node) MarkEventsAsCommitted() {
	n.events = []DomainEvent{}
}

// Private helper methods

// addEvent adds a domain event to the uncommitted events list
func (n *Node) addEvent(event DomainEvent) {
	n.events = append(n.events, event)
}

// Helper function to compare tags (since we can't easily compare Tags value objects)
func tagsEqual(tags1, tags2 Tags) bool {
	slice1 := tags1.ToSlice()
	slice2 := tags2.ToSlice()

	if len(slice1) != len(slice2) {
		return false
	}

	for i, tag := range slice1 {
		if tag != slice2[i] {
			return false
		}
	}

	return true
}

// Validate validates the node's state
func (n *Node) Validate() error {
	if n.ID.IsEmpty() {
		return NewValidationError("id", "node ID is required", n.ID)
	}
	if n.UserID.IsEmpty() {
		return NewValidationError("user_id", "user ID is required", n.UserID)
	}
	if err := n.Content.Validate(); err != nil {
		return err
	}
	return nil
}

// SetMetadata sets the metadata for the node
func (n *Node) SetMetadata(metadata map[string]interface{}) {
	n.Metadata = metadata
}

// SetTags sets the tags for the node
func (n *Node) SetTags(tags Tags) {
	n.tags = tags
	n.Tags = tags
}


// UpdateTimestamp updates the node's timestamp
func (n *Node) UpdateTimestamp() {
	n.updatedAt = time.Now()
	n.UpdatedAt = n.updatedAt
}

// Events returns the domain events for this node
func (n *Node) Events() []DomainEvent {
	return n.events
}

// =====

// NodeRepository defines the persistence methods for a Node.
type NodeRepository interface {
	FindByID(ctx context.Context, userID string, id NodeID) (*Node, error)
	FindByGraphID(ctx context.Context, userID string, graphID GraphID) ([]*Node, error)
	Save(ctx context.Context, node *Node) error
	Delete(ctx context.Context, userID string, id NodeID) error
}
