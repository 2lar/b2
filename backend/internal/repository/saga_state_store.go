package repository

import (
	"context"
	"time"
)

// SagaStateStore provides persistence for saga orchestration state
type SagaStateStore interface {
	// SaveSagaState persists the current state of a saga
	SaveSagaState(ctx context.Context, state *SagaStateData) error
	
	// GetSagaState retrieves the state of a saga by ID
	GetSagaState(ctx context.Context, sagaID string) (*SagaStateData, error)
	
	// ListPendingSagas returns all sagas that haven't completed
	ListPendingSagas(ctx context.Context) ([]*SagaStateData, error)
	
	// DeleteSagaState removes a saga state (typically after successful completion)
	DeleteSagaState(ctx context.Context, sagaID string) error
	
	// UpdateSagaProgress updates the progress of a saga
	UpdateSagaProgress(ctx context.Context, sagaID string, completedSteps []int) error
}

// SagaStateData represents the persisted state of a saga
type SagaStateData struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	State          string     `json:"state"`
	CompletedSteps []int      `json:"completed_steps"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	TTL            int64      `json:"ttl,omitempty"` // For automatic cleanup
}