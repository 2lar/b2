// Package errors provides HTTP middleware for error enrichment and handling.
package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
	
	sharedContext "brain2-backend/internal/context"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// HTTPErrorResponse represents the standardized error response structure
type HTTPErrorResponse struct {
	Error      HTTPErrorDetails `json:"error"`
	RequestID  string           `json:"request_id,omitempty"`
	Timestamp  string           `json:"timestamp"`
	APIVersion string           `json:"api_version,omitempty"`
}

// HTTPErrorDetails contains the error details
type HTTPErrorDetails struct {
	Type       string                 `json:"type"`
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Resource   string                 `json:"resource,omitempty"`
	Field      string                 `json:"field,omitempty"`
	Retryable  bool                   `json:"retryable,omitempty"`
	RetryAfter int                    `json:"retry_after,omitempty"` // seconds
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// errorCapturingResponseWriter wraps http.ResponseWriter to capture errors
type errorCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
	error      error
}

func (w *errorCapturingResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *errorCapturingResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// ErrorEnrichmentMiddleware enriches errors with context and handles panics
func ErrorEnrichmentMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap response writer
			wrapped := &errorCapturingResponseWriter{
				ResponseWriter: w,
				statusCode:     200,
			}
			
			// Recover from panics
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					stackTrace := string(debug.Stack())
					requestID := middleware.GetReqID(r.Context())
					userID, _ := sharedContext.GetUserIDFromContext(r.Context())
					
					logger.Error("Panic recovered",
						zap.String("request_id", requestID),
						zap.String("user_id", userID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("panic", err),
						zap.String("stack_trace", stackTrace),
					)
					
					// Create internal error
					appErr := Internal("PANIC_RECOVERED", "An unexpected error occurred").
						WithOperation(fmt.Sprintf("%s %s", r.Method, r.URL.Path)).
						WithRequestID(requestID).
						WithUserID(userID).
						Build()
					
					// Write error response
					WriteHTTPError(w, appErr, logger)
				}
			}()
			
			// Add correlation ID if not present
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = middleware.GetReqID(r.Context())
			}
			w.Header().Set("X-Correlation-ID", correlationID)
			
			// Process request
			next.ServeHTTP(wrapped, r)
		})
	}
}

// WriteHTTPError writes a standardized error response
func WriteHTTPError(w http.ResponseWriter, err error, logger *zap.Logger) {
	if err == nil {
		return
	}
	
	// Extract or create UnifiedError
	var unifiedErr *UnifiedError
	if !errors.As(err, &unifiedErr) {
		// Convert to UnifiedError if it's not already
		unifiedErr = FromLegacyError(err)
	}
	
	// Get request context from response writer if available
	ctx := context.Background()
	if rw, ok := w.(*errorCapturingResponseWriter); ok && rw.ResponseWriter != nil {
		// Try to extract context from the original request
		ctx = context.WithValue(ctx, "request_id", middleware.GetReqID(ctx))
	}
	
	// Determine HTTP status code
	statusCode := getHTTPStatusCode(unifiedErr)
	
	// Build error response
	response := HTTPErrorResponse{
		Error: HTTPErrorDetails{
			Type:      string(unifiedErr.Type),
			Code:      unifiedErr.Code,
			Message:   unifiedErr.Message,
			Details:   unifiedErr.Details,
			Resource:  unifiedErr.Resource,
			Retryable: unifiedErr.Retryable,
		},
		RequestID:  unifiedErr.RequestID,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		APIVersion: w.Header().Get("X-API-Version"),
	}
	
	// Add retry-after header if applicable
	if unifiedErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(unifiedErr.RetryAfter.Seconds())))
		response.Error.RetryAfter = int(unifiedErr.RetryAfter.Seconds())
	}
	
	// Add metadata if present
	if len(unifiedErr.RecoveryMetadata) > 0 {
		response.Error.Metadata = unifiedErr.RecoveryMetadata
	}
	
	// Log the error
	logLevel := getLogLevel(unifiedErr.Severity)
	logger.Log(logLevel,
		"HTTP error response",
		zap.String("error_type", string(unifiedErr.Type)),
		zap.String("error_code", unifiedErr.Code),
		zap.String("message", unifiedErr.Message),
		zap.String("request_id", unifiedErr.RequestID),
		zap.String("user_id", unifiedErr.UserID),
		zap.Int("status_code", statusCode),
		zap.Bool("retryable", unifiedErr.Retryable),
		zap.Error(unifiedErr.Cause),
	)
	
	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode error response",
			zap.Error(err),
			zap.String("request_id", unifiedErr.RequestID),
		)
	}
}

// getHTTPStatusCode determines the HTTP status code for an error
func getHTTPStatusCode(err *UnifiedError) int {
	// Check if error code has a specific status code
	if err.Code != "" {
		if code := ErrorCode(err.Code).HTTPStatusCode(); code != 500 {
			return code
		}
	}
	
	// Fall back to error type mapping
	switch err.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case ErrorTypeForbidden:
		return http.StatusForbidden
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeUnavailable, ErrorTypeConnection:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// getLogLevel converts error severity to zap log level
func getLogLevel(severity ErrorSeverity) zapcore.Level {
	switch severity {
	case SeverityCritical:
		return zapcore.ErrorLevel
	case SeverityHigh:
		return zapcore.ErrorLevel
	case SeverityMedium:
		return zapcore.WarnLevel
	case SeverityLow:
		return zapcore.InfoLevel
	default:
		return zapcore.WarnLevel
	}
}

// ExtractErrorFromContext extracts error information from context
func ExtractErrorFromContext(ctx context.Context) (*UnifiedError, bool) {
	if err, ok := ctx.Value("error").(*UnifiedError); ok {
		return err, true
	}
	return nil, false
}

// WithErrorContext adds error to context
func WithErrorContext(ctx context.Context, err *UnifiedError) context.Context {
	return context.WithValue(ctx, "error", err)
}

// ErrorHandlerFunc is a convenience function for handling errors in HTTP handlers
func ErrorHandlerFunc(logger *zap.Logger) func(w http.ResponseWriter, err error) {
	return func(w http.ResponseWriter, err error) {
		WriteHTTPError(w, err, logger)
	}
}