package events

import (
	"time"
	"backend/domain/core/valueobjects"
)

// WebSocket-compatible event types
// These events are designed to be easily serializable for WebSocket broadcasting

// NodeCreatedEvent represents a node creation for WebSocket broadcasting
type NodeCreatedEvent struct {
	BaseEvent
	NodeID   string                 `json:"nodeId"`
	GraphID  string                 `json:"graphId"`
	UserID   string                 `json:"userId"`
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	Keywords []string               `json:"keywords"`
	Tags     []string               `json:"tags"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NewNodeCreatedEvent creates a new WebSocket-compatible node creation event
func NewNodeCreatedEvent(nodeID valueobjects.NodeID, graphID, userID, title, content string, keywords, tags []string) NodeCreatedEvent {
	return NodeCreatedEvent{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "NodeCreated",
			Timestamp:   time.Now(),
			Version:     1,
		},
		NodeID:   nodeID.String(),
		GraphID:  graphID,
		UserID:   userID,
		Title:    title,
		Content:  content,
		Keywords: keywords,
		Tags:     tags,
		Metadata: make(map[string]interface{}),
	}
}

// NodeUpdatedEvent represents a node update for WebSocket broadcasting
type NodeUpdatedEvent struct {
	BaseEvent
	NodeID   string                 `json:"nodeId"`
	GraphID  string                 `json:"graphId"`
	UserID   string                 `json:"userId"`
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	Keywords []string               `json:"keywords"`
	Tags     []string               `json:"tags"`
	Metadata map[string]interface{} `json:"metadata"`
	Version  int                    `json:"version"`
}

// NewNodeUpdatedEvent creates a new WebSocket-compatible node update event
func NewNodeUpdatedEvent(nodeID valueobjects.NodeID, graphID, userID, title, content string, keywords, tags []string, version int) NodeUpdatedEvent {
	return NodeUpdatedEvent{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "NodeUpdated",
			Timestamp:   time.Now(),
			Version:     1,
		},
		NodeID:   nodeID.String(),
		GraphID:  graphID,
		UserID:   userID,
		Title:    title,
		Content:  content,
		Keywords: keywords,
		Tags:     tags,
		Metadata: make(map[string]interface{}),
		Version:  version,
	}
}

// EdgeCreatedEvent represents an edge creation for WebSocket broadcasting
type EdgeCreatedEvent struct {
	BaseEvent
	EdgeID   string                 `json:"edgeId"`
	GraphID  string                 `json:"graphId"`
	SourceID string                 `json:"sourceId"`
	TargetID string                 `json:"targetId"`
	UserID   string                 `json:"userId"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NewEdgeCreatedEvent creates a new WebSocket-compatible edge creation event
func NewEdgeCreatedEvent(edgeID, graphID, sourceID, targetID, userID, edgeType string, weight float64) EdgeCreatedEvent {
	return EdgeCreatedEvent{
		BaseEvent: BaseEvent{
			AggregateID: edgeID,
			EventType:   "EdgeCreated",
			Timestamp:   time.Now(),
			Version:     1,
		},
		EdgeID:   edgeID,
		GraphID:  graphID,
		SourceID: sourceID,
		TargetID: targetID,
		UserID:   userID,
		Type:     edgeType,
		Weight:   weight,
		Metadata: make(map[string]interface{}),
	}
}

// GraphUpdatedEvent represents a graph update for WebSocket broadcasting
type GraphUpdatedEvent struct {
	BaseEvent
	GraphID   string                 `json:"graphId"`
	UserID    string                 `json:"userId"`
	NodeCount int                    `json:"nodeCount"`
	EdgeCount int                    `json:"edgeCount"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// NewGraphUpdatedEvent creates a new WebSocket-compatible graph update event
func NewGraphUpdatedEvent(graphID, userID string, nodeCount, edgeCount int) GraphUpdatedEvent {
	return GraphUpdatedEvent{
		BaseEvent: BaseEvent{
			AggregateID: graphID,
			EventType:   "GraphUpdated",
			Timestamp:   time.Now(),
			Version:     1,
		},
		GraphID:   graphID,
		UserID:    userID,
		NodeCount: nodeCount,
		EdgeCount: edgeCount,
		Metadata:  make(map[string]interface{}),
	}
}

// GraphDeletedEvent represents a graph deletion for WebSocket broadcasting
type GraphDeletedEvent struct {
	BaseEvent
	GraphID string `json:"graphId"`
	UserID  string `json:"userId"`
}

// NewGraphDeletedEvent creates a new WebSocket-compatible graph deletion event
func NewGraphDeletedEvent(graphID, userID string) GraphDeletedEvent {
	return GraphDeletedEvent{
		BaseEvent: BaseEvent{
			AggregateID: graphID,
			EventType:   "GraphDeleted",
			Timestamp:   time.Now(),
			Version:     1,
		},
		GraphID: graphID,
		UserID:  userID,
	}
}