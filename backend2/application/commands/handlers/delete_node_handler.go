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

	// Get the user's default graph to find edges and update metadata
	var graphID string
	graph, err := h.graphRepo.GetUserDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		h.logger.Warn("Failed to get user's default graph for edge deletion",
			zap.String("userID", cmd.UserID),
			zap.Error(err),
		)
		// Continue with node deletion even if graph lookup fails
	} else {
		graphID = graph.ID().String()
		// Delete all edges connected to this node
		if err := h.edgeRepo.DeleteByNodeID(ctx, graphID, cmd.NodeID); err != nil {
			h.logger.Error("Failed to delete edges for node",
				zap.String("nodeID", cmd.NodeID),
				zap.String("graphID", graphID),
				zap.Error(err),
			)
			// Continue with node deletion even if edge deletion fails
			// This prevents orphaned nodes but may leave orphaned edges
		} else {
			h.logger.Info("Deleted edges connected to node",
				zap.String("nodeID", cmd.NodeID),
				zap.String("graphID", graphID),
			)
		}
	}

	// Delete the node
	if err := h.nodeRepo.Delete(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	// Update graph metadata to reflect the actual node/edge counts in the database
	if graphID != "" {
		if err := h.graphRepo.UpdateGraphMetadata(ctx, graphID); err != nil {
			h.logger.Error("Failed to update graph metadata after node deletion",
				zap.String("graphID", graphID),
				zap.String("nodeID", cmd.NodeID),
				zap.Error(err),
			)
			// Don't fail the operation, as node was already deleted
		} else {
			h.logger.Info("Updated graph metadata after node deletion",
				zap.String("graphID", graphID),
				zap.String("nodeID", cmd.NodeID),
			)
		}
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