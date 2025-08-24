// Package errors provides a unified error handling system that consolidates
// the multiple error handling approaches found in the codebase.
// This unified system follows SOLID principles and provides consistent
// error handling across all application layers.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ============================================================================
// UNIFIED ERROR TYPES AND CLASSIFICATION
// ============================================================================

// ErrorType defines the category of error for proper handling and response.
type ErrorType string

const (
	// Business logic errors
	ErrorTypeValidation   ErrorType = "VALIDATION"
	ErrorTypeNotFound     ErrorType = "NOT_FOUND" 
	ErrorTypeConflict     ErrorType = "CONFLICT"
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"
	ErrorTypeForbidden    ErrorType = "FORBIDDEN"
	
	// Infrastructure errors
	ErrorTypeInternal     ErrorType = "INTERNAL"
	ErrorTypeTimeout      ErrorType = "TIMEOUT"
	ErrorTypeConnection   ErrorType = "CONNECTION"
	ErrorTypeRateLimit    ErrorType = "RATE_LIMIT"
	
	// External service errors
	ErrorTypeExternal     ErrorType = "EXTERNAL"
	ErrorTypeUnavailable  ErrorType = "UNAVAILABLE"
)

// ErrorSeverity defines the severity level for logging and monitoring.
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "LOW"
	SeverityMedium   ErrorSeverity = "MEDIUM"
	SeverityHigh     ErrorSeverity = "HIGH"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// ============================================================================
// UNIFIED ERROR STRUCTURE
// ============================================================================

// UnifiedError is the single error type that consolidates all error handling.
// This error type replaces the multiple error types found in the codebase
// (pkg/errors, domain/shared/errors, repository/errors).
type UnifiedError struct {
	// Core error information
	Type     ErrorType     `json:"type"`
	Code     string        `json:"code"`      // Specific error code for programmatic handling
	Message  string        `json:"message"`   // Human-readable message
	Details  string        `json:"details"`   // Additional context information
	
	// Error context
	Operation string        `json:"operation"` // The operation that failed
	Resource  string        `json:"resource"`  // The resource being operated on
	UserID    string        `json:"userId"`    // User context (if applicable)
	RequestID string        `json:"requestId"` // Request tracing ID
	
	// Error metadata
	Severity  ErrorSeverity `json:"severity"`
	Retryable bool          `json:"retryable"` // Whether the operation can be retried
	RetryAfter time.Duration `json:"retryAfter,omitempty"` // How long to wait before retry
	RetryCount int          `json:"retryCount,omitempty"` // Number of retries attempted
	MaxRetries int          `json:"maxRetries,omitempty"` // Maximum retries allowed
	Cause     error         `json:"-"`         // Underlying cause (not serialized)
	
	// Recovery information
	RecoveryStrategy string                 `json:"recoveryStrategy,omitempty"` // Suggested recovery approach
	CompensationFunc CompensationFunc       `json:"-"` // Function to compensate for the error
	RecoveryMetadata map[string]interface{} `json:"recoveryMetadata,omitempty"` // Additional recovery data
	
	// Stack trace information (for debugging)
	StackTrace []string      `json:"stackTrace,omitempty"`
	File       string        `json:"file,omitempty"`
	Line       int           `json:"line,omitempty"`
}

// Error implements the error interface.
func (e *UnifiedError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s:%s] %s: %s", e.Type, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap allows errors.Is and errors.As to work with the underlying cause.
func (e *UnifiedError) Unwrap() error {
	return e.Cause
}

// String provides a detailed string representation for logging.
func (e *UnifiedError) String() string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("Error: %s\n", e.Error()))
	
	if e.Operation != "" {
		builder.WriteString(fmt.Sprintf("Operation: %s\n", e.Operation))
	}
	if e.Resource != "" {
		builder.WriteString(fmt.Sprintf("Resource: %s\n", e.Resource))
	}
	if e.UserID != "" {
		builder.WriteString(fmt.Sprintf("UserID: %s\n", e.UserID))
	}
	if e.RequestID != "" {
		builder.WriteString(fmt.Sprintf("RequestID: %s\n", e.RequestID))
	}
	
	builder.WriteString(fmt.Sprintf("Severity: %s\n", e.Severity))
	builder.WriteString(fmt.Sprintf("Retryable: %t\n", e.Retryable))
	
	if e.Cause != nil {
		builder.WriteString(fmt.Sprintf("Cause: %v\n", e.Cause))
	}
	
	if e.File != "" && e.Line > 0 {
		builder.WriteString(fmt.Sprintf("Location: %s:%d\n", e.File, e.Line))
	}
	
	return builder.String()
}

// ============================================================================
// ERROR BUILDER FOR FLUENT CONSTRUCTION
// ============================================================================

// ErrorBuilder provides a fluent interface for constructing UnifiedError instances.
type ErrorBuilder struct {
	error *UnifiedError
}

// NewError creates a new error builder with the specified type and message.
func NewError(errType ErrorType, code, message string) *ErrorBuilder {
	// Capture stack trace information
	_, file, line, _ := runtime.Caller(1)
	
	return &ErrorBuilder{
		error: &UnifiedError{
			Type:       errType,
			Code:       code,
			Message:    message,
			Severity:   SeverityMedium, // Default severity
			Retryable:  false,          // Default to non-retryable
			File:       file,
			Line:       line,
			StackTrace: captureStackTrace(),
		},
	}
}

// WithDetails adds additional details to the error.
func (b *ErrorBuilder) WithDetails(details string) *ErrorBuilder {
	b.error.Details = details
	return b
}

// WithOperation specifies the operation that failed.
func (b *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	b.error.Operation = operation
	return b
}

// WithResource specifies the resource being operated on.
func (b *ErrorBuilder) WithResource(resource string) *ErrorBuilder {
	b.error.Resource = resource
	return b
}

// WithUserID adds user context to the error.
func (b *ErrorBuilder) WithUserID(userID string) *ErrorBuilder {
	b.error.UserID = userID
	return b
}

// WithRequestID adds request tracing information.
func (b *ErrorBuilder) WithRequestID(requestID string) *ErrorBuilder {
	b.error.RequestID = requestID
	return b
}

// WithSeverity sets the error severity.
func (b *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder {
	b.error.Severity = severity
	return b
}

// WithRetryable marks the error as retryable.
func (b *ErrorBuilder) WithRetryable(retryable bool) *ErrorBuilder {
	b.error.Retryable = retryable
	return b
}

// WithCause adds the underlying cause error.
func (b *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	b.error.Cause = cause
	return b
}

// WithRetryAfter sets how long to wait before retrying.
func (b *ErrorBuilder) WithRetryAfter(duration time.Duration) *ErrorBuilder {
	b.error.RetryAfter = duration
	b.error.Retryable = true // Automatically mark as retryable
	return b
}

// WithRetryInfo sets retry metadata.
func (b *ErrorBuilder) WithRetryInfo(retryCount, maxRetries int) *ErrorBuilder {
	b.error.RetryCount = retryCount
	b.error.MaxRetries = maxRetries
	return b
}

// WithRecoveryStrategy sets the suggested recovery approach.
func (b *ErrorBuilder) WithRecoveryStrategy(strategy string) *ErrorBuilder {
	b.error.RecoveryStrategy = strategy
	return b
}

// WithCompensation adds a compensation function.
func (b *ErrorBuilder) WithCompensation(fn CompensationFunc) *ErrorBuilder {
	b.error.CompensationFunc = fn
	return b
}

// WithRecoveryMetadata adds recovery metadata.
func (b *ErrorBuilder) WithRecoveryMetadata(metadata map[string]interface{}) *ErrorBuilder {
	b.error.RecoveryMetadata = metadata
	return b
}

// Build returns the constructed UnifiedError.
func (b *ErrorBuilder) Build() *UnifiedError {
	return b.error
}

// ============================================================================
// CONVENIENCE CONSTRUCTORS
// ============================================================================

// Validation creates a validation error.
func Validation(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeValidation, code, message).
		WithSeverity(SeverityLow).
		WithRetryable(false)
}

// NotFound creates a not found error.
func NotFound(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeNotFound, code, message).
		WithSeverity(SeverityLow).
		WithRetryable(false)
}

// Conflict creates a conflict error.
func Conflict(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeConflict, code, message).
		WithSeverity(SeverityMedium).
		WithRetryable(true)
}

// Unauthorized creates an unauthorized error.
func Unauthorized(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeUnauthorized, code, message).
		WithSeverity(SeverityMedium).
		WithRetryable(false)
}

// Internal creates an internal error.
func Internal(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeInternal, code, message).
		WithSeverity(SeverityHigh).
		WithRetryable(false)
}

// Timeout creates a timeout error.
func Timeout(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeTimeout, code, message).
		WithSeverity(SeverityMedium).
		WithRetryable(true)
}

// Connection creates a connection error.
func Connection(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeConnection, code, message).
		WithSeverity(SeverityHigh).
		WithRetryable(true)
}

// External creates an external service error.
func External(code, message string) *ErrorBuilder {
	return NewError(ErrorTypeExternal, code, message).
		WithSeverity(SeverityMedium).
		WithRetryable(true)
}

// ============================================================================
// ERROR CLASSIFICATION AND CHECKING
// ============================================================================

// IsType checks if an error is of a specific type.
func IsType(err error, errType ErrorType) bool {
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		return unifiedErr.Type == errType
	}
	return false
}

// IsValidation checks if an error is a validation error.
func IsValidation(err error) bool {
	return IsType(err, ErrorTypeValidation)
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return IsType(err, ErrorTypeNotFound)
}

// IsConflict checks if an error is a conflict error.
func IsConflict(err error) bool {
	return IsType(err, ErrorTypeConflict)
}

// IsUnauthorized checks if an error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return IsType(err, ErrorTypeUnauthorized)
}

// IsInternal checks if an error is an internal error.
func IsInternal(err error) bool {
	return IsType(err, ErrorTypeInternal)
}

// IsTimeout checks if an error is a timeout error.
func IsTimeout(err error) bool {
	return IsType(err, ErrorTypeTimeout)
}

// IsConnection checks if an error is a connection error.
func IsConnection(err error) bool {
	return IsType(err, ErrorTypeConnection)
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		return unifiedErr.Retryable
	}
	return false
}

// GetSeverity returns the severity of an error.
func GetSeverity(err error) ErrorSeverity {
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		return unifiedErr.Severity
	}
	return SeverityMedium // Default severity
}

// ============================================================================
// ERROR WRAPPING AND CONTEXT PRESERVATION
// ============================================================================

// Wrap wraps an existing error with additional context while preserving the original error chain.
func Wrap(err error, operation, message string) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// If it's already a UnifiedError, preserve the original type and add context
	var existingErr *UnifiedError
	if errors.As(err, &existingErr) {
		return &UnifiedError{
			Type:       existingErr.Type,
			Code:       existingErr.Code,
			Message:    message,
			Details:    existingErr.Message, // Original message becomes details
			Operation:  operation,
			Resource:   existingErr.Resource,
			UserID:     existingErr.UserID,
			RequestID:  existingErr.RequestID,
			Severity:   existingErr.Severity,
			Retryable:  existingErr.Retryable,
			Cause:      err,
			StackTrace: existingErr.StackTrace,
			File:       existingErr.File,
			Line:       existingErr.Line,
		}
	}
	
	// For non-UnifiedError types, create a new internal error
	_, file, line, _ := runtime.Caller(1)
	return &UnifiedError{
		Type:       ErrorTypeInternal,
		Code:       "WRAP_ERROR",
		Message:    message,
		Details:    err.Error(),
		Operation:  operation,
		Severity:   SeverityMedium,
		Retryable:  false,
		Cause:      err,
		File:       file,
		Line:       line,
		StackTrace: captureStackTrace(),
	}
}

// ============================================================================
// MIGRATION HELPERS FOR EXISTING ERROR TYPES
// ============================================================================

// FromLegacyError converts errors from the old error systems to UnifiedError.
func FromLegacyError(err error) *UnifiedError {
	if err == nil {
		return nil
	}
	
	// Handle existing domain errors (from domain/shared/errors.go)
	errMsg := err.Error()
	
	switch {
	case strings.Contains(errMsg, "validation failed") || 
		 strings.Contains(errMsg, "cannot be empty") ||
		 strings.Contains(errMsg, "invalid"):
		return Validation("DOMAIN_VALIDATION", err.Error()).Build()
		
	case strings.Contains(errMsg, "not found"):
		return NotFound("RESOURCE_NOT_FOUND", err.Error()).Build()
		
	case strings.Contains(errMsg, "already exists"):
		return Conflict("RESOURCE_EXISTS", err.Error()).Build()
		
	case strings.Contains(errMsg, "unauthorized"):
		return Unauthorized("ACCESS_DENIED", err.Error()).Build()
		
	case strings.Contains(errMsg, "timeout") || 
		 strings.Contains(errMsg, "context deadline exceeded"):
		return Timeout("OPERATION_TIMEOUT", err.Error()).Build()
		
	case strings.Contains(errMsg, "connection"):
		return Connection("CONNECTION_FAILED", err.Error()).Build()
		
	default:
		return Internal("UNKNOWN_ERROR", err.Error()).WithCause(err).Build()
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// captureStackTrace captures the current stack trace for debugging.
func captureStackTrace() []string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(2, pcs[:]) // Skip captureStackTrace and NewError
	
	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	
	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}
	
	return stack
}