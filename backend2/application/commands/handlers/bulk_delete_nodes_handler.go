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
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
	logger    *zap.Logger
}

// NewBulkDeleteNodesHandler creates a new bulk delete handler
func NewBulkDeleteNodesHandler(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *BulkDeleteNodesHandler {
	return &BulkDeleteNodesHandler{
		nodeRepo:  nodeRepo,
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

	// Get user's default graph
	graph, err := h.graphRepo.GetUserDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		// If no default graph, nodes might be orphaned, still try to delete them
		h.logger.Warn("No default graph found for user",
			zap.String("userID", cmd.UserID),
			zap.Error(err),
		)
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
		if err := h.deleteNode(ctx, nodeID, cmd.UserID, graph); err != nil {
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

// deleteNode deletes a single node
func (h *BulkDeleteNodesHandler) deleteNode(ctx context.Context, nodeID valueobjects.NodeID, userID string, graph interface{}) error {
	// First, verify the node exists and belongs to the user
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Verify ownership
	if node.UserID() != userID {
		return fmt.Errorf("node does not belong to user")
	}

	// If we have a graph, remove the node from it
	if graph != nil {
		if g, ok := graph.(interface {
			RemoveNode(valueobjects.NodeID) error
		}); ok {
			if err := g.RemoveNode(nodeID); err != nil {
				// Log but don't fail - node might not be in graph
				h.logger.Warn("Failed to remove node from graph",
					zap.String("nodeID", nodeID.String()),
					zap.Error(err),
				)
			}
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