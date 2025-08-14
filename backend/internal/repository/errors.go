package repository

import (
	"fmt"
	"time"
)

// ErrorCode represents standardized error codes for repository operations
type ErrorCode string

const (
	// Resource errors
	ErrCodeNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeAlreadyExists ErrorCode = "RESOURCE_ALREADY_EXISTS"
	ErrCodeConflict      ErrorCode = "RESOURCE_CONFLICT"

	// Validation errors
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeInvalidQuery ErrorCode = "INVALID_QUERY"
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"

	// Operation errors
	ErrCodeTransactionFailed ErrorCode = "TRANSACTION_FAILED"
	ErrCodeOperationFailed   ErrorCode = "OPERATION_FAILED"
	ErrCodeTimeout           ErrorCode = "TIMEOUT"
	ErrCodeRateLimited       ErrorCode = "RATE_LIMITED"

	// Consistency errors
	ErrCodeOptimisticLock    ErrorCode = "OPTIMISTIC_LOCK_CONFLICT"
	ErrCodeDataCorruption    ErrorCode = "DATA_CORRUPTION"
	ErrCodeInconsistentState ErrorCode = "INCONSISTENT_STATE"

	// Infrastructure errors
	ErrCodeUnavailable       ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeInternalError     ErrorCode = "INTERNAL_ERROR"
	ErrCodeConnectionError   ErrorCode = "CONNECTION_ERROR"
	ErrCodeConnectionFailed  ErrorCode = "CONNECTION_FAILED"
	
	// Unit of Work specific errors
	ErrCodeInvalidOperation         ErrorCode = "INVALID_OPERATION"
	ErrCodeEventPublishingFailed    ErrorCode = "EVENT_PUBLISHING_FAILED"
	ErrCodeTransactionConflict      ErrorCode = "TRANSACTION_CONFLICT"
)

// RepositoryError represents a standardized repository error
type RepositoryError struct {
	Code      ErrorCode              // Standardized error code
	Message   string                 // Human-readable error message
	Details   map[string]interface{} // Additional error details
	Cause     error                  // Underlying error that caused this
	Timestamp time.Time              // When the error occurred
	Retryable bool                   // Whether the operation can be retried
}

func (e RepositoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e RepositoryError) Unwrap() error {
	return e.Cause
}

// IsRepositoryError checks if an error is a repository error
func IsRepositoryError(err error) bool {
	_, ok := err.(RepositoryError)
	return ok
}

// GetErrorCode extracts the error code from a repository error
func GetErrorCode(err error) ErrorCode {
	if repoErr, ok := err.(RepositoryError); ok {
		return repoErr.Code
	}
	return ErrCodeInternalError
}

// ErrNotFound represents a resource not found error in the repository layer.
type ErrNotFound struct {
	Resource string // The type of resource (e.g., "node", "edge")
	ID       string // The identifier that was not found
	UserID   string // The user context, if applicable
}

func (e ErrNotFound) Error() string {
	if e.UserID != "" {
		return fmt.Sprintf("%s with ID '%s' not found for user '%s'", e.Resource, e.ID, e.UserID)
	}
	return fmt.Sprintf("%s with ID '%s' not found", e.Resource, e.ID)
}

// IsNotFound checks if an error is a repository not found error.
func IsNotFound(err error) bool {
	_, ok := err.(ErrNotFound)
	if ok {
		return true
	}

	// Check for repository error with not found code
	if repoErr, ok := err.(RepositoryError); ok {
		return repoErr.Code == ErrCodeNotFound
	}

	return false
}

// ErrConflict represents a conflict error in the repository layer.
type ErrConflict struct {
	Resource string // The type of resource (e.g., "node", "edge")
	ID       string // The identifier that caused the conflict
	Reason   string // The reason for the conflict
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("conflict with %s '%s': %s", e.Resource, e.ID, e.Reason)
}

// IsConflict checks if an error is a repository conflict error.
func IsConflict(err error) bool {
	_, ok := err.(ErrConflict)
	if ok {
		return true
	}

	// Check for repository error with conflict code
	if repoErr, ok := err.(RepositoryError); ok {
		return repoErr.Code == ErrCodeConflict || repoErr.Code == ErrCodeOptimisticLock
	}

	return false
}

// ErrInvalidQuery represents an invalid query error in the repository layer.
type ErrInvalidQuery struct {
	Field  string // The field that caused the invalid query
	Reason string // The reason why the query is invalid
}

func (e ErrInvalidQuery) Error() string {
	return fmt.Sprintf("invalid query for field '%s': %s", e.Field, e.Reason)
}

// IsInvalidQuery checks if an error is a repository invalid query error.
func IsInvalidQuery(err error) bool {
	_, ok := err.(ErrInvalidQuery)
	if ok {
		return true
	}

	// Check for repository error with invalid query code
	if repoErr, ok := err.(RepositoryError); ok {
		return repoErr.Code == ErrCodeInvalidQuery || repoErr.Code == ErrCodeValidation
	}

	return false
}

// NewRepositoryError creates a new repository error
func NewRepositoryError(code ErrorCode, message string, cause error) RepositoryError {
	return RepositoryError{
		Code:      code,
		Message:   message,
		Details:   make(map[string]interface{}),
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorCode(code),
	}
}

// NewRepositoryErrorWithDetails creates a new repository error with details
func NewRepositoryErrorWithDetails(code ErrorCode, message string, details map[string]interface{}, cause error) RepositoryError {
	return RepositoryError{
		Code:      code,
		Message:   message,
		Details:   details,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorCode(code),
	}
}

// isRetryableErrorCode determines if an error code represents a retryable error
func isRetryableErrorCode(code ErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeRateLimited, ErrCodeUnavailable, ErrCodeConnectionError:
		return true
	default:
		return false
	}
}

// NewNotFound creates a new ErrNotFound.
func NewNotFound(resource, id string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id}
}

// NewNotFoundWithUser creates a new ErrNotFound with user context.
func NewNotFoundWithUser(resource, id, userID string) ErrNotFound {
	return ErrNotFound{Resource: resource, ID: id, UserID: userID}
}

// NewConflict creates a new ErrConflict.
func NewConflict(resource, id, reason string) ErrConflict {
	return ErrConflict{Resource: resource, ID: id, Reason: reason}
}

// NewInvalidQuery creates a new ErrInvalidQuery.
func NewInvalidQuery(field, reason string) ErrInvalidQuery {
	return ErrInvalidQuery{Field: field, Reason: reason}
}

// Helper functions for creating common repository errors

// NewNotFoundError creates a standardized not found error
func NewNotFoundError(resource, id, userID string) RepositoryError {
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
func NewValidationError(field, reason string, cause error) RepositoryError {
	message := fmt.Sprintf("validation error for field '%s': %s", field, reason)

	return NewRepositoryErrorWithDetails(ErrCodeValidation, message, map[string]interface{}{
		"field":  field,
		"reason": reason,
	}, cause)
}

// NewOptimisticLockError creates a standardized optimistic lock error
func NewOptimisticLockError(resourceID string, expectedVersion, actualVersion int) RepositoryError {
	message := fmt.Sprintf("optimistic lock conflict for resource %s: expected version %d, actual version %d",
		resourceID, expectedVersion, actualVersion)

	return NewRepositoryErrorWithDetails(ErrCodeOptimisticLock, message, map[string]interface{}{
		"resource_id":      resourceID,
		"expected_version": expectedVersion,
		"actual_version":   actualVersion,
	}, nil)
}

// NewTransactionError creates a standardized transaction error
func NewTransactionError(operation string, cause error) RepositoryError {
	message := fmt.Sprintf("transaction failed for operation: %s", operation)

	return NewRepositoryErrorWithDetails(ErrCodeTransactionFailed, message, map[string]interface{}{
		"operation": operation,
	}, cause)
}

// NewTimeoutError creates a standardized timeout error
func NewTimeoutError(operation string, timeout time.Duration) RepositoryError {
	message := fmt.Sprintf("operation '%s' timed out after %v", operation, timeout)

	return NewRepositoryErrorWithDetails(ErrCodeTimeout, message, map[string]interface{}{
		"operation": operation,
		"timeout":   timeout.String(),
	}, nil)
}

// NewRateLimitError creates a standardized rate limit error
func NewRateLimitError(operation string, retryAfter time.Duration) RepositoryError {
	message := fmt.Sprintf("rate limit exceeded for operation: %s", operation)

	return NewRepositoryErrorWithDetails(ErrCodeRateLimited, message, map[string]interface{}{
		"operation":   operation,
		"retry_after": retryAfter.String(),
	}, nil)
}
