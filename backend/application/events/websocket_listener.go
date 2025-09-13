package events

import (
	"context"

	"backend/domain/events"
	"backend/interfaces/websocket"
	"go.uber.org/zap"
)

// WebSocketListener listens to domain events and broadcasts them via WebSocket
type WebSocketListener struct {
	broadcaster *websocket.Broadcaster
	logger      *zap.Logger
	enabled     bool
}

// NewWebSocketListener creates a new WebSocket event listener
func NewWebSocketListener(hub *websocket.Hub, logger *zap.Logger) *WebSocketListener {
	return &WebSocketListener{
		broadcaster: websocket.NewBroadcaster(hub, logger),
		logger:      logger,
		enabled:     true,
	}
}

// SetEnabled enables or disables the WebSocket listener
func (l *WebSocketListener) SetEnabled(enabled bool) {
	l.enabled = enabled
	if enabled {
		l.logger.Info("WebSocket event listener enabled")
	} else {
		l.logger.Info("WebSocket event listener disabled")
	}
}

// HandleEvent processes any domain event
func (l *WebSocketListener) HandleEvent(ctx context.Context, event events.DomainEvent) error {
	if !l.enabled {
		return nil
	}

	// Broadcast the event to relevant WebSocket connections
	l.broadcaster.BroadcastDomainEvent(event)
	return nil
}

// HandleNodeCreated handles node creation events
func (l *WebSocketListener) HandleNodeCreated(ctx context.Context, event events.NodeCreatedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting NodeCreated event",
		zap.String("nodeID", event.NodeID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastNodeCreated(event)
	return nil
}

// HandleNodeUpdated handles node update events
func (l *WebSocketListener) HandleNodeUpdated(ctx context.Context, event events.NodeUpdatedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting NodeUpdated event",
		zap.String("nodeID", event.NodeID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastNodeUpdated(event)
	return nil
}

// HandleNodeDeleted handles node deletion events
func (l *WebSocketListener) HandleNodeDeleted(ctx context.Context, event events.NodeDeletedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting NodeDeleted event",
		zap.String("nodeID", event.NodeID.String()),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastNodeDeleted(event)
	return nil
}

// HandleEdgeCreated handles edge creation events
func (l *WebSocketListener) HandleEdgeCreated(ctx context.Context, event events.EdgeCreatedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting EdgeCreated event",
		zap.String("edgeID", event.EdgeID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastEdgeCreated(event)
	return nil
}

// HandleEdgeDeleted handles edge deletion events
func (l *WebSocketListener) HandleEdgeDeleted(ctx context.Context, event events.EdgeDeletedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting EdgeDeleted event",
		zap.String("edgeID", event.EdgeID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastEdgeDeleted(event)
	return nil
}

// HandleGraphUpdated handles graph update events
func (l *WebSocketListener) HandleGraphUpdated(ctx context.Context, event events.GraphUpdatedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting GraphUpdated event",
		zap.String("graphID", event.GraphID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastGraphUpdated(event)
	return nil
}

// HandleGraphDeleted handles graph deletion events
func (l *WebSocketListener) HandleGraphDeleted(ctx context.Context, event events.GraphDeletedEvent) error {
	if !l.enabled {
		return nil
	}

	l.logger.Debug("Broadcasting GraphDeleted event",
		zap.String("graphID", event.GraphID),
		zap.String("userID", event.UserID),
	)

	l.broadcaster.BroadcastGraphDeleted(event)
	return nil
}

// HandleBatch handles a batch of events
func (l *WebSocketListener) HandleBatch(ctx context.Context, events []events.DomainEvent) error {
	if !l.enabled {
		return nil
	}

	for _, event := range events {
		if err := l.HandleEvent(ctx, event); err != nil {
			l.logger.Error("Failed to handle event in batch",
				zap.Error(err),
				zap.String("eventType", event.GetEventType()),
			)
			// Continue processing other events
		}
	}

	return nil
}

// GetEventTypes returns the event types this listener handles
func (l *WebSocketListener) GetEventTypes() []string {
	return []string{
		"NodeCreated",
		"NodeUpdated",
		"NodeDeleted",
		"EdgeCreated",
		"EdgeDeleted",
		"GraphUpdated",
		"GraphDeleted",
	}
}

// GetName returns the name of this listener
func (l *WebSocketListener) GetName() string {
	return "WebSocketListener"
}