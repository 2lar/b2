package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware adds distributed tracing to HTTP requests
func TracingMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract route pattern
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}
			
			// Start span
			ctx, span := tracer.Start(
				r.Context(),
				routePattern,
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.route", routePattern),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()
			
			// Wrap response writer to capture status
			ww := &responseWriter{ResponseWriter: w, status: 200}
			
			// Continue with traced context
			next.ServeHTTP(ww, r.WithContext(ctx))
			
			// Record response
			span.SetAttributes(
				attribute.Int("http.status_code", ww.status),
			)
			
			if ww.status >= 400 {
				span.SetStatus(codes.Error, http.StatusText(ww.status))
			}
		})
	}
}

// MetricsMiddleware adds Prometheus metrics to HTTP requests
func MetricsMiddleware(collector *Collector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Get route pattern
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = "unknown"
			}
			
			// Wrap response writer
			ww := &responseWriter{ResponseWriter: w, status: 200}
			
			// Process request
			next.ServeHTTP(ww, r)
			
			// Record metrics
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.status)
			
			collector.HTTPRequests.WithLabelValues(
				r.Method,
				routePattern,
				status,
			).Inc()
			
			collector.HTTPDuration.WithLabelValues(
				r.Method,
				routePattern,
			).Observe(duration)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture response status
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}