// Package queries contains CQRS query implementations for read operations
package queries

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
)

// SearchNodesQuery represents a query to search nodes
type SearchNodesQuery struct {
	cqrs.BaseQuery
	Query   string                 `json:"query"`
	UserID  string                 `json:"user_id"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
	Filters map[string]interface{} `json:"filters"`
}

// GetQueryName returns the query name
func (q SearchNodesQuery) GetQueryName() string {
	return "SearchNodes"
}

// Validate validates the query
func (q SearchNodesQuery) Validate() error {
	if q.Query == "" {
		return fmt.Errorf("search query is required")
	}
	if len(q.Query) < 2 {
		return fmt.Errorf("search query must be at least 2 characters")
	}
	if len(q.Query) > 500 {
		return fmt.Errorf("search query is too long")
	}
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if q.Limit > 100 {
		return fmt.Errorf("limit cannot exceed 100")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}
	return nil
}

// GetCacheKey returns the cache key for this query
func (q SearchNodesQuery) GetCacheKey() string {
	// Search results are not cached due to their dynamic nature
	return ""
}

// SearchNodesResult is the result of the SearchNodes query
type SearchNodesResult struct {
	Hits       []ports.SearchHit `json:"hits"`
	TotalCount int              `json:"total_count"`
	Duration   int64            `json:"duration_ms"`
	Facets     map[string][]ports.FacetValue `json:"facets,omitempty"`
}

// IsEmpty checks if the result is empty
func (r *SearchNodesResult) IsEmpty() bool {
	return len(r.Hits) == 0
}

// SearchNodesHandler handles search nodes queries
type SearchNodesHandler struct {
	searchService ports.SearchService
	logger        ports.Logger
}

// NewSearchNodesHandler creates a new search nodes handler
func NewSearchNodesHandler(
	searchService ports.SearchService,
	logger ports.Logger,
) *SearchNodesHandler {
	return &SearchNodesHandler{
		searchService: searchService,
		logger:        logger,
	}
}

// Handle processes the search nodes query
func (h *SearchNodesHandler) Handle(ctx context.Context, query cqrs.Query) (cqrs.QueryResult, error) {
	q, ok := query.(*SearchNodesQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}
	
	// Set defaults
	if q.Limit == 0 {
		q.Limit = 20
	}
	
	// Build search query
	searchQuery := ports.SearchQuery{
		Query:   q.Query,
		UserID:  q.UserID,
		Limit:   q.Limit,
		Offset:  q.Offset,
		Filters: q.Filters,
	}
	
	// Perform search
	searchResult, err := h.searchService.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	
	result := &SearchNodesResult{
		Hits:       searchResult.Hits,
		TotalCount: searchResult.TotalCount,
		Duration:   searchResult.Duration.Milliseconds(),
		Facets:     searchResult.Facets,
	}
	
	h.logger.Debug("Search completed",
		ports.Field{Key: "query", Value: q.Query},
		ports.Field{Key: "user_id", Value: q.UserID},
		ports.Field{Key: "hits", Value: len(result.Hits)},
		ports.Field{Key: "duration_ms", Value: result.Duration})
	
	return result, nil
}

// CanHandle checks if this handler can handle the query
func (h *SearchNodesHandler) CanHandle(query cqrs.Query) bool {
	_, ok := query.(*SearchNodesQuery)
	return ok
}