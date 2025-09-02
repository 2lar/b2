// Package aggregates contains the domain aggregate roots
package aggregates

import (
	"fmt"
	"time"

	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
)

// NodeAggregate represents a knowledge node aggregate root
type NodeAggregate struct {
	BaseAggregate

	// Current state
	userID      valueobjects.UserID
	content     valueobjects.Content
	title       valueobjects.Title
	tags        valueobjects.Tags
	keywords    valueobjects.Keywords
	categoryIDs []valueobjects.CategoryID
	isArchived  bool
	metadata    map[string]interface{}

	// Connections
	connections map[string]float64 // targetNodeID -> weight
}

// NewNodeAggregate creates a new node aggregate
func NewNodeAggregate() *NodeAggregate {
	return &NodeAggregate{
		BaseAggregate: *NewBaseAggregate(""),
		connections:   make(map[string]float64),
		metadata:      make(map[string]interface{}),
	}
}

// CreateNode creates a new node with the given parameters
func (n *NodeAggregate) CreateNode(
	id valueobjects.NodeID,
	userID valueobjects.UserID,
	content valueobjects.Content,
	title valueobjects.Title,
	tags valueobjects.Tags,
	keywords valueobjects.Keywords,
) error {
	if n.ID != "" {
		return fmt.Errorf("node already exists")
	}

	event := &events.NodeCreatedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID:   id.String(),
			AggregateType: "Node",
			EventType:     "NodeCreated",
			Version:       1,
			Timestamp:     time.Now(),
			Metadata: events.EventMetadata{
				UserID: userID.String(),
			},
		},
		UserID:   userID.String(),
		Content:  content.String(),
		Title:    title.String(),
		Tags:     tags.Values(),
		Keywords: keywords.Values(),
	}

	return n.Apply(event)
}

// UpdateContent updates the node content
func (n *NodeAggregate) UpdateContent(content valueobjects.Content) error {
	if n.isArchived {
		return fmt.Errorf("cannot update archived node")
	}

	if n.content.Equals(content) {
		return nil // No change
	}

	event := &events.NodeUpdatedEvent{
		BaseEvent: *events.NewBaseEvent(n.ID, "Node", "NodeUpdated", n.Version+1),
		Content:   content.String(),
	}

	return n.Apply(event)
}

// UpdateTitle updates the node title
func (n *NodeAggregate) UpdateTitle(title valueobjects.Title) error {
	if n.isArchived {
		return fmt.Errorf("cannot update archived node")
	}

	if n.title.Equals(title) {
		return nil // No change
	}

	event := &events.NodeUpdatedEvent{
		BaseEvent: *events.NewBaseEvent(n.ID, "Node", "NodeUpdated", n.Version+1),
		Title:     title.String(),
	}

	return n.Apply(event)
}

// Archive archives the node
func (n *NodeAggregate) Archive(reason string) error {
	if n.isArchived {
		return nil // Already archived
	}

	event := events.NewNodeArchivedEvent(n.ID, n.Version+1, reason)
	return n.Apply(event)
}

// Restore restores an archived node
func (n *NodeAggregate) Restore() error {
	if !n.isArchived {
		return nil // Not archived
	}

	event := events.NewNodeRestoredEvent(n.ID, n.Version+1)
	return n.Apply(event)
}

// Connect creates a connection to another node
func (n *NodeAggregate) Connect(targetNodeID string, weight float64) error {
	if n.isArchived {
		return fmt.Errorf("cannot connect archived node")
	}

	if weight < 0 || weight > 1 {
		return fmt.Errorf("weight must be between 0 and 1")
	}

	event := events.NewNodeConnectedEvent(n.ID, targetNodeID, n.Version+1, weight)
	return n.Apply(event)
}

// Disconnect removes a connection to another node
func (n *NodeAggregate) Disconnect(targetNodeID string) error {
	if _, exists := n.connections[targetNodeID]; !exists {
		return fmt.Errorf("connection does not exist")
	}

	event := events.NewNodeDisconnectedEvent(n.ID, targetNodeID, n.Version+1)
	return n.Apply(event)
}

// Apply applies an event to the aggregate
func (n *NodeAggregate) Apply(event events.DomainEvent) error {
	switch e := event.(type) {
	case *events.NodeCreatedEvent:
		return n.applyNodeCreated(e)
	case *events.NodeUpdatedEvent:
		return n.applyNodeUpdated(e)
	case *events.NodeArchivedEvent:
		return n.applyNodeArchived(e)
	case *events.NodeRestoredEvent:
		return n.applyNodeRestored(e)
	case *events.NodeConnectedEvent:
		return n.applyNodeConnected(e)
	case *events.NodeDisconnectedEvent:
		return n.applyNodeDisconnected(e)
	default:
		return fmt.Errorf("unknown event type: %T", event)
	}
}

// Event handlers

func (n *NodeAggregate) applyNodeCreated(e *events.NodeCreatedEvent) error {
	n.ID = e.AggregateID
	n.userID = valueobjects.NewUserID(e.UserID)
	n.content = valueobjects.NewContent(e.Content)
	n.title = valueobjects.NewTitle(e.Title)
	n.tags = valueobjects.NewTags(e.Tags)
	n.keywords = valueobjects.NewKeywords(e.Keywords)
	n.Version = e.Version
	n.CreatedAt = e.Timestamp
	n.UpdatedAt = e.Timestamp
	
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

func (n *NodeAggregate) applyNodeUpdated(e *events.NodeUpdatedEvent) error {
	if e.Content != "" {
		n.content = valueobjects.NewContent(e.Content)
	}
	if e.Title != "" {
		n.title = valueobjects.NewTitle(e.Title)
	}
	if len(e.Tags) > 0 {
		n.tags = valueobjects.NewTags(e.Tags)
	}
	if len(e.Keywords) > 0 {
		n.keywords = valueobjects.NewKeywords(e.Keywords)
	}
	
	n.Version = e.Version
	n.UpdatedAt = e.Timestamp
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

func (n *NodeAggregate) applyNodeArchived(e *events.NodeArchivedEvent) error {
	n.isArchived = true
	n.Version = e.Version
	n.UpdatedAt = e.Timestamp
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

func (n *NodeAggregate) applyNodeRestored(e *events.NodeRestoredEvent) error {
	n.isArchived = false
	n.Version = e.Version
	n.UpdatedAt = e.Timestamp
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

func (n *NodeAggregate) applyNodeConnected(e *events.NodeConnectedEvent) error {
	n.connections[e.TargetNodeID] = e.Weight
	n.Version = e.Version
	n.UpdatedAt = e.Timestamp
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

func (n *NodeAggregate) applyNodeDisconnected(e *events.NodeDisconnectedEvent) error {
	delete(n.connections, e.TargetNodeID)
	n.Version = e.Version
	n.UpdatedAt = e.Timestamp
	n.BaseAggregate.addUncommittedEvent(e)
	return nil
}

// Getters

// GetRoot returns the aggregate root (for compatibility)
func (n *NodeAggregate) GetRoot() interface{} {
	return n
}

// GetUserID returns the user ID
func (n *NodeAggregate) GetUserID() valueobjects.UserID {
	return n.userID
}

// GetContent returns the content
func (n *NodeAggregate) GetContent() valueobjects.Content {
	return n.content
}

// GetTitle returns the title
func (n *NodeAggregate) GetTitle() valueobjects.Title {
	return n.title
}

// GetTags returns the tags
func (n *NodeAggregate) GetTags() valueobjects.Tags {
	return n.tags
}

// GetKeywords returns the keywords
func (n *NodeAggregate) GetKeywords() valueobjects.Keywords {
	return n.keywords
}

// IsArchived returns whether the node is archived
func (n *NodeAggregate) IsArchived() bool {
	return n.isArchived
}

// GetConnections returns the node connections
func (n *NodeAggregate) GetConnections() map[string]float64 {
	// Return a copy to prevent external modification
	result := make(map[string]float64)
	for k, v := range n.connections {
		result[k] = v
	}
	return result
}

// SetVersion sets the aggregate version (for testing)
func (n *NodeAggregate) SetVersion(version int64) {
	n.Version = version
}

// IncrementVersion increments the version
func (n *NodeAggregate) IncrementVersion() {
	n.Version++
}