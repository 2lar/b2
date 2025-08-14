// Package repository - Specification pattern implementation
//
// The Specification pattern encapsulates query logic in reusable, composable objects.
// This pattern demonstrates how to build complex queries by combining simple specifications
// using logical operators (AND, OR, NOT).
//
// Educational Goals:
//   - Show how to encapsulate business rules in query specifications
//   - Demonstrate composition of complex queries from simple parts
//   - Illustrate the Open/Closed principle (open for extension, closed for modification)
//   - Provide type-safe query building
//   - Enable reusable business logic across different contexts
package repository

import (
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain"
)

// Specification defines the interface for query specifications.
// Each specification encapsulates a particular business rule or query condition
// and can be combined with other specifications to build complex queries.
//
// Key Methods:
//   - IsSatisfiedBy: Checks if an entity satisfies the specification (in-memory evaluation)
//   - ToSQL: Converts the specification to SQL for database queries
//   - And/Or/Not: Logical composition methods for building complex specifications
type Specification interface {
	// IsSatisfiedBy checks if the given entity satisfies this specification
	// This method is used for in-memory filtering and validation
	IsSatisfiedBy(entity interface{}) bool
	
	// ToSQL converts the specification to SQL WHERE clause
	// Returns the SQL string and parameter values for prepared statements
	ToSQL() (string, []interface{})
	
	// Logical composition methods
	And(other Specification) Specification
	Or(other Specification) Specification
	Not() Specification
}

// BaseSpecification provides default implementations for logical operations
// This demonstrates the Template Method pattern where subclasses only need
// to implement IsSatisfiedBy and ToSQL methods
type BaseSpecification struct{}

// IsSatisfiedBy is abstract and should be overridden by concrete implementations
func (s BaseSpecification) IsSatisfiedBy(entity interface{}) bool {
	panic("IsSatisfiedBy must be implemented by concrete specification")
}

// ToSQL is abstract and should be overridden by concrete implementations
func (s BaseSpecification) ToSQL() (string, []interface{}) {
	panic("ToSQL must be implemented by concrete specification")
}

func (s BaseSpecification) And(other Specification) Specification {
	return &AndSpecification{left: s, right: other}
}

func (s BaseSpecification) Or(other Specification) Specification {
	return &OrSpecification{left: s, right: other}
}

func (s BaseSpecification) Not() Specification {
	return &NotSpecification{spec: s}
}

// Composite Specifications - These implement logical operations between specifications

// AndSpecification combines two specifications with AND logic
type AndSpecification struct {
	left  Specification
	right Specification
}

func (s *AndSpecification) IsSatisfiedBy(entity interface{}) bool {
	return s.left.IsSatisfiedBy(entity) && s.right.IsSatisfiedBy(entity)
}

func (s *AndSpecification) ToSQL() (string, []interface{}) {
	leftSQL, leftArgs := s.left.ToSQL()
	rightSQL, rightArgs := s.right.ToSQL()
	
	sql := fmt.Sprintf("(%s) AND (%s)", leftSQL, rightSQL)
	args := append(leftArgs, rightArgs...)
	
	return sql, args
}

func (s *AndSpecification) And(other Specification) Specification {
	return &AndSpecification{left: s, right: other}
}

func (s *AndSpecification) Or(other Specification) Specification {
	return &OrSpecification{left: s, right: other}
}

func (s *AndSpecification) Not() Specification {
	return &NotSpecification{spec: s}
}

// OrSpecification combines two specifications with OR logic
type OrSpecification struct {
	left  Specification
	right Specification
}

func (s *OrSpecification) IsSatisfiedBy(entity interface{}) bool {
	return s.left.IsSatisfiedBy(entity) || s.right.IsSatisfiedBy(entity)
}

func (s *OrSpecification) ToSQL() (string, []interface{}) {
	leftSQL, leftArgs := s.left.ToSQL()
	rightSQL, rightArgs := s.right.ToSQL()
	
	sql := fmt.Sprintf("(%s) OR (%s)", leftSQL, rightSQL)
	args := append(leftArgs, rightArgs...)
	
	return sql, args
}

func (s *OrSpecification) And(other Specification) Specification {
	return &AndSpecification{left: s, right: other}
}

func (s *OrSpecification) Or(other Specification) Specification {
	return &OrSpecification{left: s, right: other}
}

func (s *OrSpecification) Not() Specification {
	return &NotSpecification{spec: s}
}

// NotSpecification negates a specification
type NotSpecification struct {
	spec Specification
}

func (s *NotSpecification) IsSatisfiedBy(entity interface{}) bool {
	return !s.spec.IsSatisfiedBy(entity)
}

func (s *NotSpecification) ToSQL() (string, []interface{}) {
	sql, args := s.spec.ToSQL()
	return fmt.Sprintf("NOT (%s)", sql), args
}

func (s *NotSpecification) And(other Specification) Specification {
	return &AndSpecification{left: s, right: other}
}

func (s *NotSpecification) Or(other Specification) Specification {
	return &OrSpecification{left: s, right: other}
}

func (s *NotSpecification) Not() Specification {
	return s.spec // Double negation cancels out
}

// Domain-Specific Specifications for Nodes

// UserOwnedSpecification ensures nodes belong to a specific user
type UserOwnedSpecification struct {
	BaseSpecification
	UserID domain.UserID
}

func NewUserOwnedSpec(userID domain.UserID) *UserOwnedSpecification {
	return &UserOwnedSpecification{UserID: userID}
}

func (s *UserOwnedSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		return node.UserID().Equals(s.UserID)
	}
	return false
}

func (s *UserOwnedSpecification) ToSQL() (string, []interface{}) {
	return "user_id = ?", []interface{}{s.UserID.String()}
}

// ContentContainsSpecification finds nodes containing specific text
type ContentContainsSpecification struct {
	BaseSpecification
	SearchText string
}

func NewContentContainsSpec(text string) *ContentContainsSpecification {
	return &ContentContainsSpecification{SearchText: text}
}

func (s *ContentContainsSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		content := strings.ToLower(node.Content().String())
		searchText := strings.ToLower(s.SearchText)
		return strings.Contains(content, searchText)
	}
	return false
}

func (s *ContentContainsSpecification) ToSQL() (string, []interface{}) {
	return "LOWER(content) LIKE ?", []interface{}{"%" + strings.ToLower(s.SearchText) + "%"}
}

// HasTagSpecification finds nodes with a specific tag
type HasTagSpecification struct {
	BaseSpecification
	Tag string
}

func NewHasTagSpec(tag string) *HasTagSpecification {
	return &HasTagSpecification{Tag: tag}
}

func (s *HasTagSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		return node.Tags().Contains(s.Tag)
	}
	return false
}

func (s *HasTagSpecification) ToSQL() (string, []interface{}) {
	return "tags @> ?", []interface{}{fmt.Sprintf("[\"%s\"]", s.Tag)}
}

// CreatedAfterSpecification finds nodes created after a specific date
type CreatedAfterSpecification struct {
	BaseSpecification
	Date time.Time
}

func NewCreatedAfterSpec(date time.Time) *CreatedAfterSpecification {
	return &CreatedAfterSpecification{Date: date}
}

func (s *CreatedAfterSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		return node.CreatedAt().After(s.Date)
	}
	return false
}

func (s *CreatedAfterSpecification) ToSQL() (string, []interface{}) {
	return "created_at > ?", []interface{}{s.Date}
}

// CreatedBeforeSpecification finds nodes created before a specific date
type CreatedBeforeSpecification struct {
	BaseSpecification
	Date time.Time
}

func NewCreatedBeforeSpec(date time.Time) *CreatedBeforeSpecification {
	return &CreatedBeforeSpecification{Date: date}
}

func (s *CreatedBeforeSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		return node.CreatedAt().Before(s.Date)
	}
	return false
}

func (s *CreatedBeforeSpecification) ToSQL() (string, []interface{}) {
	return "created_at < ?", []interface{}{s.Date}
}

// UpdatedRecentlySpecification finds nodes updated within a time duration
type UpdatedRecentlySpecification struct {
	BaseSpecification
	Duration time.Duration
}

func NewUpdatedRecentlySpec(duration time.Duration) *UpdatedRecentlySpecification {
	return &UpdatedRecentlySpecification{Duration: duration}
}

func (s *UpdatedRecentlySpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		cutoff := time.Now().Add(-s.Duration)
		return node.UpdatedAt().After(cutoff)
	}
	return false
}

func (s *UpdatedRecentlySpecification) ToSQL() (string, []interface{}) {
	cutoff := time.Now().Add(-s.Duration)
	return "updated_at > ?", []interface{}{cutoff}
}

// ArchivedSpecification finds archived nodes
type ArchivedSpecification struct {
	BaseSpecification
	IsArchived bool
}

func NewArchivedSpec(archived bool) *ArchivedSpecification {
	return &ArchivedSpecification{IsArchived: archived}
}

func (s *ArchivedSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		return node.IsArchived() == s.IsArchived
	}
	return false
}

func (s *ArchivedSpecification) ToSQL() (string, []interface{}) {
	return "archived = ?", []interface{}{s.IsArchived}
}

// KeywordMatchSpecification finds nodes with specific keywords
type KeywordMatchSpecification struct {
	BaseSpecification
	Keywords []string
}

func NewKeywordMatchSpec(keywords []string) *KeywordMatchSpecification {
	return &KeywordMatchSpecification{Keywords: keywords}
}

func (s *KeywordMatchSpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		nodeKeywords := node.Keywords()
		for _, keyword := range s.Keywords {
			if nodeKeywords.Contains(keyword) {
				return true
			}
		}
	}
	return false
}

func (s *KeywordMatchSpecification) ToSQL() (string, []interface{}) {
	if len(s.Keywords) == 0 {
		return "1 = 1", []interface{}{}
	}
	
	// Build array overlap query for PostgreSQL
	placeholders := make([]string, len(s.Keywords))
	args := make([]interface{}, len(s.Keywords))
	
	for i, keyword := range s.Keywords {
		placeholders[i] = "?"
		args[i] = keyword
	}
	
	sql := fmt.Sprintf("keywords && ARRAY[%s]", strings.Join(placeholders, ","))
	return sql, args
}

// SimilaritySpecification finds nodes similar to a given node
type SimilaritySpecification struct {
	BaseSpecification
	ReferenceNode       *domain.Node
	MinimumSimilarity   float64
}

func NewSimilaritySpec(node *domain.Node, minSimilarity float64) *SimilaritySpecification {
	return &SimilaritySpecification{
		ReferenceNode:     node,
		MinimumSimilarity: minSimilarity,
	}
}

func (s *SimilaritySpecification) IsSatisfiedBy(entity interface{}) bool {
	if node, ok := entity.(*domain.Node); ok {
		if node.ID().Equals(s.ReferenceNode.ID()) {
			return false // Don't match self
		}
		similarity := s.ReferenceNode.CalculateSimilarityTo(node)
		return similarity >= s.MinimumSimilarity
	}
	return false
}

func (s *SimilaritySpecification) ToSQL() (string, []interface{}) {
	// This would implement similarity calculation in SQL
	// For simplicity, we'll use keyword overlap
	referenceKeywords := s.ReferenceNode.Keywords().ToSlice()
	
	if len(referenceKeywords) == 0 {
		return "1 = 0", []interface{}{} // No matches if no keywords
	}
	
	placeholders := make([]string, len(referenceKeywords))
	args := make([]interface{}, len(referenceKeywords)+2)
	
	for i, keyword := range referenceKeywords {
		placeholders[i] = "?"
		args[i] = keyword
	}
	
	args[len(referenceKeywords)] = s.ReferenceNode.ID().String()
	args[len(referenceKeywords)+1] = s.MinimumSimilarity
	
	sql := fmt.Sprintf(`
		(
			SELECT COUNT(*) 
			FROM unnest(keywords) AS keyword 
			WHERE keyword = ANY(ARRAY[%s])
		) >= ? 
		AND id != ?`, strings.Join(placeholders, ","))
	
	return sql, args
}

// Specification Builder - Provides a fluent interface for building specifications

// SpecificationBuilder provides a fluent API for constructing complex specifications
type SpecificationBuilder struct {
	spec Specification
}

// NewSpecificationBuilder creates a new builder starting with a base specification
func NewSpecificationBuilder(baseSpec Specification) *SpecificationBuilder {
	return &SpecificationBuilder{spec: baseSpec}
}

// And adds an AND condition
func (b *SpecificationBuilder) And(spec Specification) *SpecificationBuilder {
	b.spec = b.spec.And(spec)
	return b
}

// Or adds an OR condition  
func (b *SpecificationBuilder) Or(spec Specification) *SpecificationBuilder {
	b.spec = b.spec.Or(spec)
	return b
}

// Not negates the current specification
func (b *SpecificationBuilder) Not() *SpecificationBuilder {
	b.spec = b.spec.Not()
	return b
}

// Build returns the constructed specification
func (b *SpecificationBuilder) Build() Specification {
	return b.spec
}

// Common specification combinations

// ActiveUserNodesSpec creates a specification for active (non-archived) nodes of a user
func ActiveUserNodesSpec(userID domain.UserID) Specification {
	return NewSpecificationBuilder(NewUserOwnedSpec(userID)).
		And(NewArchivedSpec(false)).
		Build()
}

// RecentUserNodesSpec creates a specification for recently updated nodes of a user
func RecentUserNodesSpec(userID domain.UserID, duration time.Duration) Specification {
	return NewSpecificationBuilder(NewUserOwnedSpec(userID)).
		And(NewUpdatedRecentlySpec(duration)).
		And(NewArchivedSpec(false)).
		Build()
}

// SearchUserNodesSpec creates a specification for searching user's nodes by content and tags
func SearchUserNodesSpec(userID domain.UserID, searchText string, tags []string) Specification {
	builder := NewSpecificationBuilder(NewUserOwnedSpec(userID)).
		And(NewArchivedSpec(false))
	
	if searchText != "" {
		builder = builder.And(NewContentContainsSpec(searchText))
	}
	
	for _, tag := range tags {
		builder = builder.And(NewHasTagSpec(tag))
	}
	
	return builder.Build()
}

// Example usage showing how specifications can be composed:
//
// Find all active nodes for a user that contain "important" and have tag "work":
// spec := NewSpecificationBuilder(NewUserOwnedSpec(userID)).
//         And(NewArchivedSpec(false)).
//         And(NewContentContainsSpec("important")).
//         And(NewHasTagSpec("work")).
//         Build()
//
// Find nodes created in the last week OR updated recently:
// recentCreated := NewCreatedAfterSpec(time.Now().AddDate(0, 0, -7))
// recentUpdated := NewUpdatedRecentlySpec(24 * time.Hour)
// spec := recentCreated.Or(recentUpdated)
//
// This demonstrates the power and flexibility of the Specification pattern!