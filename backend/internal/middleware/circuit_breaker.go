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
		MaxRequests:      3,
		Interval:         10 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 0.6, // 60% failure rate
		MinRequests:      3,    // Minimum requests before evaluating failure rate
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
				log.Printf("Circuit breaker '%s' rejected request: %v", config.Name, err)
				
				switch err {
				case gobreaker.ErrOpenState:
					api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - too many failures")
				case gobreaker.ErrTooManyRequests:
					api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - too many requests")
				default:
					// Internal error occurred
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