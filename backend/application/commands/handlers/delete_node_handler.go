package handlers

import (
	"context"
	"fmt"

	"backend/application/commands"
	"backend/application/ports"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	"go.uber.org/zap"
)

// DeleteNodeHandler handles node deletion commands
type DeleteNodeHandler struct {
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	graphRepo  ports.GraphRepository
	eventStore ports.EventStore
	eventBus   ports.EventBus
	logger     *zap.Logger
}

// NewDeleteNodeHandler creates a new delete node handler
func NewDeleteNodeHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *DeleteNodeHandler {
	return &DeleteNodeHandler{
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		graphRepo:  graphRepo,
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

	// Get the user's default graph ID for the async cleanup event
	var graphID string
	graph, err := h.graphRepo.GetUserDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		h.logger.Warn("Failed to get user's default graph",
			zap.String("userID", cmd.UserID),
			zap.Error(err),
		)
		// Continue with node deletion even if graph lookup fails
	} else {
		graphID = graph.ID().String()
	}

	// Delete the node
	if err := h.nodeRepo.Delete(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	// Graph metadata will be updated asynchronously after edge cleanup

	// Create and publish deletion event with GraphID for async cleanup
	content := node.Content()
	event := events.NewNodeDeletedEvent(
		nodeID,
		cmd.UserID,
		graphID,
		content.Title(),
		node.GetTags(),
		[]string{}, // Keywords - can be extracted from content if needed
		node.UpdatedAt(),
	)

	if err := h.eventBus.PublishBatch(ctx, []events.DomainEvent{event}); err != nil {
		h.logger.Warn("Failed to publish deletion event", zap.Error(err))
	}

	// Event cleanup will happen asynchronously via the cleanup handler
	// This ensures immediate response to the user while heavy cleanup happens in background

	h.logger.Info("Node deleted successfully, async cleanup initiated",
		zap.String("nodeID", cmd.NodeID),
		zap.String("userID", cmd.UserID),
	)

	return nil
}
