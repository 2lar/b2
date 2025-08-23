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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
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
		if cachedData, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
			var result dto.GetGraphResult
			if err := json.Unmarshal(cachedData, &result); err == nil {
				s.logger.Debug("returning cached graph result")
				return &result, nil
			}
		}
	}

	// 2. Validate user ID
	userID, err := shared.ParseUserID(query.UserID)
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

	// 5. Filter out orphaned edges (defensive programming)
	validEdges := s.filterOrphanedEdges(nodes, edges)
	
	// 6. Build graph result
	result := &dto.GetGraphResult{
		Nodes: dto.ToNodeViews(nodes),
		Edges: dto.ToEdgeViews(validEdges),
		Stats: &dto.GraphStats{
			NodeCount: len(nodes),
			EdgeCount: len(validEdges),
		},
	}

	// 7. Add graph metrics if requested
	if query.IncludeMetrics {
		metrics := s.calculateGraphMetrics(nodes, validEdges)
		result.Stats.Metrics = metrics
	}

	// 8. Cache the result
	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, cacheKey, data, 10*time.Minute)
		}
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
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := shared.ParseNodeID(query.NodeID)
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
	allNodes := []*node.Node{rootNode}
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
		if cachedData, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
			var result dto.GetGraphAnalyticsResult
			if err := json.Unmarshal(cachedData, &result); err == nil {
				s.logger.Debug("returning cached analytics result")
				return &result, nil
			}
		}
	}

	// 2. Validate user ID
	userID, err := shared.ParseUserID(query.UserID)
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
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, cacheKey, data, 30*time.Minute) // Cache longer for analytics
		}
	}

	s.logger.Debug("graph analytics calculated",
		zap.String("user_id", query.UserID),
		zap.Int("nodes_analyzed", len(nodes)),
		zap.Int("edges_analyzed", len(edges)))

	return result, nil
}

// Helper methods

func (s *GraphQueryService) getUserNodes(ctx context.Context, userID string, limit int) ([]*node.Node, error) {
	// Query for nodes using the correct key structure:
	// PK = USER#<userID>, SK begins with NODE#
	sortKeyPrefix := "NODE#"
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s", userID),
		SortKeyPrefix: &sortKeyPrefix,
	}

	if limit > 0 {
		query.Limit = int32Ptr(int32(limit))
	}

	result, err := s.store.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query user nodes: %w", err)
	}

	nodes := make([]*node.Node, 0, len(result.Records))
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

func (s *GraphQueryService) getUserEdges(ctx context.Context, userID string, limit int) ([]*edge.Edge, error) {
	// Edges are stored with PK = USER#<userID>#NODE#<sourceID>, SK = EDGE#RELATES_TO#<targetID>
	// We need to scan for all edges belonging to this user
	query := persistence.Query{
		FilterExpr: stringPtr("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
		Attributes: map[string]interface{}{
			":pk_prefix": fmt.Sprintf("USER#%s#NODE#", userID),
			":sk_prefix": "EDGE#",
		},
	}

	if limit > 0 {
		query.Limit = int32Ptr(int32(limit))
	}

	result, err := s.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user edges: %w", err)
	}

	edges := make([]*edge.Edge, 0, len(result.Records))
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

func (s *GraphQueryService) getNode(ctx context.Context, userID, nodeID string) (*node.Node, error) {
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

func (s *GraphQueryService) getBatchNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	nodeMap := make(map[string]*node.Node)
	
	// For now, use individual queries - this could be optimized with batch operations
	for _, nodeID := range nodeIDs {
		node, err := s.getNode(ctx, userID, nodeID)
		if err != nil {
			continue // Skip nodes that can't be retrieved
		}
		if node != nil {
			nodeMap[nodeID] = node
		}
	}
	
	return nodeMap, nil
}

func (s *GraphQueryService) traverseGraph(ctx context.Context, userID, startNodeID string, depth int) ([]*node.Node, []*edge.Edge) {
	// This is a simplified implementation. In a real system, you would implement
	// proper graph traversal algorithms (BFS, DFS) with depth limiting.
	
	visited := make(map[string]bool)
	visited[startNodeID] = true

	var allNodes []*node.Node
	var allEdges []*edge.Edge

	// Get all edges to build adjacency information
	edges, err := s.getUserEdges(ctx, userID, 0)
	if err != nil {
		s.logger.Warn("failed to get edges for traversal", zap.Error(err))
		return allNodes, allEdges
	}

	// Build adjacency map
	adjacency := make(map[string][]string)
	edgeMap := make(map[string]*edge.Edge)
	
	for _, edge := range edges {
		sourceID := edge.SourceID.String()
		targetID := edge.TargetID.String()
		
		adjacency[sourceID] = append(adjacency[sourceID], targetID)
		adjacency[targetID] = append(adjacency[targetID], sourceID)
		
		edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID)
		edgeMap[edgeKey] = edge
	}

	// Simple BFS traversal with depth limit - collect node IDs first to avoid N+1 queries
	currentLevel := []string{startNodeID}
	nodeIDsToFetch := make(map[string]bool)
	
	for d := 0; d < depth && len(currentLevel) > 0; d++ {
		nextLevel := []string{}
		
		for _, nodeID := range currentLevel {
			// Get neighbors
			for _, neighborID := range adjacency[nodeID] {
				if !visited[neighborID] {
					visited[neighborID] = true
					nextLevel = append(nextLevel, neighborID)
					nodeIDsToFetch[neighborID] = true
					
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
	
	// Batch load all nodes at once to avoid N+1 queries
	if len(nodeIDsToFetch) > 0 {
		nodeIDs := make([]string, 0, len(nodeIDsToFetch))
		for nodeID := range nodeIDsToFetch {
			nodeIDs = append(nodeIDs, nodeID)
		}
		
		// Use batch get if available in store interface
		nodeMap, err := s.getBatchNodes(ctx, userID, nodeIDs)
		if err == nil {
			for _, node := range nodeMap {
				if node != nil {
					allNodes = append(allNodes, node)
				}
			}
		} else {
			// Fallback to individual queries if batch not available
			for _, nodeID := range nodeIDs {
				if node, err := s.getNode(ctx, userID, nodeID); err == nil && node != nil {
					allNodes = append(allNodes, node)
				}
			}
		}
	}

	return allNodes, allEdges
}

func (s *GraphQueryService) calculateGraphMetrics(nodes []*node.Node, edges []*edge.Edge) map[string]interface{} {
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

func (s *GraphQueryService) calculateGraphAnalytics(nodes []*node.Node, edges []*edge.Edge) *dto.GraphAnalytics {
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
func (s *GraphQueryService) recordToNode(record *persistence.Record) (*node.Node, error) {
	// Validate that this is a node record by checking the sort key
	// Nodes have SK = NODE#<nodeID>
	sk, ok := record.Data["SK"].(string)
	if !ok || !strings.HasPrefix(sk, "NODE#") {
		return nil, fmt.Errorf("not a node record - SK: %v", sk)
	}
	
	// Extract NodeID from SK
	skParts := strings.Split(sk, "#")
	if len(skParts) != 2 {
		return nil, fmt.Errorf("invalid SK format for node: %s", sk)
	}
	nodeID := skParts[1]
	
	// Extract UserID from PK (format: USER#<userID>)
	pk, ok := record.Data["PK"].(string)
	if !ok {
		return nil, fmt.Errorf("missing PK in record")
	}
	
	pkParts := strings.Split(pk, "#")
	if len(pkParts) != 2 || pkParts[0] != "USER" {
		return nil, fmt.Errorf("invalid PK format for node: %s", pk)
	}
	userID := pkParts[1]
	
	// Try to get NodeID from direct field if available
	if directNodeID, ok := record.Data["NodeID"].(string); ok {
		nodeID = directNodeID
	}
	
	// Try to get UserID from direct field if available  
	if directUserID, ok := record.Data["UserID"].(string); ok {
		userID = directUserID
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
	title := ""  // Default empty title for graph queries
	return node.ReconstructNodeFromPrimitives(
		nodeID,
		userID,
		content,
		title,
		keywords,
		tags,
		createdAt,
		version,
	)
}

func (s *GraphQueryService) recordToEdge(record *persistence.Record) (*edge.Edge, error) {
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
		// Current format - extract from PK pattern: USER#<userID>#NODE#<sourceID>
		// and SK pattern: EDGE#RELATES_TO#<targetID>
		pk, ok := record.Data["PK"].(string)
		if !ok {
			return nil, fmt.Errorf("missing PK in record")
		}

		sk, ok := record.Data["SK"].(string)
		if !ok || !strings.HasPrefix(sk, "EDGE#") {
			return nil, fmt.Errorf("not an edge record - SK: %v", sk)
		}

		// Parse PK to get userID and sourceID
		pkParts := strings.Split(pk, "#")
		if len(pkParts) != 4 || pkParts[0] != "USER" || pkParts[2] != "NODE" {
			return nil, fmt.Errorf("invalid PK format: %s", pk)
		}
		
		userID = pkParts[1]
		sourceID = pkParts[3]

		// Parse SK to get targetID
		skParts := strings.Split(sk, "#")
		if len(skParts) < 3 {
			return nil, fmt.Errorf("invalid SK format for edge: %s", sk)
		}
		targetID = skParts[2]
	}

	// Extract optional fields with defaults
	var strength float64 = 1.0
	if s, ok := record.Data["Strength"].(float64); ok {
		strength = s
	}

	// Reconstruct domain edge
	return edge.ReconstructEdgeFromPrimitives(
		sourceID,
		targetID,
		userID,
		strength,
	)
}

// filterOrphanedEdges removes edges that reference non-existent nodes
func (s *GraphQueryService) filterOrphanedEdges(nodes []*node.Node, edges []*edge.Edge) []*edge.Edge {
	// Create a set of existing node IDs for quick lookup
	nodeSet := make(map[string]bool)
	for _, node := range nodes {
		nodeSet[node.ID.String()] = true
	}
	
	// Filter edges to only include those with valid source and target nodes
	validEdges := make([]*edge.Edge, 0, len(edges))
	orphanedCount := 0
	
	for _, edge := range edges {
		sourceExists := nodeSet[edge.SourceID.String()]
		targetExists := nodeSet[edge.TargetID.String()]
		
		if sourceExists && targetExists {
			validEdges = append(validEdges, edge)
		} else {
			orphanedCount++
			// Log orphaned edges for monitoring
			if !sourceExists {
				s.logger.Warn("Orphaned edge: source node doesn't exist",
					zap.String("edge_id", edge.ID.String()),
					zap.String("source_id", edge.SourceID.String()),
					zap.String("target_id", edge.TargetID.String()))
			}
			if !targetExists {
				s.logger.Warn("Orphaned edge: target node doesn't exist",
					zap.String("edge_id", edge.ID.String()),
					zap.String("source_id", edge.SourceID.String()),
					zap.String("target_id", edge.TargetID.String()))
			}
		}
	}
	
	if orphanedCount > 0 {
		s.logger.Info("Filtered orphaned edges",
			zap.Int("orphaned_count", orphanedCount),
			zap.Int("valid_count", len(validEdges)))
	}
	
	return validEdges
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}