package specifications

// Specification is the base interface for all specifications
// It follows the Specification pattern for encapsulating business rules
type Specification[T any] interface {
	// IsSatisfiedBy checks if the specification is satisfied by the given object
	IsSatisfiedBy(candidate T) bool

	// And creates a composite specification with AND logic
	And(other Specification[T]) Specification[T]

	// Or creates a composite specification with OR logic
	Or(other Specification[T]) Specification[T]

	// Not creates a specification with NOT logic
	Not() Specification[T]
}

// BaseSpecification provides default implementations for specification operations
type BaseSpecification[T any] struct {
	evaluator func(T) bool
}

// NewBaseSpecification creates a new base specification with a custom evaluator
func NewBaseSpecification[T any](evaluator func(T) bool) *BaseSpecification[T] {
	return &BaseSpecification[T]{
		evaluator: evaluator,
	}
}

// IsSatisfiedBy checks if the specification is satisfied
func (s *BaseSpecification[T]) IsSatisfiedBy(candidate T) bool {
	return s.evaluator(candidate)
}

// And creates an AND composite specification
func (s *BaseSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates an OR composite specification
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

// AndSpecification represents an AND composite specification
type AndSpecification[T any] struct {
	left  Specification[T]
	right Specification[T]
}

// IsSatisfiedBy checks if both specifications are satisfied
func (s *AndSpecification[T]) IsSatisfiedBy(candidate T) bool {
	return s.left.IsSatisfiedBy(candidate) && s.right.IsSatisfiedBy(candidate)
}

// And creates a new AND composite specification
func (s *AndSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates a new OR composite specification
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

// OrSpecification represents an OR composite specification
type OrSpecification[T any] struct {
	left  Specification[T]
	right Specification[T]
}

// IsSatisfiedBy checks if at least one specification is satisfied
func (s *OrSpecification[T]) IsSatisfiedBy(candidate T) bool {
	return s.left.IsSatisfiedBy(candidate) || s.right.IsSatisfiedBy(candidate)
}

// And creates a new AND composite specification
func (s *OrSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates a new OR composite specification
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

// NotSpecification represents a NOT specification
type NotSpecification[T any] struct {
	spec Specification[T]
}

// IsSatisfiedBy checks if the specification is NOT satisfied
func (s *NotSpecification[T]) IsSatisfiedBy(candidate T) bool {
	return !s.spec.IsSatisfiedBy(candidate)
}

// And creates a new AND composite specification
func (s *NotSpecification[T]) And(other Specification[T]) Specification[T] {
	return &AndSpecification[T]{
		left:  s,
		right: other,
	}
}

// Or creates a new OR composite specification
func (s *NotSpecification[T]) Or(other Specification[T]) Specification[T] {
	return &OrSpecification[T]{
		left:  s,
		right: other,
	}
}

// Not creates a double NOT specification (which cancels out)
func (s *NotSpecification[T]) Not() Specification[T] {
	return s.spec
}