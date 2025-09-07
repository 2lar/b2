package entities

import (
	"errors"
	"time"

	"backend2/domain/core/valueobjects"
	"backend2/domain/events"
)

// NodeStatus represents the state of a node
type NodeStatus string

const (
	StatusDraft     NodeStatus = "draft"
	StatusPublished NodeStatus = "published"
	StatusArchived  NodeStatus = "archived"
)

// Node is the main entity representing a knowledge unit
// This is a rich domain model with encapsulated business logic
type Node struct {
	// Private fields ensure encapsulation
	id         valueobjects.NodeID
	userID     string
	graphID    string  // ID of the graph this node belongs to
	content    valueobjects.NodeContent
	position   valueobjects.Position
	metadata   Metadata
	edges      []EdgeReference
	createdAt  time.Time
	updatedAt  time.Time
	version    int
	status     NodeStatus
	
	// Domain events that occurred during this aggregate's lifetime
	events     []events.DomainEvent
}

// EdgeReference is a lightweight reference to connected edges
type EdgeReference struct {
	EdgeID   string
	TargetID valueobjects.NodeID
	Type     EdgeType
}

// EdgeType defines the type of relationship
type EdgeType string

const (
	EdgeTypeReference   EdgeType = "reference"
	EdgeTypeParentChild EdgeType = "parent_child"
	EdgeTypeSimilar     EdgeType = "similar"
	EdgeTypeSequential  EdgeType = "sequential"
)

// Metadata contains additional node information
type Metadata struct {
	Tags       []string
	Categories []string
	Color      string
	Icon       string
	Priority   int
	Custom     map[string]interface{}
}

// NewNode creates a new node with full business rule validation
func NewNode(userID string, content valueobjects.NodeContent, position valueobjects.Position) (*Node, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	
	if content.IsEmpty() {
		return nil, errors.New("content cannot be empty")
	}
	
	now := time.Now()
	node := &Node{
		id:        valueobjects.NewNodeID(),
		userID:    userID,
		content:   content,
		position:  position,
		metadata:  Metadata{Custom: make(map[string]interface{})},
		edges:     []EdgeReference{},
		createdAt: now,
		updatedAt: now,
		version:   1,
		status:    StatusDraft,
		events:    []events.DomainEvent{},
	}
	
	node.addEvent(events.NewNodeCreated(node.id, userID, now))
	
	return node, nil
}

// ReconstructNode reconstructs a node from repository data with preserved timestamps
func ReconstructNode(
	id valueobjects.NodeID,
	userID string,
	content valueobjects.NodeContent,
	position valueobjects.Position,
	graphID string,
	createdAt, updatedAt time.Time,
	status NodeStatus,
) (*Node, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	
	if content.IsEmpty() {
		return nil, errors.New("content cannot be empty")
	}
	
	node := &Node{
		id:        id,
		userID:    userID,
		graphID:   graphID,
		content:   content,
		position:  position,
		metadata:  Metadata{},
		edges:     []EdgeReference{},
		createdAt: createdAt,
		updatedAt: updatedAt,
		version:   1,
		status:    status,
		events:    []events.DomainEvent{},
	}
	
	return node, nil
}

// ID returns the node's unique identifier
func (n *Node) ID() valueobjects.NodeID {
	return n.id
}

// UserID returns the owner's ID
func (n *Node) UserID() string {
	return n.userID
}

// Content returns the node's content
func (n *Node) Content() valueobjects.NodeContent {
	return n.content
}

// Position returns the node's position
func (n *Node) Position() valueobjects.Position {
	return n.position
}

// Status returns the node's current status
func (n *Node) Status() NodeStatus {
	return n.status
}

// Version returns the node's version for optimistic locking
func (n *Node) Version() int {
	return n.version
}

// GraphID returns the ID of the graph this node belongs to
func (n *Node) GraphID() string {
	return n.graphID
}

// SetGraphID sets the graph ID for this node
func (n *Node) SetGraphID(graphID string) {
	n.graphID = graphID
	n.updatedAt = time.Now()
}

// UpdateContent updates the node's content with validation
func (n *Node) UpdateContent(content valueobjects.NodeContent) error {
	if n.status == StatusArchived {
		return errors.New("cannot update archived node")
	}
	
	if content.IsEmpty() {
		return errors.New("content cannot be empty")
	}
	
	if content.Equals(n.content) {
		return nil // No change needed
	}
	
	oldContent := n.content
	n.content = content
	n.updatedAt = time.Now()
	n.version++
	
	n.addEvent(events.NewNodeContentUpdated(n.id, oldContent, content, n.updatedAt))
	
	return nil
}

// MoveTo moves the node to a new position
func (n *Node) MoveTo(position valueobjects.Position) error {
	if n.status == StatusArchived {
		return errors.New("cannot move archived node")
	}
	
	if position.Equals(n.position) {
		return nil // No movement needed
	}
	
	oldPosition := n.position
	n.position = position
	n.updatedAt = time.Now()
	
	n.addEvent(events.NewNodeMoved(n.id, oldPosition, position, n.updatedAt))
	
	return nil
}

// ConnectTo creates a connection to another node
func (n *Node) ConnectTo(targetID valueobjects.NodeID, edgeType EdgeType) error {
	// Check for self-reference
	if n.id.Equals(targetID) {
		return errors.New("cannot connect node to itself")
	}
	
	// Check for duplicate connection
	for _, edge := range n.edges {
		if edge.TargetID.Equals(targetID) && edge.Type == edgeType {
			return errors.New("connection already exists")
		}
	}
	
	// Check connection limit (business rule)
	const maxConnections = 50
	if len(n.edges) >= maxConnections {
		return errors.New("maximum connections reached")
	}
	
	edgeRef := EdgeReference{
		EdgeID:   generateEdgeID(),
		TargetID: targetID,
		Type:     edgeType,
	}
	
	n.edges = append(n.edges, edgeRef)
	n.updatedAt = time.Now()
	
	n.addEvent(events.NewNodesConnected(n.id, targetID, string(edgeType), n.updatedAt))
	
	return nil
}

// Disconnect removes a connection to another node
func (n *Node) Disconnect(targetID valueobjects.NodeID) error {
	found := false
	newEdges := []EdgeReference{}
	
	for _, edge := range n.edges {
		if !edge.TargetID.Equals(targetID) {
			newEdges = append(newEdges, edge)
		} else {
			found = true
		}
	}
	
	if !found {
		return errors.New("connection not found")
	}
	
	n.edges = newEdges
	n.updatedAt = time.Now()
	
	n.addEvent(events.NewNodesDisconnected(n.id, targetID, n.updatedAt))
	
	return nil
}

// Publish changes the node status to published
func (n *Node) Publish() error {
	if n.status == StatusArchived {
		return errors.New("cannot publish archived node")
	}
	
	if n.status == StatusPublished {
		return nil // Already published
	}
	
	n.status = StatusPublished
	n.updatedAt = time.Now()
	n.version++
	
	n.addEvent(events.NewNodePublished(n.id, n.updatedAt))
	
	return nil
}

// Archive moves the node to archived status
func (n *Node) Archive() error {
	if n.status == StatusArchived {
		return nil // Already archived
	}
	
	n.status = StatusArchived
	n.updatedAt = time.Now()
	n.version++
	
	// Remove all connections when archiving
	n.edges = []EdgeReference{}
	
	n.addEvent(events.NewNodeArchived(n.id, n.updatedAt))
	
	return nil
}

// AddTag adds a tag to the node
func (n *Node) AddTag(tag string) error {
	if tag == "" {
		return errors.New("tag cannot be empty")
	}
	
	// Check for duplicate
	for _, t := range n.metadata.Tags {
		if t == tag {
			return nil // Tag already exists
		}
	}
	
	// Check tag limit
	const maxTags = 20
	if len(n.metadata.Tags) >= maxTags {
		return errors.New("maximum tags reached")
	}
	
	n.metadata.Tags = append(n.metadata.Tags, tag)
	n.updatedAt = time.Now()
	
	return nil
}

// RemoveTag removes a tag from the node
func (n *Node) RemoveTag(tag string) error {
	newTags := []string{}
	found := false
	
	for _, t := range n.metadata.Tags {
		if t != tag {
			newTags = append(newTags, t)
		} else {
			found = true
		}
	}
	
	if !found {
		return errors.New("tag not found")
	}
	
	n.metadata.Tags = newTags
	n.updatedAt = time.Now()
	
	return nil
}

// GetConnections returns all edge references
func (n *Node) GetConnections() []EdgeReference {
	// Return a copy to maintain encapsulation
	edges := make([]EdgeReference, len(n.edges))
	copy(edges, n.edges)
	return edges
}

// GetTags returns all tags
func (n *Node) GetTags() []string {
	// Return a copy to maintain encapsulation
	tags := make([]string, len(n.metadata.Tags))
	copy(tags, n.metadata.Tags)
	return tags
}

// CreatedAt returns when the node was created
func (n *Node) CreatedAt() time.Time {
	return n.createdAt
}

// UpdatedAt returns when the node was last updated
func (n *Node) UpdatedAt() time.Time {
	return n.updatedAt
}

// GetUncommittedEvents returns all uncommitted domain events
func (n *Node) GetUncommittedEvents() []events.DomainEvent {
	return n.events
}

// MarkEventsAsCommitted clears the uncommitted events
func (n *Node) MarkEventsAsCommitted() {
	n.events = []events.DomainEvent{}
}

// addEvent adds a domain event to the uncommitted list
func (n *Node) addEvent(event events.DomainEvent) {
	n.events = append(n.events, event)
}

// generateEdgeID generates a unique edge ID
func generateEdgeID() string {
	return valueobjects.NewNodeID().String() // Reuse UUID generation
}