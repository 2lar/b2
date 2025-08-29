package concurrency

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// PoolMetrics collects and reports metrics for worker pools
type PoolMetrics struct {
	environment RuntimeEnvironment
	poolName    string
	
	// Counters (use atomic operations)
	tasksSubmitted   uint64
	tasksCompleted   uint64
	tasksFailed      uint64
	workerPanics     uint64
	poolStarts       uint64
	
	// Timing metrics
	taskDurations    []time.Duration
	durationMu       sync.RWMutex
	
	// Cold start tracking
	coldStartTime    time.Time
	coldStartRecorded bool
	
	// Queue metrics
	maxQueueDepth    int32
	currentQueueDepth int32
}

// NewPoolMetrics creates a new metrics collector
func NewPoolMetrics(env RuntimeEnvironment, poolName string) *PoolMetrics {
	return &PoolMetrics{
		environment:   env,
		poolName:      poolName,
		taskDurations: make([]time.Duration, 0, 100),
	}
}

// RecordTaskSubmission records that a task was submitted
func (m *PoolMetrics) RecordTaskSubmission() {
	atomic.AddUint64(&m.tasksSubmitted, 1)
}

// RecordTaskExecution records task execution metrics
func (m *PoolMetrics) RecordTaskExecution(duration time.Duration, err error) {
	if err != nil {
		atomic.AddUint64(&m.tasksFailed, 1)
	} else {
		atomic.AddUint64(&m.tasksCompleted, 1)
	}
	
	// Store duration for percentile calculations
	m.durationMu.Lock()
	m.taskDurations = append(m.taskDurations, duration)
	// Keep only last 1000 durations to avoid memory growth
	if len(m.taskDurations) > 1000 {
		m.taskDurations = m.taskDurations[len(m.taskDurations)-1000:]
	}
	m.durationMu.Unlock()
	
	// Emit metrics in Lambda environment
	if m.environment == EnvironmentLambda && shouldEmitMetrics() {
		m.emitTaskMetrics(duration, err)
	}
}

// RecordWorkerPanic records a worker panic event
func (m *PoolMetrics) RecordWorkerPanic(workerID int, panicValue interface{}) {
	atomic.AddUint64(&m.workerPanics, 1)
	
	if m.environment == EnvironmentLambda {
		// Log panic details for CloudWatch
		log.Printf("[METRIC] Worker panic - Pool: %s, Worker: %d, Panic: %v", 
			m.poolName, workerID, panicValue)
	}
}

// RecordPoolStart records pool initialization
func (m *PoolMetrics) RecordPoolStart(workerCount int) {
	atomic.AddUint64(&m.poolStarts, 1)
	
	if m.environment == EnvironmentLambda {
		log.Printf("[METRIC] Pool started - Pool: %s, Workers: %d", m.poolName, workerCount)
	}
}

// RecordColdStart records cold start duration
func (m *PoolMetrics) RecordColdStart(duration time.Duration) {
	if !m.coldStartRecorded {
		m.coldStartTime = time.Now().Add(-duration)
		m.coldStartRecorded = true
		
		if m.environment == EnvironmentLambda {
			// Emit cold start metric
			m.emitColdStartMetric(duration)
		}
	}
}

// UpdateQueueDepth updates current queue depth
func (m *PoolMetrics) UpdateQueueDepth(depth int) {
	atomic.StoreInt32(&m.currentQueueDepth, int32(depth))
	
	// Update max if needed
	for {
		oldMax := atomic.LoadInt32(&m.maxQueueDepth)
		if int32(depth) <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt32(&m.maxQueueDepth, oldMax, int32(depth)) {
			break
		}
	}
}

// GetSummary returns a summary of collected metrics
func (m *PoolMetrics) GetSummary() map[string]interface{} {
	m.durationMu.RLock()
	durations := make([]time.Duration, len(m.taskDurations))
	copy(durations, m.taskDurations)
	m.durationMu.RUnlock()
	
	summary := map[string]interface{}{
		"tasks_submitted":    atomic.LoadUint64(&m.tasksSubmitted),
		"tasks_completed":    atomic.LoadUint64(&m.tasksCompleted),
		"tasks_failed":       atomic.LoadUint64(&m.tasksFailed),
		"worker_panics":      atomic.LoadUint64(&m.workerPanics),
		"pool_starts":        atomic.LoadUint64(&m.poolStarts),
		"current_queue_depth": atomic.LoadInt32(&m.currentQueueDepth),
		"max_queue_depth":    atomic.LoadInt32(&m.maxQueueDepth),
	}
	
	// Calculate percentiles if we have data
	if len(durations) > 0 {
		summary["task_duration_p50"] = calculatePercentile(durations, 50).Milliseconds()
		summary["task_duration_p95"] = calculatePercentile(durations, 95).Milliseconds()
		summary["task_duration_p99"] = calculatePercentile(durations, 99).Milliseconds()
	}
	
	if m.coldStartRecorded {
		summary["time_since_cold_start"] = time.Since(m.coldStartTime).String()
	}
	
	return summary
}

// EmitFinalMetrics emits final metrics before shutdown
func (m *PoolMetrics) EmitFinalMetrics() {
	if m.environment != EnvironmentLambda {
		return
	}
	
	summary := m.GetSummary()
	
	// Emit final summary as CloudWatch EMF
	metric := map[string]interface{}{
		"_aws": map[string]interface{}{
			"Timestamp": time.Now().Unix() * 1000,
			"CloudWatchMetrics": []map[string]interface{}{
				{
					"Namespace": "Brain2/Concurrency",
					"Dimensions": [][]string{
						{"Environment", "PoolName"},
					},
					"Metrics": []map[string]interface{}{
						{"Name": "TasksCompleted", "Unit": "Count"},
						{"Name": "TasksFailed", "Unit": "Count"},
						{"Name": "WorkerPanics", "Unit": "Count"},
						{"Name": "MaxQueueDepth", "Unit": "Count"},
					},
				},
			},
		},
		"Environment":    string(m.environment),
		"PoolName":       m.poolName,
		"TasksCompleted": summary["tasks_completed"],
		"TasksFailed":    summary["tasks_failed"],
		"WorkerPanics":   summary["worker_panics"],
		"MaxQueueDepth":  summary["max_queue_depth"],
	}
	
	if jsonBytes, err := json.Marshal(metric); err == nil {
		fmt.Println(string(jsonBytes))
	}
}

// emitTaskMetrics emits metrics for a single task execution
func (m *PoolMetrics) emitTaskMetrics(duration time.Duration, err error) {
	success := 0
	if err == nil {
		success = 1
	}
	
	metric := map[string]interface{}{
		"_aws": map[string]interface{}{
			"Timestamp": time.Now().Unix() * 1000,
			"CloudWatchMetrics": []map[string]interface{}{
				{
					"Namespace": "Brain2/Concurrency",
					"Dimensions": [][]string{
						{"Environment", "PoolName"},
					},
					"Metrics": []map[string]interface{}{
						{"Name": "TaskDuration", "Unit": "Milliseconds"},
						{"Name": "TaskSuccess", "Unit": "Count"},
					},
				},
			},
		},
		"Environment":  string(m.environment),
		"PoolName":     m.poolName,
		"TaskDuration": duration.Milliseconds(),
		"TaskSuccess":  success,
	}
	
	if jsonBytes, err := json.Marshal(metric); err == nil {
		fmt.Println(string(jsonBytes))
	}
}

// emitColdStartMetric emits cold start duration metric
func (m *PoolMetrics) emitColdStartMetric(duration time.Duration) {
	metric := map[string]interface{}{
		"_aws": map[string]interface{}{
			"Timestamp": time.Now().Unix() * 1000,
			"CloudWatchMetrics": []map[string]interface{}{
				{
					"Namespace": "Brain2/Concurrency",
					"Dimensions": [][]string{
						{"Environment"},
					},
					"Metrics": []map[string]interface{}{
						{"Name": "ColdStartDuration", "Unit": "Milliseconds"},
					},
				},
			},
		},
		"Environment":        string(m.environment),
		"ColdStartDuration":  duration.Milliseconds(),
	}
	
	if jsonBytes, err := json.Marshal(metric); err == nil {
		fmt.Println(string(jsonBytes))
	}
}

// shouldEmitMetrics determines if metrics should be emitted
// This helps control metric volume in Lambda
func shouldEmitMetrics() bool {
	// Sample metrics - emit 10% of task metrics to control volume
	// Always emit cold start and error metrics
	return time.Now().UnixNano()%10 == 0
}

// calculatePercentile calculates the percentile value from a slice of durations
func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Create a copy and sort it to get accurate percentiles
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	
	// Sort durations in ascending order
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	
	// Calculate the percentile index
	// Using the nearest-rank method for percentile calculation
	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}
	
	return sorted[index]
}

// EmitMetrics outputs CloudWatch EMF format to stdout (for manual emission)
func (m *PoolMetrics) EmitMetrics(queueDepth int, taskDuration time.Duration, workerCount int) {
	if m.environment != EnvironmentLambda {
		return // Only emit in Lambda
	}
	
	// Calculate worker utilization
	optimalWorkers := GetOptimalWorkerCount(m.environment)
	utilization := float64(workerCount) / float64(optimalWorkers) * 100
	
	// EMF format - CloudWatch auto-captures from stdout
	metric := map[string]interface{}{
		"_aws": map[string]interface{}{
			"Timestamp": time.Now().Unix() * 1000,
			"CloudWatchMetrics": []map[string]interface{}{
				{
					"Namespace": "Brain2/Concurrency",
					"Dimensions": [][]string{
						{"Environment", "PoolName"},
					},
					"Metrics": []map[string]interface{}{
						{"Name": "QueueDepth", "Unit": "Count"},
						{"Name": "TaskDuration", "Unit": "Milliseconds"},
						{"Name": "WorkerUtilization", "Unit": "Percent"},
					},
				},
			},
		},
		"Environment":        string(m.environment),
		"PoolName":           m.poolName,
		"QueueDepth":         queueDepth,
		"TaskDuration":       taskDuration.Milliseconds(),
		"WorkerUtilization":  utilization,
	}
	
	// Output to stdout (Lambda captures automatically)
	if jsonBytes, err := json.Marshal(metric); err == nil {
		fmt.Println(string(jsonBytes))
	}
}