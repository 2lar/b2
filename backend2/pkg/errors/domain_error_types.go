package errors

import (
	"fmt"
	"strings"
	"time"
)

// DomainErrorType represents the category of domain error
type DomainErrorType string

const (
	// DomainValidationError indicates input validation failure
	DomainValidationError DomainErrorType = "VALIDATION_ERROR"

	// DomainBusinessRuleError indicates a business rule violation
	DomainBusinessRuleError DomainErrorType = "BUSINESS_RULE_ERROR"

	// DomainNotFoundError indicates a resource was not found
	DomainNotFoundError DomainErrorType = "NOT_FOUND"

	// DomainConflictError indicates a conflict with existing state
	DomainConflictError DomainErrorType = "CONFLICT"

	// DomainInfrastructureError indicates an infrastructure-level failure
	DomainInfrastructureError DomainErrorType = "INFRASTRUCTURE_ERROR"

	// DomainAuthorizationError indicates insufficient permissions
	DomainAuthorizationError DomainErrorType = "AUTHORIZATION_ERROR"

	// DomainAuthenticationError indicates authentication failure
	DomainAuthenticationError DomainErrorType = "AUTHENTICATION_ERROR"

	// DomainRateLimitError indicates rate limit exceeded
	DomainRateLimitError DomainErrorType = "RATE_LIMIT_ERROR"

	// DomainTimeoutError indicates operation timeout
	DomainTimeoutError DomainErrorType = "TIMEOUT_ERROR"
)

// DomainError represents a domain-specific error with rich context
type DomainError struct {
	Type       DomainErrorType        `json:"type"`
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	Retryable  bool                   `json:"retryable"`
	StatusCode int                    `json:"status_code"`
}

// NewDomainError creates a new domain error
func NewDomainError(errorType DomainErrorType, code string, message string) *DomainError {
	return &DomainError{
		Type:       errorType,
		Code:       code,
		Message:    message,
		Details:    make(map[string]interface{}),
		Retryable:  false,
		StatusCode: domainErrorTypeToStatusCode(errorType),
	}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// WithCause adds a cause to the error
func (e *DomainError) WithCause(cause error) *DomainError {
	e.Cause = cause
	return e
}

// WithDetail adds a detail to the error
func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
	e.Details[key] = value
	return e
}

// WithDetails adds multiple details to the error
func (e *DomainError) WithDetails(details map[string]interface{}) *DomainError {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithRetryable sets whether the error is retryable
func (e *DomainError) WithRetryable(retryable bool) *DomainError {
	e.Retryable = retryable
	return e
}

// WithStatusCode sets a custom HTTP status code
func (e *DomainError) WithStatusCode(code int) *DomainError {
	e.StatusCode = code
	return e
}

// Is checks if the error is of a specific type
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Type == t.Type && e.Code == t.Code
}

// Unwrap returns the underlying cause
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// domainErrorTypeToStatusCode maps error types to HTTP status codes
func domainErrorTypeToStatusCode(errorType DomainErrorType) int {
	switch errorType {
	case DomainValidationError:
		return 400 // Bad Request
	case DomainBusinessRuleError:
		return 422 // Unprocessable Entity
	case DomainNotFoundError:
		return 404 // Not Found
	case DomainConflictError:
		return 409 // Conflict
	case DomainAuthenticationError:
		return 401 // Unauthorized
	case DomainAuthorizationError:
		return 403 // Forbidden
	case DomainRateLimitError:
		return 429 // Too Many Requests
	case DomainTimeoutError:
		return 504 // Gateway Timeout
	case DomainInfrastructureError:
		return 500 // Internal Server Error
	default:
		return 500 // Internal Server Error
	}
}

// Common domain errors - these are pre-defined errors that can be reused

var (
	// Node errors
	ErrNodeNotFound = NewDomainError(
		DomainNotFoundError,
		"NODE_NOT_FOUND",
		"The requested node does not exist",
	)

	ErrNodeTitleRequired = NewDomainError(
		DomainValidationError,
		"NODE_TITLE_REQUIRED",
		"Node title is required",
	)

	ErrNodeTitleTooLong = NewDomainError(
		DomainValidationError,
		"NODE_TITLE_TOO_LONG",
		"Node title exceeds maximum length",
	).WithDetail("max_length", 255)

	ErrNodeContentTooLong = NewDomainError(
		DomainValidationError,
		"NODE_CONTENT_TOO_LONG",
		"Node content exceeds maximum length",
	).WithDetail("max_length", 50000)

	ErrInvalidNodePosition = NewDomainError(
		DomainValidationError,
		"INVALID_NODE_POSITION",
		"Node position coordinates are invalid",
	)

	// Graph errors
	ErrGraphNotFound = NewDomainError(
		DomainNotFoundError,
		"GRAPH_NOT_FOUND",
		"The requested graph does not exist",
	)

	ErrGraphLimitExceeded = NewDomainError(
		DomainBusinessRuleError,
		"GRAPH_LIMIT_EXCEEDED",
		"Maximum number of nodes in graph exceeded",
	).WithDetail("limit", 10000)

	ErrGraphNameRequired = NewDomainError(
		DomainValidationError,
		"GRAPH_NAME_REQUIRED",
		"Graph name is required",
	)

	ErrDuplicateGraphName = NewDomainError(
		DomainConflictError,
		"DUPLICATE_GRAPH_NAME",
		"A graph with this name already exists",
	)

	// Edge errors
	ErrEdgeNotFound = NewDomainError(
		DomainNotFoundError,
		"EDGE_NOT_FOUND",
		"The requested edge does not exist",
	)

	ErrSelfReferentialEdge = NewDomainError(
		DomainBusinessRuleError,
		"SELF_REFERENTIAL_EDGE",
		"Cannot create an edge from a node to itself",
	)

	ErrDuplicateEdge = NewDomainError(
		DomainConflictError,
		"DUPLICATE_EDGE",
		"An edge between these nodes already exists",
	)

	ErrCyclicDependency = NewDomainError(
		DomainBusinessRuleError,
		"CYCLIC_DEPENDENCY",
		"Creating this edge would result in a cyclic dependency",
	)

	// User errors
	ErrUserNotFound = NewDomainError(
		DomainNotFoundError,
		"USER_NOT_FOUND",
		"The requested user does not exist",
	)

	ErrUserNotAuthorized = NewDomainError(
		DomainAuthorizationError,
		"USER_NOT_AUTHORIZED",
		"User is not authorized to perform this action",
	)

	// Transaction errors
	ErrConcurrentModification = NewDomainError(
		DomainConflictError,
		"CONCURRENT_MODIFICATION",
		"The resource was modified by another process",
	).WithRetryable(true)

	ErrTransactionFailed = NewDomainError(
		DomainInfrastructureError,
		"TRANSACTION_FAILED",
		"Database transaction failed",
	).WithRetryable(true)

	// Rate limiting errors
	ErrRateLimitExceeded = NewDomainError(
		DomainRateLimitError,
		"RATE_LIMIT_EXCEEDED",
		"Too many requests, please try again later",
	).WithRetryable(true)

	// Infrastructure errors
	ErrDatabaseConnection = NewDomainError(
		DomainInfrastructureError,
		"DATABASE_CONNECTION_ERROR",
		"Failed to connect to database",
	).WithRetryable(true)

	ErrEventPublishFailed = NewDomainError(
		DomainInfrastructureError,
		"EVENT_PUBLISH_FAILED",
		"Failed to publish domain event",
	).WithRetryable(true)
)

// ValidationErrors aggregates multiple validation errors
type ValidationErrors struct {
	Errors []*DomainError `json:"errors"`
}

// NewValidationErrors creates a new validation errors collection
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]*DomainError, 0),
	}
}

// Add adds a validation error
func (v *ValidationErrors) Add(field string, message string) {
	err := NewDomainError(DomainValidationError, "FIELD_VALIDATION_ERROR", message).
		WithDetail("field", field)
	v.Errors = append(v.Errors, err)
}

// AddError adds a pre-existing domain error
func (v *ValidationErrors) AddError(err *DomainError) {
	v.Errors = append(v.Errors, err)
}

// HasErrors returns true if there are validation errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// Error implements the error interface
func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return ""
	}

	messages := make([]string, len(v.Errors))
	for i, err := range v.Errors {
		messages[i] = err.Message
	}
	return fmt.Sprintf("Validation failed: %s", strings.Join(messages, "; "))
}

// ToMap converts validation errors to a map for JSON serialization
func (v *ValidationErrors) ToMap() map[string][]string {
	result := make(map[string][]string)

	for _, err := range v.Errors {
		field, ok := err.Details["field"].(string)
		if !ok {
			field = "general"
		}

		if _, exists := result[field]; !exists {
			result[field] = make([]string, 0)
		}
		result[field] = append(result[field], err.Message)
	}

	return result
}

// DomainErrorResponse represents the API error response format for domain errors
type DomainErrorResponse struct {
	Error     bool                   `json:"error"`
	Type      DomainErrorType        `json:"type"`
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// NewDomainErrorResponse creates an error response from a domain error
func NewDomainErrorResponse(err *DomainError, requestID string) *DomainErrorResponse {
	return &DomainErrorResponse{
		Error:     true,
		Type:      err.Type,
		Code:      err.Code,
		Message:   err.Message,
		Details:   err.Details,
		Retryable: err.Retryable,
		RequestID: requestID,
		Timestamp: fmt.Sprintf("%d", timeNow().Unix()),
	}
}

// Helper function for testing (can be mocked)
var timeNow = func() time.Time {
	return time.Now()
}
