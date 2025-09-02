// Package projections contains additional read model projections for specific views
package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// GraphProjection maintains a denormalized graph structure for efficient traversal
type GraphProjection struct {
	store      ProjectionStore
	graphStore GraphStore
	logger     ports.Logger
	metrics    ports.Metrics
	checkpoint int64
	mutex      sync.RWMutex
}

// GraphStore is a specialized store for graph operations
type GraphStore interface {
	// Node operations
	AddNode(ctx context.Context, node GraphNode) error
	UpdateNode(ctx context.Context, nodeID string, updates map[string]interface{}) error
	GetNode(ctx context.Context, nodeID string) (*GraphNode, error)
	RemoveNode(ctx context.Context, nodeID string) error
	
	// Edge operations
	AddEdge(ctx context.Context, edge GraphEdge) error
	UpdateEdge(ctx context.Context, edgeID string, updates map[string]interface{}) error
	GetEdge(ctx context.Context, sourceID, targetID string) (*GraphEdge, error)
	RemoveEdge(ctx context.Context, sourceID, targetID string) error
	
	// Graph queries
	GetNeighbors(ctx context.Context, nodeID string, depth int) ([]GraphNode, error)
	GetShortestPath(ctx context.Context, sourceID, targetID string) ([]string, error)
	GetConnectedComponents(ctx context.Context, userID string) ([][]string, error)
	GetCentralityScores(ctx context.Context, userID string) (map[string]float64, error)
	GetClusteringCoefficient(ctx context.Context, nodeID string) (float64, error)
	
	// Batch operations
	BatchAddNodes(ctx context.Context, nodes []GraphNode) error
	BatchAddEdges(ctx context.Context, edges []GraphEdge) error
}

// GraphNode represents a node in the graph projection
type GraphNode struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	Title           string                 `json:"title"`
	NodeType        string                 `json:"node_type"`
	InDegree        int                    `json:"in_degree"`
	OutDegree       int                    `json:"out_degree"`
	Centrality      float64                `json:"centrality"`
	ClusteringCoeff float64                `json:"clustering_coeff"`
	ComponentID     string                 `json:"component_id"`
	IsHub           bool                   `json:"is_hub"`
	IsArchived      bool                   `json:"is_archived"`
	CreatedAt       int64                  `json:"created_at"`
	UpdatedAt       int64                  `json:"updated_at"`
	Metadata        map[string]interface{} `json:"metadata"`
	
	// Cached neighbor lists for fast traversal
	IncomingEdges  []string `json:"incoming_edges"`
	OutgoingEdges  []string `json:"outgoing_edges"`
}

// GraphEdge represents an edge in the graph projection
type GraphEdge struct {
	ID         string                 `json:"id"`
	SourceID   string                 `json:"source_id"`
	TargetID   string                 `json:"target_id"`
	EdgeType   string                 `json:"edge_type"`
	Weight     float64                `json:"weight"`
	UserID     string                 `json:"user_id"`
	CreatedAt  int64                  `json:"created_at"`
	UpdatedAt  int64                  `json:"updated_at"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// NewGraphProjection creates a new graph projection
func NewGraphProjection(
	store ProjectionStore,
	graphStore GraphStore,
	logger ports.Logger,
	metrics ports.Metrics,
) *GraphProjection {
	return &GraphProjection{
		store:      store,
		graphStore: graphStore,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes an event to update the graph projection
func (p *GraphProjection) Handle(ctx context.Context, event events.DomainEvent) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	switch event.GetEventType() {
	case "NodeCreated":
		return p.handleNodeCreated(ctx, event)
	case "NodeUpdated":
		return p.handleNodeUpdated(ctx, event)
	case "NodeArchived":
		return p.handleNodeArchived(ctx, event)
	case "NodeRestored":
		return p.handleNodeRestored(ctx, event)
	case "NodeConnected":
		return p.handleNodeConnected(ctx, event)
	case "NodeDisconnected":
		return p.handleNodeDisconnected(ctx, event)
	case "EdgeCreated":
		return p.handleEdgeCreated(ctx, event)
	case "EdgeUpdated":
		return p.handleEdgeUpdated(ctx, event)
	case "EdgeDeleted":
		return p.handleEdgeDeleted(ctx, event)
	default:
		// Unknown event type, log and continue
		p.logger.Debug("Unknown event type for graph projection",
			ports.Field{Key: "event_type", Value: event.GetEventType()})
		return nil
	}
}

// handleNodeCreated handles NodeCreated events
func (p *GraphProjection) handleNodeCreated(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID   string   `json:"user_id"`
		Title    string   `json:"title"`
		NodeType string   `json:"node_type"`
		Tags     []string `json:"tags"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	node := GraphNode{
		ID:              event.GetAggregateID(),
		UserID:          data.UserID,
		Title:           data.Title,
		NodeType:        data.NodeType,
		InDegree:        0,
		OutDegree:       0,
		Centrality:      0.0,
		ClusteringCoeff: 0.0,
		ComponentID:     event.GetAggregateID(), // Initially its own component
		IsHub:           false,
		IsArchived:      false,
		CreatedAt:       event.GetTimestamp().Unix(),
		UpdatedAt:       event.GetTimestamp().Unix(),
		Metadata: map[string]interface{}{
			"tags": data.Tags,
		},
		IncomingEdges: []string{},
		OutgoingEdges: []string{},
	}
	
	if err := p.graphStore.AddNode(ctx, node); err != nil {
		p.metrics.IncrementCounter("projection.graph.node_add_failed")
		return fmt.Errorf("failed to add node to graph: %w", err)
	}
	
	// Update component analysis
	go p.updateComponents(context.Background(), data.UserID)
	
	p.metrics.IncrementCounter("projection.graph.node_created")
	return nil
}

// handleNodeUpdated handles NodeUpdated events
func (p *GraphProjection) handleNodeUpdated(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		NewTitle string `json:"new_title"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	updates := map[string]interface{}{
		"title":      data.NewTitle,
		"updated_at": event.GetTimestamp().Unix(),
	}
	
	if err := p.graphStore.UpdateNode(ctx, event.GetAggregateID(), updates); err != nil {
		p.metrics.IncrementCounter("projection.graph.node_update_failed")
		return fmt.Errorf("failed to update node in graph: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.graph.node_updated")
	return nil
}

// handleNodeArchived handles NodeArchived events
func (p *GraphProjection) handleNodeArchived(ctx context.Context, event events.DomainEvent) error {
	updates := map[string]interface{}{
		"is_archived": true,
		"updated_at":  event.GetTimestamp().Unix(),
	}
	
	if err := p.graphStore.UpdateNode(ctx, event.GetAggregateID(), updates); err != nil {
		p.metrics.IncrementCounter("projection.graph.node_archive_failed")
		return fmt.Errorf("failed to archive node in graph: %w", err)
	}
	
	// Update connected nodes' metrics
	node, err := p.graphStore.GetNode(ctx, event.GetAggregateID())
	if err == nil {
		go p.updateNeighborMetrics(context.Background(), node)
	}
	
	p.metrics.IncrementCounter("projection.graph.node_archived")
	return nil
}

// handleNodeRestored handles NodeRestored events
func (p *GraphProjection) handleNodeRestored(ctx context.Context, event events.DomainEvent) error {
	updates := map[string]interface{}{
		"is_archived": false,
		"updated_at":  event.GetTimestamp().Unix(),
	}
	
	if err := p.graphStore.UpdateNode(ctx, event.GetAggregateID(), updates); err != nil {
		p.metrics.IncrementCounter("projection.graph.node_restore_failed")
		return fmt.Errorf("failed to restore node in graph: %w", err)
	}
	
	// Update connected nodes' metrics
	node, err := p.graphStore.GetNode(ctx, event.GetAggregateID())
	if err == nil {
		go p.updateNeighborMetrics(context.Background(), node)
	}
	
	p.metrics.IncrementCounter("projection.graph.node_restored")
	return nil
}

// handleNodeConnected handles NodeConnected events
func (p *GraphProjection) handleNodeConnected(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		TargetNodeID string  `json:"target_node_id"`
		EdgeType     string  `json:"edge_type"`
		Weight       float64 `json:"weight"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update source node
	sourceNode, err := p.graphStore.GetNode(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get source node: %w", err)
	}
	
	sourceNode.OutDegree++
	sourceNode.OutgoingEdges = append(sourceNode.OutgoingEdges, data.TargetNodeID)
	
	if err := p.graphStore.UpdateNode(ctx, event.GetAggregateID(), map[string]interface{}{
		"out_degree":     sourceNode.OutDegree,
		"outgoing_edges": sourceNode.OutgoingEdges,
		"updated_at":     event.GetTimestamp().Unix(),
	}); err != nil {
		return err
	}
	
	// Update target node
	targetNode, err := p.graphStore.GetNode(ctx, data.TargetNodeID)
	if err != nil {
		return fmt.Errorf("failed to get target node: %w", err)
	}
	
	targetNode.InDegree++
	targetNode.IncomingEdges = append(targetNode.IncomingEdges, event.GetAggregateID())
	
	if err := p.graphStore.UpdateNode(ctx, data.TargetNodeID, map[string]interface{}{
		"in_degree":      targetNode.InDegree,
		"incoming_edges": targetNode.IncomingEdges,
		"updated_at":     event.GetTimestamp().Unix(),
	}); err != nil {
		return err
	}
	
	// Update graph metrics asynchronously
	go p.updateGraphMetrics(context.Background(), sourceNode.UserID, []string{
		event.GetAggregateID(),
		data.TargetNodeID,
	})
	
	p.metrics.IncrementCounter("projection.graph.node_connected")
	return nil
}

// handleNodeDisconnected handles NodeDisconnected events
func (p *GraphProjection) handleNodeDisconnected(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		TargetNodeID string `json:"target_node_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update source node
	sourceNode, err := p.graphStore.GetNode(ctx, event.GetAggregateID())
	if err != nil {
		return fmt.Errorf("failed to get source node: %w", err)
	}
	
	sourceNode.OutDegree--
	sourceNode.OutgoingEdges = removeFromSlice(sourceNode.OutgoingEdges, data.TargetNodeID)
	
	if err := p.graphStore.UpdateNode(ctx, event.GetAggregateID(), map[string]interface{}{
		"out_degree":     sourceNode.OutDegree,
		"outgoing_edges": sourceNode.OutgoingEdges,
		"updated_at":     event.GetTimestamp().Unix(),
	}); err != nil {
		return err
	}
	
	// Update target node
	targetNode, err := p.graphStore.GetNode(ctx, data.TargetNodeID)
	if err != nil {
		return fmt.Errorf("failed to get target node: %w", err)
	}
	
	targetNode.InDegree--
	targetNode.IncomingEdges = removeFromSlice(targetNode.IncomingEdges, event.GetAggregateID())
	
	if err := p.graphStore.UpdateNode(ctx, data.TargetNodeID, map[string]interface{}{
		"in_degree":      targetNode.InDegree,
		"incoming_edges": targetNode.IncomingEdges,
		"updated_at":     event.GetTimestamp().Unix(),
	}); err != nil {
		return err
	}
	
	// Update graph metrics asynchronously
	go p.updateGraphMetrics(context.Background(), sourceNode.UserID, []string{
		event.GetAggregateID(),
		data.TargetNodeID,
	})
	
	p.metrics.IncrementCounter("projection.graph.node_disconnected")
	return nil
}

// handleEdgeCreated handles EdgeCreated events
func (p *GraphProjection) handleEdgeCreated(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		SourceID string  `json:"source_id"`
		TargetID string  `json:"target_id"`
		EdgeType string  `json:"edge_type"`
		Weight   float64 `json:"weight"`
		UserID   string  `json:"user_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	edge := GraphEdge{
		ID:        event.GetAggregateID(),
		SourceID:  data.SourceID,
		TargetID:  data.TargetID,
		EdgeType:  data.EdgeType,
		Weight:    data.Weight,
		UserID:    data.UserID,
		CreatedAt: event.GetTimestamp().Unix(),
		UpdatedAt: event.GetTimestamp().Unix(),
		Metadata:  make(map[string]interface{}),
	}
	
	if err := p.graphStore.AddEdge(ctx, edge); err != nil {
		p.metrics.IncrementCounter("projection.graph.edge_add_failed")
		return fmt.Errorf("failed to add edge to graph: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.graph.edge_created")
	return nil
}

// handleEdgeUpdated handles EdgeUpdated events
func (p *GraphProjection) handleEdgeUpdated(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		Weight float64 `json:"weight"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	updates := map[string]interface{}{
		"weight":     data.Weight,
		"updated_at": event.GetTimestamp().Unix(),
	}
	
	if err := p.graphStore.UpdateEdge(ctx, event.GetAggregateID(), updates); err != nil {
		p.metrics.IncrementCounter("projection.graph.edge_update_failed")
		return fmt.Errorf("failed to update edge in graph: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.graph.edge_updated")
	return nil
}

// handleEdgeDeleted handles EdgeDeleted events
func (p *GraphProjection) handleEdgeDeleted(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		SourceID string `json:"source_id"`
		TargetID string `json:"target_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	if err := p.graphStore.RemoveEdge(ctx, data.SourceID, data.TargetID); err != nil {
		p.metrics.IncrementCounter("projection.graph.edge_delete_failed")
		return fmt.Errorf("failed to remove edge from graph: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.graph.edge_deleted")
	return nil
}

// updateComponents updates connected component analysis
func (p *GraphProjection) updateComponents(ctx context.Context, userID string) {
	components, err := p.graphStore.GetConnectedComponents(ctx, userID)
	if err != nil {
		p.logger.Error("Failed to get connected components",
			err,
			ports.Field{Key: "user_id", Value: userID})
		return
	}
	
	// Update component IDs for all nodes
	for i, component := range components {
		componentID := fmt.Sprintf("component-%d", i)
		for _, nodeID := range component {
			if err := p.graphStore.UpdateNode(ctx, nodeID, map[string]interface{}{
				"component_id": componentID,
			}); err != nil {
				p.logger.Warn("Failed to update component ID",
					ports.Field{Key: "node_id", Value: nodeID},
					ports.Field{Key: "error", Value: err.Error()})
			}
		}
	}
	
	p.logger.Info("Connected components updated",
		ports.Field{Key: "user_id", Value: userID},
		ports.Field{Key: "components", Value: len(components)})
}

// updateGraphMetrics updates various graph metrics
func (p *GraphProjection) updateGraphMetrics(ctx context.Context, userID string, nodeIDs []string) {
	// Update centrality scores
	centrality, err := p.graphStore.GetCentralityScores(ctx, userID)
	if err != nil {
		p.logger.Error("Failed to calculate centrality",
			err,
			ports.Field{Key: "user_id", Value: userID})
		return
	}
	
	for nodeID, score := range centrality {
		// Mark nodes with high centrality as hubs
		isHub := score > 0.7
		
		if err := p.graphStore.UpdateNode(ctx, nodeID, map[string]interface{}{
			"centrality": score,
			"is_hub":     isHub,
		}); err != nil {
			p.logger.Warn("Failed to update centrality",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Update clustering coefficients for affected nodes
	for _, nodeID := range nodeIDs {
		coeff, err := p.graphStore.GetClusteringCoefficient(ctx, nodeID)
		if err != nil {
			p.logger.Warn("Failed to calculate clustering coefficient",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		
		if err := p.graphStore.UpdateNode(ctx, nodeID, map[string]interface{}{
			"clustering_coeff": coeff,
		}); err != nil {
			p.logger.Warn("Failed to update clustering coefficient",
				ports.Field{Key: "node_id", Value: nodeID},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	p.metrics.IncrementCounter("projection.graph.metrics_updated",
		ports.Tag{Key: "user_id", Value: userID})
}

// updateNeighborMetrics updates metrics for a node's neighbors
func (p *GraphProjection) updateNeighborMetrics(ctx context.Context, node *GraphNode) {
	allNeighbors := append(node.IncomingEdges, node.OutgoingEdges...)
	uniqueNeighbors := make(map[string]bool)
	
	for _, neighbor := range allNeighbors {
		uniqueNeighbors[neighbor] = true
	}
	
	neighbors := make([]string, 0, len(uniqueNeighbors))
	for neighbor := range uniqueNeighbors {
		neighbors = append(neighbors, neighbor)
	}
	
	if len(neighbors) > 0 {
		p.updateGraphMetrics(ctx, node.UserID, neighbors)
	}
}

// GetProjectionName returns the name of this projection
func (p *GraphProjection) GetProjectionName() string {
	return "GraphProjection"
}

// Reset clears and rebuilds the projection from events
func (p *GraphProjection) Reset(ctx context.Context) error {
	// This would clear the graph store and replay all events
	return fmt.Errorf("not implemented")
}

// GetCheckpoint returns the last processed event position
func (p *GraphProjection) GetCheckpoint(ctx context.Context) (int64, error) {
	return p.store.GetCheckpoint(ctx, p.GetProjectionName())
}

// SaveCheckpoint saves the processing checkpoint
func (p *GraphProjection) SaveCheckpoint(ctx context.Context, position int64) error {
	p.checkpoint = position
	return p.store.SaveCheckpoint(ctx, p.GetProjectionName(), position)
}

// parseEventData parses event data into the target structure
func (p *GraphProjection) parseEventData(event events.DomainEvent, target interface{}) error {
	data, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	
	return nil
}

// removeFromSlice removes an element from a string slice
func removeFromSlice(slice []string, element string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != element {
			result = append(result, item)
		}
	}
	return result
}