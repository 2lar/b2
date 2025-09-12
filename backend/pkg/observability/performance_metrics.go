package observability

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PerformanceMetrics tracks performance metrics for the domain layer
type PerformanceMetrics struct {
	logger           *zap.Logger
	aggregateMetrics *AggregateMetrics
	queryMetrics     *QueryMetrics
	commandMetrics   *CommandMetrics
	mu               sync.RWMutex
}

// AggregateMetrics tracks aggregate-specific metrics
type AggregateMetrics struct {
	LoadTimes      map[string][]time.Duration // Aggregate type -> load times
	MemoryUsage    map[string][]int64        // Aggregate type -> memory usage in bytes
	NodeCounts     map[string][]int          // Graph ID -> node counts
	EdgeCounts     map[string][]int          // Graph ID -> edge counts
	LastMeasured   time.Time
}

// QueryMetrics tracks query performance
type QueryMetrics struct {
	ExecutionTimes map[string][]time.Duration // Query type -> execution times
	ResultSizes    map[string][]int          // Query type -> result sizes
	CacheHits      int64
	CacheMisses    int64
	LastMeasured   time.Time
}

// CommandMetrics tracks command performance
type CommandMetrics struct {
	ExecutionTimes map[string][]time.Duration // Command type -> execution times
	SuccessCount   map[string]int64          // Command type -> success count
	FailureCount   map[string]int64          // Command type -> failure count
	LastMeasured   time.Time
}

// NewPerformanceMetrics creates a new performance metrics tracker
func NewPerformanceMetrics(logger *zap.Logger) *PerformanceMetrics {
	return &PerformanceMetrics{
		logger: logger,
		aggregateMetrics: &AggregateMetrics{
			LoadTimes:    make(map[string][]time.Duration),
			MemoryUsage:  make(map[string][]int64),
			NodeCounts:   make(map[string][]int),
			EdgeCounts:   make(map[string][]int),
			LastMeasured: time.Now(),
		},
		queryMetrics: &QueryMetrics{
			ExecutionTimes: make(map[string][]time.Duration),
			ResultSizes:    make(map[string][]int),
			LastMeasured:   time.Now(),
		},
		commandMetrics: &CommandMetrics{
			ExecutionTimes: make(map[string][]time.Duration),
			SuccessCount:   make(map[string]int64),
			FailureCount:   make(map[string]int64),
			LastMeasured:   time.Now(),
		},
	}
}

// RecordAggregateLoad records the time taken to load an aggregate
func (m *PerformanceMetrics) RecordAggregateLoad(aggregateType string, loadTime time.Duration, memoryUsage int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only last 100 measurements for each type
	if len(m.aggregateMetrics.LoadTimes[aggregateType]) >= 100 {
		m.aggregateMetrics.LoadTimes[aggregateType] = m.aggregateMetrics.LoadTimes[aggregateType][1:]
	}
	m.aggregateMetrics.LoadTimes[aggregateType] = append(m.aggregateMetrics.LoadTimes[aggregateType], loadTime)

	if len(m.aggregateMetrics.MemoryUsage[aggregateType]) >= 100 {
		m.aggregateMetrics.MemoryUsage[aggregateType] = m.aggregateMetrics.MemoryUsage[aggregateType][1:]
	}
	m.aggregateMetrics.MemoryUsage[aggregateType] = append(m.aggregateMetrics.MemoryUsage[aggregateType], memoryUsage)

	m.aggregateMetrics.LastMeasured = time.Now()

	// Log if load time exceeds threshold
	if loadTime > 100*time.Millisecond {
		m.logger.Warn("Slow aggregate load detected",
			zap.String("aggregate_type", aggregateType),
			zap.Duration("load_time", loadTime),
			zap.Int64("memory_bytes", memoryUsage),
		)
	}
}

// RecordGraphSize records the size of a graph
func (m *PerformanceMetrics) RecordGraphSize(graphID string, nodeCount, edgeCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only last 100 measurements
	if len(m.aggregateMetrics.NodeCounts[graphID]) >= 100 {
		m.aggregateMetrics.NodeCounts[graphID] = m.aggregateMetrics.NodeCounts[graphID][1:]
	}
	m.aggregateMetrics.NodeCounts[graphID] = append(m.aggregateMetrics.NodeCounts[graphID], nodeCount)

	if len(m.aggregateMetrics.EdgeCounts[graphID]) >= 100 {
		m.aggregateMetrics.EdgeCounts[graphID] = m.aggregateMetrics.EdgeCounts[graphID][1:]
	}
	m.aggregateMetrics.EdgeCounts[graphID] = append(m.aggregateMetrics.EdgeCounts[graphID], edgeCount)

	// Alert if graph is getting too large
	if nodeCount > 10000 || edgeCount > 50000 {
		m.logger.Warn("Large graph detected",
			zap.String("graph_id", graphID),
			zap.Int("node_count", nodeCount),
			zap.Int("edge_count", edgeCount),
		)
	}
}

// RecordQueryExecution records query execution metrics
func (m *PerformanceMetrics) RecordQueryExecution(queryType string, executionTime time.Duration, resultSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only last 100 measurements
	if len(m.queryMetrics.ExecutionTimes[queryType]) >= 100 {
		m.queryMetrics.ExecutionTimes[queryType] = m.queryMetrics.ExecutionTimes[queryType][1:]
	}
	m.queryMetrics.ExecutionTimes[queryType] = append(m.queryMetrics.ExecutionTimes[queryType], executionTime)

	if len(m.queryMetrics.ResultSizes[queryType]) >= 100 {
		m.queryMetrics.ResultSizes[queryType] = m.queryMetrics.ResultSizes[queryType][1:]
	}
	m.queryMetrics.ResultSizes[queryType] = append(m.queryMetrics.ResultSizes[queryType], resultSize)

	m.queryMetrics.LastMeasured = time.Now()

	// Log slow queries
	if executionTime > 200*time.Millisecond {
		m.logger.Warn("Slow query detected",
			zap.String("query_type", queryType),
			zap.Duration("execution_time", executionTime),
			zap.Int("result_size", resultSize),
		)
	}
}

// RecordCacheHit records a cache hit
func (m *PerformanceMetrics) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryMetrics.CacheHits++
}

// RecordCacheMiss records a cache miss
func (m *PerformanceMetrics) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryMetrics.CacheMisses++
}

// RecordCommandExecution records command execution metrics
func (m *PerformanceMetrics) RecordCommandExecution(ctx context.Context, commandType string, executionTime time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only last 100 measurements
	if len(m.commandMetrics.ExecutionTimes[commandType]) >= 100 {
		m.commandMetrics.ExecutionTimes[commandType] = m.commandMetrics.ExecutionTimes[commandType][1:]
	}
	m.commandMetrics.ExecutionTimes[commandType] = append(m.commandMetrics.ExecutionTimes[commandType], executionTime)

	if err != nil {
		m.commandMetrics.FailureCount[commandType]++
		m.logger.Error("Command execution failed",
			zap.String("command_type", commandType),
			zap.Duration("execution_time", executionTime),
			zap.Error(err),
		)
	} else {
		m.commandMetrics.SuccessCount[commandType]++
	}

	m.commandMetrics.LastMeasured = time.Now()

	// Log slow commands
	if executionTime > 500*time.Millisecond {
		m.logger.Warn("Slow command detected",
			zap.String("command_type", commandType),
			zap.Duration("execution_time", executionTime),
			zap.Bool("success", err == nil),
		)
	}
}

// GetAggregateStats returns statistics for aggregate loading
func (m *PerformanceMetrics) GetAggregateStats(aggregateType string) AggregateStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	loadTimes := m.aggregateMetrics.LoadTimes[aggregateType]
	memoryUsages := m.aggregateMetrics.MemoryUsage[aggregateType]

	if len(loadTimes) == 0 {
		return AggregateStats{}
	}

	return AggregateStats{
		AverageLoadTime:   calculateAverageDuration(loadTimes),
		MaxLoadTime:       calculateMaxDuration(loadTimes),
		MinLoadTime:       calculateMinDuration(loadTimes),
		AverageMemoryUsage: calculateAverageInt64(memoryUsages),
		MaxMemoryUsage:    calculateMaxInt64(memoryUsages),
		SampleCount:       len(loadTimes),
	}
}

// GetQueryStats returns statistics for query execution
func (m *PerformanceMetrics) GetQueryStats(queryType string) QueryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	executionTimes := m.queryMetrics.ExecutionTimes[queryType]
	resultSizes := m.queryMetrics.ResultSizes[queryType]

	if len(executionTimes) == 0 {
		return QueryStats{}
	}

	cacheHitRate := float64(0)
	if m.queryMetrics.CacheHits+m.queryMetrics.CacheMisses > 0 {
		cacheHitRate = float64(m.queryMetrics.CacheHits) / float64(m.queryMetrics.CacheHits+m.queryMetrics.CacheMisses)
	}

	return QueryStats{
		AverageExecutionTime: calculateAverageDuration(executionTimes),
		MaxExecutionTime:     calculateMaxDuration(executionTimes),
		MinExecutionTime:     calculateMinDuration(executionTimes),
		AverageResultSize:    calculateAverageInt(resultSizes),
		CacheHitRate:         cacheHitRate,
		SampleCount:          len(executionTimes),
	}
}

// GetCommandStats returns statistics for command execution
func (m *PerformanceMetrics) GetCommandStats(commandType string) CommandStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	executionTimes := m.commandMetrics.ExecutionTimes[commandType]
	successCount := m.commandMetrics.SuccessCount[commandType]
	failureCount := m.commandMetrics.FailureCount[commandType]

	if len(executionTimes) == 0 {
		return CommandStats{}
	}

	successRate := float64(0)
	if successCount+failureCount > 0 {
		successRate = float64(successCount) / float64(successCount+failureCount)
	}

	return CommandStats{
		AverageExecutionTime: calculateAverageDuration(executionTimes),
		MaxExecutionTime:     calculateMaxDuration(executionTimes),
		MinExecutionTime:     calculateMinDuration(executionTimes),
		SuccessRate:          successRate,
		SuccessCount:         successCount,
		FailureCount:         failureCount,
		SampleCount:          len(executionTimes),
	}
}

// GetGraphStats returns statistics for a specific graph
func (m *PerformanceMetrics) GetGraphStats(graphID string) GraphStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodeCounts := m.aggregateMetrics.NodeCounts[graphID]
	edgeCounts := m.aggregateMetrics.EdgeCounts[graphID]

	if len(nodeCounts) == 0 {
		return GraphStats{}
	}

	return GraphStats{
		AverageNodeCount: calculateAverageInt(nodeCounts),
		MaxNodeCount:     calculateMaxInt(nodeCounts),
		CurrentNodeCount: nodeCounts[len(nodeCounts)-1],
		AverageEdgeCount: calculateAverageInt(edgeCounts),
		MaxEdgeCount:     calculateMaxInt(edgeCounts),
		CurrentEdgeCount: edgeCounts[len(edgeCounts)-1],
		SampleCount:      len(nodeCounts),
	}
}

// ReportMetrics generates a comprehensive metrics report
func (m *PerformanceMetrics) ReportMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("Performance Metrics Report",
		zap.Time("aggregate_last_measured", m.aggregateMetrics.LastMeasured),
		zap.Time("query_last_measured", m.queryMetrics.LastMeasured),
		zap.Time("command_last_measured", m.commandMetrics.LastMeasured),
		zap.Int64("cache_hits", m.queryMetrics.CacheHits),
		zap.Int64("cache_misses", m.queryMetrics.CacheMisses),
	)

	// Report aggregate metrics
	for aggregateType := range m.aggregateMetrics.LoadTimes {
		stats := m.GetAggregateStats(aggregateType)
		m.logger.Info("Aggregate Performance",
			zap.String("type", aggregateType),
			zap.Duration("avg_load_time", stats.AverageLoadTime),
			zap.Int64("avg_memory_bytes", stats.AverageMemoryUsage),
		)
	}

	// Report query metrics
	for queryType := range m.queryMetrics.ExecutionTimes {
		stats := m.GetQueryStats(queryType)
		m.logger.Info("Query Performance",
			zap.String("type", queryType),
			zap.Duration("avg_execution_time", stats.AverageExecutionTime),
			zap.Float64("cache_hit_rate", stats.CacheHitRate),
		)
	}

	// Report command metrics
	for commandType := range m.commandMetrics.ExecutionTimes {
		stats := m.GetCommandStats(commandType)
		m.logger.Info("Command Performance",
			zap.String("type", commandType),
			zap.Duration("avg_execution_time", stats.AverageExecutionTime),
			zap.Float64("success_rate", stats.SuccessRate),
		)
	}
}

// Stats structures

type AggregateStats struct {
	AverageLoadTime    time.Duration
	MaxLoadTime        time.Duration
	MinLoadTime        time.Duration
	AverageMemoryUsage int64
	MaxMemoryUsage     int64
	SampleCount        int
}

type QueryStats struct {
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
	MinExecutionTime     time.Duration
	AverageResultSize    int
	CacheHitRate         float64
	SampleCount          int
}

type CommandStats struct {
	AverageExecutionTime time.Duration
	MaxExecutionTime     time.Duration
	MinExecutionTime     time.Duration
	SuccessRate          float64
	SuccessCount         int64
	FailureCount         int64
	SampleCount          int
}

type GraphStats struct {
	AverageNodeCount int
	MaxNodeCount     int
	CurrentNodeCount int
	AverageEdgeCount int
	MaxEdgeCount     int
	CurrentEdgeCount int
	SampleCount      int
}

// Helper functions

func calculateAverageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func calculateMaxDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

func calculateMinDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

func calculateAverageInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return sum / int64(len(values))
}

func calculateMaxInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func calculateAverageInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func calculateMaxInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}