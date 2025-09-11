package ports

import (
	"context"
	"time"
)

// OperationStatus represents the status of an async operation
type OperationStatus string

const (
	OperationStatusPending   OperationStatus = "pending"
	OperationStatusCompleted OperationStatus = "completed"
	OperationStatusFailed    OperationStatus = "failed"
)

// OperationResult stores the result of an async operation
type OperationResult struct {
	OperationID string          `json:"operation_id"`
	Status      OperationStatus `json:"status"`
	StartedAt   time.Time       `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Result      interface{}     `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// OperationStore manages async operation results
type OperationStore interface {
	// Store saves an operation result
	Store(ctx context.Context, result *OperationResult) error
	
	// Get retrieves an operation result by ID
	Get(ctx context.Context, operationID string) (*OperationResult, error)
	
	// Update updates an existing operation result
	Update(ctx context.Context, operationID string, result *OperationResult) error
	
	// Delete removes an operation result
	Delete(ctx context.Context, operationID string) error
	
	// CleanupExpired removes operations older than the given duration
	CleanupExpired(ctx context.Context, olderThan time.Duration) error
}