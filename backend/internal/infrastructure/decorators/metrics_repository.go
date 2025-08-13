// Package decorators - Metrics Repository Decorator
//
// This file demonstrates adding comprehensive metrics collection to repository operations
// using the Decorator pattern. The metrics decorator provides:
//   - Operation counters (success/failure)
//   - Performance histograms (latency distribution)
//   - Resource utilization tracking
//   - Custom business metrics
//   - Error rate monitoring
//
// Educational Goals:
//   - Show how to add observability without changing existing code
//   - Demonstrate metrics collection best practices
//   - Illustrate performance monitoring patterns
//   - Provide operational insights for production systems
package decorators

import (
	"context"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// MetricsCollector defines the interface for metrics collection
// This abstraction allows integration with different metrics systems (Prometheus, StatsD, etc.)
type MetricsCollector interface {
	// Counters - track the number of occurrences
	IncrementCounter(name string, tags map[string]string)
	
	// Gauges - track current values
	SetGauge(name string, value float64, tags map[string]string)
	
	// Histograms - track distributions of values (like latency)
	RecordHistogram(name string, value float64, tags map[string]string)
	
	// Timers - convenience method for recording durations
	RecordTimer(name string, duration time.Duration, tags map[string]string)
}

// MetricsNodeRepository is a decorator that adds comprehensive metrics collection
// to any NodeRepository implementation. This demonstrates how metrics can be
// added transparently without changing the repository interface or implementation.
//
// Collected Metrics:
//   - Operation counters (calls, successes, failures)
//   - Latency histograms for performance monitoring
//   - Resource metrics (nodes processed, cache hit rates)
//   - Business metrics (user activity, content analysis)
//   - Error categorization and tracking
type MetricsNodeRepository struct {
	// inner is the wrapped repository
	inner repository.NodeRepository
	
	// metrics is the metrics collection implementation
	metrics MetricsCollector
	
	// Common tags applied to all metrics
	baseTags map[string]string
	
	// Component identifier for metric naming
	component string
}

// NewMetricsNodeRepository creates a new metrics decorator for NodeRepository
func NewMetricsNodeRepository(
	inner repository.NodeRepository,
	metrics MetricsCollector,
	baseTags map[string]string,
) repository.NodeRepository {
	if baseTags == nil {
		baseTags = make(map[string]string)
	}
	
	// Add component tag if not present
	if _, exists := baseTags["component"]; !exists {
		baseTags["component"] = "node_repository"
	}
	
	return &MetricsNodeRepository{
		inner:     inner,
		metrics:   metrics,
		baseTags:  baseTags,
		component: "node_repository",
	}
}

// Helper methods for metrics collection

// recordMethodCall records basic method call metrics
func (r *MetricsNodeRepository) recordMethodCall(method string, tags map[string]string) {
	allTags := r.mergeTags(tags, map[string]string{"method": method})
	r.metrics.IncrementCounter("repository.calls.total", allTags)
}

// recordMethodSuccess records successful method completion
func (r *MetricsNodeRepository) recordMethodSuccess(method string, duration time.Duration, tags map[string]string) {
	allTags := r.mergeTags(tags, map[string]string{
		"method": method,
		"status": "success",
	})
	
	// Record success counter
	r.metrics.IncrementCounter("repository.calls.success", allTags)
	
	// Record latency histogram
	r.metrics.RecordTimer("repository.latency", duration, allTags)
	
	// Record latency histogram in milliseconds for better granularity
	r.metrics.RecordHistogram("repository.duration_ms", float64(duration.Milliseconds()), allTags)
}

// recordMethodError records method failures
func (r *MetricsNodeRepository) recordMethodError(method string, duration time.Duration, err error, tags map[string]string) {
	errorType := r.categorizeError(err)
	
	allTags := r.mergeTags(tags, map[string]string{
		"method":     method,
		"status":     "error",
		"error_type": errorType,
	})
	
	// Record error counter
	r.metrics.IncrementCounter("repository.calls.error", allTags)
	
	// Record error-specific counter
	r.metrics.IncrementCounter("repository.errors.by_type", allTags)
	
	// Still record latency for failed operations (useful for timeout analysis)
	r.metrics.RecordTimer("repository.latency", duration, allTags)
}

// recordBusinessMetrics records business-specific metrics
func (r *MetricsNodeRepository) recordBusinessMetrics(method string, result interface{}, tags map[string]string) {
	allTags := r.mergeTags(tags, map[string]string{"method": method})
	
	switch v := result.(type) {
	case *domain.Node:
		// Record node-related metrics
		r.metrics.IncrementCounter("repository.nodes.accessed", allTags)
		r.metrics.RecordHistogram("repository.node.content_length", float64(len(v.Content().String())), allTags)
		r.metrics.RecordHistogram("repository.node.keywords_count", float64(v.Keywords().Count()), allTags)
		r.metrics.RecordHistogram("repository.node.tags_count", float64(v.Tags().Count()), allTags)
		
	case []*domain.Node:
		// Record collection metrics
		count := len(v)
		r.metrics.IncrementCounter("repository.collections.returned", allTags)
		r.metrics.RecordHistogram("repository.collection.size", float64(count), allTags)
		
		if count > 0 {
			// Analyze the collection
			totalContentLength := 0
			totalKeywords := 0
			totalTags := 0
			
			for _, node := range v {
				totalContentLength += len(node.Content().String())
				totalKeywords += node.Keywords().Count()
				totalTags += node.Tags().Count()
			}
			
			// Record averages
			r.metrics.RecordHistogram("repository.collection.avg_content_length", float64(totalContentLength)/float64(count), allTags)
			r.metrics.RecordHistogram("repository.collection.avg_keywords", float64(totalKeywords)/float64(count), allTags)
			r.metrics.RecordHistogram("repository.collection.avg_tags", float64(totalTags)/float64(count), allTags)
		}
		
	case bool:
		// For existence checks
		if v {
			r.metrics.IncrementCounter("repository.existence.found", allTags)
		} else {
			r.metrics.IncrementCounter("repository.existence.not_found", allTags)
		}
		
	case int:
		// For count operations
		r.metrics.RecordHistogram("repository.count.result", float64(v), allTags)
	}
}

// categorizeError categorizes errors for better metrics granularity
func (r *MetricsNodeRepository) categorizeError(err error) string {
	if err == nil {
		return "none"
	}
	
	// Categorize by error type
	switch {
	case repository.IsNotFoundError(err):
		return "not_found"
	case repository.IsValidationError(err):
		return "validation"
	case repository.IsConflictError(err):
		return "conflict"
	case repository.IsTimeoutError(err):
		return "timeout"
	case repository.IsConnectionError(err):
		return "connection"
	default:
		return "unknown"
	}
}

// mergeTags combines base tags with method-specific tags
func (r *MetricsNodeRepository) mergeTags(methodTags map[string]string, additionalTags map[string]string) map[string]string {
	result := make(map[string]string)
	
	// Copy base tags
	for k, v := range r.baseTags {
		result[k] = v
	}
	
	// Copy method tags
	for k, v := range methodTags {
		result[k] = v
	}
	
	// Copy additional tags
	for k, v := range additionalTags {
		result[k] = v
	}
	
	return result
}

// Repository method implementations with metrics

// FindByID retrieves a node by ID with comprehensive metrics
func (r *MetricsNodeRepository) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	start := time.Now()
	method := "find_by_id"
	
	tags := map[string]string{
		"operation": "read",
	}
	
	r.recordMethodCall(method, tags)
	
	// Call the wrapped repository
	result, err := r.inner.FindByID(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return nil, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	return result, nil
}

// FindByUser retrieves nodes for a user with metrics
func (r *MetricsNodeRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "find_by_user"
	
	tags := map[string]string{
		"operation":  "read",
		"query_type": "user_nodes",
		"has_options": r.boolToString(len(opts) > 0),
	}
	
	r.recordMethodCall(method, tags)
	
	result, err := r.inner.FindByUser(ctx, userID, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return nil, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	return result, nil
}

// Exists checks if a node exists with metrics
func (r *MetricsNodeRepository) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	start := time.Now()
	method := "exists"
	
	tags := map[string]string{
		"operation": "read",
		"query_type": "existence_check",
	}
	
	r.recordMethodCall(method, tags)
	
	result, err := r.inner.Exists(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return false, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	return result, nil
}

// Count returns node count with metrics
func (r *MetricsNodeRepository) Count(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) (int, error) {
	start := time.Now()
	method := "count"
	
	tags := map[string]string{
		"operation":  "read",
		"query_type": "count",
		"has_options": r.boolToString(len(opts) > 0),
	}
	
	r.recordMethodCall(method, tags)
	
	result, err := r.inner.Count(ctx, userID, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return 0, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	return result, nil
}

// FindByKeywords searches by keywords with metrics
func (r *MetricsNodeRepository) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "find_by_keywords"
	
	tags := map[string]string{
		"operation":     "read",
		"query_type":    "keyword_search",
		"keyword_count": r.intToString(len(keywords)),
		"has_options":   r.boolToString(len(opts) > 0),
	}
	
	r.recordMethodCall(method, tags)
	
	// Record search-specific metrics
	r.metrics.RecordHistogram("repository.search.keywords_count", float64(len(keywords)), r.mergeTags(tags, nil))
	
	result, err := r.inner.FindByKeywords(ctx, userID, keywords, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return nil, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	// Record search result metrics
	resultCount := len(result)
	r.metrics.RecordHistogram("repository.search.results_count", float64(resultCount), r.mergeTags(tags, nil))
	
	// Calculate and record search effectiveness
	if len(keywords) > 0 {
		effectiveness := float64(resultCount) / float64(len(keywords))
		r.metrics.RecordHistogram("repository.search.effectiveness", effectiveness, r.mergeTags(tags, nil))
	}
	
	return result, nil
}

// FindSimilar finds similar nodes with metrics
func (r *MetricsNodeRepository) FindSimilar(ctx context.Context, node *domain.Node, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "find_similar"
	
	tags := map[string]string{
		"operation":   "read",
		"query_type":  "similarity_search",
		"has_options": r.boolToString(len(opts) > 0),
	}
	
	r.recordMethodCall(method, tags)
	
	result, err := r.inner.FindSimilar(ctx, node, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return nil, err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	r.recordBusinessMetrics(method, result, tags)
	
	// Record similarity search metrics
	r.metrics.RecordHistogram("repository.similarity.results_count", float64(len(result)), r.mergeTags(tags, nil))
	
	return result, nil
}

// Write operations with audit metrics

// Save creates or updates a node with metrics
func (r *MetricsNodeRepository) Save(ctx context.Context, node *domain.Node) error {
	start := time.Now()
	method := "save"
	
	tags := map[string]string{
		"operation": "write",
		"write_type": "upsert",
	}
	
	r.recordMethodCall(method, tags)
	
	// Record pre-save metrics
	r.metrics.RecordHistogram("repository.save.content_length", float64(len(node.Content().String())), r.mergeTags(tags, nil))
	r.metrics.RecordHistogram("repository.save.keywords_count", float64(node.Keywords().Count()), r.mergeTags(tags, nil))
	r.metrics.RecordHistogram("repository.save.tags_count", float64(node.Tags().Count()), r.mergeTags(tags, nil))
	
	err := r.inner.Save(ctx, node)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	
	// Record successful write metrics
	r.metrics.IncrementCounter("repository.writes.successful", r.mergeTags(tags, nil))
	
	return nil
}

// Delete removes a node with metrics
func (r *MetricsNodeRepository) Delete(ctx context.Context, id domain.NodeID) error {
	start := time.Now()
	method := "delete"
	
	tags := map[string]string{
		"operation": "write",
		"write_type": "delete",
	}
	
	r.recordMethodCall(method, tags)
	
	err := r.inner.Delete(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	
	// Record delete metrics
	r.metrics.IncrementCounter("repository.deletes.successful", r.mergeTags(tags, nil))
	
	return nil
}

// SaveBatch saves multiple nodes with batch metrics
func (r *MetricsNodeRepository) SaveBatch(ctx context.Context, nodes []*domain.Node) error {
	start := time.Now()
	method := "save_batch"
	
	batchSize := len(nodes)
	tags := map[string]string{
		"operation":  "write",
		"write_type": "batch_upsert",
		"batch_size": r.intToString(batchSize),
	}
	
	r.recordMethodCall(method, tags)
	
	// Record batch metrics
	r.metrics.RecordHistogram("repository.batch.size", float64(batchSize), r.mergeTags(tags, nil))
	
	// Analyze batch contents
	if batchSize > 0 {
		totalContent := 0
		totalKeywords := 0
		totalTags := 0
		
		for _, node := range nodes {
			totalContent += len(node.Content().String())
			totalKeywords += node.Keywords().Count()
			totalTags += node.Tags().Count()
		}
		
		r.metrics.RecordHistogram("repository.batch.total_content", float64(totalContent), r.mergeTags(tags, nil))
		r.metrics.RecordHistogram("repository.batch.avg_content", float64(totalContent)/float64(batchSize), r.mergeTags(tags, nil))
	}
	
	err := r.inner.SaveBatch(ctx, nodes)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	
	// Record batch success metrics
	r.metrics.IncrementCounter("repository.batches.successful", r.mergeTags(tags, nil))
	r.metrics.RecordHistogram("repository.batch.throughput", float64(batchSize)/duration.Seconds(), r.mergeTags(tags, nil))
	
	return nil
}

// DeleteBatch removes multiple nodes with batch metrics
func (r *MetricsNodeRepository) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	start := time.Now()
	method := "delete_batch"
	
	batchSize := len(ids)
	tags := map[string]string{
		"operation":  "write",
		"write_type": "batch_delete",
		"batch_size": r.intToString(batchSize),
	}
	
	r.recordMethodCall(method, tags)
	r.metrics.RecordHistogram("repository.batch.size", float64(batchSize), r.mergeTags(tags, nil))
	
	err := r.inner.DeleteBatch(ctx, ids)
	
	duration := time.Since(start)
	
	if err != nil {
		r.recordMethodError(method, duration, err, tags)
		return err
	}
	
	r.recordMethodSuccess(method, duration, tags)
	
	// Record batch delete metrics
	r.metrics.IncrementCounter("repository.batch_deletes.successful", r.mergeTags(tags, nil))
	r.metrics.RecordHistogram("repository.delete_batch.throughput", float64(batchSize)/duration.Seconds(), r.mergeTags(tags, nil))
	
	return nil
}

// Helper functions for tag values

func (r *MetricsNodeRepository) boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (r *MetricsNodeRepository) intToString(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return "10+"
}

// Periodic metrics collection (would be called by a background process)
func (r *MetricsNodeRepository) CollectHealthMetrics(ctx context.Context) {
	// This method could be called periodically to collect health metrics
	// For example, connection pool status, cache hit rates, etc.
	
	tags := r.mergeTags(nil, map[string]string{"health_check": "periodic"})
	
	// Record that health metrics were collected
	r.metrics.IncrementCounter("repository.health.metrics_collected", tags)
	
	// In a real implementation, you might:
	// - Check connection pool status
	// - Measure cache hit rates
	// - Check disk usage
	// - Monitor error rates
	// - Record uptime metrics
}

// Example usage showing composition with other decorators:
//
// // Create metrics collector (Prometheus, StatsD, etc.)
// metrics := prometheus.NewRegistry()
//
// // Create base repository
// baseRepo := dynamodb.NewNodeRepository(client)
//
// // Add metrics collection
// metricsRepo := NewMetricsNodeRepository(
//     baseRepo,
//     metrics,
//     map[string]string{
//         "service":     "brain2",
//         "environment": "production",
//         "version":     "1.0.0",
//     },
// )
//
// // Add logging on top of metrics
// loggedRepo := NewLoggingNodeRepository(metricsRepo, logger, LogLevelInfo, true, false)
//
// // Add caching on top of logging
// cachedRepo := NewCachingNodeRepository(loggedRepo, cache, 10*time.Minute, "brain2")
//
// // Final stack: Cache -> Logging -> Metrics -> Base Repository
// // Every operation is cached, logged, AND metered automatically!

// This demonstrates the power of the Decorator pattern for building
// comprehensive observability into your application architecture.