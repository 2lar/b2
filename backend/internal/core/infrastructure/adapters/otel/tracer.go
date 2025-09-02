// Package otel provides an OpenTelemetry tracer adapter for the ports.Tracer interface
package otel

import (
	"context"

	"brain2-backend/internal/core/application/ports"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracerAdapter adapts OpenTelemetry tracer to implement ports.Tracer
type TracerAdapter struct {
	tracer trace.Tracer
}

// NewTracerAdapter creates a new OpenTelemetry tracer adapter
func NewTracerAdapter(name string) *TracerAdapter {
	tracer := otel.Tracer(name)
	return &TracerAdapter{
		tracer: tracer,
	}
}

// NewTracerAdapterWithProvider creates a new tracer adapter with a specific provider
func NewTracerAdapterWithProvider(tracer trace.Tracer) *TracerAdapter {
	return &TracerAdapter{
		tracer: tracer,
	}
}

// StartSpan starts a new span
func (t *TracerAdapter) StartSpan(ctx context.Context, name string, opts ...ports.SpanOption) (context.Context, ports.Span) {
	// Apply options
	config := &ports.SpanConfig{}
	for _, opt := range opts {
		opt(config)
	}
	
	// Convert attributes
	otelAttrs := t.convertAttributes(config.Attributes)
	
	// Set span kind
	spanOpts := []trace.SpanStartOption{
		trace.WithAttributes(otelAttrs...),
		trace.WithSpanKind(t.convertSpanKind(config.Kind)),
	}
	
	ctx, span := t.tracer.Start(ctx, name, spanOpts...)
	return ctx, &spanAdapter{span: span}
}

// Extract extracts span context from carrier
func (t *TracerAdapter) Extract(ctx context.Context, carrier interface{}) context.Context {
	// This would need a proper implementation with propagators
	// For now, just return the context as-is
	return ctx
}

// Inject injects span context into carrier
func (t *TracerAdapter) Inject(ctx context.Context, carrier interface{}) error {
	// This would need a proper implementation with propagators
	// For now, this is a no-op
	return nil
}

// convertSpanKind converts ports.SpanKind to trace.SpanKind
func (t *TracerAdapter) convertSpanKind(kind ports.SpanKind) trace.SpanKind {
	switch kind {
	case ports.SpanKindServer:
		return trace.SpanKindServer
	case ports.SpanKindClient:
		return trace.SpanKindClient
	case ports.SpanKindProducer:
		return trace.SpanKindProducer
	case ports.SpanKindConsumer:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindInternal
	}
}

// SpanFromContext returns the current span from context
func (t *TracerAdapter) SpanFromContext(ctx context.Context) ports.Span {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return &noopSpan{}
	}
	return &spanAdapter{span: span}
}

// convertAttributes converts ports.Attribute to OpenTelemetry attributes
func (t *TracerAdapter) convertAttributes(attrs []ports.Attribute) []attribute.KeyValue {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		otelAttrs = append(otelAttrs, t.convertAttribute(attr))
	}
	return otelAttrs
}

// convertAttribute converts a single attribute
func (t *TracerAdapter) convertAttribute(attr ports.Attribute) attribute.KeyValue {
	switch v := attr.Value.(type) {
	case string:
		return attribute.String(attr.Key, v)
	case int:
		return attribute.Int(attr.Key, v)
	case int64:
		return attribute.Int64(attr.Key, v)
	case float64:
		return attribute.Float64(attr.Key, v)
	case bool:
		return attribute.Bool(attr.Key, v)
	case []string:
		return attribute.StringSlice(attr.Key, v)
	default:
		// For any other type, convert to string
		return attribute.String(attr.Key, toString(v))
	}
}

// spanAdapter adapts OpenTelemetry span to implement ports.Span
type spanAdapter struct {
	span trace.Span
}

// End ends the span
func (s *spanAdapter) End() {
	s.span.End()
}

// SetTag sets a tag on the span
func (s *spanAdapter) SetTag(key string, value interface{}) {
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	default:
		s.span.SetAttributes(attribute.String(key, toString(v)))
	}
}

// SetError marks the span as errored
func (s *spanAdapter) SetError(err error) {
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
}

// AddEvent adds an event to the span
func (s *spanAdapter) AddEvent(name string, attrs ...ports.Attribute) {
	otelAttrs := make([]trace.EventOption, 0, len(attrs))
	for _, attr := range attrs {
		switch v := attr.Value.(type) {
		case string:
			otelAttrs = append(otelAttrs, trace.WithAttributes(attribute.String(attr.Key, v)))
		case int:
			otelAttrs = append(otelAttrs, trace.WithAttributes(attribute.Int(attr.Key, v)))
		default:
			otelAttrs = append(otelAttrs, trace.WithAttributes(attribute.String(attr.Key, toString(v))))
		}
	}
	s.span.AddEvent(name, otelAttrs...)
}

// convertStatusCode converts ports status code to OpenTelemetry status code
func (s *spanAdapter) convertStatusCode(code ports.SpanStatusCode) codes.Code {
	switch code {
	case ports.SpanStatusOK:
		return codes.Ok
	case ports.SpanStatusError:
		return codes.Error
	default:
		return codes.Unset
	}
}

// noopSpan is a no-op implementation of ports.Span
type noopSpan struct{}

func (n *noopSpan) End()                                           {}
func (n *noopSpan) SetTag(key string, value interface{})           {}
func (n *noopSpan) SetError(err error)                             {}
func (n *noopSpan) AddEvent(name string, attrs ...ports.Attribute) {}

// toString converts any value to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}