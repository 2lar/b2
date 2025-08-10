package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"brain2-backend/pkg/api"
)

// Timeout middleware wraps requests with a timeout context
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			// Create a channel to signal completion
			done := make(chan struct{})
			
			// Create a new request with the timeout context
			r = r.WithContext(ctx)
			
			go func() {
				defer func() {
					if err := recover(); err != nil {
						// Handle panic in goroutine
						requestID := GetRequestIDFromRequest(r)
						log.Printf("PANIC in timeout handler [Request ID: %s]: %v", requestID, err)
					}
				}()
				
				next.ServeHTTP(w, r)
				close(done)
			}()
			
			select {
			case <-done:
				// Request completed normally
				return
			case <-ctx.Done():
				// Request timed out
				requestID := GetRequestIDFromRequest(r)
				log.Printf("Request timeout [Request ID: %s]: %v", requestID, ctx.Err())
				
				// Check if we can still write to response
				if w.Header().Get("Content-Type") == "" {
					api.Error(w, http.StatusRequestTimeout, "Request timeout")
				}
				return
			}
		})
	}
}

// TimeoutWithCustomHandler allows custom handling of timeouts
func TimeoutWithCustomHandler(timeout time.Duration, handler func(w http.ResponseWriter, r *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			// Create a channel to signal completion
			done := make(chan struct{})
			
			// Create a new request with the timeout context
			r = r.WithContext(ctx)
			
			go func() {
				defer func() {
					if err := recover(); err != nil {
						// Handle panic in goroutine
						requestID := GetRequestIDFromRequest(r)
						log.Printf("PANIC in timeout handler [Request ID: %s]: %v", requestID, err)
					}
				}()
				
				next.ServeHTTP(w, r)
				close(done)
			}()
			
			select {
			case <-done:
				// Request completed normally
				return
			case <-ctx.Done():
				// Request timed out - call custom handler
				handler(w, r)
				return
			}
		})
	}
}

// DefaultTimeoutHandler is a default timeout handler
func DefaultTimeoutHandler(w http.ResponseWriter, r *http.Request) {
	requestID := GetRequestIDFromRequest(r)
	log.Printf("Request timeout [Request ID: %s]", requestID)
	
	if w.Header().Get("Content-Type") == "" {
		api.Error(w, http.StatusRequestTimeout, 
			"Request timeout - Request ID: "+requestID)
	}
}