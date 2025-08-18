// Package handlers provides common functionality for HTTP handlers.
package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"brain2-backend/internal/repository"
	"brain2-backend/pkg/api"
	appErrors "brain2-backend/pkg/errors"
	sharedContext "brain2-backend/internal/context"

	"github.com/awslabs/aws-lambda-go-api-proxy/core"
)

// getUserID safely extracts userID from context
func getUserID(r *http.Request) (string, bool) {
	return sharedContext.GetUserIDFromContext(r.Context())
}

// handleServiceError converts service errors to appropriate HTTP responses
func handleServiceError(w http.ResponseWriter, err error) {
	if appErrors.IsValidation(err) {
		log.Printf("VALIDATION ERROR: %v", err)
		api.Error(w, http.StatusBadRequest, err.Error())
	} else if appErrors.IsNotFound(err) {
		log.Printf("NOT FOUND ERROR: %v", err)
		api.Error(w, http.StatusNotFound, err.Error())
	} else if repository.IsConflict(err) {
		log.Printf("CONFLICT ERROR: %v", err)
		api.Error(w, http.StatusConflict, "The resource has been modified by another request. Please retry with the latest version.")
	} else if isTimeoutError(err) {
		log.Printf("TIMEOUT ERROR: %v", err)
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable")
	} else if isConnectionError(err) {
		log.Printf("CONNECTION ERROR: %v", err)
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable")
	} else {
		// Log the full error details for debugging while hiding sensitive info from client
		log.Printf("INTERNAL ERROR (full): %+v", err)
		log.Printf("INTERNAL ERROR (type): %T", err)
		api.Error(w, http.StatusInternalServerError, "An internal error occurred")
	}
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
			log.Println("Error: could not get proxy request context from context")
			api.Error(w, http.StatusInternalServerError, "Authentication context not available")
			return
		}

		if proxyCtx.Authorizer == nil || proxyCtx.Authorizer.Lambda == nil {
			log.Println("Error: missing authorizer context")
			api.Error(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		subValue := proxyCtx.Authorizer.Lambda["sub"]
		if subValue == nil {
			log.Println("Error: missing user ID in authorizer context")
			api.Error(w, http.StatusUnauthorized, "Invalid authentication")
			return
		}
		
		userID, ok := subValue.(string)
		if !ok || userID == "" {
			log.Printf("Error: invalid user ID in authorizer context - expected string, got %T", subValue)
			api.Error(w, http.StatusUnauthorized, "Invalid authentication")
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, sharedContext.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
