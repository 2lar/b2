package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "requestID"

// RequestID middleware generates or extracts request ID and adds it to context and response headers
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		
		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)
		
		// Continue to next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetRequestIDFromRequest extracts the request ID from the request context
func GetRequestIDFromRequest(r *http.Request) string {
	return GetRequestID(r.Context())
}