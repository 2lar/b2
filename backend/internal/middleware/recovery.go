package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"brain2-backend/pkg/api"
)

// Recovery middleware handles panics and converts them to proper HTTP error responses
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Get request ID for correlation if available
				requestID := GetRequestIDFromRequest(r)
				
				// Log the panic with stack trace
				log.Printf("PANIC [Request ID: %s]: %v\nStack trace:\n%s", 
					requestID, err, string(debug.Stack()))
				
				// Check if response has already been written
				if w.Header().Get("Content-Type") == "" {
					// Response hasn't been written yet, we can send our error response
					api.Error(w, http.StatusInternalServerError, "Internal server error")
				}
				
				// If response was already partially written, there's nothing we can do
				// The connection will be closed by the HTTP server
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// RecoveryWithHandler allows custom handling of panics
func RecoveryWithHandler(handler func(w http.ResponseWriter, r *http.Request, err any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get request ID for correlation if available
					requestID := GetRequestIDFromRequest(r)
					
					// Log the panic with stack trace
					log.Printf("PANIC [Request ID: %s]: %v\nStack trace:\n%s", 
						requestID, err, string(debug.Stack()))
					
					// Call custom handler
					handler(w, r, err)
				}
			}()
			
			next.ServeHTTP(w, r)
		})
	}
}

// DefaultPanicHandler is a default panic handler that can be used with RecoveryWithHandler
func DefaultPanicHandler(w http.ResponseWriter, r *http.Request, err any) {
	// Check if response has already been written
	if w.Header().Get("Content-Type") == "" {
		api.Error(w, http.StatusInternalServerError, 
			fmt.Sprintf("Internal server error - Request ID: %s", GetRequestIDFromRequest(r)))
	}
}