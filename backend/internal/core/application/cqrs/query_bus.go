// Package cqrs implements the query side of CQRS pattern
package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
	
	"brain2-backend/internal/core/application/ports"
)

// Query is the base interface for all queries (read operations)
type Query interface {
	// GetQueryName returns the name of the query
	GetQueryName() string
	
	// Validate validates the query parameters
	Validate() error
}

// BaseQuery provides common functionality for queries
type BaseQuery struct {
	UserID    string            `json:"user_id"`
	Filters   map[string]interface{} `json:"filters"`
	Pagination PaginationParams  `json:"pagination"`
}

// PaginationParams contains pagination parameters
type PaginationParams struct {
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	SortBy string `json:"sort_by"`
	Order  string `json:"order"`
}

// Validate validates pagination parameters
func (p PaginationParams) Validate() error {
	if p.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if p.Limit < 0 || p.Limit > 1000 {
		return fmt.Errorf("limit must be between 0 and 1000")
	}
	if p.Order != "" && p.Order != "asc" && p.Order != "desc" {
		return fmt.Errorf("order must be 'asc' or 'desc'")
	}
	return nil
}

// QueryResult is the base interface for query results
type QueryResult interface {
	// IsEmpty checks if the result is empty
	IsEmpty() bool
}

// PagedResult represents a paginated query result
type PagedResult struct {
	Items      interface{} `json:"items"`
	TotalCount int64       `json:"total_count"`
	Offset     int         `json:"offset"`
	Limit      int         `json:"limit"`
	HasMore    bool        `json:"has_more"`
}

// IsEmpty checks if the result is empty
func (r PagedResult) IsEmpty() bool {
	return r.TotalCount == 0
}

// QueryHandler handles a specific query type
type QueryHandler interface {
	// Handle processes the query and returns the result
	Handle(ctx context.Context, query Query) (QueryResult, error)
	
	// CanHandle checks if this handler can handle the query
	CanHandle(query Query) bool
}

// QueryHandlerFunc is a function adapter for QueryHandler
type QueryHandlerFunc func(context.Context, Query) (QueryResult, error)

// QueryBus routes queries to their handlers
type QueryBus struct {
	handlers   map[string]QueryHandler
	middleware []QueryMiddleware
	cache      ports.Cache
	logger     ports.Logger
	metrics    ports.Metrics
	tracer     ports.Tracer
	mu         sync.RWMutex
}

// NewQueryBus creates a new query bus
func NewQueryBus(
	cache ports.Cache,
	logger ports.Logger,
	metrics ports.Metrics,
	tracer ports.Tracer,
) *QueryBus {
	return &QueryBus{
		handlers:   make(map[string]QueryHandler),
		middleware: []QueryMiddleware{},
		cache:      cache,
		logger:     logger,
		metrics:    metrics,
		tracer:     tracer,
	}
}

// Register registers a handler for a query type
func (b *QueryBus) Register(queryType string, handler QueryHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if _, exists := b.handlers[queryType]; exists {
		return fmt.Errorf("handler already registered for query type: %s", queryType)
	}
	
	b.handlers[queryType] = handler
	b.logger.Info("Registered query handler",
		ports.Field{Key: "query_type", Value: queryType})
	
	return nil
}

// RegisterFunc registers a handler function for a query type
func (b *QueryBus) RegisterFunc(queryType string, handler QueryHandlerFunc) error {
	return b.Register(queryType, &funcQueryHandler{handler: handler})
}

// Send sends a query to its handler and returns the result
func (b *QueryBus) Send(ctx context.Context, query Query) (QueryResult, error) {
	// Start tracing
	ctx, span := b.tracer.StartSpan(ctx, "QueryBus.Send",
		SpanOptionWithKind(ports.SpanKindInternal),
		SpanOptionWithAttributes(
			ports.Attribute{Key: "query.type", Value: query.GetQueryName()},
		),
	)
	defer span.End()
	
	// Record metrics
	timer := b.metrics.StartTimer("query.duration",
		ports.Tag{Key: "query", Value: query.GetQueryName()})
	defer timer.Stop()
	
	// Validate query
	if err := query.Validate(); err != nil {
		b.metrics.IncrementCounter("query.validation.failed",
			ports.Tag{Key: "query", Value: query.GetQueryName()})
		span.SetError(err)
		return nil, fmt.Errorf("query validation failed: %w", err)
	}
	
	// Check cache
	cacheKey := b.getCacheKey(query)
	if cached, err := b.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		b.metrics.IncrementCounter("query.cache.hit",
			ports.Tag{Key: "query", Value: query.GetQueryName()})
		return cached, nil
	}
	
	// Apply middleware
	handler := b.applyMiddleware(b.handleQuery)
	
	// Execute query
	result, err := handler(ctx, query)
	if err != nil {
		b.metrics.IncrementCounter("query.failed",
			ports.Tag{Key: "query", Value: query.GetQueryName()})
		span.SetError(err)
		return nil, err
	}
	
	// Cache result
	if err := b.cacheResult(ctx, cacheKey, result); err != nil {
		b.logger.Warn("Failed to cache query result",
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	b.metrics.IncrementCounter("query.success",
		ports.Tag{Key: "query", Value: query.GetQueryName()})
	
	return result, nil
}

// handleQuery routes the query to its handler
func (b *QueryBus) handleQuery(ctx context.Context, query Query) (QueryResult, error) {
	b.mu.RLock()
	handler, exists := b.handlers[query.GetQueryName()]
	b.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no handler registered for query: %s", query.GetQueryName())
	}
	
	if !handler.CanHandle(query) {
		return nil, fmt.Errorf("handler cannot handle query: %s", query.GetQueryName())
	}
	
	return handler.Handle(ctx, query)
}

// Use adds middleware to the query bus
func (b *QueryBus) Use(middleware QueryMiddleware) {
	b.middleware = append(b.middleware, middleware)
}

// applyMiddleware applies all middleware to the handler
func (b *QueryBus) applyMiddleware(handler QueryHandlerFunc) QueryHandlerFunc {
	// Apply middleware in reverse order so they execute in the order added
	for i := len(b.middleware) - 1; i >= 0; i-- {
		handler = b.middleware[i](handler)
	}
	return handler
}

// getCacheKey generates a cache key for a query
func (b *QueryBus) getCacheKey(query Query) string {
	// Implementation would generate a unique key based on query type and parameters
	return fmt.Sprintf("query:%s:%v", query.GetQueryName(), query)
}

// getFromCache retrieves a cached query result
func (b *QueryBus) getFromCache(ctx context.Context, key string) (QueryResult, error) {
	if b.cache == nil {
		return nil, fmt.Errorf("cache not configured")
	}
	
	data, err := b.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	// Deserialize the cached data
	// Implementation would handle proper deserialization
	_ = data
	return nil, fmt.Errorf("not implemented")
}

// cacheResult caches a query result
func (b *QueryBus) cacheResult(ctx context.Context, key string, result QueryResult) error {
	if b.cache == nil || result == nil || result.IsEmpty() {
		return nil
	}
	
	// Serialize the result
	// Implementation would handle proper serialization
	data := []byte{}
	
	// Cache with TTL
	return b.cache.Set(ctx, key, data, 5*time.Minute)
}

// QueryMiddleware is a function that wraps a query handler
type QueryMiddleware func(QueryHandlerFunc) QueryHandlerFunc

// CachingMiddleware caches query results
func CachingMiddleware(cache ports.Cache, ttl time.Duration) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			// Generate cache key
			key := fmt.Sprintf("query:%s:%v", query.GetQueryName(), query)
			
			// Check cache
			if data, err := cache.Get(ctx, key); err == nil && data != nil {
				// Deserialize and return cached result
				// Implementation needed
				_ = data
			}
			
			// Execute query
			result, err := next(ctx, query)
			if err != nil {
				return nil, err
			}
			
			// Cache result
			if !result.IsEmpty() {
				// Serialize and cache result
				// Implementation needed
			}
			
			return result, nil
		}
	}
}

// TimeoutMiddleware adds timeout to queries
func TimeoutMiddleware(timeout time.Duration) QueryMiddleware {
	return func(next QueryHandlerFunc) QueryHandlerFunc {
		return func(ctx context.Context, query Query) (QueryResult, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			
			done := make(chan struct {
				result QueryResult
				err    error
			}, 1)
			
			go func() {
				result, err := next(ctx, query)
				done <- struct {
					result QueryResult
					err    error
				}{result, err}
			}()
			
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("query timeout: %w", ctx.Err())
			case res := <-done:
				return res.result, res.err
			}
		}
	}
}

// funcQueryHandler wraps a function as a QueryHandler
type funcQueryHandler struct {
	handler QueryHandlerFunc
}

func (h *funcQueryHandler) Handle(ctx context.Context, query Query) (QueryResult, error) {
	return h.handler(ctx, query)
}

func (h *funcQueryHandler) CanHandle(query Query) bool {
	return true
}

// QueryRegistry maintains a registry of all queries
type QueryRegistry struct {
	queries map[string]reflect.Type
	mu      sync.RWMutex
}

// NewQueryRegistry creates a new query registry
func NewQueryRegistry() *QueryRegistry {
	return &QueryRegistry{
		queries: make(map[string]reflect.Type),
	}
}

// Register registers a query type
func (r *QueryRegistry) Register(name string, queryType reflect.Type) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries[name] = queryType
}

// Create creates a new instance of a query
func (r *QueryRegistry) Create(name string) (Query, error) {
	r.mu.RLock()
	queryType, exists := r.queries[name]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("unknown query type: %s", name)
	}
	
	return reflect.New(queryType).Interface().(Query), nil
}

// ProjectionQuery is a specialized query for read model projections
type ProjectionQuery struct {
	BaseQuery
	ProjectionName string `json:"projection_name"`
}

// GetQueryName returns the query name
func (q ProjectionQuery) GetQueryName() string {
	return "ProjectionQuery"
}

// Validate validates the projection query
func (q ProjectionQuery) Validate() error {
	if q.ProjectionName == "" {
		return fmt.Errorf("projection name is required")
	}
	return q.BaseQuery.Pagination.Validate()
}