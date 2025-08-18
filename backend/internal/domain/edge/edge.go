package edge

import (
	"time"

	"brain2-backend/internal/domain/shared"
)

// EdgeType represents the type of relationship between nodes
type EdgeType string

const (
	EdgeTypeRelated   EdgeType = "related"
	EdgeTypeSimilar   EdgeType = "similar"
	EdgeTypeReference EdgeType = "reference"
)

// Edge represents a directed relationship between two memory nodes.
// This is a rich domain model that encapsulates business logic for node connections.
//
// Key Design Principles Demonstrated:
//   - Rich Domain Model: Contains behavior and validation logic
//   - Value Objects: Uses strongly-typed IDs instead of strings
//   - Business Invariants: Ensures edges are always valid
//   - Domain Events: Tracks edge creation and deletion
//   - Immutability: Once created, core edge properties cannot change
type Edge struct {
	// Private fields for encapsulation
	id       shared.NodeID    // Unique identifier for this edge
	sourceID shared.NodeID    // Source node of the relationship
	targetID shared.NodeID    // Target node of the relationship
	userID   shared.UserID    // Owner of both nodes (enforced business rule)
	weight   float64   // Strength of the connection (0.0 to 1.0)
	createdAt time.Time // When the edge was created
	version   shared.Version   // For optimistic locking
	
	// Public fields for compatibility
	ID        shared.NodeID    `json:"id"`
	SourceID  shared.NodeID    `json:"source_id"`
	TargetID  shared.NodeID    `json:"target_id"`
	EdgeType  EdgeType  `json:"edge_type"`
	Strength  float64   `json:"strength"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
	
	// Domain events
	events []shared.DomainEvent
}

// NewEdge creates a new edge between two nodes with validation.
//
// Business Rules Enforced:
//   - Source and target must be different nodes
//   - Weight must be between 0.0 and 1.0
//   - Domain events are generated for creation
func NewEdge(sourceID, targetID shared.NodeID, userID shared.UserID, weight float64) (*Edge, error) {
	// Validate business rules
	if sourceID.Equals(targetID) {
		return nil, shared.ErrInvalidEdge
	}
	
	if weight < 0.0 || weight > 1.0 {
		return nil, shared.NewValidationError("weight", "weight must be between 0.0 and 1.0", weight)
	}
	
	now := time.Now()
	edgeID := shared.NewNodeID() // Reuse shared.NodeID generator for edge IDs
	
	edge := &Edge{
		// Private fields
		id:        edgeID,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weight,
		createdAt: now,
		version:   shared.NewVersion(),
		events:    []shared.DomainEvent{},
		// Public fields (for compatibility)
		ID:        edgeID,
		SourceID:  sourceID,
		TargetID:  targetID,
		Strength:  weight,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   0,
	}
	
	// Generate domain event
	event := shared.NewEdgeCreatedEvent(edgeID, sourceID, targetID, userID, weight, edge.version)
	edge.addEvent(event)
	
	return edge, nil
}

// ReconstructEdge creates an edge from persistence (no events generated)
func ReconstructEdge(id, sourceID, targetID shared.NodeID, userID shared.UserID, weight float64, createdAt time.Time, version shared.Version) *Edge {
	return &Edge{
		// Private fields
		id:        id,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weight,
		createdAt: createdAt,
		version:   version,
		events:    []shared.DomainEvent{},
		// Public fields (for compatibility)
		ID:        id,
		SourceID:  sourceID,
		TargetID:  targetID,
		Strength:  weight,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Version:   version.Int(),
	}
}

// ReconstructEdgeFromPrimitives creates an edge from primitive types (for repository layer)
func ReconstructEdgeFromPrimitives(sourceIDStr, targetIDStr, userIDStr string, weight float64) (*Edge, error) {
	sourceID, err := shared.ParseNodeID(sourceIDStr)
	if err != nil {
		return nil, err
	}
	
	targetID, err := shared.ParseNodeID(targetIDStr)
	if err != nil {
		return nil, err
	}
	
	userID, err := shared.NewUserID(userIDStr)
	if err != nil {
		return nil, err
	}
	
	return NewEdge(sourceID, targetID, userID, weight)
}

// Getters (read-only access to internal state)

// UserID returns the user who owns this edge
func (e *Edge) UserID() shared.UserID {
	return e.userID
}

// Weight returns the connection strength
func (e *Edge) Weight() float64 {
	return e.weight
}

// Business Methods

// IsReverse checks if this edge is the reverse of another edge
func (e *Edge) IsReverse(other *Edge) bool {
	return e.sourceID.Equals(other.targetID) && e.targetID.Equals(other.sourceID)
}

// ConnectsNodes checks if this edge connects two specific nodes (in either direction)
func (e *Edge) ConnectsNodes(node1ID, node2ID shared.NodeID) bool {
	return (e.sourceID.Equals(node1ID) && e.targetID.Equals(node2ID)) ||
		   (e.sourceID.Equals(node2ID) && e.targetID.Equals(node1ID))
}

// HasNode checks if this edge involves a specific node
func (e *Edge) HasNode(nodeID shared.NodeID) bool {
	return e.sourceID.Equals(nodeID) || e.targetID.Equals(nodeID)
}

// IsStrongConnection checks if this is a strong connection based on weight
func (e *Edge) IsStrongConnection() bool {
	return e.weight >= 0.7 // 70% similarity threshold for strong connections
}

// IsWeakConnection checks if this is a weak connection based on weight
func (e *Edge) IsWeakConnection() bool {
	return e.weight < 0.3 // Below 30% similarity is considered weak
}

// CalculateReciprocalWeight calculates what the weight should be for the reverse edge
func (e *Edge) CalculateReciprocalWeight() float64 {
	// For now, return the same weight, but this could implement more complex logic
	// based on the directionality of the relationship
	return e.weight
}

// Delete marks this edge for deletion and generates appropriate events
func (e *Edge) Delete() {
	// Generate domain event for edge deletion
	event := shared.NewEdgeDeletedEvent(e.id, e.sourceID, e.targetID, e.userID, e.weight, e.version)
	e.addEvent(event)
	
	// Increment version for optimistic locking
	e.version = e.version.Next()
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (e *Edge) GetUncommittedEvents() []shared.DomainEvent {
	return e.events
}

// MarkEventsAsCommitted clears the events after persistence
func (e *Edge) MarkEventsAsCommitted() {
	e.events = []shared.DomainEvent{}
}

// Private helper methods

// addEvent adds a domain event to the uncommitted events list
func (e *Edge) addEvent(event shared.DomainEvent) {
	e.events = append(e.events, event)
}

