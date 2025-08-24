package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware adds comprehensive distributed tracing to HTTP requests.
// Enhanced with trace context propagation, detailed attributes, and performance tracking.
func TracingMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming headers for distributed tracing
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			
			// Extract route pattern
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = r.URL.Path
			}
			
			// Create descriptive span name
			spanName := fmt.Sprintf("%s %s", r.Method, routePattern)
			
			// Start span with comprehensive attributes
			ctx, span := tracer.Start(
				ctx,
				spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					// HTTP semantic conventions
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.host", r.Host),
					attribute.String("http.scheme", func() string {
						if r.TLS != nil {
							return "https"
						}
						return "http"
					}()),
					attribute.String("http.route", routePattern),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
					
					// Request metadata
					attribute.String("http.request_id", r.Header.Get("X-Request-ID")),
					attribute.String("http.client_ip", r.Header.Get("X-Forwarded-For")),
				),
			)
			defer span.End()
			
			// Add user context if available
			if userID := ctx.Value("userID"); userID != nil {
				span.SetAttributes(attribute.String("user.id", fmt.Sprintf("%v", userID)))
			}
			
			// Wrap response writer to capture status and size
			ww := &enhancedResponseWriter{
				ResponseWriter: w,
				status:         200,
				startTime:      time.Now(),
			}
			
			// Propagate trace context in response headers
			propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))
			
			// Add trace ID to response for debugging
			if spanCtx := span.SpanContext(); spanCtx.HasTraceID() {
				w.Header().Set("X-Trace-ID", spanCtx.TraceID().String())
			}
			
			// Continue with traced context
			next.ServeHTTP(ww, r.WithContext(ctx))
			
			// Calculate request duration
			duration := time.Since(ww.startTime)
			
			// Record response attributes
			span.SetAttributes(
				attribute.Int("http.status_code", ww.status),
				attribute.Int64("http.response_size", ww.bytesWritten),
				attribute.Float64("http.duration_ms", float64(duration.Milliseconds())),
			)
			
			// Set span status based on HTTP status
			if ww.status >= 400 {
				span.SetStatus(codes.Error, http.StatusText(ww.status))
				span.RecordError(fmt.Errorf("HTTP %d: %s", ww.status, http.StatusText(ww.status)))
			} else {
				span.SetStatus(codes.Ok, "")
			}
			
			// Add performance warning for slow requests
			if duration > 5*time.Second {
				span.AddEvent("slow_request_warning",
					trace.WithAttributes(
						attribute.Float64("duration_seconds", duration.Seconds()),
					),
				)
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

// enhancedResponseWriter captures additional response metadata for tracing
type enhancedResponseWriter struct {
	http.ResponseWriter
	status        int
	bytesWritten  int64
	startTime     time.Time
	headerWritten bool
}

func (w *enhancedResponseWriter) WriteHeader(status int) {
	if !w.headerWritten {
		w.status = status
		w.headerWritten = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *enhancedResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}