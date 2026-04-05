package queries

import (
	"context"
	"fmt"

	"backend/application/services"
)

// HybridSearchQuery represents a query for hybrid BM25 + semantic search.
type HybridSearchQuery struct {
	UserID string `json:"user_id"`
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Validate validates the query.
func (q *HybridSearchQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.Query == "" {
		return fmt.Errorf("search query is required")
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	return nil
}

// HybridSearchResult holds ranked search results with scoring metadata.
type HybridSearchResult struct {
	Results []HybridSearchResultItem `json:"results"`
	Total   int                      `json:"total"`
	Query   string                   `json:"query"`
}

// HybridSearchResultItem is a single result entry.
type HybridSearchResultItem struct {
	NodeID        string   `json:"node_id"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	Score         float64  `json:"score"`
	BM25Score     float64  `json:"bm25_score"`
	SemanticScore float64  `json:"semantic_score"`
	Sources       []string `json:"sources"`
	Tags          []string `json:"tags"`
}

// HybridSearchHandler handles hybrid search queries using the domain search service.
type HybridSearchHandler struct {
	searchService *services.HybridSearchService
}

// NewHybridSearchHandler creates a new handler.
func NewHybridSearchHandler(searchService *services.HybridSearchService) *HybridSearchHandler {
	return &HybridSearchHandler{searchService: searchService}
}

// Handle executes the hybrid search query.
func (h *HybridSearchHandler) Handle(ctx context.Context, query interface{}) (interface{}, error) {
	q, ok := query.(*HybridSearchQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	results, err := h.searchService.Search(ctx, q.UserID, q.Query, q.Limit+q.Offset)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply offset
	if q.Offset >= len(results) {
		return &HybridSearchResult{
			Results: []HybridSearchResultItem{},
			Total:   len(results),
			Query:   q.Query,
		}, nil
	}
	results = results[q.Offset:]
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}

	// Map domain results to query result items
	items := make([]HybridSearchResultItem, len(results))
	for i, r := range results {
		content := r.Node.Content()
		items[i] = HybridSearchResultItem{
			NodeID:        r.Node.ID().String(),
			Title:         content.Title(),
			Body:          content.Body(),
			Score:         r.Score,
			BM25Score:     r.BM25Score,
			SemanticScore: r.SemanticScore,
			Sources:       r.Sources,
			Tags:          r.Node.GetTags(),
		}
	}

	return &HybridSearchResult{
		Results: items,
		Total:   len(results) + q.Offset, // Approximate total
		Query:   q.Query,
	}, nil
}
