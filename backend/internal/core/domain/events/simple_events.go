// Package events contains domain event definitions
package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Simple event structs for command handlers
// These implement the DomainEvent interface

// NodeDeleted is raised when a node is deleted
type NodeDeleted struct {
	NodeID    string
	UserID    string
	Timestamp int64
	eventID   string
}

func (e NodeDeleted) GetEventID() string {
	if e.eventID == "" {
		e.eventID = uuid.New().String()
	}
	return e.eventID
}
func (e NodeDeleted) GetEventType() string     { return "NodeDeleted" }
func (e NodeDeleted) GetAggregateID() string   { return e.NodeID }
func (e NodeDeleted) GetAggregateType() string { return "Node" }
func (e NodeDeleted) GetTimestamp() time.Time  { return time.Unix(e.Timestamp, 0) }
func (e NodeDeleted) GetVersion() int64        { return 1 }
func (e NodeDeleted) GetMetadata() EventMetadata {
	return EventMetadata{UserID: e.UserID}
}
func (e NodeDeleted) GetCorrelationID() string { return "" }
func (e NodeDeleted) GetOccurredAt() time.Time { return e.GetTimestamp() }
func (e NodeDeleted) GetData() interface{}     { return e }
func (e NodeDeleted) Marshal() ([]byte, error) { return json.Marshal(e) }

// NodesConnected is raised when two nodes are connected
type NodesConnected struct {
	SourceNodeID string
	TargetNodeID string
	UserID       string
	Weight       float64
	Timestamp    int64
	eventID      string
}

func (e NodesConnected) GetEventID() string {
	if e.eventID == "" {
		e.eventID = uuid.New().String()
	}
	return e.eventID
}
func (e NodesConnected) GetEventType() string     { return "NodesConnected" }
func (e NodesConnected) GetAggregateID() string   { return e.SourceNodeID }
func (e NodesConnected) GetAggregateType() string { return "Edge" }
func (e NodesConnected) GetTimestamp() time.Time  { return time.Unix(e.Timestamp, 0) }
func (e NodesConnected) GetVersion() int64        { return 1 }
func (e NodesConnected) GetMetadata() EventMetadata {
	return EventMetadata{UserID: e.UserID}
}
func (e NodesConnected) GetCorrelationID() string { return "" }
func (e NodesConnected) GetOccurredAt() time.Time { return e.GetTimestamp() }
func (e NodesConnected) GetData() interface{}     { return e }
func (e NodesConnected) Marshal() ([]byte, error) { return json.Marshal(e) }

// NodesDisconnected is raised when two nodes are disconnected
type NodesDisconnected struct {
	SourceNodeID string
	TargetNodeID string
	UserID       string
	Timestamp    int64
	eventID      string
}

func (e NodesDisconnected) GetEventID() string {
	if e.eventID == "" {
		e.eventID = uuid.New().String()
	}
	return e.eventID
}
func (e NodesDisconnected) GetEventType() string     { return "NodesDisconnected" }
func (e NodesDisconnected) GetAggregateID() string   { return e.SourceNodeID }
func (e NodesDisconnected) GetAggregateType() string { return "Edge" }
func (e NodesDisconnected) GetTimestamp() time.Time  { return time.Unix(e.Timestamp, 0) }
func (e NodesDisconnected) GetVersion() int64        { return 1 }
func (e NodesDisconnected) GetMetadata() EventMetadata {
	return EventMetadata{UserID: e.UserID}
}
func (e NodesDisconnected) GetCorrelationID() string { return "" }
func (e NodesDisconnected) GetOccurredAt() time.Time { return e.GetTimestamp() }
func (e NodesDisconnected) GetData() interface{}     { return e }
func (e NodesDisconnected) Marshal() ([]byte, error) { return json.Marshal(e) }

// BulkNodesDeleted is raised when multiple nodes are deleted
type BulkNodesDeleted struct {
	NodeIDs   []string
	UserID    string
	Timestamp int64
	eventID   string
}

func (e BulkNodesDeleted) GetEventID() string {
	if e.eventID == "" {
		e.eventID = uuid.New().String()
	}
	return e.eventID
}
func (e BulkNodesDeleted) GetEventType() string     { return "BulkNodesDeleted" }
func (e BulkNodesDeleted) GetAggregateID() string   { return e.UserID }
func (e BulkNodesDeleted) GetAggregateType() string { return "User" }
func (e BulkNodesDeleted) GetTimestamp() time.Time  { return time.Unix(e.Timestamp, 0) }
func (e BulkNodesDeleted) GetVersion() int64        { return 1 }
func (e BulkNodesDeleted) GetMetadata() EventMetadata {
	return EventMetadata{UserID: e.UserID}
}
func (e BulkNodesDeleted) GetCorrelationID() string { return "" }
func (e BulkNodesDeleted) GetOccurredAt() time.Time { return e.GetTimestamp() }
func (e BulkNodesDeleted) GetData() interface{}     { return e }
func (e BulkNodesDeleted) Marshal() ([]byte, error) { return json.Marshal(e) }