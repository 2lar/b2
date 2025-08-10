package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"brain2-backend/pkg/api"
	"github.com/stretchr/testify/assert"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("Should generate request ID when not provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := GetRequestIDFromRequest(r)
			assert.NotEmpty(t, requestID)
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	})

	t.Run("Should use provided request ID", func(t *testing.T) {
		expectedID := "test-request-id"
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", expectedID)
		w := httptest.NewRecorder()

		handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := GetRequestIDFromRequest(r)
			assert.Equal(t, expectedID, requestID)
			w.WriteHeader(http.StatusOK)
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, expectedID, w.Header().Get("X-Request-ID"))
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("Should handle panic gracefully", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		
		// Check if the response body contains an error message
		body := w.Body.String()
		assert.Contains(t, body, "error")
	})

	t.Run("Should pass through normal requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("Should allow normal requests to complete", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := Timeout(5*time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Short delay
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	t.Run("Should pass through successful requests", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig("test")
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := CircuitBreaker(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		}))

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Should handle 5xx errors as failures", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig("test-failure")
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := CircuitBreaker(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		handler.ServeHTTP(w, req)

		// The response should still be 500 (the circuit breaker records it but doesn't change it)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestGetRequestID(t *testing.T) {
	t.Run("Should return request ID from context", func(t *testing.T) {
		expectedID := "test-id"
		ctx := context.WithValue(context.Background(), RequestIDKey, expectedID)
		
		requestID := GetRequestID(ctx)
		assert.Equal(t, expectedID, requestID)
	})

	t.Run("Should return empty string when no request ID in context", func(t *testing.T) {
		ctx := context.Background()
		
		requestID := GetRequestID(ctx)
		assert.Empty(t, requestID)
	})
}