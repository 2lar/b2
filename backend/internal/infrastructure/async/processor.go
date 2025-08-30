// Package async provides asynchronous processing capabilities.
package async

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job represents an async job to be processed.
type Job struct {
	ID        string
	Type      string
	Payload   interface{}
	CreatedAt time.Time
	Retries   int
	MaxRetries int
}

// Result represents the result of an async job.
type Result struct {
	JobID     string
	Success   bool
	Data      interface{}
	Error     error
	ProcessedAt time.Time
}

// Handler is a function that processes a job.
type Handler func(ctx context.Context, job Job) (interface{}, error)

// Processor manages async job processing with worker pools.
type Processor struct {
	logger      *zap.Logger
	workers     int
	jobQueue    chan Job
	resultQueue chan Result
	handlers    map[string]Handler
	wg          sync.WaitGroup
	stopCh      chan struct{}
	
	// Metrics
	mu              sync.RWMutex
	processedCount  int64
	failedCount     int64
	averageTime     time.Duration
}

// NewProcessor creates a new async processor.
func NewProcessor(logger *zap.Logger, workers int, queueSize int) *Processor {
	return &Processor{
		logger:      logger,
		workers:     workers,
		jobQueue:    make(chan Job, queueSize),
		resultQueue: make(chan Result, queueSize),
		handlers:    make(map[string]Handler),
		stopCh:      make(chan struct{}),
	}
}

// RegisterHandler registers a handler for a specific job type.
func (p *Processor) RegisterHandler(jobType string, handler Handler) {
	p.handlers[jobType] = handler
	p.logger.Info("Registered handler", zap.String("type", jobType))
}

// Start starts the async processor with worker pool.
func (p *Processor) Start(ctx context.Context) {
	p.logger.Info("Starting async processor", zap.Int("workers", p.workers))
	
	// Start workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	
	// Start result processor
	p.wg.Add(1)
	go p.resultProcessor(ctx)
	
	p.logger.Info("Async processor started")
}

// Submit submits a job for async processing.
func (p *Processor) Submit(job Job) error {
	select {
	case p.jobQueue <- job:
		p.logger.Debug("Job submitted",
			zap.String("id", job.ID),
			zap.String("type", job.Type),
		)
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// SubmitWithTimeout submits a job with a timeout.
func (p *Processor) SubmitWithTimeout(job Job, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case p.jobQueue <- job:
		return nil
	case <-timer.C:
		return fmt.Errorf("submit timeout after %v", timeout)
	}
}

// worker processes jobs from the queue.
func (p *Processor) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	
	p.logger.Debug("Worker started", zap.Int("worker_id", id))
	
	for {
		select {
		case <-ctx.Done():
			p.logger.Debug("Worker stopping due to context", zap.Int("worker_id", id))
			return
			
		case <-p.stopCh:
			p.logger.Debug("Worker stopping", zap.Int("worker_id", id))
			return
			
		case job := <-p.jobQueue:
			p.processJob(ctx, job)
		}
	}
}

// processJob processes a single job.
func (p *Processor) processJob(ctx context.Context, job Job) {
	startTime := time.Now()
	
	p.logger.Debug("Processing job",
		zap.String("id", job.ID),
		zap.String("type", job.Type),
	)
	
	// Find handler
	handler, ok := p.handlers[job.Type]
	if !ok {
		p.sendResult(Result{
			JobID:       job.ID,
			Success:     false,
			Error:       fmt.Errorf("no handler for job type: %s", job.Type),
			ProcessedAt: time.Now(),
		})
		return
	}
	
	// Execute handler with timeout
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	resultCh := make(chan Result, 1)
	go func() {
		data, err := handler(jobCtx, job)
		resultCh <- Result{
			JobID:       job.ID,
			Success:     err == nil,
			Data:        data,
			Error:       err,
			ProcessedAt: time.Now(),
		}
	}()
	
	select {
	case result := <-resultCh:
		p.sendResult(result)
		p.updateMetrics(result.Success, time.Since(startTime))
		
	case <-jobCtx.Done():
		// Job timed out
		p.sendResult(Result{
			JobID:       job.ID,
			Success:     false,
			Error:       fmt.Errorf("job timeout"),
			ProcessedAt: time.Now(),
		})
		p.updateMetrics(false, time.Since(startTime))
	}
}

// sendResult sends a result to the result queue.
func (p *Processor) sendResult(result Result) {
	select {
	case p.resultQueue <- result:
		// Result sent
	default:
		// Result queue full, log and drop
		p.logger.Warn("Result queue full, dropping result",
			zap.String("job_id", result.JobID),
		)
	}
}

// resultProcessor processes job results.
func (p *Processor) resultProcessor(ctx context.Context) {
	defer p.wg.Done()
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case <-p.stopCh:
			return
			
		case result := <-p.resultQueue:
			if result.Success {
				p.logger.Debug("Job completed successfully",
					zap.String("job_id", result.JobID),
				)
			} else {
				p.logger.Error("Job failed",
					zap.String("job_id", result.JobID),
					zap.Error(result.Error),
				)
			}
		}
	}
}

// updateMetrics updates processing metrics.
func (p *Processor) updateMetrics(success bool, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.processedCount++
	if !success {
		p.failedCount++
	}
	
	// Update average time (simple moving average)
	if p.averageTime == 0 {
		p.averageTime = duration
	} else {
		p.averageTime = (p.averageTime + duration) / 2
	}
}

// GetMetrics returns current processing metrics.
func (p *Processor) GetMetrics() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	successRate := float64(0)
	if p.processedCount > 0 {
		successRate = float64(p.processedCount-p.failedCount) / float64(p.processedCount)
	}
	
	return map[string]interface{}{
		"processed_count": p.processedCount,
		"failed_count":    p.failedCount,
		"success_rate":    successRate,
		"average_time":    p.averageTime.String(),
		"queue_size":      len(p.jobQueue),
		"workers":         p.workers,
	}
}

// Stop stops the async processor.
func (p *Processor) Stop() {
	p.logger.Info("Stopping async processor")
	close(p.stopCh)
	p.wg.Wait()
	p.logger.Info("Async processor stopped")
}

// JobQueue provides a persistent job queue with priority support.
type JobQueue struct {
	highPriority   chan Job
	normalPriority chan Job
	lowPriority    chan Job
	logger         *zap.Logger
}

// NewJobQueue creates a new job queue with priority levels.
func NewJobQueue(size int, logger *zap.Logger) *JobQueue {
	return &JobQueue{
		highPriority:   make(chan Job, size/4),
		normalPriority: make(chan Job, size/2),
		lowPriority:    make(chan Job, size/4),
		logger:         logger,
	}
}

// Enqueue adds a job to the appropriate priority queue.
func (jq *JobQueue) Enqueue(job Job, priority string) error {
	var queue chan Job
	
	switch priority {
	case "high":
		queue = jq.highPriority
	case "low":
		queue = jq.lowPriority
	default:
		queue = jq.normalPriority
	}
	
	select {
	case queue <- job:
		return nil
	default:
		return fmt.Errorf("queue full for priority: %s", priority)
	}
}

// Dequeue retrieves the next job, respecting priority.
func (jq *JobQueue) Dequeue() (Job, bool) {
	// Try high priority first
	select {
	case job := <-jq.highPriority:
		return job, true
	default:
	}
	
	// Then normal priority
	select {
	case job := <-jq.normalPriority:
		return job, true
	default:
	}
	
	// Finally low priority
	select {
	case job := <-jq.lowPriority:
		return job, true
	default:
		return Job{}, false
	}
}

// BatchProcessor processes jobs in batches for efficiency.
type BatchProcessor struct {
	processor   *Processor
	batchSize   int
	batchTimeout time.Duration
	logger      *zap.Logger
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor(
	processor *Processor,
	batchSize int,
	batchTimeout time.Duration,
	logger *zap.Logger,
) *BatchProcessor {
	return &BatchProcessor{
		processor:    processor,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		logger:       logger,
	}
}

// ProcessBatch processes jobs in batches.
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, jobs []Job) []Result {
	results := make([]Result, 0, len(jobs))
	resultCh := make(chan Result, len(jobs))
	
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			
			// Submit job to processor
			if err := bp.processor.Submit(j); err != nil {
				resultCh <- Result{
					JobID:       j.ID,
					Success:     false,
					Error:       err,
					ProcessedAt: time.Now(),
				}
			}
		}(job)
	}
	
	// Wait for all jobs to be submitted
	go func() {
		wg.Wait()
		close(resultCh)
	}()
	
	// Collect results
	for result := range resultCh {
		results = append(results, result)
	}
	
	bp.logger.Debug("Batch processed",
		zap.Int("jobs", len(jobs)),
		zap.Int("results", len(results)),
	)
	
	return results
}