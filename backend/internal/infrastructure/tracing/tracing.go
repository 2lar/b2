package tracing

import (
	"context"
	"fmt"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// TracerProvider wraps OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// InitTracing initializes distributed tracing
func InitTracing(serviceName, environment, endpoint string) (*TracerProvider, error) {
	// Create OTLP exporter
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(), // Use TLS in production
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}
	
	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Adjust sampling in production
	)
	
	// Set global provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	
	return &TracerProvider{
		provider: tp,
		tracer:   tp.Tracer(serviceName),
	}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.provider.Shutdown(ctx)
}

// StartSpan starts a new span
func (tp *TracerProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tp.tracer.Start(ctx, name, opts...)
}

// TraceRepository wraps a repository with tracing
func TraceRepository(repo repository.NodeRepository, tracer trace.Tracer) repository.NodeRepository {
	return &tracedNodeRepository{
		inner:  repo,
		tracer: tracer,
	}
}

type tracedNodeRepository struct {
	inner  repository.NodeRepository
	tracer trace.Tracer
}

func (r *tracedNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	ctx, span := r.tracer.Start(ctx, "repository.CreateNodeAndKeywords",
		trace.WithAttributes(
			attribute.String("node.id", node.ID.String()),
			attribute.String("user.id", node.UserID.String()),
		),
	)
	defer span.End()
	
	err := r.inner.CreateNodeAndKeywords(ctx, node)
	if err != nil {
		span.RecordError(err)
	}
	
	return err
}

func (r *tracedNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	ctx, span := r.tracer.Start(ctx, "repository.FindNodeByID",
		trace.WithAttributes(
			attribute.String("node.id", nodeID),
			attribute.String("user.id", userID),
		),
	)
	defer span.End()
	
	node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		span.RecordError(err)
	}
	
	return node, err
}

// UpdateNode is not part of the NodeRepository interface - removed

func (r *tracedNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	ctx, span := r.tracer.Start(ctx, "repository.DeleteNode",
		trace.WithAttributes(
			attribute.String("node.id", nodeID),
			attribute.String("user.id", userID),
		),
	)
	defer span.End()
	
	err := r.inner.DeleteNode(ctx, userID, nodeID)
	if err != nil {
		span.RecordError(err)
	}
	
	return err
}

func (r *tracedNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	ctx, span := r.tracer.Start(ctx, "repository.FindNodes",
		trace.WithAttributes(
			attribute.String("user.id", query.UserID),
		),
	)
	defer span.End()
	
	nodes, err := r.inner.FindNodes(ctx, query)
	if err != nil {
		span.RecordError(err)
	}
	
	return nodes, err
}

func (r *tracedNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	ctx, span := r.tracer.Start(ctx, "repository.GetNodesPage",
		trace.WithAttributes(
			attribute.String("user.id", query.UserID),
			attribute.Int("limit", pagination.Limit),
			attribute.Int("offset", pagination.Offset),
		),
	)
	defer span.End()
	
	page, err := r.inner.GetNodesPage(ctx, query, pagination)
	if err != nil {
		span.RecordError(err)
	}
	
	return page, err
}

func (r *tracedNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	ctx, span := r.tracer.Start(ctx, "repository.GetNodeNeighborhood",
		trace.WithAttributes(
			attribute.String("node.id", nodeID),
			attribute.String("user.id", userID),
			attribute.Int("depth", depth),
		),
	)
	defer span.End()
	
	graph, err := r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	if err != nil {
		span.RecordError(err)
	}
	
	return graph, err
}

func (r *tracedNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "repository.CountNodes",
		trace.WithAttributes(
			attribute.String("user.id", userID),
		),
	)
	defer span.End()
	
	count, err := r.inner.CountNodes(ctx, userID)
	if err != nil {
		span.RecordError(err)
	}
	
	return count, err
}

// Add the missing methods from the NodeRepository interface
func (r *tracedNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	ctx, span := r.tracer.Start(ctx, "repository.FindNodesWithOptions",
		trace.WithAttributes(
			attribute.String("user.id", query.UserID),
		),
	)
	defer span.End()
	
	nodes, err := r.inner.FindNodesWithOptions(ctx, query, opts...)
	if err != nil {
		span.RecordError(err)
	}
	
	return nodes, err
}

func (r *tracedNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	ctx, span := r.tracer.Start(ctx, "repository.FindNodesPageWithOptions",
		trace.WithAttributes(
			attribute.String("user.id", query.UserID),
			attribute.Int("limit", pagination.Limit),
			attribute.Int("offset", pagination.Offset),
		),
	)
	defer span.End()
	
	page, err := r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
	if err != nil {
		span.RecordError(err)
	}
	
	return page, err
}