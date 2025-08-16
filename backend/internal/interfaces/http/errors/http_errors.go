// Package errors provides structured HTTP error handling following RFC 7807 (Problem Details).
// This package demonstrates best practices for API error responses that are both
// machine-readable and human-friendly.
//
// Key Concepts Illustrated:
//   - RFC 7807 Problem Details: Standardized error format
//   - Error Classification: Mapping domain errors to HTTP status codes
//   - Security: Hiding sensitive information in production
//   - Correlation: Request IDs for tracing errors
//   - Internationalization: Support for error code lookup
//
// Design Principles:
//   - Consistent error format across all endpoints
//   - Clear separation between client and server errors
//   - Detailed errors in development, generic in production
//   - Structured logging for debugging
//   - No sensitive data leakage
package errors

import (
	"brain2-backend/internal/domain"
	"brain2-backend/internal/interfaces/http/dto"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// HTTPError represents a structured HTTP error response following RFC 7807
type HTTPError struct {
	// RFC 7807 standard fields
	Type     string                 `json:"type"`               // URI reference that identifies the problem type
	Title    string                 `json:"title"`              // Short, human-readable summary
	Status   int                    `json:"status"`             // HTTP status code
	Detail   string                 `json:"detail,omitempty"`   // Human-readable explanation
	Instance string                 `json:"instance,omitempty"` // URI reference for this occurrence

	// Extensions for better debugging and UX
	Code      string                 `json:"code,omitempty"`       // Application-specific error code
	RequestID string                 `json:"request_id,omitempty"` // Correlation ID for tracing
	Timestamp string                 `json:"timestamp"`             // When the error occurred
	Path      string                 `json:"path,omitempty"`       // Request path that caused the error
	Method    string                 `json:"method,omitempty"`     // HTTP method used
	Fields    map[string]interface{} `json:"fields,omitempty"`     // Field-specific errors for validation

	// Internal fields (not serialized)
	internal      error  `json:"-"` // Original error for logging
	stackTrace    string `json:"-"` // Stack trace for debugging
	isProduction  bool   `json:"-"` // Whether to hide sensitive details
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return e.Title
}

// Unwrap allows errors.Is and errors.As to work
func (e *HTTPError) Unwrap() error {
	return e.internal
}

// WithInternal adds the original error for logging
func (e *HTTPError) WithInternal(err error) *HTTPError {
	e.internal = err
	return e
}

// WithRequestID adds correlation ID
func (e *HTTPError) WithRequestID(id string) *HTTPError {
	e.RequestID = id
	return e
}

// WithPath adds the request path
func (e *HTTPError) WithPath(path, method string) *HTTPError {
	e.Path = path
	e.Method = method
	return e
}

// WithFields adds field-specific errors (for validation)
func (e *HTTPError) WithFields(fields map[string]interface{}) *HTTPError {
	e.Fields = fields
	return e
}

// WithStackTrace captures the current stack trace
func (e *HTTPError) WithStackTrace() *HTTPError {
	e.stackTrace = string(debug.Stack())
	return e
}

// MarshalJSON customizes JSON serialization based on environment
func (e *HTTPError) MarshalJSON() ([]byte, error) {
	type Alias HTTPError
	aux := (*Alias)(e)

	// In production, hide sensitive details
	if e.isProduction {
		if e.Status >= 500 {
			// Don't expose internal error details in production
			aux.Detail = "An internal error occurred. Please try again later."
			aux.Fields = nil
		}
	}

	return json.Marshal(aux)
}

// Write sends the error response to the client
func (e *HTTPError) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(e.Status)
	json.NewEncoder(w).Encode(e)
}

// Common HTTP error constructors

// NewBadRequest creates a 400 Bad Request error
func NewBadRequest(detail string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/bad-request",
		Title:     "Bad Request",
		Status:    http.StatusBadRequest,
		Detail:    detail,
		Code:      "BAD_REQUEST",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewValidationError creates a 400 error with field-level details
func NewValidationError(validationErr error) *HTTPError {
	httpErr := &HTTPError{
		Type:      "/errors/validation",
		Title:     "Validation Failed",
		Status:    http.StatusBadRequest,
		Code:      "VALIDATION_ERROR",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Handle dto.ValidationErrors specially
	var valErrors dto.ValidationErrors
	if errors.As(validationErr, &valErrors) {
		fields := make(map[string]interface{})
		var details []string

		for _, err := range valErrors.Errors {
			fields[err.Field] = err.Message
			details = append(details, fmt.Sprintf("%s: %s", err.Field, err.Message))
		}

		httpErr.Fields = fields
		httpErr.Detail = "Validation failed for one or more fields: " + strings.Join(details, "; ")
	} else {
		httpErr.Detail = validationErr.Error()
	}

	return httpErr.WithInternal(validationErr)
}

// NewUnauthorized creates a 401 Unauthorized error
func NewUnauthorized(detail string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/unauthorized",
		Title:     "Unauthorized",
		Status:    http.StatusUnauthorized,
		Detail:    detail,
		Code:      "UNAUTHORIZED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewForbidden creates a 403 Forbidden error
func NewForbidden(detail string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/forbidden",
		Title:     "Forbidden",
		Status:    http.StatusForbidden,
		Detail:    detail,
		Code:      "FORBIDDEN",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewNotFound creates a 404 Not Found error
func NewNotFound(resource string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/not-found",
		Title:     "Resource Not Found",
		Status:    http.StatusNotFound,
		Detail:    fmt.Sprintf("The requested %s was not found", resource),
		Code:      "NOT_FOUND",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewMethodNotAllowed creates a 405 Method Not Allowed error
func NewMethodNotAllowed(method string, allowed []string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/method-not-allowed",
		Title:     "Method Not Allowed",
		Status:    http.StatusMethodNotAllowed,
		Detail:    fmt.Sprintf("Method %s is not allowed. Allowed methods: %s", method, strings.Join(allowed, ", ")),
		Code:      "METHOD_NOT_ALLOWED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewConflict creates a 409 Conflict error
func NewConflict(detail string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/conflict",
		Title:     "Conflict",
		Status:    http.StatusConflict,
		Detail:    detail,
		Code:      "CONFLICT",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewTooManyRequests creates a 429 Too Many Requests error
func NewTooManyRequests(retryAfter int) *HTTPError {
	return &HTTPError{
		Type:      "/errors/rate-limit",
		Title:     "Too Many Requests",
		Status:    http.StatusTooManyRequests,
		Detail:    fmt.Sprintf("Rate limit exceeded. Please retry after %d seconds", retryAfter),
		Code:      "RATE_LIMIT_EXCEEDED",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Fields: map[string]interface{}{
			"retry_after": retryAfter,
		},
	}
}

// NewInternalServerError creates a 500 Internal Server Error
func NewInternalServerError() *HTTPError {
	err := &HTTPError{
		Type:      "/errors/internal",
		Title:     "Internal Server Error",
		Status:    http.StatusInternalServerError,
		Detail:    "An unexpected error occurred",
		Code:      "INTERNAL_ERROR",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	err.WithStackTrace()
	return err
}

// NewServiceUnavailable creates a 503 Service Unavailable error
func NewServiceUnavailable(detail string) *HTTPError {
	return &HTTPError{
		Type:      "/errors/service-unavailable",
		Title:     "Service Unavailable",
		Status:    http.StatusServiceUnavailable,
		Detail:    detail,
		Code:      "SERVICE_UNAVAILABLE",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// FromError converts various error types to HTTPError
func FromError(err error, isProduction bool) *HTTPError {
	if err == nil {
		return nil
	}

	// Check if it's already an HTTPError
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		httpErr.isProduction = isProduction
		return httpErr
	}

	// Handle validation errors from DTOs
	var valErrors dto.ValidationErrors
	if errors.As(err, &valErrors) {
		return NewValidationError(err)
	}

	// Handle application errors
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		switch appErr.Type {
		case appErrors.ErrorTypeValidation:
			return NewBadRequest(appErr.Message).WithInternal(appErr.Err)
		case appErrors.ErrorTypeNotFound:
			return NewNotFound("resource").WithInternal(appErr.Err)
		case appErrors.ErrorTypeUnauthorized:
			return NewUnauthorized(appErr.Message).WithInternal(appErr.Err)
		default:
			httpErr := NewInternalServerError().WithInternal(err)
			httpErr.isProduction = isProduction
			return httpErr
		}
	}

	// Handle domain errors
	if errors.Is(err, domain.ErrNotFound) {
		return NewNotFound("resource")
	}
	if errors.Is(err, domain.ErrValidation) {
		return NewBadRequest(err.Error())
	}
	if errors.Is(err, domain.ErrUnauthorized) {
		return NewUnauthorized("Authentication required")
	}
	if errors.Is(err, domain.ErrConflict) {
		return NewConflict("Resource conflict detected")
	}

	// Handle repository errors
	if repository.IsConflict(err) {
		return NewConflict("The resource has been modified. Please refresh and try again.")
	}
	if repository.IsValidationError(err) {
		return NewBadRequest(err.Error())
	}

	// Handle timeout errors
	if isTimeoutError(err) {
		return NewServiceUnavailable("Request timed out. Please try again.")
	}

	// Handle connection errors
	if isConnectionError(err) {
		return NewServiceUnavailable("Service temporarily unavailable. Please try again later.")
	}

	// Default to internal server error
	httpErr = NewInternalServerError()
	httpErr.WithInternal(err)
	httpErr.isProduction = isProduction
	return httpErr
}

// Helper functions

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "i/o timeout")
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable")
}

// ErrorHandler is a middleware that handles panics and converts them to proper error responses
type ErrorHandler struct {
	IsProduction bool
}

// Handle wraps an http.Handler and handles any panics
func (h *ErrorHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				switch t := rec.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = fmt.Errorf("panic: %v", t)
				}

				httpErr := NewInternalServerError().
					WithInternal(err).
					WithStackTrace().
					WithPath(r.URL.Path, r.Method)

				// Get request ID if available
				if reqID := r.Context().Value("request_id"); reqID != nil {
					if id, ok := reqID.(string); ok {
						httpErr.WithRequestID(id)
					}
				}

				httpErr.isProduction = h.IsProduction
				httpErr.Write(w)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// WriteError is a helper function to write errors consistently
func WriteError(w http.ResponseWriter, r *http.Request, err error, isProduction bool) {
	httpErr := FromError(err, isProduction)
	
	// Add request context
	httpErr.WithPath(r.URL.Path, r.Method)
	
	// Add request ID if available
	if reqID := r.Context().Value("request_id"); reqID != nil {
		if id, ok := reqID.(string); ok {
			httpErr.WithRequestID(id)
		}
	}
	
	httpErr.Write(w)
}