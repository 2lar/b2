package commands

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
)

// CreateEdgeCommand represents a command to create an edge between two nodes
type CreateEdgeCommand struct {
	EdgeID   string                 `json:"edge_id"`
	UserID   string                 `json:"user_id"`
	GraphID  string                 `json:"graph_id"`
	SourceID string                 `json:"source_id"`
	TargetID string                 `json:"target_id"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Validate validates the command
func (c CreateEdgeCommand) Validate() error {
	if c.EdgeID == "" {
		return fmt.Errorf("edge ID is required")
	}
	if c.SourceID == "" {
		return fmt.Errorf("source node ID is required")
	}
	if c.TargetID == "" {
		return fmt.Errorf("target node ID is required")
	}
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.Type == "" {
		return fmt.Errorf("edge type is required")
	}
	if c.Weight < 0 || c.Weight > 1 {
		return fmt.Errorf("weight must be between 0 and 1")
	}
	return nil
}

// CreateEdgeHandler handles the creation of edges between nodes
type CreateEdgeHandler struct {
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
}

// NewCreateEdgeHandler creates a new handler for edge creation
func NewCreateEdgeHandler(nodeRepo ports.NodeRepository, graphRepo ports.GraphRepository, eventBus ports.EventBus) *CreateEdgeHandler {
	return &CreateEdgeHandler{
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		eventBus:  eventBus,
	}
}

// Handle executes the create edge command
func (h *CreateEdgeHandler) Handle(ctx context.Context, cmd interface{}) error {
	createCmd, ok := cmd.(*CreateEdgeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}

	// Validate the command
	if err := createCmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Parse node IDs
	sourceID, err := valueobjects.NewNodeIDFromString(createCmd.SourceID)
	if err != nil {
		return fmt.Errorf("invalid source node ID: %w", err)
	}

	targetID, err := valueobjects.NewNodeIDFromString(createCmd.TargetID)
	if err != nil {
		return fmt.Errorf("invalid target node ID: %w", err)
	}

	// Get both nodes to validate they exist and get their graph ID
	sourceNode, err := h.nodeRepo.GetByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("source node not found: %w", err)
	}

	targetNode, err := h.nodeRepo.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}

	// Ensure both nodes belong to the same graph
	if sourceNode.GraphID() != targetNode.GraphID() {
		return fmt.Errorf("nodes belong to different graphs")
	}

	// Ensure the user owns the source node
	if sourceNode.UserID() != createCmd.UserID {
		return fmt.Errorf("user does not own the source node")
	}

	// Get the graph
	graphID := aggregates.GraphID(sourceNode.GraphID())
	graph, err := h.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return fmt.Errorf("failed to get graph: %w", err)
	}

	// Create the edge in the graph aggregate
	edgeType := entities.EdgeType(createCmd.Type)
	if edgeType == "" {
		edgeType = entities.EdgeTypeSimilar
	}

	edge, err := graph.ConnectNodes(sourceID, targetID, edgeType)
	if err != nil {
		return fmt.Errorf("failed to connect nodes in graph: %w", err)
	}

	// Set the weight if provided
	if createCmd.Weight > 0 {
		edge.Weight = createCmd.Weight
	}

	// Set metadata if provided
	if createCmd.Metadata != nil {
		edge.Metadata = createCmd.Metadata
	}

	// Save the updated graph
	if err := h.graphRepo.Save(ctx, graph); err != nil {
		return fmt.Errorf("failed to save graph: %w", err)
	}

	// Also connect the nodes themselves
	if err := sourceNode.ConnectTo(targetID, edgeType); err != nil {
		return fmt.Errorf("failed to connect source node: %w", err)
	}
	if err := targetNode.ConnectTo(sourceID, edgeType); err != nil {
		return fmt.Errorf("failed to connect target node: %w", err)
	}

	// Save the updated nodes
	if err := h.nodeRepo.Save(ctx, sourceNode); err != nil {
		return fmt.Errorf("failed to save source node: %w", err)
	}
	if err := h.nodeRepo.Save(ctx, targetNode); err != nil {
		return fmt.Errorf("failed to save target node: %w", err)
	}

	// Publish domain events
	var allEvents []events.DomainEvent
	allEvents = append(allEvents, graph.GetUncommittedEvents()...)
	allEvents = append(allEvents, sourceNode.GetUncommittedEvents()...)
	allEvents = append(allEvents, targetNode.GetUncommittedEvents()...)

	if err := h.eventBus.PublishBatch(ctx, allEvents); err != nil {
		// Log error but don't fail
	}

	// Mark events as committed
	graph.MarkEventsAsCommitted()
	sourceNode.MarkEventsAsCommitted()
	targetNode.MarkEventsAsCommitted()

	return nil
}

// DeleteEdgeCommand represents a command to delete an edge between two nodes
type DeleteEdgeCommand struct {
	UserID string `json:"user_id"`
	EdgeID string `json:"edge_id"`
}

// Validate validates the command
func (c DeleteEdgeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.EdgeID == "" {
		return fmt.Errorf("edge ID is required")
	}
	return nil
}

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
	deleteCmd, ok := cmd.(*DeleteEdgeCommand)
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
