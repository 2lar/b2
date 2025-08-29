// Package edge implements the Edge domain entity for the Brain2 knowledge graph.
//
// PURPOSE: Represents directed relationships between memory nodes (thoughts/ideas),
// enabling the system to build a connected knowledge graph where related concepts
// are automatically linked based on content similarity and user-defined relationships.
//
// DOMAIN ROLE: Edge is an Aggregate Root that encapsulates all business logic
// related to node connections, including relationship strength calculation,
// bi-directional link management, and connection validation rules.
//
// KEY FEATURES:
//   • Relationship Types: related, similar, reference connections
//   • Strength Calculation: Automatic weight assignment based on content overlap
//   • Business Rules: Prevents invalid connections (self-loops, cross-user links)
//   • Event Generation: Publishes domain events for graph updates and analytics
//
// The Edge entity works with the Node entity to form the core graph structure
// that powers Brain2's automatic knowledge discovery and visualization features.
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
//   - Value Objects: Uses strongly-typed IDs and Weight instead of primitives
//   - Business Invariants: Ensures edges are always valid
//   - Domain Events: Tracks edge creation, updates, and deletion
//   - Aggregate Root: Extends BaseAggregateRoot for consistency
type Edge struct {
	// Embedded base aggregate root for common functionality
	shared.BaseAggregateRoot
	
	// Private fields for encapsulation
	id       shared.NodeID    // Unique identifier for this edge
	sourceID shared.NodeID    // Source node of the relationship
	targetID shared.NodeID    // Target node of the relationship
	userID   shared.UserID    // Owner of both nodes (enforced business rule)
	weight   shared.Weight    // Strength of the connection (0.0 to 1.0)
	metadata shared.EdgeMetadata // Edge metadata for extensibility
	createdAt time.Time // When the edge was created
	updatedAt time.Time // When the edge was last updated
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
	
	// Create weight value object with validation
	weightVO, err := shared.NewWeight(weight)
	if err != nil {
		return nil, err
	}
	
	now := time.Now()
	edgeID := shared.NewNodeID() // Reuse shared.NodeID generator for edge IDs
	
	edge := &Edge{
		BaseAggregateRoot: shared.NewBaseAggregateRoot(edgeID.String()),
		// Private fields
		id:        edgeID,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weightVO,
		metadata:  shared.NewEdgeMetadata(),
		createdAt: now,
		updatedAt: now,
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
	weightVO, _ := shared.NewWeight(weight) // Weight already validated in DB
	return &Edge{
		BaseAggregateRoot: shared.NewBaseAggregateRoot(id.String()),
		// Private fields
		id:        id,
		sourceID:  sourceID,
		targetID:  targetID,
		userID:    userID,
		weight:    weightVO,
		metadata:  shared.NewEdgeMetadata(),
		createdAt: createdAt,
		updatedAt: createdAt,
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

// GetEdgeID returns the edge identifier (internal)
func (e *Edge) GetEdgeID() shared.NodeID {
	return e.id
}

// Source returns the source node ID
func (e *Edge) Source() shared.NodeID {
	return e.sourceID
}

// Target returns the target node ID  
func (e *Edge) Target() shared.NodeID {
	return e.targetID
}

// UserID returns the user who owns this edge
func (e *Edge) UserID() shared.UserID {
	return e.userID
}

// Weight returns the connection strength
func (e *Edge) Weight() float64 {
	return e.weight.Value()
}

// GetCreatedAt returns when the edge was created (internal)
func (e *Edge) GetCreatedAt() time.Time {
	return e.createdAt
}

// Note: Public fields ID, CreatedAt, UpdatedAt can be accessed directly

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
	return e.weight.IsStrong()
}

// IsWeakConnection checks if this is a weak connection based on weight
func (e *Edge) IsWeakConnection() bool {
	return e.weight.IsWeak()
}

// CalculateReciprocalWeight calculates what the weight should be for the reverse edge
func (e *Edge) CalculateReciprocalWeight() float64 {
	// For now, return the same weight, but this could implement more complex logic
	// based on the directionality of the relationship
	return e.weight.Value()
}

// UpdateWeight updates the edge weight with validation and event generation
//
// Business Rules Enforced:
//   - Weight must be between 0.0 and 1.0
//   - Version is incremented for optimistic locking
//   - Domain events are generated if weight actually changes
func (e *Edge) UpdateWeight(newWeight float64) error {
	// Create new weight value object with validation
	newWeightVO, err := shared.NewWeight(newWeight)
	if err != nil {
		return err
	}
	
	// Check if weight actually changed
	if e.weight.Equals(newWeightVO) {
		return nil // No change needed
	}
	
	// Store old weight for event
	oldWeight := e.weight
	
	// Apply changes
	e.weight = newWeightVO
	e.updatedAt = time.Now()
	e.version = e.version.Next()
	e.Strength = newWeight // Update public field for compatibility
	e.UpdatedAt = e.updatedAt
	e.Version = e.version.Int()
	
	// Generate domain event
	event := shared.NewEdgeWeightUpdatedEvent(e.id, e.sourceID, e.targetID, e.userID, oldWeight, newWeightVO, e.version)
	e.addEvent(event)
	
	return nil
}

// ValidateInvariants ensures all business rules are satisfied
func (e *Edge) ValidateInvariants() error {
	// Source and target must be different
	if e.sourceID.Equals(e.targetID) {
		return shared.NewDomainError("invalid_edge_state", "edge cannot connect a node to itself", nil)
	}
	
	// Weight must be valid
	if !e.weight.IsValid() {
		return shared.NewDomainError("invalid_edge_state", "edge weight must be between 0.0 and 1.0", nil)
	}
	
	// Edge ID must be valid
	if e.id.String() == "" {
		return shared.NewDomainError("invalid_edge_state", "edge must have a valid ID", nil)
	}
	
	// UserID must be valid
	if e.userID.String() == "" {
		return shared.NewDomainError("invalid_edge_state", "edge must have a valid user ID", nil)
	}
	
	// Timestamps must be valid
	if e.createdAt.IsZero() {
		return shared.NewDomainError("invalid_edge_state", "edge must have a creation timestamp", nil)
	}
	
	if e.updatedAt.Before(e.createdAt) {
		return shared.NewDomainError("invalid_edge_state", "edge update timestamp cannot be before creation timestamp", nil)
	}
	
	// Version must be non-negative
	if e.version.Int() < 0 {
		return shared.NewDomainError("invalid_edge_state", "edge version must be non-negative", nil)
	}
	
	return nil
}

// Metadata returns the edge metadata
func (e *Edge) Metadata() shared.EdgeMetadata {
	return e.metadata
}

// SetMetadata sets the edge metadata
func (e *Edge) SetMetadata(metadata shared.EdgeMetadata) {
	e.metadata = metadata
	e.updatedAt = time.Now()
	e.UpdatedAt = e.updatedAt
}

// Delete marks this edge for deletion and generates appropriate events
func (e *Edge) Delete() {
	// Generate domain event for edge deletion
	event := shared.NewEdgeDeletedEvent(e.id, e.sourceID, e.targetID, e.userID, e.weight.Value(), e.version)
	e.addEvent(event)
	
	// Increment version for optimistic locking
	e.version = e.version.Next()
	e.Version = e.version.Int()
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (e *Edge) GetUncommittedEvents() []shared.DomainEvent {
	// Use the BaseAggregateRoot's implementation if events are tracked there
	baseEvents := e.BaseAggregateRoot.GetUncommittedEvents()
	if len(baseEvents) > 0 {
		return baseEvents
	}
	// Fall back to local events for backward compatibility
	return e.events
}

// MarkEventsAsCommitted clears the events after persistence
func (e *Edge) MarkEventsAsCommitted() {
	e.BaseAggregateRoot.MarkEventsAsCommitted()
	e.events = []shared.DomainEvent{}
}

// GetID returns the unique identifier of the edge aggregate
func (e *Edge) GetID() string {
	return e.id.String()
}

// GetVersion returns the current version for optimistic locking
func (e *Edge) GetVersion() int {
	return e.version.Int()
}

// IncrementVersion increments the version after successful persistence
func (e *Edge) IncrementVersion() {
	e.version = e.version.Next()
	e.Version = e.version.Int()
}

// Private helper methods

// addEvent adds a domain event to the uncommitted events list
func (e *Edge) addEvent(event shared.DomainEvent) {
	e.BaseAggregateRoot.AddEvent(event)
	e.events = append(e.events, event) // Keep for backward compatibility
}

