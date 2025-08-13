package middleware

import (
	"log"
	"net/http"
	"time"

	"brain2-backend/pkg/api"
	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	Name        string
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
	// ReadyToTrip function determines when to trip the circuit breaker
	FailureThreshold float64
	MinRequests      uint32
}

// DefaultCircuitBreakerConfig returns a default configuration for circuit breaker
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:             name,
		MaxRequests:      5,                // Allow more requests in half-open state
		Interval:         30 * time.Second, // Longer interval before resetting stats
		Timeout:          60 * time.Second, // Longer timeout before trying half-open
		FailureThreshold: 0.8,              // 80% failure rate (less aggressive)
		MinRequests:      5,                // More requests before evaluating failure rate
	}
}

// CircuitBreaker creates a circuit breaker middleware with the given configuration
func CircuitBreaker(config CircuitBreakerConfig) func(http.Handler) http.Handler {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        config.Name,
		MaxRequests: config.MaxRequests,
		Interval:    config.Interval,
		Timeout:     config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Only trip if we have enough requests to make a decision
			if counts.Requests < config.MinRequests {
				return false
			}
			
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= config.FailureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker '%s' state changed from %v to %v", name, from, to)
		},
		// Add request callback for debugging
		IsSuccessful: func(err error) bool {
			// Log detailed error information for debugging
			if err != nil {
				log.Printf("Circuit breaker request failed: %v", err)
				return false
			}
			return true
		},
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := cb.Execute(func() (any, error) {
				// Create a custom response writer to capture status code
				wrapper := &responseWrapper{
					ResponseWriter: w,
					statusCode:     http.StatusOK,
				}
				
				next.ServeHTTP(wrapper, r)
				
				// Consider 5xx status codes as failures for circuit breaker
				if wrapper.statusCode >= 500 {
					return nil, http.ErrAbortHandler
				}
				
				return nil, nil
			})

			if err != nil {
				// Circuit breaker is open or half-open and request failed
				log.Printf("Circuit breaker '%s' rejected request to %s %s: %v", config.Name, r.Method, r.URL.Path, err)
				
				switch err {
				case gobreaker.ErrOpenState:
					log.Printf("Circuit breaker '%s' is OPEN - blocking request to %s", config.Name, r.URL.Path)
					api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - too many failures")
				case gobreaker.ErrTooManyRequests:
					log.Printf("Circuit breaker '%s' is HALF-OPEN - too many requests to %s", config.Name, r.URL.Path)
					api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - too many requests")
				default:
					// Internal error occurred
					log.Printf("Circuit breaker '%s' internal error for %s: %v", config.Name, r.URL.Path, err)
					api.Error(w, http.StatusInternalServerError, "Service error")
				}
			}
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}