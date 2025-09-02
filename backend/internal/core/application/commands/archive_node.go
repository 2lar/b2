// Package commands contains the ArchiveNodeCommand implementation
package commands

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
)

// ArchiveNodeCommand represents a command to archive a node
type ArchiveNodeCommand struct {
	cqrs.BaseCommand
	NodeID string `json:"node_id"`
	Reason string `json:"reason,omitempty"`
}

// GetCommandName returns the command name
func (c ArchiveNodeCommand) GetCommandName() string {
	return "ArchiveNode"
}

// Validate validates the command
func (c ArchiveNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	return nil
}

// RestoreNodeCommand represents a command to restore an archived node
type RestoreNodeCommand struct {
	cqrs.BaseCommand
	NodeID string `json:"node_id"`
}

// GetCommandName returns the command name
func (c RestoreNodeCommand) GetCommandName() string {
	return "RestoreNode"
}

// Validate validates the command
func (c RestoreNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	return nil
}

// ArchiveNodeHandler handles both archive and restore commands
type ArchiveNodeHandler struct {
	nodeRepo   ports.NodeRepository
	edgeRepo   ports.EdgeRepository
	eventStore ports.EventStore
	eventBus   ports.EventBus
	uowFactory ports.UnitOfWorkFactory
	logger     ports.Logger
	metrics    ports.Metrics
}

// NewArchiveNodeHandler creates a new ArchiveNodeHandler
func NewArchiveNodeHandler(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *ArchiveNodeHandler {
	return &ArchiveNodeHandler{
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes archive or restore commands
func (h *ArchiveNodeHandler) Handle(ctx context.Context, cmd cqrs.Command) error {
	switch command := cmd.(type) {
	case *ArchiveNodeCommand:
		return h.handleArchive(ctx, command)
	case *RestoreNodeCommand:
		return h.handleRestore(ctx, command)
	default:
		return fmt.Errorf("unsupported command type: %T", cmd)
	}
}

// handleArchive processes the ArchiveNodeCommand
func (h *ArchiveNodeHandler) handleArchive(ctx context.Context, command *ArchiveNodeCommand) error {
	// Start unit of work
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	// Load aggregate from event store
	events, err := h.eventStore.LoadEvents(ctx, command.NodeID)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}
	
	if len(events) == 0 {
		return fmt.Errorf("node not found: %s", command.NodeID)
	}
	
	// Rebuild aggregate from events
	aggregate, err := h.rebuildAggregate(command.NodeID, events)
	if err != nil {
		return fmt.Errorf("failed to rebuild aggregate: %w", err)
	}
	
	// Check ownership
	if aggregate.GetUserID() != command.UserID {
		h.metrics.IncrementCounter("node.archive.unauthorized")
		return fmt.Errorf("unauthorized: user does not own this node")
	}
	
	// Check if already archived
	if aggregate.IsArchived() {
		return nil // Idempotent
	}
	
	// Archive the node
	if err := aggregate.Archive(); err != nil {
		return fmt.Errorf("failed to archive node: %w", err)
	}
	
	// Get uncommitted events
	newEvents := aggregate.GetUncommittedEvents()
	
	// Save new events to event store
	if err := h.eventStore.SaveEvents(ctx, command.NodeID, newEvents, aggregate.GetVersion()); err != nil {
		h.metrics.IncrementCounter("node.archive.failed",
			ports.Tag{Key: "reason", Value: "event_store"})
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Update repository
	if err := uow.NodeRepository().Save(ctx, aggregate); err != nil {
		h.metrics.IncrementCounter("node.archive.failed",
			ports.Tag{Key: "reason", Value: "repository"})
		return fmt.Errorf("failed to save node: %w", err)
	}
	
	// Optionally remove edges (cascade)
	if err := h.handleEdgeCascade(ctx, uow, command.NodeID); err != nil {
		h.logger.Warn("Failed to cascade edge removal",
			ports.Field{Key: "node_id", Value: command.NodeID},
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	// Commit unit of work
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	
	// Mark events as committed
	aggregate.MarkEventsAsCommitted()
	
	// Publish events asynchronously
	go h.publishEvents(context.Background(), newEvents)
	
	// Record metrics
	h.metrics.IncrementCounter("node.archived",
		ports.Tag{Key: "node_id", Value: command.NodeID},
		ports.Tag{Key: "has_reason", Value: fmt.Sprintf("%v", command.Reason != "")})
	
	h.logger.Info("Node archived successfully",
		ports.Field{Key: "node_id", Value: command.NodeID},
		ports.Field{Key: "reason", Value: command.Reason})
	
	return nil
}

// handleRestore processes the RestoreNodeCommand
func (h *ArchiveNodeHandler) handleRestore(ctx context.Context, command *RestoreNodeCommand) error {
	// Start unit of work
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	// Load aggregate from event store
	events, err := h.eventStore.LoadEvents(ctx, command.NodeID)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}
	
	if len(events) == 0 {
		return fmt.Errorf("node not found: %s", command.NodeID)
	}
	
	// Rebuild aggregate from events
	aggregate, err := h.rebuildAggregate(command.NodeID, events)
	if err != nil {
		return fmt.Errorf("failed to rebuild aggregate: %w", err)
	}
	
	// Check ownership
	if aggregate.GetUserID() != command.UserID {
		h.metrics.IncrementCounter("node.restore.unauthorized")
		return fmt.Errorf("unauthorized: user does not own this node")
	}
	
	// Check if not archived
	if !aggregate.IsArchived() {
		return nil // Idempotent
	}
	
	// Restore the node
	if err := aggregate.Restore(); err != nil {
		return fmt.Errorf("failed to restore node: %w", err)
	}
	
	// Get uncommitted events
	newEvents := aggregate.GetUncommittedEvents()
	
	// Save new events to event store
	if err := h.eventStore.SaveEvents(ctx, command.NodeID, newEvents, aggregate.GetVersion()); err != nil {
		h.metrics.IncrementCounter("node.restore.failed",
			ports.Tag{Key: "reason", Value: "event_store"})
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Update repository
	if err := uow.NodeRepository().Save(ctx, aggregate); err != nil {
		h.metrics.IncrementCounter("node.restore.failed",
			ports.Tag{Key: "reason", Value: "repository"})
		return fmt.Errorf("failed to save node: %w", err)
	}
	
	// Commit unit of work
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	
	// Mark events as committed
	aggregate.MarkEventsAsCommitted()
	
	// Publish events asynchronously
	go h.publishEvents(context.Background(), newEvents)
	
	// Record metrics
	h.metrics.IncrementCounter("node.restored",
		ports.Tag{Key: "node_id", Value: command.NodeID})
	
	h.logger.Info("Node restored successfully",
		ports.Field{Key: "node_id", Value: command.NodeID})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *ArchiveNodeHandler) CanHandle(cmd cqrs.Command) bool {
	switch cmd.(type) {
	case *ArchiveNodeCommand, *RestoreNodeCommand:
		return true
	default:
		return false
	}
}

// handleEdgeCascade handles cascading edge removal when archiving
func (h *ArchiveNodeHandler) handleEdgeCascade(ctx context.Context, uow ports.UnitOfWork, nodeID string) error {
	// Find all edges connected to this node
	edges, err := uow.EdgeRepository().FindEdgesByNode(ctx, nodeID)
	if err != nil {
		return err
	}
	
	// Delete each edge
	for _, edge := range edges {
		if err := uow.EdgeRepository().DeleteEdge(ctx, edge.SourceID, edge.TargetID); err != nil {
			h.logger.Warn("Failed to delete edge during cascade",
				ports.Field{Key: "source", Value: edge.SourceID},
				ports.Field{Key: "target", Value: edge.TargetID})
		}
	}
	
	return nil
}

// rebuildAggregate rebuilds an aggregate from events
func (h *ArchiveNodeHandler) rebuildAggregate(id string, events []events.DomainEvent) (*node.Aggregate, error) {
	// This would use the actual aggregate's LoadFromHistory method
	// For now, returning a placeholder
	return node.LoadFromHistory(id, events)
}

// publishEvents publishes events to the event bus
func (h *ArchiveNodeHandler) publishEvents(ctx context.Context, events []events.DomainEvent) {
	for _, event := range events {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Error("Failed to publish event",
				err,
				ports.Field{Key: "event_type", Value: event.GetEventType()})
		}
	}
}