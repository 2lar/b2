package services

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/errors"
)

// SagaStep represents a single step in a saga
type SagaStep interface {
	// Execute performs the step's action
	Execute(ctx context.Context) error
	
	// Compensate reverses the step's action if needed
	Compensate(ctx context.Context) error
	
	// Name returns the step name for logging
	Name() string
}

// SagaState represents the current state of a saga execution
type SagaState string

const (
	SagaStatePending     SagaState = "PENDING"
	SagaStateRunning     SagaState = "RUNNING"
	SagaStateCompleted   SagaState = "COMPLETED"
	SagaStateFailed      SagaState = "FAILED"
	SagaStateCompensating SagaState = "COMPENSATING"
	SagaStateCompensated  SagaState = "COMPENSATED"
)

// Saga orchestrates a multi-step process across multiple aggregates
// It ensures consistency through compensation if any step fails
type Saga struct {
	id            string
	name          string
	steps         []SagaStep
	completedSteps []SagaStep
	state         SagaState
	startedAt     time.Time
	completedAt   *time.Time
	error         error
	eventBus      shared.EventBus
}

// NewSaga creates a new saga instance
func NewSaga(name string, eventBus shared.EventBus) *Saga {
	return &Saga{
		id:            shared.NewNodeID().String(), // Reuse ID generator
		name:          name,
		steps:         []SagaStep{},
		completedSteps: []SagaStep{},
		state:         SagaStatePending,
		eventBus:      eventBus,
	}
}

// AddStep adds a step to the saga
func (s *Saga) AddStep(step SagaStep) {
	s.steps = append(s.steps, step)
}

// Execute runs all saga steps in order
func (s *Saga) Execute(ctx context.Context) error {
	s.state = SagaStateRunning
	s.startedAt = time.Now()
	
	// Publish saga started event
	if s.eventBus != nil {
		s.eventBus.Publish(ctx, NewSagaStartedEvent(s.id, s.name))
	}
	
	// Execute each step in order
	for _, step := range s.steps {
		if err := s.executeStep(ctx, step); err != nil {
			s.state = SagaStateFailed
			s.error = err
			
			// Start compensation
			if compensateErr := s.compensate(ctx); compensateErr != nil {
				// Log compensation failure but return original error
				s.error = errors.Wrap(err, errors.CodeInternalError.String(), fmt.Sprintf("compensation also failed: %v", compensateErr))
			}
			
			// Publish saga failed event
			if s.eventBus != nil {
				s.eventBus.Publish(ctx, NewSagaFailedEvent(s.id, s.name, err.Error()))
			}
			
			return s.error
		}
		
		// Track completed step for potential compensation
		s.completedSteps = append(s.completedSteps, step)
	}
	
	// All steps completed successfully
	s.state = SagaStateCompleted
	now := time.Now()
	s.completedAt = &now
	
	// Publish saga completed event
	if s.eventBus != nil {
		s.eventBus.Publish(ctx, NewSagaCompletedEvent(s.id, s.name))
	}
	
	return nil
}

// executeStep executes a single saga step with proper error handling
func (s *Saga) executeStep(ctx context.Context, step SagaStep) error {
	// Add timeout to prevent hanging
	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	if err := step.Execute(stepCtx); err != nil {
		return errors.Wrap(err, errors.CodeInternalError.String(), fmt.Sprintf("saga step '%s' failed", step.Name()))
	}
	
	return nil
}

// compensate reverses completed steps in reverse order
func (s *Saga) compensate(ctx context.Context) error {
	s.state = SagaStateCompensating
	
	// Compensate in reverse order
	for i := len(s.completedSteps) - 1; i >= 0; i-- {
		step := s.completedSteps[i]
		
		// Add timeout to compensation
		compensateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := step.Compensate(compensateCtx)
		cancel()
		
		if err != nil {
			// Log but continue compensating other steps
			// In production, this should use proper logging
			continue
		}
	}
	
	s.state = SagaStateCompensated
	return nil
}

// Common Saga Steps

// CreateNodeStep creates a node as part of a saga
type CreateNodeStep struct {
	nodeRepo repository.NodeRepository
	nodeID   string
	userID   shared.UserID
	content  shared.Content
	tags     shared.Tags
}

func NewCreateNodeStep(nodeRepo repository.NodeRepository, userID shared.UserID, content shared.Content, tags shared.Tags) *CreateNodeStep {
	return &CreateNodeStep{
		nodeRepo: nodeRepo,
		userID:   userID,
		content:  content,
		tags:     tags,
	}
}

func (s *CreateNodeStep) Execute(ctx context.Context) error {
	// Implementation would call node service to create node
	// Store nodeID for potential compensation
	return nil
}

func (s *CreateNodeStep) Compensate(ctx context.Context) error {
	// Delete the created node if it exists
	if s.nodeID != "" {
		// Implementation would delete the node
	}
	return nil
}

func (s *CreateNodeStep) Name() string {
	return "CreateNode"
}

// CreateEdgeStep creates an edge between nodes
type CreateEdgeStep struct {
	edgeRepo repository.EdgeRepository
	edgeID   string
	fromID   shared.NodeID
	toID     shared.NodeID
}

func NewCreateEdgeStep(edgeRepo repository.EdgeRepository, fromID, toID shared.NodeID) *CreateEdgeStep {
	return &CreateEdgeStep{
		edgeRepo: edgeRepo,
		fromID:   fromID,
		toID:     toID,
	}
}

func (s *CreateEdgeStep) Execute(ctx context.Context) error {
	// Implementation would create edge
	return nil
}

func (s *CreateEdgeStep) Compensate(ctx context.Context) error {
	// Delete the created edge if it exists
	if s.edgeID != "" {
		// Implementation would delete the edge
	}
	return nil
}

func (s *CreateEdgeStep) Name() string {
	return "CreateEdge"
}

// TransactionalStep wraps a step with unit of work transaction management
type TransactionalStep struct {
	step       SagaStep
	uowFactory repository.UnitOfWorkFactory
}

func NewTransactionalStep(step SagaStep, uowFactory repository.UnitOfWorkFactory) *TransactionalStep {
	return &TransactionalStep{
		step:       step,
		uowFactory: uowFactory,
	}
}

func (s *TransactionalStep) Execute(ctx context.Context) error {
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return err
	}
	
	if err := uow.Begin(ctx); err != nil {
		return err
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	if err := s.step.Execute(ctx); err != nil {
		uow.Rollback()
		return err
	}
	
	return uow.Commit()
}

func (s *TransactionalStep) Compensate(ctx context.Context) error {
	// Compensation might also need transaction
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return err
	}
	
	if err := uow.Begin(ctx); err != nil {
		return err
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	if err := s.step.Compensate(ctx); err != nil {
		uow.Rollback()
		return err
	}
	
	return uow.Commit()
}

func (s *TransactionalStep) Name() string {
	return fmt.Sprintf("Transactional[%s]", s.step.Name())
}

// Saga Events

type SagaStartedEvent struct {
	shared.BaseEvent
	SagaID   string `json:"saga_id"`
	SagaName string `json:"saga_name"`
}

func NewSagaStartedEvent(sagaID, sagaName string) *SagaStartedEvent {
	return &SagaStartedEvent{
		BaseEvent: shared.NewBaseEvent("SagaStarted", sagaID, "", 0),
		SagaID:    sagaID,
		SagaName:  sagaName,
	}
}

func (e *SagaStartedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"saga_id":   e.SagaID,
		"saga_name": e.SagaName,
	}
}

type SagaCompletedEvent struct {
	shared.BaseEvent
	SagaID   string `json:"saga_id"`
	SagaName string `json:"saga_name"`
}

func NewSagaCompletedEvent(sagaID, sagaName string) *SagaCompletedEvent {
	return &SagaCompletedEvent{
		BaseEvent: shared.NewBaseEvent("SagaCompleted", sagaID, "", 0),
		SagaID:    sagaID,
		SagaName:  sagaName,
	}
}

func (e *SagaCompletedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"saga_id":   e.SagaID,
		"saga_name": e.SagaName,
	}
}

type SagaFailedEvent struct {
	shared.BaseEvent
	SagaID   string `json:"saga_id"`
	SagaName string `json:"saga_name"`
	Error    string `json:"error"`
}

func NewSagaFailedEvent(sagaID, sagaName, errorMsg string) *SagaFailedEvent {
	return &SagaFailedEvent{
		BaseEvent: shared.NewBaseEvent("SagaFailed", sagaID, "", 0),
		SagaID:    sagaID,
		SagaName:  sagaName,
		Error:     errorMsg,
	}
}

func (e *SagaFailedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"saga_id":   e.SagaID,
		"saga_name": e.SagaName,
		"error":     e.Error,
	}
}