// Package queries contains CQRS query implementations for read operations
package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
)

// GetGraphQuery represents a query to get the graph structure
type GetGraphQuery struct {
	cqrs.BaseQuery
	UserID string `json:"user_id"`
	NodeID string `json:"node_id,omitempty"` // Optional: center node for subgraph
	Depth  int    `json:"depth,omitempty"`   // Optional: depth for subgraph
}

// GetQueryName returns the query name
func (q GetGraphQuery) GetQueryName() string {
	return "GetGraph"
}

// Validate validates the query
func (q GetGraphQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.Depth < 0 {
		return fmt.Errorf("depth cannot be negative")
	}
	if q.Depth > 5 {
		return fmt.Errorf("depth cannot exceed 5")
	}
	return nil
}

// GetCacheKey returns the cache key for this query
func (q GetGraphQuery) GetCacheKey() string {
	if q.NodeID != "" {
		return fmt.Sprintf("graph:user:%s:node:%s:depth:%d", q.UserID, q.NodeID, q.Depth)
	}
	return fmt.Sprintf("graph:user:%s:full", q.UserID)
}

// GetGraphResult is the result of the GetGraph query
type GetGraphResult struct {
	Nodes []ports.NodeView `json:"nodes"`
	Edges []ports.EdgeView `json:"edges"`
	Stats GraphStats       `json:"stats"`
}

// IsEmpty checks if the result is empty
func (r *GetGraphResult) IsEmpty() bool {
	return len(r.Nodes) == 0 && len(r.Edges) == 0
}

// GraphStats contains graph statistics
type GraphStats struct {
	NodeCount       int     `json:"node_count"`
	EdgeCount       int     `json:"edge_count"`
	ConnectedGroups int     `json:"connected_groups"`
	AvgDegree       float64 `json:"avg_degree"`
}

// GetGraphHandler handles get graph queries
type GetGraphHandler struct {
	queryRepo ports.QueryRepository
	cache     ports.Cache
	logger    ports.Logger
}

// NewGetGraphHandler creates a new get graph handler
func NewGetGraphHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
) *GetGraphHandler {
	return &GetGraphHandler{
		queryRepo: queryRepo,
		cache:     cache,
		logger:    logger,
	}
}

// Handle processes the get graph query
func (h *GetGraphHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*GetGraphQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Set defaults
	if q.Depth == 0 && q.NodeID != "" {
		q.Depth = 2 // Default depth for subgraph
	}
	
	// Check cache first
	if h.cache != nil {
		cacheKey := q.GetCacheKey()
		if cacheKey != "" {
			if data, err := h.cache.Get(ctx, cacheKey); err == nil {
				var result GetGraphResult
				if err := json.Unmarshal(data, &result); err == nil {
					h.logger.Debug("Cache hit for graph",
						ports.Field{Key: "user_id", Value: q.UserID},
						ports.Field{Key: "node_id", Value: q.NodeID})
					return &result, nil
				}
			}
		}
	}
	
	var graphView *ports.GraphView
	var err error
	
	if q.NodeID != "" {
		// Get subgraph centered on a specific node
		graphView, err = h.queryRepo.GetNodeGraph(ctx, q.NodeID, q.Depth)
		if err != nil {
			return nil, fmt.Errorf("failed to get node graph: %w", err)
		}
	} else {
		// Get full graph for user
		options := ports.QueryOptions{
			Limit: 1000, // Reasonable limit for full graph
		}
		
		nodeResult, err := h.queryRepo.FindNodesByUser(ctx, q.UserID, options)
		if err != nil {
			return nil, fmt.Errorf("failed to get user nodes: %w", err)
		}
		
		// For full graph, we need to get all edges
		// This is simplified - in production, would need a more efficient approach
		graphView = &ports.GraphView{
			Nodes: nodeResult.Nodes,
			Edges: []ports.EdgeView{}, // Would need to fetch edges separately
		}
	}
	
	// Calculate statistics
	stats := h.calculateStats(graphView)
	
	result := &GetGraphResult{
		Nodes: graphView.Nodes,
		Edges: graphView.Edges,
		Stats: stats,
	}
	
	// Cache the result
	if h.cache != nil && q.GetCacheKey() != "" {
		if data, err := json.Marshal(result); err == nil {
			cacheKey := q.GetCacheKey()
			cacheDuration := 2 * time.Minute
			if q.NodeID == "" {
				// Cache full graph for shorter time
				cacheDuration = 30 * time.Second
			}
			if err := h.cache.Set(ctx, cacheKey, data, cacheDuration); err != nil {
				h.logger.Warn("Failed to cache graph",
					ports.Field{Key: "user_id", Value: q.UserID},
					ports.Field{Key: "error", Value: err.Error()})
			}
		}
	}
	
	h.logger.Debug("Retrieved graph",
		ports.Field{Key: "user_id", Value: q.UserID},
		ports.Field{Key: "node_id", Value: q.NodeID},
		ports.Field{Key: "nodes", Value: len(result.Nodes)},
		ports.Field{Key: "edges", Value: len(result.Edges)})
	
	return result, nil
}

// calculateStats calculates graph statistics
func (h *GetGraphHandler) calculateStats(graph *ports.GraphView) GraphStats {
	stats := GraphStats{
		NodeCount: len(graph.Nodes),
		EdgeCount: len(graph.Edges),
	}
	
	if stats.NodeCount > 0 {
		// Calculate average degree
		degreeSum := 0
		nodeDegrees := make(map[string]int)
		
		for _, edge := range graph.Edges {
			nodeDegrees[edge.SourceID]++
			nodeDegrees[edge.TargetID]++
		}
		
		for _, degree := range nodeDegrees {
			degreeSum += degree
		}
		
		if len(nodeDegrees) > 0 {
			stats.AvgDegree = float64(degreeSum) / float64(len(nodeDegrees))
		}
		
		// Count connected components (simplified - would need proper graph traversal)
		visited := make(map[string]bool)
		components := 0
		
		for _, node := range graph.Nodes {
			if !visited[node.ID] {
				components++
				// Would perform DFS/BFS here to mark all connected nodes
				visited[node.ID] = true
			}
		}
		
		stats.ConnectedGroups = components
	}
	
	return stats
}

// CanHandle checks if this handler can handle the query
func (h *GetGraphHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*GetGraphQuery)
	return ok
}