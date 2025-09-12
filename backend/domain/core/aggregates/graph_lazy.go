package aggregates

import (
	"context"
	"fmt"
	"time"

	"backend/domain/config"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	pkgerrors "backend/pkg/errors"
)

// GraphLazy is an optimized version of Graph that uses lazy loading
// It only loads nodes and edges when explicitly requested, not on aggregate load
type GraphLazy struct {
	// Core metadata - always loaded
	id          GraphID
	userID      string
	name        string
	description string
	metadata    GraphMetadata
	createdAt   time.Time
	updatedAt   time.Time
	version     int
	
	// Configuration
	config      *config.DomainConfig
	
	// Lazy-loaded data - only IDs are stored, not full objects
	nodeIDs     map[valueobjects.NodeID]bool  // Set of node IDs only
	edgeKeys    map[string]bool                // Set of edge keys only
	
	// Repository for lazy loading (injected)
	nodeLoader  NodeLoader
	edgeLoader  EdgeLoader
	
	// Events
	events      []events.DomainEvent
}

// NodeLoader interface for lazy loading nodes
type NodeLoader interface {
	LoadNode(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error)
	LoadNodes(ctx context.Context, nodeIDs []valueobjects.NodeID) ([]*entities.Node, error)
}

// EdgeLoader interface for lazy loading edges
type EdgeLoader interface {
	LoadEdge(ctx context.Context, edgeKey string) (*Edge, error)
	LoadEdges(ctx context.Context, edgeKeys []string) ([]*Edge, error)
	LoadEdgesByNodeID(ctx context.Context, nodeID valueobjects.NodeID) ([]*Edge, error)
}

// NewGraphLazy creates a new lazy-loading graph aggregate
func NewGraphLazy(userID, name string) (*GraphLazy, error) {
	return NewGraphLazyWithConfig(userID, name, config.DefaultDomainConfig())
}

// NewGraphLazyWithConfig creates a new lazy-loading graph with specific configuration
func NewGraphLazyWithConfig(userID, name string, cfg *config.DomainConfig) (*GraphLazy, error) {
	if userID == "" {
		return nil, pkgerrors.NewValidationError("userID is required")
	}

	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}

	if name == "" {
		name = cfg.DefaultGraphName
	}

	now := time.Now()
	graph := &GraphLazy{
		id:          NewGraphID(),
		userID:      userID,
		name:        name,
		description: "Knowledge graph for " + name,
		nodeIDs:     make(map[valueobjects.NodeID]bool),
		edgeKeys:    make(map[string]bool),
		config:      cfg,
		metadata: GraphMetadata{
			ViewSettings: ViewSettings{
				Layout:     LayoutForceDirected,
				ShowLabels: true,
			},
		},
		createdAt: now,
		updatedAt: now,
		version:   1,
		events:    []events.DomainEvent{},
	}

	graph.addEvent(events.GraphCreated{
		BaseEvent: events.BaseEvent{
			AggregateID: graph.id.String(),
			EventType:   "graph.created",
			Timestamp:   now,
			Version:     1,
		},
		GraphID: graph.id.String(),
		UserID:  userID,
		Name:    name,
	})

	return graph, nil
}

// SetLoaders injects the lazy loading dependencies
func (g *GraphLazy) SetLoaders(nodeLoader NodeLoader, edgeLoader EdgeLoader) {
	g.nodeLoader = nodeLoader
	g.edgeLoader = edgeLoader
}

// ID returns the graph's unique identifier
func (g *GraphLazy) ID() GraphID {
	return g.id
}

// UserID returns the owner's ID
func (g *GraphLazy) UserID() string {
	return g.userID
}

// Name returns the graph's name
func (g *GraphLazy) Name() string {
	return g.name
}

// Description returns the graph's description
func (g *GraphLazy) Description() string {
	return g.description
}

// NodeCount returns the number of nodes without loading them
func (g *GraphLazy) NodeCount() int {
	return len(g.nodeIDs)
}

// EdgeCount returns the number of edges without loading them
func (g *GraphLazy) EdgeCount() int {
	return len(g.edgeKeys)
}

// HasNode checks if a node exists without loading it
func (g *GraphLazy) HasNode(nodeID valueobjects.NodeID) bool {
	return g.nodeIDs[nodeID]
}

// AddNodeID registers a node ID without loading the full node
func (g *GraphLazy) AddNodeID(nodeID valueobjects.NodeID) error {
	if g.nodeIDs[nodeID] {
		return pkgerrors.NewConflictError("node already exists in graph")
	}

	// Check node limit
	if g.config != nil && len(g.nodeIDs) >= g.config.MaxNodesPerGraph {
		return fmt.Errorf("maximum nodes reached: %d", g.config.MaxNodesPerGraph)
	}

	g.nodeIDs[nodeID] = true
	g.metadata.NodeCount = len(g.nodeIDs)
	g.updatedAt = time.Now()
	g.version++

	g.addEvent(events.NodeAddedToGraph{
		BaseEvent: events.BaseEvent{
			AggregateID: g.id.String(),
			EventType:   "graph.node_added",
			Timestamp:   g.updatedAt,
			Version:     1,
		},
		GraphID: g.id.String(),
		NodeID:  nodeID,
	})

	return nil
}

// RemoveNodeID removes a node ID without loading the full node
func (g *GraphLazy) RemoveNodeID(nodeID valueobjects.NodeID) error {
	if !g.nodeIDs[nodeID] {
		return pkgerrors.NewNotFoundError("node")
	}

	// Remove node
	delete(g.nodeIDs, nodeID)
	g.metadata.NodeCount = len(g.nodeIDs)

	// Remove all edges connected to this node
	edgesToRemove := []string{}
	for key := range g.edgeKeys {
		// Parse edge key to check if it involves this node
		// Edge key format: "sourceID->targetID"
		if g.edgeInvolvesNode(key, nodeID) {
			edgesToRemove = append(edgesToRemove, key)
		}
	}

	for _, key := range edgesToRemove {
		delete(g.edgeKeys, key)
	}
	g.metadata.EdgeCount = len(g.edgeKeys)

	g.updatedAt = time.Now()
	g.version++

	g.addEvent(events.NodeRemovedFromGraph{
		BaseEvent: events.BaseEvent{
			AggregateID: g.id.String(),
			EventType:   "graph.node_removed",
			Timestamp:   g.updatedAt,
			Version:     1,
		},
		GraphID: g.id.String(),
		NodeID:  nodeID,
	})

	return nil
}

// AddEdgeKey registers an edge key without loading the full edge
func (g *GraphLazy) AddEdgeKey(sourceID, targetID valueobjects.NodeID) error {
	// Validate nodes exist
	if !g.nodeIDs[sourceID] || !g.nodeIDs[targetID] {
		return pkgerrors.NewValidationError("both nodes must exist in graph")
	}

	// Check for self-reference
	if sourceID.Equals(targetID) {
		return pkgerrors.NewValidationError("cannot connect node to itself")
	}

	edgeKey := g.makeEdgeKey(sourceID, targetID)
	if g.edgeKeys[edgeKey] {
		return pkgerrors.NewConflictError("edge already exists")
	}

	// Check edge limit
	if g.config != nil && len(g.edgeKeys) >= g.config.MaxEdgesPerGraph {
		return fmt.Errorf("maximum edges reached: %d", g.config.MaxEdgesPerGraph)
	}

	g.edgeKeys[edgeKey] = true
	g.metadata.EdgeCount = len(g.edgeKeys)
	g.updatedAt = time.Now()
	g.version++

	return nil
}

// GetNode loads a single node on-demand
func (g *GraphLazy) GetNode(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error) {
	if !g.nodeIDs[nodeID] {
		return nil, pkgerrors.NewNotFoundError("node not in graph")
	}

	if g.nodeLoader == nil {
		return nil, fmt.Errorf("node loader not configured")
	}

	return g.nodeLoader.LoadNode(ctx, nodeID)
}

// GetNodes loads multiple nodes with pagination
func (g *GraphLazy) GetNodesPaginated(ctx context.Context, limit int, offset int) ([]*entities.Node, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	// Collect node IDs for the requested page
	nodeIDList := make([]valueobjects.NodeID, 0, limit)
	currentIndex := 0
	
	for nodeID := range g.nodeIDs {
		if currentIndex >= offset && len(nodeIDList) < limit {
			nodeIDList = append(nodeIDList, nodeID)
		}
		currentIndex++
		
		if len(nodeIDList) >= limit {
			break
		}
	}

	hasMore := currentIndex < len(g.nodeIDs)

	if g.nodeLoader == nil {
		return nil, false, fmt.Errorf("node loader not configured")
	}

	// Load the actual nodes
	nodes, err := g.nodeLoader.LoadNodes(ctx, nodeIDList)
	if err != nil {
		return nil, false, err
	}

	return nodes, hasMore, nil
}

// GetEdges loads edges on-demand with pagination
func (g *GraphLazy) GetEdgesPaginated(ctx context.Context, limit int, offset int) ([]*Edge, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	// Collect edge keys for the requested page
	edgeKeyList := make([]string, 0, limit)
	currentIndex := 0
	
	for edgeKey := range g.edgeKeys {
		if currentIndex >= offset && len(edgeKeyList) < limit {
			edgeKeyList = append(edgeKeyList, edgeKey)
		}
		currentIndex++
		
		if len(edgeKeyList) >= limit {
			break
		}
	}

	hasMore := currentIndex < len(g.edgeKeys)

	if g.edgeLoader == nil {
		return nil, false, fmt.Errorf("edge loader not configured")
	}

	// Load the actual edges
	edges, err := g.edgeLoader.LoadEdges(ctx, edgeKeyList)
	if err != nil {
		return nil, false, err
	}

	return edges, hasMore, nil
}

// GetNodeConnectivity gets the connectivity of a node without loading all edges
func (g *GraphLazy) GetNodeConnectivity(nodeID valueobjects.NodeID) int {
	count := 0
	for edgeKey := range g.edgeKeys {
		if g.edgeInvolvesNode(edgeKey, nodeID) {
			count++
		}
	}
	return count
}

// Validate ensures graph invariants without loading all data
func (g *GraphLazy) Validate() error {
	// Check metadata consistency
	if len(g.nodeIDs) != g.metadata.NodeCount {
		return pkgerrors.NewValidationError("node count mismatch")
	}
	if len(g.edgeKeys) != g.metadata.EdgeCount {
		return pkgerrors.NewValidationError("edge count mismatch")
	}
	return nil
}

// GetUncommittedEvents returns all uncommitted domain events
func (g *GraphLazy) GetUncommittedEvents() []events.DomainEvent {
	return g.events
}

// MarkEventsAsCommitted clears all uncommitted events
func (g *GraphLazy) MarkEventsAsCommitted() {
	g.events = []events.DomainEvent{}
}

// Private helper methods

func (g *GraphLazy) addEvent(event events.DomainEvent) {
	g.events = append(g.events, event)
}

func (g *GraphLazy) makeEdgeKey(sourceID, targetID valueobjects.NodeID) string {
	return sourceID.String() + "->" + targetID.String()
}

func (g *GraphLazy) edgeInvolvesNode(edgeKey string, nodeID valueobjects.NodeID) bool {
	// Simple check - in production, parse the edge key properly
	nodeIDStr := nodeID.String()
	return len(edgeKey) > len(nodeIDStr) && 
		(edgeKey[:len(nodeIDStr)] == nodeIDStr || 
		 edgeKey[len(edgeKey)-len(nodeIDStr):] == nodeIDStr)
}

// ReconstructGraphLazy recreates a lazy graph from stored data
func ReconstructGraphLazy(
	id string,
	userID string,
	name string,
	description string,
	nodeIDs []valueobjects.NodeID,
	edgeKeys []string,
	metadata GraphMetadata,
	createdAt time.Time,
	updatedAt time.Time,
	version int,
) (*GraphLazy, error) {
	if id == "" || userID == "" || name == "" {
		return nil, pkgerrors.NewValidationError("required fields missing for graph reconstruction")
	}

	graph := &GraphLazy{
		id:          GraphID(id),
		userID:      userID,
		name:        name,
		description: description,
		nodeIDs:     make(map[valueobjects.NodeID]bool),
		edgeKeys:    make(map[string]bool),
		config:      config.DefaultDomainConfig(),
		metadata:    metadata,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		version:     version,
		events:      []events.DomainEvent{},
	}

	// Populate node IDs
	for _, nodeID := range nodeIDs {
		graph.nodeIDs[nodeID] = true
	}

	// Populate edge keys
	for _, edgeKey := range edgeKeys {
		graph.edgeKeys[edgeKey] = true
	}

	return graph, nil
}