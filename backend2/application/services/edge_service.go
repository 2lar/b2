package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"backend2/application/ports"
	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
	"backend2/domain/core/valueobjects"
	"go.uber.org/zap"
)

// EdgeService provides simple, direct edge creation functionality
// This service is used internally by Lambda functions for efficient edge creation
// without the overhead of the command bus
type EdgeService struct {
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	edgeRepo  ports.EdgeRepository
	logger    *zap.Logger
}

// NewEdgeService creates a new edge service
func NewEdgeService(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *EdgeService {
	return &EdgeService{
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		edgeRepo:  edgeRepo,
		logger:    logger,
	}
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

	// Get all nodes in the graph to find similar ones
	existingNodes, err := graph.Nodes()
	if err != nil {
		// For large graphs, fall back to pagination or skip auto-connection
		return nil, fmt.Errorf("graph too large for automatic edge creation: %w", err)
	}
	if len(existingNodes) <= 1 {
		// No other nodes to connect to
		return nil, nil
	}

	// Parse the source node ID
	sourceNodeID, err := valueobjects.NewNodeIDFromString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	// Pre-process keywords and tags for O(1) lookups
	keywordSet := make(map[string]bool, len(keywords))
	for _, kw := range keywords {
		if kw != "" {
			keywordSet[strings.ToLower(kw)] = true
		}
	}

	tagSet := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag != "" {
			tagSet[strings.ToLower(tag)] = true
		}
	}

	// Use default limits for now
	// TODO: Get these from graph config when available
	maxEdges := 10             // Default max edges per node
	similarityThreshold := 0.3 // Default threshold

	// Calculate similarity scores for all nodes
	type nodeSimilarity struct {
		node       *entities.Node
		similarity float64
	}

	similarities := make([]nodeSimilarity, 0, len(existingNodes))

	for _, targetNode := range existingNodes {
		// Skip self
		if targetNode.ID() == sourceNodeID {
			continue
		}

		// Calculate similarity based on keywords and tags
		similarity := s.calculateSimilarity(targetNode, keywordSet, tagSet)

		// Only consider if above threshold
		if similarity > similarityThreshold {
			similarities = append(similarities, nodeSimilarity{
				node:       targetNode,
				similarity: similarity,
			})
		}
	}

	// Sort by similarity (highest first) and limit
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].similarity > similarities[j].similarity
	})

	// Limit to maxEdges
	if len(similarities) > maxEdges {
		similarities = similarities[:maxEdges]
	}

	// Create edges for top similar nodes
	var createdEdgeIDs []string
	for _, sim := range similarities {
		// Use Graph's ConnectNodes method to create edge
		edge, err := graph.ConnectNodes(
			sourceNodeID,
			sim.node.ID(),
			entities.EdgeTypeSimilar,
		)
		if err != nil {
			s.logger.Warn("Failed to create edge",
				zap.Error(err),
				zap.String("source", nodeID),
				zap.String("target", sim.node.ID().String()),
			)
			continue
		}

		// Set the weight based on similarity
		edge.Weight = sim.similarity

		createdEdgeIDs = append(createdEdgeIDs, edge.ID)

		s.logger.Debug("Created edge",
			zap.String("edgeID", edge.ID),
			zap.String("source", nodeID),
			zap.String("target", sim.node.ID().String()),
			zap.Float64("weight", sim.similarity),
		)
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

// calculateSimilarity calculates similarity between a target node and the source node's keywords/tags
// Optimized version with O(1) lookups and pre-processing
func (s *EdgeService) calculateSimilarity(
	targetNode *entities.Node,
	sourceKeywords map[string]bool,
	sourceTags map[string]bool,
) float64 {
	if targetNode == nil || (len(sourceKeywords) == 0 && len(sourceTags) == 0) {
		return 0
	}

	matches := 0
	total := len(sourceKeywords) + len(sourceTags)

	// Build word set from target node for O(1) lookups
	nodeContent := targetNode.Content()
	if !nodeContent.IsEmpty() {
		// Pre-process target text into word set
		targetWords := s.extractWords(nodeContent.Title() + " " + nodeContent.Body())

		// Check keyword matches with O(1) lookup
		for keyword := range sourceKeywords {
			if targetWords[keyword] {
				matches++
			}
		}
	}

	// Check tag matches (already O(1))
	targetTags := targetNode.GetTags()
	for _, tag := range targetTags {
		tagLower := strings.ToLower(tag)
		if sourceTags[tagLower] {
			matches++
		}
	}

	// Calculate similarity as percentage of matches
	similarity := float64(matches) / float64(total)

	// Cap at 1.0
	if similarity > 1.0 {
		similarity = 1.0
	}

	return similarity
}

// extractWords tokenizes text into lowercase words for fast lookup
func (s *EdgeService) extractWords(text string) map[string]bool {
	words := make(map[string]bool)

	// Simple word extraction (can be improved with proper tokenization)
	text = strings.ToLower(text)
	tokens := strings.Fields(text)

	for _, token := range tokens {
		// Clean token of punctuation
		cleaned := strings.Trim(token, ".,!?;:\"'()[]{}#@$%^&*+=<>/\\|`~")
		if len(cleaned) > 0 {
			words[cleaned] = true
		}
	}

	return words
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
		entityEdgeType = entities.EdgeTypeSimilar
	case "reference":
		entityEdgeType = entities.EdgeTypeReference
	case "parent_child":
		entityEdgeType = entities.EdgeTypeParentChild
	case "sequential":
		entityEdgeType = entities.EdgeTypeSequential
	default:
		entityEdgeType = entities.EdgeTypeSimilar // Default to similar
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
