// Package queries contains query services for read operations.
// Query services implement the read side of Command/Query Responsibility Segregation (CQRS).
//
// Key Concepts Illustrated:
//   - CQRS: Separates read operations from write operations
//   - Query Service Pattern: Optimized for read scenarios
//   - Caching: Improves performance for frequently accessed data
//   - View Models: Data structures optimized for presentation
//   - Read-Only Operations: No side effects, focused on data retrieval
//   - Performance Optimization: Efficient queries and caching strategies
package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/errors"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NodeQueryService handles read operations for nodes with caching and optimization.
// This service is separate from the command service to allow for different optimizations.
type NodeQueryService struct {
	// Read-only dependencies
	nodeReader repository.NodeReader // Focused interface for reading nodes
	edgeReader repository.EdgeReader // Focused interface for reading edges
	graphRepo  repository.GraphRepository // For complex graph queries
	cache      Cache                 // Cache interface for performance
	tracer     trace.Tracer          // For distributed tracing
}

// NewNodeQueryService creates a new NodeQueryService with all required dependencies.
func NewNodeQueryService(
	nodeReader repository.NodeReader,
	edgeReader repository.EdgeReader,
	graphRepo repository.GraphRepository,
	cache Cache,
) *NodeQueryService {
	return &NodeQueryService{
		nodeReader: nodeReader,
		edgeReader: edgeReader,
		graphRepo:  graphRepo,
		cache:      cache,
		tracer:     otel.Tracer("brain2-backend.queries.node_query_service"),
	}
}

// GetNode retrieves a single node with optional connections and metadata.
// This method demonstrates caching and view model optimization.
func (s *NodeQueryService) GetNode(ctx context.Context, query *GetNodeQuery) (*dto.GetNodeResult, error) {
	// Start tracing span
	ctx, span := s.tracer.Start(ctx, "NodeQueryService.GetNode",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("user.id", query.UserID),
			attribute.String("node.id", query.NodeID),
			attribute.Bool("include.connections", query.IncludeConnections),
			attribute.Bool("include.metadata", query.IncludeMetadata),
		),
	)
	defer span.End()

	// 1. Check cache first for performance
	cacheKey := fmt.Sprintf("node:%s:%s:conn=%t:meta=%t", 
		query.UserID, query.NodeID, query.IncludeConnections, query.IncludeMetadata)
	
	if s.cache != nil {
		cacheCtx, cacheSpan := s.tracer.Start(ctx, "cache.get",
			trace.WithAttributes(attribute.String("cache.key", cacheKey)))
		
		if cachedData, found, err := s.cache.Get(cacheCtx, cacheKey); err == nil && found {
			var result dto.GetNodeResult
			if err := json.Unmarshal(cachedData, &result); err == nil {
				cacheSpan.SetAttributes(attribute.Bool("cache.hit", true))
				cacheSpan.End()
				span.AddEvent("cache_hit")
				return &result, nil
			}
		}
		cacheSpan.SetAttributes(attribute.Bool("cache.hit", false))
		cacheSpan.End()
		span.AddEvent("cache_miss")
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid user id: " + err.Error()).Build()
	}

	nodeID, err := shared.ParseNodeID(query.NodeID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid node id: " + err.Error()).Build()
	}

	// 3. Retrieve node from repository - userID passed explicitly
	node, err := s.nodeReader.FindByID(ctx, userID, nodeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve node")
	}
	if node == nil {
		return nil, errors.NotFound(errors.CodeNodeNotFound.String(), "node not found").Build()
	}

	// 4. Verify ownership
	if !node.UserID().Equals(userID) {
		return nil, errors.Unauthorized(errors.CodeUserUnauthorized.String(), "node belongs to different user").Build()
	}

	// 5. Build result with optional components
	result := &dto.GetNodeResult{
		Node: dto.ToNodeView(node),
	}

	// 6. Include connections if requested
	if query.IncludeConnections {
		// Get outgoing edges (where this node is the source)
		outgoingQuery := repository.EdgeQuery{
			UserID:   query.UserID,
			SourceID: query.NodeID,
		}
		outgoingEdges, err := s.edgeReader.FindEdges(ctx, outgoingQuery)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve outgoing connections")
		}
		
		// Get incoming edges (where this node is the target)
		incomingQuery := repository.EdgeQuery{
			UserID:   query.UserID,
			TargetID: query.NodeID,
		}
		incomingEdges, err := s.edgeReader.FindEdges(ctx, incomingQuery)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve incoming connections")
		}
		
		// Combine both sets of edges
		allEdges := append(outgoingEdges, incomingEdges...)
		result.Connections = dto.ToConnectionViews(allEdges)
	}

	// 7. Include metadata if requested
	if query.IncludeMetadata {
		connectionCount := 0
		if result.Connections != nil {
			connectionCount = len(result.Connections)
		} else {
			// Get connection count without loading all connections
			count, err := s.edgeReader.CountBySourceID(ctx, nodeID)
			if err == nil {
				connectionCount = count
			}
		}

		result.Metadata = &dto.NodeMetadata{
			WordCount:       node.Content().WordCount(),
			KeywordCount:    len(node.Keywords().ToSlice()),
			TagCount:        len(node.Tags().ToSlice()),
			ConnectionCount: connectionCount,
			LastModified:    node.UpdatedAt(),
			Version:         node.Version(),
		}
	}

	// 8. Cache the result for future requests
	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
		}
	}

	return result, nil
}

// ListNodes retrieves a paginated list of nodes with optional filtering and sorting.
func (s *NodeQueryService) ListNodes(ctx context.Context, query *ListNodesQuery) (*dto.ListNodesResult, error) {
	// 1. Parse and validate domain identifiers
	_, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid user id: " + err.Error()).Build()
	}

	// 2. Build repository query from application query
	nodeQuery := repository.NodeQuery{
		UserID: query.UserID,
	}

	// Add filtering if specified
	if len(query.TagFilter) > 0 {
		nodeQuery.Tags = query.TagFilter
	}

	if query.SearchQuery != "" {
		nodeQuery.SearchText = query.SearchQuery
	}

	// 3. Build pagination parameters
	pagination := repository.Pagination{
		Limit:  query.Limit,
		Cursor: query.NextToken,
	}

	// Add sorting if specified
	if query.SortBy != "" {
		pagination.SortBy = query.SortBy
		pagination.SortDirection = query.SortDirection
	}

	// 4. Execute query - userID is in the query struct
	page, err := s.nodeReader.GetNodesPage(ctx, nodeQuery, pagination)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve nodes page")
	}

	if page == nil {
		return &dto.ListNodesResult{
			Nodes:   []*dto.NodeView{},
			HasMore: false,
			Total:   0,
			Count:   0,
		}, nil
	}

	// 5. Get total count for pagination metadata
	total, err := s.nodeReader.CountNodes(ctx, query.UserID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to count total nodes")
	}

	// 6. Convert domain nodes to view models
	nodeViews := dto.ToNodeViews(page.Items)

	// 7. Build paginated result
	result := &dto.ListNodesResult{
		Nodes:     nodeViews,
		NextToken: page.NextCursor,
		HasMore:   page.HasMore,
		Total:     total,
		Count:     len(nodeViews),
	}

	return result, nil
}

// GetNodeConnections retrieves connections for a specific node.
func (s *NodeQueryService) GetNodeConnections(ctx context.Context, query *GetNodeConnectionsQuery) (*dto.GetNodeConnectionsResult, error) {
	// 1. Check cache first
	cacheKey := fmt.Sprintf("node_connections:%s:%s:type=%s:limit=%d", 
		query.UserID, query.NodeID, query.ConnectionType, query.Limit)
	
	if s.cache != nil {
		if cachedData, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
			var result dto.GetNodeConnectionsResult
			if err := json.Unmarshal(cachedData, &result); err == nil {
				return &result, nil
			}
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid user id: " + err.Error()).Build()
	}

	nodeID, err := shared.ParseNodeID(query.NodeID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid node id: " + err.Error()).Build()
	}

	// 3. Verify node exists and user owns it
	node, err := s.nodeReader.FindByID(ctx, userID, nodeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to find node")
	}
	if node == nil {
		return nil, errors.NotFound(errors.CodeNodeNotFound.String(), "node not found").Build()
	}
	if !node.UserID().Equals(userID) {
		return nil, errors.Unauthorized(errors.CodeUserUnauthorized.String(), "node belongs to different user").Build()
	}

	// 4. Build edge query based on connection type
	var edgeQuery repository.EdgeQuery
	switch query.ConnectionType {
	case "outgoing":
		edgeQuery = repository.EdgeQuery{
			UserID:   query.UserID,
			SourceID: query.NodeID,
			Limit:    query.Limit,
		}
	case "incoming":
		edgeQuery = repository.EdgeQuery{
			UserID:   query.UserID,
			TargetID: query.NodeID,
			Limit:    query.Limit,
		}
	case "bidirectional":
		// For bidirectional, we need to query both directions
		outgoingEdges, err := s.edgeReader.FindEdges(ctx, repository.EdgeQuery{
			UserID:   query.UserID,
			SourceID: query.NodeID,
			Limit:    query.Limit / 2,
		})
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve outgoing connections")
		}

		incomingEdges, err := s.edgeReader.FindEdges(ctx, repository.EdgeQuery{
			UserID:   query.UserID,
			TargetID: query.NodeID,
			Limit:    query.Limit / 2,
		})
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve incoming connections")
		}

		// Combine both directions
		allEdges := append(outgoingEdges, incomingEdges...)
		result := &dto.GetNodeConnectionsResult{
			NodeID:      query.NodeID,
			Connections: dto.ToConnectionViews(allEdges),
			Count:       len(allEdges),
		}

		// Cache the result
		if s.cache != nil {
			if data, err := json.Marshal(result); err == nil {
				s.cache.Set(ctx, cacheKey, data, 2*time.Minute)
			}
		}

		return result, nil
	default:
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid connection type").Build()
	}

	// 5. Execute query for single direction
	edges, err := s.edgeReader.FindEdges(ctx, edgeQuery)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve connections")
	}

	// 6. Build result
	result := &dto.GetNodeConnectionsResult{
		NodeID:      query.NodeID,
		Connections: dto.ToConnectionViews(edges),
		Count:       len(edges),
	}

	// 7. Cache the result
	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, cacheKey, data, 2*time.Minute)
		}
	}

	return result, nil
}

// GetGraphData retrieves the complete graph data for a user.
func (s *NodeQueryService) GetGraphData(ctx context.Context, query *GetGraphDataQuery) (*dto.GetGraphDataResult, error) {
	// 1. Check cache first - graph data is expensive to compute
	cacheKey := fmt.Sprintf("graph:%s:archived=%t:nodes=%d:edges=%d", 
		query.UserID, query.IncludeArchived, query.MaxNodes, query.MaxEdges)
	
	if s.cache != nil {
		if cachedData, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
			var result dto.GetGraphDataResult
			if err := json.Unmarshal(cachedData, &result); err == nil {
				return &result, nil
			}
		}
	}

	// 2. Parse and validate domain identifiers
	_, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, errors.Validation(errors.CodeValidationFailed.String(), "invalid user id: " + err.Error()).Build()
	}

	// 3. Build graph query
	graphQuery := repository.GraphQuery{
		UserID:          query.UserID,
		IncludeEdges:    true,
		IncludeArchived: query.IncludeArchived,
		MaxNodes:        query.MaxNodes,
		MaxEdges:        query.MaxEdges,
	}

	// Add tag filtering if specified
	if len(query.TagFilter) > 0 {
		graphQuery.TagFilter = query.TagFilter
	}

	// 4. Retrieve graph data from repository
	graph, err := s.graphRepo.GetGraphData(ctx, graphQuery)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError.String(), "failed to retrieve graph data")
	}

	// 5. Convert to view model with statistics
	graphView := dto.ToGraphView(graph)

	result := &dto.GetGraphDataResult{
		Graph: graphView,
	}

	// 6. Cache the result - graph data changes less frequently
	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			s.cache.Set(ctx, cacheKey, data, 10*time.Minute)
		}
	}

	return result, nil
}

// InvalidateNodeCache invalidates cached data for a specific node.
// This method should be called when a node is updated or deleted.
func (s *NodeQueryService) InvalidateNodeCache(ctx context.Context, userID, nodeID string) {
	if s.cache == nil {
		return
	}

	// Invalidate various cache keys related to this node
	patterns := []string{
		fmt.Sprintf("node:%s:%s:*", userID, nodeID),
		fmt.Sprintf("node_connections:%s:%s:*", userID, nodeID),
		fmt.Sprintf("graph:%s:*", userID),
	}

	for _, pattern := range patterns {
		s.cache.Delete(ctx, pattern)
	}
}

// InvalidateUserCache invalidates all cached data for a user.
// This method should be called when bulk operations are performed.
func (s *NodeQueryService) InvalidateUserCache(ctx context.Context, userID string) {
	if s.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("node:%s:*", userID),
		fmt.Sprintf("node_connections:%s:*", userID),
		fmt.Sprintf("graph:%s:*", userID),
	}

	for _, pattern := range patterns {
		s.cache.Delete(ctx, pattern)
	}
}