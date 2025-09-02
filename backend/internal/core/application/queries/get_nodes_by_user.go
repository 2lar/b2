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

// GetNodesByUserQuery represents a query to get all nodes for a user
type GetNodesByUserQuery struct {
	cqrs.BaseQuery
	UserID  string `json:"user_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	SortBy  string `json:"sort_by"`
	Order   string `json:"order"`
	Filters map[string]interface{} `json:"filters"`
}

// GetQueryName returns the query name
func (q GetNodesByUserQuery) GetQueryName() string {
	return "GetNodesByUser"
}

// Validate validates the query
func (q GetNodesByUserQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if q.Limit > 1000 {
		return fmt.Errorf("limit cannot exceed 1000")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	if q.Order != "" && q.Order != "asc" && q.Order != "desc" {
		return fmt.Errorf("order must be 'asc' or 'desc'")
	}
	return nil
}

// GetCacheKey returns the cache key for this query
func (q GetNodesByUserQuery) GetCacheKey() string {
	return fmt.Sprintf("nodes:user:%s:limit:%d:offset:%d:sort:%s:%s", 
		q.UserID, q.Limit, q.Offset, q.SortBy, q.Order)
}

// GetNodesByUserResult is the result of the GetNodesByUser query
type GetNodesByUserResult struct {
	Nodes      []ports.NodeView `json:"nodes"`
	TotalCount int64           `json:"total_count"`
	HasMore    bool            `json:"has_more"`
}

// IsEmpty checks if the result is empty
func (r *GetNodesByUserResult) IsEmpty() bool {
	return len(r.Nodes) == 0
}

// GetNodesByUserHandler handles get nodes by user queries
type GetNodesByUserHandler struct {
	queryRepo ports.QueryRepository
	cache     ports.Cache
	logger    ports.Logger
}

// NewGetNodesByUserHandler creates a new get nodes by user handler
func NewGetNodesByUserHandler(
	queryRepo ports.QueryRepository,
	cache ports.Cache,
	logger ports.Logger,
) *GetNodesByUserHandler {
	return &GetNodesByUserHandler{
		queryRepo: queryRepo,
		cache:     cache,
		logger:    logger,
	}
}

// Handle processes the get nodes by user query
func (h *GetNodesByUserHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*GetNodesByUserQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Set defaults
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Order == "" {
		q.Order = "desc"
	}
	if q.SortBy == "" {
		q.SortBy = "created_at"
	}
	
	// Check cache first
	if h.cache != nil {
		cacheKey := q.GetCacheKey()
		if data, err := h.cache.Get(ctx, cacheKey); err == nil {
			var result GetNodesByUserResult
			if err := json.Unmarshal(data, &result); err == nil {
				h.logger.Debug("Cache hit for user nodes",
					ports.Field{Key: "user_id", Value: q.UserID})
				return &result, nil
			}
		}
	}
	
	// Build query options
	options := ports.QueryOptions{
		Limit:   q.Limit,
		Offset:  q.Offset,
		SortBy:  q.SortBy,
		Order:   q.Order,
		Filters: q.Filters,
	}
	
	// Get nodes from repository
	queryResult, err := h.queryRepo.FindNodesByUser(ctx, q.UserID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	
	result := &GetNodesByUserResult{
		Nodes:      queryResult.Nodes,
		TotalCount: queryResult.TotalCount,
		HasMore:    queryResult.HasMore,
	}
	
	// Cache the result for a shorter time since lists change frequently
	if h.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			cacheKey := q.GetCacheKey()
			if err := h.cache.Set(ctx, cacheKey, data, 1*time.Minute); err != nil {
				h.logger.Warn("Failed to cache user nodes",
					ports.Field{Key: "user_id", Value: q.UserID},
					ports.Field{Key: "error", Value: err.Error()})
			}
		}
	}
	
	h.logger.Debug("Retrieved nodes for user",
		ports.Field{Key: "user_id", Value: q.UserID},
		ports.Field{Key: "count", Value: len(result.Nodes)},
		ports.Field{Key: "total", Value: result.TotalCount})
	
	return result, nil
}

// CanHandle checks if this handler can handle the query
func (h *GetNodesByUserHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*GetNodesByUserQuery)
	return ok
}