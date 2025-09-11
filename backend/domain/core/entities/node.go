package entities

import (
	"fmt"
	"strings"
	"time"

	"backend/domain/config"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	pkgerrors "backend/pkg/errors"
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
	id        valueobjects.NodeID
	userID    string
	graphID   string // ID of the graph this node belongs to
	content   valueobjects.NodeContent
	position  valueobjects.Position
	metadata  Metadata
	edges     []EdgeReference
	createdAt time.Time
	updatedAt time.Time
	version   int
	status    NodeStatus

	// Domain events that occurred during this aggregate's lifetime
	events []events.DomainEvent
}

// EdgeReference is a lightweight reference to connected edges
type EdgeReference struct {
	EdgeID   string
	TargetID valueobjects.NodeID
	Type     EdgeType
}

// EdgeType is now defined in edge_types.go

// Metadata contains additional node information
type Metadata struct {
	Tags       []string
	Categories []string
	Category   string                 // Single category for backward compatibility
	URL        string                 // URL if this node represents a web resource
	Color      string
	Icon       string
	Priority   int
	Properties map[string]interface{} // Additional custom properties
}

// NewNode creates a new node with full business rule validation
func NewNode(userID string, content valueobjects.NodeContent, position valueobjects.Position) (*Node, error) {
	if userID == "" {
		return nil, pkgerrors.NewValidationError("userID cannot be empty")
	}

	if content.IsEmpty() {
		return nil, pkgerrors.NewValidationError("content cannot be empty")
	}

	now := time.Now()
	node := &Node{
		id:        valueobjects.NewNodeID(),
		userID:    userID,
		content:   content,
		position:  position,
		metadata:  Metadata{
			Properties: make(map[string]interface{}),
		},
		edges:     []EdgeReference{},
		createdAt: now,
		updatedAt: now,
		version:   1,
		status:    StatusDraft,
		events:    []events.DomainEvent{},
	}

	// Extract keywords for the event (will be used by connect-node Lambda)
	keywords := extractKeywords(content.Title() + " " + content.Body())

	// Note: graphID will be set later when node is added to a graph
	// Tags will be populated when AddTag is called
	node.addEvent(events.NewNodeCreated(
		node.id,
		userID,
		"", // graphID will be set when SetGraphID is called
		content.Title(),
		content.Body(),
		keywords,
		[]string{}, // tags will be populated when AddTag is called
		now,
	))

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
		return nil, pkgerrors.NewValidationError("userID cannot be empty")
	}

	if content.IsEmpty() {
		return nil, pkgerrors.NewValidationError("content cannot be empty")
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

	// Update the NodeCreated event with the graph ID
	// This is important for the connect-node Lambda to know which graph to work with
	for i, event := range n.events {
		if nodeCreated, ok := event.(events.NodeCreated); ok {
			// Update the event with the graph ID
			nodeCreated.GraphID = graphID
			nodeCreated.Tags = n.GetTags() // Also update tags in case they were added
			n.events[i] = nodeCreated
			break
		}
	}
}

// UpdateContent updates the node's content with validation
func (n *Node) UpdateContent(content valueobjects.NodeContent) error {
	if n.status == StatusArchived {
		return pkgerrors.NewValidationError("cannot update archived node")
	}

	if content.IsEmpty() {
		return pkgerrors.NewValidationError("content cannot be empty")
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
		return pkgerrors.NewValidationError("cannot move archived node")
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
	return n.ConnectToWithConfig(targetID, edgeType, config.DefaultDomainConfig())
}

// ConnectToWithConfig creates a connection to another node with configuration
func (n *Node) ConnectToWithConfig(targetID valueobjects.NodeID, edgeType EdgeType, cfg *config.DomainConfig) error {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}

	// Check for self-reference
	if !cfg.AllowSelfConnections && n.id.Equals(targetID) {
		return pkgerrors.NewValidationError("cannot connect node to itself")
	}

	// Check for duplicate connection
	if !cfg.AllowDuplicateEdges {
		for _, edge := range n.edges {
			if edge.TargetID.Equals(targetID) && edge.Type == edgeType {
				return pkgerrors.NewConflictError("connection already exists")
			}
		}
	}

	// Check connection limit (business rule)
	if len(n.edges) >= cfg.MaxConnectionsPerNode {
		return fmt.Errorf("maximum connections reached: %d", cfg.MaxConnectionsPerNode)
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
		return pkgerrors.NewNotFoundError("connection")
	}

	n.edges = newEdges
	n.updatedAt = time.Now()

	n.addEvent(events.NewNodesDisconnected(n.id, targetID, n.updatedAt))

	return nil
}

// Publish changes the node status to published
func (n *Node) Publish() error {
	if n.status == StatusArchived {
		return pkgerrors.NewValidationError("cannot publish archived node")
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
	return n.AddTagWithConfig(tag, config.DefaultDomainConfig())
}

// AddTagWithConfig adds a tag to the node with configuration
func (n *Node) AddTagWithConfig(tag string, cfg *config.DomainConfig) error {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}

	if tag == "" {
		return pkgerrors.NewValidationError("tag cannot be empty")
	}

	// Check for duplicate
	for _, t := range n.metadata.Tags {
		if t == tag {
			return nil // Tag already exists
		}
	}

	// Check tag limit
	if len(n.metadata.Tags) >= cfg.MaxTagsPerNode {
		return fmt.Errorf("maximum tags reached: %d", cfg.MaxTagsPerNode)
	}

	n.metadata.Tags = append(n.metadata.Tags, tag)
	n.updatedAt = time.Now()

	// Update the NodeCreated event with the new tags
	for i, event := range n.events {
		if nodeCreated, ok := event.(events.NodeCreated); ok {
			nodeCreated.Tags = n.GetTags()
			n.events[i] = nodeCreated
			break
		}
	}

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
		return pkgerrors.NewNotFoundError("tag")
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

// GetMetadata returns the node's metadata as a map
func (n *Node) GetMetadata() map[string]interface{} {
	// Convert Metadata struct to map for external use
	result := make(map[string]interface{})
	result["tags"] = n.GetTags()
	if n.metadata.Category != "" {
		result["category"] = n.metadata.Category
	}
	if n.metadata.URL != "" {
		result["url"] = n.metadata.URL
	}
	if n.metadata.Color != "" {
		result["color"] = n.metadata.Color
	}
	// Add any custom properties
	for k, v := range n.metadata.Properties {
		result[k] = v
	}
	return result
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

// extractKeywords extracts significant words from text for similarity matching
func extractKeywords(text string) []string {
	// Simple keyword extraction - in production, use NLP
	words := strings.Fields(strings.ToLower(text))
	keywords := []string{}

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
	}

	seen := make(map[string]bool)
	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,!?;:\"'()[]{}")

		// Skip short words, stop words, and duplicates
		if len(word) > 3 && !stopWords[word] && !seen[word] {
			keywords = append(keywords, word)
			seen[word] = true
		}
	}

	return keywords
}

// Business Methods for Rich Domain Model

// IsSimilarTo checks if this node is similar to another based on content
func (n *Node) IsSimilarTo(other *Node, threshold float64) bool {
	if other == nil || n.id.Equals(other.id) {
		return false
	}
	
	// Simple similarity check based on keyword overlap
	myKeywords := extractKeywords(n.content.Title() + " " + n.content.Body())
	otherKeywords := extractKeywords(other.content.Title() + " " + other.content.Body())
	
	if len(myKeywords) == 0 || len(otherKeywords) == 0 {
		return false
	}
	
	// Calculate Jaccard similarity
	intersection := 0
	keywordSet := make(map[string]bool)
	for _, kw := range myKeywords {
		keywordSet[kw] = true
	}
	
	for _, kw := range otherKeywords {
		if keywordSet[kw] {
			intersection++
		}
	}
	
	union := len(myKeywords) + len(otherKeywords) - intersection
	if union == 0 {
		return false
	}
	
	similarity := float64(intersection) / float64(union)
	return similarity >= threshold
}

// CanConnectTo validates if this node can connect to another
func (n *Node) CanConnectTo(targetID valueobjects.NodeID, cfg *config.DomainConfig) error {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}
	
	// Check status
	if n.status == StatusArchived {
		return pkgerrors.NewValidationError("cannot connect from archived node")
	}
	
	// Check for self-reference
	if !cfg.AllowSelfConnections && n.id.Equals(targetID) {
		return pkgerrors.NewValidationError("cannot connect node to itself")
	}
	
	// Check connection limit
	if len(n.edges) >= cfg.MaxConnectionsPerNode {
		return fmt.Errorf("maximum connections reached: %d", cfg.MaxConnectionsPerNode)
	}
	
	return nil
}

// HasConnectionTo checks if this node has a connection to the target
func (n *Node) HasConnectionTo(targetID valueobjects.NodeID) bool {
	for _, edge := range n.edges {
		if edge.TargetID.Equals(targetID) {
			return true
		}
	}
	return false
}

// GetConnectionType returns the edge type for a connection if it exists
func (n *Node) GetConnectionType(targetID valueobjects.NodeID) (EdgeType, bool) {
	for _, edge := range n.edges {
		if edge.TargetID.Equals(targetID) {
			return edge.Type, true
		}
	}
	return "", false
}

// IsPublished checks if the node is in published state
func (n *Node) IsPublished() bool {
	return n.status == StatusPublished
}

// IsArchived checks if the node is in archived state
func (n *Node) IsArchived() bool {
	return n.status == StatusArchived
}

// IsDraft checks if the node is in draft state
func (n *Node) IsDraft() bool {
	return n.status == StatusDraft
}

// SetMetadataProperty sets a custom property in the metadata
func (n *Node) SetMetadataProperty(key string, value interface{}) {
	if n.metadata.Properties == nil {
		n.metadata.Properties = make(map[string]interface{})
	}
	n.metadata.Properties[key] = value
	n.updatedAt = time.Now()
}

// GetMetadataProperty retrieves a custom property from metadata
func (n *Node) GetMetadataProperty(key string) (interface{}, bool) {
	if n.metadata.Properties == nil {
		return nil, false
	}
	val, exists := n.metadata.Properties[key]
	return val, exists
}

// SetURL sets the URL metadata for the node
func (n *Node) SetURL(url string) {
	n.metadata.URL = url
	n.updatedAt = time.Now()
}

// GetURL returns the URL metadata for the node
func (n *Node) GetURL() string {
	return n.metadata.URL
}

// SetColor sets the color metadata for visual representation
func (n *Node) SetColor(color string) {
	n.metadata.Color = color
	n.updatedAt = time.Now()
}

// GetColor returns the color metadata
func (n *Node) GetColor() string {
	return n.metadata.Color
}

// SetIcon sets the icon metadata for visual representation
func (n *Node) SetIcon(icon string) {
	n.metadata.Icon = icon
	n.updatedAt = time.Now()
}

// GetIcon returns the icon metadata
func (n *Node) GetIcon() string {
	return n.metadata.Icon
}

// SetPriority sets the priority of the node
func (n *Node) SetPriority(priority int) {
	n.metadata.Priority = priority
	n.updatedAt = time.Now()
}

// GetPriority returns the priority of the node
func (n *Node) GetPriority() int {
	return n.metadata.Priority
}

// AddCategory adds a category to the node
func (n *Node) AddCategory(category string) error {
	if category == "" {
		return pkgerrors.NewValidationError("category cannot be empty")
	}
	
	// Check for duplicate
	for _, c := range n.metadata.Categories {
		if c == category {
			return nil // Already exists
		}
	}
	
	n.metadata.Categories = append(n.metadata.Categories, category)
	n.updatedAt = time.Now()
	return nil
}

// RemoveCategory removes a category from the node
func (n *Node) RemoveCategory(category string) error {
	newCategories := []string{}
	found := false
	
	for _, c := range n.metadata.Categories {
		if c != category {
			newCategories = append(newCategories, c)
		} else {
			found = true
		}
	}
	
	if !found {
		return pkgerrors.NewNotFoundError("category")
	}
	
	n.metadata.Categories = newCategories
	n.updatedAt = time.Now()
	return nil
}

// GetCategories returns all categories
func (n *Node) GetCategories() []string {
	// Return a copy to maintain encapsulation
	categories := make([]string, len(n.metadata.Categories))
	copy(categories, n.metadata.Categories)
	return categories
}

// HasTag checks if the node has a specific tag
func (n *Node) HasTag(tag string) bool {
	for _, t := range n.metadata.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasCategory checks if the node has a specific category
func (n *Node) HasCategory(category string) bool {
	for _, c := range n.metadata.Categories {
		if c == category {
			return true
		}
	}
	// Also check the legacy single category field
	return n.metadata.Category == category
}
