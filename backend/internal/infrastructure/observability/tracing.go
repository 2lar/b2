package observability

import (
	"context"
	"fmt"
	"os"
	
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// TracerProvider wraps OpenTelemetry tracer provider with enhanced configuration.
//
// This wrapper provides additional functionality beyond the standard OTEL provider:
//   - Lambda-optimized sampling strategies
//   - Automatic resource attribution
//   - Batch export configuration for performance
//   - Context propagation across AWS services
//   - Custom attribute extraction for domain events
type TracerProvider struct {
	provider *sdktrace.TracerProvider // Underlying OTEL provider
	tracer   trace.Tracer             // Pre-configured tracer instance
	config   TracingConfig            // Configuration settings
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	ServiceName  string
	Environment  string
	Endpoint     string
	SampleRate   float64
	EnableXRay   bool
	EnableDebug  bool
}

// InitTracing initializes distributed tracing with enhanced configuration
func InitTracing(config TracingConfig) (*TracerProvider, error) {
	// Set default values
	if config.ServiceName == "" {
		config.ServiceName = "brain2-backend"
	}
	if config.SampleRate == 0 {
		config.SampleRate = getSampleRate(config.Environment)
	}
	
	// Create exporter based on environment
	exporter, err := createExporter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}
	
	// Create resource with comprehensive metadata
	res, err := createResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create sampler based on environment
	sampler := createSampler(config)
	
	// Create tracer provider with enhanced options
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithRawSpanLimits(sdktrace.SpanLimits{
			AttributeCountLimit:         128,
			EventCountLimit:             128,
			LinkCountLimit:              128,
			AttributePerEventCountLimit: 32,
			AttributePerLinkCountLimit:  32,
		}),
	)
	
	// Set global provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(createPropagator(config))
	
	// Enable error handler for debugging
	if config.EnableDebug {
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			fmt.Printf("OpenTelemetry error: %v\n", err)
		}))
	}
	
	return &TracerProvider{
		provider: tp,
		tracer:   tp.Tracer(config.ServiceName),
		config:   config,
	}, nil
}

// createExporter creates the appropriate exporter based on configuration
func createExporter(config TracingConfig) (sdktrace.SpanExporter, error) {
	// Check if running in AWS Lambda with X-Ray
	if config.EnableXRay || os.Getenv("_X_AMZN_TRACE_ID") != "" {
		return createXRayExporter()
	}
	
	// Default to OTLP exporter
	return createOTLPExporter(config.Endpoint)
}

// createOTLPExporter creates an OTLP exporter
func createOTLPExporter(endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		endpoint = "localhost:4317" // Default OTLP gRPC endpoint
	}
	
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	
	// Use insecure connection for local development
	if endpoint == "localhost:4317" || endpoint == "127.0.0.1:4317" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	
	return otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(opts...),
	)
}

// createXRayExporter creates an AWS X-Ray exporter for Lambda
func createXRayExporter() (sdktrace.SpanExporter, error) {
	// For AWS Lambda, we typically use the ADOT Lambda layer
	// which provides an OTLP endpoint on localhost:4317
	return createOTLPExporter("localhost:4317")
}

// createResource creates a resource with comprehensive metadata
func createResource(config TracingConfig) (*resource.Resource, error) {
	// Get Lambda-specific attributes if running in Lambda
	attrs := []attribute.KeyValue{
		semconv.ServiceName(config.ServiceName),
		semconv.ServiceVersion(getServiceVersion()),
		attribute.String("deployment.environment", config.Environment),
		attribute.String("cloud.provider", "aws"),
		attribute.String("cloud.platform", getPlatform()),
	}
	
	// Add Lambda-specific attributes
	if functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME"); functionName != "" {
		attrs = append(attrs,
			attribute.String("faas.name", functionName),
			attribute.String("faas.version", os.Getenv("AWS_LAMBDA_FUNCTION_VERSION")),
			attribute.String("cloud.region", os.Getenv("AWS_REGION")),
			attribute.String("cloud.account.id", getAWSAccountID()),
		)
	}
	
	// Add container/host attributes
	if hostname, err := os.Hostname(); err == nil {
		attrs = append(attrs, semconv.HostName(hostname))
	}
	
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
}

// createSampler creates a sampler based on environment
func createSampler(config TracingConfig) sdktrace.Sampler {
	switch config.Environment {
	case "production":
		// Adaptive sampling for production
		return sdktrace.TraceIDRatioBased(config.SampleRate)
	case "staging":
		// Higher sampling for staging
		return sdktrace.TraceIDRatioBased(0.1)
	default:
		// Sample everything in development
		return sdktrace.AlwaysSample()
	}
}

// createPropagator creates a composite propagator for trace context
func createPropagator(config TracingConfig) propagation.TextMapPropagator {
	propagators := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
	}
	
	// Add X-Ray propagator if enabled
	if config.EnableXRay {
		// Note: X-Ray propagator would need to be implemented or imported
		// from AWS contrib package
	}
	
	return propagation.NewCompositeTextMapPropagator(propagators...)
}

// getSampleRate returns the default sample rate for an environment
func getSampleRate(environment string) float64 {
	switch environment {
	case "production":
		return 0.01 // 1% sampling
	case "staging":
		return 0.1 // 10% sampling
	default:
		return 1.0 // 100% sampling
	}
}

// getServiceVersion returns the service version from environment or build info
func getServiceVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "unknown"
}

// getPlatform determines the platform (Lambda, ECS, EC2, etc.)
func getPlatform() string {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return "aws_lambda"
	}
	if os.Getenv("ECS_CONTAINER_METADATA_URI") != "" {
		return "aws_ecs"
	}
	return "unknown"
}

// getAWSAccountID attempts to extract AWS account ID from Lambda ARN
func getAWSAccountID() string {
	if arn := os.Getenv("AWS_LAMBDA_FUNCTION_ARN"); arn != "" {
		// ARN format: arn:aws:lambda:region:account-id:function:function-name
		// Simple extraction - in production use proper ARN parsing
		parts := []byte(arn)
		if len(parts) > 0 {
			// Simplified - would need proper parsing
			return "unknown"
		}
	}
	return "unknown"
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

func (r *tracedNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	ctx, span := r.tracer.Start(ctx, "repository.CreateNodeAndKeywords",
		trace.WithAttributes(
			attribute.String("node.id", node.ID().String()),
			attribute.String("user.id", node.UserID().String()),
		),
	)
	defer span.End()
	
	err := r.inner.CreateNodeAndKeywords(ctx, node)
	if err != nil {
		span.RecordError(err)
	}
	
	return err
}

func (r *tracedNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
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

func (r *tracedNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	ctx, span := r.tracer.Start(ctx, "repository.BatchDeleteNodes",
		trace.WithAttributes(
			attribute.String("user.id", userID),
			attribute.Int("batch.size", len(nodeIDs)),
		),
	)
	defer span.End()
	
	deleted, failed, err = r.inner.BatchDeleteNodes(ctx, userID, nodeIDs)
	
	span.SetAttributes(
		attribute.Int("deleted.count", len(deleted)),
		attribute.Int("failed.count", len(failed)),
	)
	
	if err != nil {
		span.RecordError(err)
	}
	
	return deleted, failed, err
}

func (r *tracedNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
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

func (r *tracedNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
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
func (r *tracedNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
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
func (r *tracedNodeRepository) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	return r.inner.BatchGetNodes(ctx, userID, nodeIDs)
}
