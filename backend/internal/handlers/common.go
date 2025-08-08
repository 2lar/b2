// Package handlers provides common functionality for HTTP handlers.
package handlers

import (
	"context"
	"log"
	"net/http"

	"brain2-backend/pkg/api"
	appErrors "brain2-backend/pkg/errors"

	"github.com/awslabs/aws-lambda-go-api-proxy/core"
)

// contextKey is used for context values
type contextKey struct {
	name string
}

var userIDKey = contextKey{"userID"}

// handleServiceError converts service errors to appropriate HTTP responses
func handleServiceError(w http.ResponseWriter, err error) {
	if appErrors.IsValidation(err) {
		api.Error(w, http.StatusBadRequest, err.Error())
	} else if appErrors.IsNotFound(err) {
		api.Error(w, http.StatusNotFound, err.Error())
	} else {
		log.Printf("INTERNAL ERROR: %v", err)
		api.Error(w, http.StatusInternalServerError, "An internal error occurred")
	}
}

// Authenticator middleware extracts user ID from Lambda authorizer context
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCtx, ok := core.GetAPIGatewayV2ContextFromContext(r.Context())
		if !ok {
			log.Println("Error: could not get proxy request context from context")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		userID, ok := proxyCtx.Authorizer.Lambda["sub"].(string)
		if !ok || userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
