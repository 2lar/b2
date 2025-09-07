package handlers

import (
	"context"
	"fmt"

	"backend2/application/commands"
	"backend2/application/ports"
	"backend2/domain/core/valueobjects"
	"go.uber.org/zap"
)

// UpdateNodeHandler handles node update commands
type UpdateNodeHandler struct {
	nodeRepo     ports.NodeRepository
	eventStore   ports.EventStore
	eventBus     ports.EventBus
	logger       *zap.Logger
}

// NewUpdateNodeHandler creates a new update node handler
func NewUpdateNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *UpdateNodeHandler {
	return &UpdateNodeHandler{
		nodeRepo:   nodeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		logger:     logger,
	}
}

// Handle executes the update node command
func (h *UpdateNodeHandler) Handle(ctx context.Context, cmd commands.UpdateNodeCommand) error {
	// Validate command
	if err := cmd.Validate(); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Create NodeID value object
	nodeID, err := valueobjects.NewNodeIDFromString(cmd.NodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}

	// Fetch existing node
	node, err := h.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Verify ownership
	if node.UserID() != cmd.UserID {
		return fmt.Errorf("node does not belong to user")
	}

	// Apply updates using the Node's update methods
	if cmd.Title != nil || cmd.Content != nil || cmd.Format != nil {
		currentContent := node.Content()
		title := currentContent.Title()
		content := currentContent.Body()
		format := currentContent.Format()
		
		if cmd.Title != nil {
			title = *cmd.Title
		}
		if cmd.Content != nil {
			content = *cmd.Content
		}
		if cmd.Format != nil {
			format = valueobjects.ContentFormat(*cmd.Format)
		}
		
		newContent, err := valueobjects.NewNodeContent(title, content, format)
		if err != nil {
			return fmt.Errorf("invalid content: %w", err)
		}
		
		if err := node.UpdateContent(newContent); err != nil {
			return fmt.Errorf("failed to update content: %w", err)
		}
	}

	// Update position if provided
	if cmd.X != nil || cmd.Y != nil || cmd.Z != nil {
		currentPosition := node.Position()
		x := currentPosition.X()
		y := currentPosition.Y()
		z := currentPosition.Z()
		
		if cmd.X != nil {
			x = *cmd.X
		}
		if cmd.Y != nil {
			y = *cmd.Y
		}
		if cmd.Z != nil {
			z = *cmd.Z
		}
		
		newPosition, err := valueobjects.NewPosition3D(x, y, z)
		if err != nil {
			return fmt.Errorf("invalid position: %w", err)
		}
		
		if err := node.MoveTo(newPosition); err != nil {
			return fmt.Errorf("failed to update position: %w", err)
		}
	}

	// Update tags if provided
	if cmd.Tags != nil {
		// Remove existing tags first
		existingTags := node.GetTags()
		for _, tag := range existingTags {
			if err := node.RemoveTag(tag); err != nil {
				h.logger.Warn("Failed to remove tag", zap.String("tag", tag), zap.Error(err))
			}
		}
		
		// Add new tags
		for _, tag := range *cmd.Tags {
			if err := node.AddTag(tag); err != nil {
				h.logger.Warn("Failed to add tag", zap.String("tag", tag), zap.Error(err))
			}
		}
	}

	// Save updated node
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return fmt.Errorf("failed to save node: %w", err)
	}

	// Store and publish events
	for _, event := range node.GetUncommittedEvents() {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Warn("Failed to publish event", zap.Error(err))
		}
	}
	
	// Mark events as committed
	node.MarkEventsAsCommitted()

	h.logger.Info("Node updated",
		zap.String("nodeID", cmd.NodeID),
		zap.String("userID", cmd.UserID),
	)

	return nil
}