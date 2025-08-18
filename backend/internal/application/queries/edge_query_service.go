package queries

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// EdgeQueryService handles read operations for edges with caching and optimization.
// This service provides specialized methods for querying graph connections.
type EdgeQueryService struct {
	// Read-only dependencies
	edgeReader repository.EdgeReader // Focused interface for reading edges
	nodeReader repository.NodeReader // For enriching edge data with node information
	cache      Cache                 // Cache interface for performance
}

// NewEdgeQueryService creates a new EdgeQueryService with all required dependencies.
func NewEdgeQueryService(
	edgeReader repository.EdgeReader,
	nodeReader repository.NodeReader,
	cache Cache,
) *EdgeQueryService {
	return &EdgeQueryService{
		edgeReader: edgeReader,
		nodeReader: nodeReader,
		cache:      cache,
	}
}

// GetEdge retrieves a specific edge with optional enrichment data.
func (s *EdgeQueryService) GetEdge(ctx context.Context, query *GetEdgeQuery) (*dto.GetEdgeResult, error) {
	// 1. Check cache first
	cacheKey := fmt.Sprintf("edge:%s:%s->%s:nodes=%t",
		query.UserID, query.SourceNodeID, query.TargetNodeID, query.IncludeNodes)

	if s.cache != nil {
		if cached, found := s.cache.Get(ctx, cacheKey); found {
			return cached.(*dto.GetEdgeResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	sourceID, err := shared.ParseNodeID(query.SourceNodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid source node id: " + err.Error())
	}

	targetID, err := shared.ParseNodeID(query.TargetNodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid target node id: " + err.Error())
	}

	// 3. Find the edge between the specified nodes
	edges, err := s.edgeReader.FindBetweenNodes(ctx, sourceID, targetID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find edge")
	}

	if len(edges) == 0 {
		return nil, appErrors.NewNotFound("edge not found")
	}

	edge := edges[0] // Take the first edge if multiple exist

	// 4. Verify ownership
	if !edge.UserID().Equals(userID) {
		return nil, appErrors.NewUnauthorized("edge belongs to different user")
	}

	// 5. Build result
	result := &dto.GetEdgeResult{
		Edge: dto.ToConnectionView(edge),
	}

	// 6. Include node data if requested
	if query.IncludeNodes {
		sourceNode, err := s.nodeReader.FindByID(ctx, sourceID)
		if err == nil && sourceNode != nil {
			result.SourceNode = dto.ToNodeView(sourceNode)
		}

		targetNode, err := s.nodeReader.FindByID(ctx, targetID)
		if err == nil && targetNode != nil {
			result.TargetNode = dto.ToNodeView(targetNode)
		}
	}

	// 7. Cache the result
	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, result, 5*time.Minute)
	}

	return result, nil
}

// ListEdges retrieves a paginated list of edges with optional filtering.
func (s *EdgeQueryService) ListEdges(ctx context.Context, query *ListEdgesQuery) (*dto.ListEdgesResult, error) {
	// 1. Parse and validate domain identifiers
	_, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 2. Build repository query from application query
	edgeQuery := repository.EdgeQuery{
		UserID: query.UserID,
	}

	// Add node filtering if specified
	if query.SourceNodeID != "" {
		edgeQuery.SourceID = query.SourceNodeID
	}

	if query.TargetNodeID != "" {
		edgeQuery.TargetID = query.TargetNodeID
	}

	if len(query.NodeIDs) > 0 {
		edgeQuery.NodeIDs = query.NodeIDs
	}

	// 3. Add weight filtering
	// TODO: Add MinWeight and MaxWeight fields to EdgeQuery when needed
	// if query.MinWeight > 0 {
	// 	edgeQuery.MinWeight = query.MinWeight
	// }
	//
	// if query.MaxWeight > 0 {
	// 	edgeQuery.MaxWeight = query.MaxWeight
	// }

	// 4. Build pagination parameters
	pagination := repository.Pagination{
		Limit:  query.Limit,
		Cursor: query.NextToken,
	}

	// Add sorting if specified
	if query.SortBy != "" {
		pagination.SortBy = query.SortBy
		pagination.SortDirection = query.SortDirection
	}

	// 5. Execute query
	page, err := s.edgeReader.FindPage(ctx, edgeQuery, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve edges page")
	}

	if page == nil {
		return &dto.ListEdgesResult{
			Edges:   []*dto.ConnectionView{},
			HasMore: false,
			Total:   0,
			Count:   0,
		}, nil
	}

	// 6. Convert domain edges to view models
	edgeViews := dto.ToConnectionViews(page.Items)

	// 7. Build paginated result
	result := &dto.ListEdgesResult{
		Edges:     edgeViews,
		NextToken: page.NextCursor,
		HasMore:   page.HasMore,
		Total:     len(edgeViews), // For now, use current count
		Count:     len(edgeViews),
	}

	return result, nil
}

// GetConnectionStatistics retrieves statistics about connections for a user.
func (s *EdgeQueryService) GetConnectionStatistics(ctx context.Context, query *GetConnectionStatisticsQuery) (*dto.ConnectionStatisticsResult, error) {
	// 1. Check cache first - statistics are expensive to compute
	cacheKey := fmt.Sprintf("connection_stats:%s:strong=%.2f:weak=%.2f",
		query.UserID, query.StrongConnectionThreshold, query.WeakConnectionThreshold)

	if s.cache != nil {
		if cached, found := s.cache.Get(ctx, cacheKey); found {
			return cached.(*dto.ConnectionStatisticsResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 3. Get all edges for the user
	edges, err := s.edgeReader.FindByUser(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve user edges")
	}

	// 4. Calculate statistics
	totalEdges := len(edges)
	strongConnections := 0
	weakConnections := 0
	var totalWeight float64
	connectionCounts := make(map[string]int)

	for _, edge := range edges {
		weight := edge.Weight()
		totalWeight += weight

		// Count connections per node
		sourceID := edge.SourceID.String()
		targetID := edge.TargetID.String()
		connectionCounts[sourceID]++
		connectionCounts[targetID]++

		// Classify connection strength
		if weight >= query.StrongConnectionThreshold {
			strongConnections++
		} else if weight <= query.WeakConnectionThreshold {
			weakConnections++
		}
	}

	// 5. Calculate averages and density
	averageWeight := float64(0)
	if totalEdges > 0 {
		averageWeight = totalWeight / float64(totalEdges)
	}

	// Calculate average connections per node
	totalNodes := len(connectionCounts)
	averageConnections := float64(0)
	if totalNodes > 0 {
		totalConnectionCount := 0
		for _, count := range connectionCounts {
			totalConnectionCount += count
		}
		averageConnections = float64(totalConnectionCount) / float64(totalNodes)
	}

	// Calculate connection density (edges / possible edges)
	density := float64(0)
	if totalNodes > 1 {
		possibleEdges := totalNodes * (totalNodes - 1) / 2 // Undirected graph
		density = float64(totalEdges) / float64(possibleEdges)
	}

	// 6. Find most connected nodes
	type nodeConnection struct {
		NodeID          string
		ConnectionCount int
	}

	var mostConnected []nodeConnection
	for nodeID, count := range connectionCounts {
		mostConnected = append(mostConnected, nodeConnection{
			NodeID:          nodeID,
			ConnectionCount: count,
		})
	}

	// Sort by connection count (simple implementation)
	// In production, you might want a more efficient sorting approach
	for i := 0; i < len(mostConnected)-1; i++ {
		for j := i + 1; j < len(mostConnected); j++ {
			if mostConnected[j].ConnectionCount > mostConnected[i].ConnectionCount {
				mostConnected[i], mostConnected[j] = mostConnected[j], mostConnected[i]
			}
		}
	}

	// Limit to top 10
	if len(mostConnected) > 10 {
		mostConnected = mostConnected[:10]
	}

	// 7. Build result
	result := &dto.ConnectionStatisticsResult{
		TotalEdges:           totalEdges,
		StrongConnections:    strongConnections,
		WeakConnections:      weakConnections,
		AverageWeight:        averageWeight,
		AverageConnections:   averageConnections,
		ConnectionDensity:    density,
		TotalNodes:           totalNodes,
		MostConnectedNodes:   make([]dto.NodeConnectionInfo, len(mostConnected)),
		CalculatedAt:         time.Now(),
	}

	for i, nc := range mostConnected {
		result.MostConnectedNodes[i] = dto.NodeConnectionInfo{
			NodeID:          nc.NodeID,
			ConnectionCount: nc.ConnectionCount,
		}
	}

	// 8. Cache the result - statistics change less frequently
	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, result, 15*time.Minute)
	}

	return result, nil
}

// GetNodeConnections retrieves all connections for a specific node with enriched data.
func (s *EdgeQueryService) GetNodeConnections(ctx context.Context, query *GetNodeConnectionsQuery) (*dto.GetNodeConnectionsResult, error) {
	// 1. Check cache first
	cacheKey := fmt.Sprintf("node_connections_detailed:%s:%s:type=%s:limit=%d",
		query.UserID, query.NodeID, query.ConnectionType, query.Limit)

	if s.cache != nil {
		if cached, found := s.cache.Get(ctx, cacheKey); found {
			return cached.(*dto.GetNodeConnectionsResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := shared.ParseNodeID(query.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 3. Get connections based on type
	var edges []*edge.Edge

	switch query.ConnectionType {
	case "outgoing":
		edges, err = s.edgeReader.FindBySourceNode(ctx, nodeID)
	case "incoming":
		edges, err = s.edgeReader.FindByTargetNode(ctx, nodeID)
	case "bidirectional":
		outgoing, err1 := s.edgeReader.FindBySourceNode(ctx, nodeID)
		incoming, err2 := s.edgeReader.FindByTargetNode(ctx, nodeID)
		if err1 != nil {
			err = err1
		} else if err2 != nil {
			err = err2
		} else {
			edges = append(outgoing, incoming...)
		}
	default:
		return nil, appErrors.NewValidation("invalid connection type")
	}

	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve connections")
	}

	// 4. Filter by user ownership
	var userEdges []*edge.Edge
	for _, edge := range edges {
		if edge.UserID().Equals(userID) {
			userEdges = append(userEdges, edge)
		}
	}

	// 5. Apply limit
	if query.Limit > 0 && len(userEdges) > query.Limit {
		userEdges = userEdges[:query.Limit]
	}

	// 6. Build result
	result := &dto.GetNodeConnectionsResult{
		NodeID:      query.NodeID,
		Connections: dto.ToConnectionViews(userEdges),
		Count:       len(userEdges),
	}

	// 7. Include enriched node data if requested
	// TODO: Add IncludeNodeData field to GetNodeConnectionsQuery when needed
	if true { // Always include node data for now
		nodeDataMap := make(map[string]*dto.NodeView)

		for _, edge := range userEdges {
			// Get source node data
			if sourceNode, err := s.nodeReader.FindByID(ctx, edge.SourceID); err == nil && sourceNode != nil {
				nodeDataMap[edge.SourceID.String()] = dto.ToNodeView(sourceNode)
			}

			// Get target node data
			if targetNode, err := s.nodeReader.FindByID(ctx, edge.TargetID); err == nil && targetNode != nil {
				nodeDataMap[edge.TargetID.String()] = dto.ToNodeView(targetNode)
			}
		}

		result.EnrichedNodes = nodeDataMap
	}

	// 8. Cache the result
	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, result, 3*time.Minute)
	}

	return result, nil
}

// InvalidateEdgeCache invalidates cached data for edge-related operations.
func (s *EdgeQueryService) InvalidateEdgeCache(ctx context.Context, userID string, nodeIDs ...string) {
	if s.cache == nil {
		return
	}

	// Invalidate various cache keys related to edges
	patterns := []string{
		fmt.Sprintf("connection_stats:%s:*", userID),
		fmt.Sprintf("node_connections_detailed:%s:*", userID),
	}

	// Add specific node cache invalidations
	for _, nodeID := range nodeIDs {
		patterns = append(patterns, fmt.Sprintf("edge:%s:*:%s*", userID, nodeID))
		patterns = append(patterns, fmt.Sprintf("edge:%s:*->%s*", userID, nodeID))
	}

	for _, pattern := range patterns {
		s.cache.Delete(ctx, pattern)
	}
}

// InvalidateUserEdgeCache invalidates all edge-related cached data for a user.
func (s *EdgeQueryService) InvalidateUserEdgeCache(ctx context.Context, userID string) {
	if s.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("edge:%s:*", userID),
		fmt.Sprintf("connection_stats:%s:*", userID),
		fmt.Sprintf("node_connections_detailed:%s:*", userID),
	}

	for _, pattern := range patterns {
		s.cache.Delete(ctx, pattern)
	}
}