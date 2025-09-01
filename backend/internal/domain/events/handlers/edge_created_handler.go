package handlers

import (
	"context"
	
	"brain2-backend/internal/domain/shared"
	"go.uber.org/zap"
)

// EdgeCreatedHandler handles edge creation events.
type EdgeCreatedHandler struct {
	logger *zap.Logger
}

// NewEdgeCreatedHandler creates a new edge created event handler
func NewEdgeCreatedHandler(logger *zap.Logger) *EdgeCreatedHandler {
	return &EdgeCreatedHandler{
		logger: logger,
	}
}

// Handle processes an edge created event
func (h *EdgeCreatedHandler) Handle(ctx context.Context, event shared.DomainEvent) error {
	// Type assert to get the specific event
	edgeEvent, ok := event.(*shared.EdgeCreatedEvent)
	if !ok {
		return nil // Not our event type
	}
	
	// Log the event
	h.logger.Info("Edge created event handled",
		zap.String("edge_id", edgeEvent.AggregateID()),
		zap.String("user_id", edgeEvent.UserID()),
		zap.String("source_node_id", edgeEvent.SourceNodeID),
		zap.String("target_node_id", edgeEvent.TargetNodeID),
		zap.Float64("weight", edgeEvent.Weight),
		zap.Time("timestamp", edgeEvent.Timestamp()))
	
	// Here you could:
	// - Update graph index
	// - Calculate new connections
	// - Update recommendation engine
	// - Send notifications about new connections
	
	return nil
}

// CanHandle checks if this handler can process the given event type
func (h *EdgeCreatedHandler) CanHandle(eventType string) bool {
	return eventType == "EdgeCreated"
}