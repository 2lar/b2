package events

import (
	"time"
)

// BulkNodesDeletedEvent is emitted when bulk node deletion completes
type BulkNodesDeletedEvent struct {
	BaseEvent
	OperationID  string   `json:"operation_id"`
	UserID       string   `json:"user_id"`
	DeletedCount int      `json:"deleted_count"`
	RequestedIDs []string `json:"requested_ids"`
	DeletedIDs   []string `json:"deleted_ids"`
	FailedIDs    []string `json:"failed_ids,omitempty"`
	Errors       []string `json:"errors,omitempty"`
	CompletedAt  time.Time `json:"completed_at"`
}

// NewBulkNodesDeletedEvent creates a new bulk nodes deleted event
func NewBulkNodesDeletedEvent(
	operationID string,
	userID string,
	deletedCount int,
	requestedIDs []string,
	deletedIDs []string,
	failedIDs []string,
	errors []string,
) *BulkNodesDeletedEvent {
	return &BulkNodesDeletedEvent{
		BaseEvent: BaseEvent{
			AggregateID: operationID, // Use operation ID as aggregate ID
			EventType:   "BulkNodesDeleted",
			Timestamp:   time.Now(),
			Version:     1,
		},
		OperationID:  operationID,
		UserID:       userID,
		DeletedCount: deletedCount,
		RequestedIDs: requestedIDs,
		DeletedIDs:   deletedIDs,
		FailedIDs:    failedIDs,
		Errors:       errors,
		CompletedAt:  time.Now(),
	}
}

// GetEventType returns the event type
func (e *BulkNodesDeletedEvent) GetEventType() string {
	return "BulkNodesDeleted"
}

// GetAggregateID returns the aggregate ID (operation ID in this case)
func (e *BulkNodesDeletedEvent) GetAggregateID() string {
	return e.OperationID
}