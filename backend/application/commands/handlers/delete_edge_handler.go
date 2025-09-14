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

// DeleteEdgeHandler handles the deletion of edges between nodes
type DeleteEdgeHandler struct {
	uow       ports.UnitOfWork
	graphRepo ports.GraphRepository
	nodeRepo  ports.NodeRepository
	eventBus  ports.EventBus
}

// NewDeleteEdgeHandler creates a new handler for edge deletion
func NewDeleteEdgeHandler(
	uow ports.UnitOfWork,
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	eventBus ports.EventBus,
) *DeleteEdgeHandler {
	return &DeleteEdgeHandler{
		uow:       uow,
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		eventBus:  eventBus,
	}
}

// Handle executes the delete edge command
func (h *DeleteEdgeHandler) Handle(ctx context.Context, cmd interface{}) error {
	deleteCmd, ok := cmd.(*commands.DeleteEdgeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}

	// Validate the command
	if err := deleteCmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Begin transaction for atomic edge deletion
	if err := h.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer h.uow.Rollback() // Will be no-op if commit succeeds

	// Get the graph
	graphID := aggregates.GraphID(deleteCmd.GraphID)
	graph, err := h.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return fmt.Errorf("failed to get graph: %w", err)
	}

	// Ensure the user owns the graph
	if graph.UserID() != deleteCmd.UserID {
		return fmt.Errorf("user does not own the graph")
	}

	// Find the edge by ID in this graph
	var sourceID, targetID valueobjects.NodeID
	var edgeFound bool

	edges := graph.Edges()
	for _, edge := range edges {
		if edge.ID == deleteCmd.EdgeID {
			sourceID = edge.SourceID
			targetID = edge.TargetID
			edgeFound = true
			break
		}
	}

	if !edgeFound {
		return fmt.Errorf("edge not found in graph")
	}

	// Get both nodes to validate user ownership
	sourceNode, err := h.nodeRepo.GetByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("source node not found: %w", err)
	}

	targetNode, err := h.nodeRepo.GetByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}

	// Ensure the user owns the source node
	if sourceNode.UserID() != deleteCmd.UserID {
		return fmt.Errorf("user does not own the source node")
	}

	// Disconnect the nodes in the graph aggregate
	deletedEdge, err := graph.DisconnectNodes(sourceID, targetID)
	if err != nil {
		return fmt.Errorf("failed to disconnect nodes in graph: %w", err)
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

	// Save the updated nodes using UoW (they were already disconnected via graph.DisconnectNodes)
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
		return fmt.Errorf("failed to commit edge deletion transaction: %w", err)
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

	// Log successful deletion for debugging
	fmt.Printf("Successfully deleted edge %s (%s -> %s)\n",
		deletedEdge.ID, sourceID.String(), targetID.String())

	return nil
}