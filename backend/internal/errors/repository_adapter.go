// Package errors provides adapters for repository layer error handling.
package errors

import (
	"errors"
	"fmt"
	"strings"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
)

// RepositoryError creates a repository-specific error
func RepositoryError(operation string, resource string, cause error) *UnifiedError {
	if cause == nil {
		return nil
	}
	
	// Check for specific DynamoDB errors
	if unifiedErr := fromDynamoDBError(cause, operation, resource); unifiedErr != nil {
		return unifiedErr
	}
	
	// Check for common repository patterns
	if unifiedErr := fromRepositoryPattern(cause, operation, resource); unifiedErr != nil {
		return unifiedErr
	}
	
	// Default repository error
	return NewError(ErrorTypeRepository, CodeRepositoryError.String(), 
		fmt.Sprintf("Repository operation failed: %s", operation)).
		WithOperation(operation).
		WithResource(resource).
		WithCause(cause).
		WithSeverity(SeverityHigh).
		Build()
}

// fromDynamoDBError converts DynamoDB-specific errors to UnifiedError
func fromDynamoDBError(err error, operation string, resource string) *UnifiedError {
	var ae smithy.APIError
	if !errors.As(err, &ae) {
		return nil
	}
	
	errorCode := ae.ErrorCode()
	message := ae.ErrorMessage()
	
	switch errorCode {
	case "ResourceNotFoundException":
		return NotFound(CodeDatabaseError.String(), "Table or resource not found").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithDetails(message).
			Build()
			
	case "ConditionalCheckFailedException":
		return Conflict(CodeOptimisticLock.String(), "Conditional check failed").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithDetails("The item has been modified by another operation").
			WithRetryable(true).
			Build()
			
	case "ItemCollectionSizeLimitExceededException":
		return NewError(ErrorTypeRepository, CodeDatabaseError.String(), 
			"Item collection size limit exceeded").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithSeverity(SeverityHigh).
			Build()
			
	case "ProvisionedThroughputExceededException":
		return NewError(ErrorTypeRateLimit, CodeRateLimitExceeded.String(),
			"DynamoDB throughput exceeded").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			WithRetryAfter(1).
			Build()
			
	case "RequestLimitExceeded":
		return NewError(ErrorTypeRateLimit, CodeRateLimitExceeded.String(),
			"DynamoDB request limit exceeded").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			WithRetryAfter(1).
			Build()
			
	case "InternalServerError":
		return Internal(CodeDynamoDBError.String(), "DynamoDB internal error").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			Build()
			
	case "ServiceUnavailable":
		return NewError(ErrorTypeUnavailable, CodeServiceUnavailable.String(),
			"DynamoDB service unavailable").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			Build()
			
	case "ValidationException":
		return Validation(CodeInvalidInput.String(), "DynamoDB validation error").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithDetails(message).
			Build()
			
	case "TransactionCanceledException":
		return handleTransactionCanceled(err, operation, resource)
		
	default:
		return nil
	}
}

// handleTransactionCanceled processes transaction cancellation reasons
func handleTransactionCanceled(err error, operation string, resource string) *UnifiedError {
	var tce *types.TransactionCanceledException
	if !errors.As(err, &tce) {
		return Conflict(CodeTransactionFailed.String(), "Transaction canceled").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			Build()
	}
	
	// Analyze cancellation reasons
	for _, reason := range tce.CancellationReasons {
		if reason.Code != nil {
			switch *reason.Code {
			case "ConditionalCheckFailed":
				return Conflict(CodeOptimisticLock.String(), "Transaction canceled: conditional check failed").
					WithOperation(operation).
					WithResource(resource).
					WithCause(err).
					WithDetails(*reason.Message).
					WithRetryable(true).
					Build()
					
			case "ItemCollectionSizeLimitExceeded":
				return NewError(ErrorTypeRepository, CodeDatabaseError.String(),
					"Transaction canceled: item collection size limit exceeded").
					WithOperation(operation).
					WithResource(resource).
					WithCause(err).
					WithSeverity(SeverityHigh).
					Build()
					
			case "ValidationError":
				return Validation(CodeInvalidInput.String(), "Transaction canceled: validation error").
					WithOperation(operation).
					WithResource(resource).
					WithCause(err).
					WithDetails(*reason.Message).
					Build()
			}
		}
	}
	
	return Conflict(CodeTransactionFailed.String(), "Transaction canceled").
		WithOperation(operation).
		WithResource(resource).
		WithCause(err).
		WithRetryable(true).
		Build()
}

// fromRepositoryPattern checks for common repository error patterns
func fromRepositoryPattern(err error, operation string, resource string) *UnifiedError {
	if err == nil {
		return nil
	}
	
	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)
	
	// Check for not found patterns
	if strings.Contains(errLower, "not found") || 
	   strings.Contains(errLower, "does not exist") ||
	   strings.Contains(errLower, "no such") {
		code := determineNotFoundCode(resource)
		return NotFound(code, fmt.Sprintf("%s not found", resource)).
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
	}
	
	// Check for already exists patterns
	if strings.Contains(errLower, "already exists") ||
	   strings.Contains(errLower, "duplicate") {
		code := determineExistsCode(resource)
		return Conflict(code, fmt.Sprintf("%s already exists", resource)).
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
	}
	
	// Check for timeout patterns
	if strings.Contains(errLower, "timeout") ||
	   strings.Contains(errLower, "context deadline exceeded") ||
	   strings.Contains(errLower, "context canceled") {
		return Timeout(CodeTimeout.String(), "Operation timed out").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			Build()
	}
	
	// Check for connection patterns
	if strings.Contains(errLower, "connection") ||
	   strings.Contains(errLower, "network") ||
	   strings.Contains(errLower, "no such host") {
		return Connection(CodeConnectionFailed.String(), "Connection failed").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			Build()
	}
	
	// Check for idempotency patterns
	if strings.Contains(errLower, "idempotency") {
		return Conflict(CodeIdempotencyConflict.String(), "Idempotency conflict detected").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithDetails("Request with same idempotency key already processed").
			Build()
	}
	
	// Check for optimistic lock patterns
	if strings.Contains(errLower, "version mismatch") ||
	   strings.Contains(errLower, "concurrent modification") ||
	   strings.Contains(errLower, "optimistic lock") {
		return Conflict(CodeOptimisticLock.String(), "Optimistic lock failure").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			WithRetryable(true).
			WithDetails("Resource was modified by another operation").
			Build()
	}
	
	return nil
}

// determineNotFoundCode returns the appropriate error code based on resource type
func determineNotFoundCode(resource string) string {
	switch strings.ToLower(resource) {
	case "node":
		return CodeNodeNotFound.String()
	case "edge":
		return CodeEdgeNotFound.String()
	case "category":
		return CodeCategoryNotFound.String()
	case "user":
		return CodeUserNotFound.String()
	default:
		return "RESOURCE_NOT_FOUND"
	}
}

// determineExistsCode returns the appropriate error code based on resource type
func determineExistsCode(resource string) string {
	switch strings.ToLower(resource) {
	case "node":
		return CodeNodeAlreadyExists.String()
	case "edge":
		return CodeEdgeAlreadyExists.String()
	case "category":
		return CodeCategoryAlreadyExists.String()
	default:
		return "RESOURCE_EXISTS"
	}
}

// WrapRepositoryError wraps a repository error with additional context
func WrapRepositoryError(err error, operation string, details map[string]interface{}) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// Check if it's already a UnifiedError
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		// Add additional context
		unifiedErr.Operation = operation
		if unifiedErr.RecoveryMetadata == nil {
			unifiedErr.RecoveryMetadata = details
		} else {
			// Merge details
			for k, v := range details {
				unifiedErr.RecoveryMetadata[k] = v
			}
		}
		return unifiedErr
	}
	
	// Create new repository error
	resource := ""
	if r, ok := details["resource"].(string); ok {
		resource = r
	}
	
	repoErr := RepositoryError(operation, resource, err)
	if repoErr != nil && details != nil {
		repoErr.RecoveryMetadata = details
	}
	
	return repoErr
}

// IsRepositoryRetryable checks if a repository error should be retried
func IsRepositoryRetryable(err error) bool {
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		return unifiedErr.Retryable
	}
	
	// Check for specific patterns that indicate retryable errors
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "throughput") ||
		strings.Contains(errMsg, "service unavailable") ||
		strings.Contains(errMsg, "internal server error") ||
		strings.Contains(errMsg, "optimistic lock") ||
		strings.Contains(errMsg, "conditional check")
}