// Package repository - Advanced Result Types and Pagination
//
// This file demonstrates comprehensive result handling patterns including
// pagination, metadata collection, performance tracking, and result analysis.
//
// Educational Goals:
//   - Show advanced pagination patterns (cursor-based, offset-based)
//   - Demonstrate result metadata collection
//   - Illustrate performance monitoring in results
//   - Provide rich result analysis capabilities
//   - Enable result caching and optimization
package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"brain2-backend/internal/domain"
)

// ==== PAGINATION TYPES ====

// PaginatedResult represents a paginated result set with comprehensive metadata
type PaginatedResult[T any] struct {
	// Data
	Items []T `json:"items"`
	
	// Pagination metadata
	Pagination PaginationMetadata `json:"pagination"`
	
	// Execution metadata
	Execution ExecutionMetadata `json:"execution"`
	
	// Performance metrics
	Performance PerformanceMetrics `json:"performance"`
	
	// Query analysis
	Analysis QueryAnalysis `json:"analysis,omitempty"`
}

// PaginationMetadata contains pagination information
type PaginationMetadata struct {
	// Counts
	TotalCount    int `json:"total_count"`
	ReturnedCount int `json:"returned_count"`
	
	// Current page info
	CurrentPage int `json:"current_page,omitempty"`
	PageSize    int `json:"page_size,omitempty"`
	TotalPages  int `json:"total_pages,omitempty"`
	
	// Offset-based pagination
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
	
	// Cursor-based pagination
	NextCursor     string `json:"next_cursor,omitempty"`
	PreviousCursor string `json:"previous_cursor,omitempty"`
	
	// Status flags
	HasMore        bool `json:"has_more"`
	HasPrevious    bool `json:"has_previous"`
	IsFirstPage    bool `json:"is_first_page"`
	IsLastPage     bool `json:"is_last_page"`
}

// ExecutionMetadata contains query execution information
type ExecutionMetadata struct {
	QueryID       string        `json:"query_id"`
	ExecutedAt    time.Time     `json:"executed_at"`
	ExecutionTime time.Duration `json:"execution_time"`
	DatabaseTime  time.Duration `json:"database_time"`
	ProcessingTime time.Duration `json:"processing_time"`
	CacheHit      bool          `json:"cache_hit"`
	CacheKey      string        `json:"cache_key,omitempty"`
}

// PerformanceMetrics contains performance analysis data
type PerformanceMetrics struct {
	// Timing breakdown
	QueryPlanTime    time.Duration `json:"query_plan_time"`
	IndexSeekTime    time.Duration `json:"index_seek_time"`
	DataRetrievalTime time.Duration `json:"data_retrieval_time"`
	PostProcessingTime time.Duration `json:"post_processing_time"`
	
	// Resource usage
	MemoryUsed       int64 `json:"memory_used_bytes"`
	CPUTime          time.Duration `json:"cpu_time"`
	IOOperations     int   `json:"io_operations"`
	NetworkRoundTrips int  `json:"network_round_trips"`
	
	// Efficiency metrics
	RowsScanned      int     `json:"rows_scanned"`
	RowsReturned     int     `json:"rows_returned"`
	SelectivityRatio float64 `json:"selectivity_ratio"`
	IndexHitRatio    float64 `json:"index_hit_ratio,omitempty"`
	
	// Performance score (0-100)
	PerformanceScore int `json:"performance_score"`
}

// QueryAnalysis contains query optimization information
type QueryAnalysis struct {
	ComplexityScore   int      `json:"complexity_score"`
	OptimizationHints []string `json:"optimization_hints,omitempty"`
	IndexesUsed       []string `json:"indexes_used,omitempty"`
	BottleneckType    string   `json:"bottleneck_type,omitempty"` // "cpu", "io", "network", "memory"
	
	// Query pattern classification
	QueryPattern string `json:"query_pattern,omitempty"` // "simple", "complex", "analytical", "search"
	QueryCost    int    `json:"query_cost"`
	
	// Recommendations
	CachingRecommendation bool   `json:"caching_recommendation"`
	IndexRecommendation   string `json:"index_recommendation,omitempty"`
	QueryRecommendation   string `json:"query_recommendation,omitempty"`
}

// ==== CURSOR IMPLEMENTATION ====

// Cursor represents a position in a result set for cursor-based pagination
type Cursor struct {
	// Primary position data
	Position CursorPosition `json:"position"`
	
	// Metadata
	CreatedAt time.Time `json:"created_at"`
	QueryHash string    `json:"query_hash"`
	Version   int       `json:"version"`
}

// CursorPosition holds the actual position information
type CursorPosition struct {
	// Primary key or unique identifier
	ID string `json:"id"`
	
	// Sort keys for ordering
	SortKeys map[string]interface{} `json:"sort_keys"`
	
	// Additional context
	UserID    string `json:"user_id"`
	Direction string `json:"direction"` // "forward", "backward"
}

// NewCursor creates a new cursor from a domain object
func NewCursor(item interface{}, sortFields []string, queryHash string) *Cursor {
	position := CursorPosition{
		SortKeys:  make(map[string]interface{}),
		Direction: "forward",
	}
	
	// Extract ID and sort keys based on item type
	switch v := item.(type) {
	case *domain.Node:
		position.ID = v.ID().String()
		position.UserID = v.UserID().String()
		
		// Add common sort keys
		for _, field := range sortFields {
			switch field {
			case "created_at":
				position.SortKeys[field] = v.CreatedAt()
			case "updated_at":
				position.SortKeys[field] = v.UpdatedAt()
			case "content_length":
				position.SortKeys[field] = len(v.Content().String())
			}
		}
	}
	
	return &Cursor{
		Position:  position,
		CreatedAt: time.Now(),
		QueryHash: queryHash,
		Version:   1,
	}
}

// Encode serializes the cursor to a string for use in APIs
func (c *Cursor) Encode() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	
	encoded := base64.URLEncoding.EncodeToString(data)
	return encoded, nil
}

// DecodeCursor deserializes a cursor string
func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}
	
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}
	
	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}
	
	return &cursor, nil
}

// IsValid checks if the cursor is valid (not expired, correct format, etc.)
func (c *Cursor) IsValid(maxAge time.Duration) bool {
	if c == nil {
		return false
	}
	
	// Check age
	if maxAge > 0 && time.Since(c.CreatedAt) > maxAge {
		return false
	}
	
	// Check required fields
	if c.Position.ID == "" {
		return false
	}
	
	return true
}

// ==== RESULT BUILDERS ====

// ResultBuilder helps construct paginated results with proper metadata
type ResultBuilder[T any] struct {
	items       []T
	query       Query
	startTime   time.Time
	totalCount  int
	pagination  *PaginationOptions
	cacheHit    bool
	performance PerformanceMetrics
}

// NewResultBuilder creates a new result builder
func NewResultBuilder[T any](query Query) *ResultBuilder[T] {
	return &ResultBuilder[T]{
		query:     query,
		startTime: time.Now(),
		items:     make([]T, 0),
	}
}

// WithItems sets the result items
func (b *ResultBuilder[T]) WithItems(items []T) *ResultBuilder[T] {
	b.items = items
	return b
}

// WithTotalCount sets the total count of available results
func (b *ResultBuilder[T]) WithTotalCount(count int) *ResultBuilder[T] {
	b.totalCount = count
	return b
}

// WithPagination sets pagination options
func (b *ResultBuilder[T]) WithPagination(pagination *PaginationOptions) *ResultBuilder[T] {
	b.pagination = pagination
	return b
}

// WithCacheHit marks this result as a cache hit
func (b *ResultBuilder[T]) WithCacheHit(hit bool) *ResultBuilder[T] {
	b.cacheHit = hit
	return b
}

// WithPerformance sets performance metrics
func (b *ResultBuilder[T]) WithPerformance(perf PerformanceMetrics) *ResultBuilder[T] {
	b.performance = perf
	return b
}

// Build constructs the final paginated result
func (b *ResultBuilder[T]) Build() *PaginatedResult[T] {
	executionTime := time.Since(b.startTime)
	returnedCount := len(b.items)
	
	// Build pagination metadata
	paginationMeta := b.buildPaginationMetadata(returnedCount)
	
	// Build execution metadata
	executionMeta := ExecutionMetadata{
		QueryID:       b.getQueryID(),
		ExecutedAt:    b.startTime,
		ExecutionTime: executionTime,
		DatabaseTime:  b.performance.QueryPlanTime + b.performance.DataRetrievalTime,
		ProcessingTime: b.performance.PostProcessingTime,
		CacheHit:      b.cacheHit,
	}
	
	// Build analysis if query is complex
	var analysis *QueryAnalysis
	if complexQuery, ok := b.query.(*NodeQuery); ok && complexQuery.IsComplexQuery() {
		analysis = b.buildQueryAnalysis(complexQuery, executionTime)
	}
	
	result := &PaginatedResult[T]{
		Items:       b.items,
		Pagination:  paginationMeta,
		Execution:   executionMeta,
		Performance: b.performance,
	}
	
	if analysis != nil {
		result.Analysis = *analysis
	}
	
	return result
}

// Helper methods for building metadata

func (b *ResultBuilder[T]) buildPaginationMetadata(returnedCount int) PaginationMetadata {
	meta := PaginationMetadata{
		TotalCount:    b.totalCount,
		ReturnedCount: returnedCount,
	}
	
	if b.pagination != nil {
		// Offset-based pagination
		if b.pagination.Limit > 0 {
			meta.Limit = b.pagination.Limit
			meta.Offset = b.pagination.Offset
			meta.HasMore = b.totalCount > (b.pagination.Offset + returnedCount)
			meta.HasPrevious = b.pagination.Offset > 0
		}
		
		// Page-based pagination
		if b.pagination.PageSize > 0 {
			meta.PageSize = b.pagination.PageSize
			meta.CurrentPage = b.pagination.PageNumber
			meta.TotalPages = (b.totalCount + b.pagination.PageSize - 1) / b.pagination.PageSize
			meta.IsFirstPage = b.pagination.PageNumber <= 1
			meta.IsLastPage = b.pagination.PageNumber >= meta.TotalPages
		}
		
		// Cursor-based pagination would be handled here
		if b.pagination.Cursor != "" {
			// Generate next cursor from last item
			if len(b.items) > 0 {
				if cursor := b.generateCursor(b.items[len(b.items)-1]); cursor != "" {
					meta.NextCursor = cursor
				}
			}
		}
	}
	
	return meta
}

func (b *ResultBuilder[T]) buildQueryAnalysis(query *NodeQuery, executionTime time.Duration) *QueryAnalysis {
	analysis := &QueryAnalysis{
		ComplexityScore:   query.GetComplexityScore(),
		QueryCost:         query.GetEstimatedCost(),
		OptimizationHints: make([]string, 0),
	}
	
	// Classify query pattern
	if query.searchQuery != nil {
		analysis.QueryPattern = "search"
	} else if query.similarity != nil {
		analysis.QueryPattern = "analytical"
	} else if query.IsComplexQuery() {
		analysis.QueryPattern = "complex"
	} else {
		analysis.QueryPattern = "simple"
	}
	
	// Determine bottleneck
	if executionTime > 2*time.Second {
		if b.performance.IOOperations > 100 {
			analysis.BottleneckType = "io"
		} else if b.performance.NetworkRoundTrips > 10 {
			analysis.BottleneckType = "network"
		} else {
			analysis.BottleneckType = "cpu"
		}
	}
	
	// Generate optimization hints
	if executionTime > 1*time.Second {
		analysis.OptimizationHints = append(analysis.OptimizationHints, "Consider adding indexes for faster queries")
	}
	
	if b.performance.RowsScanned > 1000 && len(b.items) < 100 {
		analysis.OptimizationHints = append(analysis.OptimizationHints, "Query has low selectivity - consider more specific filters")
	}
	
	if !b.cacheHit && analysis.QueryPattern == "simple" {
		analysis.CachingRecommendation = true
		analysis.OptimizationHints = append(analysis.OptimizationHints, "This query would benefit from caching")
	}
	
	return analysis
}

func (b *ResultBuilder[T]) getQueryID() string {
	// Extract query ID based on query type
	if nodeQuery, ok := b.query.(*NodeQuery); ok {
		return nodeQuery.queryID
	}
	return fmt.Sprintf("query_%d", time.Now().UnixNano())
}

func (b *ResultBuilder[T]) generateCursor(item T) string {
	// This would generate a cursor based on the item
	// Implementation depends on the item type and sort fields
	return ""
}

// ==== SPECIALIZED RESULT TYPES ====

// NodeResult represents a node query result with node-specific metadata
type NodeResult struct {
	*PaginatedResult[*domain.Node]
	
	// Node-specific metadata
	ContentStats  ContentStatistics  `json:"content_stats,omitempty"`
	KeywordStats  KeywordStatistics  `json:"keyword_stats,omitempty"`
	TagStats      TagStatistics      `json:"tag_stats,omitempty"`
	SearchStats   *SearchStatistics  `json:"search_stats,omitempty"`
}

// ContentStatistics provides analysis of content in the result set
type ContentStatistics struct {
	TotalWords      int     `json:"total_words"`
	AverageWords    float64 `json:"average_words"`
	MinWords        int     `json:"min_words"`
	MaxWords        int     `json:"max_words"`
	TotalCharacters int     `json:"total_characters"`
	
	// Content distribution
	ShortContent  int `json:"short_content"`  // < 100 words
	MediumContent int `json:"medium_content"` // 100-500 words  
	LongContent   int `json:"long_content"`   // > 500 words
}

// KeywordStatistics provides keyword analysis
type KeywordStatistics struct {
	TotalKeywords    int               `json:"total_keywords"`
	UniqueKeywords   int               `json:"unique_keywords"`
	AverageKeywords  float64           `json:"average_keywords"`
	TopKeywords      []KeywordCount    `json:"top_keywords"`
	KeywordFrequency map[string]int    `json:"keyword_frequency,omitempty"`
}

// KeywordCount represents a keyword and its frequency
type KeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

// TagStatistics provides tag analysis
type TagStatistics struct {
	TotalTags     int            `json:"total_tags"`
	UniqueTags    int            `json:"unique_tags"`
	AverageTags   float64        `json:"average_tags"`
	TopTags       []TagCount     `json:"top_tags"`
	TagFrequency  map[string]int `json:"tag_frequency,omitempty"`
}

// TagCount represents a tag and its frequency
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// SearchStatistics provides search-specific metadata
type SearchStatistics struct {
	Query         string             `json:"query"`
	ResultScores  map[string]float64 `json:"result_scores,omitempty"`
	MaxScore      float64            `json:"max_score"`
	MinScore      float64            `json:"min_score"`
	AverageScore  float64            `json:"average_score"`
	IndexUsed     string             `json:"index_used,omitempty"`
	SearchTime    time.Duration      `json:"search_time"`
	
	// Term analysis
	QueryTerms        []string       `json:"query_terms"`
	MatchedTerms      []string       `json:"matched_terms"`
	TermFrequencies   map[string]int `json:"term_frequencies,omitempty"`
}

// NewNodeResult creates a new node result with statistics
func NewNodeResult(paginatedResult *PaginatedResult[*domain.Node]) *NodeResult {
	result := &NodeResult{
		PaginatedResult: paginatedResult,
	}
	
	// Calculate statistics
	result.calculateStatistics()
	
	return result
}

// calculateStatistics computes node-specific statistics
func (r *NodeResult) calculateStatistics() {
	if len(r.Items) == 0 {
		return
	}
	
	// Content statistics
	r.ContentStats = r.calculateContentStats()
	
	// Keyword statistics
	r.KeywordStats = r.calculateKeywordStats()
	
	// Tag statistics
	r.TagStats = r.calculateTagStats()
}

func (r *NodeResult) calculateContentStats() ContentStatistics {
	stats := ContentStatistics{}
	
	totalWords := 0
	totalChars := 0
	minWords := int(^uint(0) >> 1) // Max int
	maxWords := 0
	
	for _, node := range r.Items {
		words := node.Content().WordCount()
		chars := len(node.Content().String())
		
		totalWords += words
		totalChars += chars
		
		if words < minWords {
			minWords = words
		}
		if words > maxWords {
			maxWords = words
		}
		
		// Categorize content length
		if words < 100 {
			stats.ShortContent++
		} else if words < 500 {
			stats.MediumContent++
		} else {
			stats.LongContent++
		}
	}
	
	count := len(r.Items)
	stats.TotalWords = totalWords
	stats.TotalCharacters = totalChars
	stats.MinWords = minWords
	stats.MaxWords = maxWords
	
	if count > 0 {
		stats.AverageWords = float64(totalWords) / float64(count)
	}
	
	return stats
}

func (r *NodeResult) calculateKeywordStats() KeywordStatistics {
	stats := KeywordStatistics{}
	keywordCounts := make(map[string]int)
	totalKeywords := 0
	
	for _, node := range r.Items {
		nodeKeywords := node.Keywords().ToSlice()
		totalKeywords += len(nodeKeywords)
		
		for _, keyword := range nodeKeywords {
			keywordCounts[keyword]++
		}
	}
	
	stats.TotalKeywords = totalKeywords
	stats.UniqueKeywords = len(keywordCounts)
	stats.KeywordFrequency = keywordCounts
	
	if len(r.Items) > 0 {
		stats.AverageKeywords = float64(totalKeywords) / float64(len(r.Items))
	}
	
	// Get top keywords
	type kc struct {
		keyword string
		count   int
	}
	
	var pairs []kc
	for k, v := range keywordCounts {
		pairs = append(pairs, kc{k, v})
	}
	
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})
	
	// Top 10 keywords
	limit := 10
	if len(pairs) < limit {
		limit = len(pairs)
	}
	
	stats.TopKeywords = make([]KeywordCount, limit)
	for i := 0; i < limit; i++ {
		stats.TopKeywords[i] = KeywordCount{
			Keyword: pairs[i].keyword,
			Count:   pairs[i].count,
		}
	}
	
	return stats
}

func (r *NodeResult) calculateTagStats() TagStatistics {
	stats := TagStatistics{}
	tagCounts := make(map[string]int)
	totalTags := 0
	
	for _, node := range r.Items {
		nodeTags := node.Tags().ToSlice()
		totalTags += len(nodeTags)
		
		for _, tag := range nodeTags {
			tagCounts[tag]++
		}
	}
	
	stats.TotalTags = totalTags
	stats.UniqueTags = len(tagCounts)
	stats.TagFrequency = tagCounts
	
	if len(r.Items) > 0 {
		stats.AverageTags = float64(totalTags) / float64(len(r.Items))
	}
	
	// Get top tags (similar to keywords logic)
	type tc struct {
		tag   string
		count int
	}
	
	var pairs []tc
	for k, v := range tagCounts {
		pairs = append(pairs, tc{k, v})
	}
	
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})
	
	limit := 10
	if len(pairs) < limit {
		limit = len(pairs)
	}
	
	stats.TopTags = make([]TagCount, limit)
	for i := 0; i < limit; i++ {
		stats.TopTags[i] = TagCount{
			Tag:   pairs[i].tag,
			Count: pairs[i].count,
		}
	}
	
	return stats
}

// Example usage:
//
// // Build a result with comprehensive metadata
// result := NewResultBuilder[*domain.Node](query).
//     WithItems(nodes).
//     WithTotalCount(totalCount).
//     WithPagination(paginationOpts).
//     WithPerformance(perfMetrics).
//     Build()
//
// // Convert to specialized node result with statistics
// nodeResult := NewNodeResult(result)
//
// // Access rich metadata
// fmt.Printf("Average content length: %.1f words\n", nodeResult.ContentStats.AverageWords)
// fmt.Printf("Top keyword: %s (%d occurrences)\n", 
//     nodeResult.KeywordStats.TopKeywords[0].Keyword,
//     nodeResult.KeywordStats.TopKeywords[0].Count)

// This demonstrates comprehensive result handling with:
// - Rich pagination metadata
// - Performance tracking
// - Query optimization hints
// - Domain-specific statistics
// - Cursor-based pagination support