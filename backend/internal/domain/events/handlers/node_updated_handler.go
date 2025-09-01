package handlers

import (
	"context"
	
	"brain2-backend/internal/domain/shared"
	"go.uber.org/zap"
)

// NodeUpdatedHandler handles node update events.
type NodeUpdatedHandler struct {
	logger *zap.Logger
}

// NewNodeUpdatedHandler creates a new node updated event handler
func NewNodeUpdatedHandler(logger *zap.Logger) *NodeUpdatedHandler {
	return &NodeUpdatedHandler{
		logger: logger,
	}
}

// Handle processes a node updated event
func (h *NodeUpdatedHandler) Handle(ctx context.Context, event shared.DomainEvent) error {
	// Type assert to get the specific event
	nodeEvent, ok := event.(*shared.NodeUpdatedEvent)
	if !ok {
		return nil // Not our event type
	}
	
	// Log the event
	h.logger.Info("Node updated event handled",
		zap.String("node_id", nodeEvent.AggregateID()),
		zap.String("user_id", nodeEvent.UserID()),
		zap.String("old_title", nodeEvent.OldTitle),
		zap.String("new_title", nodeEvent.NewTitle),
		zap.Time("timestamp", nodeEvent.Timestamp()))
	
	// Here you could:
	// - Update search index
	// - Send notifications
	// - Update cache
	// - Trigger workflows
	
	return nil
}

// CanHandle checks if this handler can process the given event type
func (h *NodeUpdatedHandler) CanHandle(eventType string) bool {
	return eventType == "NodeUpdated"
}