package projections

import (
	"context"
	"sync"
	"time"

	appevents "backend/application/events"
	"backend/application/ports"
	"backend/domain/events"
	"go.uber.org/zap"
)

// GraphStatistics holds cached statistics for a graph
type GraphStatistics struct {
	GraphID            string    `json:"graph_id"`
	NodeCount          int       `json:"node_count"`
	EdgeCount          int       `json:"edge_count"`
	AverageConnections float64   `json:"average_connections"`
	LastUpdated        time.Time `json:"last_updated"`
}

// GraphStatsProjection maintains cached graph statistics
// This projection listens to node and edge events to maintain up-to-date statistics
// without expensive queries
type GraphStatsProjection struct {
	appevents.BaseEventHandler
	cache  ports.Cache
	logger *zap.Logger
	mu     sync.RWMutex
	stats  map[string]*GraphStatistics // graphID -> stats
}

// NewGraphStatsProjection creates a new graph statistics projection
func NewGraphStatsProjection(cache ports.Cache, logger *zap.Logger) *GraphStatsProjection {
	return &GraphStatsProjection{
		BaseEventHandler: appevents.NewBaseEventHandler(
			"GraphStatsProjection",
			5, // high priority
			[]string{
				"node.created.with.pending.edges",
				"NodeDeleted", 
				"BulkNodesDeleted",
			},
		),
		cache:  cache,
		logger: logger,
		stats:  make(map[string]*GraphStatistics),
	}
}

// Handle processes domain events and updates statistics
func (p *GraphStatsProjection) Handle(ctx context.Context, event events.DomainEvent) error {
	switch e := event.(type) {
	case *events.NodeCreatedWithPendingEdges:
		return p.handleNodeCreated(ctx, e)
	case *events.NodeDeletedEvent:
		return p.handleNodeDeleted(ctx, e)
	case *events.BulkNodesDeletedEvent:
		return p.handleBulkNodesDeleted(ctx, e)
	default:
		// Ignore unknown events
		return nil
	}
}

// handleNodeCreated updates stats when a node is created
func (p *GraphStatsProjection) handleNodeCreated(ctx context.Context, event *events.NodeCreatedWithPendingEdges) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	graphID := event.GraphID
	stats := p.getOrCreateStats(graphID)
	stats.NodeCount++
	stats.LastUpdated = time.Now()
	
	// Recalculate average connections
	if stats.NodeCount > 0 {
		stats.AverageConnections = float64(stats.EdgeCount*2) / float64(stats.NodeCount)
	}

	// Update cache
	cacheKey := p.getCacheKey(graphID)
	if err := p.cache.Set(ctx, cacheKey, stats, 3600); err != nil {
		p.logger.Warn("Failed to update cache for graph stats",
			zap.String("graphID", graphID),
			zap.Error(err))
	}

	p.logger.Debug("Updated graph stats after node creation",
		zap.String("graphID", graphID),
		zap.Int("nodeCount", stats.NodeCount))

	return nil
}

// handleNodeDeleted updates stats when a node is deleted
func (p *GraphStatsProjection) handleNodeDeleted(ctx context.Context, event *events.NodeDeletedEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	graphID := event.GraphID
	stats := p.getOrCreateStats(graphID)
	if stats.NodeCount > 0 {
		stats.NodeCount--
	}
	
	// Also decrease edge count by the number of edges the node had
	// Note: The NodeDeletedEvent doesn't contain edge count, so we can't update it accurately here
	// In a production system, we'd need to track this separately or include it in the event
	
	stats.LastUpdated = time.Now()
	
	// Recalculate average connections
	if stats.NodeCount > 0 {
		stats.AverageConnections = float64(stats.EdgeCount*2) / float64(stats.NodeCount)
	} else {
		stats.AverageConnections = 0
	}

	// Update cache
	cacheKey := p.getCacheKey(graphID)
	if err := p.cache.Set(ctx, cacheKey, stats, 3600); err != nil {
		p.logger.Warn("Failed to update cache for graph stats",
			zap.String("graphID", graphID),
			zap.Error(err))
	}

	p.logger.Debug("Updated graph stats after node deletion",
		zap.String("graphID", graphID),
		zap.Int("nodeCount", stats.NodeCount))

	return nil
}


// handleBulkNodesDeleted updates stats when multiple nodes are deleted
func (p *GraphStatsProjection) handleBulkNodesDeleted(ctx context.Context, event *events.BulkNodesDeletedEvent) error {
	// Note: BulkNodesDeletedEvent doesn't contain GraphID
	// In a production system, we'd need to track which graph(s) were affected
	// For now, we'll just log the event
	
	p.logger.Info("Bulk nodes deleted",
		zap.String("operationID", event.OperationID),
		zap.Int("deletedCount", event.DeletedCount),
		zap.String("userID", event.UserID))

	// In a real implementation, we would:
	// 1. Query which graphs were affected
	// 2. Update stats for each affected graph
	// 3. Update the cache
	
	return nil
}

// GetStats retrieves cached statistics for a graph
func (p *GraphStatsProjection) GetStats(ctx context.Context, graphID string) (*GraphStatistics, error) {
	// Try cache first
	cacheKey := p.getCacheKey(graphID)
	if cached, found := p.cache.Get(ctx, cacheKey); found {
		if stats, ok := cached.(*GraphStatistics); ok {
			return stats, nil
		}
	}

	// Check in-memory stats
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if stats, exists := p.stats[graphID]; exists {
		// Update cache
		p.cache.Set(ctx, cacheKey, stats, 3600)
		return stats, nil
	}

	// Return empty stats if not found
	return &GraphStatistics{
		GraphID:     graphID,
		LastUpdated: time.Now(),
	}, nil
}

// Reset clears all cached statistics
func (p *GraphStatsProjection) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats = make(map[string]*GraphStatistics)
	
	// Clear cache
	if err := p.cache.Clear(ctx); err != nil {
		p.logger.Warn("Failed to clear cache", zap.Error(err))
	}

	p.logger.Info("Graph statistics projection reset")
	return nil
}

// getOrCreateStats gets or creates statistics for a graph
func (p *GraphStatsProjection) getOrCreateStats(graphID string) *GraphStatistics {
	if stats, exists := p.stats[graphID]; exists {
		return stats
	}

	stats := &GraphStatistics{
		GraphID:     graphID,
		LastUpdated: time.Now(),
	}
	p.stats[graphID] = stats
	return stats
}

// getCacheKey generates a cache key for graph statistics
func (p *GraphStatsProjection) getCacheKey(graphID string) string {
	return "graph_stats:" + graphID
}