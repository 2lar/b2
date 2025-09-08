package sagas

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SagaStep represents a single step in a saga
type SagaStep struct {
	Name         string
	Execute      func(ctx context.Context, data interface{}) (interface{}, error)
	Compensate   func(ctx context.Context, data interface{}) error
	MaxRetries   int
	RetryDelay   time.Duration
}

// SagaState represents the current state of a saga execution
type SagaState string

const (
	SagaStatePending    SagaState = "PENDING"
	SagaStateRunning    SagaState = "RUNNING"
	SagaStateCompleted  SagaState = "COMPLETED"
	SagaStateFailed     SagaState = "FAILED"
	SagaStateCompensating SagaState = "COMPENSATING"
	SagaStateCompensated  SagaState = "COMPENSATED"
)

// Saga orchestrates a series of steps with compensation logic
type Saga struct {
	id            string
	name          string
	steps         []SagaStep
	compensations []func(ctx context.Context) error
	state         SagaState
	currentStep   int
	logger        *zap.Logger
	metadata      map[string]interface{}
}

// NewSaga creates a new saga instance
func NewSaga(name string, logger *zap.Logger) *Saga {
	return &Saga{
		id:            generateSagaID(),
		name:          name,
		steps:         make([]SagaStep, 0),
		compensations: make([]func(ctx context.Context) error, 0),
		state:         SagaStatePending,
		currentStep:   0,
		logger:        logger,
		metadata:      make(map[string]interface{}),
	}
}

// AddStep adds a step to the saga
func (s *Saga) AddStep(step SagaStep) *Saga {
	s.steps = append(s.steps, step)
	return s
}

// SetMetadata sets metadata for the saga
func (s *Saga) SetMetadata(key string, value interface{}) *Saga {
	s.metadata[key] = value
	return s
}

// Execute runs the saga
func (s *Saga) Execute(ctx context.Context, initialData interface{}) (interface{}, error) {
	s.state = SagaStateRunning
	s.logger.Info("Starting saga execution",
		zap.String("saga_id", s.id),
		zap.String("saga_name", s.name),
		zap.Int("total_steps", len(s.steps)),
	)

	var data interface{} = initialData
	completedSteps := 0

	for i, step := range s.steps {
		s.currentStep = i
		s.logger.Debug("Executing saga step",
			zap.String("saga_id", s.id),
			zap.String("step_name", step.Name),
			zap.Int("step_number", i+1),
		)

		// Execute step with retry logic
		result, err := s.executeStepWithRetry(ctx, step, data)
		if err != nil {
			s.state = SagaStateFailed
			s.logger.Error("Saga step failed",
				zap.String("saga_id", s.id),
				zap.String("step_name", step.Name),
				zap.Error(err),
			)

			// Start compensation
			if compensateErr := s.compensate(ctx, completedSteps); compensateErr != nil {
				s.logger.Error("Saga compensation failed",
					zap.String("saga_id", s.id),
					zap.Error(compensateErr),
				)
				return nil, fmt.Errorf("saga %s failed at step %s and compensation failed: %w", s.name, step.Name, err)
			}

			s.state = SagaStateCompensated
			return nil, fmt.Errorf("saga %s failed at step %s: %w", s.name, step.Name, err)
		}

		data = result
		completedSteps = i + 1

		// Register compensation for this step if available
		if step.Compensate != nil {
			stepData := data // Capture current data for compensation
			s.compensations = append(s.compensations, func(ctx context.Context) error {
				return step.Compensate(ctx, stepData)
			})
		}

		s.logger.Debug("Saga step completed successfully",
			zap.String("saga_id", s.id),
			zap.String("step_name", step.Name),
		)
	}

	s.state = SagaStateCompleted
	s.logger.Info("Saga completed successfully",
		zap.String("saga_id", s.id),
		zap.String("saga_name", s.name),
		zap.Int("completed_steps", completedSteps),
	)

	return data, nil
}

// executeStepWithRetry executes a step with retry logic
func (s *Saga) executeStepWithRetry(ctx context.Context, step SagaStep, data interface{}) (interface{}, error) {
	maxRetries := step.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1 // At least try once
	}

	retryDelay := step.RetryDelay
	if retryDelay == 0 {
		retryDelay = time.Second // Default retry delay
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Debug("Retrying saga step",
				zap.String("saga_id", s.id),
				zap.String("step_name", step.Name),
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", maxRetries),
			)
			time.Sleep(retryDelay)
		}

		result, err := step.Execute(ctx, data)
		if err == nil {
			return result, nil
		}

		lastErr = err
		s.logger.Warn("Saga step execution failed",
			zap.String("saga_id", s.id),
			zap.String("step_name", step.Name),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
		)
	}

	return nil, fmt.Errorf("step %s failed after %d attempts: %w", step.Name, maxRetries, lastErr)
}

// compensate runs compensation logic in reverse order
func (s *Saga) compensate(ctx context.Context, steps int) error {
	s.state = SagaStateCompensating
	s.logger.Info("Starting saga compensation",
		zap.String("saga_id", s.id),
		zap.String("saga_name", s.name),
		zap.Int("steps_to_compensate", steps),
	)

	// Execute compensations in reverse order
	for i := steps - 1; i >= 0; i-- {
		if i < len(s.compensations) && s.compensations[i] != nil {
			s.logger.Debug("Executing compensation",
				zap.String("saga_id", s.id),
				zap.Int("step_number", i+1),
			)

			if err := s.compensations[i](ctx); err != nil {
				s.logger.Error("Compensation failed",
					zap.String("saga_id", s.id),
					zap.Int("step_number", i+1),
					zap.Error(err),
				)
				// Continue compensating other steps even if one fails
			}
		}
	}

	return nil
}

// GetState returns the current state of the saga
func (s *Saga) GetState() SagaState {
	return s.state
}

// GetID returns the saga ID
func (s *Saga) GetID() string {
	return s.id
}

// GetCurrentStep returns the current step index
func (s *Saga) GetCurrentStep() int {
	return s.currentStep
}

// generateSagaID generates a unique saga ID
func generateSagaID() string {
	return fmt.Sprintf("saga_%d", time.Now().UnixNano())
}

// SagaBuilder provides a fluent interface for building sagas
type SagaBuilder struct {
	saga *Saga
}

// NewSagaBuilder creates a new saga builder
func NewSagaBuilder(name string, logger *zap.Logger) *SagaBuilder {
	return &SagaBuilder{
		saga: NewSaga(name, logger),
	}
}

// WithStep adds a step to the saga
func (b *SagaBuilder) WithStep(name string, execute func(context.Context, interface{}) (interface{}, error)) *SagaBuilder {
	b.saga.AddStep(SagaStep{
		Name:    name,
		Execute: execute,
	})
	return b
}

// WithCompensableStep adds a step with compensation logic
func (b *SagaBuilder) WithCompensableStep(
	name string,
	execute func(context.Context, interface{}) (interface{}, error),
	compensate func(context.Context, interface{}) error,
) *SagaBuilder {
	b.saga.AddStep(SagaStep{
		Name:       name,
		Execute:    execute,
		Compensate: compensate,
	})
	return b
}

// WithRetryableStep adds a step with retry logic
func (b *SagaBuilder) WithRetryableStep(
	name string,
	execute func(context.Context, interface{}) (interface{}, error),
	maxRetries int,
	retryDelay time.Duration,
) *SagaBuilder {
	b.saga.AddStep(SagaStep{
		Name:       name,
		Execute:    execute,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
	})
	return b
}

// WithMetadata adds metadata to the saga
func (b *SagaBuilder) WithMetadata(key string, value interface{}) *SagaBuilder {
	b.saga.SetMetadata(key, value)
	return b
}

// Build returns the constructed saga
func (b *SagaBuilder) Build() *Saga {
	return b.saga
}