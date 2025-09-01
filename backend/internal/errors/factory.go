package errors

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

// ErrorFactory provides convenient methods for creating common error types
// This reduces code duplication and ensures consistent error creation patterns
type ErrorFactory struct {
	service   string
	operation string
}

// NewErrorFactory creates a new error factory for a specific service
func NewErrorFactory(service string) *ErrorFactory {
	return &ErrorFactory{
		service: service,
	}
}

// ForOperation sets the current operation context
func (f *ErrorFactory) ForOperation(operation string) *ErrorFactory {
	return &ErrorFactory{
		service:   f.service,
		operation: operation,
	}
}

// Common Domain Errors

// NotFound creates a not found error with consistent formatting
func (f *ErrorFactory) NotFound(resourceType, resourceID string) *UnifiedError {
	return NotFound(
		fmt.Sprintf("%s_not_found", strings.ToLower(resourceType)),
		fmt.Sprintf("%s with ID %s not found", resourceType, resourceID),
	).
		WithOperation(f.operation).
		WithResource(fmt.Sprintf("%s:%s", resourceType, resourceID)).
		Build()
}

// AlreadyExists creates a conflict error for duplicate resources
func (f *ErrorFactory) AlreadyExists(resourceType, resourceID string) *UnifiedError {
	return Conflict(
		fmt.Sprintf("%s_already_exists", strings.ToLower(resourceType)),
		fmt.Sprintf("%s with ID %s already exists", resourceType, resourceID),
	).
		WithOperation(f.operation).
		WithResource(fmt.Sprintf("%s:%s", resourceType, resourceID)).
		Build()
}

// InvalidInput creates a validation error for invalid input
func (f *ErrorFactory) InvalidInput(field, reason string) *UnifiedError {
	return Validation(
		"invalid_input",
		fmt.Sprintf("Invalid %s: %s", field, reason),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("field: %s", field)).
		Build()
}

// RequiredField creates a validation error for missing required fields
func (f *ErrorFactory) RequiredField(field string) *UnifiedError {
	return Validation(
		"required_field",
		fmt.Sprintf("%s is required", field),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("field: %s", field)).
		Build()
}

// InvalidFormat creates a validation error for incorrectly formatted data
func (f *ErrorFactory) InvalidFormat(field, expectedFormat string) *UnifiedError {
	return Validation(
		"invalid_format",
		fmt.Sprintf("%s must be in format: %s", field, expectedFormat),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("field: %s, expected format: %s", field, expectedFormat)).
		Build()
}

// Repository Errors

// DatabaseError creates an internal error for database operations
func (f *ErrorFactory) DatabaseError(operation string, err error) *UnifiedError {
	return Internal(
		"database_error",
		fmt.Sprintf("Database %s failed", operation),
	).
		WithCause(err).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("db_operation: %s", operation)).
		Build()
}

// TransactionFailed creates an error for failed transactions
func (f *ErrorFactory) TransactionFailed(reason string, err error) *UnifiedError {
	return Internal(
		"transaction_failed",
		fmt.Sprintf("Transaction failed: %s", reason),
	).
		WithCause(err).
		WithOperation(f.operation).
		Build()
}

// ConcurrencyConflict creates an error for optimistic locking failures
func (f *ErrorFactory) ConcurrencyConflict(resourceType, resourceID string, expectedVersion, actualVersion int) *UnifiedError {
	return Conflict(
		"concurrency_conflict",
		fmt.Sprintf("%s has been modified by another process", resourceType),
	).
		WithOperation(f.operation).
		WithResource(fmt.Sprintf("%s:%s", resourceType, resourceID)).
		WithDetails(fmt.Sprintf("expected_version: %d, actual_version: %d", expectedVersion, actualVersion)).
		Build()
}

// Service Layer Errors

// BusinessRuleViolation creates an error for business rule violations
func (f *ErrorFactory) BusinessRuleViolation(rule, reason string) *UnifiedError {
	return Validation(
		"business_rule_violation",
		fmt.Sprintf("Business rule '%s' violated: %s", rule, reason),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("rule: %s", rule)).
		Build()
}

// PermissionDenied creates an authorization error
func (f *ErrorFactory) PermissionDenied(userID, resource, action string) *UnifiedError {
	return Unauthorized(
		"permission_denied",
		fmt.Sprintf("User %s does not have permission to %s %s", userID, action, resource),
	).
		WithOperation(f.operation).
		WithUserID(userID).
		WithResource(resource).
		WithDetails(fmt.Sprintf("action: %s", action)).
		Build()
}

// QuotaExceeded creates an error for quota/limit violations
func (f *ErrorFactory) QuotaExceeded(resource string, limit, current int) *UnifiedError {
	return Validation(
		"quota_exceeded",
		fmt.Sprintf("Quota exceeded for %s: limit %d, current %d", resource, limit, current),
	).
		WithOperation(f.operation).
		WithResource(resource).
		WithDetails(fmt.Sprintf("limit: %d, current: %d", limit, current)).
		Build()
}

// External Service Errors

// ExternalServiceError creates an error for external service failures
func (f *ErrorFactory) ExternalServiceError(service string, err error) *UnifiedError {
	return External(
		"external_service_error",
		fmt.Sprintf("External service %s is unavailable", service),
	).
		WithCause(err).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("external_service: %s", service)).
		Build()
}

// TimeoutError creates an error for operation timeouts
func (f *ErrorFactory) TimeoutError(operation string, timeout string) *UnifiedError {
	return Timeout(
		"operation_timeout",
		fmt.Sprintf("Operation %s timed out after %s", operation, timeout),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("timeout: %s", timeout)).
		Build()
}

// CircuitBreakerOpen creates an error when circuit breaker is open
func (f *ErrorFactory) CircuitBreakerOpen(service string) *UnifiedError {
	return External(
		"circuit_breaker_open",
		fmt.Sprintf("Circuit breaker is open for service %s", service),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("protected_service: %s", service)).
		Build()
}

// Batch Operation Errors

// PartialFailure creates an error for partially failed batch operations
func (f *ErrorFactory) PartialFailure(succeeded, failed int, failures []error) *UnifiedError {
	failureMessages := make([]string, len(failures))
	for i, err := range failures {
		failureMessages[i] = err.Error()
	}
	
	return Internal(
		"partial_failure",
		fmt.Sprintf("Batch operation partially failed: %d succeeded, %d failed", succeeded, failed),
	).
		WithOperation(f.operation).
		WithDetails(fmt.Sprintf("succeeded: %d, failed: %d, failures: %v", succeeded, failed, failureMessages)).
		Build()
}

// Helper Functions for Common Patterns

// WrapRepositoryError wraps repository errors with consistent context
func (f *ErrorFactory) WrapRepositoryError(err error, operation string) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// If it's already a UnifiedError, enhance it
	if ue, ok := err.(*UnifiedError); ok {
		ue.Operation = operation
		return ue
	}
	
	// Otherwise create a new database error
	return f.DatabaseError(operation, err)
}

// WrapValidationError wraps validation errors with field context
func (f *ErrorFactory) WrapValidationError(err error, field string) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// If it's already a UnifiedError, add field info to details
	if ue, ok := err.(*UnifiedError); ok {
		// Create a new error with updated details
		return &UnifiedError{
			Type:      ue.Type,
			Code:      ue.Code,
			Message:   ue.Message,
			Details:   fmt.Sprintf("%s (field: %s)", ue.Details, field),
			Operation: ue.Operation,
			Resource:  ue.Resource,
			UserID:    ue.UserID,
			RequestID: ue.RequestID,
			Severity:  ue.Severity,
			Retryable: ue.Retryable,
			Cause:     ue.Cause,
		}
	}
	
	// Otherwise create a new validation error
	return f.InvalidInput(field, err.Error())
}

// WrapWithContext wraps any error with operation context
func (f *ErrorFactory) WrapWithContext(err error, ctx context.Context) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// Extract context values
	var ue *UnifiedError
	if unified, ok := err.(*UnifiedError); ok {
		ue = unified
	} else {
		ue = Internal("wrapped_error", err.Error()).WithCause(err).Build()
	}
	
	// Add context information
	ue.Operation = f.operation
	
	// Add request ID if present
	if reqID := ctx.Value("request_id"); reqID != nil {
		ue.RequestID = reqID.(string)
	}
	
	// Add user ID if present
	if userID := ctx.Value("user_id"); userID != nil {
		ue.UserID = userID.(string)
	}
	
	// Add stack trace
	ue.StackTrace = strings.Split(captureFactoryStackTrace(), "\n")
	
	return ue
}

// captureFactoryStackTrace captures the current stack trace for factory errors
func captureFactoryStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	
	var sb strings.Builder
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") {
			sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}
	return sb.String()
}

// Global factory instances for common services

var (
	// NodeServiceErrors provides error creation for node service
	NodeServiceErrors = NewErrorFactory("NodeService")
	
	// CategoryServiceErrors provides error creation for category service
	CategoryServiceErrors = NewErrorFactory("CategoryService")
	
	// EdgeServiceErrors provides error creation for edge service
	EdgeServiceErrors = NewErrorFactory("EdgeService")
	
	// AuthServiceErrors provides error creation for auth service
	AuthServiceErrors = NewErrorFactory("AuthService")
	
	// RepositoryErrors provides error creation for repository layer
	RepositoryErrors = NewErrorFactory("Repository")
)

// Quick error creation functions using default factory

// QuickNotFound creates a not found error quickly
func QuickNotFound(resourceType, resourceID string) *UnifiedError {
	return NewErrorFactory("").NotFound(resourceType, resourceID)
}

// QuickValidation creates a validation error quickly
func QuickValidation(field, reason string) *UnifiedError {
	return NewErrorFactory("").InvalidInput(field, reason)
}

// QuickDatabase creates a database error quickly
func QuickDatabase(operation string, err error) *UnifiedError {
	return NewErrorFactory("").DatabaseError(operation, err)
}

// QuickConflict creates a conflict error quickly
func QuickConflict(resourceType, resourceID string) *UnifiedError {
	return NewErrorFactory("").AlreadyExists(resourceType, resourceID)
}