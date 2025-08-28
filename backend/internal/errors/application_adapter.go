// Package errors provides adapters for application service layer error handling.
package errors

import (
	"context"
	"errors"
	"fmt"
	
	sharedContext "brain2-backend/internal/context"
)

// ApplicationError creates an application service layer error with context
func ApplicationError(ctx context.Context, operation string, err error) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// Check if it's already a UnifiedError
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		// Enrich with application context
		return enrichWithContext(ctx, unifiedErr, operation)
	}
	
	// Domain errors are already handled by the As check above
	// since domain layer now uses UnifiedError directly
	
	// Try to convert from repository error  
	if repoErr := fromRepositoryPattern(err, operation, ""); repoErr != nil {
		return enrichWithContext(ctx, repoErr, operation)
	}
	
	// Default application error
	appErr := NewError(ErrorTypeApplication, CodeInternalError.String(),
		fmt.Sprintf("Application error in %s", operation)).
		WithOperation(operation).
		WithCause(err).
		WithSeverity(SeverityMedium).
		Build()
		
	return enrichWithContext(ctx, appErr, operation)
}

// enrichWithContext adds context information to the error
func enrichWithContext(ctx context.Context, err *UnifiedError, operation string) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// Add user ID from context
	if userID, ok := sharedContext.GetUserIDFromContext(ctx); ok && userID != "" {
		err.UserID = userID
	}
	
	// Add request ID from context
	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		err.RequestID = requestID
	}
	
	// Add correlation ID from context
	if correlationID, ok := ctx.Value("correlation_id").(string); ok && correlationID != "" {
		if err.RecoveryMetadata == nil {
			err.RecoveryMetadata = make(map[string]interface{})
		}
		err.RecoveryMetadata["correlation_id"] = correlationID
	}
	
	// Add API version from context (if available)
	// Note: GetAPIVersionFromContext would need to be implemented in sharedContext
	// For now, we'll check if it's in the context directly
	if version, ok := ctx.Value("api_version").(string); ok && version != "" {
		if err.RecoveryMetadata == nil {
			err.RecoveryMetadata = make(map[string]interface{})
		}
		err.RecoveryMetadata["api_version"] = version
	}
	
	// Set operation if not already set
	if err.Operation == "" {
		err.Operation = operation
	}
	
	return err
}

// ServiceValidationError creates a validation error for service layer
func ServiceValidationError(field string, message string, value interface{}) *UnifiedError {
	return Validation(CodeValidationFailed.String(), 
		fmt.Sprintf("Validation failed for %s: %s", field, message)).
		WithDetails(fmt.Sprintf("Invalid value: %v", value)).
		WithRecoveryMetadata(map[string]interface{}{
			"field": field,
			"value": value,
		}).
		Build()
}

// ServiceNotFoundError creates a not found error for service layer
func ServiceNotFoundError(resource string, identifier string) *UnifiedError {
	code := determineNotFoundCode(resource)
	return NotFound(code, fmt.Sprintf("%s not found: %s", resource, identifier)).
		WithResource(resource).
		WithRecoveryMetadata(map[string]interface{}{
			"identifier": identifier,
		}).
		Build()
}

// ServiceConflictError creates a conflict error for service layer
func ServiceConflictError(resource string, message string) *UnifiedError {
	code := determineExistsCode(resource)
	return Conflict(code, message).
		WithResource(resource).
		WithRetryable(true).
		Build()
}

// ServiceAuthorizationError creates an authorization error for service layer
func ServiceAuthorizationError(userID string, resource string, action string) *UnifiedError {
	return Unauthorized(CodeUserUnauthorized.String(),
		fmt.Sprintf("User %s is not authorized to %s %s", userID, action, resource)).
		WithUserID(userID).
		WithResource(resource).
		WithRecoveryMetadata(map[string]interface{}{
			"action": action,
		}).
		Build()
}

// ServiceTimeoutError creates a timeout error for service layer
func ServiceTimeoutError(operation string, timeout int) *UnifiedError {
	return Timeout(CodeTimeout.String(),
		fmt.Sprintf("Operation %s timed out after %d seconds", operation, timeout)).
		WithOperation(operation).
		WithRetryable(true).
		WithRecoveryMetadata(map[string]interface{}{
			"timeout_seconds": timeout,
		}).
		Build()
}

// BulkOperationError represents errors in bulk operations
type BulkOperationError struct {
	Operation       string
	TotalItems      int
	SuccessfulItems int
	FailedItems     int
	Errors          []BulkItemError
}

// BulkItemError represents an error for a specific item in bulk operation
type BulkItemError struct {
	Index      int
	Identifier string
	Error      *UnifiedError
}

// NewBulkOperationError creates a new bulk operation error
func NewBulkOperationError(operation string, totalItems int) *BulkOperationError {
	return &BulkOperationError{
		Operation:  operation,
		TotalItems: totalItems,
		Errors:     make([]BulkItemError, 0),
	}
}

// AddError adds an error for a specific item
func (b *BulkOperationError) AddError(index int, identifier string, err error) {
	var unifiedErr *UnifiedError
	if !errors.As(err, &unifiedErr) {
		unifiedErr = ApplicationError(context.Background(), b.Operation, err)
	}
	
	b.Errors = append(b.Errors, BulkItemError{
		Index:      index,
		Identifier: identifier,
		Error:      unifiedErr,
	})
	b.FailedItems++
}

// AddSuccess increments the success counter
func (b *BulkOperationError) AddSuccess() {
	b.SuccessfulItems++
}

// HasErrors returns true if there are any errors
func (b *BulkOperationError) HasErrors() bool {
	return len(b.Errors) > 0
}

// ToUnifiedError converts bulk operation error to unified error
func (b *BulkOperationError) ToUnifiedError() *UnifiedError {
	if !b.HasErrors() {
		return nil
	}
	
	message := fmt.Sprintf("Bulk %s completed with errors: %d/%d items failed",
		b.Operation, b.FailedItems, b.TotalItems)
		
	// Determine severity based on failure rate
	var severity ErrorSeverity
	failureRate := float64(b.FailedItems) / float64(b.TotalItems)
	switch {
	case failureRate >= 0.5:
		severity = SeverityHigh
	case failureRate >= 0.2:
		severity = SeverityMedium
	default:
		severity = SeverityLow
	}
	
	// Build error details
	errorDetails := make([]map[string]interface{}, len(b.Errors))
	for i, itemErr := range b.Errors {
		errorDetails[i] = map[string]interface{}{
			"index":      itemErr.Index,
			"identifier": itemErr.Identifier,
			"error_code": itemErr.Error.Code,
			"message":    itemErr.Error.Message,
		}
	}
	
	return NewError(ErrorTypeApplication, "BULK_OPERATION_PARTIAL_FAILURE", message).
		WithOperation(b.Operation).
		WithSeverity(severity).
		WithRecoveryMetadata(map[string]interface{}{
			"total_items":      b.TotalItems,
			"successful_items": b.SuccessfulItems,
			"failed_items":     b.FailedItems,
			"errors":           errorDetails,
		}).
		Build()
}

// WrapServiceError wraps a service error with operation context
func WrapServiceError(ctx context.Context, err error, operation string, details map[string]interface{}) *UnifiedError {
	if err == nil {
		return nil
	}
	
	appErr := ApplicationError(ctx, operation, err)
	
	// Add additional details
	if details != nil {
		if appErr.RecoveryMetadata == nil {
			appErr.RecoveryMetadata = details
		} else {
			// Merge details
			for k, v := range details {
				appErr.RecoveryMetadata[k] = v
			}
		}
	}
	
	return appErr
}