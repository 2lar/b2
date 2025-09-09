package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	"go.uber.org/zap"
)

// ListNodesHandler handles list nodes queries
type ListNodesHandler struct {
	nodeRepo ports.NodeRepository
	logger   *zap.Logger
}

// NewListNodesHandler creates a new list nodes handler
func NewListNodesHandler(nodeRepo ports.NodeRepository, logger *zap.Logger) *ListNodesHandler {
	return &ListNodesHandler{
		nodeRepo: nodeRepo,
		logger:   logger,
	}
}

// Handle executes the list nodes query
func (h *ListNodesHandler) Handle(ctx context.Context, query queries.ListNodesQuery) (*queries.ListNodesResult, error) {
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

	// Get nodes from repository
	nodes, err := h.nodeRepo.GetByUserID(ctx, query.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Apply sorting
	// TODO: Implement proper sorting in repository layer

	// Apply pagination
	totalCount := len(nodes)
	start := query.Offset
	if start > totalCount {
		start = totalCount
	}
	end := start + query.Limit
	if end > totalCount {
		end = totalCount
	}

	// Convert to node summaries
	summaries := make([]queries.NodeSummary, 0, end-start)
	for i := start; i < end; i++ {
		node := nodes[i]
		content := node.Content()
		summaries = append(summaries, queries.NodeSummary{
			ID:        node.ID().String(),
			Title:     content.Title(),
			Format:    string(content.Format()),
			Tags:      node.GetTags(),
			CreatedAt: node.CreatedAt().Format(time.RFC3339),
			UpdatedAt: node.UpdatedAt().Format(time.RFC3339),
		})
	}

	result := &queries.ListNodesResult{
		Nodes:      summaries,
		TotalCount: totalCount,
		Limit:      query.Limit,
		Offset:     query.Offset,
	}

	h.logger.Debug("Nodes listed",
		zap.String("userID", query.UserID),
		zap.Int("count", len(summaries)),
		zap.Int("total", totalCount),
	)

	return result, nil
}
