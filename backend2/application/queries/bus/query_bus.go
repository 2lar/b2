package bus

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Query represents a read-only query
type Query interface {
	Validate() error
}

// QueryHandler handles a specific query type
type QueryHandler interface {
	Handle(ctx context.Context, query Query) (interface{}, error)
}

// QueryBus dispatches queries to their handlers
type QueryBus struct {
	handlers map[reflect.Type]QueryHandler
	mu       sync.RWMutex
}

// NewQueryBus creates a new query bus
func NewQueryBus() *QueryBus {
	return &QueryBus{
		handlers: make(map[reflect.Type]QueryHandler),
	}
}

// Register registers a handler for a query type
func (b *QueryBus) Register(queryType Query, handler QueryHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	t := reflect.TypeOf(queryType)
	if _, exists := b.handlers[t]; exists {
		return fmt.Errorf("handler already registered for query type %s", t.Name())
	}
	
	b.handlers[t] = handler
	return nil
}

// Ask dispatches a query to its handler and returns the result
func (b *QueryBus) Ask(ctx context.Context, query Query) (interface{}, error) {
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("query validation failed: %w", err)
	}
	
	b.mu.RLock()
	handler, exists := b.handlers[reflect.TypeOf(query)]
	b.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no handler registered for query type %T", query)
	}
	
	// Execute handler
	result, err := handler.Handle(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query handler failed: %w", err)
	}
	
	return result, nil
}

// QueryHandlerFunc is an adapter to allow functions to be used as handlers
type QueryHandlerFunc func(ctx context.Context, query Query) (interface{}, error)

// Handle implements QueryHandler
func (f QueryHandlerFunc) Handle(ctx context.Context, query Query) (interface{}, error) {
	return f(ctx, query)
}

// CachingMiddleware adds caching to query handlers
type CachingMiddleware struct {
	cache Cache
	ttl   int // TTL in seconds
}

// NewCachingMiddleware creates a new caching middleware
func NewCachingMiddleware(cache Cache, ttl int) *CachingMiddleware {
	return &CachingMiddleware{
		cache: cache,
		ttl:   ttl,
	}
}

// Wrap wraps a query handler with caching
func (m *CachingMiddleware) Wrap(next QueryHandler) QueryHandler {
	return QueryHandlerFunc(func(ctx context.Context, query Query) (interface{}, error) {
		// Generate cache key from query
		cacheKey := m.generateCacheKey(query)
		
		// Check cache
		if cached, found := m.cache.Get(ctx, cacheKey); found {
			return cached, nil
		}
		
		// Execute query
		result, err := next.Handle(ctx, query)
		if err != nil {
			return nil, err
		}
		
		// Store in cache
		m.cache.Set(ctx, cacheKey, result, m.ttl)
		
		return result, nil
	})
}

func (m *CachingMiddleware) generateCacheKey(query Query) string {
	// Simple implementation - in production you'd want something more sophisticated
	return fmt.Sprintf("%T:%+v", query, query)
}

// Cache interface for caching
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl int) error
}

// MetricsMiddleware adds metrics to query handlers
type MetricsMiddleware struct {
	metrics Metrics
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(metrics Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		metrics: metrics,
	}
}

// Wrap wraps a query handler with metrics
func (m *MetricsMiddleware) Wrap(next QueryHandler) QueryHandler {
	return QueryHandlerFunc(func(ctx context.Context, query Query) (interface{}, error) {
		queryType := reflect.TypeOf(query).Name()
		
		// Start timer
		timer := m.metrics.StartTimer("query_duration", queryType)
		defer timer.Stop()
		
		// Increment counter
		m.metrics.Increment("query_count", queryType)
		
		// Execute query
		result, err := next.Handle(ctx, query)
		if err != nil {
			m.metrics.Increment("query_errors", queryType)
			return nil, err
		}
		
		m.metrics.Increment("query_success", queryType)
		return result, nil
	})
}

// Metrics interface
type Metrics interface {
	StartTimer(metric, label string) Timer
	Increment(metric, label string)
}

// Timer interface
type Timer interface {
	Stop()
}