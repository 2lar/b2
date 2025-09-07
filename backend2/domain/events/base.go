package events

import (
	"time"
	"backend2/domain/core/valueobjects"
)

// DomainEvent is the base interface for all domain events
// Events represent something that has happened in the past
type DomainEvent interface {
	GetAggregateID() string
	GetEventType() string
	GetTimestamp() time.Time
	GetVersion() int
}

// BaseEvent provides common event fields
type BaseEvent struct {
	AggregateID string    `json:"aggregate_id"`
	EventType   string    `json:"event_type"`
	Timestamp   time.Time `json:"timestamp"`
	Version     int       `json:"version"`
}

func (e BaseEvent) GetAggregateID() string { return e.AggregateID }
func (e BaseEvent) GetEventType() string   { return e.EventType }
func (e BaseEvent) GetTimestamp() time.Time { return e.Timestamp }
func (e BaseEvent) GetVersion() int        { return e.Version }

// Node Events

// NodeCreated is raised when a new node is created
type NodeCreated struct {
	BaseEvent
	NodeID   valueobjects.NodeID `json:"node_id"`
	UserID   string              `json:"user_id"`
}

// NewNodeCreated creates a NodeCreated event
func NewNodeCreated(nodeID valueobjects.NodeID, userID string, timestamp time.Time) NodeCreated {
	return NodeCreated{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "node.created",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID: nodeID,
		UserID: userID,
	}
}

// NodeContentUpdated is raised when node content is updated
type NodeContentUpdated struct {
	BaseEvent
	NodeID      valueobjects.NodeID      `json:"node_id"`
	OldContent  valueobjects.NodeContent `json:"old_content"`
	NewContent  valueobjects.NodeContent `json:"new_content"`
}

// NewNodeContentUpdated creates a NodeContentUpdated event
func NewNodeContentUpdated(nodeID valueobjects.NodeID, oldContent, newContent valueobjects.NodeContent, timestamp time.Time) NodeContentUpdated {
	return NodeContentUpdated{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "node.content_updated",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID:     nodeID,
		OldContent: oldContent,
		NewContent: newContent,
	}
}

// NodeMoved is raised when a node is moved to a new position
type NodeMoved struct {
	BaseEvent
	NodeID      valueobjects.NodeID   `json:"node_id"`
	OldPosition valueobjects.Position `json:"old_position"`
	NewPosition valueobjects.Position `json:"new_position"`
}

// NewNodeMoved creates a NodeMoved event
func NewNodeMoved(nodeID valueobjects.NodeID, oldPos, newPos valueobjects.Position, timestamp time.Time) NodeMoved {
	return NodeMoved{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "node.moved",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID:      nodeID,
		OldPosition: oldPos,
		NewPosition: newPos,
	}
}

// NodePublished is raised when a node is published
type NodePublished struct {
	BaseEvent
	NodeID valueobjects.NodeID `json:"node_id"`
}

// NewNodePublished creates a NodePublished event
func NewNodePublished(nodeID valueobjects.NodeID, timestamp time.Time) NodePublished {
	return NodePublished{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "node.published",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID: nodeID,
	}
}

// NodeArchived is raised when a node is archived
type NodeArchived struct {
	BaseEvent
	NodeID valueobjects.NodeID `json:"node_id"`
}

// NewNodeArchived creates a NodeArchived event
func NewNodeArchived(nodeID valueobjects.NodeID, timestamp time.Time) NodeArchived {
	return NodeArchived{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "node.archived",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID: nodeID,
	}
}

// Edge Events

// NodesConnected is raised when two nodes are connected
type NodesConnected struct {
	BaseEvent
	SourceID valueobjects.NodeID `json:"source_id"`
	TargetID valueobjects.NodeID `json:"target_id"`
	EdgeType string              `json:"edge_type"`
}

// NewNodesConnected creates a NodesConnected event
func NewNodesConnected(sourceID, targetID valueobjects.NodeID, edgeType string, timestamp time.Time) NodesConnected {
	return NodesConnected{
		BaseEvent: BaseEvent{
			AggregateID: sourceID.String(),
			EventType:   "nodes.connected",
			Timestamp:   timestamp,
			Version:     1,
		},
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: edgeType,
	}
}

// NodesDisconnected is raised when two nodes are disconnected
type NodesDisconnected struct {
	BaseEvent
	SourceID valueobjects.NodeID `json:"source_id"`
	TargetID valueobjects.NodeID `json:"target_id"`
}

// NewNodesDisconnected creates a NodesDisconnected event
func NewNodesDisconnected(sourceID, targetID valueobjects.NodeID, timestamp time.Time) NodesDisconnected {
	return NodesDisconnected{
		BaseEvent: BaseEvent{
			AggregateID: sourceID.String(),
			EventType:   "nodes.disconnected",
			Timestamp:   timestamp,
			Version:     1,
		},
		SourceID: sourceID,
		TargetID: targetID,
	}
}

// Node Deletion Events

// NodeDeletedEvent is raised when a node is deleted
type NodeDeletedEvent struct {
	BaseEvent
	NodeID   valueobjects.NodeID `json:"node_id"`
	UserID   string              `json:"user_id"`
	Content  string              `json:"content"`
	Keywords []string            `json:"keywords"`
	Tags     []string            `json:"tags"`
}

// NewNodeDeletedEvent creates a NodeDeletedEvent
func NewNodeDeletedEvent(nodeID valueobjects.NodeID, userID string, content string, keywords, tags []string, timestamp time.Time) NodeDeletedEvent {
	return NodeDeletedEvent{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   "NodeDeleted",
			Timestamp:   timestamp,
			Version:     1,
		},
		NodeID:   nodeID,
		UserID:   userID,
		Content:  content,
		Keywords: keywords,
		Tags:     tags,
	}
}

// Edge Deletion Events

// EdgeDeletedEvent is raised when an edge is deleted
type EdgeDeletedEvent struct {
	BaseEvent
	EdgeID       string              `json:"edge_id"`
	SourceNodeID valueobjects.NodeID `json:"source_node_id"`
	TargetNodeID valueobjects.NodeID `json:"target_node_id"`
	UserID       string              `json:"user_id"`
}

// NewEdgeDeletedEvent creates an EdgeDeletedEvent
func NewEdgeDeletedEvent(edgeID string, sourceID, targetID valueobjects.NodeID, userID string, timestamp time.Time) EdgeDeletedEvent {
	return EdgeDeletedEvent{
		BaseEvent: BaseEvent{
			AggregateID: edgeID,
			EventType:   "EdgeDeleted",
			Timestamp:   timestamp,
			Version:     1,
		},
		EdgeID:       edgeID,
		SourceNodeID: sourceID,
		TargetNodeID: targetID,
		UserID:       userID,
	}
}

// Graph Events

// GraphCreated is raised when a new graph is created
type GraphCreated struct {
	BaseEvent
	GraphID string `json:"graph_id"`
	UserID  string `json:"user_id"`
	Name    string `json:"name"`
}

// NodeAddedToGraph is raised when a node is added to a graph
type NodeAddedToGraph struct {
	BaseEvent
	GraphID string              `json:"graph_id"`
	NodeID  valueobjects.NodeID `json:"node_id"`
}

// NodeRemovedFromGraph is raised when a node is removed from a graph
type NodeRemovedFromGraph struct {
	BaseEvent
	GraphID string              `json:"graph_id"`
	NodeID  valueobjects.NodeID `json:"node_id"`
}