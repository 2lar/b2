// Package commands contains the UpdateNodeCommand implementation
package commands

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
)

// UpdateNodeCommand represents a command to update an existing node
type UpdateNodeCommand struct {
	cqrs.BaseCommand
	NodeID         string   `json:"node_id"`
	Content        string   `json:"content"`
	Title          string   `json:"title"`
	Tags           []string `json:"tags"`
	ExpectedVersion int64   `json:"expected_version"`
}

// GetCommandName returns the command name
func (c UpdateNodeCommand) GetCommandName() string {
	return "UpdateNode"
}

// Validate validates the command
func (c UpdateNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if c.Content == "" {
		return fmt.Errorf("content is required")
	}
	if len(c.Content) > 10000 {
		return fmt.Errorf("content exceeds maximum length")
	}
	if len(c.Title) > 200 {
		return fmt.Errorf("title exceeds maximum length")
	}
	if c.ExpectedVersion < 0 {
		return fmt.Errorf("invalid expected version")
	}
	return nil
}

// UpdateNodeHandler handles the UpdateNodeCommand
type UpdateNodeHandler struct {
	nodeRepo   ports.NodeRepository
	eventStore ports.EventStore
	eventBus   ports.EventBus
	uowFactory ports.UnitOfWorkFactory
	logger     ports.Logger
	metrics    ports.Metrics
}

// NewUpdateNodeHandler creates a new UpdateNodeHandler
func NewUpdateNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *UpdateNodeHandler {
	return &UpdateNodeHandler{
		nodeRepo:   nodeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes the UpdateNodeCommand
func (h *UpdateNodeHandler) Handle(ctx context.Context, cmd cqrs.Command) error {
	command, ok := cmd.(*UpdateNodeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
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
		h.metrics.IncrementCounter("node.update.unauthorized")
		return fmt.Errorf("unauthorized: user does not own this node")
	}
	
	// Check version for optimistic locking
	if command.ExpectedVersion > 0 && aggregate.GetVersion() != command.ExpectedVersion {
		h.metrics.IncrementCounter("node.update.version_conflict")
		return fmt.Errorf("version conflict: expected %d, got %d", 
			command.ExpectedVersion, aggregate.GetVersion())
	}
	
	// Create value objects
	newContent := valueobjects.NewContent(command.Content)
	newTitle := valueobjects.NewTitle(command.Title)
	
	// Update aggregate
	if err := aggregate.UpdateContent(newContent, newTitle); err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}
	
	// Update tags if changed
	if len(command.Tags) > 0 {
		if err := aggregate.AddTags(command.Tags...); err != nil {
			h.logger.Warn("Failed to update tags",
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Get uncommitted events
	newEvents := aggregate.GetUncommittedEvents()
	
	if len(newEvents) == 0 {
		// No changes were made
		return nil
	}
	
	// Save new events to event store
	if err := h.eventStore.SaveEvents(ctx, command.NodeID, newEvents, aggregate.GetVersion()); err != nil {
		h.metrics.IncrementCounter("node.update.failed",
			ports.Tag{Key: "reason", Value: "event_store"})
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Save updated aggregate to repository
	if err := uow.NodeRepository().Save(ctx, aggregate); err != nil {
		h.metrics.IncrementCounter("node.update.failed",
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
	h.metrics.IncrementCounter("node.updated",
		ports.Tag{Key: "node_id", Value: command.NodeID})
	
	h.logger.Info("Node updated successfully",
		ports.Field{Key: "node_id", Value: command.NodeID},
		ports.Field{Key: "version", Value: aggregate.GetVersion()})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *UpdateNodeHandler) CanHandle(cmd cqrs.Command) bool {
	_, ok := cmd.(*UpdateNodeCommand)
	return ok
}

// rebuildAggregate rebuilds an aggregate from events
func (h *UpdateNodeHandler) rebuildAggregate(id string, events []events.DomainEvent) (*node.Aggregate, error) {
	// This would use the actual aggregate's LoadFromHistory method
	return node.LoadFromHistory(id, events)
}

// publishEvents publishes events to the event bus
func (h *UpdateNodeHandler) publishEvents(ctx context.Context, events []events.DomainEvent) {
	for _, event := range events {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Error("Failed to publish event",
				err,
				ports.Field{Key: "event_type", Value: event.GetEventType()})
		}
	}
}