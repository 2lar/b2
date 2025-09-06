package errors

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// Domain errors
	ErrorTypeValidation   ErrorType = "VALIDATION"
	ErrorTypeNotFound     ErrorType = "NOT_FOUND"
	ErrorTypeConflict     ErrorType = "CONFLICT"
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"
	ErrorTypeForbidden    ErrorType = "FORBIDDEN"
	
	// Application errors
	ErrorTypeInternal     ErrorType = "INTERNAL"
	ErrorTypeTimeout      ErrorType = "TIMEOUT"
	ErrorTypeRateLimit    ErrorType = "RATE_LIMIT"
	ErrorTypeUnavailable  ErrorType = "UNAVAILABLE"
	
	// Infrastructure errors
	ErrorTypeDatabase     ErrorType = "DATABASE"
	ErrorTypeNetwork      ErrorType = "NETWORK"
	ErrorTypeExternal     ErrorType = "EXTERNAL"
)

// AppError represents an application-specific error
type AppError struct {
	Type       ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	Code       string                 `json:"code,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	StackTrace string                 `json:"-"`
	HTTPStatus int                    `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithCode adds an error code
func (e *AppError) WithCode(code string) *AppError {
	e.Code = code
	return e
}

// WithDetails adds error details
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	e.Details = details
	return e
}

// WithCause wraps an underlying error
func (e *AppError) WithCause(err error) *AppError {
	e.Cause = err
	return e
}

// captureStackTrace captures the current stack trace
func captureStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	
	stack := ""
	for {
		frame, more := frames.Next()
		stack += fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}
	return stack
}

// Constructor functions for common error types

// NewValidationError creates a validation error
func NewValidationError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
		StackTrace: captureStackTrace(),
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
		StackTrace: captureStackTrace(),
	}
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeConflict,
		Message:    message,
		HTTPStatus: http.StatusConflict,
		StackTrace: captureStackTrace(),
	}
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "unauthorized"
	}
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
		StackTrace: captureStackTrace(),
	}
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "forbidden"
	}
	return &AppError{
		Type:       ErrorTypeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
		StackTrace: captureStackTrace(),
	}
}

// NewInternalError creates an internal error
func NewInternalError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		StackTrace: captureStackTrace(),
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string) *AppError {
	return &AppError{
		Type:       ErrorTypeTimeout,
		Message:    fmt.Sprintf("operation '%s' timed out", operation),
		HTTPStatus: http.StatusRequestTimeout,
		StackTrace: captureStackTrace(),
	}
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(limit int, window string) *AppError {
	return &AppError{
		Type:       ErrorTypeRateLimit,
		Message:    fmt.Sprintf("rate limit exceeded: %d requests per %s", limit, window),
		HTTPStatus: http.StatusTooManyRequests,
		StackTrace: captureStackTrace(),
	}
}

// NewUnavailableError creates a service unavailable error
func NewUnavailableError(service string) *AppError {
	return &AppError{
		Type:       ErrorTypeUnavailable,
		Message:    fmt.Sprintf("service '%s' is unavailable", service),
		HTTPStatus: http.StatusServiceUnavailable,
		StackTrace: captureStackTrace(),
	}
}

// NewDatabaseError creates a database error
func NewDatabaseError(operation string, err error) *AppError {
	return &AppError{
		Type:       ErrorTypeDatabase,
		Message:    fmt.Sprintf("database operation '%s' failed", operation),
		Cause:      err,
		HTTPStatus: http.StatusInternalServerError,
		StackTrace: captureStackTrace(),
	}
}

// NewNetworkError creates a network error
func NewNetworkError(message string, err error) *AppError {
	return &AppError{
		Type:       ErrorTypeNetwork,
		Message:    message,
		Cause:      err,
		HTTPStatus: http.StatusBadGateway,
		StackTrace: captureStackTrace(),
	}
}

// NewExternalError creates an external service error
func NewExternalError(service string, err error) *AppError {
	return &AppError{
		Type:       ErrorTypeExternal,
		Message:    fmt.Sprintf("external service '%s' error", service),
		Cause:      err,
		HTTPStatus: http.StatusBadGateway,
		StackTrace: captureStackTrace(),
	}
}

// Helper functions

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts AppError from an error chain
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// IsType checks if an error is of a specific type
func IsType(err error, errType ErrorType) bool {
	appErr := GetAppError(err)
	return appErr != nil && appErr.Type == errType
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return IsType(err, ErrorTypeNotFound)
}

// IsValidation checks if an error is a validation error
func IsValidation(err error) bool {
	return IsType(err, ErrorTypeValidation)
}

// IsUnauthorized checks if an error is an unauthorized error
func IsUnauthorized(err error) bool {
	return IsType(err, ErrorTypeUnauthorized)
}

// IsForbidden checks if an error is a forbidden error
func IsForbidden(err error) bool {
	return IsType(err, ErrorTypeForbidden)
}

// IsConflict checks if an error is a conflict error
func IsConflict(err error) bool {
	return IsType(err, ErrorTypeConflict)
}

// IsInternal checks if an error is an internal error
func IsInternal(err error) bool {
	return IsType(err, ErrorTypeInternal)
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	
	// If it's already an AppError, add context to message
	if appErr := GetAppError(err); appErr != nil {
		appErr.Message = fmt.Sprintf("%s: %s", message, appErr.Message)
		return appErr
	}
	
	// Otherwise create a new internal error
	return NewInternalError(message).WithCause(err)
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, format string, args ...interface{}) error {
	return Wrap(err, fmt.Sprintf(format, args...))
}