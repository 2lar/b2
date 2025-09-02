// Package queries contains CQRS query implementations for read operations
package queries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
)

// GetNodeByIDQuery represents a query to get a node by ID
type GetNodeByIDQuery struct {
	cqrs.BaseQuery
	NodeID string `json:"node_id"`
	UserID string `json:"user_id"`
}

// GetQueryName returns the query name
func (q GetNodeByIDQuery) GetQueryName() string {
	return "GetNodeByID"
}

// Validate validates the query
func (q GetNodeByIDQuery) Validate() error {
	if q.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	return nil
}

// GetCacheKey returns the cache key for this query
func (q GetNodeByIDQuery) GetCacheKey() string {
	return fmt.Sprintf("node:%s:%s", q.UserID, q.NodeID)
}

// GetNodeByIDResult is the result of the GetNodeByID query
type GetNodeByIDResult struct {
	Node *node.Aggregate `json:"node"`
}

// IsEmpty checks if the result is empty
func (r *GetNodeByIDResult) IsEmpty() bool {
	return r.Node == nil
}

// GetNodeByIDHandler handles get node by ID queries
type GetNodeByIDHandler struct {
	nodeRepo ports.NodeRepository
	cache    ports.Cache
	logger   ports.Logger
}

// NewGetNodeByIDHandler creates a new get node by ID handler
func NewGetNodeByIDHandler(
	nodeRepo ports.NodeRepository,
	cache ports.Cache,
	logger ports.Logger,
) *GetNodeByIDHandler {
	return &GetNodeByIDHandler{
		nodeRepo: nodeRepo,
		cache:    cache,
		logger:   logger,
	}
}

// Handle processes the get node by ID query
func (h *GetNodeByIDHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*GetNodeByIDQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Check cache first
	if h.cache != nil {
		cacheKey := q.GetCacheKey()
		if data, err := h.cache.Get(ctx, cacheKey); err == nil {
			var result GetNodeByIDResult
			if err := json.Unmarshal(data, &result); err == nil {
				h.logger.Debug("Cache hit for node",
					ports.Field{Key: "node_id", Value: q.NodeID})
				return &result, nil
			}
		}
	}
	
	// Get node from repository
	nodeAgg, err := h.nodeRepo.FindByID(ctx, q.NodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}
	
	// Verify the node belongs to the user
	if nodeAgg.GetUserID() != q.UserID {
		return nil, fmt.Errorf("unauthorized: node does not belong to user")
	}
	
	result := &GetNodeByIDResult{
		Node: nodeAgg,
	}
	
	// Cache the result
	if h.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			cacheKey := q.GetCacheKey()
			if err := h.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
				h.logger.Warn("Failed to cache node",
					ports.Field{Key: "node_id", Value: q.NodeID},
					ports.Field{Key: "error", Value: err.Error()})
			}
		}
	}
	
	return result, nil
}

// CanHandle checks if this handler can handle the query
func (h *GetNodeByIDHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*GetNodeByIDQuery)
	return ok
}