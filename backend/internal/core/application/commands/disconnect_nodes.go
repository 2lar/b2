// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// DisconnectNodesCommand represents a command to disconnect two nodes
type DisconnectNodesCommand struct {
	cqrs.BaseCommand
	SourceNodeID string `json:"source_node_id"`
	TargetNodeID string `json:"target_node_id"`
}

// GetCommandName returns the command name
func (c DisconnectNodesCommand) GetCommandName() string {
	return "DisconnectNodes"
}

// Validate validates the command
func (c DisconnectNodesCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.SourceNodeID == "" {
		return fmt.Errorf("source node ID is required")
	}
	if c.TargetNodeID == "" {
		return fmt.Errorf("target node ID is required")
	}
	return nil
}

// DisconnectNodesHandler handles node disconnection commands
type DisconnectNodesHandler struct {
	edgeRepo ports.EdgeRepository
	eventBus ports.EventBus
	logger   ports.Logger
	metrics  ports.Metrics
}

// NewDisconnectNodesHandler creates a new disconnect nodes handler
func NewDisconnectNodesHandler(
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
) *DisconnectNodesHandler {
	return &DisconnectNodesHandler{
		edgeRepo: edgeRepo,
		eventBus: eventBus,
		logger:   logger,
		metrics:  metrics,
	}
}

// Handle processes the disconnect nodes command
func (h *DisconnectNodesHandler) Handle(ctx context.Context, command cqrs.Command) error {
	cmd, ok := command.(*DisconnectNodesCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start metrics
	timer := h.metrics.StartTimer("command.disconnect_nodes.duration")
	defer timer.Stop()
	
	// Check if edge exists
	edge, err := h.edgeRepo.GetEdge(ctx, cmd.SourceNodeID, cmd.TargetNodeID)
	if err != nil {
		h.metrics.IncrementCounter("command.disconnect_nodes.edge_not_found")
		return fmt.Errorf("edge not found: %w", err)
	}
	
	// Verify the edge belongs to the user
	if edge.UserID != cmd.UserID {
		h.metrics.IncrementCounter("command.disconnect_nodes.unauthorized")
		return fmt.Errorf("unauthorized: edge does not belong to user")
	}
	
	// Delete the edge
	if err := h.edgeRepo.DeleteEdge(ctx, cmd.SourceNodeID, cmd.TargetNodeID); err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}
	
	// Publish nodes disconnected event
	event := events.NodesDisconnected{
		SourceNodeID: cmd.SourceNodeID,
		TargetNodeID: cmd.TargetNodeID,
		UserID:       cmd.UserID,
		Timestamp:    cmd.Timestamp.Unix(),
	}
	
	if err := h.eventBus.Publish(ctx, event); err != nil {
		h.logger.Error("Failed to publish nodes disconnected event", err,
			ports.Field{Key: "source", Value: cmd.SourceNodeID},
			ports.Field{Key: "target", Value: cmd.TargetNodeID})
	}
	
	h.metrics.IncrementCounter("command.disconnect_nodes.success")
	h.logger.Info("Nodes disconnected successfully",
		ports.Field{Key: "source", Value: cmd.SourceNodeID},
		ports.Field{Key: "target", Value: cmd.TargetNodeID},
		ports.Field{Key: "user_id", Value: cmd.UserID})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *DisconnectNodesHandler) CanHandle(command cqrs.Command) bool {
	_, ok := command.(*DisconnectNodesCommand)
	return ok
}