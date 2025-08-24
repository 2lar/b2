package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
	
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/pkg/errors"
)

// SagaOrchestrator manages distributed transactions with compensation
type SagaOrchestrator struct {
	id               string
	name             string
	steps            []SagaStep
	compensators     []CompensationStep
	completedSteps   []int // Track which steps completed
	state            SagaState
	startedAt        time.Time
	completedAt      *time.Time
	lastError        error
	eventBus         shared.EventBus
	stateStore       repository.SagaStateStore // For persistence
	retryPolicy      *RetryPolicy
	timeout          time.Duration
}

// CompensationStep represents a compensation action for a saga step
type CompensationStep interface {
	// Compensate reverses a completed step
	Compensate(ctx context.Context) error
	
	// CanCompensate checks if compensation is possible
	CanCompensate() bool
	
	// Priority returns the compensation priority (higher = more urgent)
	Priority() int
}

// RetryPolicy defines retry behavior for saga steps
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64 // 0-1, adds randomness to prevent thundering herd
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
	}
}

// NewSagaOrchestrator creates a new saga orchestrator with enhanced features
func NewSagaOrchestrator(name string, eventBus shared.EventBus, stateStore repository.SagaStateStore) *SagaOrchestrator {
	return &SagaOrchestrator{
		id:             shared.NewNodeID().String(),
		name:           name,
		steps:          []SagaStep{},
		compensators:   []CompensationStep{},
		completedSteps: []int{},
		state:          SagaStatePending,
		eventBus:       eventBus,
		stateStore:     stateStore,
		retryPolicy:    DefaultRetryPolicy(),
		timeout:        5 * time.Minute, // Default timeout
	}
}

// AddStepWithCompensation adds a step with its compensation
func (s *SagaOrchestrator) AddStepWithCompensation(step SagaStep, compensator CompensationStep) {
	s.steps = append(s.steps, step)
	s.compensators = append(s.compensators, compensator)
}

// SetRetryPolicy configures the retry behavior
func (s *SagaOrchestrator) SetRetryPolicy(policy *RetryPolicy) {
	s.retryPolicy = policy
}

// SetTimeout sets the overall saga timeout
func (s *SagaOrchestrator) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

// Execute runs the saga with retries and compensation
func (s *SagaOrchestrator) Execute(ctx context.Context) error {
	// Create context with timeout
	sagaCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	
	s.state = SagaStateRunning
	s.startedAt = time.Now()
	
	// Persist initial state
	if err := s.persistState(sagaCtx); err != nil {
		return errors.Wrap(err, "failed to persist initial saga state")
	}
	
	// Publish saga started event
	if s.eventBus != nil {
		s.eventBus.Publish(sagaCtx, NewSagaStartedEvent(s.id, s.name))
	}
	
	// Execute each step with retry logic
	for i, step := range s.steps {
		if err := s.executeStepWithRetry(sagaCtx, step, i); err != nil {
			s.state = SagaStateFailed
			s.lastError = err
			
			// Persist failed state
			s.persistState(sagaCtx)
			
			// Start compensation process
			if compensateErr := s.compensateWithStrategy(sagaCtx); compensateErr != nil {
				s.lastError = errors.Wrap(err, fmt.Sprintf("compensation also failed: %v", compensateErr))
			}
			
			// Publish saga failed event
			if s.eventBus != nil {
				s.eventBus.Publish(sagaCtx, NewSagaFailedEvent(s.id, s.name, err.Error()))
			}
			
			return s.lastError
		}
		
		// Mark step as completed
		s.completedSteps = append(s.completedSteps, i)
		
		// Persist progress
		if err := s.persistState(sagaCtx); err != nil {
			// Log but continue - state persistence failure shouldn't stop saga
			// In production, this should use proper logging
		}
	}
	
	// All steps completed successfully
	s.state = SagaStateCompleted
	now := time.Now()
	s.completedAt = &now
	
	// Persist final state
	s.persistState(sagaCtx)
	
	// Publish saga completed event
	if s.eventBus != nil {
		s.eventBus.Publish(sagaCtx, NewSagaCompletedEvent(s.id, s.name))
	}
	
	return nil
}

// executeStepWithRetry executes a step with exponential backoff retry
func (s *SagaOrchestrator) executeStepWithRetry(ctx context.Context, step SagaStep, stepIndex int) error {
	var lastErr error
	delay := s.retryPolicy.InitialDelay
	
	for attempt := 0; attempt < s.retryPolicy.MaxAttempts; attempt++ {
		// Add timeout for individual step
		stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := step.Execute(stepCtx)
		cancel()
		
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}
		
		// Check if we've exhausted retries
		if attempt == s.retryPolicy.MaxAttempts-1 {
			break
		}
		
		// Calculate next delay with exponential backoff and jitter
		delay = s.calculateBackoffDelay(delay, attempt)
		
		// Wait before retry
		select {
		case <-time.After(delay):
			// Continue to retry
		case <-ctx.Done():
			return ctx.Err()
		}
		
		// Publish retry event
		if s.eventBus != nil {
			s.eventBus.Publish(ctx, NewSagaStepRetryEvent(s.id, s.name, step.Name(), attempt+1))
		}
	}
	
	return errors.Wrap(lastErr, fmt.Sprintf("step '%s' failed after %d attempts", step.Name(), s.retryPolicy.MaxAttempts))
}

// calculateBackoffDelay calculates the next retry delay with jitter
func (s *SagaOrchestrator) calculateBackoffDelay(currentDelay time.Duration, attempt int) time.Duration {
	// Exponential backoff
	nextDelay := time.Duration(float64(currentDelay) * s.retryPolicy.BackoffFactor)
	
	// Cap at max delay
	if nextDelay > s.retryPolicy.MaxDelay {
		nextDelay = s.retryPolicy.MaxDelay
	}
	
	// Add jitter to prevent thundering herd
	if s.retryPolicy.JitterFactor > 0 {
		jitter := time.Duration(rand.Float64() * s.retryPolicy.JitterFactor * float64(nextDelay))
		nextDelay = nextDelay + jitter
	}
	
	return nextDelay
}

// compensateWithStrategy performs compensation with priority-based ordering
func (s *SagaOrchestrator) compensateWithStrategy(ctx context.Context) error {
	s.state = SagaStateCompensating
	s.persistState(ctx)
	
	// Build compensation plan based on priority
	compensationPlan := s.buildCompensationPlan()
	
	var compensationErrors []error
	
	for _, compIndex := range compensationPlan {
		compensator := s.compensators[compIndex]
		
		// Check if compensation is possible
		if !compensator.CanCompensate() {
			continue
		}
		
		// Add timeout to compensation
		compensateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := s.executeCompensationWithRetry(compensateCtx, compensator)
		cancel()
		
		if err != nil {
			compensationErrors = append(compensationErrors, err)
			// Continue compensating other steps even if one fails
		}
	}
	
	s.state = SagaStateCompensated
	s.persistState(ctx)
	
	if len(compensationErrors) > 0 {
		return errors.NewInternal("compensation completed with errors", fmt.Sprintf("%v", compensationErrors))
	}
	
	return nil
}

// executeCompensationWithRetry executes compensation with retry logic
func (s *SagaOrchestrator) executeCompensationWithRetry(ctx context.Context, compensator CompensationStep) error {
	// Use more aggressive retry for compensation
	maxAttempts := s.retryPolicy.MaxAttempts * 2
	delay := s.retryPolicy.InitialDelay / 2
	
	var lastErr error
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := compensator.Compensate(ctx)
		
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if attempt < maxAttempts-1 {
			select {
			case <-time.After(delay):
				delay = s.calculateBackoffDelay(delay, attempt)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	
	return lastErr
}

// buildCompensationPlan creates an ordered compensation plan
func (s *SagaOrchestrator) buildCompensationPlan() []int {
	// Create a slice of indices sorted by priority
	plan := make([]int, 0, len(s.completedSteps))
	
	// First, add high-priority compensations
	for i := len(s.completedSteps) - 1; i >= 0; i-- {
		stepIndex := s.completedSteps[i]
		if s.compensators[stepIndex].Priority() >= 100 {
			plan = append(plan, stepIndex)
		}
	}
	
	// Then add normal priority in reverse order
	for i := len(s.completedSteps) - 1; i >= 0; i-- {
		stepIndex := s.completedSteps[i]
		priority := s.compensators[stepIndex].Priority()
		if priority > 0 && priority < 100 {
			plan = append(plan, stepIndex)
		}
	}
	
	// Finally add low priority
	for i := len(s.completedSteps) - 1; i >= 0; i-- {
		stepIndex := s.completedSteps[i]
		if s.compensators[stepIndex].Priority() <= 0 {
			plan = append(plan, stepIndex)
		}
	}
	
	return plan
}

// persistState saves the saga state for recovery
func (s *SagaOrchestrator) persistState(ctx context.Context) error {
	if s.stateStore == nil {
		return nil // State persistence is optional
	}
	
	state := &SagaStateData{
		ID:             s.id,
		Name:           s.name,
		State:          string(s.state),
		CompletedSteps: s.completedSteps,
		StartedAt:      s.startedAt,
		CompletedAt:    s.completedAt,
		LastError:      "",
	}
	
	if s.lastError != nil {
		state.LastError = s.lastError.Error()
	}
	
	return s.stateStore.SaveSagaState(ctx, state)
}

// RecoverFromState recovers a saga from persisted state
func (s *SagaOrchestrator) RecoverFromState(ctx context.Context, sagaID string) error {
	if s.stateStore == nil {
		return errors.NewInternal("state store not configured", "cannot recover saga without state store")
	}
	
	state, err := s.stateStore.GetSagaState(ctx, sagaID)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve saga state")
	}
	
	s.id = state.ID
	s.name = state.Name
	s.state = SagaState(state.State)
	s.completedSteps = state.CompletedSteps
	s.startedAt = state.StartedAt
	
	if state.CompletedAt != nil {
		s.completedAt = state.CompletedAt
	}
	
	if state.LastError != "" {
		s.lastError = errors.NewInternal("recovered error", state.LastError)
	}
	
	return nil
}

// SagaStateData represents persisted saga state
type SagaStateData struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	State          string     `json:"state"`
	CompletedSteps []int      `json:"completed_steps"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// Concrete compensation implementations

// BaseCompensation provides common compensation functionality
type BaseCompensation struct {
	priority    int
	canCompensate bool
	compensateFunc func(context.Context) error
}

func (c *BaseCompensation) Priority() int {
	return c.priority
}

func (c *BaseCompensation) CanCompensate() bool {
	return c.canCompensate
}

func (c *BaseCompensation) Compensate(ctx context.Context) error {
	if c.compensateFunc != nil {
		return c.compensateFunc(ctx)
	}
	return nil
}

// NodeDeletionCompensation compensates for node creation
type NodeDeletionCompensation struct {
	BaseCompensation
	nodeRepo repository.NodeWriter
	userID   shared.UserID
	nodeID   shared.NodeID
}

func NewNodeDeletionCompensation(nodeRepo repository.NodeWriter, userID shared.UserID, nodeID shared.NodeID) *NodeDeletionCompensation {
	return &NodeDeletionCompensation{
		BaseCompensation: BaseCompensation{
			priority:      50, // Medium priority
			canCompensate: true,
		},
		nodeRepo: nodeRepo,
		userID:   userID,
		nodeID:   nodeID,
	}
}

func (c *NodeDeletionCompensation) Compensate(ctx context.Context) error {
	return c.nodeRepo.Delete(ctx, c.userID, c.nodeID)
}

// EdgeDeletionCompensation compensates for edge creation
type EdgeDeletionCompensation struct {
	BaseCompensation
	edgeRepo repository.EdgeWriter
	userID   shared.UserID
	edgeID   shared.NodeID
}

func NewEdgeDeletionCompensation(edgeRepo repository.EdgeWriter, userID shared.UserID, edgeID shared.NodeID) *EdgeDeletionCompensation {
	return &EdgeDeletionCompensation{
		BaseCompensation: BaseCompensation{
			priority:      30, // Lower priority than nodes
			canCompensate: true,
		},
		edgeRepo: edgeRepo,
		userID:   userID,
		edgeID:   edgeID,
	}
}

func (c *EdgeDeletionCompensation) Compensate(ctx context.Context) error {
	return c.edgeRepo.Delete(ctx, c.userID, c.edgeID)
}

// EventReverseCompensation publishes a reverse event
type EventReverseCompensation struct {
	BaseCompensation
	eventBus shared.EventBus
	event    shared.DomainEvent
}

func NewEventReverseCompensation(eventBus shared.EventBus, originalEvent shared.DomainEvent) *EventReverseCompensation {
	return &EventReverseCompensation{
		BaseCompensation: BaseCompensation{
			priority:      100, // High priority
			canCompensate: true,
		},
		eventBus: eventBus,
		event:    originalEvent,
	}
}

func (c *EventReverseCompensation) Compensate(ctx context.Context) error {
	// Create a reverse event
	reverseEvent := NewCompensationEvent(c.event.AggregateID(), c.event.EventType())
	return c.eventBus.Publish(ctx, reverseEvent)
}

// Helper functions

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	// Check for specific error types that are retryable
	if errors.IsTimeout(err) || errors.IsConnectionError(err) {
		return true
	}
	
	// Check for rate limiting errors
	if errors.IsRateLimit(err) {
		return true
	}
	
	// Don't retry on validation or business logic errors
	if errors.IsValidation(err) || errors.IsNotFound(err) {
		return false
	}
	
	// Default to retryable for unknown errors
	return true
}

// Saga Events

type SagaStepRetryEvent struct {
	shared.BaseEvent
	SagaID   string `json:"saga_id"`
	SagaName string `json:"saga_name"`
	StepName string `json:"step_name"`
	Attempt  int    `json:"attempt"`
}

func NewSagaStepRetryEvent(sagaID, sagaName, stepName string, attempt int) *SagaStepRetryEvent {
	return &SagaStepRetryEvent{
		BaseEvent: shared.NewBaseEvent("SagaStepRetry", sagaID, "", 0),
		SagaID:    sagaID,
		SagaName:  sagaName,
		StepName:  stepName,
		Attempt:   attempt,
	}
}

func (e *SagaStepRetryEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"saga_id":   e.SagaID,
		"saga_name": e.SagaName,
		"step_name": e.StepName,
		"attempt":   e.Attempt,
	}
}

type CompensationEvent struct {
	shared.BaseEvent
	OriginalEventType string `json:"original_event_type"`
}

func NewCompensationEvent(aggregateID, originalEventType string) *CompensationEvent {
	return &CompensationEvent{
		BaseEvent:         shared.NewBaseEvent("Compensation", aggregateID, "", 0),
		OriginalEventType: originalEventType,
	}
}

func (e *CompensationEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"original_event_type": e.OriginalEventType,
		"aggregate_id":        e.AggregateID(),
	}
}