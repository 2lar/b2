package handlers

import (
	"context"
	"fmt"

	"backend/application/commands"
	"backend/application/ports"
)

// DeleteEdgeHandler handles the deletion of edges between nodes
type DeleteEdgeHandler struct {
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
}

// NewDeleteEdgeHandler creates a new handler for edge deletion
func NewDeleteEdgeHandler(graphRepo ports.GraphRepository, eventBus ports.EventBus) *DeleteEdgeHandler {
	return &DeleteEdgeHandler{
		graphRepo: graphRepo,
		eventBus:  eventBus,
	}
}

// Handle executes the delete edge command
func (h *DeleteEdgeHandler) Handle(ctx context.Context, cmd interface{}) error {
	deleteCmd, ok := cmd.(*commands.DeleteEdgeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}

	// Simplified implementation
	fmt.Printf("Deleting edge %s\n", deleteCmd.EdgeID)

	// In a full implementation, this would:
	// 1. Delete the edge from the graph
	// 2. Publish EdgeDeleted event

	return nil
}