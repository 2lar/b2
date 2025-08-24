package observability

import (
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SpanAttributes provides common attribute sets for consistent tracing
type SpanAttributes struct{}

// NewSpanAttributes creates a new span attributes helper
func NewSpanAttributes() *SpanAttributes {
	return &SpanAttributes{}
}

// UserAttributes returns attributes for user context
func (s *SpanAttributes) UserAttributes(userID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("tenant.id", userID), // In multi-tenant scenarios
	}
}

// NodeAttributes returns attributes for node operations
func (s *SpanAttributes) NodeAttributes(n *node.Node) []attribute.KeyValue {
	if n == nil {
		return []attribute.KeyValue{}
	}
	
	return []attribute.KeyValue{
		attribute.String("node.id", n.ID().String()),
		attribute.String("node.user_id", n.UserID().String()),
		attribute.Int("node.version", n.Version()),
		attribute.Int("node.keyword_count", len(n.Keywords().ToSlice())),
		attribute.Int("node.tag_count", len(n.Tags().ToSlice())),
		attribute.Bool("node.archived", n.IsArchived()),
		attribute.String("node.created_at", n.CreatedAt().Format(time.RFC3339)),
	}
}

// EdgeAttributes returns attributes for edge operations
func (s *SpanAttributes) EdgeAttributes(e *edge.Edge) []attribute.KeyValue {
	if e == nil {
		return []attribute.KeyValue{}
	}
	
	return []attribute.KeyValue{
		attribute.String("edge.id", e.ID.String()),
		attribute.String("edge.source_id", e.SourceID.String()),
		attribute.String("edge.target_id", e.TargetID.String()),
		attribute.String("edge.user_id", e.UserID().String()),
		attribute.Float64("edge.weight", e.Weight()),
		attribute.String("edge.created_at", e.CreatedAt.Format(time.RFC3339)),
	}
}

// CategoryAttributes returns attributes for category operations
func (s *SpanAttributes) CategoryAttributes(c *category.Category) []attribute.KeyValue {
	if c == nil {
		return []attribute.KeyValue{}
	}
	
	attrs := []attribute.KeyValue{
		attribute.String("category.id", string(c.ID)),
		attribute.String("category.user_id", c.UserID),
		attribute.String("category.name", c.Name),
		attribute.Int("category.level", c.Level),
		attribute.String("category.created_at", c.CreatedAt.Format(time.RFC3339)),
	}
	
	// Add color if not nil
	if c.Color != nil {
		attrs = append(attrs, attribute.String("category.color", *c.Color))
	}
	
	return attrs
}

// OperationAttributes returns attributes for business operations
func (s *SpanAttributes) OperationAttributes(operation string, metadata map[string]interface{}) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("operation.type", operation),
		attribute.String("operation.timestamp", time.Now().Format(time.RFC3339)),
	}
	
	// Add metadata attributes
	for key, value := range metadata {
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(fmt.Sprintf("operation.%s", key), v))
		case int:
			attrs = append(attrs, attribute.Int(fmt.Sprintf("operation.%s", key), v))
		case int64:
			attrs = append(attrs, attribute.Int64(fmt.Sprintf("operation.%s", key), v))
		case float64:
			attrs = append(attrs, attribute.Float64(fmt.Sprintf("operation.%s", key), v))
		case bool:
			attrs = append(attrs, attribute.Bool(fmt.Sprintf("operation.%s", key), v))
		case time.Time:
			attrs = append(attrs, attribute.String(fmt.Sprintf("operation.%s", key), v.Format(time.RFC3339)))
		}
	}
	
	return attrs
}

// QueryAttributes returns attributes for query operations
func (s *SpanAttributes) QueryAttributes(queryType string, params map[string]interface{}) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("query.type", queryType),
	}
	
	// Add query parameters
	for key, value := range params {
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(fmt.Sprintf("query.param.%s", key), v))
		case int:
			attrs = append(attrs, attribute.Int(fmt.Sprintf("query.param.%s", key), v))
		case []string:
			attrs = append(attrs, attribute.StringSlice(fmt.Sprintf("query.param.%s", key), v))
		}
	}
	
	return attrs
}

// DatabaseAttributes returns attributes for database operations
func (s *SpanAttributes) DatabaseAttributes(operation, table string, itemCount int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
		attribute.Int("db.item_count", itemCount),
		attribute.String("aws.service", "dynamodb"),
		attribute.String("aws.region", "us-east-1"), // Get from config
	}
}

// ErrorAttributes returns attributes for error tracking
func (s *SpanAttributes) ErrorAttributes(err error, errorType string) []attribute.KeyValue {
	if err == nil {
		return []attribute.KeyValue{}
	}
	
	attrs := []attribute.KeyValue{
		attribute.String("error.type", errorType),
		attribute.String("error.message", err.Error()),
	}
	
	// Add domain error details if applicable
	if domainErr, ok := err.(*shared.DomainError); ok {
		attrs = append(attrs,
			attribute.String("error.type", string(domainErr.Type)),
			attribute.String("error.domain", "business"),
		)
	}
	
	return attrs
}

// CacheAttributes returns attributes for cache operations
func (s *SpanAttributes) CacheAttributes(operation string, key string, hit bool) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cache.operation", operation),
		attribute.String("cache.key", key),
		attribute.Bool("cache.hit", hit),
		attribute.String("cache.backend", "memory"), // or redis, memcached, etc.
	}
}

// EventAttributes returns attributes for domain events
func (s *SpanAttributes) EventAttributes(event shared.DomainEvent) []attribute.KeyValue {
	if event == nil {
		return []attribute.KeyValue{}
	}
	
	return []attribute.KeyValue{
		attribute.String("event.type", event.EventType()),
		attribute.String("event.aggregate_id", event.AggregateID()),
		// Version and OccurredAt are not part of the base interface
	}
}

// PerformanceAttributes returns attributes for performance tracking
func (s *SpanAttributes) PerformanceAttributes(duration time.Duration, itemCount int) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Float64("performance.duration_ms", float64(duration.Milliseconds())),
		attribute.Int("performance.item_count", itemCount),
	}
	
	if itemCount > 0 {
		attrs = append(attrs,
			attribute.Float64("performance.items_per_second", float64(itemCount)/duration.Seconds()),
		)
	}
	
	return attrs
}

// CircuitBreakerAttributes returns attributes for circuit breaker state
func (s *SpanAttributes) CircuitBreakerAttributes(state string, failureCount, successCount int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("circuit_breaker.state", state),
		attribute.Int("circuit_breaker.failure_count", failureCount),
		attribute.Int("circuit_breaker.success_count", successCount),
	}
}

// SetSpanAttributes is a helper to add multiple attribute sets to a span
func SetSpanAttributes(span trace.Span, attrSets ...[]attribute.KeyValue) {
	for _, attrs := range attrSets {
		span.SetAttributes(attrs...)
	}
}

// RecordSpanError records an error with context to a span
func RecordSpanError(span trace.Span, err error, description string) {
	if err == nil || !span.IsRecording() {
		return
	}
	
	span.RecordError(err,
		trace.WithAttributes(
			attribute.String("error.description", description),
			attribute.String("error.timestamp", time.Now().Format(time.RFC3339)),
		),
	)
}

// AddSpanEvent adds a custom event to a span with attributes
func AddSpanEvent(span trace.Span, eventName string, attrs map[string]interface{}) {
	if !span.IsRecording() {
		return
	}
	
	eventAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			eventAttrs = append(eventAttrs, attribute.String(key, v))
		case int:
			eventAttrs = append(eventAttrs, attribute.Int(key, v))
		case bool:
			eventAttrs = append(eventAttrs, attribute.Bool(key, v))
		case float64:
			eventAttrs = append(eventAttrs, attribute.Float64(key, v))
		}
	}
	
	span.AddEvent(eventName, trace.WithAttributes(eventAttrs...))
}