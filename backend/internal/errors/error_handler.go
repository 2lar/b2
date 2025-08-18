// Package errors provides unified error handling for HTTP responses and logging.
package errors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"brain2-backend/pkg/api"
	"go.uber.org/zap"
)

// ============================================================================
// UNIFIED ERROR HANDLER
// ============================================================================

// ErrorHandler provides unified error handling across all application layers.
// This handler replaces the multiple error handling functions found in the codebase
// and provides consistent error processing, logging, and HTTP response generation.
type ErrorHandler struct {
	logger        *zap.Logger
	enableDebug   bool
	enableMetrics bool
	metricsClient MetricsClient
}

// MetricsClient defines the interface for error metrics collection.
type MetricsClient interface {
	IncrementCounter(name string, tags map[string]string)
	RecordDuration(name string, duration time.Duration, tags map[string]string)
}

// ErrorHandlerConfig contains configuration for the error handler.
type ErrorHandlerConfig struct {
	Logger        *zap.Logger
	EnableDebug   bool     // Include debug information in responses
	EnableMetrics bool     // Collect error metrics
	MetricsClient MetricsClient
}

// NewErrorHandler creates a new unified error handler.
func NewErrorHandler(config ErrorHandlerConfig) *ErrorHandler {
	return &ErrorHandler{
		logger:        config.Logger,
		enableDebug:   config.EnableDebug,
		enableMetrics: config.EnableMetrics,
		metricsClient: config.MetricsClient,
	}
}

// ============================================================================
// HTTP ERROR HANDLING
// ============================================================================

// HandleHTTPError processes an error and writes an appropriate HTTP response.
// This method consolidates all the error handling logic from different handlers.
func (h *ErrorHandler) HandleHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	
	// Convert to UnifiedError if necessary
	unifiedErr := h.ensureUnifiedError(err)
	
	// Add request context if missing
	unifiedErr = h.addRequestContext(unifiedErr, r)
	
	// Log the error
	h.logError(unifiedErr)
	
	// Collect metrics
	h.collectMetrics(unifiedErr)
	
	// Write HTTP response
	h.writeHTTPResponse(w, unifiedErr)
}

// ensureUnifiedError converts any error to a UnifiedError.
func (h *ErrorHandler) ensureUnifiedError(err error) *UnifiedError {
	var unifiedErr *UnifiedError
	if !errors.As(err, &unifiedErr) {
		// Convert legacy errors
		unifiedErr = FromLegacyError(err)
	}
	return unifiedErr
}

// addRequestContext adds request-specific context to the error.
func (h *ErrorHandler) addRequestContext(err *UnifiedError, r *http.Request) *UnifiedError {
	if r == nil {
		return err
	}
	
	// Add request ID if available
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" && err.RequestID == "" {
		err.RequestID = requestID
	}
	
	// Add user ID from context if available
	if userID := getUserIDFromContext(r.Context()); userID != "" && err.UserID == "" {
		err.UserID = userID
	}
	
	// Add operation from request path if missing
	if err.Operation == "" {
		err.Operation = r.Method + " " + r.URL.Path
	}
	
	return err
}

// logError logs the error with appropriate level and context.
func (h *ErrorHandler) logError(err *UnifiedError) {
	if h.logger == nil {
		return
	}
	
	fields := []zap.Field{
		zap.String("error_type", string(err.Type)),
		zap.String("error_code", err.Code),
		zap.String("error_message", err.Message),
		zap.String("severity", string(err.Severity)),
		zap.Bool("retryable", err.Retryable),
	}
	
	// Add context fields if available
	if err.Operation != "" {
		fields = append(fields, zap.String("operation", err.Operation))
	}
	if err.Resource != "" {
		fields = append(fields, zap.String("resource", err.Resource))
	}
	if err.UserID != "" {
		fields = append(fields, zap.String("user_id", err.UserID))
	}
	if err.RequestID != "" {
		fields = append(fields, zap.String("request_id", err.RequestID))
	}
	
	// Add cause if available
	if err.Cause != nil {
		fields = append(fields, zap.NamedError("cause", err.Cause))
	}
	
	// Add debug information if enabled
	if h.enableDebug && err.File != "" && err.Line > 0 {
		fields = append(fields, zap.String("file", err.File))
		fields = append(fields, zap.Int("line", err.Line))
	}
	
	// Log at appropriate level based on severity
	message := "Error occurred"
	switch err.Severity {
	case SeverityLow:
		h.logger.Info(message, fields...)
	case SeverityMedium:
		h.logger.Warn(message, fields...)
	case SeverityHigh:
		h.logger.Error(message, fields...)
	case SeverityCritical:
		h.logger.Error(message, fields...)
		// Could trigger alerts here
	}
}

// collectMetrics collects error metrics for monitoring.
func (h *ErrorHandler) collectMetrics(err *UnifiedError) {
	if !h.enableMetrics || h.metricsClient == nil {
		return
	}
	
	tags := map[string]string{
		"error_type": string(err.Type),
		"error_code": err.Code,
		"severity":   string(err.Severity),
		"retryable":  fmt.Sprintf("%t", err.Retryable),
	}
	
	if err.Operation != "" {
		tags["operation"] = err.Operation
	}
	
	h.metricsClient.IncrementCounter("errors_total", tags)
}

// writeHTTPResponse writes the appropriate HTTP response for the error.
func (h *ErrorHandler) writeHTTPResponse(w http.ResponseWriter, err *UnifiedError) {
	statusCode := h.mapErrorTypeToHTTPStatus(err.Type)
	message := h.getClientMessage(err)
	
	// Include debug information if enabled and it's an internal error
	if h.enableDebug && err.Type == ErrorTypeInternal {
		response := map[string]interface{}{
			"error":   message,
			"code":    err.Code,
			"details": err.Details,
		}
		if err.RequestID != "" {
			response["requestId"] = err.RequestID
		}
		api.ErrorWithData(w, statusCode, message, response)
	} else {
		api.Error(w, statusCode, message)
	}
}

// mapErrorTypeToHTTPStatus maps error types to HTTP status codes.
func (h *ErrorHandler) mapErrorTypeToHTTPStatus(errType ErrorType) int {
	switch errType {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case ErrorTypeForbidden:
		return http.StatusForbidden
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorTypeConnection:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	case ErrorTypeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// getClientMessage returns an appropriate message for the client.
func (h *ErrorHandler) getClientMessage(err *UnifiedError) string {
	// For validation and not found errors, use the actual message
	switch err.Type {
	case ErrorTypeValidation, ErrorTypeNotFound, ErrorTypeUnauthorized, ErrorTypeForbidden:
		return err.Message
	case ErrorTypeConflict:
		if err.Message != "" {
			return err.Message
		}
		return "The resource has been modified by another request. Please retry with the latest version."
	case ErrorTypeTimeout:
		return "The request timed out. Please try again."
	case ErrorTypeRateLimit:
		return "Too many requests. Please slow down."
	case ErrorTypeUnavailable, ErrorTypeConnection:
		return "Service temporarily unavailable. Please try again later."
	case ErrorTypeExternal:
		return "External service error. Please try again later."
	case ErrorTypeInternal:
		return "An internal error occurred. Please contact support if the problem persists."
	default:
		return "An error occurred. Please try again."
	}
}

// ============================================================================
// SERVICE LAYER ERROR HANDLING
// ============================================================================

// HandleServiceError processes errors from the service layer and adds appropriate context.
func (h *ErrorHandler) HandleServiceError(operation, resource string, err error) error {
	if err == nil {
		return nil
	}
	
	// If it's already a UnifiedError with operation context, return as-is
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) && unifiedErr.Operation != "" {
		return err
	}
	
	// Wrap the error with additional context
	return Wrap(err, operation, fmt.Sprintf("Failed to %s %s", operation, resource))
}

// ============================================================================
// REPOSITORY LAYER ERROR HANDLING
// ============================================================================

// HandleRepositoryError processes errors from the repository layer.
func (h *ErrorHandler) HandleRepositoryError(operation, resource string, err error) error {
	if err == nil {
		return nil
	}
	
	// Map common repository errors to appropriate types
	errMsg := err.Error()
	
	switch {
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "item not found"):
		return NotFound("RESOURCE_NOT_FOUND", fmt.Sprintf("%s not found", resource)).
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
			
	case strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate"):
		return Conflict("RESOURCE_EXISTS", fmt.Sprintf("%s already exists", resource)).
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
			
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "context deadline exceeded"):
		return Timeout("REPOSITORY_TIMEOUT", "Database operation timed out").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
			
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
		return Connection("DATABASE_CONNECTION", "Database connection failed").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
			
	default:
		return Internal("REPOSITORY_ERROR", "Database operation failed").
			WithOperation(operation).
			WithResource(resource).
			WithCause(err).
			Build()
	}
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// getUserIDFromContext extracts user ID from request context.
func getUserIDFromContext(ctx context.Context) string {
	if userID := ctx.Value("userID"); userID != nil {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}
	return ""
}

// ============================================================================
// MIGRATION HELPERS
// ============================================================================

// CreateCompatibilityHandler creates a handler that's compatible with existing error handling patterns.
func CreateCompatibilityHandler(logger *zap.Logger) func(http.ResponseWriter, error) {
	handler := NewErrorHandler(ErrorHandlerConfig{
		Logger:      logger,
		EnableDebug: false, // Disable debug for production compatibility
	})
	
	return func(w http.ResponseWriter, err error) {
		handler.HandleHTTPError(w, nil, err)
	}
}