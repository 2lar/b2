package websocket

import (
	"encoding/json"
	"fmt"
	"time"

	"backend/domain/events"
	"go.uber.org/zap"
)

// EventType represents WebSocket event types
type EventType string

const (
	// System events
	EventConnectionEstablished EventType = "CONNECTION_ESTABLISHED"
	EventPing                  EventType = "PING"
	EventPong                  EventType = "PONG"
	EventError                 EventType = "ERROR"

	// Domain events
	EventNodeCreated   EventType = "NODE_CREATED"
	EventNodeUpdated   EventType = "NODE_UPDATED"
	EventNodeDeleted   EventType = "NODE_DELETED"
	EventEdgeCreated   EventType = "EDGE_CREATED"
	EventEdgeDeleted   EventType = "EDGE_DELETED"
	EventGraphUpdated  EventType = "GRAPH_UPDATED"
	EventGraphDeleted  EventType = "GRAPH_DELETED"
)

// Broadcaster handles broadcasting domain events to WebSocket clients
type Broadcaster struct {
	hub    *Hub
	logger *zap.Logger
}

// NewBroadcaster creates a new event broadcaster
func NewBroadcaster(hub *Hub, logger *zap.Logger) *Broadcaster {
	return &Broadcaster{
		hub:    hub,
		logger: logger,
	}
}

// BroadcastNodeCreated broadcasts a node creation event
func (b *Broadcaster) BroadcastNodeCreated(event events.NodeCreatedEvent) {
	data := map[string]interface{}{
		"nodeId":   event.NodeID,
		"graphId":  event.GraphID,
		"title":    event.Title,
		"content":  event.Content,
		"keywords": event.Keywords,
		"metadata": event.Metadata,
		"createdAt": event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventNodeCreated, data)
}

// BroadcastNodeUpdated broadcasts a node update event
func (b *Broadcaster) BroadcastNodeUpdated(event events.NodeUpdatedEvent) {
	data := map[string]interface{}{
		"nodeId":    event.NodeID,
		"graphId":   event.GraphID,
		"title":     event.Title,
		"content":   event.Content,
		"keywords":  event.Keywords,
		"metadata":  event.Metadata,
		"updatedAt": event.Timestamp.Format(time.RFC3339),
		"version":   event.Version,
	}

	b.broadcastToUser(event.UserID, EventNodeUpdated, data)
}

// BroadcastNodeDeleted broadcasts a node deletion event
func (b *Broadcaster) BroadcastNodeDeleted(event events.NodeDeletedEvent) {
	data := map[string]interface{}{
		"nodeId":    event.NodeID,
		"graphId":   event.GraphID,
		"deletedAt": event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventNodeDeleted, data)
}

// BroadcastEdgeCreated broadcasts an edge creation event
func (b *Broadcaster) BroadcastEdgeCreated(event events.EdgeCreatedEvent) {
	data := map[string]interface{}{
		"edgeId":    event.EdgeID,
		"graphId":   event.GraphID,
		"sourceId":  event.SourceID,
		"targetId":  event.TargetID,
		"type":      event.Type,
		"weight":    event.Weight,
		"metadata":  event.Metadata,
		"createdAt": event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventEdgeCreated, data)
}

// BroadcastEdgeDeleted broadcasts an edge deletion event
func (b *Broadcaster) BroadcastEdgeDeleted(event events.EdgeDeletedEvent) {
	data := map[string]interface{}{
		"edgeId":    event.EdgeID,
		"sourceId":  event.SourceNodeID.String(),
		"targetId":  event.TargetNodeID.String(),
		"deletedAt": event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventEdgeDeleted, data)
}

// BroadcastGraphUpdated broadcasts a graph update event
func (b *Broadcaster) BroadcastGraphUpdated(event events.GraphUpdatedEvent) {
	data := map[string]interface{}{
		"graphId":    event.GraphID,
		"nodeCount":  event.NodeCount,
		"edgeCount":  event.EdgeCount,
		"metadata":   event.Metadata,
		"updatedAt":  event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventGraphUpdated, data)
}

// BroadcastGraphDeleted broadcasts a graph deletion event
func (b *Broadcaster) BroadcastGraphDeleted(event events.GraphDeletedEvent) {
	data := map[string]interface{}{
		"graphId":   event.GraphID,
		"deletedAt": event.Timestamp.Format(time.RFC3339),
	}

	b.broadcastToUser(event.UserID, EventGraphDeleted, data)
}

// BroadcastDomainEvent broadcasts any domain event
func (b *Broadcaster) BroadcastDomainEvent(event events.DomainEvent) {
	switch e := event.(type) {
	case events.NodeCreatedEvent:
		b.BroadcastNodeCreated(e)
	case events.NodeUpdatedEvent:
		b.BroadcastNodeUpdated(e)
	case events.NodeDeletedEvent:
		b.BroadcastNodeDeleted(e)
	case events.EdgeCreatedEvent:
		b.BroadcastEdgeCreated(e)
	case events.EdgeDeletedEvent:
		b.BroadcastEdgeDeleted(e)
	case events.GraphUpdatedEvent:
		b.BroadcastGraphUpdated(e)
	case events.GraphDeletedEvent:
		b.BroadcastGraphDeleted(e)
	default:
		b.logger.Debug("Unknown event type, not broadcasting",
			zap.String("eventType", fmt.Sprintf("%T", event)),
		)
	}
}

// broadcastToUser sends a message to all connections of a specific user
func (b *Broadcaster) broadcastToUser(userID string, eventType EventType, data interface{}) {
	if userID == "" {
		b.logger.Warn("Cannot broadcast to empty user ID",
			zap.String("eventType", string(eventType)),
		)
		return
	}

	err := b.hub.SendToUser(userID, string(eventType), data)
	if err != nil {
		b.logger.Error("Failed to broadcast event",
			zap.String("userID", userID),
			zap.String("eventType", string(eventType)),
			zap.Error(err),
		)
	} else {
		b.logger.Debug("Event broadcasted",
			zap.String("userID", userID),
			zap.String("eventType", string(eventType)),
		)
	}
}

// BroadcastError sends an error message to a user
func (b *Broadcaster) BroadcastError(userID string, errorMessage string, details map[string]interface{}) {
	data := map[string]interface{}{
		"error":   errorMessage,
		"details": details,
		"timestamp": time.Now().Unix(),
	}

	b.broadcastToUser(userID, EventError, data)
}

// BroadcastCustom sends a custom event to a user
func (b *Broadcaster) BroadcastCustom(userID string, eventType string, data interface{}) {
	// Marshal and unmarshal to ensure proper JSON conversion
	jsonData, err := json.Marshal(data)
	if err != nil {
		b.logger.Error("Failed to marshal custom event data",
			zap.Error(err),
			zap.String("eventType", eventType),
		)
		return
	}

	var cleanData interface{}
	if err := json.Unmarshal(jsonData, &cleanData); err != nil {
		b.logger.Error("Failed to unmarshal custom event data",
			zap.Error(err),
			zap.String("eventType", eventType),
		)
		return
	}

	b.broadcastToUser(userID, EventType(eventType), cleanData)
}