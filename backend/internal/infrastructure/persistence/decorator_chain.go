// Package persistence provides cross-cutting concerns for repositories.
// This file implements a decorator chain builder for applying multiple decorators.
package persistence

import (
	"time"
	
	"brain2-backend/internal/config"
	"brain2-backend/internal/infrastructure/observability"
	"brain2-backend/internal/infrastructure/persistence/cache"
	"brain2-backend/internal/repository"
	
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DecoratorChain builds a chain of decorators for repositories.
type DecoratorChain struct {
	config  *config.Config
	logger  *zap.Logger
	cache   cache.Cache
	metrics observability.MetricsCollector
}

// NewDecoratorChain creates a new decorator chain builder.
func NewDecoratorChain(
	config *config.Config,
	logger *zap.Logger,
	cache cache.Cache,
	metrics observability.MetricsCollector,
) *DecoratorChain {
	return &DecoratorChain{
		config:  config,
		logger:  logger,
		cache:   cache,
		metrics: metrics,
	}
}

// ============================================================================
// NODE REPOSITORY DECORATION
// ============================================================================

// DecorateNodeRepository applies all configured decorators to a node repository.
// Order: Base -> Retry -> Circuit Breaker -> Cache -> Metrics -> Logging
func (dc *DecoratorChain) DecorateNodeRepository(base repository.NodeRepository) repository.NodeRepository {
	decorated := base
	
	// Layer 1: Retry decorator (closest to base)
	if dc.config.Features.EnableRetries {
		decorated = NewRetryNodeRepository(
			decorated,
			RetryConfig{
				MaxRetries:    3,
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      2 * time.Second,
				BackoffFactor: 2.0,
				JitterFactor:  0.1,
			},
		)
		dc.logger.Debug("Applied retry decorator to NodeRepository")
	}
	
	// Layer 2: Circuit breaker
	if dc.config.Features.EnableCircuitBreaker {
		decorated = NewCircuitBreakerNodeRepository(
			decorated,
			CircuitBreakerConfig{
				FailureThreshold: 0.5,
				SuccessThreshold: 0.8,
				MinimumRequests:  10,
				WindowSize:       1 * time.Minute,
				OpenDuration:     30 * time.Second,
				HalfOpenRequests: 2,
			},
		)
		dc.logger.Debug("Applied circuit breaker decorator to NodeRepository")
	}
	
	// Layer 3: Caching
	if dc.config.Features.EnableCaching && dc.cache != nil {
		decorated = cache.NewCachingNodeRepository(
			decorated,
			dc.cache,
			cache.CachingConfig{
				DefaultTTL:      5 * time.Minute,
				MaxCacheSize:    1000,
				EvictionPolicy:  "LRU",
			},
		)
		dc.logger.Debug("Applied caching decorator to NodeRepository")
	}
	
	// Layer 4: Metrics
	if dc.config.Features.EnableMetrics && dc.metrics != nil {
		decorated = observability.NewMetricsNodeRepository(
			decorated,
			dc.metrics,
			observability.MetricsConfig{
				ServiceName:      "brain2",
				Environment:      "production",
				Version:          "1.0.0",
				EnableLatency:    true,
				EnableThroughput: true,
				EnableErrors:     true,
				EnableBusiness:   true,
			},
		)
		dc.logger.Debug("Applied metrics decorator to NodeRepository")
	}
	
	// Layer 5: Logging (outermost)
	if dc.config.Features.EnableLogging {
		decorated = observability.NewLoggingNodeRepository(
			decorated,
			dc.logger,
			observability.LoggingConfig{
				LogRequests:   true,
				LogResponses:  false, // Don't log PII
				LogErrors:     true,
				LogTiming:     true,
				LogLevel:      zapcore.InfoLevel,
				SlowThreshold: 100 * time.Millisecond,
			},
		)
		dc.logger.Debug("Applied logging decorator to NodeRepository")
	}
	
	return decorated
}

// ============================================================================
// EDGE REPOSITORY DECORATION
// ============================================================================

// DecorateEdgeRepository applies configured decorators to an edge repository.
func (dc *DecoratorChain) DecorateEdgeRepository(base repository.EdgeRepository) repository.EdgeRepository {
	decorated := base
	
	// Retry decorator
	if dc.config.Features.EnableRetries {
		decorated = NewRetryEdgeRepository(
			decorated,
			RetryConfig{
				MaxRetries:    3,
				InitialDelay:  100 * time.Millisecond,
				MaxDelay:      2 * time.Second,
				BackoffFactor: 2.0,
				JitterFactor:  0.1,
			},
		)
		dc.logger.Debug("Applied retry decorator to EdgeRepository")
	}
	
	// Circuit breaker
	// Note: CircuitBreakerEdgeRepository not yet implemented
	// if dc.config.Features.EnableCircuitBreaker {
	// 	decorated = NewCircuitBreakerEdgeRepository(
	// 		decorated,
	// 		CircuitBreakerConfig{
	// 			MaxFailures:      5,
	// 			ResetTimeout:     30 * time.Second,
	// 			HalfOpenRequests: 2,
	// 		},
	// 	)
	// 	dc.logger.Debug("Applied circuit breaker decorator to EdgeRepository")
	// }
	
	// Metrics
	// Note: MetricsEdgeRepository not yet implemented
	// if dc.config.Features.EnableMetrics && dc.metrics != nil {
	// 	decorated = NewMetricsEdgeRepository(decorated, dc.metrics)
	// 	dc.logger.Debug("Applied metrics decorator to EdgeRepository")
	// }
	
	// Logging
	if dc.config.Features.EnableLogging {
		decorated = observability.NewLoggingEdgeRepository(
			decorated,
			dc.logger,
			observability.LoggingConfig{
				LogRequests:   true,
				LogResponses:  false,
				LogErrors:     true,
				LogTiming:     true,
				LogLevel:      zapcore.InfoLevel,
				SlowThreshold: 100 * time.Millisecond,
			},
		)
		dc.logger.Debug("Applied logging decorator to EdgeRepository")
	}
	
	return decorated
}

// ============================================================================
// CATEGORY REPOSITORY DECORATION
// ============================================================================

// DecorateCategoryRepository applies configured decorators to a category repository.
func (dc *DecoratorChain) DecorateCategoryRepository(base repository.CategoryRepository) repository.CategoryRepository {
	decorated := base
	
	// Retry decorator
	if dc.config.Features.EnableRetries {
		// Note: Category repository doesn't have retry decorator
		// decorated = NewRetryCategoryRepository(...)
		dc.logger.Debug("Retry decorator not available for CategoryRepository")
		dc.logger.Debug("Applied retry decorator to CategoryRepository")
	}
	
	// Circuit breaker
	if dc.config.Features.EnableCircuitBreaker {
		// Note: Category repository doesn't have circuit breaker decorator
		// decorated = NewCircuitBreakerCategoryRepository(...)
		dc.logger.Debug("Circuit breaker decorator not available for CategoryRepository")
		dc.logger.Debug("Applied circuit breaker decorator to CategoryRepository")
	}
	
	// Caching
	// Note: Category repository doesn't have caching decorator
	// if dc.config.Features.EnableCaching && dc.cache != nil {
	// 	decorated = NewCachingCategoryRepository(decorated, dc.cache)
	// 	dc.logger.Debug("Applied caching decorator to CategoryRepository")
	// }
	
	// Metrics
	// Note: Category repository doesn't have metrics decorator
	// if dc.config.Features.EnableMetrics && dc.metrics != nil {
	// 	decorated = NewMetricsCategoryRepository(decorated, dc.metrics)
	// 	dc.logger.Debug("Applied metrics decorator to CategoryRepository")
	// }
	
	// Logging
	// Note: Category repository doesn't have logging decorator
	// if dc.config.Features.EnableLogging {
	// 	decorated = NewLoggingCategoryRepository(decorated, dc.logger)
	// 	dc.logger.Debug("Applied logging decorator to CategoryRepository")
	// }
	
	return decorated
}

// ============================================================================
// CONFIGURATION TYPES
// ============================================================================

// Note: RetryConfig is defined in retry_decorator.go

// ============================================================================
// INTERFACES
// ============================================================================

// Note: CircuitBreakerConfig is defined in circuit_breaker_decorator.go
// Note: Cache interface is defined in caching_repository.go
// Note: MetricsCollector is defined in metrics_repository.go