package node

import (
	"time"

	"brain2-backend/internal/domain/shared"
)

// NodeCategory represents the enhanced many-to-many relationship between nodes and categories
// This replaces the deprecated CategoryMemory for better functionality
type NodeCategory struct {
	UserID     string    `json:"user_id"`
	NodeID     string    `json:"node_id"`
	CategoryID string    `json:"category_id"`
	AssignedAt time.Time `json:"assigned_at"`
	CreatedAt  time.Time `json:"created_at"`  // Alias for AssignedAt for compatibility
	Confidence float64   `json:"confidence"`  // AI confidence or manual = 1.0
	Source     string    `json:"source"`      // "manual", "ai", "bulk"
	Method     string    `json:"method"`      // Alias for Source for compatibility
	
	// Domain events
	events []shared.DomainEvent `json:"-"`
}

// NewNodeCategory creates a new node-category mapping
func NewNodeCategory(userID, nodeID, categoryID string) (*NodeCategory, error) {
	if userID == "" || nodeID == "" || categoryID == "" {
		return nil, shared.ErrValidation
	}
	
	now := time.Now()
	nodeCategory := &NodeCategory{
		UserID:     userID,
		NodeID:     nodeID,
		CategoryID: categoryID,
		AssignedAt: now,
		CreatedAt:  now,
		Confidence: 1.0, // Default to manual assignment
		Source:     "manual",
		Method:     "manual",
	}
	
	// Generate domain event for node-category assignment
	assignedEvent := shared.NewNodeAssignedToCategoryEvent(nodeID, categoryID, userID, now)
	nodeCategory.addEvent(assignedEvent)
	
	return nodeCategory, nil
}

// NewAINodeCategory creates a new AI-generated node-category mapping
func NewAINodeCategory(userID, nodeID, categoryID string, confidence float64) *NodeCategory {
	now := time.Now()
	return &NodeCategory{
		UserID:     userID,
		NodeID:     nodeID,
		CategoryID: categoryID,
		AssignedAt: now,
		CreatedAt:  now,
		Confidence: confidence,
		Source:     "ai",
		Method:     "ai",
	}
}

// IsHighConfidence returns true if the assignment has high confidence (>= 0.8)
func (nc *NodeCategory) IsHighConfidence() bool {
	return nc.Confidence >= 0.8
}

// IsManual returns true if the assignment was made manually
func (nc *NodeCategory) IsManual() bool {
	return nc.Source == "manual"
}

// EventAggregate implementation

// GetUncommittedEvents returns all uncommitted domain events
func (nc *NodeCategory) GetUncommittedEvents() []shared.DomainEvent {
	return nc.events
}

// MarkEventsAsCommitted clears all domain events
func (nc *NodeCategory) MarkEventsAsCommitted() {
	nc.events = nil
}

// addEvent adds a domain event to the aggregate
func (nc *NodeCategory) addEvent(event shared.DomainEvent) {
	nc.events = append(nc.events, event)
}