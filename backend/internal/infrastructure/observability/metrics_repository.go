package observability

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// MetricsCollector interface abstracts metrics collection
// This allows different metrics backends (Prometheus, StatsD, CloudWatch, etc.)
type MetricsCollector interface {
	// Counter metrics
	IncrementCounter(name string, tags map[string]string)
	IncrementCounterBy(name string, value float64, tags map[string]string)
	
	// Gauge metrics
	SetGauge(name string, value float64, tags map[string]string)
	IncrementGauge(name string, value float64, tags map[string]string)
	
	// Histogram/Timer metrics
	RecordDuration(name string, duration time.Duration, tags map[string]string)
	RecordValue(name string, value float64, tags map[string]string)
	
	// Distribution metrics
	RecordDistribution(name string, value float64, tags map[string]string)
}

// MetricsNodeRepository is a decorator that adds comprehensive metrics to NodeRepository operations.
//
// Key Concepts Illustrated:
//   1. Decorator Pattern: Transparently adds metrics without changing the interface
//   2. Observability: Provides insights into application performance and behavior
//   3. SLI/SLO Monitoring: Tracks Service Level Indicators for SLO compliance
//   4. Business Metrics: Captures business-relevant metrics alongside technical ones
//   5. Performance Profiling: Identifies bottlenecks and optimization opportunities
//
// Metrics Categories Captured:
//   - Latency: Operation duration and percentiles
//   - Throughput: Operations per second and request rates
//   - Errors: Error rates, error types, and failure patterns
//   - Utilization: Resource usage and capacity metrics
//   - Business Metrics: Domain-specific indicators
//
// Example Usage:
//   baseRepo := dynamodb.NewNodeRepository(client, table, index)
//   metricsCollector := prometheus.NewCollector()
//   metricsRepo := NewMetricsNodeRepository(baseRepo, metricsCollector, MetricsConfig{
//       ServiceName:    "brain2-backend",
//       Environment:    "production",
//       EnableLatency:  true,
//       EnableBusiness: true,
//   })
type MetricsNodeRepository struct {
	inner     repository.NodeRepository
	metrics   MetricsCollector
	config    MetricsConfig
	startTime time.Time
}

// MetricsConfig controls which metrics are collected
type MetricsConfig struct {
	// Service identification
	ServiceName  string
	Environment  string
	Version      string
	
	// Metric categories
	EnableLatency   bool // Track operation latency
	EnableThroughput bool // Track operation rates
	EnableErrors    bool // Track error rates and types
	EnableBusiness  bool // Track business-specific metrics
	EnableRetries   bool // Track retry patterns
	
	// Performance settings
	SampleRate      float64       // Sampling rate for high-volume metrics (0.0-1.0)
	LatencyBuckets  []float64     // Custom histogram buckets for latency
	SlowThreshold   time.Duration // Threshold for marking operations as slow
	
	// Alerting thresholds
	ErrorRateThreshold    float64 // Alert when error rate exceeds this
	LatencyP99Threshold   time.Duration // Alert when P99 latency exceeds this
}

// DefaultMetricsConfig returns sensible defaults for metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		ServiceName:           "brain2-backend",
		Environment:          "development",
		Version:              "1.0.0",
		EnableLatency:        true,
		EnableThroughput:     true,
		EnableErrors:         true,
		EnableBusiness:       true,
		EnableRetries:        false,
		SampleRate:           1.0, // Collect all metrics in development
		LatencyBuckets:       []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		SlowThreshold:        time.Second,
		ErrorRateThreshold:   0.05, // 5% error rate threshold
		LatencyP99Threshold:  5 * time.Second,
	}
}

// NewMetricsNodeRepository creates a new metrics decorator for NodeRepository
func NewMetricsNodeRepository(
	inner repository.NodeRepository,
	metrics MetricsCollector,
	config MetricsConfig,
) repository.NodeRepository {
	return &MetricsNodeRepository{
		inner:     inner,
		metrics:   metrics,
		config:    config,
		startTime: time.Now(),
	}
}

// CreateNodeAndKeywords wraps node creation with comprehensive metrics
func (r *MetricsNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	start := time.Now()
	operation := "create_node_and_keywords"
	
	// Base tags for all metrics
	tags := r.buildBaseTags(operation)
	tags["user_id"] = node.UserID.String()
	
	// Increment operation counter
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.create.total", tags)
	}
	
	// Track business metrics
	if r.config.EnableBusiness {
		r.metrics.IncrementCounter("business.nodes.created.total", tags)
		r.metrics.RecordValue("business.node.content_length", float64(len(node.Content.String())), tags)
		r.metrics.RecordValue("business.node.keyword_count", float64(len(node.Keywords().ToSlice())), tags)
		r.metrics.RecordValue("business.node.tag_count", float64(len(node.Tags.ToSlice())), tags)
	}
	
	// Execute the operation
	err := r.inner.CreateNodeAndKeywords(ctx, node)
	
	// Record completion metrics
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.create.duration", duration, tags)
		
		// Track slow operations
		if duration > r.config.SlowThreshold {
			slowTags := r.copyTags(tags)
			slowTags["slow"] = "true"
			r.metrics.IncrementCounter("repository.operations.slow.total", slowTags)
		}
	}
	
	// Track errors
	if err != nil && r.config.EnableErrors {
		errorTags := r.copyTags(tags)
		errorTags["error_type"] = r.classifyError(err)
		r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
		r.metrics.IncrementCounter("repository.operations.create.errors.total", errorTags)
		
		// Track specific error patterns
		if repository.IsConflict(err) {
			r.metrics.IncrementCounter("business.conflicts.node_creation.total", errorTags)
		}
	} else {
		// Track successful operations
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
	}
	
	return err
}

// FindNodeByID wraps node lookup with performance metrics
func (r *MetricsNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	start := time.Now()
	operation := "find_node_by_id"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = userID
	
	// Track read operations
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.read.total", tags)
	}
	
	// Execute the operation
	node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
	
	duration := time.Since(start)
	
	// Record latency metrics
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.read.duration", duration, tags)
	}
	
	// Handle results and errors
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			
			if repository.IsNotFound(err) {
				errorTags["error_type"] = "not_found"
				r.metrics.IncrementCounter("repository.operations.not_found.total", errorTags)
				
				// Business metric: track cache miss patterns
				if r.config.EnableBusiness {
					r.metrics.IncrementCounter("business.cache_miss.node_lookup.total", errorTags)
				}
			} else {
				errorTags["error_type"] = r.classifyError(err)
				r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
			}
		}
	} else {
		// Track successful reads
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
		
		// Track business metrics for found nodes
		if node != nil && r.config.EnableBusiness {
			businessTags := r.copyTags(tags)
			businessTags["archived"] = boolToString(node.IsArchived())
			r.metrics.IncrementCounter("business.nodes.accessed.total", businessTags)
			
			// Track node age (time since creation)
			age := time.Since(node.CreatedAt)
			r.metrics.RecordValue("business.node.age_days", age.Hours()/24, businessTags)
		}
	}
	
	return node, err
}

// FindNodes wraps node search with query performance metrics
func (r *MetricsNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	start := time.Now()
	operation := "find_nodes"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = query.UserID
	tags["has_keywords"] = boolToString(query.HasKeywords())
	tags["has_node_ids"] = boolToString(query.HasNodeIDs())
	
	// Track query complexity
	if r.config.EnableBusiness {
		r.metrics.RecordValue("business.query.keyword_count", float64(len(query.Keywords)), tags)
		r.metrics.RecordValue("business.query.node_id_count", float64(len(query.NodeIDs)), tags)
		r.metrics.RecordValue("business.query.limit", float64(query.Limit), tags)
	}
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.search.total", tags)
	}
	
	// Execute the operation
	nodes, err := r.inner.FindNodes(ctx, query)
	
	duration := time.Since(start)
	
	// Record performance metrics
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.search.duration", duration, tags)
		
		// Track query performance by complexity
		queryComplexity := r.calculateQueryComplexity(query)
		complexityTags := r.copyTags(tags)
		complexityTags["complexity"] = queryComplexity
		r.metrics.RecordDuration("repository.query.duration_by_complexity", duration, complexityTags)
	}
	
	// Handle results
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
		}
	} else {
		resultCount := len(nodes)
		
		// Track search results
		if r.config.EnableBusiness {
			r.metrics.RecordValue("business.search.result_count", float64(resultCount), tags)
			
			// Track search effectiveness
			if resultCount == 0 {
				r.metrics.IncrementCounter("business.search.no_results.total", tags)
			}
			
			// Track result set statistics
			if resultCount > 0 {
				r.metrics.SetGauge("business.search.avg_result_count", float64(resultCount), tags)
			}
		}
		
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
	}
	
	return nodes, err
}

// DeleteNode wraps node deletion with audit metrics
func (r *MetricsNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	start := time.Now()
	operation := "delete_node"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = userID
	
	// Track destructive operations carefully
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.delete.total", tags)
	}
	
	// Business metric: track deletions for data retention analysis
	if r.config.EnableBusiness {
		r.metrics.IncrementCounter("business.nodes.deleted.total", tags)
	}
	
	// Execute the operation
	err := r.inner.DeleteNode(ctx, userID, nodeID)
	
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.delete.duration", duration, tags)
	}
	
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
			
			// Track failed deletions specifically (important for data consistency)
			r.metrics.IncrementCounter("business.deletion.failed.total", errorTags)
		}
	} else {
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
		
		// Track successful deletions for audit purposes
		if r.config.EnableBusiness {
			r.metrics.IncrementCounter("business.deletion.successful.total", tags)
		}
	}
	
	return err
}

// BatchDeleteNodes wraps batch node deletion with metrics
func (r *MetricsNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	start := time.Now()
	operation := "batch_delete_nodes"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = userID
	tags["batch_size"] = fmt.Sprintf("%d", len(nodeIDs))
	
	// Track batch operations
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.batch_operations.total", tags)
	}
	
	// Business metric: track batch deletions
	if r.config.EnableBusiness {
		r.metrics.IncrementCounter("business.batch_deletions.total", tags)
		// Record batch size as a value metric
		r.metrics.RecordValue("business.batch_deletions.size", float64(len(nodeIDs)), tags)
	}
	
	// Execute the operation
	deleted, failed, err = r.inner.BatchDeleteNodes(ctx, userID, nodeIDs)
	
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.batch_operations.duration", duration, tags)
	}
	
	// Track success/failure counts
	resultTags := r.copyTags(tags)
	resultTags["deleted_count"] = fmt.Sprintf("%d", len(deleted))
	resultTags["failed_count"] = fmt.Sprintf("%d", len(failed))
	
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(resultTags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
			r.metrics.IncrementCounter("repository.batch_operations.errors.total", errorTags)
		}
	} else {
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", resultTags)
			r.metrics.IncrementCounter("repository.batch_operations.success.total", resultTags)
		}
	}
	
	// Track batch efficiency
	if r.config.EnableBusiness && len(nodeIDs) > 0 {
		efficiency := float64(len(deleted)) / float64(len(nodeIDs)) * 100
		// Record efficiency as a value metric
		r.metrics.RecordValue("business.batch_deletions.efficiency", efficiency, tags)
	}
	
	return deleted, failed, err
}

// GetNodesPage wraps paginated queries with pagination metrics
func (r *MetricsNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	start := time.Now()
	operation := "get_nodes_page"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = query.UserID
	tags["has_cursor"] = boolToString(pagination.HasCursor())
	
	// Track pagination patterns
	if r.config.EnableBusiness {
		r.metrics.RecordValue("business.pagination.page_size", float64(pagination.GetEffectiveLimit()), tags)
		r.metrics.RecordValue("business.pagination.offset", float64(pagination.Offset), tags)
		
		if pagination.HasCursor() {
			r.metrics.IncrementCounter("business.pagination.cursor_based.total", tags)
		} else {
			r.metrics.IncrementCounter("business.pagination.offset_based.total", tags)
		}
	}
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.page.total", tags)
	}
	
	// Execute the operation
	page, err := r.inner.GetNodesPage(ctx, query, pagination)
	
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.page.duration", duration, tags)
	}
	
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
		}
	} else {
		if page != nil && r.config.EnableBusiness {
			// Track pagination effectiveness
			r.metrics.RecordValue("business.pagination.items_returned", float64(len(page.Items)), tags)
			r.metrics.IncrementCounter("business.pagination.pages_served.total", tags)
			
			// Track pagination depth (how deep users paginate)
			currentPage := page.PageInfo.CurrentPage
			r.metrics.RecordValue("business.pagination.depth", float64(currentPage), tags)
			
			if currentPage > 10 {
				deepTags := r.copyTags(tags)
				deepTags["deep_pagination"] = "true"
				r.metrics.IncrementCounter("business.pagination.deep_pages.total", deepTags)
			}
		}
		
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
	}
	
	return page, err
}

// GetNodeNeighborhood wraps graph queries with network metrics
func (r *MetricsNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	start := time.Now()
	operation := "get_node_neighborhood"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = userID
	tags["depth"] = string(rune(depth + '0')) // Convert int to string
	
	// Track graph query patterns
	if r.config.EnableBusiness {
		r.metrics.RecordValue("business.graph.query_depth", float64(depth), tags)
		r.metrics.IncrementCounter("business.graph.neighborhood_queries.total", tags)
	}
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.graph.total", tags)
	}
	
	// Execute the operation
	graph, err := r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.graph.duration", duration, tags)
		
		// Track performance by graph size
		if graph != nil {
			sizeCategory := r.categorizeGraphSize(len(graph.Nodes))
			sizeTags := r.copyTags(tags)
			sizeTags["graph_size"] = sizeCategory
			r.metrics.RecordDuration("repository.graph.duration_by_size", duration, sizeTags)
		}
	}
	
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
		}
	} else {
		if graph != nil && r.config.EnableBusiness {
			// Track graph characteristics
			nodeCount := len(graph.Nodes)
			edgeCount := len(graph.Edges)
			
			r.metrics.RecordValue("business.graph.nodes_returned", float64(nodeCount), tags)
			r.metrics.RecordValue("business.graph.edges_returned", float64(edgeCount), tags)
			
			// Calculate graph density
			if nodeCount > 1 {
				maxPossibleEdges := nodeCount * (nodeCount - 1)
				density := float64(edgeCount) / float64(maxPossibleEdges)
				r.metrics.RecordValue("business.graph.density", density, tags)
			}
		}
		
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
	}
	
	return graph, err
}

// CountNodes wraps count operations with performance metrics
func (r *MetricsNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	start := time.Now()
	operation := "count_nodes"
	
	tags := r.buildBaseTags(operation)
	tags["user_id"] = userID
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operations.total", tags)
		r.metrics.IncrementCounter("repository.operations.count.total", tags)
	}
	
	// Execute the operation
	count, err := r.inner.CountNodes(ctx, userID)
	
	duration := time.Since(start)
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operations.duration", duration, tags)
		r.metrics.RecordDuration("repository.operations.count.duration", duration, tags)
	}
	
	if err != nil {
		if r.config.EnableErrors {
			errorTags := r.copyTags(tags)
			errorTags["error_type"] = r.classifyError(err)
			r.metrics.IncrementCounter("repository.operations.errors.total", errorTags)
		}
	} else {
		// Track user activity levels
		if r.config.EnableBusiness {
			r.metrics.SetGauge("business.users.node_count", float64(count), tags)
			
			// Categorize users by activity level
			activityLevel := r.categorizeActivityLevel(count)
			activityTags := r.copyTags(tags)
			activityTags["activity_level"] = activityLevel
			r.metrics.IncrementCounter("business.users.by_activity.total", activityTags)
		}
		
		if r.config.EnableThroughput {
			r.metrics.IncrementCounter("repository.operations.success.total", tags)
		}
	}
	
	return count, err
}

// Helper methods for metrics collection

func (r *MetricsNodeRepository) buildBaseTags(operation string) map[string]string {
	return map[string]string{
		"service":     r.config.ServiceName,
		"environment": r.config.Environment,
		"version":     r.config.Version,
		"operation":   operation,
		"repository":  "node",
	}
}

func (r *MetricsNodeRepository) copyTags(tags map[string]string) map[string]string {
	copied := make(map[string]string)
	for k, v := range tags {
		copied[k] = v
	}
	return copied
}

func (r *MetricsNodeRepository) classifyError(err error) string {
	if repository.IsNotFound(err) {
		return "not_found"
	}
	if repository.IsConflict(err) {
		return "conflict"
	}
	if repository.IsInvalidQuery(err) {
		return "invalid_query"
	}
	
	// Check for repository-specific error codes
	if repoErr, ok := err.(*repository.RepositoryError); ok {
		return string(repoErr.Code)
	}
	
	return "unknown"
}

func (r *MetricsNodeRepository) calculateQueryComplexity(query repository.NodeQuery) string {
	score := 0
	
	if len(query.Keywords) > 0 {
		score += len(query.Keywords)
	}
	if len(query.NodeIDs) > 0 {
		score += len(query.NodeIDs) * 2 // Node ID queries are more specific
	}
	if query.Limit > 100 {
		score += 5 // Large result sets are more complex
	}
	
	switch {
	case score <= 2:
		return "simple"
	case score <= 10:
		return "medium"
	default:
		return "complex"
	}
}

func (r *MetricsNodeRepository) categorizeGraphSize(nodeCount int) string {
	switch {
	case nodeCount <= 10:
		return "small"
	case nodeCount <= 100:
		return "medium"
	case nodeCount <= 1000:
		return "large"
	default:
		return "very_large"
	}
}

func (r *MetricsNodeRepository) categorizeActivityLevel(nodeCount int) string {
	switch {
	case nodeCount == 0:
		return "inactive"
	case nodeCount <= 10:
		return "light"
	case nodeCount <= 100:
		return "moderate"
	case nodeCount <= 1000:
		return "heavy"
	default:
		return "power_user"
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Phase 2 Enhanced Methods - Added for interface compatibility

// FindNodesWithOptions adds metrics collection to enhanced node queries with options
func (r *MetricsNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	start := time.Now()
	nodes, err := r.inner.FindNodesWithOptions(ctx, query, opts...)
	duration := time.Since(start)
	
	// Record metrics
	tags := map[string]string{
		"operation": "FindNodesWithOptions",
		"userID":    query.UserID,
		"success":   boolToString(err == nil),
	}
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operation.duration", duration, tags)
	}
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operation.count", tags)
	}
	
	if err != nil && r.config.EnableErrors {
		errorTags := map[string]string{
			"operation":  "FindNodesWithOptions",
			"error_type": fmt.Sprintf("%T", err),
		}
		r.metrics.IncrementCounter("repository.operation.errors", errorTags)
	}
	
	return nodes, err
}

// FindNodesPageWithOptions adds metrics collection to enhanced paginated node queries with options
func (r *MetricsNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	start := time.Now()
	page, err := r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
	duration := time.Since(start)
	
	// Record metrics
	tags := map[string]string{
		"operation": "FindNodesPageWithOptions",
		"userID":    query.UserID,
		"success":   boolToString(err == nil),
	}
	
	if r.config.EnableLatency {
		r.metrics.RecordDuration("repository.operation.duration", duration, tags)
	}
	
	if r.config.EnableThroughput {
		r.metrics.IncrementCounter("repository.operation.count", tags)
	}
	
	if err != nil && r.config.EnableErrors {
		errorTags := map[string]string{
			"operation":  "FindNodesPageWithOptions",
			"error_type": fmt.Sprintf("%T", err),
		}
		r.metrics.IncrementCounter("repository.operation.errors", errorTags)
	}
	
	return page, err
}