// Package observability provides comprehensive observability infrastructure for the Brain2 application.
//
// This package demonstrates enterprise-grade observability patterns including:
//   - Distributed tracing with OpenTelemetry
//   - Application metrics with Prometheus
//   - Structured logging integration
//   - Performance monitoring and alerting
//   - Request correlation across services
//
// # Architecture Overview
//
// The observability system follows the three pillars of observability:
//   1. **Logs**: Structured events for debugging and auditing
//   2. **Metrics**: Numerical measurements for monitoring and alerting  
//   3. **Traces**: Request flows for understanding system behavior
//
// # Key Components
//
// ## Distributed Tracing (tracing.go)
//
// Implements OpenTelemetry distributed tracing to track requests across:
//   - HTTP handlers
//   - Application services
//   - Repository operations
//   - External AWS service calls
//
// Usage example:
//
//	ctx, span := tracer.Start(ctx, "NodeService.CreateNode",
//		trace.WithSpanKind(trace.SpanKindInternal),
//		trace.WithAttributes(
//			attribute.String("user.id", userID),
//			attribute.Int("content.length", len(content)),
//		),
//	)
//	defer span.End()
//
// ## Application Metrics (metrics.go)
//
// Provides Prometheus-compatible metrics for:
//   - Request rates and latency
//   - Error rates and types
//   - Business metrics (nodes created, connections made)
//   - Infrastructure metrics (cache hit rates, DB performance)
//
// Usage example:
//
//	collector := observability.NewMetricsCollector(cfg, logger)
//	collector.RecordRequest(ctx, "create_node", duration, err)
//	collector.RecordCacheHit(ctx, "node_cache", true)
//
// ## Request Middleware (middleware.go)
//
// HTTP middleware that automatically:
//   - Creates trace spans for requests
//   - Records request metrics
//   - Adds correlation IDs
//   - Propagates trace context
//
// Integration example:
//
//	router.Use(observability.TracingMiddleware(tracer))
//	router.Use(observability.MetricsMiddleware(collector))
//
// ## Context Propagation (propagation.go)
//
// Ensures trace context flows through:
//   - HTTP requests (via headers)
//   - AWS SDK calls
//   - Background jobs
//   - Domain events
//
// # Performance Considerations
//
// ## Low Overhead Design
//
// The observability infrastructure is designed for minimal performance impact:
//   - Sampling strategies reduce trace volume
//   - Metrics are pre-aggregated locally
//   - Async export prevents request blocking
//   - Smart batching reduces network calls
//
// ## Cold Start Optimization
//
// Special handling for AWS Lambda cold starts:
//   - Pre-warmed metric collectors
//   - Lazy tracer initialization
//   - Minimal startup overhead
//   - Cold start duration tracking
//
// # Configuration
//
// Observability can be configured via environment variables:
//
//	ENABLE_TRACING=true              # Enable distributed tracing
//	ENABLE_METRICS=true              # Enable metrics collection
//	TRACE_SAMPLE_RATE=0.1           # Sample 10% of traces
//	METRICS_EXPORT_INTERVAL=30s     # Export metrics every 30 seconds
//	OTEL_EXPORTER_OTLP_ENDPOINT=... # OTLP collector endpoint
//
// # Monitoring Integration
//
// ## AWS CloudWatch
//
// Metrics are exported to CloudWatch for:
//   - Dashboard visualization
//   - Automated alerting
//   - Log correlation
//   - Cost tracking
//
// ## AWS X-Ray
//
// Traces are exported to X-Ray for:
//   - Service maps
//   - Performance analysis
//   - Error root cause analysis
//   - Dependency tracking
//
// # Development vs Production
//
// ## Development
//   - Console exporter for immediate feedback
//   - Verbose logging enabled
//   - 100% trace sampling
//   - Debug metrics included
//
// ## Production
//   - OTLP exporter to collectors
//   - Structured JSON logging
//   - Intelligent sampling
//   - Only essential metrics
//
// # Best Practices Demonstrated
//
// ## Trace Naming
//
// Consistent span naming convention:
//   - Service.Method format (e.g., "NodeService.CreateNode")
//   - HTTP endpoints use route patterns (e.g., "GET /api/v1/nodes/{id}")
//   - Database operations include entity (e.g., "DB.Node.Create")
//
// ## Attribute Standards
//
// Standard attributes for better correlation:
//   - user.id: User performing the operation
//   - request.id: Unique request identifier
//   - operation.name: Business operation name
//   - error.type: Categorized error type
//
// ## Error Handling
//
// Proper error recording in traces:
//
//	if err != nil {
//		span.RecordError(err)
//		span.SetStatus(codes.Error, err.Error())
//		metrics.RecordError(ctx, "create_node", err)
//	}
//
// ## Resource Attribution
//
// All telemetry includes resource attributes:
//   - service.name: "brain2-backend"
//   - service.version: Git commit hash
//   - deployment.environment: "production"/"staging"/"development"
//
// # Security Considerations
//
// ## Data Sanitization
//
// Sensitive data is automatically scrubbed from:
//   - Trace attributes
//   - Log messages
//   - Error messages
//   - Metric labels
//
// ## Access Control
//
// Observability data access is controlled via:
//   - IAM roles for AWS services
//   - API keys for external services
//   - Network policies for collectors
//   - Data retention policies
//
// # Troubleshooting
//
// ## Common Issues
//
// **Traces not appearing:**
// - Check OTEL_EXPORTER_OTLP_ENDPOINT configuration
// - Verify network connectivity to collector
// - Check sampling rate settings
// - Review authentication credentials
//
// **High latency from observability:**
// - Reduce sampling rate
// - Increase export batch sizes
// - Check collector performance
// - Review attribute volume
//
// **Missing correlations:**
// - Ensure context propagation in async operations
// - Check header propagation in HTTP clients
// - Verify span creation in all layers
// - Review correlation ID generation
//
// # Extension Points
//
// The observability system can be extended with:
//   - Custom metric types
//   - Additional trace exporters
//   - Business-specific attributes
//   - Custom sampling strategies
//   - Alert rule templates
//
// Example custom metric:
//
//	type BusinessMetrics struct {
//		*MetricsCollector
//		nodesCreatedTotal prometheus.Counter
//	}
//
//	func (m *BusinessMetrics) RecordNodeCreation(ctx context.Context, categoryID string) {
//		m.nodesCreatedTotal.Inc()
//		// Additional business logic...
//	}
package observability