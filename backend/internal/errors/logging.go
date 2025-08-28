// Package errors provides structured logging utilities for error handling.
package errors

import (
	"context"
	"errors"
	"net/http"
	"time"
	
	sharedContext "brain2-backend/internal/context"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StructuredLogger wraps zap logger with context-aware functionality
type StructuredLogger struct {
	*zap.Logger
}

// NewStructuredLogger creates a new structured logger with proper configuration
func NewStructuredLogger(environment string) (*StructuredLogger, error) {
	var config zap.Config
	
	if environment == "production" {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	
	// Configure output paths
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	
	// Add sampling to prevent log flooding in production
	if environment == "production" {
		config.Sampling = &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		}
	}
	
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(1), // Skip wrapper functions
	)
	
	if err != nil {
		return nil, err
	}
	
	return &StructuredLogger{logger}, nil
}

// WithContext creates a logger with context fields
func (l *StructuredLogger) WithContext(ctx context.Context) *StructuredLogger {
	fields := []zap.Field{}
	
	// Add correlation ID
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		fields = append(fields, zap.String("correlation_id", correlationID))
	}
	
	// Add request ID
	if requestID, ok := ctx.Value("request_id").(string); ok {
		fields = append(fields, zap.String("request_id", requestID))
	} else if requestID := middleware.GetReqID(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	
	// Add user ID
	if userID, ok := sharedContext.GetUserIDFromContext(ctx); ok && userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}
	
	// Add trace ID (if tracing is enabled)
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	
	// Add API version (if available in context)
	if version, ok := ctx.Value("api_version").(string); ok && version != "" {
		fields = append(fields, zap.String("api_version", version))
	}
	
	// Add operation name if present
	if operation, ok := ctx.Value("operation").(string); ok {
		fields = append(fields, zap.String("operation", operation))
	}
	
	return &StructuredLogger{l.Logger.With(fields...)}
}

// LogError logs an error with appropriate severity and context
func (l *StructuredLogger) LogError(err error, message string, fields ...zap.Field) {
	if err == nil {
		return
	}
	
	var unifiedErr *UnifiedError
	if errors.As(err, &unifiedErr) {
		// Add unified error fields
		fields = append(fields,
			zap.String("error_type", string(unifiedErr.Type)),
			zap.String("error_code", unifiedErr.Code),
			zap.String("error_message", unifiedErr.Message),
			zap.String("error_severity", string(unifiedErr.Severity)),
			zap.Bool("retryable", unifiedErr.Retryable),
		)
		
		if unifiedErr.Operation != "" {
			fields = append(fields, zap.String("failed_operation", unifiedErr.Operation))
		}
		
		if unifiedErr.Resource != "" {
			fields = append(fields, zap.String("resource", unifiedErr.Resource))
		}
		
		if unifiedErr.UserID != "" {
			fields = append(fields, zap.String("affected_user", unifiedErr.UserID))
		}
		
		if unifiedErr.RequestID != "" {
			fields = append(fields, zap.String("error_request_id", unifiedErr.RequestID))
		}
		
		if unifiedErr.Cause != nil {
			fields = append(fields, zap.Error(unifiedErr.Cause))
		}
		
		// Log with appropriate level based on severity
		level := getLogLevel(unifiedErr.Severity)
		l.Log(level, message, fields...)
	} else {
		// Standard error logging
		fields = append(fields, zap.Error(err))
		l.Error(message, fields...)
	}
}

// CorrelationIDMiddleware adds correlation ID to requests
func CorrelationIDMiddleware(logger *StructuredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for existing correlation ID
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				// Generate new correlation ID
				correlationID = uuid.New().String()
			}
			
			// Add to context
			ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
			
			// Add to response headers
			w.Header().Set("X-Correlation-ID", correlationID)
			
			// Log request start
			logger.WithContext(ctx).Info("Request started",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			
			// Continue with request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestLoggingMiddleware logs all HTTP requests with context
func RequestLoggingMiddleware(logger *StructuredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap response writer to capture status
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			
			// Process request
			next.ServeHTTP(wrapped, r)
			
			// Calculate duration
			duration := time.Since(start)
			
			// Get logger with context
			contextLogger := logger.WithContext(r.Context())
			
			// Prepare fields
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.Status()),
				zap.Int("bytes_written", wrapped.BytesWritten()),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			}
			
			// Add query parameters if present
			if r.URL.RawQuery != "" {
				fields = append(fields, zap.String("query", r.URL.RawQuery))
			}
			
			// Log based on status code
			switch {
			case wrapped.Status() >= 500:
				contextLogger.Error("Request failed", fields...)
			case wrapped.Status() >= 400:
				contextLogger.Warn("Request client error", fields...)
			case wrapped.Status() >= 300:
				contextLogger.Info("Request redirected", fields...)
			default:
				contextLogger.Info("Request completed", fields...)
			}
		})
	}
}

// ErrorLoggingMiddleware logs errors with full context
func ErrorLoggingMiddleware(logger *StructuredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create error capturing response writer
			wrapped := &errorCapturingResponseWriter{
				ResponseWriter: w,
				statusCode:     200,
			}
			
			// Defer error logging
			defer func() {
				if wrapped.error != nil {
					contextLogger := logger.WithContext(r.Context())
					contextLogger.LogError(wrapped.error, "Request error occurred",
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Int("status", wrapped.statusCode),
					)
				}
			}()
			
			next.ServeHTTP(wrapped, r)
		})
	}
}

// LogServiceCall logs service layer operations
func LogServiceCall(ctx context.Context, logger *StructuredLogger, operation string, fn func() error) error {
	contextLogger := logger.WithContext(ctx)
	
	// Log operation start
	contextLogger.Debug("Service operation started",
		zap.String("operation", operation),
	)
	
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		contextLogger.LogError(err, "Service operation failed",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
		)
	} else {
		contextLogger.Debug("Service operation completed",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
		)
	}
	
	return err
}

// LogRepositoryCall logs repository layer operations
func LogRepositoryCall(ctx context.Context, logger *StructuredLogger, operation string, resource string, fn func() error) error {
	contextLogger := logger.WithContext(ctx)
	
	// Log operation start
	contextLogger.Debug("Repository operation started",
		zap.String("operation", operation),
		zap.String("resource", resource),
	)
	
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		contextLogger.LogError(err, "Repository operation failed",
			zap.String("operation", operation),
			zap.String("resource", resource),
			zap.Duration("duration", duration),
		)
	} else {
		contextLogger.Debug("Repository operation completed",
			zap.String("operation", operation),
			zap.String("resource", resource),
			zap.Duration("duration", duration),
		)
	}
	
	return err
}

// AuditLog logs security-relevant events
func AuditLog(ctx context.Context, logger *StructuredLogger, event string, details map[string]interface{}) {
	contextLogger := logger.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("audit_event", event),
		zap.Time("timestamp", time.Now().UTC()),
	}
	
	// Add all details as fields
	for k, v := range details {
		fields = append(fields, zap.Any(k, v))
	}
	
	contextLogger.Info("Audit event", fields...)
}

// MetricsLog logs performance metrics
func MetricsLog(ctx context.Context, logger *StructuredLogger, metric string, value float64, tags map[string]string) {
	contextLogger := logger.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("metric_name", metric),
		zap.Float64("metric_value", value),
		zap.Time("timestamp", time.Now().UTC()),
	}
	
	// Add tags as fields
	for k, v := range tags {
		fields = append(fields, zap.String("tag_"+k, v))
	}
	
	contextLogger.Info("Metrics", fields...)
}