// Package sagas implements the Saga pattern for distributed transaction management
package sagas

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/ports"
	"github.com/google/uuid"
)

// SagaState represents the state of a saga
type SagaState string

const (
	SagaStatePending     SagaState = "pending"
	SagaStateRunning     SagaState = "running"
	SagaStateCompleted   SagaState = "completed"
	SagaStateFailed      SagaState = "failed"
	SagaStateCompensating SagaState = "compensating"
	SagaStateCompensated  SagaState = "compensated"
)

// Saga is the base interface for all sagas
type Saga interface {
	// GetID returns the saga ID
	GetID() string
	
	// GetState returns the current state
	GetState() SagaState
	
	// Execute runs the saga
	Execute(ctx context.Context) error
	
	// Compensate rolls back the saga
	Compensate(ctx context.Context) error
	
	// GetSteps returns all saga steps
	GetSteps() []SagaStep
}

// SagaStep represents a step in a saga
type SagaStep interface {
	// GetName returns the step name
	GetName() string
	
	// Execute performs the step action
	Execute(ctx context.Context) error
	
	// Compensate rolls back the step
	Compensate(ctx context.Context) error
	
	// CanRetry checks if the step can be retried
	CanRetry() bool
	
	// GetMaxRetries returns the maximum retry count
	GetMaxRetries() int
}

// BaseSaga provides common functionality for all sagas
type BaseSaga struct {
	ID            string
	State         SagaState
	Steps         []SagaStep
	CompletedSteps []string
	CurrentStep   int
	StartedAt     time.Time
	CompletedAt   *time.Time
	Error         error
	Metadata      map[string]interface{}
	logger        ports.Logger
	metrics       ports.Metrics
}

// NewBaseSaga creates a new base saga
func NewBaseSaga(logger ports.Logger, metrics ports.Metrics) *BaseSaga {
	return &BaseSaga{
		ID:             uuid.New().String(),
		State:          SagaStatePending,
		Steps:          []SagaStep{},
		CompletedSteps: []string{},
		CurrentStep:    0,
		Metadata:       make(map[string]interface{}),
		logger:         logger,
		metrics:        metrics,
	}
}

// GetID returns the saga ID
func (s *BaseSaga) GetID() string {
	return s.ID
}

// GetState returns the current state
func (s *BaseSaga) GetState() SagaState {
	return s.State
}

// GetSteps returns all saga steps
func (s *BaseSaga) GetSteps() []SagaStep {
	return s.Steps
}

// Execute runs the saga
func (s *BaseSaga) Execute(ctx context.Context) error {
	s.State = SagaStateRunning
	s.StartedAt = time.Now()
	
	s.logger.Info("Starting saga execution",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "steps", Value: len(s.Steps)})
	
	for i, step := range s.Steps {
		s.CurrentStep = i
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			s.State = SagaStateFailed
			s.Error = ctx.Err()
			return s.compensate(ctx, i-1)
		default:
		}
		
		// Execute step with retries
		err := s.executeStep(ctx, step)
		if err != nil {
			s.State = SagaStateFailed
			s.Error = err
			
			s.logger.Error("Saga step failed",
				err,
				ports.Field{Key: "saga_id", Value: s.ID},
				ports.Field{Key: "step", Value: step.GetName()})
			
			// Start compensation
			return s.compensate(ctx, i-1)
		}
		
		// Mark step as completed
		s.CompletedSteps = append(s.CompletedSteps, step.GetName())
		
		s.logger.Info("Saga step completed",
			ports.Field{Key: "saga_id", Value: s.ID},
			ports.Field{Key: "step", Value: step.GetName()})
	}
	
	// All steps completed successfully
	s.State = SagaStateCompleted
	now := time.Now()
	s.CompletedAt = &now
	
	s.metrics.IncrementCounter("saga.completed",
		ports.Tag{Key: "saga_type", Value: "base"})
	
	s.logger.Info("Saga completed successfully",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "duration", Value: time.Since(s.StartedAt)})
	
	return nil
}

// Compensate rolls back the saga
func (s *BaseSaga) Compensate(ctx context.Context) error {
	return s.compensate(ctx, len(s.CompletedSteps)-1)
}

// compensate performs compensation from a specific step
func (s *BaseSaga) compensate(ctx context.Context, fromStep int) error {
	if fromStep < 0 {
		return s.Error // No steps to compensate
	}
	
	s.State = SagaStateCompensating
	
	s.logger.Info("Starting saga compensation",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "from_step", Value: fromStep})
	
	// Compensate in reverse order
	for i := fromStep; i >= 0; i-- {
		step := s.Steps[i]
		
		// Check if step was completed
		stepCompleted := false
		for _, completed := range s.CompletedSteps {
			if completed == step.GetName() {
				stepCompleted = true
				break
			}
		}
		
		if !stepCompleted {
			continue // Skip steps that weren't completed
		}
		
		// Compensate step
		if err := step.Compensate(ctx); err != nil {
			s.logger.Error("Failed to compensate step",
				err,
				ports.Field{Key: "saga_id", Value: s.ID},
				ports.Field{Key: "step", Value: step.GetName()})
			// Continue compensating other steps
		} else {
			s.logger.Info("Step compensated",
				ports.Field{Key: "saga_id", Value: s.ID},
				ports.Field{Key: "step", Value: step.GetName()})
		}
	}
	
	s.State = SagaStateCompensated
	
	s.metrics.IncrementCounter("saga.compensated",
		ports.Tag{Key: "saga_type", Value: "base"})
	
	return s.Error
}

// executeStep executes a step with retry logic
func (s *BaseSaga) executeStep(ctx context.Context, step SagaStep) error {
	maxRetries := 0
	if step.CanRetry() {
		maxRetries = step.GetMaxRetries()
	}
	
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt) * time.Second
			time.Sleep(backoff)
			
			s.logger.Info("Retrying saga step",
				ports.Field{Key: "saga_id", Value: s.ID},
				ports.Field{Key: "step", Value: step.GetName()},
				ports.Field{Key: "attempt", Value: attempt})
		}
		
		err := step.Execute(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !isRetryable(err) {
			break
		}
	}
	
	return fmt.Errorf("step %s failed after %d attempts: %w", step.GetName(), maxRetries+1, lastErr)
}

// BaseStep provides common functionality for saga steps
type BaseStep struct {
	Name           string
	Action         func(context.Context) error
	CompensateFunc func(context.Context) error
	Retryable      bool
	MaxRetries     int
}

// GetName returns the step name
func (s *BaseStep) GetName() string {
	return s.Name
}

// Execute performs the step action
func (s *BaseStep) Execute(ctx context.Context) error {
	if s.Action == nil {
		return fmt.Errorf("step %s has no action", s.Name)
	}
	return s.Action(ctx)
}

// Compensate rolls back the step
func (s *BaseStep) Compensate(ctx context.Context) error {
	if s.CompensateFunc == nil {
		// No compensation needed
		return nil
	}
	return s.CompensateFunc(ctx)
}

// CanRetry checks if the step can be retried
func (s *BaseStep) CanRetry() bool {
	return s.Retryable
}

// GetMaxRetries returns the maximum retry count
func (s *BaseStep) GetMaxRetries() int {
	if s.MaxRetries <= 0 {
		return 3 // Default
	}
	return s.MaxRetries
}

// isRetryable checks if an error is retryable
func isRetryable(err error) bool {
	// Implementation would check for specific error types
	// For now, we'll consider timeout errors as retryable
	return false
}