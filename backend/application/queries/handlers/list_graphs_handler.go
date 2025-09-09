package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	"go.uber.org/zap"
)

// ListGraphsHandler handles list graphs queries
type ListGraphsHandler struct {
	graphRepo ports.GraphRepository
	logger    *zap.Logger
}

// NewListGraphsHandler creates a new list graphs handler
func NewListGraphsHandler(graphRepo ports.GraphRepository, logger *zap.Logger) *ListGraphsHandler {
	return &ListGraphsHandler{
		graphRepo: graphRepo,
		logger:    logger,
	}
}

// Handle executes the list graphs query
func (h *ListGraphsHandler) Handle(ctx context.Context, query queries.ListGraphsQuery) (*queries.ListGraphsResult, error) {
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Set defaults
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	if query.SortBy == "" {
		query.SortBy = "updated"
	}
	if query.Order == "" {
		query.Order = "desc"
	}

	// Get graphs from repository
	graphs, err := h.graphRepo.GetByUserID(ctx, query.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to list graphs: %w", err)
	}

	// Apply pagination
	totalCount := len(graphs)
	start := query.Offset
	if start > totalCount {
		start = totalCount
	}
	end := start + query.Limit
	if end > totalCount {
		end = totalCount
	}

	// Convert to summaries
	summaries := make([]queries.GraphSummary, 0, end-start)
	for i := start; i < end; i++ {
		graph := graphs[i]

		// Get node count safely
		nodes, err := graph.Nodes()
		if err != nil {
			// For large graphs, use approximate count or pagination
			h.logger.Warn("Graph too large for Nodes() method",
				zap.String("graphID", graph.ID().String()),
				zap.Error(err))
			summaries = append(summaries, queries.GraphSummary{
				ID:          graph.ID().String(),
				Name:        graph.Name(),
				Description: graph.Description(),
				NodeCount:   -1, // Indicate large graph
				EdgeCount:   len(graph.Edges()),
				IsDefault:   graph.IsDefault(),
				CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
				UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
			})
			continue
		}

		summaries = append(summaries, queries.GraphSummary{
			ID:          graph.ID().String(),
			Name:        graph.Name(),
			Description: graph.Description(),
			NodeCount:   len(nodes),
			EdgeCount:   len(graph.Edges()),
			IsDefault:   graph.IsDefault(),
			CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
			UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
		})
	}

	result := &queries.ListGraphsResult{
		Graphs:     summaries,
		TotalCount: totalCount,
		Limit:      query.Limit,
		Offset:     query.Offset,
	}

	h.logger.Debug("Graphs listed",
		zap.String("userID", query.UserID),
		zap.Int("count", len(summaries)),
		zap.Int("total", totalCount),
	)

	return result, nil
}
