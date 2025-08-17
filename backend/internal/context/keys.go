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