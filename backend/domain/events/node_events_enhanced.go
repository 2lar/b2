package events

import (
	"time"
	"backend/domain/core/valueobjects"
)

// EdgeCandidate represents a potential edge to be created asynchronously
type EdgeCandidate struct {
	SourceID   string  `json:"sourceId"`
	TargetID   string  `json:"targetId"`
	Type       string  `json:"type"`
	Similarity float64 `json:"similarity"`
}

// NodeCreatedWithPendingEdges is emitted when a node is created with async edges pending
type NodeCreatedWithPendingEdges struct {
	BaseEvent
	NodeID          string          `json:"nodeId"`
	GraphID         string          `json:"graphId"`
	UserID          string          `json:"userId"`
	Title           string          `json:"title"`
	Keywords        []string        `json:"keywords"`
	Tags            []string        `json:"tags"`
	SyncEdgesCreated int            `json:"syncEdgesCreated"`
	AsyncCandidates []EdgeCandidate `json:"asyncCandidates"`
}

// NewNodeCreatedWithPendingEdges creates a new enhanced node created event
func NewNodeCreatedWithPendingEdges(
	nodeID valueobjects.NodeID,
	graphID string,
	userID string,
	title string,
	keywords []string,
	tags []string,
	syncEdgesCreated int,
	asyncCandidates []EdgeCandidate,
) *NodeCreatedWithPendingEdges {
	return &NodeCreatedWithPendingEdges{
		BaseEvent: BaseEvent{
			AggregateID: nodeID.String(),
			EventType:   TypeNodeCreatedWithPending,
			Timestamp:   time.Now(),
			Version:     1,
		},
		NodeID:          nodeID.String(),
		GraphID:         graphID,
		UserID:          userID,
		Title:           title,
		Keywords:        keywords,
		Tags:            tags,
		SyncEdgesCreated: syncEdgesCreated,
		AsyncCandidates: asyncCandidates,
	}
}

// GetEventType returns the event type
func (e *NodeCreatedWithPendingEdges) GetEventType() string {
	return e.EventType
}

// GetAggregateID returns the aggregate ID
func (e *NodeCreatedWithPendingEdges) GetAggregateID() string {
	return e.AggregateID
}

// GetTimestamp returns the event timestamp
func (e *NodeCreatedWithPendingEdges) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetVersion returns the event version
func (e *NodeCreatedWithPendingEdges) GetVersion() int {
	return e.Version
}