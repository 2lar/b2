package domain

import (
	"time"
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
	id       NodeID    // Unique identifier for this edge
	sourceID NodeID    // Source node of the relationship
	targetID NodeID    // Target node of the relationship
	userID   UserID    // Owner of both nodes (enforced business rule)
	weight   float64   // Strength of the connection (0.0 to 1.0)
	createdAt time.Time // When the edge was created
	version   Version   // For optimistic locking
	
	// Domain events
	events []DomainEvent
}

// NewEdge creates a new edge between two nodes with validation.
//
// Business Rules Enforced:
//   - Source and target must be different nodes
//   - Weight must be between 0.0 and 1.0
//   - Domain events are generated for creation
func NewEdge(sourceID, targetID NodeID, userID UserID, weight float64) (*Edge, error) {
	// Validate business rules
	if sourceID.Equals(targetID) {
		return nil, ErrInvalidEdge
	}
	
	if weight < 0.0 || weight > 1.0 {
		return nil, NewValidationError("weight", "weight must be between 0.0 and 1.0", weight)
	}
	
	now := time.Now()
	edgeID := NewNodeID() // Reuse NodeID generator for edge IDs
	
	edge := &Edge{
		id:        edgeID,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weight,
		createdAt: now,
		version:   NewVersion(),
		events:    []DomainEvent{},
	}
	
	// Generate domain event
	event := NewEdgeCreatedEvent(edgeID, sourceID, targetID, userID, weight, edge.version)
	edge.addEvent(event)
	
	return edge, nil
}

// ReconstructEdge creates an edge from persistence (no events generated)
func ReconstructEdge(id, sourceID, targetID NodeID, userID UserID, weight float64, createdAt time.Time, version Version) *Edge {
	return &Edge{
		id:        id,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weight,
		createdAt: createdAt,
		version:   version,
		events:    []DomainEvent{},
	}
}

// ReconstructEdgeFromPrimitives creates an edge from primitive types (for repository layer)
func ReconstructEdgeFromPrimitives(sourceIDStr, targetIDStr, userIDStr string, weight float64) (*Edge, error) {
	sourceID, err := ParseNodeID(sourceIDStr)
	if err != nil {
		return nil, err
	}
	
	targetID, err := ParseNodeID(targetIDStr)
	if err != nil {
		return nil, err
	}
	
	userID, err := NewUserID(userIDStr)
	if err != nil {
		return nil, err
	}
	
	return NewEdge(sourceID, targetID, userID, weight)
}

// Getters (read-only access to internal state)

// ID returns the edge's unique identifier
func (e *Edge) ID() NodeID {
	return e.id
}

// SourceID returns the source node identifier
func (e *Edge) SourceID() NodeID {
	return e.sourceID
}

// TargetID returns the target node identifier
func (e *Edge) TargetID() NodeID {
	return e.targetID
}

// UserID returns the user who owns this edge
func (e *Edge) UserID() UserID {
	return e.userID
}

// Weight returns the connection strength
func (e *Edge) Weight() float64 {
	return e.weight
}

// CreatedAt returns when the edge was created
func (e *Edge) CreatedAt() time.Time {
	return e.createdAt
}

// Version returns the current version for optimistic locking
func (e *Edge) Version() Version {
	return e.version
}

// Business Methods

// IsReverse checks if this edge is the reverse of another edge
func (e *Edge) IsReverse(other *Edge) bool {
	return e.sourceID.Equals(other.targetID) && e.targetID.Equals(other.sourceID)
}

// ConnectsNodes checks if this edge connects two specific nodes (in either direction)
func (e *Edge) ConnectsNodes(node1ID, node2ID NodeID) bool {
	return (e.sourceID.Equals(node1ID) && e.targetID.Equals(node2ID)) ||
		   (e.sourceID.Equals(node2ID) && e.targetID.Equals(node1ID))
}

// HasNode checks if this edge involves a specific node
func (e *Edge) HasNode(nodeID NodeID) bool {
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
	event := NewEdgeDeletedEvent(e.id, e.sourceID, e.targetID, e.userID, e.weight, e.version)
	e.addEvent(event)
	
	// Increment version for optimistic locking
	e.version = e.version.Next()
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (e *Edge) GetUncommittedEvents() []DomainEvent {
	return e.events
}

// MarkEventsAsCommitted clears the events after persistence
func (e *Edge) MarkEventsAsCommitted() {
	e.events = []DomainEvent{}
}

// Private helper methods

// addEvent adds a domain event to the uncommitted events list
func (e *Edge) addEvent(event DomainEvent) {
	e.events = append(e.events, event)
}


// EdgeWeightCalculator is a value object for calculating edge weights based on node similarity
type EdgeWeightCalculator struct {
	keywordWeight float64
	tagWeight     float64
	recencyWeight float64
}

// NewEdgeWeightCalculator creates a new weight calculator with specified weights
func NewEdgeWeightCalculator(keywordWeight, tagWeight, recencyWeight float64) EdgeWeightCalculator {
	return EdgeWeightCalculator{
		keywordWeight: keywordWeight,
		tagWeight:     tagWeight,
		recencyWeight: recencyWeight,
	}
}

// CalculateWeight calculates the edge weight between two nodes
func (calc EdgeWeightCalculator) CalculateWeight(source, target *Node) float64 {
	keywordSimilarity := source.Keywords().Overlap(target.Keywords())
	tagSimilarity := source.Tags().Overlap(target.Tags())
	
	// Calculate recency factor (more recent connections get higher weight)
	timeDiff := source.CreatedAt().Sub(target.CreatedAt())
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	
	// Recency score decreases as time difference increases
	daysDiff := timeDiff.Hours() / 24
	recencyScore := 1.0 / (1.0 + daysDiff*0.1)
	
	// Weighted combination
	weight := keywordSimilarity*calc.keywordWeight + 
			  tagSimilarity*calc.tagWeight + 
			  recencyScore*calc.recencyWeight
	
	// Ensure weight is within valid range
	if weight > 1.0 {
		weight = 1.0
	}
	if weight < 0.0 {
		weight = 0.0
	}
	
	return weight
}
