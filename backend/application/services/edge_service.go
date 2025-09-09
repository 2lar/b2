package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/infrastructure/config"
	"go.uber.org/zap"
)

// EdgeCandidate represents a potential edge to be created
type EdgeCandidate struct {
	SourceID   valueobjects.NodeID
	TargetID   valueobjects.NodeID
	Type       entities.EdgeType
	Similarity float64
}

// EdgeService provides simple, direct edge creation functionality
// This service is used internally by Lambda functions for efficient edge creation
// without the overhead of the command bus
type EdgeService struct {
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	edgeRepo  ports.EdgeRepository
	config    *config.EdgeCreationConfig
	logger    *zap.Logger
}

// NewEdgeService creates a new edge service
func NewEdgeService(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	config *config.EdgeCreationConfig,
	logger *zap.Logger,
) *EdgeService {
	return &EdgeService{
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		edgeRepo:  edgeRepo,
		config:    config,
		logger:    logger,
	}
}

// DiscoverEdges analyzes a node and returns edge candidates split into sync and async groups
// This supports the hybrid edge creation approach where critical edges are created synchronously
// and the rest are created asynchronously
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

	// Get all nodes in the graph
	existingNodes, err := graph.Nodes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get nodes from graph: %w", err)
	}

	if len(existingNodes) <= 1 {
		// No other nodes to connect to
		return nil, nil, nil
	}

	// Extract keywords and tags from the source node
	nodeContent := node.Content()
	keywords := s.extractWords(nodeContent.Title() + " " + nodeContent.Body())
	tags := node.GetTags()

	// Pre-process for O(1) lookups
	keywordSet := make(map[string]bool)
	for word := range keywords {
		keywordSet[word] = true
	}
	
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[strings.ToLower(tag)] = true
	}

	// Calculate similarity scores for all nodes
	var candidates []EdgeCandidate
	
	for _, targetNode := range existingNodes {
		// Skip self
		if targetNode.ID() == node.ID() {
			continue
		}

		// Calculate similarity
		similarity := s.calculateSimilarity(targetNode, keywordSet, tagSet)
		
		// Only consider if above threshold
		if similarity > s.config.SimilarityThreshold {
			candidates = append(candidates, EdgeCandidate{
				SourceID:   node.ID(),
				TargetID:   targetNode.ID(),
				Type:       entities.EdgeTypeSimilar,
				Similarity: similarity,
			})
		}
	}

	// Sort by similarity (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Similarity > candidates[j].Similarity
	})

	// Apply max edges limit
	if len(candidates) > s.config.MaxEdgesPerNode {
		candidates = candidates[:s.config.MaxEdgesPerNode]
	}

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

	// Get all nodes in the graph to find similar ones
	existingNodes, err := graph.Nodes()
	if err != nil {
		// For large graphs, fall back to pagination or skip auto-connection
		return nil, fmt.Errorf("graph too large for automatic edge creation: %w", err)
	}
	
	s.logger.Info("Checking nodes for edge creation",
		zap.String("nodeID", nodeID),
		zap.String("graphID", graphID),
		zap.Int("existingNodes", len(existingNodes)),
	)
	
	if len(existingNodes) <= 1 {
		// No other nodes to connect to
		s.logger.Info("Not enough nodes to create edges",
			zap.String("nodeID", nodeID),
			zap.Int("nodeCount", len(existingNodes)),
		)
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

	// Use configured limits
	maxEdges := s.config.MaxEdgesPerNode
	similarityThreshold := s.config.SimilarityThreshold

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
		
		s.logger.Debug("Calculated similarity",
			zap.String("sourceNode", nodeID),
			zap.String("targetNode", targetNode.ID().String()),
			zap.Float64("similarity", similarity),
			zap.Float64("threshold", similarityThreshold),
		)

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

	s.logger.Info("Found similar nodes for edge creation",
		zap.String("nodeID", nodeID),
		zap.Int("similarNodes", len(similarities)),
		zap.Int("maxEdges", maxEdges),
	)

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
