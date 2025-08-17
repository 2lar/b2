// Package queries contains query services for graph read operations.
// This GraphQueryService provides graph visualization data and analysis.
//
// Key Concepts Illustrated:
//   - CQRS: Separates read operations from write operations
//   - Query Service Pattern: Optimized for complex graph read scenarios
//   - Direct Store Usage: Uses Store interface for database independence
//   - Graph Analysis: Provides graph metrics and traversal data
//   - Caching: Improves performance for expensive graph operations
package queries

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/persistence"
	appErrors "brain2-backend/pkg/errors"
	"go.uber.org/zap"
)

// GraphQueryService handles read operations for graph data visualization and analysis.
type GraphQueryService struct {
	store  persistence.Store
	logger *zap.Logger
	cache  Cache // Cache interface for performance
}

// NewGraphQueryService creates a new GraphQueryService.
func NewGraphQueryService(
	store persistence.Store,
	logger *zap.Logger,
	cache Cache,
) *GraphQueryService {
	return &GraphQueryService{
		store:  store,
		logger: logger,
		cache:  cache,
	}
}

// GetGraph retrieves a complete graph for a user with nodes and edges.
func (s *GraphQueryService) GetGraph(ctx context.Context, query *GetGraphQuery) (*dto.GetGraphResult, error) {
	s.logger.Debug("getting graph for user", zap.String("user_id", query.UserID))

	// 1. Check cache first if enabled
	cacheKey := fmt.Sprintf("graph:%s:limit=%d", query.UserID, query.Limit)
	if s.cache != nil {
		if cached, found := s.cache.Get(ctx, cacheKey); found {
			s.logger.Debug("returning cached graph result")
			return cached.(*dto.GetGraphResult), nil
		}
	}

	// 2. Validate user ID
	userID, err := domain.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 3. Get nodes for the user
	nodes, err := s.getUserNodes(ctx, userID.String(), query.Limit)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve user nodes")
	}

	// 4. Get edges for the user
	edges, err := s.getUserEdges(ctx, userID.String(), query.Limit)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve user edges")
	}

	// 5. Build graph result
	result := &dto.GetGraphResult{
		Nodes: dto.ToNodeViews(nodes),
		Edges: dto.ToEdgeViews(edges),
		Stats: &dto.GraphStats{
			NodeCount: len(nodes),
			EdgeCount: len(edges),
		},
	}

	// 6. Add graph metrics if requested
	if query.IncludeMetrics {
		metrics := s.calculateGraphMetrics(nodes, edges)
		result.Stats.Metrics = metrics
	}

	// 7. Cache the result
	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, result, 10*time.Minute)
	}

	s.logger.Debug("graph retrieved successfully",
		zap.String("user_id", query.UserID),
		zap.Int("node_count", len(nodes)),
		zap.Int("edge_count", len(edges)))

	return result, nil
}

// GetNodeNeighborhood retrieves nodes connected to a specific node with a given depth.
func (s *GraphQueryService) GetNodeNeighborhood(ctx context.Context, query *GetNodeNeighborhoodQuery) (*dto.GetNodeNeighborhoodResult, error) {
	s.logger.Debug("getting node neighborhood",
		zap.String("user_id", query.UserID),
		zap.String("node_id", query.NodeID),
		zap.Int("depth", query.Depth))

	// 1. Validate inputs
	userID, err := domain.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := domain.ParseNodeID(query.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 2. Check if the root node exists and belongs to the user
	rootNode, err := s.getNode(ctx, userID.String(), nodeID.String())
	if err != nil {
		return nil, err
	}
	if rootNode == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	// 3. Traverse the graph to find connected nodes
	neighborNodes, edges := s.traverseGraph(ctx, userID.String(), nodeID.String(), query.Depth)

	// 4. Include the root node in the result
	allNodes := []*domain.Node{rootNode}
	allNodes = append(allNodes, neighborNodes...)

	// 5. Build result
	result := &dto.GetNodeNeighborhoodResult{
		RootNode: dto.ToNodeView(rootNode),
		Nodes:    dto.ToNodeViews(allNodes),
		Edges:    dto.ToEdgeViews(edges),
		Stats: &dto.NeighborhoodStats{
			TotalNodes: len(allNodes),
			TotalEdges: len(edges),
			Depth:      query.Depth,
		},
	}

	s.logger.Debug("node neighborhood retrieved",
		zap.String("node_id", query.NodeID),
		zap.Int("total_nodes", len(allNodes)),
		zap.Int("total_edges", len(edges)))

	return result, nil
}

// GetGraphAnalytics retrieves analytics and metrics for a user's graph.
func (s *GraphQueryService) GetGraphAnalytics(ctx context.Context, query *GetGraphAnalyticsQuery) (*dto.GetGraphAnalyticsResult, error) {
	s.logger.Debug("getting graph analytics", zap.String("user_id", query.UserID))

	// 1. Check cache first
	cacheKey := fmt.Sprintf("analytics:%s", query.UserID)
	if s.cache != nil {
		if cached, found := s.cache.Get(ctx, cacheKey); found {
			s.logger.Debug("returning cached analytics result")
			return cached.(*dto.GetGraphAnalyticsResult), nil
		}
	}

	// 2. Validate user ID
	userID, err := domain.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 3. Get all nodes and edges for analytics
	nodes, err := s.getUserNodes(ctx, userID.String(), 0) // No limit for analytics
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve nodes for analytics")
	}

	edges, err := s.getUserEdges(ctx, userID.String(), 0) // No limit for analytics
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve edges for analytics")
	}

	// 4. Calculate comprehensive analytics
	analytics := s.calculateGraphAnalytics(nodes, edges)

	// 5. Build result
	result := &dto.GetGraphAnalyticsResult{
		UserID:    query.UserID,
		Analytics: analytics,
		Timestamp: time.Now(),
	}

	// 6. Cache the result
	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, result, 30*time.Minute) // Cache longer for analytics
	}

	s.logger.Debug("graph analytics calculated",
		zap.String("user_id", query.UserID),
		zap.Int("nodes_analyzed", len(nodes)),
		zap.Int("edges_analyzed", len(edges)))

	return result, nil
}

// Helper methods

func (s *GraphQueryService) getUserNodes(ctx context.Context, userID string, limit int) ([]*domain.Node, error) {
	query := persistence.Query{
		FilterExpr: stringPtr("begins_with(PK, :pk_prefix)"),
		Attributes: map[string]interface{}{
			":pk_prefix": fmt.Sprintf("USER#%s#NODE#", userID),
		},
	}

	if limit > 0 {
		query.Limit = int32Ptr(int32(limit))
	}

	result, err := s.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user nodes: %w", err)
	}

	nodes := make([]*domain.Node, 0, len(result.Records))
	for _, record := range result.Records {
		node, err := s.recordToNode(&record)
		if err != nil {
			s.logger.Warn("failed to convert record to node", zap.Error(err))
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (s *GraphQueryService) getUserEdges(ctx context.Context, userID string, limit int) ([]*domain.Edge, error) {
	query := persistence.Query{
		PartitionKey: fmt.Sprintf("USER#%s#EDGE", userID), // Use GSI2PK format
		IndexName:    stringPtr("EdgeIndex"),               // Use GSI2 index
	}

	if limit > 0 {
		query.Limit = int32Ptr(int32(limit))
	}

	result, err := s.store.Query(ctx, query) // Use Query instead of Scan
	if err != nil {
		return nil, fmt.Errorf("failed to query user edges: %w", err)
	}

	edges := make([]*domain.Edge, 0, len(result.Records))
	for _, record := range result.Records {
		edge, err := s.recordToEdge(&record)
		if err != nil {
			s.logger.Warn("failed to convert record to edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

func (s *GraphQueryService) getNode(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID),
		SortKey:      "METADATA#v0",
	}

	record, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if record == nil {
		return nil, nil
	}

	return s.recordToNode(record)
}

func (s *GraphQueryService) traverseGraph(ctx context.Context, userID, startNodeID string, depth int) ([]*domain.Node, []*domain.Edge) {
	// This is a simplified implementation. In a real system, you would implement
	// proper graph traversal algorithms (BFS, DFS) with depth limiting.
	
	visited := make(map[string]bool)
	visited[startNodeID] = true

	var allNodes []*domain.Node
	var allEdges []*domain.Edge

	// Get all edges to build adjacency information
	edges, err := s.getUserEdges(ctx, userID, 0)
	if err != nil {
		s.logger.Warn("failed to get edges for traversal", zap.Error(err))
		return allNodes, allEdges
	}

	// Build adjacency map
	adjacency := make(map[string][]string)
	edgeMap := make(map[string]*domain.Edge)
	
	for _, edge := range edges {
		sourceID := edge.SourceID.String()
		targetID := edge.TargetID.String()
		
		adjacency[sourceID] = append(adjacency[sourceID], targetID)
		adjacency[targetID] = append(adjacency[targetID], sourceID)
		
		edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID)
		edgeMap[edgeKey] = edge
	}

	// Simple BFS traversal with depth limit
	currentLevel := []string{startNodeID}
	
	for d := 0; d < depth && len(currentLevel) > 0; d++ {
		nextLevel := []string{}
		
		for _, nodeID := range currentLevel {
			// Get neighbors
			for _, neighborID := range adjacency[nodeID] {
				if !visited[neighborID] {
					visited[neighborID] = true
					nextLevel = append(nextLevel, neighborID)
					
					// Add the node
					if node, err := s.getNode(ctx, userID, neighborID); err == nil && node != nil {
						allNodes = append(allNodes, node)
					}
					
					// Add the edge
					edgeKey1 := fmt.Sprintf("%s-%s", nodeID, neighborID)
					edgeKey2 := fmt.Sprintf("%s-%s", neighborID, nodeID)
					
					if edge, exists := edgeMap[edgeKey1]; exists {
						allEdges = append(allEdges, edge)
					} else if edge, exists := edgeMap[edgeKey2]; exists {
						allEdges = append(allEdges, edge)
					}
				}
			}
		}
		
		currentLevel = nextLevel
	}

	return allNodes, allEdges
}

func (s *GraphQueryService) calculateGraphMetrics(nodes []*domain.Node, edges []*domain.Edge) map[string]interface{} {
	metrics := make(map[string]interface{})
	
	// Basic counts
	metrics["node_count"] = len(nodes)
	metrics["edge_count"] = len(edges)
	
	// Calculate density
	if len(nodes) > 1 {
		maxPossibleEdges := len(nodes) * (len(nodes) - 1) / 2
		density := float64(len(edges)) / float64(maxPossibleEdges)
		metrics["density"] = density
	} else {
		metrics["density"] = 0.0
	}
	
	// Calculate degree distribution
	degreeCount := make(map[string]int)
	for _, edge := range edges {
		sourceID := edge.SourceID.String()
		targetID := edge.TargetID.String()
		degreeCount[sourceID]++
		degreeCount[targetID]++
	}
	
	// Average degree
	totalDegree := 0
	for _, degree := range degreeCount {
		totalDegree += degree
	}
	
	if len(nodes) > 0 {
		metrics["avg_degree"] = float64(totalDegree) / float64(len(nodes))
	} else {
		metrics["avg_degree"] = 0.0
	}
	
	// Max degree
	maxDegree := 0
	for _, degree := range degreeCount {
		if degree > maxDegree {
			maxDegree = degree
		}
	}
	metrics["max_degree"] = maxDegree
	
	return metrics
}

func (s *GraphQueryService) calculateGraphAnalytics(nodes []*domain.Node, edges []*domain.Edge) *dto.GraphAnalytics {
	analytics := &dto.GraphAnalytics{
		NodeCount: len(nodes),
		EdgeCount: len(edges),
		Metrics:   s.calculateGraphMetrics(nodes, edges),
	}
	
	// Calculate additional analytics
	if len(nodes) > 0 {
		// Most connected nodes
		degreeCount := make(map[string]int)
		for _, edge := range edges {
			sourceID := edge.SourceID.String()
			targetID := edge.TargetID.String()
			degreeCount[sourceID]++
			degreeCount[targetID]++
		}
		
		// Find top 5 most connected nodes
		type nodeRank struct {
			NodeID string
			Degree int
		}
		
		var ranks []nodeRank
		for nodeID, degree := range degreeCount {
			ranks = append(ranks, nodeRank{NodeID: nodeID, Degree: degree})
		}
		
		// Sort by degree (simple bubble sort for small datasets)
		for i := 0; i < len(ranks)-1; i++ {
			for j := i + 1; j < len(ranks); j++ {
				if ranks[i].Degree < ranks[j].Degree {
					ranks[i], ranks[j] = ranks[j], ranks[i]
				}
			}
		}
		
		// Take top 5
		topNodes := make([]map[string]interface{}, 0)
		for i := 0; i < len(ranks) && i < 5; i++ {
			topNodes = append(topNodes, map[string]interface{}{
				"node_id": ranks[i].NodeID,
				"degree":  ranks[i].Degree,
			})
		}
		analytics.TopConnectedNodes = topNodes
	}
	
	return analytics
}

// Helper functions for domain object reconstruction
func (s *GraphQueryService) recordToNode(record *persistence.Record) (*domain.Node, error) {
	// First, validate that this is actually a node record by checking the sort key
	sk, ok := record.Data["SK"].(string)
	if !ok || !strings.HasPrefix(sk, "METADATA#") {
		return nil, fmt.Errorf("not a node record - SK: %v", sk)
	}
	
	// Skip keyword records and other non-metadata records
	if strings.Contains(sk, "KEYWORD#") || strings.Contains(sk, "IDEMPOTENCY#") {
		return nil, fmt.Errorf("skipping non-node record type")
	}
	
	// Extract NodeID - try direct field first, then parse from PK
	nodeID, hasNodeID := record.Data["NodeID"].(string)
	var userID string
	
	if !hasNodeID {
		// Extract from PK pattern: USER#<userID>#NODE#<nodeID>
		pk, ok := record.Data["PK"].(string)
		if !ok {
			return nil, fmt.Errorf("missing PK in record")
		}
		
		pkParts := strings.Split(pk, "#")
		if len(pkParts) != 4 || pkParts[0] != "USER" || pkParts[2] != "NODE" {
			return nil, fmt.Errorf("invalid PK format for node: %s", pk)
		}
		
		userID = pkParts[1]
		nodeID = pkParts[3]
	} else {
		// NodeID exists directly, extract UserID
		userIDVal, ok := record.Data["UserID"].(string)
		if !ok {
			return nil, fmt.Errorf("missing UserID in record")
		}
		userID = userIDVal
	}

	content, ok := record.Data["Content"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Content in record")
	}

	// Extract optional fields with defaults
	var keywords []string
	if kw, ok := record.Data["Keywords"].([]interface{}); ok {
		keywords = make([]string, len(kw))
		for i, k := range kw {
			if s, ok := k.(string); ok {
				keywords[i] = s
			}
		}
	}

	var tags []string
	if tg, ok := record.Data["Tags"].([]interface{}); ok {
		tags = make([]string, len(tg))
		for i, t := range tg {
			if s, ok := t.(string); ok {
				tags[i] = s
			}
		}
	}

	var version int
	if v, ok := record.Data["Version"].(int); ok {
		version = v
	}

	// Parse timestamp
	var createdAt time.Time
	if ts, ok := record.Data["Timestamp"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, ts)
	} else {
		createdAt = record.CreatedAt
	}

	// Reconstruct domain node
	return domain.ReconstructNodeFromPrimitives(
		nodeID,
		userID,
		content,
		keywords,
		tags,
		createdAt,
		version,
	)
}

func (s *GraphQueryService) recordToEdge(record *persistence.Record) (*domain.Edge, error) {
	// Try to extract sourceID and targetID directly from record fields first (EdgeIndex GSI format)
	sourceID, hasSourceID := record.Data["SourceID"].(string)
	targetID, hasTargetID := record.Data["TargetID"].(string)
	
	var userID string
	
	if hasSourceID && hasTargetID {
		// EdgeIndex GSI format - extract userID from PK
		pk, ok := record.Data["PK"].(string)
		if !ok {
			return nil, fmt.Errorf("missing PK in record")
		}
		
		// Parse PK pattern for EdgeIndex: USER#<userID>#EDGE or similar
		pkParts := strings.Split(pk, "#")
		if len(pkParts) < 2 || pkParts[0] != "USER" {
			return nil, fmt.Errorf("invalid PK format for edge: %s", pk)
		}
		
		userID = pkParts[1]
	} else {
		// Legacy format - extract from PK pattern: USER#<userID>#NODE#<sourceID>
		pk, ok := record.Data["PK"].(string)
		if !ok {
			return nil, fmt.Errorf("missing PK in record")
		}

		pkParts := strings.Split(pk, "#")
		if len(pkParts) != 4 || pkParts[0] != "USER" || pkParts[2] != "NODE" {
			return nil, fmt.Errorf("invalid PK format: %s", pk)
		}
		
		userID = pkParts[1]
		sourceID = pkParts[3]

		// Extract TargetID from record
		targetIDVal, ok := record.Data["TargetID"].(string)
		if !ok {
			return nil, fmt.Errorf("missing TargetID in record")
		}
		targetID = targetIDVal
	}

	// Extract optional fields with defaults
	var strength float64 = 1.0
	if s, ok := record.Data["Strength"].(float64); ok {
		strength = s
	}

	// Reconstruct domain edge
	return domain.ReconstructEdgeFromPrimitives(
		sourceID,
		targetID,
		userID,
		strength,
	)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}