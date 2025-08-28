// Package handlers provides common functionality for HTTP handlers.
package handlers

import (
	"context"
	"net/http"
	"strings"

	"brain2-backend/pkg/api"
	"brain2-backend/internal/errors"
	sharedContext "brain2-backend/internal/context"

	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"go.uber.org/zap"
)

// getUserID safely extracts userID from context
func getUserID(r *http.Request) (string, bool) {
	return sharedContext.GetUserIDFromContext(r.Context())
}

// Global logger for handlers - should be injected via dependency injection in production
var logger *zap.Logger

func init() {
	// Create a default logger - in production this should be injected
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		// Fallback to a no-op logger if production logger fails
		logger = zap.NewNop()
	}
}

// SetLogger allows setting a custom logger for the handlers
func SetLogger(l *zap.Logger) {
	if l != nil {
		logger = l
	}
}

// handleServiceError converts service errors to appropriate HTTP responses
// This now uses the unified error system for consistent error handling
func handleServiceError(w http.ResponseWriter, err error) {
	// Add API version to error responses
	if w.Header().Get("X-API-Version") == "" {
		w.Header().Set("X-API-Version", "1")
	}
	
	// Use unified error system's WriteHTTPError which handles all error types
	// It automatically determines the correct HTTP status code and formats the response
	errors.WriteHTTPError(w, err, logger)
}

// isTimeoutError checks if the error is related to timeouts
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "context deadline exceeded") ||
		   strings.Contains(errStr, "i/o timeout")
}

// isConnectionError checks if the error is related to connection issues
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

// Authenticator middleware extracts user ID from Lambda authorizer context
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCtx, ok := core.GetAPIGatewayV2ContextFromContext(r.Context())
		if !ok {
			// Could not get proxy request context
			api.Error(w, http.StatusInternalServerError, "Authentication context not available")
			return
		}

		if proxyCtx.Authorizer == nil || proxyCtx.Authorizer.Lambda == nil {
			// Missing authorizer context
			api.Error(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		subValue := proxyCtx.Authorizer.Lambda["sub"]
		if subValue == nil {
			// Missing user ID in authorizer context
			api.Error(w, http.StatusUnauthorized, "Invalid authentication")
			return
		}
		
		userID, ok := subValue.(string)
		if !ok || userID == "" {
			// Invalid user ID in authorizer context
			api.Error(w, http.StatusUnauthorized, "Invalid authentication")
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, sharedContext.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
