package repository

import (
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
)

// QueryBuilder provides a fluent API for building complex repository queries.
//
// Key Concepts Illustrated:
//   1. Builder Pattern: Step-by-step construction of complex objects
//   2. Fluent Interface: Method chaining for readable query construction
//   3. Functional Options: Flexible configuration using functions
//   4. Type Safety: Compile-time validation of query parameters
//   5. Query Optimization: Automatic optimization of query structures
//
// This implementation demonstrates how to create expressive, type-safe queries
// that are both powerful for complex scenarios and simple for basic use cases.
//
// Example Usage:
//   // Simple query
//   nodes, err := repo.FindBySpecification(ctx,
//       NewQueryBuilder().
//           ForUser(userID).
//           WithKeywords("machine", "learning").
//           CreatedAfter(lastWeek).
//           Build())
//   
//   // Complex query with multiple conditions
//   query := NewQueryBuilder().
//       ForUser(userID).
//       Where(
//           ContentContains("neural networks").
//           Or(TaggedWith("AI").And(CreatedAfter(lastMonth))).
//           And(Not(ArchivedSpec()))).
//       OrderBy("created_at", Descending).
//       Limit(50).
//       Build()
type QueryBuilder struct {
	userID         shared.UserID
	specifications []Specification
	sorting        []SortCriteria
	pagination     PaginationConfig
	options        QueryOptions
	filters        []QueryFilter
	projections    []string
	aggregations   []Aggregation
}

// SortCriteria represents sorting configuration
type SortCriteria struct {
	Field     string
	Direction SortDirection
	Priority  int // For multi-field sorting
}

// SortDirection represents sort direction
type SortDirection string

const (
	Ascending  SortDirection = "asc"
	Descending SortDirection = "desc"
)

// PaginationConfig represents pagination settings
type PaginationConfig struct {
	Limit  int
	Offset int
	Cursor string
	Style  PaginationStyle
}

// PaginationStyle represents pagination approach
type PaginationStyle string

const (
	OffsetPagination PaginationStyle = "offset"
	CursorPagination PaginationStyle = "cursor"
	KeysetPagination PaginationStyle = "keyset"
)

// QueryFilter represents a single filter condition
type QueryFilter struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
	Negate   bool
}

// FilterOperator represents comparison operators
type FilterOperator string

const (
	Equals       FilterOperator = "eq"
	NotEquals    FilterOperator = "ne"
	GreaterThan  FilterOperator = "gt"
	LessThan     FilterOperator = "lt"
	Contains     FilterOperator = "contains"
	StartsWith   FilterOperator = "starts_with"
	EndsWith     FilterOperator = "ends_with"
	InFilter     FilterOperator = "in"
	Between      FilterOperator = "between"
	IsNull       FilterOperator = "is_null"
	IsNotNull    FilterOperator = "is_not_null"
	Matches      FilterOperator = "matches" // Regex
)

// Aggregation represents aggregation operations
type Aggregation struct {
	Type   AggregationType
	Field  string
	Alias  string
	Filter Specification // Optional filter for aggregation
}

// AggregationType represents different aggregation operations
type AggregationType string

const (
	Count     AggregationType = "count"
	Sum       AggregationType = "sum"
	Avg       AggregationType = "avg"
	Min       AggregationType = "min"
	Max       AggregationType = "max"
	Distinct  AggregationType = "distinct"
	GroupBy   AggregationType = "group_by"
)

// NewQueryBuilder creates a new query builder instance
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		specifications: make([]Specification, 0),
		sorting:        make([]SortCriteria, 0),
		filters:        make([]QueryFilter, 0),
		projections:    make([]string, 0),
		aggregations:   make([]Aggregation, 0),
		options:        QueryOptions{},
		pagination: PaginationConfig{
			Style:  OffsetPagination,
			Limit:  50, // Default limit
		},
	}
}

// Basic Query Building Methods

// ForUser sets the user context for the query (required for most queries)
func (qb *QueryBuilder) ForUser(userID shared.UserID) *QueryBuilder {
	qb.userID = userID
	return qb
}

// Where adds a specification to the query
func (qb *QueryBuilder) Where(spec Specification) *QueryBuilder {
	qb.specifications = append(qb.specifications, spec)
	return qb
}

// Filter adds a simple field-based filter
func (qb *QueryBuilder) Filter(field string, operator FilterOperator, value interface{}) *QueryBuilder {
	qb.filters = append(qb.filters, QueryFilter{
		Field:    field,
		Operator: operator,
		Value:    value,
		Negate:   false,
	})
	return qb
}

// FilterNot adds a negated field-based filter
func (qb *QueryBuilder) FilterNot(field string, operator FilterOperator, value interface{}) *QueryBuilder {
	qb.filters = append(qb.filters, QueryFilter{
		Field:    field,
		Operator: operator,
		Value:    value,
		Negate:   true,
	})
	return qb
}

// Convenience Methods for Common Filters

// WithKeywords adds a keyword search condition
func (qb *QueryBuilder) WithKeywords(keywords ...string) *QueryBuilder {
	for _, keyword := range keywords {
		spec := NewKeywordContainsSpec(keyword)
		qb.specifications = append(qb.specifications, spec)
	}
	return qb
}

// WithTags adds tag filtering conditions
func (qb *QueryBuilder) WithTags(tags ...string) *QueryBuilder {
	for _, tag := range tags {
		spec := NewTaggedWithSpec(tag)
		qb.specifications = append(qb.specifications, spec)
	}
	return qb
}

// CreatedAfter adds a creation time filter
func (qb *QueryBuilder) CreatedAfter(afterTime time.Time) *QueryBuilder {
	spec := NewCreatedAfterSpec(afterTime)
	qb.specifications = append(qb.specifications, spec)
	return qb
}

// CreatedBefore adds a creation time filter
func (qb *QueryBuilder) CreatedBefore(beforeTime time.Time) *QueryBuilder {
	qb.Filter("created_at", LessThan, beforeTime)
	return qb
}

// CreatedBetween adds a creation time range filter
func (qb *QueryBuilder) CreatedBetween(startTime, endTime time.Time) *QueryBuilder {
	qb.Filter("created_at", Between, []time.Time{startTime, endTime})
	return qb
}

// ContentContains adds a content search condition
func (qb *QueryBuilder) ContentContains(searchTerm string, fuzzy bool) *QueryBuilder {
	spec := NewContentContainsSpec(searchTerm, fuzzy)
	qb.specifications = append(qb.specifications, spec)
	return qb
}

// OnlyArchived filters for archived entities only
func (qb *QueryBuilder) OnlyArchived() *QueryBuilder {
	spec := NewArchivedSpec(true)
	qb.specifications = append(qb.specifications, spec)
	return qb
}

// ExcludeArchived filters out archived entities
func (qb *QueryBuilder) ExcludeArchived() *QueryBuilder {
	spec := NewArchivedSpec(false)
	qb.specifications = append(qb.specifications, spec)
	return qb
}

// Sorting Methods

// OrderBy adds a sorting criterion
func (qb *QueryBuilder) OrderBy(field string, direction SortDirection) *QueryBuilder {
	qb.sorting = append(qb.sorting, SortCriteria{
		Field:     field,
		Direction: direction,
		Priority:  len(qb.sorting) + 1,
	})
	return qb
}

// OrderByCreatedAt sorts by creation time
func (qb *QueryBuilder) OrderByCreatedAt(direction SortDirection) *QueryBuilder {
	return qb.OrderBy("created_at", direction)
}

// OrderByUpdatedAt sorts by update time
func (qb *QueryBuilder) OrderByUpdatedAt(direction SortDirection) *QueryBuilder {
	return qb.OrderBy("updated_at", direction)
}

// OrderByRelevance sorts by relevance score (for search queries)
func (qb *QueryBuilder) OrderByRelevance() *QueryBuilder {
	return qb.OrderBy("relevance_score", Descending)
}

// Pagination Methods

// Limit sets the maximum number of results
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.pagination.Limit = limit
	return qb
}

// Offset sets the number of results to skip (offset pagination)
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.pagination.Offset = offset
	qb.pagination.Style = OffsetPagination
	return qb
}

// After sets cursor for cursor-based pagination
func (qb *QueryBuilder) After(cursor string) *QueryBuilder {
	qb.pagination.Cursor = cursor
	qb.pagination.Style = CursorPagination
	return qb
}

// Page sets page number and size (convenience method)
func (qb *QueryBuilder) Page(pageNumber, pageSize int) *QueryBuilder {
	qb.pagination.Limit = pageSize
	qb.pagination.Offset = (pageNumber - 1) * pageSize
	qb.pagination.Style = OffsetPagination
	return qb
}

// Projection Methods

// Select specifies which fields to include in results
func (qb *QueryBuilder) Select(fields ...string) *QueryBuilder {
	qb.projections = append(qb.projections, fields...)
	return qb
}

// SelectOnly replaces current projections with specified fields
func (qb *QueryBuilder) SelectOnly(fields ...string) *QueryBuilder {
	qb.projections = fields
	return qb
}

// Aggregation Methods

// Count adds a count aggregation
func (qb *QueryBuilder) Count(field string, alias string) *QueryBuilder {
	qb.aggregations = append(qb.aggregations, Aggregation{
		Type:  Count,
		Field: field,
		Alias: alias,
	})
	return qb
}

// CountDistinct adds a distinct count aggregation
func (qb *QueryBuilder) CountDistinct(field string, alias string) *QueryBuilder {
	qb.aggregations = append(qb.aggregations, Aggregation{
		Type:  Distinct,
		Field: field,
		Alias: alias,
	})
	return qb
}

// GroupBy adds a group by clause
func (qb *QueryBuilder) GroupBy(field string) *QueryBuilder {
	qb.aggregations = append(qb.aggregations, Aggregation{
		Type:  GroupBy,
		Field: field,
	})
	return qb
}

// Advanced Query Options

// WithCache enables caching for this query
func (qb *QueryBuilder) WithCache(ttl time.Duration) *QueryBuilder {
	qb.options.UseCache = true
	qb.options.CacheTimeout = int(ttl.Seconds())
	return qb
}

// WithReadPreference sets the read preference
func (qb *QueryBuilder) WithReadPreference(preference ReadPreference) *QueryBuilder {
	qb.options.ReadPreference = preference
	return qb
}

// IncludeDeleted includes soft-deleted entities in results
func (qb *QueryBuilder) IncludeDeleted() *QueryBuilder {
	qb.options.IncludeDeleted = true
	return qb
}

// Explain enables query execution plan explanation (for debugging)
func (qb *QueryBuilder) Explain() *QueryBuilder {
	qb.options.Fields = append(qb.options.Fields, "_explain")
	return qb
}

// Build Methods

// Build finalizes the query and returns a combined specification
func (qb *QueryBuilder) Build() Specification {
	if len(qb.specifications) == 0 && len(qb.filters) == 0 {
		// Return user-owned spec if no other conditions specified
		return NewUserOwnedSpec(qb.userID)
	}
	
	// Combine all specifications
	var combined Specification
	
	// Start with user ownership (always required)
	combined = NewUserOwnedSpec(qb.userID)
	
	// Add all specifications
	for _, spec := range qb.specifications {
		combined = combined.And(spec)
	}
	
	// Convert filters to specifications
	for _, filter := range qb.filters {
		filterSpec := qb.filterToSpecification(filter)
		if filterSpec != nil {
			combined = combined.And(filterSpec)
		}
	}
	
	return combined
}

// BuildQuery builds a traditional repository query object
func (qb *QueryBuilder) BuildQuery() NodeQuery {
	query := NodeQuery{
		UserID: qb.userID.String(),
		Limit:  qb.pagination.Limit,
		Offset: qb.pagination.Offset,
	}
	
	// Extract keywords and node IDs from specifications
	for range qb.specifications {
		// Note: This requires exposing the keyword field or adding a method
		// For now, we'll leave this as a placeholder
	}
	
	return query
}

// BuildOptions builds query options from the builder state
func (qb *QueryBuilder) BuildOptions() *QueryOptions {
	options := qb.options
	options.Limit = qb.pagination.Limit
	options.Offset = qb.pagination.Offset
	options.Cursor = qb.pagination.Cursor
	options.Fields = qb.projections
	
	// Convert sorting to options
	if len(qb.sorting) > 0 {
		// Use the first sort criteria (can be extended to support multiple)
		primary := qb.sorting[0]
		options.SortBy = primary.Field
		options.SortOrder = SortOrder(primary.Direction)
	}
	
	// Convert filters to Filter objects
	for _, filter := range qb.filters {
		filterObj := qb.convertToFilter(filter)
		options.Filters = append(options.Filters, filterObj)
	}
	
	return &options
}

// Helper Methods

// filterToSpecification converts a QueryFilter to a Specification
func (qb *QueryBuilder) filterToSpecification(filter QueryFilter) Specification {
	// This would create appropriate specifications based on the filter
	// For now, we'll return a simple field-based specification
	switch filter.Field {
	case "created_at":
		if filter.Operator == GreaterThan {
			if timestamp, ok := filter.Value.(time.Time); ok {
				spec := NewCreatedAfterSpec(timestamp)
				if filter.Negate {
					return spec.Not()
				}
				return spec
			}
		}
	case "archived":
		if filter.Operator == Equals {
			if archived, ok := filter.Value.(bool); ok {
				spec := NewArchivedSpec(archived)
				if filter.Negate {
					return spec.Not()
				}
				return spec
			}
		}
	}
	
	// For unsupported filters, return nil (they'll be handled at the repository level)
	return nil
}

// convertToFilter converts a QueryFilter to a Filter object
func (qb *QueryBuilder) convertToFilter(queryFilter QueryFilter) Filter {
	fieldFilter := FieldFilter{
		Field:    queryFilter.Field,
		Operator: Operator(queryFilter.Operator),
		Value:    queryFilter.Value,
	}
	
	filter := Filter{
		FieldFilters: []FieldFilter{fieldFilter},
	}
	
	if queryFilter.Negate {
		filter.LogicalOperator = LogicalOperatorNot
		filter.SubFilters = []Filter{{FieldFilters: []FieldFilter{fieldFilter}}}
		filter.FieldFilters = nil
	}
	
	return filter
}

// Complex Query Builder for Advanced Scenarios

// ComplexQueryBuilder provides advanced query building capabilities
type ComplexQueryBuilder struct {
	*QueryBuilder
	subQueries []SubQuery
	joins      []Join
	unions     []Union
}

// SubQuery represents a sub-query within a larger query
type SubQuery struct {
	Query  Specification
	Alias  string
	Type   SubQueryType
}

// SubQueryType represents different types of sub-queries
type SubQueryType string

const (
	Exists    SubQueryType = "exists"
	NotExists SubQueryType = "not_exists"
	InSub     SubQueryType = "in"
	NotInSub  SubQueryType = "not_in"
)

// Join represents a join operation between entities
type Join struct {
	Type      JoinType
	Entity    string
	Condition Specification
	Alias     string
}

// JoinType represents different join types
type JoinType string

const (
	InnerJoin JoinType = "inner"
	LeftJoin  JoinType = "left"
	RightJoin JoinType = "right"
	FullJoin  JoinType = "full"
)

// Union represents a union operation between queries
type Union struct {
	Query Specification
	Type  UnionType
}

// UnionType represents union operation types
type UnionType string

const (
	UnionAll      UnionType = "union_all"
	UnionDistinct UnionType = "union_distinct"
)

// NewComplexQueryBuilder creates a new complex query builder
func NewComplexQueryBuilder() *ComplexQueryBuilder {
	return &ComplexQueryBuilder{
		QueryBuilder: NewQueryBuilder(),
		subQueries:   make([]SubQuery, 0),
		joins:        make([]Join, 0),
		unions:       make([]Union, 0),
	}
}

// Exists adds an EXISTS sub-query condition
func (cqb *ComplexQueryBuilder) Exists(subQuery Specification, alias string) *ComplexQueryBuilder {
	cqb.subQueries = append(cqb.subQueries, SubQuery{
		Query: subQuery,
		Alias: alias,
		Type:  Exists,
	})
	return cqb
}

// NotExists adds a NOT EXISTS sub-query condition
func (cqb *ComplexQueryBuilder) NotExists(subQuery Specification, alias string) *ComplexQueryBuilder {
	cqb.subQueries = append(cqb.subQueries, SubQuery{
		Query: subQuery,
		Alias: alias,
		Type:  NotExists,
	})
	return cqb
}

// JoinWith adds a join condition
func (cqb *ComplexQueryBuilder) JoinWith(joinType JoinType, entity string, condition Specification, alias string) *ComplexQueryBuilder {
	cqb.joins = append(cqb.joins, Join{
		Type:      joinType,
		Entity:    entity,
		Condition: condition,
		Alias:     alias,
	})
	return cqb
}

// UnionWith adds a union operation
func (cqb *ComplexQueryBuilder) UnionWith(query Specification, unionType UnionType) *ComplexQueryBuilder {
	cqb.unions = append(cqb.unions, Union{
		Query: query,
		Type:  unionType,
	})
	return cqb
}

// Predefined Query Templates for Common Use Cases

// RecentActivityQuery builds a query for recently active nodes
func RecentActivityQuery(userID shared.UserID, days int) *QueryBuilder {
	since := time.Now().AddDate(0, 0, -days)
	
	return NewQueryBuilder().
		ForUser(userID).
		CreatedAfter(since).
		ExcludeArchived().
		OrderByCreatedAt(Descending).
		Limit(100)
}

// PopularContentQuery builds a query for popular content (highly connected nodes)
func PopularContentQuery(userID shared.UserID, threshold float64) *QueryBuilder {
	// This would require additional specifications for connection counting
	// For now, we'll create a basic query
	return NewQueryBuilder().
		ForUser(userID).
		ExcludeArchived().
		OrderByCreatedAt(Descending).
		Limit(50)
}

// ContentSearchQuery builds a full-text search query
func ContentSearchQuery(userID shared.UserID, searchTerm string, fuzzy bool) *QueryBuilder {
	return NewQueryBuilder().
		ForUser(userID).
		ContentContains(searchTerm, fuzzy).
		ExcludeArchived().
		OrderByRelevance().
		Limit(100)
}

// StaleContentQuery builds a query for old, unused content
func StaleContentQuery(userID shared.UserID, olderThanDays int) *QueryBuilder {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	
	return NewQueryBuilder().
		ForUser(userID).
		CreatedBefore(cutoff).
		ExcludeArchived().
		OrderByCreatedAt(Ascending). // Oldest first
		Limit(200)
}

// Query Validation and Optimization

// QueryValidator validates query correctness
type QueryValidator struct {
	rules []ValidationRule
}

// ValidationRule represents a query validation rule
type ValidationRule struct {
	Name      string
	Validator func(*QueryBuilder) error
	Message   string
}

// NewQueryValidator creates a new query validator with default rules
func NewQueryValidator() *QueryValidator {
	return &QueryValidator{
		rules: []ValidationRule{
			{
				Name:      "user_required",
				Validator: validateUserRequired,
				Message:   "User ID is required for all queries",
			},
			{
				Name:      "reasonable_limit",
				Validator: validateReasonableLimit,
				Message:   "Query limit should be between 1 and 10000",
			},
			{
				Name:      "valid_sorting",
				Validator: validateSorting,
				Message:   "Sort fields must be valid and sortable",
			},
		},
	}
}

// Validate validates a query using all rules
func (qv *QueryValidator) Validate(qb *QueryBuilder) error {
	for _, rule := range qv.rules {
		if err := rule.Validator(qb); err != nil {
			return fmt.Errorf("validation rule '%s' failed: %s - %v", rule.Name, rule.Message, err)
		}
	}
	return nil
}

// Validation rule implementations

func validateUserRequired(qb *QueryBuilder) error {
	if qb.userID.String() == "" {
		return fmt.Errorf("user ID is required")
	}
	return nil
}

func validateReasonableLimit(qb *QueryBuilder) error {
	limit := qb.pagination.Limit
	if limit < 1 || limit > 10000 {
		return fmt.Errorf("limit %d is outside reasonable range [1, 10000]", limit)
	}
	return nil
}

func validateSorting(qb *QueryBuilder) error {
	validSortFields := map[string]bool{
		"created_at":       true,
		"updated_at":       true,
		"relevance_score":  true,
		"content_length":   true,
		"keyword_count":    true,
	}
	
	for _, sort := range qb.sorting {
		if !validSortFields[sort.Field] {
			return fmt.Errorf("sort field '%s' is not valid or sortable", sort.Field)
		}
		if sort.Direction != Ascending && sort.Direction != Descending {
			return fmt.Errorf("sort direction '%s' is invalid", sort.Direction)
		}
	}
	
	return nil
}

// Query Performance Optimizer

// QueryOptimizer optimizes queries for better performance
type QueryOptimizer struct {
	rules []OptimizationRule
}

// OptimizationRule represents a query optimization rule
type OptimizationRule struct {
	Name      string
	Optimizer func(*QueryBuilder) *QueryBuilder
	Condition func(*QueryBuilder) bool
}

// NewQueryOptimizer creates a new query optimizer with default rules
func NewQueryOptimizer() *QueryOptimizer {
	return &QueryOptimizer{
		rules: []OptimizationRule{
			{
				Name:      "limit_large_queries",
				Optimizer: optimizeLargeQueries,
				Condition: func(qb *QueryBuilder) bool { return qb.pagination.Limit > 1000 },
			},
			{
				Name:      "index_hint_for_sorting",
				Optimizer: optimizeSorting,
				Condition: func(qb *QueryBuilder) bool { return len(qb.sorting) > 0 },
			},
		},
	}
}

// Optimize applies all applicable optimization rules
func (qo *QueryOptimizer) Optimize(qb *QueryBuilder) *QueryBuilder {
	optimized := qb
	
	for _, rule := range qo.rules {
		if rule.Condition(optimized) {
			optimized = rule.Optimizer(optimized)
		}
	}
	
	return optimized
}

// Optimization rule implementations

func optimizeLargeQueries(qb *QueryBuilder) *QueryBuilder {
	// Limit very large queries for performance
	if qb.pagination.Limit > 1000 {
		return qb.Limit(1000)
	}
	return qb
}

func optimizeSorting(qb *QueryBuilder) *QueryBuilder {
	// Add cache preference for sorted queries
	return qb.WithReadPreference(ReadPreferenceSecondary)
}

// Example Usage Functions

// ExampleSimpleQuery demonstrates basic query building
func ExampleSimpleQuery(userID shared.UserID) ([]*node.Node, error) {
	// Simple query for recent nodes with specific keywords
	_ = NewQueryBuilder().
		ForUser(userID).
		WithKeywords("machine learning", "AI").
		CreatedAfter(time.Now().AddDate(0, 0, -30)). // Last 30 days
		ExcludeArchived().
		OrderByCreatedAt(Descending).
		Limit(20).
		Build()
	
	// This would be used with a repository that supports specifications
	// return repo.FindBySpecification(ctx, spec)
	return nil, nil // Placeholder
}

// ExampleComplexQuery demonstrates advanced query building
func ExampleComplexQuery(userID shared.UserID, searchTerm string) (Specification, error) {
	// Complex query combining multiple conditions
	validator := NewQueryValidator()
	optimizer := NewQueryOptimizer()
	
	// Build the query
	builder := NewQueryBuilder().
		ForUser(userID).
		Where(
			// Content search OR tag match
			NewContentContainsSpec(searchTerm, true).
			Or(NewTaggedWithSpec("important"))).
		Where(
			// Created in last 3 months AND not archived
			NewCreatedAfterSpec(time.Now().AddDate(0, -3, 0)).
			And(NewArchivedSpec(false))).
		OrderByRelevance().
		Limit(50).
		WithCache(5 * time.Minute)
	
	// Validate the query
	if err := validator.Validate(builder); err != nil {
		return nil, fmt.Errorf("query validation failed: %w", err)
	}
	
	// Optimize the query
	optimized := optimizer.Optimize(builder)
	
	return optimized.Build(), nil
}