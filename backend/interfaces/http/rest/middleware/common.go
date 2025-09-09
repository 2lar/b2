package middleware

import (
	"github.com/google/uuid"
	"go.uber.org/zap"
	"net/http"
)

// Logging creates a simple logging middleware
func Logging() func(next http.Handler) http.Handler {
	// Create a basic logger
	logger, _ := zap.NewProduction()
	return Logger(logger)
}

// CORS adds CORS headers to responses
func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestID adds a unique request ID to each request
func RequestID() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request ID exists in header
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				// Generate new request ID
				requestID = uuid.New().String()
			}

			// Add request ID to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to context for logging
			ctx := r.Context()
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
