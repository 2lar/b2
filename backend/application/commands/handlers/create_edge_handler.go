package handlers

import (
	"context"
	"fmt"

	"backend/application/commands"
	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
)

// CreateEdgeHandler handles the creation of edges between nodes
type CreateEdgeHandler struct {
	uow       ports.UnitOfWork
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	edgeRepo  ports.EdgeRepository
	eventBus  ports.EventBus
}

// NewCreateEdgeHandler creates a new handler for edge creation
func NewCreateEdgeHandler(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	eventBus ports.EventBus,
) *CreateEdgeHandler {
	return &CreateEdgeHandler{
		uow:       uow,
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		edgeRepo:  edgeRepo,
		eventBus:  eventBus,
	}
}

// Handle executes the create edge command
func (h *CreateEdgeHandler) Handle(ctx context.Context, cmd interface{}) error {
	createCmd, ok := cmd.(*commands.CreateEdgeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}

	// Validate the command
	if err := createCmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Validate edge type
	edgeType := entities.EdgeType(createCmd.Type)
	if edgeType == "" {
		edgeType = entities.EdgeTypeNormal
	}
	if !edgeType.IsValid() {
		return fmt.Errorf("invalid edge type: %s", createCmd.Type)
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

	// Begin transaction for atomic edge creation
	if err := h.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer h.uow.Rollback() // Will be no-op if commit succeeds

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
	// edgeType was already validated and normalized above

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

	// Save the updated graph using UoW
	if repoWithUoW, ok := h.graphRepo.(interface {
		SaveWithUoW(context.Context, *aggregates.Graph, interface{}) error
	}); ok {
		if err := repoWithUoW.SaveWithUoW(ctx, graph, h.uow); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}
	} else {
		return fmt.Errorf("graph repository does not support unit of work")
	}

	// Save the updated nodes using UoW (they were already connected via graph.ConnectNodes)
	if nodeRepoWithUoW, ok := h.nodeRepo.(interface {
		SaveWithUoW(context.Context, *entities.Node, interface{}) error
	}); ok {
		if err := nodeRepoWithUoW.SaveWithUoW(ctx, sourceNode, h.uow); err != nil {
			return fmt.Errorf("failed to save source node: %w", err)
		}
		if err := nodeRepoWithUoW.SaveWithUoW(ctx, targetNode, h.uow); err != nil {
			return fmt.Errorf("failed to save target node: %w", err)
		}
	} else {
		return fmt.Errorf("node repository does not support unit of work")
	}

	// Commit the transaction
	if err := h.uow.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit edge creation transaction: %w", err)
	}

	// Publish domain events after successful commit
	var allEvents []events.DomainEvent
	allEvents = append(allEvents, graph.GetUncommittedEvents()...)
	allEvents = append(allEvents, sourceNode.GetUncommittedEvents()...)
	allEvents = append(allEvents, targetNode.GetUncommittedEvents()...)

	if err := h.eventBus.PublishBatch(ctx, allEvents); err != nil {
		// Log error but don't fail - events can be retried
	}

	// Mark events as committed after successful publishing
	graph.MarkEventsAsCommitted()
	sourceNode.MarkEventsAsCommitted()
	targetNode.MarkEventsAsCommitted()

	return nil
}