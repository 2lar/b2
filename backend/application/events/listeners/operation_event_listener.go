package listeners

import (
	"context"
	"time"

	appevents "backend/application/events"
	"backend/application/ports"
	"backend/application/queries"
	"backend/domain/events"
	"go.uber.org/zap"
)

// OperationEventListener listens for operation-related events and updates the operation store
type OperationEventListener struct {
	operationStore ports.OperationStore
	logger         *zap.Logger
}

// NewOperationEventListener creates a new operation event listener
func NewOperationEventListener(operationStore ports.OperationStore, logger *zap.Logger) *OperationEventListener {
	return &OperationEventListener{
		operationStore: operationStore,
		logger:         logger,
	}
}

// HandleBulkNodesDeleted handles the BulkNodesDeletedEvent
func (l *OperationEventListener) HandleBulkNodesDeleted(ctx context.Context, event *events.BulkNodesDeletedEvent) error {
	// Create operation result from event
	completedAt := time.Now()
	status := ports.OperationStatusCompleted
	errorMsg := ""
	
	// If there were errors, mark as failed
	if len(event.Errors) > 0 {
		status = ports.OperationStatusFailed
		errorMsg = event.Errors[0] // Take first error for summary
	}
	
	// Store the result
	operationResult := &ports.OperationResult{
		OperationID: event.OperationID,
		Status:      status,
		StartedAt:   event.Timestamp, // Use event timestamp as start time
		CompletedAt: &completedAt,
		Result: queries.BulkDeleteResult{
			DeletedCount: event.DeletedCount,
			RequestedIDs: event.RequestedIDs,
			DeletedIDs:   event.DeletedIDs,
			FailedIDs:    event.FailedIDs,
			Errors:       event.Errors,
		},
		Error: errorMsg,
		Metadata: map[string]interface{}{
			"user_id": event.UserID,
		},
	}
	
	// Update or store the operation result
	err := l.operationStore.Update(ctx, event.OperationID, operationResult)
	if err != nil {
		// If update fails, try to store (operation might not exist yet)
		err = l.operationStore.Store(ctx, operationResult)
		if err != nil {
			l.logger.Error("Failed to store operation result",
				zap.String("operationID", event.OperationID),
				zap.Error(err),
			)
			return err
		}
	}
	
	l.logger.Info("Operation result stored",
		zap.String("operationID", event.OperationID),
		zap.String("status", string(status)),
		zap.Int("deletedCount", event.DeletedCount),
	)
	
	return nil
}

// Subscribe subscribes the listener to relevant events
func (l *OperationEventListener) Subscribe(registry *appevents.HandlerRegistry) error {
	// Create an adapter that implements the EventHandler interface
	adapter := NewOperationEventHandlerAdapter(l)
	
	// Register the adapter with the handler registry
	eventTypes := []string{"BulkNodesDeletedEvent"}
	if err := registry.Register(eventTypes, adapter); err != nil {
		l.logger.Error("Failed to register operation event handler", 
			zap.Error(err),
			zap.Strings("eventTypes", eventTypes))
		return err
	}
	
	l.logger.Info("Operation event listener subscribed successfully",
		zap.Strings("eventTypes", eventTypes))
	
	return nil
}