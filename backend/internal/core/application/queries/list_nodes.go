// Package queries contains the ListNodesQuery implementation
package queries

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
)

// ListNodesQuery represents a query to list nodes with filtering and pagination
type ListNodesQuery struct {
	cqrs.BaseQuery
	CategoryID      string    `json:"category_id,omitempty"`
	Tags            []string  `json:"tags,omitempty"`
	IncludeArchived bool      `json:"include_archived"`
	FromDate        time.Time `json:"from_date,omitempty"`
	ToDate          time.Time `json:"to_date,omitempty"`
}

// GetQueryName returns the query name
func (q ListNodesQuery) GetQueryName() string {
	return "ListNodes"
}

// Validate validates the query
func (q ListNodesQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if err := q.Pagination.Validate(); err != nil {
		return fmt.Errorf("invalid pagination: %w", err)
	}
	if !q.FromDate.IsZero() && !q.ToDate.IsZero() && q.FromDate.After(q.ToDate) {
		return fmt.Errorf("from_date must be before to_date")
	}
	return nil
}

// NodeListResult represents a paginated list of nodes
type NodeListResult struct {
	cqrs.PagedResult
	Nodes []NodeSummary `json:"nodes"`
}

// NodeSummary represents a summary view of a node
type NodeSummary struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	ContentPreview  string    `json:"content_preview"`
	Tags            []string  `json:"tags"`
	CategoryName    string    `json:"category_name,omitempty"`
	ConnectionCount int       `json:"connection_count"`
	IsArchived      bool      `json:"is_archived"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// IsEmpty checks if the result is empty
func (r NodeListResult) IsEmpty() bool {
	return len(r.Nodes) == 0
}

// ListNodesHandler handles the ListNodesQuery
type ListNodesHandler struct {
	queryRepo ports.QueryRepository
	cache     ports.Cache
	logger    ports.Logger
	metrics   ports.Metrics
}

// NewListNodesHandler creates a new ListNodesHandler
func NewListNodesHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
	metrics ports.Metrics,
) *ListNodesHandler {
	return &ListNodesHandler{
		queryRepo: queryRepo,
		cache:     cache,
		logger:    logger,
		metrics:   metrics,
	}
}

// Handle processes the ListNodesQuery
func (h *ListNodesHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*ListNodesQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Build query options
	options := h.buildQueryOptions(q)
	
	// Check cache for this specific query
	cacheKey := h.buildCacheKey(q)
	if cached, err := h.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		h.metrics.IncrementCounter("query.list_nodes.cache_hit")
		return cached, nil
	}
	
	// Query from read model
	result, err := h.queryRepo.FindNodesByUser(ctx, q.UserID, options)
	if err != nil {
		h.metrics.IncrementCounter("query.list_nodes.failed")
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	
	// Convert to result
	nodeListResult := &NodeListResult{
		PagedResult: cqrs.PagedResult{
			TotalCount: result.TotalCount,
			Offset:     q.Pagination.Offset,
			Limit:      q.Pagination.Limit,
			HasMore:    result.HasMore,
		},
		Nodes: h.mapToSummaries(result.Nodes),
	}
	
	// Cache the result if not empty
	if !nodeListResult.IsEmpty() {
		if err := h.cacheResult(ctx, cacheKey, nodeListResult); err != nil {
			h.logger.Warn("Failed to cache list result",
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	h.metrics.IncrementCounter("query.list_nodes.success",
		ports.Tag{Key: "count", Value: fmt.Sprintf("%d", len(nodeListResult.Nodes))})
	
	return nodeListResult, nil
}

// CanHandle checks if this handler can handle the query
func (h *ListNodesHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*ListNodesQuery)
	return ok
}

// buildQueryOptions builds query options from the query
func (h *ListNodesHandler) buildQueryOptions(q *ListNodesQuery) ports.QueryOptions {
	filters := make(map[string]interface{})
	
	if q.CategoryID != "" {
		filters["category_id"] = q.CategoryID
	}
	
	if len(q.Tags) > 0 {
		filters["tags"] = q.Tags
	}
	
	if !q.IncludeArchived {
		filters["archived"] = false
	}
	
	if !q.FromDate.IsZero() {
		filters["created_after"] = q.FromDate.Unix()
	}
	
	if !q.ToDate.IsZero() {
		filters["created_before"] = q.ToDate.Unix()
	}
	
	sortBy := q.Pagination.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	
	order := q.Pagination.Order
	if order == "" {
		order = "desc"
	}
	
	return ports.QueryOptions{
		Offset:  q.Pagination.Offset,
		Limit:   q.Pagination.Limit,
		SortBy:  sortBy,
		Order:   order,
		Filters: filters,
	}
}

// buildCacheKey builds a cache key for the query
func (h *ListNodesHandler) buildCacheKey(q *ListNodesQuery) string {
	// Create a unique key based on query parameters
	return fmt.Sprintf("list:%s:%v:%v:%d:%d:%s:%s",
		q.UserID,
		q.CategoryID,
		q.Tags,
		q.Pagination.Offset,
		q.Pagination.Limit,
		q.Pagination.SortBy,
		q.Pagination.Order,
	)
}

// mapToSummaries converts node views to summaries
func (h *ListNodesHandler) mapToSummaries(nodes []ports.NodeView) []NodeSummary {
	summaries := make([]NodeSummary, len(nodes))
	for i, node := range nodes {
		summaries[i] = NodeSummary{
			ID:              node.ID,
			Title:           node.Title,
			ContentPreview:  h.createPreview(node.Content, 200),
			Tags:            node.Tags,
			CategoryName:    h.getCategoryName(node.Categories),
			ConnectionCount: node.ConnectionCount,
			IsArchived:      false, // Would be set based on node state
			CreatedAt:       time.Unix(node.CreatedAt, 0),
			UpdatedAt:       time.Unix(node.UpdatedAt, 0),
		}
	}
	return summaries
}

// createPreview creates a content preview
func (h *ListNodesHandler) createPreview(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength-3] + "..."
}

// getCategoryName gets the primary category name
func (h *ListNodesHandler) getCategoryName(categories []string) string {
	if len(categories) > 0 {
		// In a real implementation, this would look up the category name
		return "Category " + categories[0]
	}
	return ""
}

// getFromCache retrieves a cached result
func (h *ListNodesHandler) getFromCache(ctx context.Context, key string) (*NodeListResult, error) {
	if h.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}
	
	data, err := h.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	// Deserialize the cached data
	// Implementation would handle proper deserialization
	_ = data
	return nil, fmt.Errorf("not implemented")
}

// cacheResult caches a query result
func (h *ListNodesHandler) cacheResult(ctx context.Context, key string, result *NodeListResult) error {
	if h.cache == nil || result == nil {
		return nil
	}
	
	// Serialize the result
	// Implementation would handle proper serialization
	data := []byte{}
	
	// Cache with shorter TTL for list queries
	return h.cache.Set(ctx, key, data, 2*time.Minute)
}