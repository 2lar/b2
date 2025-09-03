// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/infrastructure/adapters/dynamodb"
)

// DeleteNodeCommand represents a command to delete a node
type DeleteNodeCommand struct {
	cqrs.BaseCommand
	NodeID string `json:"node_id"`
}

// GetCommandName returns the command name
func (c DeleteNodeCommand) GetCommandName() string {
	return "DeleteNode"
}

// Validate validates the command
func (c DeleteNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	return nil
}

// DeleteNodeHandler handles node deletion commands
type DeleteNodeHandler struct {
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	eventBus   ports.EventBus
	uowFactory ports.UnitOfWorkFactory
	cache      ports.Cache
	logger     ports.Logger
	metrics    ports.Metrics
}

// NewDeleteNodeHandler creates a new delete node handler
func NewDeleteNodeHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *DeleteNodeHandler {
	return &DeleteNodeHandler{
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		cache:      nil, // Cache will be injected if available
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes the delete node command
func (h *DeleteNodeHandler) Handle(ctx context.Context, command cqrs.Command) error {
	cmd, ok := command.(*DeleteNodeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start metrics
	timer := h.metrics.StartTimer("command.delete_node.duration")
	defer timer.Stop()
	
	// Create unit of work
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	// Get the node to verify it exists and belongs to the user
	// Use FindByUserAndID if available to avoid scans
	if repo, ok := h.nodeRepo.(*dynamodb.NodeRepository); ok {
		_, err = repo.FindByUserAndID(ctx, cmd.UserID, cmd.NodeID)
		if err != nil {
			h.metrics.IncrementCounter("command.delete_node.not_found")
			return fmt.Errorf("node not found: %w", err)
		}
	} else {
		node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
		if err != nil {
			h.metrics.IncrementCounter("command.delete_node.not_found")
			return fmt.Errorf("node not found: %w", err)
		}
		
		if node.GetUserID() != cmd.UserID {
			h.metrics.IncrementCounter("command.delete_node.unauthorized")
			return fmt.Errorf("unauthorized: node does not belong to user")
		}
	}
	
	// Delete all edges connected to this node
	edges, err := h.edgeRepo.FindEdgesByNode(ctx, cmd.NodeID)
	if err != nil {
		return fmt.Errorf("failed to find edges: %w", err)
	}
	
	for _, edge := range edges {
		if err := h.edgeRepo.DeleteEdge(ctx, edge.SourceID, edge.TargetID); err != nil {
			h.logger.Warn("Failed to delete edge",
				ports.Field{Key: "source", Value: edge.SourceID},
				ports.Field{Key: "target", Value: edge.TargetID},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Delete the node using the proper method
	if repo, ok := h.nodeRepo.(*dynamodb.NodeRepository); ok {
		// Use DeleteByUserAndID for DynamoDB to properly handle composite keys
		if err := repo.DeleteByUserAndID(ctx, cmd.UserID, cmd.NodeID); err != nil {
			return fmt.Errorf("failed to delete node: %w", err)
		}
	} else {
		// Fallback to regular Delete for other implementations
		if err := h.nodeRepo.Delete(ctx, cmd.NodeID); err != nil {
			return fmt.Errorf("failed to delete node: %w", err)
		}
	}
	
	// Commit the transaction
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// Publish node deleted event
	event := events.NodeDeleted{
		NodeID:    cmd.NodeID,
		UserID:    cmd.UserID,
		Timestamp: cmd.Timestamp.Unix(),
	}
	
	if err := h.eventBus.Publish(ctx, event); err != nil {
		h.logger.Error("Failed to publish node deleted event", err,
			ports.Field{Key: "node_id", Value: cmd.NodeID})
	}
	
	// Invalidate cache for user's data
	if h.cache != nil {
		go h.invalidateUserCache(context.Background(), cmd.UserID)
	}
	
	h.metrics.IncrementCounter("command.delete_node.success")
	h.logger.Info("Node deleted successfully",
		ports.Field{Key: "node_id", Value: cmd.NodeID},
		ports.Field{Key: "user_id", Value: cmd.UserID})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *DeleteNodeHandler) CanHandle(command cqrs.Command) bool {
	_, ok := command.(*DeleteNodeCommand)
	return ok
}

// SetCache sets the cache instance for the handler
func (h *DeleteNodeHandler) SetCache(cache ports.Cache) {
	h.cache = cache
}

// invalidateUserCache invalidates cached data for a user
func (h *DeleteNodeHandler) invalidateUserCache(ctx context.Context, userID string) {
	// Clear ALL cache patterns for the user to ensure consistency
	patterns := []string{
		fmt.Sprintf("nodes:user:%s:*", userID),    // User's nodes list
		fmt.Sprintf("graph:user:%s:*", userID),      // User's graph data
		fmt.Sprintf("node:%s:*", userID),           // Individual nodes
		fmt.Sprintf("user:%s:*", userID),           // Any user-specific cache
		fmt.Sprintf("*:%s:*", userID),              // Any cache with userID
	}
	
	for _, pattern := range patterns {
		if err := h.cache.Delete(ctx, pattern); err != nil {
			h.logger.Debug("Failed to invalidate cache",
				ports.Field{Key: "pattern", Value: pattern},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Also try to clear the entire cache if pattern matching is not supported
	// This is a more aggressive approach but ensures consistency
	if err := h.cache.Delete(ctx, "*"); err != nil {
		h.logger.Debug("Failed to clear all cache",
			ports.Field{Key: "error", Value: err.Error()})
	}
}