package services

import (
	"fmt"

	"backend/domain/config"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	pkgerrors "backend/pkg/errors"
)

// GraphValidationService centralizes graph validation and business rules
// This service ensures graph integrity and enforces domain constraints
type GraphValidationService struct {
	config *config.DomainConfig
}

// NewGraphValidationService creates a new graph validation service
func NewGraphValidationService(cfg *config.DomainConfig) *GraphValidationService {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}
	return &GraphValidationService{
		config: cfg,
	}
}

// ValidateGraph performs comprehensive validation of a graph
func (s *GraphValidationService) ValidateGraph(graph *aggregates.Graph) error {
	if graph == nil {
		return pkgerrors.NewValidationError("graph cannot be nil")
	}

	// Validate graph metadata
	if err := s.validateGraphMetadata(graph); err != nil {
		return err
	}

	// Validate node constraints
	if err := s.validateNodeConstraints(graph); err != nil {
		return err
	}

	// Validate edge constraints
	if err := s.validateEdgeConstraints(graph); err != nil {
		return err
	}

	// Validate graph invariants
	if err := s.validateGraphInvariants(graph); err != nil {
		return err
	}

	// Validate graph consistency
	if err := s.validateGraphConsistency(graph); err != nil {
		return err
	}

	return nil
}

// ValidateNodeAddition validates if a node can be added to the graph
func (s *GraphValidationService) ValidateNodeAddition(
	graph *aggregates.Graph,
	node *entities.Node,
) error {
	if graph == nil {
		return pkgerrors.NewValidationError("graph cannot be nil")
	}
	if node == nil {
		return pkgerrors.NewValidationError("node cannot be nil")
	}

	nodes, err := graph.Nodes()
	if err != nil {
		// Large graph, check node count from metadata
		if graph.NodeCount() >= s.config.MaxNodesPerGraph {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("maximum nodes reached: %d", s.config.MaxNodesPerGraph),
			)
		}
	} else {
		// Check if node already exists
		if _, exists := nodes[node.ID()]; exists {
			return pkgerrors.NewConflictError("node already exists in graph")
		}

		// Check node limit
		if len(nodes) >= s.config.MaxNodesPerGraph {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("maximum nodes reached: %d", s.config.MaxNodesPerGraph),
			)
		}
	}

	// Validate node content
	if err := s.validateNodeContent(node); err != nil {
		return err
	}

	// Validate node position
	if err := s.validateNodePosition(node); err != nil {
		return err
	}

	return nil
}

// ValidateEdgeAddition validates if an edge can be added to the graph
func (s *GraphValidationService) ValidateEdgeAddition(
	graph *aggregates.Graph,
	sourceID, targetID valueobjects.NodeID,
	edgeType entities.EdgeType,
) error {
	if graph == nil {
		return pkgerrors.NewValidationError("graph cannot be nil")
	}

	// Validate nodes exist
	nodes, err := graph.Nodes()
	if err != nil {
		return fmt.Errorf("cannot validate edge for large graph: %w", err)
	}

	sourceNode, sourceExists := nodes[sourceID]
	targetNode, targetExists := nodes[targetID]

	if !sourceExists {
		return pkgerrors.NewValidationError("source node does not exist")
	}
	if !targetExists {
		return pkgerrors.NewValidationError("target node does not exist")
	}

	// Check for self-reference
	if sourceID.Equals(targetID) {
		return pkgerrors.NewValidationError("cannot connect node to itself")
	}

	// Check for duplicate edge
	edges := graph.Edges()
	edgeKey := s.makeEdgeKey(sourceID, targetID)
	if _, exists := edges[edgeKey]; exists {
		return pkgerrors.NewConflictError("edge already exists")
	}

	// Check edge limit
	if len(edges) >= s.config.MaxEdgesPerGraph {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("maximum edges reached: %d", s.config.MaxEdgesPerGraph),
		)
	}

	// Check node connection limits
	if err := s.validateNodeConnectionLimits(edges, sourceID); err != nil {
		return fmt.Errorf("source node: %w", err)
	}
	if err := s.validateNodeConnectionLimits(edges, targetID); err != nil {
		return fmt.Errorf("target node: %w", err)
	}

	// Validate edge type
	if !s.isValidEdgeType(edgeType) {
		return pkgerrors.NewValidationError(fmt.Sprintf("invalid edge type: %s", edgeType))
	}

	// Check for cycles if needed (for hierarchical edges)
	if edgeType == entities.EdgeTypeHierarchical {
		if s.wouldCreateCycle(edges, sourceID, targetID) {
			return pkgerrors.NewValidationError("edge would create a cycle in hierarchy")
		}
	}

	// Validate nodes are compatible for this edge type
	if !s.areNodesCompatibleForEdgeType(sourceNode, targetNode, edgeType) {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("nodes are not compatible for edge type: %s", edgeType),
		)
	}

	return nil
}

// ValidateNodeRemoval validates if a node can be removed from the graph
func (s *GraphValidationService) ValidateNodeRemoval(
	graph *aggregates.Graph,
	nodeID valueobjects.NodeID,
) error {
	if graph == nil {
		return pkgerrors.NewValidationError("graph cannot be nil")
	}

	nodes, err := graph.Nodes()
	if err != nil {
		return fmt.Errorf("cannot validate node removal for large graph: %w", err)
	}

	node, exists := nodes[nodeID]
	if !exists {
		return pkgerrors.NewNotFoundError("node")
	}

	// Check if node can be archived
	if node.Status() == entities.StatusArchived {
		return pkgerrors.NewValidationError("node is already archived")
	}

	// Check if removing this node would violate any invariants
	edges := graph.Edges()
	criticalConnections := s.findCriticalConnections(edges, nodeID)
	if len(criticalConnections) > 0 {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node has %d critical connections that must be removed first",
				len(criticalConnections)),
		)
	}

	return nil
}

// ValidateBulkOperation validates if a bulk operation can be performed
func (s *GraphValidationService) ValidateBulkOperation(
	graph *aggregates.Graph,
	operation string,
	itemCount int,
) error {
	if graph == nil {
		return pkgerrors.NewValidationError("graph cannot be nil")
	}

	// Check operation type
	switch operation {
	case "add_nodes":
		currentCount := graph.NodeCount()
		if currentCount+itemCount > s.config.MaxNodesPerGraph {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("bulk operation would exceed max nodes: %d + %d > %d",
					currentCount, itemCount, s.config.MaxNodesPerGraph),
			)
		}
	case "add_edges":
		currentCount := graph.EdgeCount()
		if currentCount+itemCount > s.config.MaxEdgesPerGraph {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("bulk operation would exceed max edges: %d + %d > %d",
					currentCount, itemCount, s.config.MaxEdgesPerGraph),
			)
		}
	case "remove_nodes", "remove_edges":
		// These are generally allowed
	default:
		return pkgerrors.NewValidationError(fmt.Sprintf("unknown operation: %s", operation))
	}

	// Check bulk operation size limit
	if itemCount > s.config.MaxBulkOperationSize {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("bulk operation size exceeds limit: %d > %d",
				itemCount, s.config.MaxBulkOperationSize),
		)
	}

	return nil
}

// Private validation methods

func (s *GraphValidationService) validateGraphMetadata(graph *aggregates.Graph) error {
	// Validate graph name
	if graph.Name() == "" {
		return pkgerrors.NewValidationError("graph name is required")
	}

	if len(graph.Name()) > 255 {
		return pkgerrors.NewValidationError("graph name exceeds maximum length")
	}

	// Validate user ID
	if graph.UserID() == "" {
		return pkgerrors.NewValidationError("graph must have an owner")
	}

	return nil
}

func (s *GraphValidationService) validateNodeConstraints(graph *aggregates.Graph) error {
	nodes, err := graph.Nodes()
	if err != nil {
		// Skip validation for large graphs
		return nil
	}

	for _, node := range nodes {
		if err := s.validateNode(node); err != nil {
			return fmt.Errorf("invalid node %s: %w", node.ID(), err)
		}
	}

	return nil
}

func (s *GraphValidationService) validateNode(node *entities.Node) error {
	// Validate node ID
	if node.ID().IsZero() {
		return pkgerrors.NewValidationError("node must have valid ID")
	}

	// Validate node content
	if err := s.validateNodeContent(node); err != nil {
		return err
	}

	// Validate node position
	if err := s.validateNodePosition(node); err != nil {
		return err
	}

	// Validate tags
	tags := node.GetTags()
	if len(tags) > s.config.MaxTagsPerNode {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node has too many tags: %d > %d", len(tags), s.config.MaxTagsPerNode),
		)
	}

	return nil
}

func (s *GraphValidationService) validateNodeContent(node *entities.Node) error {
	content := node.Content()

	// Validate title
	if len(content.Title()) > 255 {
		return pkgerrors.NewValidationError("node title exceeds maximum length")
	}

	// Validate body
	if len(content.Body()) > 50000 {
		return pkgerrors.NewValidationError("node content exceeds maximum length")
	}

	// Validate format
	validFormats := []valueobjects.ContentFormat{
		valueobjects.FormatPlainText,
		valueobjects.FormatMarkdown,
		valueobjects.FormatHTML,
		valueobjects.FormatJSON,
	}

	formatValid := false
	for _, valid := range validFormats {
		if content.Format() == valid {
			formatValid = true
			break
		}
	}

	if !formatValid {
		return pkgerrors.NewValidationError(fmt.Sprintf("invalid content format: %s", content.Format()))
	}

	return nil
}

func (s *GraphValidationService) validateNodePosition(node *entities.Node) error {
	pos := node.Position()

	// Validate position bounds
	maxCoordinate := 10000.0
	minCoordinate := -10000.0

	if pos.X() < minCoordinate || pos.X() > maxCoordinate {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node X position out of bounds: %f", pos.X()),
		)
	}

	if pos.Y() < minCoordinate || pos.Y() > maxCoordinate {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node Y position out of bounds: %f", pos.Y()),
		)
	}

	if pos.Z() < minCoordinate || pos.Z() > maxCoordinate {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node Z position out of bounds: %f", pos.Z()),
		)
	}

	return nil
}

func (s *GraphValidationService) validateEdgeConstraints(graph *aggregates.Graph) error {
	edges := graph.Edges()

	for _, edge := range edges {
		if err := s.validateEdge(graph, edge); err != nil {
			return fmt.Errorf("invalid edge %s: %w", edge.ID, err)
		}
	}

	return nil
}

func (s *GraphValidationService) validateEdge(graph *aggregates.Graph, edge *aggregates.Edge) error {
	// Validate edge weight
	if edge.Weight < 0 || edge.Weight > 1 {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("edge weight must be between 0 and 1: %f", edge.Weight),
		)
	}

	// Validate edge type
	if !s.isValidEdgeType(edge.Type) {
		return pkgerrors.NewValidationError(fmt.Sprintf("invalid edge type: %s", edge.Type))
	}

	// Validate nodes exist
	nodes, err := graph.Nodes()
	if err == nil {
		if _, exists := nodes[edge.SourceID]; !exists {
			return pkgerrors.NewValidationError("edge references non-existent source node")
		}
		if _, exists := nodes[edge.TargetID]; !exists {
			return pkgerrors.NewValidationError("edge references non-existent target node")
		}
	}

	return nil
}

func (s *GraphValidationService) validateGraphInvariants(graph *aggregates.Graph) error {
	// Check metadata consistency
	nodes, err := graph.Nodes()
	if err == nil {
		if len(nodes) != graph.NodeCount() {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("node count mismatch: actual=%d, metadata=%d",
					len(nodes), graph.NodeCount()),
			)
		}
	}

	edges := graph.Edges()
	if len(edges) != graph.EdgeCount() {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("edge count mismatch: actual=%d, metadata=%d",
				len(edges), graph.EdgeCount()),
		)
	}

	return nil
}

func (s *GraphValidationService) validateGraphConsistency(graph *aggregates.Graph) error {
	// Check for orphaned edges
	nodes, err := graph.Nodes()
	if err != nil {
		// Skip for large graphs
		return nil
	}

	edges := graph.Edges()
	for _, edge := range edges {
		if _, exists := nodes[edge.SourceID]; !exists {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("edge %s references non-existent source node", edge.ID),
			)
		}
		if _, exists := nodes[edge.TargetID]; !exists {
			return pkgerrors.NewValidationError(
				fmt.Sprintf("edge %s references non-existent target node", edge.ID),
			)
		}
	}

	return nil
}

func (s *GraphValidationService) validateNodeConnectionLimits(
	edges map[string]*aggregates.Edge,
	nodeID valueobjects.NodeID,
) error {
	connectionCount := 0
	for _, edge := range edges {
		if edge.SourceID.Equals(nodeID) || edge.TargetID.Equals(nodeID) {
			connectionCount++
		}
	}

	if connectionCount >= s.config.MaxConnectionsPerNode {
		return pkgerrors.NewValidationError(
			fmt.Sprintf("node has reached maximum connections: %d", s.config.MaxConnectionsPerNode),
		)
	}

	return nil
}

// Helper methods

func (s *GraphValidationService) makeEdgeKey(sourceID, targetID valueobjects.NodeID) string {
	return sourceID.String() + "->" + targetID.String()
}

func (s *GraphValidationService) isValidEdgeType(edgeType entities.EdgeType) bool {
	validTypes := []entities.EdgeType{
		entities.EdgeTypeNormal,
		entities.EdgeTypeStrong,
		entities.EdgeTypeWeak,
		entities.EdgeTypeReference,
		entities.EdgeTypeHierarchical,
		entities.EdgeTypeTemporal,
	}

	for _, valid := range validTypes {
		if edgeType == valid {
			return true
		}
	}

	return false
}

func (s *GraphValidationService) wouldCreateCycle(
	edges map[string]*aggregates.Edge,
	sourceID, targetID valueobjects.NodeID,
) bool {
	// Simple cycle detection using DFS
	visited := make(map[valueobjects.NodeID]bool)
	return s.hasCycleDFS(edges, targetID, sourceID, visited)
}

func (s *GraphValidationService) hasCycleDFS(
	edges map[string]*aggregates.Edge,
	current, target valueobjects.NodeID,
	visited map[valueobjects.NodeID]bool,
) bool {
	if current.Equals(target) {
		return true
	}

	visited[current] = true

	for _, edge := range edges {
		if edge.Type == entities.EdgeTypeHierarchical && edge.SourceID.Equals(current) {
			if !visited[edge.TargetID] {
				if s.hasCycleDFS(edges, edge.TargetID, target, visited) {
					return true
				}
			}
		}
	}

	return false
}

func (s *GraphValidationService) areNodesCompatibleForEdgeType(
	sourceNode, targetNode *entities.Node,
	edgeType entities.EdgeType,
) bool {
	// Add type-specific compatibility rules
	switch edgeType {
	case entities.EdgeTypeHierarchical:
		// Hierarchical edges might have specific requirements
		// For example, can't have circular hierarchies
		return true
	case entities.EdgeTypeTemporal:
		// Temporal edges might require timestamp metadata
		return true
	default:
		return true
	}
}

func (s *GraphValidationService) findCriticalConnections(
	edges map[string]*aggregates.Edge,
	nodeID valueobjects.NodeID,
) []*aggregates.Edge {
	critical := []*aggregates.Edge{}

	for _, edge := range edges {
		// Check if this edge is critical (e.g., hierarchical parent)
		if edge.Type == entities.EdgeTypeHierarchical && edge.TargetID.Equals(nodeID) {
			critical = append(critical, edge)
		}
	}

	return critical
}