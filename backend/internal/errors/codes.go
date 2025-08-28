// Package errors provides standardized error codes for consistent error handling.
package errors

// ErrorCode represents a unique error code for specific error scenarios
type ErrorCode string

// Domain error codes
const (
	// Node-related errors
	CodeNodeNotFound          ErrorCode = "NODE_NOT_FOUND"
	CodeNodeAlreadyExists     ErrorCode = "NODE_ALREADY_EXISTS"
	CodeNodeValidationFailed  ErrorCode = "NODE_VALIDATION_FAILED"
	CodeNodeContentEmpty      ErrorCode = "NODE_CONTENT_EMPTY"
	CodeNodeContentTooLong    ErrorCode = "NODE_CONTENT_TOO_LONG"
	CodeNodeTitleTooLong      ErrorCode = "NODE_TITLE_TOO_LONG"
	CodeNodeArchived          ErrorCode = "NODE_ARCHIVED"
	CodeNodeSelfConnection    ErrorCode = "NODE_SELF_CONNECTION"
	CodeNodeCrossUser         ErrorCode = "NODE_CROSS_USER"
	CodeNodeCreationFailed    ErrorCode = "NODE_CREATION_FAILED"
	CodeNodeUpdateFailed      ErrorCode = "NODE_UPDATE_FAILED"
	CodeNodeDeletionFailed    ErrorCode = "NODE_DELETION_FAILED"
	
	// Edge-related errors
	CodeEdgeNotFound          ErrorCode = "EDGE_NOT_FOUND"
	CodeEdgeAlreadyExists     ErrorCode = "EDGE_ALREADY_EXISTS"
	CodeEdgeValidationFailed  ErrorCode = "EDGE_VALIDATION_FAILED"
	CodeEdgeInvalidWeight     ErrorCode = "EDGE_INVALID_WEIGHT"
	CodeEdgeCreationFailed    ErrorCode = "EDGE_CREATION_FAILED"
	CodeEdgeDeletionFailed    ErrorCode = "EDGE_DELETION_FAILED"
	
	// Category-related errors
	CodeCategoryNotFound      ErrorCode = "CATEGORY_NOT_FOUND"
	CodeCategoryAlreadyExists ErrorCode = "CATEGORY_ALREADY_EXISTS"
	CodeCategoryCircularRef   ErrorCode = "CATEGORY_CIRCULAR_REF"
	CodeCategoryInvalidLevel  ErrorCode = "CATEGORY_INVALID_LEVEL"
	
	// User-related errors
	CodeUserNotFound          ErrorCode = "USER_NOT_FOUND"
	CodeUserUnauthorized      ErrorCode = "USER_UNAUTHORIZED"
	CodeUserForbidden         ErrorCode = "USER_FORBIDDEN"
	CodeUserIDEmpty           ErrorCode = "USER_ID_EMPTY"
	CodeUserIDTooLong         ErrorCode = "USER_ID_TOO_LONG"
	
	// Content-related errors
	CodeContentEmpty          ErrorCode = "CONTENT_EMPTY"
	CodeContentTooLong        ErrorCode = "CONTENT_TOO_LONG"
	CodeInappropriateContent  ErrorCode = "INAPPROPRIATE_CONTENT"
	CodeCannotConnectArchived ErrorCode = "CANNOT_CONNECT_ARCHIVED"
	
	// Validation errors
	CodeValidationFailed      ErrorCode = "VALIDATION_FAILED"
	CodeInvalidInput          ErrorCode = "INVALID_INPUT"
	CodeMissingField          ErrorCode = "MISSING_FIELD"
	CodeInvalidFormat         ErrorCode = "INVALID_FORMAT"
	CodeInvalidUUID           ErrorCode = "INVALID_UUID"
	
	// Repository errors
	CodeRepositoryError       ErrorCode = "REPOSITORY_ERROR"
	CodeDatabaseError         ErrorCode = "DATABASE_ERROR"
	CodeOptimisticLock        ErrorCode = "OPTIMISTIC_LOCK"
	CodeTransactionFailed     ErrorCode = "TRANSACTION_FAILED"
	CodeIdempotencyConflict   ErrorCode = "IDEMPOTENCY_CONFLICT"
	CodeDataCorruption        ErrorCode = "DATA_CORRUPTION"
	
	// Infrastructure errors
	CodeInternalError         ErrorCode = "INTERNAL_ERROR"
	CodeServiceUnavailable    ErrorCode = "SERVICE_UNAVAILABLE"
	CodeTimeout               ErrorCode = "TIMEOUT"
	CodeConnectionFailed      ErrorCode = "CONNECTION_FAILED"
	CodeRateLimitExceeded     ErrorCode = "RATE_LIMIT_EXCEEDED"
	CodeEventPublishFailed    ErrorCode = "EVENT_PUBLISH_FAILED"
	
	// External service errors
	CodeExternalServiceError  ErrorCode = "EXTERNAL_SERVICE_ERROR"
	CodeDynamoDBError         ErrorCode = "DYNAMODB_ERROR"
	CodeEventBridgeError      ErrorCode = "EVENTBRIDGE_ERROR"
	CodeAPIGatewayError       ErrorCode = "API_GATEWAY_ERROR"
)

// HTTPStatusCode returns the appropriate HTTP status code for an error code
func (c ErrorCode) HTTPStatusCode() int {
	switch c {
	// 400 Bad Request
	case CodeNodeValidationFailed, CodeNodeContentEmpty, CodeNodeContentTooLong,
		CodeNodeTitleTooLong, CodeEdgeValidationFailed, CodeEdgeInvalidWeight,
		CodeValidationFailed, CodeInvalidInput, CodeMissingField, 
		CodeInvalidFormat, CodeInvalidUUID, CodeUserIDEmpty, CodeUserIDTooLong:
		return 400
		
	// 401 Unauthorized
	case CodeUserUnauthorized:
		return 401
		
	// 403 Forbidden
	case CodeUserForbidden, CodeNodeCrossUser:
		return 403
		
	// 404 Not Found
	case CodeNodeNotFound, CodeEdgeNotFound, CodeCategoryNotFound, CodeUserNotFound:
		return 404
		
	// 409 Conflict
	case CodeNodeAlreadyExists, CodeEdgeAlreadyExists, CodeCategoryAlreadyExists,
		CodeOptimisticLock, CodeIdempotencyConflict, CodeCategoryCircularRef,
		CodeNodeArchived, CodeNodeSelfConnection:
		return 409
		
	// 429 Too Many Requests
	case CodeRateLimitExceeded:
		return 429
		
	// 503 Service Unavailable
	case CodeServiceUnavailable, CodeConnectionFailed, CodeTimeout:
		return 503
		
	// 500 Internal Server Error (default)
	default:
		return 500
	}
}

// String returns the string representation of the error code
func (c ErrorCode) String() string {
	return string(c)
}

// IsRetryable returns whether an error with this code should be retried
func (c ErrorCode) IsRetryable() bool {
	switch c {
	case CodeTimeout, CodeConnectionFailed, CodeServiceUnavailable,
		CodeDynamoDBError, CodeEventBridgeError, CodeOptimisticLock,
		CodeRateLimitExceeded, CodeEventPublishFailed, CodeTransactionFailed:
		return true
	default:
		return false
	}
}

// Severity returns the severity level for the error code
func (c ErrorCode) Severity() ErrorSeverity {
	switch c {
	// Critical - System failures
	case CodeInternalError, CodeDatabaseError:
		return SeverityCritical
		
	// High - Service disruptions
	case CodeServiceUnavailable, CodeConnectionFailed, CodeTransactionFailed,
		CodeNodeCreationFailed, CodeNodeUpdateFailed, CodeNodeDeletionFailed,
		CodeEdgeCreationFailed, CodeEdgeDeletionFailed, CodeEventPublishFailed:
		return SeverityHigh
		
	// Medium - Business logic violations
	case CodeOptimisticLock, CodeIdempotencyConflict, CodeTimeout,
		CodeRateLimitExceeded, CodeNodeArchived:
		return SeverityMedium
		
	// Low - User errors
	default:
		return SeverityLow
	}
}