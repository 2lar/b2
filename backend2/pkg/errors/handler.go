package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	
	"go.uber.org/zap"
)

// ErrorResponse represents the API error response format
type ErrorResponse struct {
	Error      bool                   `json:"error"`
	Type       string                 `json:"type"`
	Message    string                 `json:"message"`
	Code       string                 `json:"code,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
}

// ErrorHandler handles errors and sends appropriate HTTP responses
type ErrorHandler struct {
	logger        *zap.Logger
	debug         bool
	defaultStatus int
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *zap.Logger, debug bool) *ErrorHandler {
	return &ErrorHandler{
		logger:        logger,
		debug:         debug,
		defaultStatus: http.StatusInternalServerError,
	}
}

// Handle processes an error and sends an HTTP response
func (h *ErrorHandler) Handle(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	
	// Extract request/trace IDs from context if available
	requestID := r.Header.Get("X-Request-ID")
	traceID := r.Header.Get("X-Trace-ID")
	
	// Determine error type and status code
	var status int
	var response ErrorResponse
	
	if appErr := GetAppError(err); appErr != nil {
		// Handle application error
		status = appErr.HTTPStatus
		if status == 0 {
			status = h.defaultStatus
		}
		
		response = ErrorResponse{
			Error:     true,
			Type:      string(appErr.Type),
			Message:   appErr.Message,
			Code:      appErr.Code,
			Details:   appErr.Details,
			RequestID: requestID,
			TraceID:   traceID,
		}
		
		// Log the error
		h.logError(r, appErr, status)
		
		// Add stack trace in debug mode
		if h.debug && appErr.StackTrace != "" {
			if response.Details == nil {
				response.Details = make(map[string]interface{})
			}
			response.Details["stack_trace"] = appErr.StackTrace
		}
	} else {
		// Handle generic error
		status = h.defaultStatus
		response = ErrorResponse{
			Error:     true,
			Type:      string(ErrorTypeInternal),
			Message:   "An internal error occurred",
			RequestID: requestID,
			TraceID:   traceID,
		}
		
		// Log the error
		h.logger.Error("Unhandled error",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("request_id", requestID),
			zap.String("trace_id", traceID),
			zap.Int("status", status),
		)
		
		// Add error details in debug mode
		if h.debug {
			response.Message = err.Error()
		}
	}
	
	// Send response
	h.sendJSON(w, status, response)
}

// HandleStatus sends an error response with a specific status code
func (h *ErrorHandler) HandleStatus(w http.ResponseWriter, r *http.Request, status int, message string) {
	response := ErrorResponse{
		Error:     true,
		Type:      h.statusToErrorType(status),
		Message:   message,
		RequestID: r.Header.Get("X-Request-ID"),
		TraceID:   r.Header.Get("X-Trace-ID"),
	}
	
	h.logger.Warn("HTTP error",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status", status),
		zap.String("message", message),
	)
	
	h.sendJSON(w, status, response)
}

// logError logs an application error with appropriate level
func (h *ErrorHandler) logError(r *http.Request, err *AppError, status int) {
	fields := []zap.Field{
		zap.String("error_type", string(err.Type)),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status", status),
		zap.String("request_id", r.Header.Get("X-Request-ID")),
		zap.String("trace_id", r.Header.Get("X-Trace-ID")),
	}
	
	if err.Code != "" {
		fields = append(fields, zap.String("error_code", err.Code))
	}
	
	if err.Cause != nil {
		fields = append(fields, zap.Error(err.Cause))
	}
	
	if err.Details != nil {
		fields = append(fields, zap.Any("details", err.Details))
	}
	
	// Log based on error type and status
	switch {
	case status >= 500:
		h.logger.Error(err.Message, fields...)
	case status >= 400:
		h.logger.Warn(err.Message, fields...)
	default:
		h.logger.Info(err.Message, fields...)
	}
}

// sendJSON sends a JSON response
func (h *ErrorHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode error response",
			zap.Error(err),
			zap.Any("data", data),
		)
	}
}

// statusToErrorType maps HTTP status to error type
func (h *ErrorHandler) statusToErrorType(status int) string {
	switch status {
	case http.StatusBadRequest:
		return string(ErrorTypeValidation)
	case http.StatusUnauthorized:
		return string(ErrorTypeUnauthorized)
	case http.StatusForbidden:
		return string(ErrorTypeForbidden)
	case http.StatusNotFound:
		return string(ErrorTypeNotFound)
	case http.StatusConflict:
		return string(ErrorTypeConflict)
	case http.StatusRequestTimeout:
		return string(ErrorTypeTimeout)
	case http.StatusTooManyRequests:
		return string(ErrorTypeRateLimit)
	case http.StatusServiceUnavailable:
		return string(ErrorTypeUnavailable)
	case http.StatusBadGateway:
		return string(ErrorTypeExternal)
	default:
		return string(ErrorTypeInternal)
	}
}

// Middleware returns an HTTP middleware that handles panics and errors
func (h *ErrorHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Recover from panics
		defer func() {
			if rec := recover(); rec != nil {
				err := NewInternalError(fmt.Sprintf("panic: %v", rec))
				h.Handle(w, r, err)
			}
		}()
		
		// Call next handler
		next.ServeHTTP(w, r)
	})
}