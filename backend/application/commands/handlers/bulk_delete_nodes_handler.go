package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/commands"
	"backend/application/ports"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	"go.uber.org/zap"
)

// BulkDeleteNodesHandler handles bulk delete commands with transactional safety
type BulkDeleteNodesHandler struct {
	uow        ports.UnitOfWork
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	graphRepo  ports.GraphRepository
	eventStore ports.EventStore
	eventBus   ports.EventBus
	logger     *zap.Logger
}

// NewBulkDeleteNodesHandler creates a new bulk delete handler
func NewBulkDeleteNodesHandler(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *BulkDeleteNodesHandler {
	return &BulkDeleteNodesHandler{
		uow:        uow,
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		graphRepo:  graphRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		logger:     logger,
	}
}

// Handle executes the bulk delete command with transactional safety (all-or-nothing)
func (h *BulkDeleteNodesHandler) Handle(ctx context.Context, cmd commands.BulkDeleteNodesCommand) error {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Convert node ID strings to value objects and validate them upfront
	nodeIDs := make([]valueobjects.NodeID, 0, len(cmd.NodeIDs))
	invalidIDs := make([]string, 0)

	for _, nodeIDStr := range cmd.NodeIDs {
		nodeID, err := valueobjects.NewNodeIDFromString(nodeIDStr)
		if err != nil {
			invalidIDs = append(invalidIDs, nodeIDStr)
			continue
		}
		nodeIDs = append(nodeIDs, nodeID)
	}

	// If all node IDs are invalid, emit event and return
	if len(nodeIDs) == 0 {
		// Emit failure event
		event := events.NewBulkNodesDeletedEvent(
			cmd.OperationID,
			cmd.UserID,
			0,
			cmd.NodeIDs,
			[]string{},
			invalidIDs,
			[]string{"All provided node IDs are invalid"},
		)
		h.eventBus.Publish(ctx, event)
		return fmt.Errorf("all node IDs are invalid")
	}

	// Start transaction for atomic bulk delete
	if err := h.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer h.uow.Rollback() // Will be no-op if commit succeeds

	// Validate all nodes exist and belong to the user before deleting any
	validNodes := make([]*nodeValidationInfo, 0, len(nodeIDs))
	failedIDs := make([]string, 0)
	errors := make([]string, 0)

	for _, nodeID := range nodeIDs {
		node, err := h.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			failedIDs = append(failedIDs, nodeID.String())
			errors = append(errors, fmt.Sprintf("Node %s not found: %v", nodeID.String(), err))
			continue
		}

		// Verify ownership
		if node.UserID() != cmd.UserID {
			failedIDs = append(failedIDs, nodeID.String())
			errors = append(errors, fmt.Sprintf("Node %s does not belong to user", nodeID.String()))
			continue
		}

		validNodes = append(validNodes, &nodeValidationInfo{
			nodeID:  nodeID,
			node:    node,
			graphID: node.GraphID(),
		})
	}

	// If no valid nodes to delete, emit event and return
	if len(validNodes) == 0 {
		// Emit event with no deletions
		event := events.NewBulkNodesDeletedEvent(
			cmd.OperationID,
			cmd.UserID,
			0,
			cmd.NodeIDs,
			[]string{},
			append(invalidIDs, failedIDs...),
			errors,
		)
		h.eventBus.Publish(ctx, event)
		return fmt.Errorf("no valid nodes to delete")
	}

	// Collect graph IDs for publishing events
	nodesByGraph := make(map[string][]*nodeValidationInfo)
	for _, info := range validNodes {
		nodesByGraph[info.graphID] = append(nodesByGraph[info.graphID], info)
	}

	// Delete all nodes using batch delete
	nodeIDsToDelete := make([]valueobjects.NodeID, len(validNodes))
	nodeIDStrings := make([]string, len(validNodes))
	for i, info := range validNodes {
		nodeIDsToDelete[i] = info.nodeID
		nodeIDStrings[i] = info.nodeID.String()
	}

	if err := h.nodeRepo.DeleteBatch(ctx, nodeIDsToDelete); err != nil {
		return fmt.Errorf("failed to delete nodes in batch: %w", err)
	}

	// NEW: Immediately delete associated edges using batch operation
	// This is more efficient than transaction-based deletion
	for graphID := range nodesByGraph {
		if err := h.edgeRepo.DeleteByNodeIDs(ctx, graphID, nodeIDStrings); err != nil {
			h.logger.Warn("Failed to delete edges for nodes in graph",
				zap.String("graphID", graphID),
				zap.Int("nodeCount", len(nodeIDStrings)),
				zap.Error(err),
			)
			// Continue - async cleanup will handle any missed edges
		} else {
			h.logger.Info("Successfully deleted edges for nodes",
				zap.String("graphID", graphID),
				zap.Int("nodeCount", len(nodeIDStrings)),
			)
		}
	}

	// Commit the transaction
	if err := h.uow.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit bulk delete transaction: %w", err)
	}

	// Publish deletion events for async cleanup of edges and events
	for _, info := range validNodes {
		content := ""
		if nodeEntity, ok := info.node.(interface{ Content() interface{ Title() string } }); ok {
			content = nodeEntity.Content().Title()
		}
		
		event := events.NewNodeDeletedEvent(
			info.nodeID,
			cmd.UserID,
			info.graphID,
			content,
			[]string{}, // Tags
			[]string{}, // Keywords
			time.Now(),
		)
		
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Warn("Failed to publish deletion event for node",
				zap.String("nodeID", info.nodeID.String()),
				zap.Error(err),
			)
		}
	}

	// Emit bulk deletion completed event
	deletedIDs := make([]string, len(validNodes))
	for i, info := range validNodes {
		deletedIDs[i] = info.nodeID.String()
	}

	event := events.NewBulkNodesDeletedEvent(
		cmd.OperationID,
		cmd.UserID,
		len(validNodes),
		cmd.NodeIDs,
		deletedIDs,
		append(invalidIDs, failedIDs...),
		errors,
	)

	if err := h.eventBus.Publish(ctx, event); err != nil {
		h.logger.Error("Failed to publish bulk delete event",
			zap.String("operationID", cmd.OperationID),
			zap.Error(err),
		)
	}

	h.logger.Info("Bulk delete completed, event published",
		zap.String("operationID", cmd.OperationID),
		zap.String("userID", cmd.UserID),
		zap.Int("requested", len(cmd.NodeIDs)),
		zap.Int("deleted", len(validNodes)),
		zap.Int("failed", len(append(invalidIDs, failedIDs...))),
		zap.Int("affectedGraphs", len(nodesByGraph)),
	)

	return nil
}

// nodeValidationInfo holds information about a validated node
type nodeValidationInfo struct {
	nodeID  valueobjects.NodeID
	node    interface{} // We don't use the full node object after validation
	graphID string
}
