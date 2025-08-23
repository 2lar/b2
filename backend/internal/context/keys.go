// Package context provides shared context utilities
package context

import (
	"context"
)

// contextKey is used for context values
type contextKey struct {
	name string
}

// UserIDKey is the key used to store userID in context
var UserIDKey = contextKey{"userID"}

// RequestIDKey is the key used to store requestID in context for tracing
var RequestIDKey = contextKey{"requestID"}

// GetUserIDFromContext extracts userID from context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userIDVal := ctx.Value(UserIDKey)
	if userIDVal == nil {
		return "", false
	}
	userID, ok := userIDVal.(string)
	return userID, ok && userID != ""
}

// WithUserID adds userID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetRequestIDFromContext extracts requestID from context for tracing
func GetRequestIDFromContext(ctx context.Context) (string, bool) {
	requestIDVal := ctx.Value(RequestIDKey)
	if requestIDVal == nil {
		return "", false
	}
	requestID, ok := requestIDVal.(string)
	return requestID, ok && requestID != ""
}

// WithRequestID adds requestID to context for tracing purposes
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}