// Package projections contains read model projections that update denormalized views from events
package projections

import (
	"context"
	"encoding/json"
	"fmt"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// NodeProjection maintains the denormalized read model for nodes
type NodeProjection struct {
	store      ProjectionStore
	logger     ports.Logger
	metrics    ports.Metrics
	checkpoint int64
}

// ProjectionStore interface for storing projections
type ProjectionStore interface {
	// SaveNodeView saves or updates a node view
	SaveNodeView(ctx context.Context, view NodeView) error
	
	// GetNodeView retrieves a node view
	GetNodeView(ctx context.Context, nodeID string) (*NodeView, error)
	
	// DeleteNodeView deletes a node view
	DeleteNodeView(ctx context.Context, nodeID string) error
	
	// UpdateConnectionCount updates the connection count
	UpdateConnectionCount(ctx context.Context, nodeID string, delta int) error
	
	// AddCategory adds a category to a node
	AddCategory(ctx context.Context, nodeID, categoryID string) error
	
	// RemoveCategory removes a category from a node
	RemoveCategory(ctx context.Context, nodeID, categoryID string) error
	
	// GetCheckpoint gets the last processed event position
	GetCheckpoint(ctx context.Context, projectionName string) (int64, error)
	
	// SaveCheckpoint saves the processing checkpoint
	SaveCheckpoint(ctx context.Context, projectionName string, position int64) error
}

// NodeView is the denormalized read model for nodes
type NodeView struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	Content         string                 `json:"content"`
	Title           string                 `json:"title"`
	Keywords        []string               `json:"keywords"`
	Tags            []string               `json:"tags"`
	Categories      []string               `json:"categories"`
	ConnectionCount int                    `json:"connection_count"`
	IsArchived      bool                   `json:"is_archived"`
	CreatedAt       int64                  `json:"created_at"`
	UpdatedAt       int64                  `json:"updated_at"`
	Version         int64                  `json:"version"`
	Metadata        map[string]interface{} `json:"metadata"`
	
	// Denormalized fields for query optimization
	CategoryNames   []string `json:"category_names"`
	ConnectedNodeIDs []string `json:"connected_node_ids"`
	LastActivityAt   int64    `json:"last_activity_at"`
	SearchText       string   `json:"search_text"` // Concatenated searchable text
}

// NewNodeProjection creates a new node projection
func NewNodeProjection(store ProjectionStore, logger ports.Logger, metrics ports.Metrics) *NodeProjection {
	return &NodeProjection{
		store:   store,
		logger:  logger,
		metrics: metrics,
	}
}

// Handle processes an event to update the projection
func (p *NodeProjection) Handle(ctx context.Context, event events.DomainEvent) error {
	switch event.GetEventType() {
	case "NodeCreated":
		return p.handleNodeCreated(ctx, event)
	case "NodeUpdated":
		return p.handleNodeUpdated(ctx, event)
	case "NodeArchived":
		return p.handleNodeArchived(ctx, event)
	case "NodeRestored":
		return p.handleNodeRestored(ctx, event)
	case "NodeTagged":
		return p.handleNodeTagged(ctx, event)
	case "NodeCategorized":
		return p.handleNodeCategorized(ctx, event)
	case "NodeConnected":
		return p.handleNodeConnected(ctx, event)
	case "NodeDisconnected":
		return p.handleNodeDisconnected(ctx, event)
	default:
		// Unknown event type, log and continue
		p.logger.Debug("Unknown event type for projection",
			ports.Field{Key: "event_type", Value: event.GetEventType()})
		return nil
	}
}

// handleNodeCreated handles NodeCreated events
func (p *NodeProjection) handleNodeCreated(ctx context.Context, event events.DomainEvent) error {
	// Parse event data
	var data struct {
		UserID   string   `json:"user_id"`
		Content  string   `json:"content"`
		Title    string   `json:"title"`
		Keywords []string `json:"keywords"`
		Tags     []string `json:"tags"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Create node view
	view := NodeView{
		ID:              event.GetAggregateID(),
		UserID:          data.UserID,
		Content:         data.Content,
		Title:           data.Title,
		Keywords:        data.Keywords,
		Tags:            data.Tags,
		Categories:      []string{},
		ConnectionCount: 0,
		IsArchived:      false,
		CreatedAt:       event.GetTimestamp().Unix(),
		UpdatedAt:       event.GetTimestamp().Unix(),
		Version:         event.GetVersion(),
		Metadata:        make(map[string]interface{}),
		
		// Denormalized fields
		CategoryNames:    []string{},
		ConnectedNodeIDs: []string{},
		LastActivityAt:   event.GetTimestamp().Unix(),
		SearchText:       p.buildSearchText(data.Title, data.Content, data.Tags, data.Keywords),
	}
	
	// Save to store
	if err := p.store.SaveNodeView(ctx, view); err != nil {
		p.metrics.IncrementCounter("projection.node.save_failed")
		return fmt.Errorf("failed to save node view: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.created")
	return nil
}

// handleNodeUpdated handles NodeUpdated events
func (p *NodeProjection) handleNodeUpdated(ctx context.Context, event events.DomainEvent) error {
	// Get existing view
	view, err := p.store.GetNodeView(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get node view: %w", err)
	}
	
	// Parse event data
	var data struct {
		NewContent string `json:"new_content"`
		NewTitle   string `json:"new_title"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update view
	view.Content = data.NewContent
	view.Title = data.NewTitle
	view.UpdatedAt = event.GetTimestamp().Unix()
	view.Version = event.GetVersion()
	view.LastActivityAt = event.GetTimestamp().Unix()
	view.SearchText = p.buildSearchText(data.NewTitle, data.NewContent, view.Tags, view.Keywords)
	
	// Save updated view
	if err := p.store.SaveNodeView(ctx, *view); err != nil {
		p.metrics.IncrementCounter("projection.node.update_failed")
		return fmt.Errorf("failed to update node view: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.updated")
	return nil
}

// handleNodeArchived handles NodeArchived events
func (p *NodeProjection) handleNodeArchived(ctx context.Context, event events.DomainEvent) error {
	// Get existing view
	view, err := p.store.GetNodeView(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get node view: %w", err)
	}
	
	// Update view
	view.IsArchived = true
	view.UpdatedAt = event.GetTimestamp().Unix()
	view.Version = event.GetVersion()
	
	// Save updated view
	if err := p.store.SaveNodeView(ctx, *view); err != nil {
		p.metrics.IncrementCounter("projection.node.archive_failed")
		return fmt.Errorf("failed to archive node view: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.archived")
	return nil
}

// handleNodeRestored handles NodeRestored events
func (p *NodeProjection) handleNodeRestored(ctx context.Context, event events.DomainEvent) error {
	// Get existing view
	view, err := p.store.GetNodeView(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get node view: %w", err)
	}
	
	// Update view
	view.IsArchived = false
	view.UpdatedAt = event.GetTimestamp().Unix()
	view.Version = event.GetVersion()
	view.LastActivityAt = event.GetTimestamp().Unix()
	
	// Save updated view
	if err := p.store.SaveNodeView(ctx, *view); err != nil {
		p.metrics.IncrementCounter("projection.node.restore_failed")
		return fmt.Errorf("failed to restore node view: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.restored")
	return nil
}

// handleNodeTagged handles NodeTagged events
func (p *NodeProjection) handleNodeTagged(ctx context.Context, event events.DomainEvent) error {
	// Get existing view
	view, err := p.store.GetNodeView(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get node view: %w", err)
	}
	
	// Parse event data
	var data struct {
		Tags []string `json:"tags"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Add new tags
	tagMap := make(map[string]bool)
	for _, tag := range view.Tags {
		tagMap[tag] = true
	}
	for _, tag := range data.Tags {
		tagMap[tag] = true
	}
	
	view.Tags = make([]string, 0, len(tagMap))
	for tag := range tagMap {
		view.Tags = append(view.Tags, tag)
	}
	
	view.UpdatedAt = event.GetTimestamp().Unix()
	view.Version = event.GetVersion()
	view.SearchText = p.buildSearchText(view.Title, view.Content, view.Tags, view.Keywords)
	
	// Save updated view
	if err := p.store.SaveNodeView(ctx, *view); err != nil {
		p.metrics.IncrementCounter("projection.node.tag_failed")
		return fmt.Errorf("failed to update tags: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.tagged")
	return nil
}

// handleNodeCategorized handles NodeCategorized events
func (p *NodeProjection) handleNodeCategorized(ctx context.Context, event events.DomainEvent) error {
	// Parse event data
	var data struct {
		CategoryID string `json:"category_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update category association
	if err := p.store.AddCategory(ctx, event.GetAggregateID(), data.CategoryID); err != nil {
		p.metrics.IncrementCounter("projection.node.categorize_failed")
		return fmt.Errorf("failed to add category: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.node.categorized")
	return nil
}

// handleNodeConnected handles NodeConnected events
func (p *NodeProjection) handleNodeConnected(ctx context.Context, event events.DomainEvent) error {
	// Parse event data
	var data struct {
		TargetNodeID string  `json:"target_node_id"`
		Strength     float64 `json:"strength"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update connection count
	if err := p.store.UpdateConnectionCount(ctx, event.GetAggregateID(), 1); err != nil {
		p.metrics.IncrementCounter("projection.node.connect_failed")
		return fmt.Errorf("failed to update connection count: %w", err)
	}
	
	// Also update target node's connection count
	if err := p.store.UpdateConnectionCount(ctx, data.TargetNodeID, 1); err != nil {
		p.logger.Warn("Failed to update target node connection count",
			ports.Field{Key: "target_id", Value: data.TargetNodeID})
	}
	
	p.metrics.IncrementCounter("projection.node.connected")
	return nil
}

// handleNodeDisconnected handles NodeDisconnected events
func (p *NodeProjection) handleNodeDisconnected(ctx context.Context, event events.DomainEvent) error {
	// Parse event data
	var data struct {
		TargetNodeID string `json:"target_node_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update connection count
	if err := p.store.UpdateConnectionCount(ctx, event.GetAggregateID(), -1); err != nil {
		p.metrics.IncrementCounter("projection.node.disconnect_failed")
		return fmt.Errorf("failed to update connection count: %w", err)
	}
	
	// Also update target node's connection count
	if err := p.store.UpdateConnectionCount(ctx, data.TargetNodeID, -1); err != nil {
		p.logger.Warn("Failed to update target node connection count",
			ports.Field{Key: "target_id", Value: data.TargetNodeID})
	}
	
	p.metrics.IncrementCounter("projection.node.disconnected")
	return nil
}

// GetProjectionName returns the name of this projection
func (p *NodeProjection) GetProjectionName() string {
	return "NodeProjection"
}

// Reset clears and rebuilds the projection from events
func (p *NodeProjection) Reset(ctx context.Context) error {
	// This would clear the projection store and replay all events
	// Implementation depends on the event store
	return fmt.Errorf("not implemented")
}

// GetCheckpoint returns the last processed event position
func (p *NodeProjection) GetCheckpoint(ctx context.Context) (int64, error) {
	return p.store.GetCheckpoint(ctx, p.GetProjectionName())
}

// SaveCheckpoint saves the processing checkpoint
func (p *NodeProjection) SaveCheckpoint(ctx context.Context, position int64) error {
	p.checkpoint = position
	return p.store.SaveCheckpoint(ctx, p.GetProjectionName(), position)
}

// parseEventData parses event data into the target structure
func (p *NodeProjection) parseEventData(event events.DomainEvent, target interface{}) error {
	// In a real implementation, this would properly deserialize the event
	// For now, we'll use a placeholder
	data, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	
	return nil
}

// buildSearchText builds concatenated searchable text
func (p *NodeProjection) buildSearchText(title, content string, tags, keywords []string) string {
	searchText := title + " " + content
	for _, tag := range tags {
		searchText += " " + tag
	}
	for _, keyword := range keywords {
		searchText += " " + keyword
	}
	return searchText
}