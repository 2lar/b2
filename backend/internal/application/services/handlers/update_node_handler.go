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

// UpdateNodeHandler handles node update operations
type UpdateNodeHandler struct {
	uowFactory repository.UnitOfWorkFactory
	eventBus   shared.EventBus
	tracer     trace.Tracer
}

// NewUpdateNodeHandler creates a new handler for node updates
func NewUpdateNodeHandler(
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	tracer trace.Tracer,
) *UpdateNodeHandler {
	return &UpdateNodeHandler{
		uowFactory: uowFactory,
		eventBus:   eventBus,
		tracer:     tracer,
	}
}

// Handle processes the UpdateNodeCommand
func (h *UpdateNodeHandler) Handle(ctx context.Context, cmd *commands.UpdateNodeCommand) (*dto.UpdateNodeResult, error) {
	if !cmd.HasChanges() {
		return nil, errors.ServiceValidationError("field", "", "no changes specified in update command")
	}

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

	// 3. Retrieve existing node
	node, err := uow.Nodes().FindNodeByID(ctx, userID.String(), nodeID.String())
	if err != nil {
		return nil, errors.ApplicationError(ctx, "FindNode", err)
	}
	if node == nil {
		return nil, errors.ServiceNotFoundError("node", "node not found")
	}

	// 4. Verify ownership
	if !node.UserID().Equals(userID) {
		return nil, errors.ServiceAuthorizationError(cmd.UserID, "node", "node belongs to different user")
	}

	// 5. Apply updates using domain methods
	if cmd.Content != "" {
		newContent, err := shared.NewContent(cmd.Content)
		if err != nil {
			return nil, errors.ServiceValidationError("field", "", "invalid content: " + err.Error())
		}

		if err := node.UpdateContent(newContent); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateContent", err)
		}
	}

	if len(cmd.Tags) > 0 {
		newTags := shared.NewTags(cmd.Tags...)
		if err := node.UpdateTags(newTags); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateTags", err)
		}
	}

	if cmd.Title != "" {
		newTitle, err := shared.NewTitle(cmd.Title)
		if err != nil {
			return nil, errors.ServiceValidationError("field", "", "invalid title: " + err.Error())
		}
		if err := node.UpdateTitle(newTitle); err != nil {
			return nil, errors.ApplicationError(ctx, "UpdateTitle", err)
		}
	}

	// 6. Save updated node
	if err := uow.Nodes().CreateNodeAndKeywords(ctx, node); err != nil {
		return nil, errors.ApplicationError(ctx, "SaveUpdatedNode", err)
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

	// 9. Convert to response DTO
	result := &dto.UpdateNodeResult{
		Node:    dto.ToNodeView(node),
		Message: "Node updated successfully",
	}

	return result, nil
}