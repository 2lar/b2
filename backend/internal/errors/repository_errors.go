// Package errors provides repository-specific error constructors.
package errors

import (
	"errors"
	"fmt"
	"time"
)

// Repository error variables for backward compatibility
var (
	ErrNodeNotFoundRepo     = NotFound(CodeNodeNotFound.String(), "Node not found").WithResource("node").Build()
	ErrEdgeNotFoundRepo     = NotFound(CodeEdgeNotFound.String(), "Edge not found").WithResource("edge").Build()
	ErrCategoryNotFoundRepo = NotFound(CodeCategoryNotFound.String(), "Category not found").WithResource("category").Build()
)

// RepositoryErrorCode represents repository-specific error codes
type RepositoryErrorCode string

const (
	ErrCodeNotFound          RepositoryErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeAlreadyExists     RepositoryErrorCode = "RESOURCE_ALREADY_EXISTS"
	ErrCodeConflict          RepositoryErrorCode = "RESOURCE_CONFLICT"
	ErrCodeInvalidInput      RepositoryErrorCode = "INVALID_INPUT"
	ErrCodeInvalidQuery      RepositoryErrorCode = "INVALID_QUERY"
	ErrCodeValidation        RepositoryErrorCode = "VALIDATION_ERROR"
	ErrCodeTransactionFailed RepositoryErrorCode = "TRANSACTION_FAILED"
	ErrCodeOperationFailed   RepositoryErrorCode = "OPERATION_FAILED"
	ErrCodeTimeout           RepositoryErrorCode = "TIMEOUT"
	ErrCodeRateLimited       RepositoryErrorCode = "RATE_LIMITED"
	ErrCodeOptimisticLock    RepositoryErrorCode = "OPTIMISTIC_LOCK_CONFLICT"
	ErrCodeDataCorruption    RepositoryErrorCode = "DATA_CORRUPTION"
	ErrCodeInconsistentState RepositoryErrorCode = "INCONSISTENT_STATE"
	ErrCodeUnavailable       RepositoryErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeInternalError     RepositoryErrorCode = "INTERNAL_ERROR"
	ErrCodeConnectionError   RepositoryErrorCode = "CONNECTION_ERROR"
	ErrCodeConnectionFailed  RepositoryErrorCode = "CONNECTION_FAILED"
	ErrCodeInvalidOperation  RepositoryErrorCode = "INVALID_OPERATION"
	ErrCodeEventPublishingFailed RepositoryErrorCode = "EVENT_PUBLISHING_FAILED"
	ErrCodeTransactionConflict   RepositoryErrorCode = "TRANSACTION_CONFLICT"
)

// NewRepositoryError creates a standardized repository error
func NewRepositoryError(code RepositoryErrorCode, message string, cause error) *UnifiedError {
	return NewError(ErrorTypeRepository, string(code), message).
		WithCause(cause).
		WithSeverity(SeverityHigh).
		WithRetryable(isRetryableRepoCode(code)).
		Build()
}

// NewRepositoryErrorWithDetails creates a repository error with details
func NewRepositoryErrorWithDetails(code RepositoryErrorCode, message string, details map[string]interface{}, cause error) *UnifiedError {
	err := NewError(ErrorTypeRepository, string(code), message).
		WithCause(cause).
		WithSeverity(SeverityHigh).
		WithRetryable(isRetryableRepoCode(code)).
		Build()
	err.RecoveryMetadata = details
	return err
}

// isRetryableRepoCode determines if an error code represents a retryable error
func isRetryableRepoCode(code RepositoryErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeRateLimited, ErrCodeUnavailable, ErrCodeConnectionError:
		return true
	default:
		return false
	}
}

// Repository error type structures for compatibility

// ErrNotFound represents a resource not found error
type ErrNotFound struct {
	Resource string
	ID       string
	UserID   string
}

func (e ErrNotFound) Error() string {
	if e.UserID != "" {
		return fmt.Sprintf("%s with ID '%s' not found for user '%s'", e.Resource, e.ID, e.UserID)
	}
	return fmt.Sprintf("%s with ID '%s' not found", e.Resource, e.ID)
}

// ErrConflict represents a conflict error
type ErrConflict struct {
	Resource string
	ID       string
	Reason   string
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("conflict with %s '%s': %s", e.Resource, e.ID, e.Reason)
}

// ErrInvalidQuery represents an invalid query error
type ErrInvalidQuery struct {
	Field  string
	Reason string
}

func (e ErrInvalidQuery) Error() string {
	return fmt.Sprintf("invalid query for field '%s': %s", e.Field, e.Reason)
}

// IsNotFoundRepo checks if an error is a repository not found error
func IsNotFoundRepo(err error) bool {
	_, ok := err.(ErrNotFound)
	if ok {
		return true
	}
	return IsNotFound(err)
}

// IsConflictRepo checks if an error is a repository conflict error
func IsConflictRepo(err error) bool {
	_, ok := err.(ErrConflict)
	if ok {
		return true
	}
	return IsConflict(err)
}

// IsInvalidQuery checks if an error is a repository invalid query error
func IsInvalidQuery(err error) bool {
	_, ok := err.(ErrInvalidQuery)
	return ok
}

// NewNotFoundRepo creates a new ErrNotFound
func NewNotFoundRepo(resource, id string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id}
}

// NewNotFoundWithUser creates a new ErrNotFound with user context
func NewNotFoundWithUser(resource, id, userID string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id, UserID: userID}
}

// NewConflictRepo creates a new ErrConflict
func NewConflictRepo(resource, id, reason string) ErrConflict {
	return ErrConflict{Resource: resource, ID: id, Reason: reason}
}

// NewInvalidQuery creates a new ErrInvalidQuery
func NewInvalidQuery(field, reason string) ErrInvalidQuery {
	return ErrInvalidQuery{Field: field, Reason: reason}
}

// Helper functions for creating common repository errors

// NewNotFoundError creates a standardized not found error
func NewNotFoundError(resource, id, userID string) *UnifiedError {
	message := fmt.Sprintf("%s with ID '%s' not found", resource, id)
	if userID != "" {
		message = fmt.Sprintf("%s with ID '%s' not found for user '%s'", resource, id, userID)
	}
	
	return NewRepositoryErrorWithDetails(ErrCodeNotFound, message, map[string]interface{}{
		"resource": resource,
		"id":       id,
		"user_id":  userID,
	}, nil)
}

// NewValidationError creates a standardized validation error
func NewValidationError(field, reason string, cause error) *UnifiedError {
	message := fmt.Sprintf("validation error for field '%s': %s", field, reason)
	
	return NewRepositoryErrorWithDetails(ErrCodeValidation, message, map[string]interface{}{
		"field":  field,
		"reason": reason,
	}, cause)
}

// NewOptimisticLockError creates a standardized optimistic lock error
func NewOptimisticLockError(resourceID string, expectedVersion, actualVersion int) *UnifiedError {
	message := fmt.Sprintf("optimistic lock conflict for resource %s: expected version %d, actual version %d",
		resourceID, expectedVersion, actualVersion)
	
	err := NewRepositoryErrorWithDetails(ErrCodeOptimisticLock, message, map[string]interface{}{
		"resource_id":      resourceID,
		"expected_version": expectedVersion,
		"actual_version":   actualVersion,
	}, nil)
	err.Retryable = true
	return err
}

// NewTransactionError creates a standardized transaction error
func NewTransactionError(operation string, cause error) *UnifiedError {
	message := fmt.Sprintf("transaction failed for operation: %s", operation)
	
	return NewRepositoryErrorWithDetails(ErrCodeTransactionFailed, message, map[string]interface{}{
		"operation": operation,
	}, cause)
}

// NewTimeoutError creates a standardized timeout error
func NewTimeoutError(operation string, timeout time.Duration) *UnifiedError {
	message := fmt.Sprintf("operation '%s' timed out after %v", operation, timeout)
	
	return NewRepositoryErrorWithDetails(ErrCodeTimeout, message, map[string]interface{}{
		"operation": operation,
		"timeout":   timeout.String(),
	}, nil)
}

// NewRateLimitError creates a standardized rate limit error
func NewRateLimitError(operation string, retryAfter time.Duration) *UnifiedError {
	message := fmt.Sprintf("rate limit exceeded for operation: %s", operation)
	
	return NewRepositoryErrorWithDetails(ErrCodeRateLimited, message, map[string]interface{}{
		"operation":   operation,
		"retry_after": retryAfter.String(),
	}, nil)
}

// GetErrorCode extracts the error code from a repository error
func GetErrorCode(err error) RepositoryErrorCode {
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		return RepositoryErrorCode(unifiedErr.Code)
	}
	return ErrCodeInternalError
}