package queries

import (
	"errors"
	"time"
)

// GetOperationStatusQuery retrieves the status of an async operation
type GetOperationStatusQuery struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
}

// Validate validates the query
func (q GetOperationStatusQuery) Validate() error {
	if q.OperationID == "" {
		return errors.New("operation ID is required")
	}
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	return nil
}

// OperationStatusResult represents the result of the operation status query
type OperationStatusResult struct {
	OperationID string                 `json:"operation_id"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BulkDeleteResult represents the result of a bulk delete operation
type BulkDeleteResult struct {
	DeletedCount int      `json:"deleted_count"`
	RequestedIDs []string `json:"requested_ids"`
	DeletedIDs   []string `json:"deleted_ids"`
	FailedIDs    []string `json:"failed_ids,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}