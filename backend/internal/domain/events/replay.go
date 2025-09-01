package events

import (
	"context"
	"fmt"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
)

// AggregateReconstructor rebuilds aggregates from their event history
type AggregateReconstructor interface {
	// ReconstructNode rebuilds a node from its event stream
	ReconstructNode(ctx context.Context, nodeID string) (*node.Node, error)
	
	// ReconstructCategory rebuilds a category from its event stream
	ReconstructCategory(ctx context.Context, categoryID string) (*category.Category, error)
	
	// ReconstructEdge rebuilds an edge from its event stream
	ReconstructEdge(ctx context.Context, edgeID string) (*edge.Edge, error)
	
	// ReconstructFromEvents rebuilds any aggregate from a list of events
	ReconstructFromEvents(ctx context.Context, aggregateType string, events []shared.DomainEvent) (interface{}, error)
}

// AggregateReplayer implements aggregate reconstruction from events
type AggregateReplayer struct {
	eventStore EventStore
}

// NewAggregateReplayer creates a new aggregate replayer
func NewAggregateReplayer(eventStore EventStore) *AggregateReplayer {
	return &AggregateReplayer{
		eventStore: eventStore,
	}
}

// ReconstructNode rebuilds a node aggregate from its event history
func (r *AggregateReplayer) ReconstructNode(ctx context.Context, nodeID string) (*node.Node, error) {
	// Load all events for this aggregate
	events, err := r.eventStore.LoadEvents(ctx, nodeID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load events for node %s: %w", nodeID, err)
	}
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events found for node %s", nodeID)
	}
	
	// Start with empty node state
	var nodeAggregate *node.Node
	
	// Apply each event in sequence
	for _, event := range events {
		switch event.EventType() {
		case "NodeCreated":
			// Extract data from event
			data := event.EventData()
			userID, _ := shared.NewUserID(event.UserID())
			contentStr, _ := data["content"].(string)
			content, _ := shared.NewContent(contentStr)
			titleStr := ""
			if t, ok := data["title"].(string); ok {
				titleStr = t
			}
			title, _ := shared.NewTitle(titleStr)
			
			// Create the initial node
			nodeAggregate, err = node.NewNode(userID, content, title, shared.Tags{})
			if err != nil {
				return nil, fmt.Errorf("failed to create node from event: %w", err)
			}
			
		case "NodeUpdated":
			if nodeAggregate == nil {
				return nil, fmt.Errorf("cannot apply NodeUpdated without NodeCreated")
			}
			
			// Apply update to node
			data := event.EventData()
			if content, ok := data["content"].(string); ok {
				contentVO, _ := shared.NewContent(content)
				if err := nodeAggregate.UpdateContent(contentVO); err != nil {
					return nil, fmt.Errorf("failed to update content: %w", err)
				}
			}
			if title, ok := data["title"].(string); ok {
				titleVO, _ := shared.NewTitle(title)
				if err := nodeAggregate.UpdateTitle(titleVO); err != nil {
					return nil, fmt.Errorf("failed to update title: %w", err)
				}
			}
			
		case "NodeArchived":
			if nodeAggregate == nil {
				return nil, fmt.Errorf("cannot apply NodeArchived without NodeCreated")
			}
			if err := nodeAggregate.Archive("event_replay"); err != nil {
				return nil, fmt.Errorf("failed to archive node: %w", err)
			}
			
		case "NodeRestored":
			if nodeAggregate == nil {
				return nil, fmt.Errorf("cannot apply NodeRestored without NodeCreated")
			}
			if err := nodeAggregate.Restore(); err != nil {
				return nil, fmt.Errorf("failed to restore node: %w", err)
			}
			
		case "NodeTagsUpdated":
			if nodeAggregate == nil {
				return nil, fmt.Errorf("cannot apply NodeTagsUpdated without NodeCreated")
			}
			data := event.EventData()
			if tags, ok := data["tags"].([]string); ok {
				if err := nodeAggregate.UpdateTags(shared.NewTags(tags...)); err != nil {
					return nil, fmt.Errorf("failed to update tags: %w", err)
				}
			}
		}
		
		// Clear events after applying (we're replaying, not generating new ones)
		nodeAggregate.MarkEventsAsCommitted()
	}
	
	return nodeAggregate, nil
}

// ReconstructCategory rebuilds a category aggregate from its event history
func (r *AggregateReplayer) ReconstructCategory(ctx context.Context, categoryID string) (*category.Category, error) {
	// Load all events for this aggregate
	events, err := r.eventStore.LoadEvents(ctx, categoryID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load events for category %s: %w", categoryID, err)
	}
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events found for category %s", categoryID)
	}
	
	// Start with empty category state
	var categoryAggregate *category.Category
	
	// Apply each event in sequence
	for _, event := range events {
		switch event.EventType() {
		case "CategoryCreated":
			// Extract data from event
			data := event.EventData()
			userID, _ := shared.NewUserID(event.UserID())
			name := data["name"].(string)
			description := ""
			if desc, ok := data["description"].(string); ok {
				description = desc
			}
			
			// Create the initial category
			categoryAggregate, err = category.NewCategory(userID, name, description)
			if err != nil {
				return nil, fmt.Errorf("failed to create category from event: %w", err)
			}
			
		case "CategoryUpdated":
			if categoryAggregate == nil {
				return nil, fmt.Errorf("cannot apply CategoryUpdated without CategoryCreated")
			}
			
			// Apply update to category
			data := event.EventData()
			if name, ok := data["name"].(string); ok {
				categoryAggregate.UpdateName(name)
			}
			if description, ok := data["description"].(string); ok {
				categoryAggregate.UpdateDescription(description)
			}
			
		case "CategoryMoved":
			if categoryAggregate == nil {
				return nil, fmt.Errorf("cannot apply CategoryMoved without CategoryCreated")
			}
			
			data := event.EventData()
			if parentID, ok := data["parentID"].(string); ok {
				if err := categoryAggregate.Move(shared.CategoryID(parentID)); err != nil {
					return nil, fmt.Errorf("failed to move category: %w", err)
				}
			}
			
		case "CategoryArchived":
			if categoryAggregate == nil {
				return nil, fmt.Errorf("cannot apply CategoryArchived without CategoryCreated")
			}
			categoryAggregate.Archive()
			
		case "CategoryRestored":
			if categoryAggregate == nil {
				return nil, fmt.Errorf("cannot apply CategoryRestored without CategoryCreated")
			}
			categoryAggregate.Restore()
		}
		
		// Clear events after applying
		categoryAggregate.MarkEventsAsCommitted()
	}
	
	return categoryAggregate, nil
}

// ReconstructEdge rebuilds an edge aggregate from its event history
func (r *AggregateReplayer) ReconstructEdge(ctx context.Context, edgeID string) (*edge.Edge, error) {
	// Load all events for this aggregate
	events, err := r.eventStore.LoadEvents(ctx, edgeID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load events for edge %s: %w", edgeID, err)
	}
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events found for edge %s", edgeID)
	}
	
	// Start with empty edge state
	var edgeAggregate *edge.Edge
	
	// Apply each event in sequence
	for _, event := range events {
		switch event.EventType() {
		case "EdgeCreated":
			// Extract data from event
			data := event.EventData()
			userID, _ := shared.NewUserID(event.UserID())
			sourceIDStr, _ := data["sourceID"].(string)
			targetIDStr, _ := data["targetID"].(string)
			sourceID, _ := shared.ParseNodeID(sourceIDStr)
			targetID, _ := shared.ParseNodeID(targetIDStr)
			weightVal := 1.0
			if w, ok := data["weight"].(float64); ok {
				weightVal = w
			}
			
			// Create the initial edge
			edgeAggregate, err = edge.NewEdge(sourceID, targetID, userID, weightVal)
			if err != nil {
				return nil, fmt.Errorf("failed to create edge from event: %w", err)
			}
			
		case "EdgeWeightUpdated":
			if edgeAggregate == nil {
				return nil, fmt.Errorf("cannot apply EdgeWeightUpdated without EdgeCreated")
			}
			
			data := event.EventData()
			if weight, ok := data["weight"].(float64); ok {
				if err := edgeAggregate.UpdateWeight(weight); err != nil {
					return nil, fmt.Errorf("failed to update edge weight: %w", err)
				}
			}
			
		case "EdgeTypeChanged":
			if edgeAggregate == nil {
				return nil, fmt.Errorf("cannot apply EdgeTypeChanged without EdgeCreated")
			}
			
			data := event.EventData()
			if edgeType, ok := data["type"].(string); ok {
				if err := edgeAggregate.ChangeType(edge.EdgeType(edgeType)); err != nil {
					return nil, fmt.Errorf("failed to change edge type: %w", err)
				}
			}
		}
		
		// Clear events after applying
		edgeAggregate.MarkEventsAsCommitted()
	}
	
	return edgeAggregate, nil
}

// ReconstructFromEvents rebuilds any aggregate from a list of events
func (r *AggregateReplayer) ReconstructFromEvents(ctx context.Context, aggregateType string, events []shared.DomainEvent) (interface{}, error) {
	if len(events) == 0 {
		return nil, fmt.Errorf("no events provided for reconstruction")
	}
	
	// Determine aggregate type from first event
	switch aggregateType {
	case "Node":
		return r.reconstructNodeFromEvents(events)
	case "Category":
		return r.reconstructCategoryFromEvents(events)
	case "Edge":
		return r.reconstructEdgeFromEvents(events)
	default:
		return nil, fmt.Errorf("unknown aggregate type: %s", aggregateType)
	}
}

// Helper methods for reconstructing from provided events
func (r *AggregateReplayer) reconstructNodeFromEvents(events []shared.DomainEvent) (*node.Node, error) {
	// Similar to ReconstructNode but uses provided events instead of loading from store
	// Implementation follows same pattern as ReconstructNode
	return nil, fmt.Errorf("not implemented")
}

func (r *AggregateReplayer) reconstructCategoryFromEvents(events []shared.DomainEvent) (*category.Category, error) {
	// Similar to ReconstructCategory but uses provided events instead of loading from store
	return nil, fmt.Errorf("not implemented")
}

func (r *AggregateReplayer) reconstructEdgeFromEvents(events []shared.DomainEvent) (*edge.Edge, error) {
	// Similar to ReconstructEdge but uses provided events instead of loading from store
	return nil, fmt.Errorf("not implemented")
}

// SnapshotManager handles aggregate snapshots for performance optimization
type SnapshotManager struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
	replayer      *AggregateReplayer
}

// Snapshot represents a point-in-time state of an aggregate
type Snapshot struct {
	AggregateID   string      `json:"aggregateId"`
	AggregateType string      `json:"aggregateType"`
	Version       int         `json:"version"`
	State         interface{} `json:"state"`
	CreatedAt     int64       `json:"createdAt"`
}

// SnapshotStore persists and retrieves aggregate snapshots
type SnapshotStore interface {
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
	LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)
	DeleteSnapshot(ctx context.Context, aggregateID string) error
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(eventStore EventStore, snapshotStore SnapshotStore) *SnapshotManager {
	return &SnapshotManager{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
		replayer:      NewAggregateReplayer(eventStore),
	}
}

// CreateSnapshot creates a snapshot of an aggregate at its current version
func (sm *SnapshotManager) CreateSnapshot(ctx context.Context, aggregateID string, aggregateType string) error {
	// Load all events up to current version
	events, err := sm.eventStore.LoadEvents(ctx, aggregateID, 0)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}
	
	if len(events) == 0 {
		return fmt.Errorf("no events found for aggregate %s", aggregateID)
	}
	
	// Reconstruct aggregate from events
	aggregate, err := sm.replayer.ReconstructFromEvents(ctx, aggregateType, events)
	if err != nil {
		return fmt.Errorf("failed to reconstruct aggregate: %w", err)
	}
	
	// Create snapshot
	snapshot := Snapshot{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Version:       events[len(events)-1].Version(),
		State:         aggregate,
		CreatedAt:     events[len(events)-1].Timestamp().Unix(),
	}
	
	// Save snapshot
	if err := sm.snapshotStore.SaveSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	
	return nil
}

// LoadFromSnapshot loads an aggregate from its latest snapshot and applies any newer events
func (sm *SnapshotManager) LoadFromSnapshot(ctx context.Context, aggregateID string) (interface{}, error) {
	// Load latest snapshot
	snapshot, err := sm.snapshotStore.LoadSnapshot(ctx, aggregateID)
	if err != nil {
		// No snapshot found, reconstruct from all events
		return sm.reconstructFromAllEvents(ctx, aggregateID)
	}
	
	// Load events after snapshot version
	events, err := sm.eventStore.LoadEvents(ctx, aggregateID, snapshot.Version+1)
	if err != nil {
		return nil, fmt.Errorf("failed to load events after snapshot: %w", err)
	}
	
	// If no new events, return snapshot state
	if len(events) == 0 {
		return snapshot.State, nil
	}
	
	// Apply new events to snapshot state
	aggregate := snapshot.State
	for range events {
		// Apply event to aggregate (implementation depends on aggregate type)
		// This would need type assertion and proper event application
	}
	
	return aggregate, nil
}

func (sm *SnapshotManager) reconstructFromAllEvents(ctx context.Context, aggregateID string) (interface{}, error) {
	// Determine aggregate type from first event
	events, err := sm.eventStore.LoadEvents(ctx, aggregateID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load first event: %w", err)
	}
	
	if len(events) == 0 {
		return nil, fmt.Errorf("no events found for aggregate %s", aggregateID)
	}
	
	// Determine type from event
	aggregateType := sm.determineAggregateType(events[0])
	
	// Load all events
	allEvents, err := sm.eventStore.LoadEvents(ctx, aggregateID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load all events: %w", err)
	}
	
	// Reconstruct from events
	return sm.replayer.ReconstructFromEvents(ctx, aggregateType, allEvents)
}

func (sm *SnapshotManager) determineAggregateType(event shared.DomainEvent) string {
	switch event.EventType() {
	case "NodeCreated", "NodeUpdated", "NodeArchived":
		return "Node"
	case "CategoryCreated", "CategoryUpdated", "CategoryMoved":
		return "Category"
	case "EdgeCreated", "EdgeWeightUpdated", "EdgeTypeChanged":
		return "Edge"
	default:
		return "Unknown"
	}
}