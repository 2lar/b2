package observability

import (
	"context"
	"fmt"

	"github.com/aws/aws-xray-sdk-go/xray"
)

// Tracer provides distributed tracing capabilities
type Tracer struct {
	serviceName string
}

// NewTracer creates a new tracer instance
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
	}
}

// StartSegment starts a new trace segment
func (t *Tracer) StartSegment(ctx context.Context, name string) (context.Context, *xray.Segment) {
	return xray.BeginSegment(ctx, fmt.Sprintf("%s.%s", t.serviceName, name))
}

// StartSubsegment starts a new subsegment within an existing segment
func (t *Tracer) StartSubsegment(ctx context.Context, name string) (context.Context, *xray.Segment) {
	return xray.BeginSubsegment(ctx, name)
}

// TraceFunction wraps a function with tracing
func (t *Tracer) TraceFunction(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, seg := t.StartSubsegment(ctx, name)
	defer seg.Close(nil)

	err := fn(ctx)
	if err != nil {
		seg.AddError(err)
	}

	return err
}

// AddMetadata adds metadata to the current segment
func (t *Tracer) AddMetadata(ctx context.Context, key string, value interface{}) {
	if seg := xray.GetSegment(ctx); seg != nil {
		seg.AddMetadata(key, value)
	}
}

// AddAnnotation adds an indexed annotation to the current segment
func (t *Tracer) AddAnnotation(ctx context.Context, key string, value string) {
	if seg := xray.GetSegment(ctx); seg != nil {
		seg.AddAnnotation(key, value)
	}
}

// RecordError records an error in the current segment
func (t *Tracer) RecordError(ctx context.Context, err error) {
	if seg := xray.GetSegment(ctx); seg != nil {
		seg.AddError(err)
	}
}