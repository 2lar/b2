package handlers

import (
	"context"
	"fmt"

	"backend2/application/commands"
	"backend2/application/ports"
	"backend2/domain/core/valueobjects"
	"backend2/domain/events"
	"go.uber.org/zap"
)

// DeleteNodeHandler handles node deletion commands
type DeleteNodeHandler struct {
	nodeRepo   ports.NodeRepository
	eventStore ports.EventStore
	eventBus   ports.EventBus
	logger     *zap.Logger
}

// NewDeleteNodeHandler creates a new delete node handler
func NewDeleteNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *DeleteNodeHandler {
	return &DeleteNodeHandler{
		nodeRepo:   nodeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		logger:     logger,
	}
}

// Handle executes the delete node command
func (h *DeleteNodeHandler) Handle(ctx context.Context, cmd commands.DeleteNodeCommand) error {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Create NodeID value object
	nodeID, err := valueobjects.NewNodeIDFromString(cmd.NodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}

	// Verify node exists and belongs to user
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.UserID() != cmd.UserID {
		return fmt.Errorf("node does not belong to user")
	}

	// Delete the node
	if err := h.nodeRepo.Delete(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	// Create and publish deletion event
	content := node.Content()
	event := events.NewNodeDeletedEvent(
		nodeID,
		cmd.UserID,
		content.Title(),
		node.GetTags(),
		[]string{}, // connected node IDs - would need to extract from edges
		node.UpdatedAt(),
	)

	if err := h.eventBus.Publish(ctx, event); err != nil {
		h.logger.Warn("Failed to publish deletion event", zap.Error(err))
	}

	h.logger.Info("Node deleted",
		zap.String("nodeID", cmd.NodeID),
		zap.String("userID", cmd.UserID),
	)

	return nil
}