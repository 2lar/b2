// Package middleware provides a composable middleware pipeline following SOLID principles.
// This file implements the Open/Closed Principle by allowing middleware extension
// without modification through a configurable pipeline system.
package middleware

import (
	"fmt"
	"net/http"
	"time"
	
	sharedContext "brain2-backend/internal/context"
	"brain2-backend/pkg/api"
	"go.uber.org/zap"
)

// MiddlewareFunc represents a middleware function in the pipeline.
type MiddlewareFunc func(http.HandlerFunc) http.HandlerFunc

// Middleware represents a composable middleware component.
// This interface follows the Strategy Pattern, allowing different middleware
// implementations to be plugged into the pipeline.
type Middleware interface {
	// Name returns the middleware name for logging and debugging
	Name() string
	
	// Execute applies the middleware logic to the handler
	Execute(next http.HandlerFunc) http.HandlerFunc
	
	// Priority returns the execution priority (lower numbers execute first)
	Priority() int
}

// Pipeline manages a collection of middleware components.
// This pipeline implements the Chain of Responsibility pattern and follows
// the Open/Closed Principle by allowing new middleware to be added without
// modifying existing code.
type Pipeline struct {
	middlewares []Middleware
	logger      *zap.Logger
}

// NewPipeline creates a new middleware pipeline.
func NewPipeline(logger *zap.Logger) *Pipeline {
	return &Pipeline{
		middlewares: make([]Middleware, 0),
		logger:      logger,
	}
}

// Add adds a middleware to the pipeline.
// Middleware are automatically sorted by priority.
func (p *Pipeline) Add(middleware Middleware) *Pipeline {
	p.middlewares = append(p.middlewares, middleware)
	p.sortByPriority()
	return p
}

// AddFunc adds a function-based middleware to the pipeline.
func (p *Pipeline) AddFunc(name string, priority int, fn MiddlewareFunc) *Pipeline {
	middleware := &FuncMiddleware{
		name:     name,
		priority: priority,
		fn:       fn,
	}
	return p.Add(middleware)
}

// Build builds the middleware chain and returns the final handler.
func (p *Pipeline) Build(handler http.HandlerFunc) http.HandlerFunc {
	// Start from the end and work backwards
	final := handler
	
	for i := len(p.middlewares) - 1; i >= 0; i-- {
		middleware := p.middlewares[i]
		final = middleware.Execute(final)
		
		if p.logger != nil {
			p.logger.Debug("Added middleware to pipeline",
				zap.String("middleware", middleware.Name()),
				zap.Int("priority", middleware.Priority()),
			)
		}
	}
	
	return final
}

// sortByPriority sorts middleware by priority (ascending order).
func (p *Pipeline) sortByPriority() {
	for i := 0; i < len(p.middlewares)-1; i++ {
		for j := i + 1; j < len(p.middlewares); j++ {
			if p.middlewares[i].Priority() > p.middlewares[j].Priority() {
				p.middlewares[i], p.middlewares[j] = p.middlewares[j], p.middlewares[i]
			}
		}
	}
}

// ============================================================================
// BUILT-IN MIDDLEWARE IMPLEMENTATIONS
// ============================================================================

// FuncMiddleware wraps a function to implement the Middleware interface.
type FuncMiddleware struct {
	name     string
	priority int
	fn       MiddlewareFunc
}

func (f *FuncMiddleware) Name() string { return f.name }
func (f *FuncMiddleware) Priority() int { return f.priority }
func (f *FuncMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return f.fn(next)
}

// AuthenticationMiddleware handles user authentication.
type AuthenticationMiddleware struct {
	logger *zap.Logger
}

func NewAuthenticationMiddleware(logger *zap.Logger) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{logger: logger}
}

func (a *AuthenticationMiddleware) Name() string { return "Authentication" }
func (a *AuthenticationMiddleware) Priority() int { return 10 } // High priority

func (a *AuthenticationMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := sharedContext.GetUserIDFromContext(r.Context())
		if !ok {
			if a.logger != nil {
				a.logger.Warn("Authentication failed", zap.String("path", r.URL.Path))
			}
			api.Error(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		
		// Add user ID to context using shared context utilities
		ctx := sharedContext.WithUserID(r.Context(), userID)
		next(w, r.WithContext(ctx))
	}
}

// ValidationMiddleware handles request validation.
type ValidationMiddleware struct {
	validator func(*http.Request) error
	logger    *zap.Logger
}

func NewValidationMiddleware(validator func(*http.Request) error, logger *zap.Logger) *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validator,
		logger:    logger,
	}
}

func (v *ValidationMiddleware) Name() string { return "Validation" }
func (v *ValidationMiddleware) Priority() int { return 20 } // After authentication

func (v *ValidationMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if v.validator != nil {
			if err := v.validator(r); err != nil {
				if v.logger != nil {
					v.logger.Warn("Request validation failed", zap.Error(err))
				}
				api.Error(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		next(w, r)
	}
}

// LoggingMiddleware handles request logging.
type LoggingMiddleware struct {
	logger *zap.Logger
}

func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

func (l *LoggingMiddleware) Name() string { return "Logging" }
func (l *LoggingMiddleware) Priority() int { return 5 } // Very high priority

func (l *LoggingMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		if l.logger != nil {
			l.logger.Info("Request started",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
		}
		
		next(w, r)
		
		if l.logger != nil {
			l.logger.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", time.Since(start)),
			)
		}
	}
}

// ServiceAvailabilityMiddleware checks service availability.
type ServiceAvailabilityMiddleware struct {
	serviceName string
	checker     func() bool
	logger      *zap.Logger
}

func NewServiceAvailabilityMiddleware(serviceName string, checker func() bool, logger *zap.Logger) *ServiceAvailabilityMiddleware {
	return &ServiceAvailabilityMiddleware{
		serviceName: serviceName,
		checker:     checker,
		logger:      logger,
	}
}

func (s *ServiceAvailabilityMiddleware) Name() string { 
	return fmt.Sprintf("ServiceAvailability_%s", s.serviceName) 
}
func (s *ServiceAvailabilityMiddleware) Priority() int { return 15 } // After auth, before validation

func (s *ServiceAvailabilityMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checker() {
			if s.logger != nil {
				s.logger.Error("Service unavailable", zap.String("service", s.serviceName))
			}
			api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable")
			return
		}
		next(w, r)
	}
}

// ErrorRecoveryMiddleware handles panic recovery.
type ErrorRecoveryMiddleware struct {
	logger *zap.Logger
}

func NewErrorRecoveryMiddleware(logger *zap.Logger) *ErrorRecoveryMiddleware {
	return &ErrorRecoveryMiddleware{logger: logger}
}

func (e *ErrorRecoveryMiddleware) Name() string { return "ErrorRecovery" }
func (e *ErrorRecoveryMiddleware) Priority() int { return 1 } // Highest priority (first to execute)

func (e *ErrorRecoveryMiddleware) Execute(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if e.logger != nil {
					e.logger.Error("Panic recovered",
						zap.Any("error", err),
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method),
					)
				}
				api.Error(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next(w, r)
	}
}

// ============================================================================
// PIPELINE BUILDER FOR COMMON SCENARIOS
// ============================================================================

// PipelineBuilder provides a fluent interface for building common middleware pipelines.
type PipelineBuilder struct {
	pipeline *Pipeline
	logger   *zap.Logger
}

// NewPipelineBuilder creates a new pipeline builder.
func NewPipelineBuilder(logger *zap.Logger) *PipelineBuilder {
	return &PipelineBuilder{
		pipeline: NewPipeline(logger),
		logger:   logger,
	}
}

// WithErrorRecovery adds error recovery middleware.
func (b *PipelineBuilder) WithErrorRecovery() *PipelineBuilder {
	b.pipeline.Add(NewErrorRecoveryMiddleware(b.logger))
	return b
}

// WithLogging adds logging middleware.
func (b *PipelineBuilder) WithLogging() *PipelineBuilder {
	b.pipeline.Add(NewLoggingMiddleware(b.logger))
	return b
}

// WithAuthentication adds authentication middleware.
func (b *PipelineBuilder) WithAuthentication() *PipelineBuilder {
	b.pipeline.Add(NewAuthenticationMiddleware(b.logger))
	return b
}

// WithServiceAvailability adds service availability checking.
func (b *PipelineBuilder) WithServiceAvailability(serviceName string, checker func() bool) *PipelineBuilder {
	b.pipeline.Add(NewServiceAvailabilityMiddleware(serviceName, checker, b.logger))
	return b
}

// WithValidation adds request validation.
func (b *PipelineBuilder) WithValidation(validator func(*http.Request) error) *PipelineBuilder {
	b.pipeline.Add(NewValidationMiddleware(validator, b.logger))
	return b
}

// WithCustom adds a custom middleware.
func (b *PipelineBuilder) WithCustom(middleware Middleware) *PipelineBuilder {
	b.pipeline.Add(middleware)
	return b
}

// WithCustomFunc adds a custom function-based middleware.
func (b *PipelineBuilder) WithCustomFunc(name string, priority int, fn MiddlewareFunc) *PipelineBuilder {
	b.pipeline.AddFunc(name, priority, fn)
	return b
}

// Build builds the final pipeline.
func (b *PipelineBuilder) Build() *Pipeline {
	return b.pipeline
}

// ============================================================================
// COMMON PIPELINE CONFIGURATIONS
// ============================================================================

// BuildAPIMiddleware creates a standard API middleware pipeline.
func BuildAPIMiddleware(logger *zap.Logger) *Pipeline {
	return NewPipelineBuilder(logger).
		WithErrorRecovery().
		WithLogging().
		WithAuthentication().
		Build()
}

// BuildCQRSQueryMiddleware creates middleware for CQRS query handlers.
func BuildCQRSQueryMiddleware(logger *zap.Logger, serviceChecker func() bool) *Pipeline {
	return NewPipelineBuilder(logger).
		WithErrorRecovery().
		WithLogging().
		WithAuthentication().
		WithServiceAvailability("QueryService", serviceChecker).
		Build()
}

// BuildCQRSCommandMiddleware creates middleware for CQRS command handlers.
func BuildCQRSCommandMiddleware(logger *zap.Logger, serviceChecker func() bool, validator func(*http.Request) error) *Pipeline {
	return NewPipelineBuilder(logger).
		WithErrorRecovery().
		WithLogging().
		WithAuthentication().
		WithServiceAvailability("CommandService", serviceChecker).
		WithValidation(validator).
		Build()
}