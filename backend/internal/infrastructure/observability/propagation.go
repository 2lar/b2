package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/domain/shared"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContextKey is used to store trace metadata in context
type TraceContextKey string

const (
	// UserIDKey stores the user ID in trace context
	UserIDKey TraceContextKey = "trace.user_id"
	// RequestIDKey stores the request ID in trace context
	RequestIDKey TraceContextKey = "trace.request_id"
	// OperationKey stores the current operation name
	OperationKey TraceContextKey = "trace.operation"
)

// TracePropagator handles trace context propagation across service boundaries
type TracePropagator struct {
	propagator propagation.TextMapPropagator
}

// NewTracePropagator creates a new trace propagator with W3C Trace Context and Baggage
func NewTracePropagator() *TracePropagator {
	// Composite propagator for W3C Trace Context and Baggage
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	
	// Set as global propagator
	otel.SetTextMapPropagator(propagator)
	
	return &TracePropagator{
		propagator: propagator,
	}
}

// InjectEventContext injects trace context into domain events for async processing
func (p *TracePropagator) InjectEventContext(ctx context.Context, event shared.DomainEvent) map[string]string {
	carrier := make(propagation.MapCarrier)
	p.propagator.Inject(ctx, carrier)
	
	// Add event-specific metadata
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		carrier["event.trace_id"] = span.SpanContext().TraceID().String()
		carrier["event.span_id"] = span.SpanContext().SpanID().String()
		carrier["event.type"] = event.EventType()
		carrier["event.aggregate_id"] = event.AggregateID()
		carrier["event.timestamp"] = time.Now().String()
	}
	
	// Add user context if available
	if userID := ctx.Value(UserIDKey); userID != nil {
		carrier["user.id"] = fmt.Sprintf("%v", userID)
	}
	
	return carrier
}

// ExtractEventContext extracts trace context from domain events
func (p *TracePropagator) ExtractEventContext(parentCtx context.Context, traceData map[string]string) context.Context {
	carrier := propagation.MapCarrier(traceData)
	ctx := p.propagator.Extract(parentCtx, carrier)
	
	// Restore user context
	if userID, ok := traceData["user.id"]; ok {
		ctx = context.WithValue(ctx, UserIDKey, userID)
	}
	
	return ctx
}

// CreateChildSpan creates a child span for async operations with proper linking
func CreateChildSpan(ctx context.Context, operationName string, eventType string) (context.Context, trace.Span) {
	tracer := otel.Tracer("brain2-backend")
	
	// Create span with event context
	ctx, span := tracer.Start(ctx, operationName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("event.type", eventType),
			attribute.String("processing.mode", "async"),
		),
	)
	
	// Add user context to span if available
	if userID := ctx.Value(UserIDKey); userID != nil {
		span.SetAttributes(attribute.String("user.id", fmt.Sprintf("%v", userID)))
	}
	
	return ctx, span
}

// LinkSpans creates a link between spans for correlation
func LinkSpans(ctx context.Context, linkedTraceID string, relationship string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	
	span.AddEvent("span_linked",
		trace.WithAttributes(
			attribute.String("linked.trace_id", linkedTraceID),
			attribute.String("link.relationship", relationship),
		),
	)
}

// PropagateToJSON embeds trace context into JSON for async messages
func PropagateToJSON(ctx context.Context, data interface{}) ([]byte, error) {
	// Create wrapper with trace context
	wrapper := struct {
		Data         interface{}       `json:"data"`
		TraceContext map[string]string `json:"_trace_context"`
	}{
		Data:         data,
		TraceContext: make(map[string]string),
	}
	
	// Inject trace context
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, propagation.MapCarrier(wrapper.TraceContext))
	
	// Add span context
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		wrapper.TraceContext["trace_id"] = span.SpanContext().TraceID().String()
		wrapper.TraceContext["span_id"] = span.SpanContext().SpanID().String()
	}
	
	return json.Marshal(wrapper)
}

// ExtractFromJSON extracts trace context from JSON messages
func ExtractFromJSON(parentCtx context.Context, jsonData []byte) (context.Context, interface{}, error) {
	var wrapper struct {
		Data         json.RawMessage   `json:"data"`
		TraceContext map[string]string `json:"_trace_context"`
	}
	
	if err := json.Unmarshal(jsonData, &wrapper); err != nil {
		return parentCtx, nil, err
	}
	
	// Extract trace context if present
	ctx := parentCtx
	if len(wrapper.TraceContext) > 0 {
		propagator := otel.GetTextMapPropagator()
		ctx = propagator.Extract(parentCtx, propagation.MapCarrier(wrapper.TraceContext))
	}
	
	return ctx, wrapper.Data, nil
}

// BaggageManager manages cross-cutting concerns in trace baggage
type BaggageManager struct{}

// SetUserContext adds user information to baggage for propagation
func (b *BaggageManager) SetUserContext(ctx context.Context, userID string) context.Context {
	member, _ := baggage.NewMember("user.id", userID)
	bag, _ := baggage.New(member)
	return baggage.ContextWithBaggage(ctx, bag)
}

// SetRequestMetadata adds request metadata to baggage
func (b *BaggageManager) SetRequestMetadata(ctx context.Context, requestID, clientID string) context.Context {
	members := []baggage.Member{}
	
	if requestID != "" {
		if m, err := baggage.NewMember("request.id", requestID); err == nil {
			members = append(members, m)
		}
	}
	
	if clientID != "" {
		if m, err := baggage.NewMember("client.id", clientID); err == nil {
			members = append(members, m)
		}
	}
	
	if len(members) > 0 {
		bag, _ := baggage.New(members...)
		return baggage.ContextWithBaggage(ctx, bag)
	}
	
	return ctx
}

// GetUserContext retrieves user information from baggage
func (b *BaggageManager) GetUserContext(ctx context.Context) string {
	bag := baggage.FromContext(ctx)
	if member := bag.Member("user.id"); member.Key() != "" {
		return member.Value()
	}
	return ""
}

// GetRequestMetadata retrieves request metadata from baggage
func (b *BaggageManager) GetRequestMetadata(ctx context.Context) (requestID, clientID string) {
	bag := baggage.FromContext(ctx)
	
	if member := bag.Member("request.id"); member.Key() != "" {
		requestID = member.Value()
	}
	
	if member := bag.Member("client.id"); member.Key() != "" {
		clientID = member.Value()
	}
	
	return requestID, clientID
}

// TraceContextCarrier implements propagation.TextMapCarrier for custom carriers
type TraceContextCarrier map[string]string

// Get returns the value for a key
func (c TraceContextCarrier) Get(key string) string {
	return c[key]
}

// Set sets the value for a key
func (c TraceContextCarrier) Set(key string, value string) {
	c[key] = value
}

// Keys returns all keys
func (c TraceContextCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}