// Package middleware provides reusable HTTP middleware components.
package middleware

import (
	"context"
	"log"
	"net/http"
	
	sharedContext "brain2-backend/internal/context"
	"brain2-backend/internal/errors"
	"brain2-backend/pkg/api"
	"go.uber.org/zap"
)

// HandlerMiddleware provides common functionality for HTTP handlers.
// This middleware implements cross-cutting concerns that apply to multiple handlers,
// following the Open/Closed Principle by allowing extension without modification.
type HandlerMiddleware struct{}

// NewHandlerMiddleware creates a new handler middleware.
func NewHandlerMiddleware() *HandlerMiddleware {
	return &HandlerMiddleware{}
}

// ServiceAvailabilityCheck creates middleware that checks if required services are available.
// This middleware implements the Circuit Breaker pattern for service availability.
type ServiceAvailabilityCheck struct {
	serviceName string
	checker     func() bool
}

// NewServiceAvailabilityCheck creates a new service availability check middleware.
func NewServiceAvailabilityCheck(serviceName string, checker func() bool) *ServiceAvailabilityCheck {
	return &ServiceAvailabilityCheck{
		serviceName: serviceName,
		checker:     checker,
	}
}

// Check performs the service availability check.
func (s *ServiceAvailabilityCheck) Check(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checker() {
			log.Printf("ERROR: %s service unavailable", s.serviceName)
			api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
			return
		}
		next(w, r)
	}
}

// UserIDExtractor extracts and validates user ID from request context.
// This middleware implements the Single Responsibility Principle by focusing
// solely on user authentication and authorization.
type UserIDExtractor struct{}

// NewUserIDExtractor creates a new user ID extractor.
func NewUserIDExtractor() *UserIDExtractor {
	return &UserIDExtractor{}
}

// Extract extracts user ID from request and adds it to context.
func (u *UserIDExtractor) Extract(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := sharedContext.GetUserIDFromContext(r.Context())
		if !ok {
			log.Printf("ERROR: Authentication failed, getUserID returned false")
			api.Error(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		
		// Add user ID to context for downstream handlers using shared context
		ctx := sharedContext.WithUserID(r.Context(), userID)
		next(w, r.WithContext(ctx))
	}
}

// GetUserIDFromContext extracts user ID from request context.
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	// Use the shared context utility for consistency
	return sharedContext.GetUserIDFromContext(ctx)
}

// ErrorHandler provides consistent error handling across handlers.
// This middleware implements the Strategy Pattern for different error handling approaches.
type ErrorHandler struct{}

// NewErrorHandler creates a new error handler.
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

// HandleServiceError handles service layer errors and converts them to HTTP responses.
func (e *ErrorHandler) HandleServiceError(w http.ResponseWriter, err error) {
	// Use the unified error system directly for consistent error handling
	// Create a logger for error handling
	logger, _ := zap.NewProduction()
	if logger == nil {
		logger = zap.NewNop()
	}
	errors.WriteHTTPError(w, err, logger)
}

// HandlerLoggingHelper provides request logging functionality specific to handlers.
// This is different from the pipeline LoggingMiddleware and focuses on handler-specific logging.
type HandlerLoggingHelper struct{}

// NewHandlerLoggingHelper creates a new handler logging helper.
func NewHandlerLoggingHelper() *HandlerLoggingHelper {
	return &HandlerLoggingHelper{}
}

// LogHandlerCall logs handler-specific details for debugging and monitoring.
func (l *HandlerLoggingHelper) LogHandlerCall(handlerName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := GetUserIDFromContext(r.Context())
		log.Printf("DEBUG: %s called for userID: %s", handlerName, userID)
		next(w, r)
	}
}