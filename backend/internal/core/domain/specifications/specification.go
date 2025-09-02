// Package specifications implements the Specification pattern for complex business rules.
// This pattern encapsulates business rules into reusable, composable objects that can
// be combined using logical operators (AND, OR, NOT) to create complex specifications.
//
// The Specification pattern is a key component of Domain-Driven Design that allows
// business rules to be expressed as first-class objects, making them testable,
// reusable, and composable. This implementation supports both in-memory evaluation
// and translation to SQL for efficient database queries.
package specifications

import (
	"context"
)

// Specification defines the interface for business rule specifications.
// It uses generics to work with any domain entity type.
type Specification[T any] interface {
	// IsSatisfiedBy checks if the specification is satisfied by the given candidate
	IsSatisfiedBy(ctx context.Context, candidate T) (bool, error)
	
	// And creates a composite specification with logical AND
	And(other Specification[T]) Specification[T]
	
	// Or creates a composite specification with logical OR
	Or(other Specification[T]) Specification[T]
	
	// Not creates a specification that negates this one
	Not() Specification[T]
	
	// GetSQL returns SQL WHERE clause and parameters for database queries
	// This allows specifications to be translated to efficient database queries
	GetSQL() (whereClause string, params []interface{})
	
	// GetDescription returns a human-readable description of the specification
	GetDescription() string
}

// BaseSpecification provides default implementations for specification composition.
// Embed this in concrete specifications to get AND, OR, NOT operations for free.
type BaseSpecification[T any] struct {
	// IsSatisfiedByFunc is the function that implements the specification logic
	IsSatisfiedByFunc func(context.Context, T) (bool, error)
	
	// GetSQLFunc returns SQL representation of the specification
	GetSQLFunc func() (string, []interface{})
	
	// Description is a human-readable description
	Description string
}

// IsSatisfiedBy delegates to the embedded function
func (s *BaseSpecification[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	if s.IsSatisfiedByFunc == nil {
		return false, nil
	}
	return s.IsSatisfiedByFunc(ctx, candidate)
}

// GetSQL returns the SQL representation
func (s *BaseSpecification[T]) GetSQL() (string, []interface{}) {
	if s.GetSQLFunc == nil {
		return "", nil
	}
	return s.GetSQLFunc()
}

// GetDescription returns the human-readable description
func (s *BaseSpecification[T]) GetDescription() string {
	return s.Description
}

// And creates a composite AND specification
func (s *BaseSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates a composite OR specification
func (s *BaseSpecification[T]) Or(other Specification[T]) Specification[T] {
	return &OrSpecification[T]{
		left:  s,
		right: other,
	}
}

// Not creates a NOT specification
func (s *BaseSpecification[T]) Not() Specification[T] {
	return &NotSpecification[T]{
		spec: s,
	}
}

// AndSpecification combines two specifications with logical AND
type AndSpecification[T any] struct {
	left  Specification[T]
	right Specification[T]
}

// IsSatisfiedBy returns true if both specifications are satisfied
func (s *AndSpecification[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	leftSatisfied, err := s.left.IsSatisfiedBy(ctx, candidate)
	if err != nil || !leftSatisfied {
		return false, err
	}
	
	return s.right.IsSatisfiedBy(ctx, candidate)
}

// GetSQL combines SQL from both specifications with AND
func (s *AndSpecification[T]) GetSQL() (string, []interface{}) {
	leftSQL, leftParams := s.left.GetSQL()
	rightSQL, rightParams := s.right.GetSQL()
	
	if leftSQL == "" {
		return rightSQL, rightParams
	}
	if rightSQL == "" {
		return leftSQL, leftParams
	}
	
	sql := "(" + leftSQL + ") AND (" + rightSQL + ")"
	params := append(leftParams, rightParams...)
	return sql, params
}

// GetDescription returns combined description
func (s *AndSpecification[T]) GetDescription() string {
	return s.left.GetDescription() + " AND " + s.right.GetDescription()
}

// And chains another AND specification
func (s *AndSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates an OR specification
func (s *AndSpecification[T]) Or(other Specification[T]) Specification[T] {
	return &OrSpecification[T]{
		left:  s,
		right: other,
	}
}

// Not creates a NOT specification
func (s *AndSpecification[T]) Not() Specification[T] {
	return &NotSpecification[T]{
		spec: s,
	}
}

// OrSpecification combines two specifications with logical OR
type OrSpecification[T any] struct {
	left  Specification[T]
	right Specification[T]
}

// IsSatisfiedBy returns true if either specification is satisfied
func (s *OrSpecification[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	leftSatisfied, err := s.left.IsSatisfiedBy(ctx, candidate)
	if err != nil {
		return false, err
	}
	if leftSatisfied {
		return true, nil
	}
	
	return s.right.IsSatisfiedBy(ctx, candidate)
}

// GetSQL combines SQL from both specifications with OR
func (s *OrSpecification[T]) GetSQL() (string, []interface{}) {
	leftSQL, leftParams := s.left.GetSQL()
	rightSQL, rightParams := s.right.GetSQL()
	
	if leftSQL == "" {
		return rightSQL, rightParams
	}
	if rightSQL == "" {
		return leftSQL, leftParams
	}
	
	sql := "(" + leftSQL + ") OR (" + rightSQL + ")"
	params := append(leftParams, rightParams...)
	return sql, params
}

// GetDescription returns combined description
func (s *OrSpecification[T]) GetDescription() string {
	return s.left.GetDescription() + " OR " + s.right.GetDescription()
}

// And creates an AND specification
func (s *OrSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or chains another OR specification
func (s *OrSpecification[T]) Or(other Specification[T]) Specification[T] {
	return &OrSpecification[T]{
		left:  s,
		right: other,
	}
}

// Not creates a NOT specification
func (s *OrSpecification[T]) Not() Specification[T] {
	return &NotSpecification[T]{
		spec: s,
	}
}

// NotSpecification negates a specification
type NotSpecification[T any] struct {
	spec Specification[T]
}

// IsSatisfiedBy returns true if the specification is NOT satisfied
func (s *NotSpecification[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	satisfied, err := s.spec.IsSatisfiedBy(ctx, candidate)
	if err != nil {
		return false, err
	}
	return !satisfied, nil
}

// GetSQL negates the SQL from the specification
func (s *NotSpecification[T]) GetSQL() (string, []interface{}) {
	sql, params := s.spec.GetSQL()
	if sql == "" {
		return "", nil
	}
	return "NOT (" + sql + ")", params
}

// GetDescription returns negated description
func (s *NotSpecification[T]) GetDescription() string {
	return "NOT " + s.spec.GetDescription()
}

// And creates an AND specification
func (s *NotSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates an OR specification
func (s *NotSpecification[T]) Or(other Specification[T]) Specification[T] {
	return &OrSpecification[T]{
		left:  s,
		right: other,
	}
}

// Not double-negates (returns the original specification)
func (s *NotSpecification[T]) Not() Specification[T] {
	return s.spec
}

// AlwaysTrue is a specification that always returns true
type AlwaysTrue[T any] struct{}

func (s *AlwaysTrue[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	return true, nil
}

func (s *AlwaysTrue[T]) GetSQL() (string, []interface{}) {
	return "1=1", nil
}

func (s *AlwaysTrue[T]) GetDescription() string {
	return "Always True"
}

func (s *AlwaysTrue[T]) And(other Specification[T]) Specification[T] {
	return other
}

func (s *AlwaysTrue[T]) Or(other Specification[T]) Specification[T] {
	return s
}

func (s *AlwaysTrue[T]) Not() Specification[T] {
	return &AlwaysFalse[T]{}
}

// AlwaysFalse is a specification that always returns false
type AlwaysFalse[T any] struct{}

func (s *AlwaysFalse[T]) IsSatisfiedBy(ctx context.Context, candidate T) (bool, error) {
	return false, nil
}

func (s *AlwaysFalse[T]) GetSQL() (string, []interface{}) {
	return "1=0", nil
}

func (s *AlwaysFalse[T]) GetDescription() string {
	return "Always False"
}

func (s *AlwaysFalse[T]) And(other Specification[T]) Specification[T] {
	return s
}

func (s *AlwaysFalse[T]) Or(other Specification[T]) Specification[T] {
	return other
}

func (s *AlwaysFalse[T]) Not() Specification[T] {
	return &AlwaysTrue[T]{}
}