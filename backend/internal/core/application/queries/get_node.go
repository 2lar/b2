// Package queries contains CQRS query implementations for read operations
package queries

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
)

// GetNodeQuery represents a query to retrieve a single node
type GetNodeQuery struct {
	cqrs.BaseQuery
	NodeID          string `json:"node_id"`
	IncludeArchived bool   `json:"include_archived"`
}

// GetQueryName returns the query name
func (q GetNodeQuery) GetQueryName() string {
	return "GetNode"
}

// Validate validates the query
func (q GetNodeQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	return nil
}

// NodeResult represents the result of a node query
type NodeResult struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	Content         string                 `json:"content"`
	Title           string                 `json:"title"`
	Keywords        []string               `json:"keywords"`
	Tags            []string               `json:"tags"`
	Categories      []CategoryInfo         `json:"categories"`
	ConnectionCount int                    `json:"connection_count"`
	IsArchived      bool                   `json:"is_archived"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	Version         int64                  `json:"version"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// CategoryInfo contains category information
type CategoryInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// IsEmpty checks if the result is empty
func (r NodeResult) IsEmpty() bool {
	return r.ID == ""
}

// GetNodeHandler handles the GetNodeQuery
type GetNodeHandler struct {
	queryRepo ports.QueryRepository
	cache     ports.Cache
	logger    ports.Logger
	metrics   ports.Metrics
}

// NewGetNodeHandler creates a new GetNodeHandler
func NewGetNodeHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
	metrics ports.Metrics,
) *GetNodeHandler {
	return &GetNodeHandler{
		queryRepo: queryRepo,
		cache:     cache,
		logger:    logger,
		metrics:   metrics,
	}
}

// Handle processes the GetNodeQuery
func (h *GetNodeHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*GetNodeQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("node:%s:%s", q.UserID, q.NodeID)
	if cached, err := h.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		h.metrics.IncrementCounter("query.node.cache_hit")
		return cached, nil
	}
	
	// Query from read model
	options := ports.QueryOptions{
		Filters: map[string]interface{}{
			"node_id":          q.NodeID,
			"user_id":          q.UserID,
			"include_archived": q.IncludeArchived,
		},
		Limit: 1,
	}
	
	result, err := h.queryRepo.FindNodesByUser(ctx, q.UserID, options)
	if err != nil {
		h.metrics.IncrementCounter("query.node.failed")
		return nil, fmt.Errorf("failed to query node: %w", err)
	}
	
	if len(result.Nodes) == 0 {
		h.metrics.IncrementCounter("query.node.not_found")
		return nil, fmt.Errorf("node not found")
	}
	
	// Convert to result
	nodeView := result.Nodes[0]
	nodeResult := &NodeResult{
		ID:              nodeView.ID,
		UserID:          nodeView.UserID,
		Content:         nodeView.Content,
		Title:           nodeView.Title,
		Tags:            nodeView.Tags,
		Categories:      h.mapCategories(nodeView.Categories),
		ConnectionCount: nodeView.ConnectionCount,
		CreatedAt:       time.Unix(nodeView.CreatedAt, 0),
		UpdatedAt:       time.Unix(nodeView.UpdatedAt, 0),
		Metadata:        make(map[string]interface{}),
	}
	
	// Cache the result
	if err := h.cacheResult(ctx, cacheKey, nodeResult); err != nil {
		h.logger.Warn("Failed to cache node result",
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	h.metrics.IncrementCounter("query.node.success")
	
	return nodeResult, nil
}

// CanHandle checks if this handler can handle the query
func (h *GetNodeHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*GetNodeQuery)
	return ok
}

// getFromCache retrieves a cached result
func (h *GetNodeHandler) getFromCache(ctx context.Context, key string) (*NodeResult, error) {
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
func (h *GetNodeHandler) cacheResult(ctx context.Context, key string, result *NodeResult) error {
	if h.cache == nil || result == nil {
		return nil
	}
	
	// Serialize the result
	// Implementation would handle proper serialization
	data := []byte{}
	
	// Cache with TTL
	return h.cache.Set(ctx, key, data, 5*time.Minute)
}

// mapCategories maps category IDs to CategoryInfo
func (h *GetNodeHandler) mapCategories(categoryIDs []string) []CategoryInfo {
	// In a real implementation, this would look up category names
	categories := make([]CategoryInfo, len(categoryIDs))
	for i, id := range categoryIDs {
		categories[i] = CategoryInfo{
			ID:   id,
			Name: "Category " + id, // Placeholder
		}
	}
	return categories
}