package observability

import (
	"context"
	"sync"
	"time"
	
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// EnhancedMetricsCollector provides advanced metrics collection capabilities
type EnhancedMetricsCollector struct {
	*Collector // Embed base collector
	
	// Additional metric instruments
	cacheHitRatio       metric.Float64Gauge
	queryLatency        metric.Float64Histogram
	connectionPoolUsage metric.Float64Gauge
	sagaSuccess         metric.Int64Counter
	sagaFailure         metric.Int64Counter
	compensationRate    metric.Float64Gauge
	
	// Cache metrics tracking
	cacheStats      *CacheStatistics
	cacheStatsMutex sync.RWMutex
	
	// Query performance tracking
	queryStats      *QueryStatistics
	queryStatsMutex sync.RWMutex
	
	logger *zap.Logger
}

// CacheStatistics tracks cache performance metrics
type CacheStatistics struct {
	TotalHits       int64
	TotalMisses     int64
	TotalEvictions  int64
	HitRatio        float64
	AverageLoadTime time.Duration
	LastReset       time.Time
}

// QueryStatistics tracks query performance metrics
type QueryStatistics struct {
	TotalQueries      int64
	SlowQueries       int64
	FailedQueries     int64
	AverageLatency    time.Duration
	P50Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	QueriesByType     map[string]int64
	LatencyByType     map[string]time.Duration
}

// NewEnhancedMetricsCollector creates a new enhanced metrics collector
func NewEnhancedMetricsCollector(meter metric.Meter, logger *zap.Logger) (*EnhancedMetricsCollector, error) {
	// Create base collector
	baseCollector := &Collector{
		meter: meter,
	}
	
	collector := &EnhancedMetricsCollector{
		Collector: baseCollector,
		cacheStats: &CacheStatistics{
			LastReset:     time.Now(),
			QueriesByType: make(map[string]int64),
			LatencyByType: make(map[string]time.Duration),
		},
		queryStats: &QueryStatistics{
			QueriesByType: make(map[string]int64),
			LatencyByType: make(map[string]time.Duration),
		},
		logger: logger,
	}
	
	// Initialize enhanced metrics
	if err := collector.initializeMetrics(); err != nil {
		return nil, err
	}
	
	return collector, nil
}

// initializeMetrics sets up all metric instruments
func (c *EnhancedMetricsCollector) initializeMetrics() error {
	var err error
	
	// Cache metrics
	c.cacheHitRatio, err = c.meter.Float64Gauge(
		"cache.hit_ratio",
		metric.WithDescription("Cache hit ratio (0-1)"),
		metric.WithUnit("ratio"),
	)
	if err != nil {
		return err
	}
	
	// Query performance metrics
	c.queryLatency, err = c.meter.Float64Histogram(
		"query.execution_time",
		metric.WithDescription("Query execution time in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}
	
	// Connection pool metrics
	c.connectionPoolUsage, err = c.meter.Float64Gauge(
		"connection_pool.utilization",
		metric.WithDescription("Connection pool utilization percentage"),
		metric.WithUnit("percent"),
	)
	if err != nil {
		return err
	}
	
	// Saga metrics
	c.sagaSuccess, err = c.meter.Int64Counter(
		"saga.completed",
		metric.WithDescription("Number of successfully completed sagas"),
	)
	if err != nil {
		return err
	}
	
	c.sagaFailure, err = c.meter.Int64Counter(
		"saga.failed",
		metric.WithDescription("Number of failed sagas"),
	)
	if err != nil {
		return err
	}
	
	// Compensation metrics
	c.compensationRate, err = c.meter.Float64Gauge(
		"saga.compensation_rate",
		metric.WithDescription("Rate of saga compensations per minute"),
		metric.WithUnit("1/min"),
	)
	if err != nil {
		return err
	}
	
	return nil
}

// RecordCacheMetrics records cache hit/miss metrics
func (c *EnhancedMetricsCollector) RecordCacheMetrics(hit, miss int) {
	c.cacheStatsMutex.Lock()
	defer c.cacheStatsMutex.Unlock()
	
	c.cacheStats.TotalHits += int64(hit)
	c.cacheStats.TotalMisses += int64(miss)
	
	total := c.cacheStats.TotalHits + c.cacheStats.TotalMisses
	if total > 0 {
		ratio := float64(c.cacheStats.TotalHits) / float64(total)
		c.cacheStats.HitRatio = ratio
		
		// Record to OpenTelemetry
		c.cacheHitRatio.Record(context.Background(), ratio)
	}
	
	// Log if hit ratio is low
	if c.cacheStats.HitRatio < 0.5 && total > 100 {
		c.logger.Warn("Low cache hit ratio detected",
			zap.Float64("hitRatio", c.cacheStats.HitRatio),
			zap.Int64("totalHits", c.cacheStats.TotalHits),
			zap.Int64("totalMisses", c.cacheStats.TotalMisses))
	}
}

// RecordCacheEviction records cache eviction events
func (c *EnhancedMetricsCollector) RecordCacheEviction(count int) {
	c.cacheStatsMutex.Lock()
	defer c.cacheStatsMutex.Unlock()
	
	c.cacheStats.TotalEvictions += int64(count)
}

// RecordQueryPerformance records query execution metrics
func (c *EnhancedMetricsCollector) RecordQueryPerformance(queryType string, duration time.Duration) {
	c.queryStatsMutex.Lock()
	defer c.queryStatsMutex.Unlock()
	
	// Update general statistics
	c.queryStats.TotalQueries++
	
	// Track by query type
	if c.queryStats.QueriesByType == nil {
		c.queryStats.QueriesByType = make(map[string]int64)
	}
	c.queryStats.QueriesByType[queryType]++
	
	// Update latency statistics
	if c.queryStats.LatencyByType == nil {
		c.queryStats.LatencyByType = make(map[string]time.Duration)
	}
	
	// Simple moving average for latency by type
	currentAvg := c.queryStats.LatencyByType[queryType]
	count := c.queryStats.QueriesByType[queryType]
	newAvg := (currentAvg*time.Duration(count-1) + duration) / time.Duration(count)
	c.queryStats.LatencyByType[queryType] = newAvg
	
	// Update overall average
	totalDuration := c.queryStats.AverageLatency * time.Duration(c.queryStats.TotalQueries-1)
	c.queryStats.AverageLatency = (totalDuration + duration) / time.Duration(c.queryStats.TotalQueries)
	
	// Track slow queries (> 1 second)
	if duration > time.Second {
		c.queryStats.SlowQueries++
		c.logger.Warn("Slow query detected",
			zap.String("queryType", queryType),
			zap.Duration("duration", duration))
	}
	
	// Record to OpenTelemetry
	c.queryLatency.Record(context.Background(), duration.Milliseconds(),
		metric.WithAttributes(
			metric.String("query_type", queryType),
		))
}

// RecordQueryFailure records failed query attempts
func (c *EnhancedMetricsCollector) RecordQueryFailure(queryType string, err error) {
	c.queryStatsMutex.Lock()
	defer c.queryStatsMutex.Unlock()
	
	c.queryStats.FailedQueries++
	
	c.logger.Error("Query failed",
		zap.String("queryType", queryType),
		zap.Error(err))
}

// RecordConnectionPoolStatus records connection pool metrics
func (c *EnhancedMetricsCollector) RecordConnectionPoolStatus(active, idle, total int) {
	utilization := float64(active) / float64(total) * 100
	
	c.connectionPoolUsage.Record(context.Background(), utilization)
	
	// Also record individual metrics
	c.SetGauge("connection_pool.active", float64(active), nil)
	c.SetGauge("connection_pool.idle", float64(idle), nil)
	c.SetGauge("connection_pool.total", float64(total), nil)
	
	// Alert on high utilization
	if utilization > 80 {
		c.logger.Warn("High connection pool utilization",
			zap.Float64("utilization", utilization),
			zap.Int("active", active),
			zap.Int("total", total))
	}
}

// RecordSagaCompletion records successful saga completion
func (c *EnhancedMetricsCollector) RecordSagaCompletion(sagaName string, duration time.Duration) {
	c.sagaSuccess.Add(context.Background(), 1,
		metric.WithAttributes(
			metric.String("saga_name", sagaName),
		))
	
	// Record duration as well
	c.RecordDuration("saga.duration", duration, map[string]string{
		"saga_name": sagaName,
		"status":    "completed",
	})
}

// RecordSagaFailure records saga failure
func (c *EnhancedMetricsCollector) RecordSagaFailure(sagaName string, duration time.Duration, err error) {
	c.sagaFailure.Add(context.Background(), 1,
		metric.WithAttributes(
			metric.String("saga_name", sagaName),
			metric.String("error_type", getErrorType(err)),
		))
	
	// Record duration
	c.RecordDuration("saga.duration", duration, map[string]string{
		"saga_name": sagaName,
		"status":    "failed",
	})
	
	c.logger.Error("Saga failed",
		zap.String("sagaName", sagaName),
		zap.Duration("duration", duration),
		zap.Error(err))
}

// RecordCompensation records saga compensation events
func (c *EnhancedMetricsCollector) RecordCompensation(sagaName string, stepName string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	c.IncrementCounter("saga.compensation", map[string]string{
		"saga_name": sagaName,
		"step_name": stepName,
		"status":    status,
	})
}

// UpdateCompensationRate updates the compensation rate metric
func (c *EnhancedMetricsCollector) UpdateCompensationRate(rate float64) {
	c.compensationRate.Record(context.Background(), rate)
}

// GetCacheStatistics returns current cache statistics
func (c *EnhancedMetricsCollector) GetCacheStatistics() CacheStatistics {
	c.cacheStatsMutex.RLock()
	defer c.cacheStatsMutex.RUnlock()
	
	return *c.cacheStats
}

// GetQueryStatistics returns current query statistics
func (c *EnhancedMetricsCollector) GetQueryStatistics() QueryStatistics {
	c.queryStatsMutex.RLock()
	defer c.queryStatsMutex.RUnlock()
	
	return *c.queryStats
}

// ResetCacheStatistics resets cache statistics
func (c *EnhancedMetricsCollector) ResetCacheStatistics() {
	c.cacheStatsMutex.Lock()
	defer c.cacheStatsMutex.Unlock()
	
	c.cacheStats = &CacheStatistics{
		LastReset: time.Now(),
	}
}

// ResetQueryStatistics resets query statistics
func (c *EnhancedMetricsCollector) ResetQueryStatistics() {
	c.queryStatsMutex.Lock()
	defer c.queryStatsMutex.Unlock()
	
	c.queryStats = &QueryStatistics{
		QueriesByType: make(map[string]int64),
		LatencyByType: make(map[string]time.Duration),
	}
}

// ExportMetrics exports all metrics for reporting
func (c *EnhancedMetricsCollector) ExportMetrics() MetricsReport {
	cacheStats := c.GetCacheStatistics()
	queryStats := c.GetQueryStatistics()
	
	return MetricsReport{
		Timestamp: time.Now(),
		Cache: CacheMetrics{
			HitRatio:       cacheStats.HitRatio,
			TotalHits:      cacheStats.TotalHits,
			TotalMisses:    cacheStats.TotalMisses,
			TotalEvictions: cacheStats.TotalEvictions,
		},
		Query: QueryMetrics{
			TotalQueries:   queryStats.TotalQueries,
			SlowQueries:    queryStats.SlowQueries,
			FailedQueries:  queryStats.FailedQueries,
			AverageLatency: queryStats.AverageLatency,
			QueriesByType:  queryStats.QueriesByType,
		},
	}
}

// MetricsReport contains exported metrics data
type MetricsReport struct {
	Timestamp time.Time     `json:"timestamp"`
	Cache     CacheMetrics  `json:"cache"`
	Query     QueryMetrics  `json:"query"`
}

// CacheMetrics contains cache-related metrics
type CacheMetrics struct {
	HitRatio       float64 `json:"hit_ratio"`
	TotalHits      int64   `json:"total_hits"`
	TotalMisses    int64   `json:"total_misses"`
	TotalEvictions int64   `json:"total_evictions"`
}

// QueryMetrics contains query-related metrics
type QueryMetrics struct {
	TotalQueries   int64                    `json:"total_queries"`
	SlowQueries    int64                    `json:"slow_queries"`
	FailedQueries  int64                    `json:"failed_queries"`
	AverageLatency time.Duration            `json:"average_latency"`
	QueriesByType  map[string]int64         `json:"queries_by_type"`
}

// Helper function to extract error type
func getErrorType(err error) string {
	if err == nil {
		return "none"
	}
	
	// Extract error type from error message or type
	// This is a simplified version - in production, you'd want more sophisticated error classification
	return "unknown"
}