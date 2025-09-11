package specifications

import (
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// EdgeSpecification is a specification for Edge entities
type EdgeSpecification interface {
	Specification[*aggregates.Edge]
}

// EdgeWeightSpec validates edge weight is within range
type EdgeWeightSpec struct {
	BaseSpecification[*aggregates.Edge]
	minWeight float64
	maxWeight float64
}

// NewEdgeWeightSpec creates a specification for edge weight
func NewEdgeWeightSpec(minWeight, maxWeight float64) *EdgeWeightSpec {
	spec := &EdgeWeightSpec{
		minWeight: minWeight,
		maxWeight: maxWeight,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeWeightSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}
	return edge.Weight >= s.minWeight && edge.Weight <= s.maxWeight
}

// EdgeTypeSpec validates edge type
type EdgeTypeSpec struct {
	BaseSpecification[*aggregates.Edge]
	allowedTypes []entities.EdgeType
}

// NewEdgeTypeSpec creates a specification for edge type
func NewEdgeTypeSpec(allowedTypes ...entities.EdgeType) *EdgeTypeSpec {
	spec := &EdgeTypeSpec{
		allowedTypes: allowedTypes,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeTypeSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}

	for _, allowed := range s.allowedTypes {
		if edge.Type == allowed {
			return true
		}
	}
	return false
}

// EdgeNotSelfLoopSpec validates that an edge is not a self-loop
type EdgeNotSelfLoopSpec struct {
	BaseSpecification[*aggregates.Edge]
}

// NewEdgeNotSelfLoopSpec creates a specification to prevent self-loops
func NewEdgeNotSelfLoopSpec() *EdgeNotSelfLoopSpec {
	spec := &EdgeNotSelfLoopSpec{}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeNotSelfLoopSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}
	return !edge.SourceID.Equals(edge.TargetID)
}

// EdgeBidirectionalSpec validates edge directionality
type EdgeBidirectionalSpec struct {
	BaseSpecification[*aggregates.Edge]
	mustBeBidirectional bool
}

// NewEdgeBidirectionalSpec creates a specification for edge directionality
func NewEdgeBidirectionalSpec(mustBeBidirectional bool) *EdgeBidirectionalSpec {
	spec := &EdgeBidirectionalSpec{
		mustBeBidirectional: mustBeBidirectional,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeBidirectionalSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}
	return edge.Bidirectional == s.mustBeBidirectional
}

// EdgeConnectsNodesSpec validates that an edge connects specific nodes
type EdgeConnectsNodesSpec struct {
	BaseSpecification[*aggregates.Edge]
	nodeIDs []valueobjects.NodeID
}

// NewEdgeConnectsNodesSpec creates a specification for edges connecting specific nodes
func NewEdgeConnectsNodesSpec(nodeIDs ...valueobjects.NodeID) *EdgeConnectsNodesSpec {
	spec := &EdgeConnectsNodesSpec{
		nodeIDs: nodeIDs,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeConnectsNodesSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}

	for _, nodeID := range s.nodeIDs {
		if edge.SourceID.Equals(nodeID) || edge.TargetID.Equals(nodeID) {
			return true
		}
	}
	return false
}

// EdgeMetadataSpec validates edge metadata
type EdgeMetadataSpec struct {
	BaseSpecification[*aggregates.Edge]
	requiredKeys []string
	validator    func(metadata map[string]interface{}) bool
}

// NewEdgeMetadataSpec creates a specification for edge metadata
func NewEdgeMetadataSpec(requiredKeys []string, validator func(map[string]interface{}) bool) *EdgeMetadataSpec {
	spec := &EdgeMetadataSpec{
		requiredKeys: requiredKeys,
		validator:    validator,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *EdgeMetadataSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}

	// Check required keys
	for _, key := range s.requiredKeys {
		if _, exists := edge.Metadata[key]; !exists {
			return false
		}
	}

	// Run custom validator if provided
	if s.validator != nil {
		return s.validator(edge.Metadata)
	}

	return true
}

// UniqueEdgeSpec validates that an edge is unique between two nodes
type UniqueEdgeSpec struct {
	BaseSpecification[*aggregates.Edge]
	existingEdges map[string]*aggregates.Edge
}

// NewUniqueEdgeSpec creates a specification for edge uniqueness
func NewUniqueEdgeSpec(existingEdges map[string]*aggregates.Edge) *UniqueEdgeSpec {
	spec := &UniqueEdgeSpec{
		existingEdges: existingEdges,
	}
	spec.BaseSpecification = BaseSpecification[*aggregates.Edge]{
		evaluator: spec.evaluate,
	}
	return spec
}

func (s *UniqueEdgeSpec) evaluate(edge *aggregates.Edge) bool {
	if edge == nil {
		return false
	}

	// Check if an edge already exists between the same nodes
	for existingID, existingEdge := range s.existingEdges {
		// Skip if checking against itself
		if existingID == edge.ID {
			continue
		}

		// Check for duplicate edge (same source and target)
		if existingEdge.SourceID.Equals(edge.SourceID) &&
			existingEdge.TargetID.Equals(edge.TargetID) {
			return false
		}

		// Check for reverse duplicate if bidirectional
		if existingEdge.Bidirectional &&
			existingEdge.SourceID.Equals(edge.TargetID) &&
			existingEdge.TargetID.Equals(edge.SourceID) {
			return false
		}
	}

	return true
}

// Common pre-configured specifications

// NewValidEdgeSpec creates a specification for a valid edge
func NewValidEdgeSpec() EdgeSpecification {
	return NewEdgeWeightSpec(0.0, 1.0).
		And(NewEdgeNotSelfLoopSpec()).
		And(NewEdgeTypeSpec(
			entities.EdgeTypeNormal,
			entities.EdgeTypeStrong,
			entities.EdgeTypeWeak,
			entities.EdgeTypeReference,
			entities.EdgeTypeHierarchical,
			entities.EdgeTypeTemporal,
		))
}

// NewHierarchicalEdgeSpec creates a specification for hierarchical edges
func NewHierarchicalEdgeSpec() EdgeSpecification {
	return NewEdgeTypeSpec(entities.EdgeTypeHierarchical).
		And(NewEdgeBidirectionalSpec(false)) // Hierarchical edges should be directional
}

// NewStrongConnectionSpec creates a specification for strong connections
func NewStrongConnectionSpec() EdgeSpecification {
	return NewEdgeTypeSpec(entities.EdgeTypeStrong).
		And(NewEdgeWeightSpec(0.7, 1.0)) // Strong edges should have high weight
}

// NewWeakConnectionSpec creates a specification for weak connections
func NewWeakConnectionSpec() EdgeSpecification {
	return NewEdgeTypeSpec(entities.EdgeTypeWeak).
		And(NewEdgeWeightSpec(0.0, 0.3)) // Weak edges should have low weight
}