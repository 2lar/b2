// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
)

// CreateNodeCommand represents a command to create a new node
type CreateNodeCommand struct {
	cqrs.BaseCommand
	Content        string   `json:"content"`
	Title          string   `json:"title"`
	Tags           []string `json:"tags"`
	CategoryIDs    []string `json:"category_ids"`
	IdempotencyKey string   `json:"idempotency_key"`
}

// GetCommandName returns the command name
func (c CreateNodeCommand) GetCommandName() string {
	return "CreateNode"
}

// Validate validates the command
func (c CreateNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
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
	if len(c.Tags) > 20 {
		return fmt.Errorf("too many tags (maximum 20)")
	}
	return nil
}

// CreateNodeResult represents the result of node creation
type CreateNodeResult struct {
	NodeID          string    `json:"node_id"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	ExtractedKeywords []string `json:"extracted_keywords"`
	SuggestedConnections []string `json:"suggested_connections"`
}

// CreateNodeHandler handles the CreateNodeCommand
type CreateNodeHandler struct {
	nodeRepo      ports.NodeRepository
	eventStore    ports.EventStore
	eventBus      ports.EventBus
	uowFactory    ports.UnitOfWorkFactory
	logger        ports.Logger
	metrics       ports.Metrics
}

// NewCreateNodeHandler creates a new CreateNodeHandler
func NewCreateNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *CreateNodeHandler {
	return &CreateNodeHandler{
		nodeRepo:   nodeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes the CreateNodeCommand
func (h *CreateNodeHandler) Handle(ctx context.Context, cmd cqrs.Command) error {
	command, ok := cmd.(*CreateNodeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start unit of work
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	// Check idempotency
	if command.IdempotencyKey != "" {
		exists, err := h.checkIdempotency(ctx, command.IdempotencyKey)
		if err != nil {
			return err
		}
		if exists {
			h.logger.Info("Idempotent request detected, skipping",
				ports.Field{Key: "idempotency_key", Value: command.IdempotencyKey})
			return nil
		}
	}
	
	// Create value objects
	nodeID := valueobjects.NewNodeID("")
	userID := valueobjects.NewUserID(command.UserID)
	content := valueobjects.NewContent(command.Content)
	title := valueobjects.NewTitle(command.Title)
	tags := valueobjects.NewTags(command.Tags)
	
	// Create aggregate
	aggregate, err := node.NewAggregate(nodeID, userID, content, title, tags)
	if err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
			ports.Tag{Key: "reason", Value: "validation"})
		return fmt.Errorf("failed to create node aggregate: %w", err)
	}
	
	// Add categories if specified
	for _, categoryID := range command.CategoryIDs {
		if err := aggregate.Categorize(categoryID); err != nil {
			h.logger.Warn("Failed to categorize node",
				ports.Field{Key: "category_id", Value: categoryID},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Get uncommitted events
	events := aggregate.GetUncommittedEvents()
	
	// Save to event store
	if err := h.eventStore.SaveEvents(ctx, aggregate.GetID(), events, 0); err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
			ports.Tag{Key: "reason", Value: "event_store"})
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Save aggregate to repository
	if err := uow.NodeRepository().Save(ctx, aggregate); err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
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
	go h.publishEvents(context.Background(), events)
	
	// Record metrics
	h.metrics.IncrementCounter("node.created",
		ports.Tag{Key: "user_id", Value: command.UserID},
		ports.Tag{Key: "has_tags", Value: fmt.Sprintf("%v", len(command.Tags) > 0)})
	
	h.logger.Info("Node created successfully",
		ports.Field{Key: "node_id", Value: aggregate.GetID()},
		ports.Field{Key: "user_id", Value: command.UserID})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *CreateNodeHandler) CanHandle(cmd cqrs.Command) bool {
	_, ok := cmd.(*CreateNodeCommand)
	return ok
}

// checkIdempotency checks if a request with the given key was already processed
func (h *CreateNodeHandler) checkIdempotency(ctx context.Context, key string) (bool, error) {
	// Implementation would check an idempotency store
	// For now, return false to indicate not processed
	return false, nil
}

// publishEvents publishes events to the event bus
func (h *CreateNodeHandler) publishEvents(ctx context.Context, events []events.DomainEvent) {
	for _, event := range events {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Error("Failed to publish event",
				err,
				ports.Field{Key: "event_type", Value: event.GetEventType()})
		}
	}
}