package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Global metrics instance for singleton pattern
	globalCollector *Collector
	collectorMutex  sync.Mutex
)

// Collector holds all Prometheus metrics for the application
type Collector struct {
	// Registry for this collector instance
	registry *prometheus.Registry
	
	// HTTP metrics
	HTTPRequests   *prometheus.CounterVec
	HTTPDuration   *prometheus.HistogramVec
	
	// Business metrics
	NodesCreated   prometheus.Counter
	NodesDeleted   prometheus.Counter
	EdgesCreated   prometheus.Counter
	
	// Repository metrics
	DBOperations   *prometheus.CounterVec
	DBDuration     *prometheus.HistogramVec
	
	// Cache metrics
	CacheHits      prometheus.Counter
	CacheMisses    prometheus.Counter
}

// NewCollector creates a new metrics collector with the given namespace
func NewCollector(namespace string) *Collector {
	// Use singleton pattern to avoid duplicate registration in tests
	collectorMutex.Lock()
	defer collectorMutex.Unlock()
	
	// Return existing collector if already created
	if globalCollector != nil {
		return globalCollector
	}
	
	// Create a new registry for this collector
	registry := prometheus.NewRegistry()
	
	// Create metrics (not auto-registered)
	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "route", "status"},
	)
	
	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	
	nodesCreated := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "nodes_created_total",
			Help:      "Total number of nodes created",
		},
	)
	
	nodesDeleted := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "nodes_deleted_total",
			Help:      "Total number of nodes deleted",
		},
	)
	
	edgesCreated := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "edges_created_total",
			Help:      "Total number of edges created",
		},
	)
	
	dbOperations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "db_operations_total",
			Help:      "Total number of database operations",
		},
		[]string{"operation", "table", "status"},
	)
	
	dbDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "db_operation_duration_seconds",
			Help:      "Database operation duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation", "table"},
	)
	
	cacheHits := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
	)
	
	cacheMisses := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
	)
	
	// Register all metrics with the registry
	registry.MustRegister(
		httpRequests,
		httpDuration,
		nodesCreated,
		nodesDeleted,
		edgesCreated,
		dbOperations,
		dbDuration,
		cacheHits,
		cacheMisses,
	)
	
	// Create and store the collector
	globalCollector = &Collector{
		registry:     registry,
		HTTPRequests: httpRequests,
		HTTPDuration: httpDuration,
		NodesCreated: nodesCreated,
		NodesDeleted: nodesDeleted,
		EdgesCreated: edgesCreated,
		DBOperations: dbOperations,
		DBDuration:   dbDuration,
		CacheHits:    cacheHits,
		CacheMisses:  cacheMisses,
	}
	
	return globalCollector
}

// ResetForTesting resets the global collector for testing purposes
func ResetForTesting() {
	collectorMutex.Lock()
	defer collectorMutex.Unlock()
	globalCollector = nil
}

// IncrementCounter increments a counter metric by 1
func (c *Collector) IncrementCounter(name string, tags map[string]string) {
	c.IncrementCounterBy(name, 1, tags)
}

// IncrementCounterBy increments a counter metric by the specified value
func (c *Collector) IncrementCounterBy(name string, value float64, tags map[string]string) {
	// For now, we can route to existing metrics based on name
	switch name {
	case "nodes_created":
		c.NodesCreated.Add(value)
	case "nodes_deleted":
		c.NodesDeleted.Add(value)
	case "edges_created":
		c.EdgesCreated.Add(value)
	case "cache_hits":
		c.CacheHits.Add(value)
	case "cache_misses":
		c.CacheMisses.Add(value)
	}
	// For other metrics, we could add them dynamically or ignore
}

// SetGauge sets a gauge metric to the specified value
func (c *Collector) SetGauge(name string, value float64, tags map[string]string) {
	// Implementation for gauge metrics - can be extended as needed
}

// IncrementGauge increments a gauge metric by the specified value
func (c *Collector) IncrementGauge(name string, value float64, tags map[string]string) {
	// Implementation for gauge metrics - can be extended as needed
}

// RecordDistribution records a distribution metric
func (c *Collector) RecordDistribution(name string, value float64, tags map[string]string) {
	// Implementation for distribution metrics - can be extended as needed
}

// RecordDuration records a duration metric
func (c *Collector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
	// Implementation for duration metrics - can be extended as needed
}

// RecordValue records a generic value metric
func (c *Collector) RecordValue(name string, value float64, tags map[string]string) {
	// Implementation for value metrics - can be extended as needed
}

// GetRegistry returns the Prometheus registry for this collector
func (c *Collector) GetRegistry() *prometheus.Registry {
	return c.registry
}