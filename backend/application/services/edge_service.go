package services

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/services"
	"backend/infrastructure/config"
	"go.uber.org/zap"
)

// EdgeCandidate is now using the aggregates' EdgeCandidate type
type EdgeCandidate = aggregates.EdgeCandidate

// EdgeService provides application-level edge operations
// It delegates domain logic to domain services and handles repository operations
type EdgeService struct {
	nodeRepo         ports.NodeRepository
	graphRepo        ports.GraphRepository
	edgeRepo         ports.EdgeRepository
	edgeDiscovery    aggregates.EdgeDiscoveryService
	similarityCalc   aggregates.SimilarityCalculator
	config           *config.EdgeCreationConfig
	logger           *zap.Logger
}

// NewEdgeService creates a new edge service
func NewEdgeService(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	config *config.EdgeCreationConfig,
	logger *zap.Logger,
) *EdgeService {
	// Create domain service configuration
	edgeConfig := &services.EdgeDiscoveryConfig{
		MinSimilarity:       config.SimilarityThreshold,
		StrongEdgeThreshold: 0.7, // TODO: add to config
		MaxEdgesPerNode:     config.MaxEdgesPerNode,
		ConsiderBidirectional: false,
	}
	
	// Create domain services
	textAnalyzer := services.NewDefaultTextAnalyzer()
	similarityCalc := services.NewDefaultSimilarityCalculator(nil, textAnalyzer)
	edgeDiscovery := services.NewDefaultEdgeDiscoveryService(edgeConfig, similarityCalc)
	
	return &EdgeService{
		nodeRepo:       nodeRepo,
		graphRepo:      graphRepo,
		edgeRepo:       edgeRepo,
		edgeDiscovery:  edgeDiscovery,
		similarityCalc: similarityCalc,
		config:         config,
		logger:         logger,
	}
}

// DiscoverEdges analyzes a node and returns edge candidates split into sync and async groups
// This uses the domain EdgeDiscoveryService for the actual edge discovery logic
func (s *EdgeService) DiscoverEdges(
	ctx context.Context,
	node *entities.Node,
	graph *aggregates.Graph,
	syncLimit int,
) (syncEdges []EdgeCandidate, asyncCandidates []EdgeCandidate, err error) {
	if node == nil || graph == nil {
		return nil, nil, fmt.Errorf("node and graph are required")
	}

	// Use config value if syncLimit is 0
	if syncLimit == 0 {
		syncLimit = s.config.SyncEdgeLimit
	}

	s.logger.Debug("Discovering edges for node",
		zap.String("nodeID", node.ID().String()),
		zap.String("graphID", string(graph.ID())),
		zap.Int("syncLimit", syncLimit),
	)

	// Use domain service to discover potential edges
	candidates := s.edgeDiscovery.DiscoverPotentialEdges(node, graph)
	
	// Rank the candidates
	candidates = s.edgeDiscovery.RankEdges(candidates)
	
	// Filter by max edges and threshold
	candidates = s.edgeDiscovery.FilterEdges(
		candidates, 
		s.config.MaxEdgesPerNode, 
		s.config.SimilarityThreshold,
	)

	// Split into sync and async
	if len(candidates) <= syncLimit {
		// All edges can be created synchronously
		syncEdges = candidates
		asyncCandidates = nil
	} else {
		// Split at the sync limit
		syncEdges = candidates[:syncLimit]
		asyncCandidates = candidates[syncLimit:]
	}

	s.logger.Info("Edge discovery complete",
		zap.String("nodeID", node.ID().String()),
		zap.Int("totalCandidates", len(candidates)),
		zap.Int("syncEdges", len(syncEdges)),
		zap.Int("asyncCandidates", len(asyncCandidates)),
	)

	return syncEdges, asyncCandidates, nil
}

// CreateEdgesForNewNode discovers and creates edges for a newly created node
// This is optimized for the connect-node Lambda use case
func (s *EdgeService) CreateEdgesForNewNode(
	ctx context.Context,
	nodeID string,
	userID string,
	graphID string,
	keywords []string,
	tags []string,
) ([]string, error) {
	// Input validation
	if nodeID == "" || userID == "" || graphID == "" {
		return nil, fmt.Errorf("invalid input: nodeID, userID, and graphID are required")
	}

	s.logger.Debug("Creating edges for new node",
		zap.String("nodeID", nodeID),
		zap.String("graphID", graphID),
		zap.Int("keywords", len(keywords)),
		zap.Int("tags", len(tags)),
	)

	// Get the graph
	graph, err := s.graphRepo.GetByID(ctx, aggregates.GraphID(graphID))
	if err != nil {
		return nil, fmt.Errorf("failed to get graph: %w", err)
	}

	// Parse the source node ID
	sourceNodeID, err := valueobjects.NewNodeIDFromString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	// Get the source node
	sourceNode, err := s.nodeRepo.GetByID(ctx, sourceNodeID)
	if err != nil {
		// If node is not found yet, we might need to wait or create a minimal node
		s.logger.Warn("Source node not found, attempting edge discovery with keywords/tags",
			zap.String("nodeID", nodeID),
			zap.Error(err),
		)
		// For now, return early - in production, might want to retry or queue
		return nil, nil
	}

	// Use the domain AutoConnect method to create edges
	err = graph.AutoConnect(sourceNode, s.edgeDiscovery, s.config.MaxEdgesPerNode)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-connect node: %w", err)
	}

	// Get the created edges from the graph
	var createdEdgeIDs []string
	edges := graph.GetEdges() // Get all edges to find the new ones
	for _, edge := range edges {
		if edge.SourceID == sourceNodeID {
			createdEdgeIDs = append(createdEdgeIDs, edge.ID)
		}
	}

	// Save the updated graph with new edges
	if len(createdEdgeIDs) > 0 {
		if err := s.graphRepo.Save(ctx, graph); err != nil {
			return nil, fmt.Errorf("failed to save graph with new edges: %w", err)
		}

		s.logger.Info("Successfully created edges for node",
			zap.String("nodeID", nodeID),
			zap.Int("edgeCount", len(createdEdgeIDs)),
		)
	}

	return createdEdgeIDs, nil
}

// CreateEdge creates a single edge between two nodes
// This can be used by the API when users manually create edges
func (s *EdgeService) CreateEdge(
	ctx context.Context,
	sourceID string,
	targetID string,
	graphID string,
	edgeType string,
	weight float64,
) (string, error) {
	// Parse IDs
	sourceNodeID, err := valueobjects.NewNodeIDFromString(sourceID)
	if err != nil {
		return "", fmt.Errorf("invalid source node ID: %w", err)
	}

	targetNodeID, err := valueobjects.NewNodeIDFromString(targetID)
	if err != nil {
		return "", fmt.Errorf("invalid target node ID: %w", err)
	}

	// Get the graph
	graph, err := s.graphRepo.GetByID(ctx, aggregates.GraphID(graphID))
	if err != nil {
		return "", fmt.Errorf("failed to get graph: %w", err)
	}

	// Map edge type string to entities.EdgeType
	var entityEdgeType entities.EdgeType
	switch edgeType {
	case "similar", "semantic":
		entityEdgeType = entities.EdgeTypeWeak
	case "reference":
		entityEdgeType = entities.EdgeTypeReference
	case "parent_child":
		entityEdgeType = entities.EdgeTypeHierarchical
	case "sequential":
		entityEdgeType = entities.EdgeTypeTemporal
	default:
		entityEdgeType = entities.EdgeTypeWeak // Default to similar
	}

	// Use Graph's ConnectNodes method to create edge
	edge, err := graph.ConnectNodes(sourceNodeID, targetNodeID, entityEdgeType)
	if err != nil {
		return "", fmt.Errorf("failed to create edge: %w", err)
	}

	// Set the weight
	edge.Weight = weight

	// Save the graph
	if err := s.graphRepo.Save(ctx, graph); err != nil {
		return "", fmt.Errorf("failed to save graph: %w", err)
	}

	s.logger.Info("Edge created",
		zap.String("edgeID", edge.ID),
		zap.String("source", sourceID),
		zap.String("target", targetID),
		zap.String("type", edgeType),
		zap.Float64("weight", weight),
	)

	return edge.ID, nil
}