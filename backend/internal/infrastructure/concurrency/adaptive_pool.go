// Package concurrency provides Lambda-optimized goroutine pool management.
//
// LAMBDA CONCURRENCY CHALLENGES:
//
// Lambda environments have unique constraints that require adaptive concurrency patterns:
//   • MEMORY LIMITS: Fixed memory allocation per Lambda (128MB - 10GB)
//   • CPU CONSTRAINTS: CPU allocation scales with memory (1.77GB = 1 vCPU)
//   • TIMEOUT LIMITS: Maximum execution time of 15 minutes
//   • COLD START IMPACT: Goroutine creation overhead during cold starts
//
// ADAPTIVE POOL STRATEGY:
//
// The AdaptiveWorkerPool adjusts its behavior based on the deployment environment:
//
// LAMBDA ENVIRONMENT:
//   • Smaller worker pools to avoid memory pressure
//   • CPU-based sizing (workers = availableCPU * 2)
//   • Timeout-aware task processing
//   • Conservative memory allocation
//
// ECS ENVIRONMENT:
//   • Larger worker pools for sustained workloads
//   • Container resource-based sizing
//   • Longer-running task optimization
//
// LOCAL ENVIRONMENT:
//   • Development-friendly defaults
//   • Debugging and profiling support
//
// This approach ensures optimal performance across different deployment targets
// while preventing resource exhaustion in constrained Lambda environments.

package concurrency

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"brain2-backend/internal/errors"
)

// RuntimeEnvironment represents the deployment environment
type RuntimeEnvironment string

const (
	EnvironmentLambda RuntimeEnvironment = "lambda"
	EnvironmentECS    RuntimeEnvironment = "ecs"
	EnvironmentLocal  RuntimeEnvironment = "local"
)

// PoolConfig contains configuration for the worker pool
type PoolConfig struct {
	MaxWorkers    int
	BatchSize     int
	QueueSize     int
	Environment   RuntimeEnvironment
	MemoryMB      int // Available memory in MB (for Lambda)
	TimeoutBuffer int // Seconds to leave as buffer before timeout
}

// AdaptiveWorkerPool provides environment-aware concurrent execution
type AdaptiveWorkerPool struct {
	environment RuntimeEnvironment
	config      PoolConfig
	workers     int
	taskQueue   chan Task
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	running     bool
	poolStarted sync.Once // Ensures workers start only once
	metrics     *PoolMetrics
}

// Task represents a unit of work to be executed
type Task struct {
	ID       string
	Execute  func(ctx context.Context) error
	Callback func(id string, err error)
}

// DetectEnvironment automatically detects the runtime environment
func DetectEnvironment() RuntimeEnvironment {
	// Check for Lambda environment
	if _, exists := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME"); exists {
		return EnvironmentLambda
	}
	
	// Check for ECS environment
	if _, exists := os.LookupEnv("ECS_CONTAINER_METADATA_URI"); exists {
		return EnvironmentECS
	}
	
	// Check for ECS Fargate specifically
	if _, exists := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4"); exists {
		return EnvironmentECS
	}
	
	return EnvironmentLocal
}

// GetLambdaMemoryMB returns the configured memory for Lambda function
func GetLambdaMemoryMB() int {
	memStr := os.Getenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
	if memStr == "" {
		return 512 // Default Lambda memory
	}
	
	mem, err := strconv.Atoi(memStr)
	if err != nil {
		return 512
	}
	
	return mem
}

// GetOptimalWorkerCount returns the optimal number of workers for the environment
func GetOptimalWorkerCount(env RuntimeEnvironment) int {
	switch env {
	case EnvironmentLambda:
		// Lambda: Conservative concurrency based on memory
		memoryMB := GetLambdaMemoryMB()
		
		// Lambda gets 1 vCPU at ~1769 MB
		// Below that, we get fractional vCPU
		if memoryMB < 512 {
			return 2 // Minimal concurrency for small Lambda
		} else if memoryMB < 1024 {
			return 3 // Slightly more for medium Lambda
		} else if memoryMB < 1769 {
			return 4 // Good balance for larger Lambda
		} else if memoryMB < 3008 {
			return 6 // 2 vCPUs available
		}
		return 8 // Max practical concurrency for Lambda
		
	case EnvironmentECS:
		// ECS: Based on available CPU cores
		// Fargate tasks can have 0.25, 0.5, 1, 2, 4+ vCPUs
		cpuCount := runtime.NumCPU()
		
		// Use 4 workers per CPU core for I/O bound tasks
		// For CPU-bound tasks, you'd want to match CPU count
		workers := cpuCount * 4
		
		// Cap at reasonable maximum for ECS
		if workers > 40 {
			return 40
		}
		return workers
		
	default:
		// Local development: Reasonable default
		cpuCount := runtime.NumCPU()
		if cpuCount < 4 {
			return 8
		}
		
		workers := cpuCount * 2
		// Cap at reasonable maximum for local development
		if workers > 20 {
			return 20
		}
		return workers
	}
}

// GetOptimalBatchSize returns the optimal batch size for the environment
func GetOptimalBatchSize(env RuntimeEnvironment) int {
	switch env {
	case EnvironmentLambda:
		// Lambda: Smaller batches to complete within timeout
		// Also considers DynamoDB batch limits (25 items)
		return 25
		
	case EnvironmentECS:
		// ECS: Larger batches for better throughput
		return 100
		
	default:
		// Local: Moderate batch size
		return 50
	}
}

// NewAdaptiveWorkerPool creates a new environment-aware worker pool
func NewAdaptiveWorkerPool(ctx context.Context, config *PoolConfig) *AdaptiveWorkerPool {
	// Auto-detect environment if not specified
	if config.Environment == "" {
		config.Environment = DetectEnvironment()
	}
	
	// Set optimal defaults if not specified
	if config.MaxWorkers == 0 {
		config.MaxWorkers = GetOptimalWorkerCount(config.Environment)
	}
	
	if config.BatchSize == 0 {
		config.BatchSize = GetOptimalBatchSize(config.Environment)
	}
	
	if config.QueueSize == 0 {
		// Queue size based on environment
		switch config.Environment {
		case EnvironmentLambda:
			config.QueueSize = 100 // Smaller queue for Lambda
		case EnvironmentECS:
			config.QueueSize = 1000 // Larger queue for ECS
		default:
			config.QueueSize = 500
		}
	}
	
	poolCtx, cancel := context.WithCancel(ctx)
	
	pool := &AdaptiveWorkerPool{
		environment: config.Environment,
		config:      *config,
		workers:     config.MaxWorkers,
		taskQueue:   make(chan Task, config.QueueSize),
		ctx:         poolCtx,
		cancel:      cancel,
		running:     false,
		metrics:     NewPoolMetrics(config.Environment, "adaptive_pool"),
	}
	
	return pool
}

// Start initializes and starts the worker pool
func (p *AdaptiveWorkerPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.running {
		return errors.Conflict("POOL_ALREADY_RUNNING", "Worker pool is already running").
			WithOperation("Start").
			WithResource("worker_pool").
			Build()
	}
	
	// Start workers with panic recovery
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.workerWithRecovery(i)
	}
	
	p.running = true
	return nil
}

// startWorkersLazy starts workers on first task submission (Lambda optimization)
func (p *AdaptiveWorkerPool) startWorkersLazy() {
	p.poolStarted.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		
		if !p.running {
			// Only log in debug mode or non-Lambda environments
			if p.environment != EnvironmentLambda {
				log.Printf("Lazy-starting %d workers for %s environment", p.workers, p.environment)
			}
			
			// Start workers with panic recovery
			for i := 0; i < p.workers; i++ {
				p.wg.Add(1)
				go p.workerWithRecovery(i)
			}
			
			p.running = true
			
			if p.metrics != nil {
				p.metrics.RecordPoolStart(p.workers)
			}
		}
	})
}

// worker processes tasks from the queue (deprecated - use workerWithRecovery)
func (p *AdaptiveWorkerPool) worker(id int) {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
			
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			
			// Execute task with context
			err := task.Execute(p.ctx)
			
			// Call callback if provided
			if task.Callback != nil {
				task.Callback(task.ID, err)
			}
		}
	}
}

// workerWithRecovery processes tasks with panic recovery
func (p *AdaptiveWorkerPool) workerWithRecovery(id int) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d recovered from panic: %v", id, r)
			
			if p.metrics != nil {
				p.metrics.RecordWorkerPanic(id, r)
			}
			
			// Restart worker if pool is still running
			// Use write lock to ensure atomic check and restart
			p.mu.Lock()
			if p.running {
				log.Printf("Restarting worker %d after panic recovery", id)
				p.wg.Add(1)
				go p.workerWithRecovery(id)
			}
			p.mu.Unlock()
		}
	}()
	
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			return
			
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			
			start := time.Now()
			
			// Execute task with context
			err := task.Execute(p.ctx)
			
			// Record metrics
			if p.metrics != nil {
				p.metrics.RecordTaskExecution(time.Since(start), err)
			}
			
			// Call callback if provided
			if task.Callback != nil {
				task.Callback(task.ID, err)
			}
		}
	}
}

// Submit adds a task to the queue
func (p *AdaptiveWorkerPool) Submit(task Task) error {
	// Lazy-start workers on first submission (Lambda optimization)
	p.startWorkersLazy()
	
	// Check context first to avoid race with Stop()
	select {
	case <-p.ctx.Done():
		return errors.Conflict("POOL_SHUTTING_DOWN", "Worker pool is shutting down").
			WithOperation("Submit").
			WithResource("worker_pool").
			WithRetryable(true).
			Build()
	default:
		// Continue with submission
	}
	
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return errors.Conflict("POOL_NOT_RUNNING", "Worker pool is not running").
			WithOperation("Submit").
			WithResource("worker_pool").
			Build()
	}
	// Keep the lock until we've submitted to prevent Stop() from closing queue
	defer p.mu.RUnlock()
	
	select {
	case p.taskQueue <- task:
		return nil
		
	case <-p.ctx.Done():
		return errors.Conflict("POOL_SHUTTING_DOWN", "Worker pool is shutting down").
			WithOperation("Submit").
			WithResource("worker_pool").
			WithRetryable(true).
			Build()
		
	default:
		// Queue is full
		if p.environment == EnvironmentLambda {
			// In Lambda, we can't wait - fail fast
			return fmt.Errorf("task queue is full (Lambda environment)")
		}
		
		// In other environments, block until space is available
		select {
		case p.taskQueue <- task:
			return nil
		case <-p.ctx.Done():
			return errors.Conflict("POOL_SHUTTING_DOWN", "Worker pool is shutting down").
			WithOperation("Submit").
			WithResource("worker_pool").
			WithRetryable(true).
			Build()
		}
	}
}

// Stop gracefully shuts down the worker pool
func (p *AdaptiveWorkerPool) Stop() {
	p.mu.Lock()
	
	if !p.running {
		p.mu.Unlock()
		return
	}
	
	// Mark as not running immediately to prevent new submissions
	p.running = false
	
	// Signal shutdown
	p.cancel()
	
	// Close task queue
	close(p.taskQueue)
	
	// Unlock before waiting to avoid deadlock
	p.mu.Unlock()
	
	// Wait for workers to finish
	p.wg.Wait()
}

// Wait blocks until all submitted tasks are completed
func (p *AdaptiveWorkerPool) Wait() {
	p.wg.Wait()
}

// GetStats returns current pool statistics
func (p *AdaptiveWorkerPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	stats := map[string]interface{}{
		"environment":     string(p.environment),
		"workers":         p.workers,
		"queue_size":      len(p.taskQueue),
		"queue_capacity":  cap(p.taskQueue),
		"batch_size":      p.config.BatchSize,
		"running":         p.running,
	}
	
	if p.metrics != nil {
		stats["metrics"] = p.metrics.GetSummary()
	}
	
	return stats
}

// IsLambdaEnvironment returns true if running in Lambda
func (p *AdaptiveWorkerPool) IsLambdaEnvironment() bool {
	return p.environment == EnvironmentLambda
}

// IsECSEnvironment returns true if running in ECS/Fargate
func (p *AdaptiveWorkerPool) IsECSEnvironment() bool {
	return p.environment == EnvironmentECS
}