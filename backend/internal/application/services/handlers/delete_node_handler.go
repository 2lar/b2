package handlers

import (
	"context"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/errors"
	"brain2-backend/internal/repository"

	"go.opentelemetry.io/otel/trace"
)

// DeleteNodeHandler handles node deletion operations
type DeleteNodeHandler struct {
	uowFactory repository.UnitOfWorkFactory
	eventBus   shared.EventBus
	tracer     trace.Tracer
}

// NewDeleteNodeHandler creates a new handler for node deletion
func NewDeleteNodeHandler(
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	tracer trace.Tracer,
) *DeleteNodeHandler {
	return &DeleteNodeHandler{
		uowFactory: uowFactory,
		eventBus:   eventBus,
		tracer:     tracer,
	}
}

// Handle processes the DeleteNodeCommand
func (h *DeleteNodeHandler) Handle(ctx context.Context, cmd *commands.DeleteNodeCommand) (*dto.DeleteNodeResult, error) {
	// 1. Create a new UnitOfWork instance for this request
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. Parse domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	nodeID, err := shared.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, errors.ServiceValidationError("field", "", "invalid node id: " + err.Error())
	}

	// 3. Verify node exists and user owns it
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindNode", err)
	}
	if node == nil {
		return nil, errors.ServiceNotFoundError("node", "node not found")
	}

	if !node.UserID().Equals(userID) {
		return nil, errors.ServiceAuthorizationError(cmd.UserID, "node", "node belongs to different user")
	}

	// 4. Delete associated edges first using proper DeleteByNode method
	if edgeDeleter, ok := uow.Edges().(interface {
		DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	}); ok {
		if err := edgeDeleter.DeleteByNode(ctx, userID, nodeID); err != nil {
			return nil, errors.ApplicationError(ctx, "DeleteNodeEdges", err)
		}
	}

	// 5. Node is ready for deletion - no special marking needed

	// 6. Delete the node
	if err := uow.Nodes().DeleteNode(ctx, userID.String(), nodeID.String()); err != nil {
		return nil, errors.ApplicationError(ctx, "DeleteNode", err)
	}

	// 7. Publish domain events
	for _, event := range node.GetUncommittedEvents() {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			return nil, errors.ApplicationError(ctx, "PublishEvent", err)
		}
	}
	node.MarkEventsAsCommitted()

	// 8. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false // Reset if commit fails
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
	}

	// 9. Return success result
	result := &dto.DeleteNodeResult{
		Message: "Node deleted successfully",
		Success: true,
	}

	return result, nil
}