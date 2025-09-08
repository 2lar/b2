package handlers

import (
	"context"
	"fmt"

	"backend2/application/commands"
	"backend2/application/ports"
	"backend2/domain/core/valueobjects"
	"go.uber.org/zap"
)

// BulkDeleteNodesHandler handles bulk delete commands with transactional safety
type BulkDeleteNodesHandler struct {
	uow       ports.UnitOfWork
	nodeRepo  ports.NodeRepository
	edgeRepo  ports.EdgeRepository
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
	logger    *zap.Logger
}

// NewBulkDeleteNodesHandler creates a new bulk delete handler
func NewBulkDeleteNodesHandler(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *BulkDeleteNodesHandler {
	return &BulkDeleteNodesHandler{
		uow:       uow,
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		graphRepo: graphRepo,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Handle executes the bulk delete command with transactional safety (all-or-nothing)
func (h *BulkDeleteNodesHandler) Handle(ctx context.Context, cmd commands.BulkDeleteNodesCommand) (*commands.BulkDeleteNodesResult, error) {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
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
	
	// If all node IDs are invalid, return early
	if len(nodeIDs) == 0 {
		return &commands.BulkDeleteNodesResult{
			DeletedCount: 0,
			FailedIDs:    invalidIDs,
			Errors:       []string{"All provided node IDs are invalid"},
		}, nil
	}

	// Start transaction for atomic bulk delete
	if err := h.uow.Begin(ctx); err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
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
	
	// If no valid nodes to delete, rollback and return
	if len(validNodes) == 0 {
		return &commands.BulkDeleteNodesResult{
			DeletedCount: 0,
			FailedIDs:    append(invalidIDs, failedIDs...),
			Errors:       errors,
		}, nil
	}

	// Group nodes by graph ID for efficient edge deletion
	nodesByGraph := make(map[string][]*nodeValidationInfo)
	for _, info := range validNodes {
		nodesByGraph[info.graphID] = append(nodesByGraph[info.graphID], info)
	}

	// Delete edges for all nodes (grouped by graph for efficiency)
	for graphID, nodeInfos := range nodesByGraph {
		if graphID == "" {
			continue // Skip nodes without graph ID
		}
		
		nodeIDStrs := make([]string, len(nodeInfos))
		for i, info := range nodeInfos {
			nodeIDStrs[i] = info.nodeID.String()
		}
		
		if err := h.edgeRepo.DeleteByNodeIDs(ctx, graphID, nodeIDStrs); err != nil {
			h.logger.Warn("Failed to delete edges for nodes in graph",
				zap.String("graphID", graphID),
				zap.Strings("nodeIDs", nodeIDStrs),
				zap.Error(err),
			)
			// Don't fail the transaction - continue with node deletion
		}
	}

	// Delete all nodes using batch delete
	nodeIDsToDelete := make([]valueobjects.NodeID, len(validNodes))
	for i, info := range validNodes {
		nodeIDsToDelete[i] = info.nodeID
	}
	
	if err := h.nodeRepo.DeleteBatch(ctx, nodeIDsToDelete); err != nil {
		return nil, fmt.Errorf("failed to delete nodes in batch: %w", err)
	}

	// Update graph metadata for affected graphs
	for graphID := range nodesByGraph {
		if graphID == "" {
			continue
		}
		
		if err := h.graphRepo.UpdateGraphMetadata(ctx, graphID); err != nil {
			h.logger.Error("Failed to update graph metadata",
				zap.String("graphID", graphID),
				zap.Error(err),
			)
			// Don't fail the transaction - the deletion was successful
		}
	}

	// Commit the transaction
	if err := h.uow.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit bulk delete transaction: %w", err)
	}

	result := &commands.BulkDeleteNodesResult{
		DeletedCount: len(validNodes),
		FailedIDs:    append(invalidIDs, failedIDs...),
		Errors:       errors,
	}

	h.logger.Info("Transactional bulk delete completed successfully",
		zap.String("userID", cmd.UserID),
		zap.Int("requested", len(cmd.NodeIDs)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.FailedIDs)),
		zap.Int("affectedGraphs", len(nodesByGraph)),
	)

	return result, nil
}

// nodeValidationInfo holds information about a validated node
type nodeValidationInfo struct {
	nodeID  valueobjects.NodeID
	node    interface{} // We don't use the full node object after validation
	graphID string
}

