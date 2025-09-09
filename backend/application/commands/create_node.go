package commands

import (
	"context"
	"errors"

	"backend/application/ports"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"go.uber.org/zap"
)

// CreateNodeCommand represents the command to create a new node
type CreateNodeCommand struct {
	NodeID  string   `json:"node_id" validate:"required"`
	UserID  string   `json:"user_id" validate:"required"`
	Title   string   `json:"title" validate:"required,min=1,max=200"`
	Content string   `json:"content" validate:"max=50000"`
	Format  string   `json:"format" validate:"oneof=text markdown html json"`
	X       float64  `json:"x" validate:"required"`
	Y       float64  `json:"y" validate:"required"`
	Z       float64  `json:"z"`
	Tags    []string `json:"tags" validate:"max=20,dive,min=1,max=30"`
}

// CreateNodeHandler handles the CreateNodeCommand
type CreateNodeHandler struct {
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
	logger    *zap.Logger
}

// NewCreateNodeHandler creates a new handler instance
func NewCreateNodeHandler(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *CreateNodeHandler {
	return &CreateNodeHandler{
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Handle executes the create node command
func (h *CreateNodeHandler) Handle(ctx context.Context, cmd CreateNodeCommand) (*entities.Node, error) {
	// Create value objects
	content, err := valueobjects.NewNodeContent(
		cmd.Title,
		cmd.Content,
		valueobjects.ContentFormat(cmd.Format),
	)
	if err != nil {
		return nil, err
	}

	position, err := valueobjects.NewPosition3D(cmd.X, cmd.Y, cmd.Z)
	if err != nil {
		return nil, err
	}

	// Create the node entity
	node, err := entities.NewNode(cmd.UserID, content, position)
	if err != nil {
		return nil, err
	}

	// Add tags if provided
	for _, tag := range cmd.Tags {
		if err := node.AddTag(tag); err != nil {
			// Log warning but don't fail the operation
			continue
		}
	}

	// Get or create user's default graph (this prevents duplicates)
	graph, err := h.graphRepo.GetOrCreateDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}

	// Set the graph ID on the node
	node.SetGraphID(graph.ID().String())

	// Save the node with its graph association
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, err
	}

	// Add node to graph
	if err := graph.AddNode(node); err != nil {
		return nil, err
	}

	// Save the updated graph (updates node count, etc.)
	if err := h.graphRepo.Save(ctx, graph); err != nil {
		return nil, err
	}

	// Edge creation now happens asynchronously via EventBridge and connect-node Lambda
	// This prevents race conditions when multiple nodes are created rapidly

	// Publish domain events
	events := node.GetUncommittedEvents()
	events = append(events, graph.GetUncommittedEvents()...)

	if err := h.eventBus.PublishBatch(ctx, events); err != nil {
		// Log error but don't fail - events can be retried
		// In production, you might want to use an outbox pattern
	}

	// Mark events as committed
	node.MarkEventsAsCommitted()
	graph.MarkEventsAsCommitted()

	return node, nil
}

// Validate validates the command
func (cmd CreateNodeCommand) Validate() error {
	if cmd.UserID == "" {
		return errors.New("user ID is required")
	}
	if cmd.Title == "" {
		return errors.New("title is required")
	}
	if len(cmd.Title) > MaxTitleLength {
		return errors.New("title exceeds maximum length")
	}
	if len(cmd.Content) > MaxContentLength {
		return errors.New("content exceeds maximum length")
	}
	return nil
}

const (
	MaxTitleLength   = 200
	MaxContentLength = 50000
)
