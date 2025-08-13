// Package repository - Strongly-typed Query Objects and Result Types
//
// This file demonstrates advanced query patterns using strongly-typed objects
// that provide type safety, validation, and rich functionality for complex database queries.
//
// Educational Goals:
//   - Show how to create type-safe query builders
//   - Demonstrate query object patterns with validation
//   - Illustrate result set handling with metadata
//   - Provide composable query construction
//   - Enable query optimization and analysis
package repository

import (
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain"
)

// Query represents the base interface for all query types
type Query interface {
	Validate() error
	GetUserID() domain.UserID
	GetQueryType() string
}

// ResultSet represents the base interface for all query results
type ResultSet interface {
	GetTotalCount() int
	GetReturnedCount() int
	GetExecutionTime() time.Duration
	HasMore() bool
}

// ==== NODE QUERIES ====

// NodeQuery represents a comprehensive query for nodes with all possible parameters
type NodeQuery struct {
	// Required fields
	userID domain.UserID

	// Filtering criteria
	nodeIDs       []domain.NodeID
	contentFilter *ContentFilter
	keywordFilter *KeywordFilter
	tagFilter     *TagFilter
	dateFilter    *DateFilter
	
	// Search parameters
	searchQuery *SearchQuery
	similarity  *SimilarityQuery
	
	// Pagination and ordering
	pagination *PaginationOptions
	ordering   *OrderingOptions
	
	// Query options
	includeArchived bool
	includeMetadata bool
	preloadEdges    bool
	
	// Query metadata
	queryID   string
	createdAt time.Time
}

// ContentFilter defines filtering by node content
type ContentFilter struct {
	Contains      string   `json:"contains,omitempty"`
	ExactMatch    string   `json:"exact_match,omitempty"`
	StartsWith    string   `json:"starts_with,omitempty"`
	EndsWith      string   `json:"ends_with,omitempty"`
	Regex         string   `json:"regex,omitempty"`
	MinLength     int      `json:"min_length,omitempty"`
	MaxLength     int      `json:"max_length,omitempty"`
	ExcludeWords  []string `json:"exclude_words,omitempty"`
	CaseSensitive bool     `json:"case_sensitive,omitempty"`
}

// KeywordFilter defines filtering by keywords
type KeywordFilter struct {
	IncludeAny    []string `json:"include_any,omitempty"`    // OR operation
	IncludeAll    []string `json:"include_all,omitempty"`    // AND operation
	ExcludeAny    []string `json:"exclude_any,omitempty"`    // NOT IN operation
	MinKeywords   int      `json:"min_keywords,omitempty"`
	MaxKeywords   int      `json:"max_keywords,omitempty"`
	MinOverlap    float64  `json:"min_overlap,omitempty"`    // Minimum overlap percentage
}

// TagFilter defines filtering by tags
type TagFilter struct {
	IncludeAny []string `json:"include_any,omitempty"`
	IncludeAll []string `json:"include_all,omitempty"`
	ExcludeAny []string `json:"exclude_any,omitempty"`
	MinTags    int      `json:"min_tags,omitempty"`
	MaxTags    int      `json:"max_tags,omitempty"`
}

// DateFilter defines filtering by dates
type DateFilter struct {
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	UpdatedAfter  *time.Time `json:"updated_after,omitempty"`
	UpdatedBefore *time.Time `json:"updated_before,omitempty"`
	
	// Relative date filters
	CreatedInLast time.Duration `json:"created_in_last,omitempty"`
	UpdatedInLast time.Duration `json:"updated_in_last,omitempty"`
}

// SearchQuery defines full-text search parameters
type SearchQuery struct {
	Query      string    `json:"query"`
	Fields     []string  `json:"fields,omitempty"`     // Fields to search in
	Operator   string    `json:"operator,omitempty"`   // "AND", "OR"
	Boost      []FieldBoost `json:"boost,omitempty"`   // Field-specific boosting
	Fuzziness  int       `json:"fuzziness,omitempty"`  // Edit distance for fuzzy matching
	Synonyms   bool      `json:"synonyms,omitempty"`   // Enable synonym expansion
}

// FieldBoost defines search boost for specific fields
type FieldBoost struct {
	Field string  `json:"field"`
	Boost float64 `json:"boost"`
}

// SimilarityQuery defines similarity-based search
type SimilarityQuery struct {
	ReferenceNodeID domain.NodeID `json:"reference_node_id"`
	MinSimilarity   float64       `json:"min_similarity"`
	Algorithm       string        `json:"algorithm,omitempty"` // "keyword", "semantic", "hybrid"
	Weights         *SimilarityWeights `json:"weights,omitempty"`
}

// SimilarityWeights defines weights for different similarity factors
type SimilarityWeights struct {
	Keywords float64 `json:"keywords"`
	Tags     float64 `json:"tags"`
	Content  float64 `json:"content"`
	Recency  float64 `json:"recency"`
}

// PaginationOptions defines pagination parameters
type PaginationOptions struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	Cursor     string `json:"cursor,omitempty"`     // Cursor-based pagination
	PageSize   int    `json:"page_size,omitempty"`
	PageNumber int    `json:"page_number,omitempty"`
}

// OrderingOptions defines result ordering
type OrderingOptions struct {
	Fields     []OrderField `json:"fields"`
	Descending bool         `json:"descending,omitempty"`
	NullsFirst bool         `json:"nulls_first,omitempty"`
}

// OrderField defines ordering by a specific field
type OrderField struct {
	Field      string `json:"field"`
	Descending bool   `json:"descending,omitempty"`
	Priority   int    `json:"priority,omitempty"` // For multi-field ordering
}

// NewNodeQuery creates a new node query with required parameters
func NewNodeQuery(userID domain.UserID) *NodeQuery {
	return &NodeQuery{
		userID:    userID,
		queryID:   domain.NewNodeID().String(), // Reuse ID generation
		createdAt: time.Now(),
	}
}

// Builder methods for NodeQuery - demonstrating fluent interface pattern

func (q *NodeQuery) WithNodeIDs(ids ...domain.NodeID) *NodeQuery {
	q.nodeIDs = append(q.nodeIDs, ids...)
	return q
}

func (q *NodeQuery) WithContentFilter(filter *ContentFilter) *NodeQuery {
	q.contentFilter = filter
	return q
}

func (q *NodeQuery) WithKeywordFilter(filter *KeywordFilter) *NodeQuery {
	q.keywordFilter = filter
	return q
}

func (q *NodeQuery) WithTagFilter(filter *TagFilter) *NodeQuery {
	q.tagFilter = filter
	return q
}

func (q *NodeQuery) WithDateFilter(filter *DateFilter) *NodeQuery {
	q.dateFilter = filter
	return q
}

func (q *NodeQuery) WithSearch(query *SearchQuery) *NodeQuery {
	q.searchQuery = query
	return q
}

func (q *NodeQuery) WithSimilarity(similarity *SimilarityQuery) *NodeQuery {
	q.similarity = similarity
	return q
}

func (q *NodeQuery) WithPagination(pagination *PaginationOptions) *NodeQuery {
	q.pagination = pagination
	return q
}

func (q *NodeQuery) WithOrdering(ordering *OrderingOptions) *NodeQuery {
	q.ordering = ordering
	return q
}

func (q *NodeQuery) IncludeArchived() *NodeQuery {
	q.includeArchived = true
	return q
}

func (q *NodeQuery) IncludeMetadata() *NodeQuery {
	q.includeMetadata = true
	return q
}

func (q *NodeQuery) PreloadEdges() *NodeQuery {
	q.preloadEdges = true
	return q
}

// Query interface implementation
func (q *NodeQuery) Validate() error {
	if q.userID.String() == "" {
		return NewRepositoryError(ErrCodeInvalidQuery, "userID is required", nil)
	}
	
	if q.pagination != nil {
		if err := q.validatePagination(); err != nil {
			return err
		}
	}
	
	if q.contentFilter != nil {
		if err := q.validateContentFilter(); err != nil {
			return err
		}
	}
	
	if q.searchQuery != nil {
		if err := q.validateSearchQuery(); err != nil {
			return err
		}
	}
	
	if q.similarity != nil {
		if err := q.validateSimilarityQuery(); err != nil {
			return err
		}
	}
	
	return nil
}

func (q *NodeQuery) GetUserID() domain.UserID {
	return q.userID
}

func (q *NodeQuery) GetQueryType() string {
	return "NodeQuery"
}

// Query analysis methods

func (q *NodeQuery) IsSimpleQuery() bool {
	return len(q.nodeIDs) > 0 && q.contentFilter == nil && q.keywordFilter == nil && 
		   q.tagFilter == nil && q.searchQuery == nil && q.similarity == nil
}

func (q *NodeQuery) IsComplexQuery() bool {
	return !q.IsSimpleQuery()
}

func (q *NodeQuery) GetComplexityScore() int {
	score := 0
	
	if len(q.nodeIDs) > 0 { score += 1 }
	if q.contentFilter != nil { score += 2 }
	if q.keywordFilter != nil { score += 2 }
	if q.tagFilter != nil { score += 1 }
	if q.dateFilter != nil { score += 1 }
	if q.searchQuery != nil { score += 3 }
	if q.similarity != nil { score += 3 }
	
	return score
}

func (q *NodeQuery) GetEstimatedCost() int {
	// Estimate query cost for optimization
	cost := 1 // Base cost
	
	if q.IsComplexQuery() {
		cost *= q.GetComplexityScore()
	}
	
	if q.searchQuery != nil {
		cost *= 2 // Full-text search is expensive
	}
	
	if q.similarity != nil {
		cost *= 3 // Similarity calculation is very expensive
	}
	
	return cost
}

// Validation helper methods
func (q *NodeQuery) validatePagination() error {
	p := q.pagination
	
	if p.Limit < 0 || p.Offset < 0 {
		return NewRepositoryError(ErrCodeInvalidQuery, "pagination values cannot be negative", nil)
	}
	
	if p.Limit > 1000 {
		return NewRepositoryError(ErrCodeInvalidQuery, "limit cannot exceed 1000", nil)
	}
	
	if p.PageSize < 0 || p.PageNumber < 0 {
		return NewRepositoryError(ErrCodeInvalidQuery, "page values cannot be negative", nil)
	}
	
	return nil
}

func (q *NodeQuery) validateContentFilter() error {
	c := q.contentFilter
	
	if c.MinLength < 0 || c.MaxLength < 0 {
		return NewRepositoryError(ErrCodeInvalidQuery, "content length values cannot be negative", nil)
	}
	
	if c.MinLength > c.MaxLength && c.MaxLength > 0 {
		return NewRepositoryError(ErrCodeInvalidQuery, "min length cannot exceed max length", nil)
	}
	
	return nil
}

func (q *NodeQuery) validateSearchQuery() error {
	s := q.searchQuery
	
	if strings.TrimSpace(s.Query) == "" {
		return NewRepositoryError(ErrCodeInvalidQuery, "search query cannot be empty", nil)
	}
	
	if s.Fuzziness < 0 || s.Fuzziness > 3 {
		return NewRepositoryError(ErrCodeInvalidQuery, "fuzziness must be between 0 and 3", nil)
	}
	
	return nil
}

func (q *NodeQuery) validateSimilarityQuery() error {
	s := q.similarity
	
	if s.ReferenceNodeID.String() == "" {
		return NewRepositoryError(ErrCodeInvalidQuery, "reference node ID is required for similarity query", nil)
	}
	
	if s.MinSimilarity < 0 || s.MinSimilarity > 1 {
		return NewRepositoryError(ErrCodeInvalidQuery, "similarity must be between 0 and 1", nil)
	}
	
	return nil
}

// ==== QUERY RESULT TYPES ====

// NodeQueryResult represents the result of a node query
type NodeQueryResult struct {
	// Result data
	Nodes []*domain.Node `json:"nodes"`
	
	// Result metadata
	TotalCount     int           `json:"total_count"`
	ReturnedCount  int           `json:"returned_count"`
	ExecutionTime  time.Duration `json:"execution_time"`
	HasMore        bool          `json:"has_more"`
	
	// Pagination metadata
	NextCursor   string `json:"next_cursor,omitempty"`
	PreviousCursor string `json:"previous_cursor,omitempty"`
	PageNumber   int    `json:"page_number,omitempty"`
	PageSize     int    `json:"page_size,omitempty"`
	TotalPages   int    `json:"total_pages,omitempty"`
	
	// Query analysis
	QueryID         string  `json:"query_id"`
	QueryComplexity int     `json:"query_complexity"`
	CacheHit        bool    `json:"cache_hit"`
	OptimizationHints []string `json:"optimization_hints,omitempty"`
	
	// Performance metrics
	DatabaseTime time.Duration `json:"database_time"`
	ProcessingTime time.Duration `json:"processing_time"`
	NetworkTime  time.Duration `json:"network_time"`
	
	// Search-specific metadata (when applicable)
	SearchMetadata *SearchMetadata `json:"search_metadata,omitempty"`
}

// SearchMetadata contains search-specific result information
type SearchMetadata struct {
	MaxScore      float64            `json:"max_score"`
	AvgScore      float64            `json:"avg_score"`
	NodeScores    map[string]float64 `json:"node_scores,omitempty"`
	SearchTime    time.Duration      `json:"search_time"`
	IndexUsed     string             `json:"index_used,omitempty"`
	TermFrequency map[string]int     `json:"term_frequency,omitempty"`
}

// NewNodeQueryResult creates a new result set
func NewNodeQueryResult(queryID string) *NodeQueryResult {
	return &NodeQueryResult{
		Nodes:             make([]*domain.Node, 0),
		QueryID:           queryID,
		OptimizationHints: make([]string, 0),
	}
}

// ResultSet interface implementation
func (r *NodeQueryResult) GetTotalCount() int {
	return r.TotalCount
}

func (r *NodeQueryResult) GetReturnedCount() int {
	return r.ReturnedCount
}

func (r *NodeQueryResult) GetExecutionTime() time.Duration {
	return r.ExecutionTime
}

func (r *NodeQueryResult) HasMore() bool {
	return r.HasMore
}

// Result analysis methods
func (r *NodeQueryResult) IsEmpty() bool {
	return len(r.Nodes) == 0
}

func (r *NodeQueryResult) GetEfficiencyRatio() float64 {
	if r.TotalCount == 0 {
		return 1.0
	}
	return float64(r.ReturnedCount) / float64(r.TotalCount)
}

func (r *NodeQueryResult) GetPerformanceScore() float64 {
	// Simple performance scoring based on execution time
	if r.ExecutionTime < 100*time.Millisecond {
		return 1.0
	} else if r.ExecutionTime < 500*time.Millisecond {
		return 0.8
	} else if r.ExecutionTime < 1*time.Second {
		return 0.6
	} else {
		return 0.4
	}
}

func (r *NodeQueryResult) AddOptimizationHint(hint string) {
	r.OptimizationHints = append(r.OptimizationHints, hint)
}

// ==== QUERY BUILDERS ====

// QueryBuilder provides a fluent interface for building complex queries
type QueryBuilder struct {
	query *NodeQuery
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(userID domain.UserID) *QueryBuilder {
	return &QueryBuilder{
		query: NewNodeQuery(userID),
	}
}

// Fluent interface methods
func (b *QueryBuilder) FindByIDs(ids ...domain.NodeID) *QueryBuilder {
	b.query.WithNodeIDs(ids...)
	return b
}

func (b *QueryBuilder) ContainingText(text string) *QueryBuilder {
	b.query.WithContentFilter(&ContentFilter{Contains: text})
	return b
}

func (b *QueryBuilder) WithKeywords(keywords ...string) *QueryBuilder {
	b.query.WithKeywordFilter(&KeywordFilter{IncludeAll: keywords})
	return b
}

func (b *QueryBuilder) WithAnyKeywords(keywords ...string) *QueryBuilder {
	b.query.WithKeywordFilter(&KeywordFilter{IncludeAny: keywords})
	return b
}

func (b *QueryBuilder) WithTags(tags ...string) *QueryBuilder {
	b.query.WithTagFilter(&TagFilter{IncludeAll: tags})
	return b
}

func (b *QueryBuilder) CreatedAfter(date time.Time) *QueryBuilder {
	if b.query.dateFilter == nil {
		b.query.dateFilter = &DateFilter{}
	}
	b.query.dateFilter.CreatedAfter = &date
	return b
}

func (b *QueryBuilder) CreatedInLast(duration time.Duration) *QueryBuilder {
	if b.query.dateFilter == nil {
		b.query.dateFilter = &DateFilter{}
	}
	b.query.dateFilter.CreatedInLast = duration
	return b
}

func (b *QueryBuilder) OrderBy(field string, descending bool) *QueryBuilder {
	b.query.WithOrdering(&OrderingOptions{
		Fields: []OrderField{{Field: field, Descending: descending}},
	})
	return b
}

func (b *QueryBuilder) Limit(limit int) *QueryBuilder {
	if b.query.pagination == nil {
		b.query.pagination = &PaginationOptions{}
	}
	b.query.pagination.Limit = limit
	return b
}

func (b *QueryBuilder) Offset(offset int) *QueryBuilder {
	if b.query.pagination == nil {
		b.query.pagination = &PaginationOptions{}
	}
	b.query.pagination.Offset = offset
	return b
}

func (b *QueryBuilder) SimilarTo(nodeID domain.NodeID, minSimilarity float64) *QueryBuilder {
	b.query.WithSimilarity(&SimilarityQuery{
		ReferenceNodeID: nodeID,
		MinSimilarity:   minSimilarity,
	})
	return b
}

func (b *QueryBuilder) Search(query string) *QueryBuilder {
	b.query.WithSearch(&SearchQuery{Query: query})
	return b
}

func (b *QueryBuilder) Build() (*NodeQuery, error) {
	if err := b.query.Validate(); err != nil {
		return nil, err
	}
	return b.query, nil
}

// ==== USAGE EXAMPLES ====

// Example: Simple query
// query := NewQueryBuilder(userID).
//     WithKeywords("machine", "learning").
//     WithTags("important").
//     OrderBy("created_at", true).
//     Limit(10).
//     Build()

// Example: Complex search query
// query := NewQueryBuilder(userID).
//     Search("artificial intelligence").
//     CreatedInLast(30 * 24 * time.Hour). // Last 30 days
//     WithAnyKeywords("AI", "ML", "neural").
//     OrderBy("relevance", true).
//     Limit(20).
//     Build()

// Example: Similarity query
// query := NewQueryBuilder(userID).
//     SimilarTo(referenceNodeID, 0.7).
//     WithTags("research").
//     OrderBy("similarity", true).
//     Limit(5).
//     Build()

// This demonstrates how strongly-typed query objects can provide:
// - Type safety and validation
// - Rich query capabilities
// - Performance optimization opportunities
// - Detailed result metadata
// - Fluent, readable query construction