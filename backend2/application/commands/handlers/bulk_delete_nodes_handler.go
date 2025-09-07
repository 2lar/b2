package handlers

import (
	"context"
	"fmt"

	"backend2/application/commands"
	"backend2/application/ports"
	"backend2/domain/core/valueobjects"
	"go.uber.org/zap"
)

// BulkDeleteNodesHandler handles bulk delete commands
type BulkDeleteNodesHandler struct {
	nodeRepo  ports.NodeRepository
	edgeRepo  ports.EdgeRepository
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
	logger    *zap.Logger
}

// NewBulkDeleteNodesHandler creates a new bulk delete handler
func NewBulkDeleteNodesHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *BulkDeleteNodesHandler {
	return &BulkDeleteNodesHandler{
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		graphRepo: graphRepo,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Handle executes the bulk delete command
func (h *BulkDeleteNodesHandler) Handle(ctx context.Context, cmd commands.BulkDeleteNodesCommand) (*commands.BulkDeleteNodesResult, error) {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	result := &commands.BulkDeleteNodesResult{
		DeletedCount: 0,
		FailedIDs:    []string{},
		Errors:       []string{},
	}

	// Get user's default graph for graph ID
	var graphID string
	graph, err := h.graphRepo.GetUserDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		// If no default graph, nodes might be orphaned, still try to delete them
		h.logger.Warn("No default graph found for user",
			zap.String("userID", cmd.UserID),
			zap.Error(err),
		)
	} else if graph != nil {
		// Graph is a concrete type, directly get the ID
		graphID = graph.ID().String()
	}

	// Process each node deletion
	for _, nodeIDStr := range cmd.NodeIDs {
		nodeID, err := valueobjects.NewNodeIDFromString(nodeIDStr)
		if err != nil {
			result.FailedIDs = append(result.FailedIDs, nodeIDStr)
			result.Errors = append(result.Errors, fmt.Sprintf("Invalid node ID %s: %v", nodeIDStr, err))
			continue
		}
		
		// Try to delete the node
		if err := h.deleteNode(ctx, nodeID, cmd.UserID, graphID); err != nil {
			result.FailedIDs = append(result.FailedIDs, nodeIDStr)
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to delete node %s: %v", nodeIDStr, err))
			h.logger.Error("Failed to delete node",
				zap.String("nodeID", nodeIDStr),
				zap.String("userID", cmd.UserID),
				zap.Error(err),
			)
			continue
		}
		
		result.DeletedCount++
	}

	// Update graph metadata to reflect the actual node/edge counts in the database
	if graphID != "" && result.DeletedCount > 0 {
		if err := h.graphRepo.UpdateGraphMetadata(ctx, graphID); err != nil {
			h.logger.Error("Failed to update graph metadata after bulk delete",
				zap.String("graphID", graphID),
				zap.String("userID", cmd.UserID),
				zap.Error(err),
			)
			// Don't fail the operation, as nodes were already deleted
		} else {
			h.logger.Info("Updated graph metadata after bulk delete",
				zap.String("graphID", graphID),
				zap.Int("deletedNodes", result.DeletedCount),
			)
		}
	}

	// Log operation summary
	h.logger.Info("Bulk delete operation completed",
		zap.String("userID", cmd.UserID),
		zap.Int("requested", len(cmd.NodeIDs)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.FailedIDs)),
	)

	// Return partial success even if some deletions failed
	return result, nil
}

// deleteNode deletes a single node and its edges
func (h *BulkDeleteNodesHandler) deleteNode(ctx context.Context, nodeID valueobjects.NodeID, userID string, graphID string) error {
	// First, verify the node exists and belongs to the user
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Verify ownership
	if node.UserID() != userID {
		return fmt.Errorf("node does not belong to user")
	}

	// Delete edges connected to this node if we have a graph ID
	if graphID != "" {
		if err := h.edgeRepo.DeleteByNodeID(ctx, graphID, nodeID.String()); err != nil {
			h.logger.Warn("Failed to delete edges for node",
				zap.String("nodeID", nodeID.String()),
				zap.String("graphID", graphID),
				zap.Error(err),
			)
			// Continue even if edge deletion fails
		}
	}

	// Delete the node from repository
	if err := h.nodeRepo.Delete(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	// Publish node deleted event
	if h.eventBus != nil {
		// We could publish a NodeDeleted event here if needed
		// For now, we'll skip event publishing
	}

	return nil
}