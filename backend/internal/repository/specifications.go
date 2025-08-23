package repository

import (
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
)

// Specification defines criteria for querying entities.
//
// Key Concepts Illustrated:
//   1. Specification Pattern: Encapsulates business rules as objects
//   2. Composability: Specifications can be combined using logical operators
//   3. Testability: Business rules can be tested in isolation
//   4. Reusability: Common criteria can be reused across different queries
//   5. Domain Alignment: Specifications express domain concepts
//
// This implementation follows Eric Evans' Domain-Driven Design principles
// and Martin Fowler's specification pattern guidelines.
//
// Example Usage:
//   // Create individual specifications
//   userSpec := NewUserOwnedSpec(userID)
//   keywordSpec := NewKeywordContainsSpec("machine learning")
//   recentSpec := NewCreatedAfterSpec(time.Now().Add(-7*24*time.Hour))
//   
//   // Compose specifications
//   complexSpec := userSpec.And(keywordSpec.Or(recentSpec))
//   
//   // Use in repository
//   nodes, err := repo.FindBySpecification(ctx, complexSpec)
type Specification interface {
	// IsSatisfiedBy tests if an entity meets this specification's criteria
	// This enables in-memory filtering and testing
	IsSatisfiedBy(entity interface{}) bool
	
	// ToFilter converts the specification to repository-specific filter criteria
	// This enables database-level filtering for performance
	ToFilter() Filter
	
	// Logical composition methods for building complex specifications
	And(spec Specification) Specification
	Or(spec Specification) Specification
	Not() Specification
	
	// Metadata for debugging and optimization
	Description() string
	EstimatedSelectivity() float64 // 0.0 (very selective) to 1.0 (not selective)
}

// Filter represents database-specific filter criteria
// This abstraction allows specifications to work with different storage backends
type Filter struct {
	// Field-based filtering
	FieldFilters []FieldFilter `json:"field_filters,omitempty"`
	
	// Text search
	TextSearch *TextSearchFilter `json:"text_search,omitempty"`
	
	// Range filtering  
	RangeFilter *RangeFilter `json:"range_filter,omitempty"`
	
	// Logical composition
	LogicalOperator LogicalOperator `json:"logical_operator,omitempty"`
	SubFilters      []Filter        `json:"sub_filters,omitempty"`
}

// FieldFilter represents filtering on a specific field
type FieldFilter struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// TextSearchFilter represents text-based searching
type TextSearchFilter struct {
	Fields   []string `json:"fields"`
	Query    string   `json:"query"`
	Fuzzy    bool     `json:"fuzzy"`
	Boosting map[string]float64 `json:"boosting,omitempty"` // Field boosting for relevance
}

// RangeFilter represents range-based filtering
type RangeFilter struct {
	Field string      `json:"field"`
	Min   interface{} `json:"min,omitempty"`
	Max   interface{} `json:"max,omitempty"`
}

// Operator represents comparison operators
type Operator string

const (
	OperatorEquals          Operator = "eq"
	OperatorNotEquals       Operator = "ne"
	OperatorGreaterThan     Operator = "gt"
	OperatorGreaterOrEqual  Operator = "gte"
	OperatorLessThan        Operator = "lt"
	OperatorLessOrEqual     Operator = "lte"
	OperatorContains        Operator = "contains"
	OperatorStartsWith      Operator = "starts_with"
	OperatorEndsWith        Operator = "ends_with"
	OperatorIn              Operator = "in"
	OperatorNotIn           Operator = "not_in"
	OperatorIsNull          Operator = "is_null"
	OperatorIsNotNull       Operator = "is_not_null"
	OperatorMatches         Operator = "matches" // Regex matching
)

// LogicalOperator represents logical composition operators
type LogicalOperator string

const (
	LogicalOperatorAnd LogicalOperator = "and"
	LogicalOperatorOr  LogicalOperator = "or"
	LogicalOperatorNot LogicalOperator = "not"
)

// Base specification implementation
// This provides common functionality for all specifications
type baseSpecification struct {
	description           string
	estimatedSelectivity  float64
}

func (s baseSpecification) Description() string {
	return s.description
}

func (s baseSpecification) EstimatedSelectivity() float64 {
	return s.estimatedSelectivity
}

// Composite specifications for logical operations
type andSpecification struct {
	baseSpecification
	left  Specification
	right Specification
}

func (s andSpecification) IsSatisfiedBy(entity interface{}) bool {
	return s.left.IsSatisfiedBy(entity) && s.right.IsSatisfiedBy(entity)
}

func (s andSpecification) ToFilter() Filter {
	return Filter{
		LogicalOperator: LogicalOperatorAnd,
		SubFilters:      []Filter{s.left.ToFilter(), s.right.ToFilter()},
	}
}

func (s andSpecification) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s andSpecification) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s andSpecification) Not() Specification {
	return newNotSpecification(s)
}

type orSpecification struct {
	baseSpecification
	left  Specification
	right Specification
}

func (s orSpecification) IsSatisfiedBy(entity interface{}) bool {
	return s.left.IsSatisfiedBy(entity) || s.right.IsSatisfiedBy(entity)
}

func (s orSpecification) ToFilter() Filter {
	return Filter{
		LogicalOperator: LogicalOperatorOr,
		SubFilters:      []Filter{s.left.ToFilter(), s.right.ToFilter()},
	}
}

func (s orSpecification) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s orSpecification) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s orSpecification) Not() Specification {
	return newNotSpecification(s)
}

type notSpecification struct {
	baseSpecification
	inner Specification
}

func (s notSpecification) IsSatisfiedBy(entity interface{}) bool {
	return !s.inner.IsSatisfiedBy(entity)
}

func (s notSpecification) ToFilter() Filter {
	return Filter{
		LogicalOperator: LogicalOperatorNot,
		SubFilters:      []Filter{s.inner.ToFilter()},
	}
}

func (s notSpecification) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s notSpecification) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s notSpecification) Not() Specification {
	return s.inner // Double negation elimination
}

// Factory functions for composite specifications
func newAndSpecification(left, right Specification) Specification {
	// Calculate combined selectivity (intersection is more selective)
	selectivity := left.EstimatedSelectivity() * right.EstimatedSelectivity()
	
	return andSpecification{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("(%s AND %s)", left.Description(), right.Description()),
			estimatedSelectivity: selectivity,
		},
		left:  left,
		right: right,
	}
}

func newOrSpecification(left, right Specification) Specification {
	// Calculate combined selectivity (union is less selective)
	selectivity := left.EstimatedSelectivity() + right.EstimatedSelectivity() - 
		(left.EstimatedSelectivity() * right.EstimatedSelectivity())
	
	return orSpecification{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("(%s OR %s)", left.Description(), right.Description()),
			estimatedSelectivity: selectivity,
		},
		left:  left,
		right: right,
	}
}

func newNotSpecification(inner Specification) Specification {
	// Inverted selectivity
	selectivity := 1.0 - inner.EstimatedSelectivity()
	
	return notSpecification{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("NOT (%s)", inner.Description()),
			estimatedSelectivity: selectivity,
		},
		inner: inner,
	}
}

// Domain-specific specifications for Node entities
// These demonstrate how to create business-meaningful specifications

// UserOwnedSpec ensures entities belong to a specific user
type UserOwnedSpec struct {
	baseSpecification
	userID shared.UserID
}

// NewUserOwnedSpec creates a specification for user-owned entities
func NewUserOwnedSpec(userID shared.UserID) Specification {
	return UserOwnedSpec{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("owned by user %s", userID.String()),
			estimatedSelectivity: 0.01, // Assuming 1% of data per user on average
		},
		userID: userID,
	}
}

func (s UserOwnedSpec) IsSatisfiedBy(entity interface{}) bool {
	switch e := entity.(type) {
	case *node.Node:
		return e.UserID().Equals(s.userID)
	case *edge.Edge:
		return e.UserID().Equals(s.userID)
	case *category.Category:
		return e.UserID == s.userID.String()
	default:
		return false
	}
}

func (s UserOwnedSpec) ToFilter() Filter {
	return Filter{
		FieldFilters: []FieldFilter{
			{
				Field:    "user_id",
				Operator: OperatorEquals,
				Value:    s.userID.String(),
			},
		},
	}
}

func (s UserOwnedSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s UserOwnedSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s UserOwnedSpec) Not() Specification {
	return newNotSpecification(s)
}

// KeywordContainsSpec filters nodes that contain specific keywords
type KeywordContainsSpec struct {
	baseSpecification
	keyword string
}

// NewKeywordContainsSpec creates a specification for nodes containing a keyword
func NewKeywordContainsSpec(keyword string) Specification {
	return KeywordContainsSpec{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("contains keyword '%s'", keyword),
			estimatedSelectivity: 0.05, // Assuming 5% of nodes contain any specific keyword
		},
		keyword: strings.ToLower(keyword),
	}
}

func (s KeywordContainsSpec) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*node.Node); ok {
		return node.HasKeyword(s.keyword)
	}
	return false
}

func (s KeywordContainsSpec) ToFilter() Filter {
	return Filter{
		FieldFilters: []FieldFilter{
			{
				Field:    "keywords",
				Operator: OperatorContains,
				Value:    s.keyword,
			},
		},
	}
}

func (s KeywordContainsSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s KeywordContainsSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s KeywordContainsSpec) Not() Specification {
	return newNotSpecification(s)
}

// TaggedWithSpec filters entities that have specific tags
type TaggedWithSpec struct {
	baseSpecification
	tag string
}

// NewTaggedWithSpec creates a specification for entities with a specific tag
func NewTaggedWithSpec(tag string) Specification {
	return TaggedWithSpec{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("tagged with '%s'", tag),
			estimatedSelectivity: 0.10, // Assuming 10% of entities have any specific tag
		},
		tag: strings.ToLower(tag),
	}
}

func (s TaggedWithSpec) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*node.Node); ok {
		return node.HasTag(s.tag)
	}
	return false
}

func (s TaggedWithSpec) ToFilter() Filter {
	return Filter{
		FieldFilters: []FieldFilter{
			{
				Field:    "tags",
				Operator: OperatorContains,
				Value:    s.tag,
			},
		},
	}
}

func (s TaggedWithSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s TaggedWithSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s TaggedWithSpec) Not() Specification {
	return newNotSpecification(s)
}

// CreatedAfterSpec filters entities created after a specific time
type CreatedAfterSpec struct {
	baseSpecification
	afterTime time.Time
}

// NewCreatedAfterSpec creates a specification for recently created entities
func NewCreatedAfterSpec(afterTime time.Time) Specification {
	return CreatedAfterSpec{
		baseSpecification: baseSpecification{
			description:          fmt.Sprintf("created after %s", afterTime.Format("2006-01-02")),
			estimatedSelectivity: 0.20, // Assuming 20% of entities are recent
		},
		afterTime: afterTime,
	}
}

func (s CreatedAfterSpec) IsSatisfiedBy(entity interface{}) bool {
	switch e := entity.(type) {
	case *node.Node:
		return e.CreatedAt().After(s.afterTime)
	case *edge.Edge:
		return e.CreatedAt.After(s.afterTime)
	case *category.Category:
		return e.CreatedAt.After(s.afterTime)
	default:
		return false
	}
}

func (s CreatedAfterSpec) ToFilter() Filter {
	return Filter{
		RangeFilter: &RangeFilter{
			Field: "created_at",
			Min:   s.afterTime,
		},
	}
}

func (s CreatedAfterSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s CreatedAfterSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s CreatedAfterSpec) Not() Specification {
	return newNotSpecification(s)
}

// ContentContainsSpec filters nodes with specific content
type ContentContainsSpec struct {
	baseSpecification
	searchTerm string
	fuzzy      bool
}

// NewContentContainsSpec creates a specification for content-based search
func NewContentContainsSpec(searchTerm string, fuzzy bool) Specification {
	description := fmt.Sprintf("content contains '%s'", searchTerm)
	if fuzzy {
		description = fmt.Sprintf("content fuzzy matches '%s'", searchTerm)
	}
	
	return ContentContainsSpec{
		baseSpecification: baseSpecification{
			description:          description,
			estimatedSelectivity: 0.15, // Assuming 15% of content matches typical search terms
		},
		searchTerm: searchTerm,
		fuzzy:      fuzzy,
	}
}

func (s ContentContainsSpec) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*node.Node); ok {
		content := strings.ToLower(node.Content().String())
		searchTerm := strings.ToLower(s.searchTerm)
		
		if s.fuzzy {
			// Simple fuzzy matching - in practice, use a proper fuzzy matching library
			return strings.Contains(content, searchTerm) || 
				   levenshteinDistance(content, searchTerm) <= 3
		}
		return strings.Contains(content, searchTerm)
	}
	return false
}

func (s ContentContainsSpec) ToFilter() Filter {
	return Filter{
		TextSearch: &TextSearchFilter{
			Fields: []string{"content"},
			Query:  s.searchTerm,
			Fuzzy:  s.fuzzy,
		},
	}
}

func (s ContentContainsSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s ContentContainsSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s ContentContainsSpec) Not() Specification {
	return newNotSpecification(s)
}

// ArchivedSpec filters archived entities
type ArchivedSpec struct {
	baseSpecification
	archived bool
}

// NewArchivedSpec creates a specification for archived/non-archived entities
func NewArchivedSpec(archived bool) Specification {
	description := "not archived"
	selectivity := 0.95 // Assuming 95% of entities are not archived
	if archived {
		description = "archived"
		selectivity = 0.05 // Assuming 5% of entities are archived
	}
	
	return ArchivedSpec{
		baseSpecification: baseSpecification{
			description:          description,
			estimatedSelectivity: selectivity,
		},
		archived: archived,
	}
}

func (s ArchivedSpec) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*node.Node); ok {
		return node.IsArchived() == s.archived
	}
	return false
}

func (s ArchivedSpec) ToFilter() Filter {
	return Filter{
		FieldFilters: []FieldFilter{
			{
				Field:    "archived",
				Operator: OperatorEquals,
				Value:    s.archived,
			},
		},
	}
}

func (s ArchivedSpec) And(spec Specification) Specification {
	return newAndSpecification(s, spec)
}

func (s ArchivedSpec) Or(spec Specification) Specification {
	return newOrSpecification(s, spec)
}

func (s ArchivedSpec) Not() Specification {
	return newNotSpecification(s)
}

// Utility functions

// levenshteinDistance calculates the Levenshtein distance between two strings
// This is a simplified implementation for demonstration purposes
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			
			matrix[i][j] = min(
				matrix[i-1][j]+1,    // deletion
				matrix[i][j-1]+1,    // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// SpecificationBuilder provides a fluent API for building complex specifications
// This demonstrates the Builder pattern for complex object construction
type SpecificationBuilder struct {
	spec Specification
}

// NewSpecificationBuilder creates a new specification builder
func NewSpecificationBuilder() *SpecificationBuilder {
	return &SpecificationBuilder{}
}

// ForUser sets the user ownership requirement
func (b *SpecificationBuilder) ForUser(userID shared.UserID) *SpecificationBuilder {
	userSpec := NewUserOwnedSpec(userID)
	if b.spec == nil {
		b.spec = userSpec
	} else {
		b.spec = b.spec.And(userSpec)
	}
	return b
}

// WithKeyword adds a keyword requirement
func (b *SpecificationBuilder) WithKeyword(keyword string) *SpecificationBuilder {
	keywordSpec := NewKeywordContainsSpec(keyword)
	if b.spec == nil {
		b.spec = keywordSpec
	} else {
		b.spec = b.spec.And(keywordSpec)
	}
	return b
}

// WithTag adds a tag requirement
func (b *SpecificationBuilder) WithTag(tag string) *SpecificationBuilder {
	tagSpec := NewTaggedWithSpec(tag)
	if b.spec == nil {
		b.spec = tagSpec
	} else {
		b.spec = b.spec.And(tagSpec)
	}
	return b
}

// CreatedAfter adds a creation time requirement
func (b *SpecificationBuilder) CreatedAfter(afterTime time.Time) *SpecificationBuilder {
	timeSpec := NewCreatedAfterSpec(afterTime)
	if b.spec == nil {
		b.spec = timeSpec
	} else {
		b.spec = b.spec.And(timeSpec)
	}
	return b
}

// WithContent adds a content search requirement
func (b *SpecificationBuilder) WithContent(searchTerm string, fuzzy bool) *SpecificationBuilder {
	contentSpec := NewContentContainsSpec(searchTerm, fuzzy)
	if b.spec == nil {
		b.spec = contentSpec
	} else {
		b.spec = b.spec.And(contentSpec)
	}
	return b
}

// OnlyArchived filters for archived entities
func (b *SpecificationBuilder) OnlyArchived() *SpecificationBuilder {
	archivedSpec := NewArchivedSpec(true)
	if b.spec == nil {
		b.spec = archivedSpec
	} else {
		b.spec = b.spec.And(archivedSpec)
	}
	return b
}

// ExcludeArchived filters out archived entities
func (b *SpecificationBuilder) ExcludeArchived() *SpecificationBuilder {
	notArchivedSpec := NewArchivedSpec(false)
	if b.spec == nil {
		b.spec = notArchivedSpec
	} else {
		b.spec = b.spec.And(notArchivedSpec)
	}
	return b
}

// Build returns the constructed specification
func (b *SpecificationBuilder) Build() Specification {
	if b.spec == nil {
		// Return a specification that matches everything
		return &alwaysTrueSpec{}
	}
	return b.spec
}

// alwaysTrueSpec is a specification that always returns true
type alwaysTrueSpec struct {
	baseSpecification
}

func init() {
	// Initialize the always true specification
}

func (s *alwaysTrueSpec) IsSatisfiedBy(entity interface{}) bool {
	return true
}

func (s *alwaysTrueSpec) ToFilter() Filter {
	return Filter{} // Empty filter matches everything
}

func (s *alwaysTrueSpec) And(spec Specification) Specification {
	return spec // AND with true is just the other specification
}

func (s *alwaysTrueSpec) Or(spec Specification) Specification {
	return s // OR with true is always true
}

func (s *alwaysTrueSpec) Not() Specification {
	return &alwaysFalseSpec{}
}

// alwaysFalseSpec is a specification that always returns false
type alwaysFalseSpec struct {
	baseSpecification
}

func (s *alwaysFalseSpec) IsSatisfiedBy(entity interface{}) bool {
	return false
}

func (s *alwaysFalseSpec) ToFilter() Filter {
	// This filter should match nothing - implementation depends on storage backend
	return Filter{
		FieldFilters: []FieldFilter{
			{
				Field:    "_impossible",
				Operator: OperatorEquals,
				Value:    "never_matches",
			},
		},
	}
}

func (s *alwaysFalseSpec) And(spec Specification) Specification {
	return s // AND with false is always false
}

func (s *alwaysFalseSpec) Or(spec Specification) Specification {
	return spec // OR with false is just the other specification
}

func (s *alwaysFalseSpec) Not() Specification {
	return &alwaysTrueSpec{}
}