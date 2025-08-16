package domain

import (
	"errors"
	"fmt"
)

// Domain errors for business rule violations and validation failures

// Value object validation errors
var (
	// NodeID errors
	ErrInvalidNodeID = errors.New("invalid node ID: must be a valid UUID")
	
	// UserID errors
	ErrEmptyUserID    = errors.New("user ID cannot be empty")
	ErrUserIDTooLong  = errors.New("user ID exceeds maximum length")
	
	// Content errors
	ErrEmptyContent         = errors.New("content cannot be empty")
	ErrContentTooLong       = errors.New("content exceeds maximum length")
	ErrInappropriateContent = errors.New("content contains inappropriate material")
	
	// Node business rule errors
	ErrCannotUpdateArchivedNode   = errors.New("cannot update archived node")
	ErrCannotConnectToSelf        = errors.New("cannot connect node to itself")
	ErrCrossUserConnection        = errors.New("cannot connect nodes from different users")
	ErrCannotConnectArchivedNodes = errors.New("cannot connect archived nodes")
	ErrNodeNotFound               = errors.New("node not found")
	ErrNodeAlreadyExists          = errors.New("node already exists")
	
	// Edge business rule errors
	ErrEdgeAlreadyExists = errors.New("edge already exists")
	ErrEdgeNotFound      = errors.New("edge not found")
	ErrInvalidEdge       = errors.New("invalid edge: source and target must be different")
	
	// Category errors
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category already exists")
	ErrCircularReference     = errors.New("circular reference detected in category hierarchy")
	ErrInvalidCategoryLevel  = errors.New("invalid category level")
	ErrInvalidCategoryID     = errors.New("invalid category ID")
	
	// General domain errors
	ErrValidation        = errors.New("validation failed")
	ErrConflict          = errors.New("conflict detected")
	ErrUnauthorized      = errors.New("unauthorized operation")
	ErrNotFound          = errors.New("resource not found")
	ErrInternalError     = errors.New("internal error")
	ErrInvalidOperation  = errors.New("invalid operation")
)

// DomainError represents a structured domain error with context
type DomainError struct {
	Type    string
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %s)", e.Type, e.Message, e.Cause.Error())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// NewDomainError creates a new domain error with context
func NewDomainError(errorType, message string, cause error) *DomainError {
	return &DomainError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context information to the error
func (e *DomainError) WithContext(key string, value interface{}) *DomainError {
	e.Context[key] = value
	return e
}

// ValidationError represents validation rule violations
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// BusinessRuleError represents a business rule violation
type BusinessRuleError struct {
	Rule    string
	Message string
	Entity  string
	Context map[string]interface{}
}

// Error implements the error interface
func (e *BusinessRuleError) Error() string {
	return fmt.Sprintf("business rule '%s' violated for %s: %s", e.Rule, e.Entity, e.Message)
}

// NewBusinessRuleError creates a new business rule error
func NewBusinessRuleError(rule, entity, message string) *BusinessRuleError {
	return &BusinessRuleError{
		Rule:    rule,
		Entity:  entity,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the business rule error
func (e *BusinessRuleError) WithContext(key string, value interface{}) *BusinessRuleError {
	e.Context[key] = value
	return e
}

// ConflictError represents resource conflicts (optimistic locking, etc.)
type ConflictError struct {
	Resource    string
	Identifier  string
	Message     string
	ExpectedVersion int
	ActualVersion   int
}

// Error implements the error interface
func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict in %s '%s': %s (expected version %d, got %d)", 
		e.Resource, e.Identifier, e.Message, e.ExpectedVersion, e.ActualVersion)
}

// NewConflictError creates a new conflict error
func NewConflictError(resource, identifier, message string, expectedVersion, actualVersion int) *ConflictError {
	return &ConflictError{
		Resource:        resource,
		Identifier:      identifier,
		Message:         message,
		ExpectedVersion: expectedVersion,
		ActualVersion:   actualVersion,
	}
}

// Error type checking helpers

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr) || errors.Is(err, ErrValidation)
}

// IsBusinessRuleError checks if an error is a business rule error
func IsBusinessRuleError(err error) bool {
	var businessErr *BusinessRuleError
	return errors.As(err, &businessErr)
}

// IsConflictError checks if an error is a conflict error
func IsConflictError(err error) bool {
	var conflictErr *ConflictError
	return errors.As(err, &conflictErr) || errors.Is(err, ErrConflict)
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound) || 
		   errors.Is(err, ErrNodeNotFound) || 
		   errors.Is(err, ErrEdgeNotFound) || 
		   errors.Is(err, ErrCategoryNotFound)
}

// IsUnauthorizedError checks if an error is an unauthorized error
func IsUnauthorizedError(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}