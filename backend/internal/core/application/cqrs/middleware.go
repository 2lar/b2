// Package cqrs provides middleware implementations for command and query buses
package cqrs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/ports"
)

// ============================================================================
// Command Middleware
// ============================================================================

// NewLoggingMiddleware creates middleware that logs command execution
func NewLoggingMiddleware(logger ports.Logger) CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			logger.Info("Executing command",
				ports.Field{Key: "command", Value: cmd.GetCommandName()},
				ports.Field{Key: "correlation_id", Value: cmd.GetCorrelationID()},
			)
			
			start := time.Now()
			err := next(ctx, cmd)
			duration := time.Since(start)
			
			if err != nil {
				logger.Error("Command failed",
					err,
					ports.Field{Key: "command", Value: cmd.GetCommandName()},
					ports.Field{Key: "duration", Value: duration},
				)
			} else {
				logger.Info("Command completed",
					ports.Field{Key: "command", Value: cmd.GetCommandName()},
					ports.Field{Key: "duration", Value: duration},
				)
			}
			
			return err
		}
	}
}

// NewMetricsMiddleware creates middleware that records command metrics
func NewMetricsMiddleware(metrics ports.Metrics) CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			timer := metrics.StartTimer("command.duration",
				ports.Tag{Key: "command", Value: cmd.GetCommandName()})
			defer timer.Stop()
			
			err := next(ctx, cmd)
			
			if err != nil {
				metrics.IncrementCounter("command.error",
					ports.Tag{Key: "command", Value: cmd.GetCommandName()})
			} else {
				metrics.IncrementCounter("command.success",
					ports.Tag{Key: "command", Value: cmd.GetCommandName()})
			}
			
			return err
		}
	}
}

// NewValidationMiddleware creates middleware that validates commands
func NewValidationMiddleware() CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			if err := cmd.Validate(); err != nil {
				return fmt.Errorf("command validation failed: %w", err)
			}
			return next(ctx, cmd)
		}
	}
}

// NewRetryMiddleware creates middleware that retries failed commands
func NewRetryMiddleware(maxRetries int, logger ports.Logger) CommandMiddleware {
	return func(next CommandHandlerFunc) CommandHandlerFunc {
		return func(ctx context.Context, cmd Command) error {
			var err error
			backoff := 100 * time.Millisecond
			
			for i := 0; i <= maxRetries; i++ {
				if i > 0 {
					logger.Debug("Retrying command",
						ports.Field{Key: "command", Value: cmd.GetCommandName()},
						ports.Field{Key: "attempt", Value: i},
						ports.Field{Key: "backoff", Value: backoff})
					time.Sleep(backoff)
					backoff *= 2 // Exponential backoff
				}
				
				err = next(ctx, cmd)
				if err == nil {
					return nil
				}
				
				// Check if error is retryable
				if !isRetryable(err) {
					return err
				}
			}
			
			return fmt.Errorf("command failed after %d retries: %w", maxRetries, err)
		}
	}
}

// ============================================================================
// Query Middleware
// ============================================================================

// NewQueryLoggingMiddleware creates middleware that logs query execution
func NewQueryLoggingMiddleware(logger ports.Logger) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			logger.Debug("Executing query",
				ports.Field{Key: "query", Value: query.GetQueryName()},
			)
			
			start := time.Now()
			result, err := next(ctx, query)
			duration := time.Since(start)
			
			if err != nil {
				logger.Error("Query failed",
					err,
					ports.Field{Key: "query", Value: query.GetQueryName()},
					ports.Field{Key: "duration", Value: duration},
				)
			} else {
				logger.Debug("Query completed",
					ports.Field{Key: "query", Value: query.GetQueryName()},
					ports.Field{Key: "duration", Value: duration},
				)
			}
			
			return result, err
		}
	}
}

// NewQueryMetricsMiddleware creates middleware that records query metrics
func NewQueryMetricsMiddleware(metrics ports.Metrics) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			timer := metrics.StartTimer("query.duration",
				ports.Tag{Key: "query", Value: query.GetQueryName()})
			defer timer.Stop()
			
			result, err := next(ctx, query)
			
			if err != nil {
				metrics.IncrementCounter("query.error",
					ports.Tag{Key: "query", Value: query.GetQueryName()})
			} else {
				metrics.IncrementCounter("query.success",
					ports.Tag{Key: "query", Value: query.GetQueryName()})
			}
			
			return result, err
		}
	}
}

// NewQueryCacheMiddleware creates middleware that caches query results
func NewQueryCacheMiddleware(cache ports.Cache) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			// If no cache is configured, just execute the query
			if cache == nil {
				return next(ctx, query)
			}
			
			// Check if query is cacheable
			cacheableQuery, ok := query.(interface {
				GetCacheKey() string
			})
			if !ok || cacheableQuery.GetCacheKey() == "" {
				return next(ctx, query)
			}
			
			cacheKey := cacheableQuery.GetCacheKey()
			
			// Try to get from cache
			if data, err := cache.Get(ctx, cacheKey); err == nil {
				// Deserialize based on query type
				// This is simplified - in production would use type registry
				// For now, return a simple paged result
				var items interface{}
				if err := json.Unmarshal(data, &items); err == nil {
					return PagedResult{Items: items}, nil
				}
			}
			
			// Execute query
			result, err := next(ctx, query)
			if err != nil {
				return nil, err
			}
			
			// Cache the result
			if data, err := json.Marshal(result); err == nil {
				// Default cache TTL of 5 minutes
				cacheTTL := 5 * time.Minute
				
				// Allow queries to specify their own TTL
				if ttlQuery, ok := query.(interface {
					GetCacheTTL() time.Duration
				}); ok {
					cacheTTL = ttlQuery.GetCacheTTL()
				}
				
				cache.Set(ctx, cacheKey, data, cacheTTL)
			}
			
			return result, nil
		}
	}
}

// NewTimeoutMiddleware creates middleware that enforces query timeouts
func NewTimeoutMiddleware(timeout time.Duration) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			
			done := make(chan struct{})
			var result QueryResult
			var err error
			
			go func() {
				result, err = next(ctx, query)
				close(done)
			}()
			
			select {
			case <-done:
				return result, err
			case <-ctx.Done():
				return nil, fmt.Errorf("query timeout after %v", timeout)
			}
		}
	}
}