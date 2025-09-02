// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// BulkDeleteNodesCommand represents a command to delete multiple nodes
type BulkDeleteNodesCommand struct {
	cqrs.BaseCommand
	NodeIDs []string `json:"node_ids"`
}

// GetCommandName returns the command name
func (c BulkDeleteNodesCommand) GetCommandName() string {
	return "BulkDeleteNodes"
}

// Validate validates the command
func (c BulkDeleteNodesCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if len(c.NodeIDs) == 0 {
		return fmt.Errorf("at least one node ID is required")
	}
	if len(c.NodeIDs) > 100 {
		return fmt.Errorf("cannot delete more than 100 nodes at once")
	}
	
	// Check for duplicates
	seen := make(map[string]bool)
	for _, id := range c.NodeIDs {
		if id == "" {
			return fmt.Errorf("empty node ID in list")
		}
		if seen[id] {
			return fmt.Errorf("duplicate node ID: %s", id)
		}
		seen[id] = true
	}
	
	return nil
}

// BulkDeleteNodesHandler handles bulk node deletion commands
type BulkDeleteNodesHandler struct {
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	eventBus   ports.EventBus
	uowFactory ports.UnitOfWorkFactory
	logger     ports.Logger
	metrics    ports.Metrics
}

// NewBulkDeleteNodesHandler creates a new bulk delete nodes handler
func NewBulkDeleteNodesHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *BulkDeleteNodesHandler {
	return &BulkDeleteNodesHandler{
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes the bulk delete nodes command
func (h *BulkDeleteNodesHandler) Handle(ctx context.Context, command cqrs.Command) error {
	cmd, ok := command.(*BulkDeleteNodesCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start metrics
	timer := h.metrics.StartTimer("command.bulk_delete_nodes.duration",
		ports.Tag{Key: "count", Value: fmt.Sprintf("%d", len(cmd.NodeIDs))})
	defer timer.Stop()
	
	// Create unit of work for transaction
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	deletedNodes := []string{}
	failedNodes := []string{}
	
	for _, nodeID := range cmd.NodeIDs {
		// Get the node to verify it exists and belongs to the user
		node, err := h.nodeRepo.FindByID(ctx, nodeID)
		if err != nil {
			h.logger.Warn("Node not found for bulk delete",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "error", Value: err.Error()})
			failedNodes = append(failedNodes, nodeID)
			continue
		}
		
		if node.GetUserID() != cmd.UserID {
			h.logger.Warn("Unauthorized bulk delete attempt",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "user_id", Value: cmd.UserID})
			failedNodes = append(failedNodes, nodeID)
			continue
		}
		
		// Delete all edges connected to this node
		edges, err := h.edgeRepo.FindEdgesByNode(ctx, nodeID)
		if err != nil {
			h.logger.Warn("Failed to find edges for node",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "error", Value: err.Error()})
		} else {
			for _, edge := range edges {
				if err := h.edgeRepo.DeleteEdge(ctx, edge.SourceID, edge.TargetID); err != nil {
					h.logger.Warn("Failed to delete edge during bulk delete",
						ports.Field{Key: "source", Value: edge.SourceID},
						ports.Field{Key: "target", Value: edge.TargetID},
						ports.Field{Key: "error", Value: err.Error()})
				}
			}
		}
		
		// Delete the node
		if err := h.nodeRepo.Delete(ctx, nodeID); err != nil {
			h.logger.Error("Failed to delete node during bulk delete", err,
				ports.Field{Key: "node_id", Value: nodeID})
			failedNodes = append(failedNodes, nodeID)
			continue
		}
		
		deletedNodes = append(deletedNodes, nodeID)
	}
	
	// If no nodes were successfully deleted, rollback
	if len(deletedNodes) == 0 {
		h.metrics.IncrementCounter("command.bulk_delete_nodes.all_failed")
		return fmt.Errorf("failed to delete any nodes")
	}
	
	// Commit the transaction
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// Publish bulk delete event for successfully deleted nodes
	if len(deletedNodes) > 0 {
		event := events.BulkNodesDeleted{
			NodeIDs:   deletedNodes,
			UserID:    cmd.UserID,
			Timestamp: cmd.Timestamp.Unix(),
		}
		
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Error("Failed to publish bulk nodes deleted event", err,
				ports.Field{Key: "count", Value: len(deletedNodes)})
		}
	}
	
	// Log results
	h.logger.Info("Bulk delete completed",
		ports.Field{Key: "deleted_count", Value: len(deletedNodes)},
		ports.Field{Key: "failed_count", Value: len(failedNodes)},
		ports.Field{Key: "user_id", Value: cmd.UserID})
	
	h.metrics.IncrementCounter("command.bulk_delete_nodes.success",
		ports.Tag{Key: "deleted", Value: fmt.Sprintf("%d", len(deletedNodes))},
		ports.Tag{Key: "failed", Value: fmt.Sprintf("%d", len(failedNodes))})
	
	// Return error if some nodes failed
	if len(failedNodes) > 0 {
		return fmt.Errorf("deleted %d nodes, failed to delete %d nodes", 
			len(deletedNodes), len(failedNodes))
	}
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *BulkDeleteNodesHandler) CanHandle(command cqrs.Command) bool {
	_, ok := command.(*BulkDeleteNodesCommand)
	return ok
}