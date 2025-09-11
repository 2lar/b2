package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	"backend/domain/core/aggregates"
	"go.uber.org/zap"
)

// GetGraphStatsHandler handles the GetGraphStatsQuery
type GetGraphStatsHandler struct {
	cache      ports.Cache
	graphRepo  ports.GraphRepository
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	logger     *zap.Logger
}

// NewGetGraphStatsHandler creates a new handler instance
func NewGetGraphStatsHandler(
	cache ports.Cache,
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *GetGraphStatsHandler {
	return &GetGraphStatsHandler{
		cache:     cache,
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		logger:    logger,
	}
}

// Handle executes the query
func (h *GetGraphStatsHandler) Handle(ctx context.Context, query queries.GetGraphStatsQuery) (*queries.GetGraphStatsResult, error) {
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// Check if user owns the graph
	graphID := aggregates.GraphID(query.GraphID)
	graph, err := h.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph: %w", err)
	}
	if graph == nil {
		return nil, fmt.Errorf("graph not found")
	}
	if graph.UserID() != query.UserID {
		return nil, fmt.Errorf("unauthorized access to graph")
	}

	// Try to get stats from cache first
	cacheKey := fmt.Sprintf("graph:stats:%s", query.GraphID)
	
	cachedValue, found := h.cache.Get(ctx, cacheKey)
	if found {
		if stats, ok := cachedValue.(*queries.GetGraphStatsResult); ok {
			h.logger.Debug("Graph stats retrieved from cache",
				zap.String("graphID", query.GraphID))
			return stats, nil
		}
	}

	// If not in cache, calculate stats from repository
	// This is a fallback - normally the GraphStatsProjection should keep cache updated
	nodes, err := h.nodeRepo.GetByGraphID(ctx, query.GraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	edges, err := h.edgeRepo.GetByGraphID(ctx, query.GraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}

	nodeCount := len(nodes)
	edgeCount := len(edges)
	averageConnections := 0.0
	if nodeCount > 0 {
		averageConnections = float64(edgeCount*2) / float64(nodeCount)
	}

	result := &queries.GetGraphStatsResult{
		GraphID:            query.GraphID,
		NodeCount:          nodeCount,
		EdgeCount:          edgeCount,
		AverageConnections: averageConnections,
		LastUpdated:        time.Now(),
	}

	// Cache the calculated stats
	if err := h.cache.Set(ctx, cacheKey, result, 3600); err != nil {
		h.logger.Warn("Failed to cache graph stats",
			zap.String("graphID", query.GraphID),
			zap.Error(err))
	}

	return result, nil
}