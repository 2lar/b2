package aggregates

import (
	"fmt"
	"time"

	"backend/domain/config"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	pkgerrors "backend/pkg/errors"
	"github.com/google/uuid"
)

// GraphID represents a unique graph identifier
type GraphID string

// NewGraphID creates a new random GraphID
func NewGraphID() GraphID {
	return GraphID(uuid.New().String())
}

// String returns the string representation
func (id GraphID) String() string {
	return string(id)
}

// Graph is the aggregate root for the knowledge graph
// It ensures consistency boundaries for the entire graph
type Graph struct {
	id          GraphID
	userID      string
	name        string
	description string
	nodes       map[valueobjects.NodeID]*entities.Node
	edges       map[string]*Edge
	metadata    GraphMetadata
	createdAt   time.Time
	updatedAt   time.Time
	version     int
	events      []events.DomainEvent
	config      *config.DomainConfig // Domain configuration for business rules
}

// Edge represents a connection between nodes
type Edge struct {
	ID            string
	SourceID      valueobjects.NodeID
	TargetID      valueobjects.NodeID
	Type          entities.EdgeType
	Weight        float64
	Bidirectional bool
	Metadata      map[string]interface{}
	CreatedAt     time.Time
}

// GraphMetadata contains graph-level information
type GraphMetadata struct {
	NodeCount    int
	EdgeCount    int
	MaxDepth     int
	IsPublic     bool
	Tags         []string
	ViewSettings ViewSettings
}

// ViewSettings contains display preferences
type ViewSettings struct {
	Layout     LayoutType
	Theme      string
	NodeSize   string
	EdgeStyle  string
	ShowLabels bool
}

// LayoutType defines graph layout algorithms
type LayoutType string

const (
	LayoutForceDirected LayoutType = "force_directed"
	LayoutHierarchical  LayoutType = "hierarchical"
	LayoutCircular      LayoutType = "circular"
	LayoutGrid          LayoutType = "grid"
)

// NewGraph creates a new graph aggregate with default configuration
func NewGraph(userID, name string) (*Graph, error) {
	return NewGraphWithConfig(userID, name, config.DefaultDomainConfig())
}

// NewGraphWithConfig creates a new graph aggregate with specific configuration
func NewGraphWithConfig(userID, name string, cfg *config.DomainConfig) (*Graph, error) {
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
	graph := &Graph{
		id:          NewGraphID(),
		userID:      userID,
		name:        name,
		description: "Knowledge graph for " + name,
		nodes:       make(map[valueobjects.NodeID]*entities.Node),
		edges:       make(map[string]*Edge),
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

// ReconstructGraph recreates a graph from stored data
func ReconstructGraph(
	id string,
	userID string,
	name string,
	description string,
	isDefault bool,
	createdAt string,
	updatedAt string,
) (*Graph, error) {
	if id == "" || userID == "" || name == "" {
		return nil, pkgerrors.NewValidationError("required fields missing for graph reconstruction")
	}

	created, _ := time.Parse(time.RFC3339, createdAt)
	updated, _ := time.Parse(time.RFC3339, updatedAt)

	graph := &Graph{
		id:          GraphID(id),
		userID:      userID,
		name:        name,
		description: description,
		nodes:       make(map[valueobjects.NodeID]*entities.Node),
		edges:       make(map[string]*Edge),
		config:      config.DefaultDomainConfig(), // Use default config for reconstructed graphs
		metadata: GraphMetadata{
			ViewSettings: ViewSettings{
				Layout:     LayoutForceDirected,
				ShowLabels: true,
			},
		},
		createdAt: created,
		updatedAt: updated,
		version:   1,
		events:    []events.DomainEvent{},
	}

	return graph, nil
}

// ID returns the graph's unique identifier
func (g *Graph) ID() GraphID {
	return g.id
}

// Version returns the graph version
func (g *Graph) Version() int {
	return g.version
}

// UserID returns the owner's ID
func (g *Graph) UserID() string {
	return g.userID
}

// Name returns the graph's name
func (g *Graph) Name() string {
	return g.name
}

// Description returns the graph's description
func (g *Graph) Description() string {
	return g.description
}

// Nodes returns all nodes in the graph
func (g *Graph) Nodes() (map[valueobjects.NodeID]*entities.Node, error) {
	// Prevent memory exhaustion for large graphs
	if len(g.nodes) > 1000 {
		return nil, fmt.Errorf("graph has %d nodes (limit: 1000), use GetNodesPaginated instead", len(g.nodes))
	}

	// Return a copy to maintain encapsulation
	nodes := make(map[valueobjects.NodeID]*entities.Node, len(g.nodes))
	for k, v := range g.nodes {
		nodes[k] = v
	}
	return nodes, nil
}

// Edges returns all edges in the graph
func (g *Graph) Edges() map[string]*Edge {
	// Return a copy to maintain encapsulation
	edges := make(map[string]*Edge, len(g.edges))
	for k, v := range g.edges {
		edges[k] = v
	}
	return edges
}

// NodeCount returns the number of nodes in the graph
func (g *Graph) NodeCount() int {
	return g.metadata.NodeCount
}

// EdgeCount returns the number of edges in the graph
func (g *Graph) EdgeCount() int {
	return g.metadata.EdgeCount
}

// Metadata returns the graph's metadata
func (g *Graph) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"nodeCount":  g.metadata.NodeCount,
		"edgeCount":  g.metadata.EdgeCount,
		"isPublic":   g.metadata.IsPublic,
		"tags":       g.metadata.Tags,
		"layout":     g.metadata.ViewSettings.Layout,
		"theme":      g.metadata.ViewSettings.Theme,
		"showLabels": g.metadata.ViewSettings.ShowLabels,
	}
}

// CreatedAt returns when the graph was created
func (g *Graph) CreatedAt() time.Time {
	return g.createdAt
}

// UpdatedAt returns when the graph was last updated
func (g *Graph) UpdatedAt() time.Time {
	return g.updatedAt
}

// IsDefault returns whether this is the user's default graph
func (g *Graph) IsDefault() bool {
	// For now, the first graph is the default
	return g.name == "Default Graph"
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *entities.Node) error {
	if node == nil {
		return pkgerrors.NewValidationError("node cannot be nil")
	}

	nodeID := node.ID()
	if _, exists := g.nodes[nodeID]; exists {
		return pkgerrors.NewConflictError("node already exists in graph")
	}

	// Check node limit (business rule)
	if g.config != nil && len(g.nodes) >= g.config.MaxNodesPerGraph {
		return fmt.Errorf("maximum nodes reached: %d", g.config.MaxNodesPerGraph)
	}

	g.nodes[nodeID] = node
	g.metadata.NodeCount++
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

// LoadNode adds a node to the graph during reconstruction from storage
// Unlike AddNode, this doesn't check for duplicates or emit events
// This is specifically for loading existing nodes from the database
func (g *Graph) LoadNode(node *entities.Node) error {
	if node == nil {
		return pkgerrors.NewValidationError("node cannot be nil")
	}

	nodeID := node.ID()
	
	// Check node limit (business rule)
	if g.config != nil && len(g.nodes) >= g.config.MaxNodesPerGraph {
		return fmt.Errorf("maximum nodes reached: %d", g.config.MaxNodesPerGraph)
	}

	// Simply add the node without duplicate checking
	// This is safe because we're loading from a trusted source (database)
	g.nodes[nodeID] = node
	
	// Update metadata to reflect the actual count
	g.metadata.NodeCount = len(g.nodes)
	
	// Don't emit events or update version during loading
	// These are already persisted state, not new changes
	
	return nil
}

// ConnectNodes creates an edge between two nodes
func (g *Graph) ConnectNodes(sourceID, targetID valueobjects.NodeID, edgeType entities.EdgeType) (*Edge, error) {
	// Validate nodes exist
	sourceNode, sourceExists := g.nodes[sourceID]
	_, targetExists := g.nodes[targetID]

	if !sourceExists || !targetExists {
		return nil, pkgerrors.NewValidationError("both nodes must exist in graph")
	}

	// Check for self-reference
	if sourceID.Equals(targetID) {
		return nil, pkgerrors.NewValidationError("cannot connect node to itself")
	}

	// Check for duplicate edge
	edgeKey := g.makeEdgeKey(sourceID, targetID)
	if _, exists := g.edges[edgeKey]; exists {
		return nil, pkgerrors.NewConflictError("edge already exists")
	}

	// Check edge limit (business rule)
	if g.config != nil && len(g.edges) >= g.config.MaxEdgesPerGraph {
		return nil, fmt.Errorf("maximum edges reached: %d", g.config.MaxEdgesPerGraph)
	}

	// Create the edge
	edge := &Edge{
		ID:            uuid.New().String(),
		SourceID:      sourceID,
		TargetID:      targetID,
		Type:          edgeType,
		Weight:        1.0,
		Bidirectional: false,
		CreatedAt:     time.Now(),
	}

	// Update the source node's connections
	if err := sourceNode.ConnectTo(targetID, edgeType); err != nil {
		return nil, err
	}

	g.edges[edgeKey] = edge
	g.metadata.EdgeCount++
	g.updatedAt = time.Now()
	g.version++

	g.addEvent(events.NodesConnected{
		BaseEvent: events.BaseEvent{
			AggregateID: g.id.String(),
			EventType:   "graph.nodes_connected",
			Timestamp:   g.updatedAt,
			Version:     1,
		},
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: string(edgeType),
	})

	return edge, nil
}

// LoadEdge loads an existing edge into the graph (used for repository reconstruction)
// This method bypasses validation since we're loading already-persisted edges
func (g *Graph) LoadEdge(edge *Edge) error {
	if edge == nil {
		return pkgerrors.NewValidationError("edge cannot be nil")
	}

	// Check that both nodes exist with detailed error reporting
	sourceNode, sourceExists := g.nodes[edge.SourceID]
	_, targetExists := g.nodes[edge.TargetID]

	if !sourceExists {
		return fmt.Errorf("source node %s not found for edge %s", edge.SourceID.String(), edge.ID)
	}
	if !targetExists {
		return fmt.Errorf("target node %s not found for edge %s", edge.TargetID.String(), edge.ID)
	}

	edgeKey := g.makeEdgeKey(edge.SourceID, edge.TargetID)

	// Only add if not already present
	if _, exists := g.edges[edgeKey]; !exists {
		// Check edge limit even when loading
		if g.config != nil && len(g.edges) >= g.config.MaxEdgesPerGraph {
			return fmt.Errorf("cannot load edge: maximum edges reached (%d)", g.config.MaxEdgesPerGraph)
		}

		g.edges[edgeKey] = edge
		g.metadata.EdgeCount++

		// Update the source node's connections with error handling
		if err := sourceNode.ConnectTo(edge.TargetID, edge.Type); err != nil {
			// Rollback the edge addition if node connection fails
			delete(g.edges, edgeKey)
			g.metadata.EdgeCount--
			return fmt.Errorf("failed to update node connections: %w", err)
		}
	}

	return nil
}

// RemoveNode removes a node and its edges from the graph
func (g *Graph) RemoveNode(nodeID valueobjects.NodeID) error {
	node, exists := g.nodes[nodeID]
	if !exists {
		return pkgerrors.NewNotFoundError("node")
	}

	// Archive the node first
	if err := node.Archive(); err != nil {
		return err
	}

	// Remove all edges connected to this node
	edgesToRemove := []string{}
	for key, edge := range g.edges {
		if edge.SourceID.Equals(nodeID) || edge.TargetID.Equals(nodeID) {
			edgesToRemove = append(edgesToRemove, key)
		}
	}

	for _, key := range edgesToRemove {
		delete(g.edges, key)
		g.metadata.EdgeCount--
	}

	// Remove the node
	delete(g.nodes, nodeID)
	g.metadata.NodeCount--
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

// GetNode retrieves a node by ID
func (g *Graph) GetNode(nodeID valueobjects.NodeID) (*entities.Node, error) {
	node, exists := g.nodes[nodeID]
	if !exists {
		return nil, pkgerrors.NewNotFoundError("node")
	}
	return node, nil
}

// HasNode checks if a node exists in the graph without error
func (g *Graph) HasNode(nodeID valueobjects.NodeID) bool {
	_, exists := g.nodes[nodeID]
	return exists
}

// GetNodes returns all nodes in the graph
// Returns error for large graphs to prevent memory issues - use GetNodesPaginated instead
func (g *Graph) GetNodes() ([]*entities.Node, error) {
	// Prevent memory exhaustion for large graphs
	if len(g.nodes) > 1000 {
		return nil, fmt.Errorf("graph has %d nodes (limit: 1000), use GetNodesPaginated instead", len(g.nodes))
	}

	nodes := make([]*entities.Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// GetNodesPaginated returns nodes with pagination support
func (g *Graph) GetNodesPaginated(limit int, lastNodeID *valueobjects.NodeID) ([]*entities.Node, bool) {
	if limit <= 0 || limit > 100 {
		limit = 100 // Sensible default
	}

	nodes := make([]*entities.Node, 0, limit)
	hasMore := false
	skipUntilFound := lastNodeID != nil

	for id, node := range g.nodes {
		if skipUntilFound {
			if id.Equals(*lastNodeID) {
				skipUntilFound = false
			}
			continue
		}

		if len(nodes) >= limit {
			hasMore = true
			break
		}

		nodes = append(nodes, node)
	}

	return nodes, hasMore
}

// GetEdges returns all edges in the graph
func (g *Graph) GetEdges() []*Edge {
	edges := make([]*Edge, 0, len(g.edges))
	for _, edge := range g.edges {
		edges = append(edges, edge)
	}
	return edges
}

// FindPath finds a path between two nodes using BFS
func (g *Graph) FindPath(startID, endID valueobjects.NodeID) ([]valueobjects.NodeID, error) {
	if _, exists := g.nodes[startID]; !exists {
		return nil, pkgerrors.NewNotFoundError("start node")
	}
	if _, exists := g.nodes[endID]; !exists {
		return nil, pkgerrors.NewNotFoundError("end node")
	}

	if startID.Equals(endID) {
		return []valueobjects.NodeID{startID}, nil
	}

	// BFS implementation
	visited := make(map[valueobjects.NodeID]bool)
	parent := make(map[valueobjects.NodeID]valueobjects.NodeID)
	queue := []valueobjects.NodeID{startID}
	visited[startID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check all edges from current node
		for _, edge := range g.edges {
			var next valueobjects.NodeID

			if edge.SourceID.Equals(current) {
				next = edge.TargetID
			} else if edge.Bidirectional && edge.TargetID.Equals(current) {
				next = edge.SourceID
			} else {
				continue
			}

			if !visited[next] {
				visited[next] = true
				parent[next] = current
				queue = append(queue, next)

				if next.Equals(endID) {
					// Reconstruct path
					path := []valueobjects.NodeID{}
					for n := endID; !n.IsZero(); n = parent[n] {
						path = append([]valueobjects.NodeID{n}, path...)
						if n.Equals(startID) {
							break
						}
					}
					return path, nil
				}
			}
		}
	}

	return nil, pkgerrors.NewNotFoundError("path between nodes")
}

// GetClusters identifies clusters of connected nodes
func (g *Graph) GetClusters() [][]valueobjects.NodeID {
	visited := make(map[valueobjects.NodeID]bool)
	var clusters [][]valueobjects.NodeID

	for nodeID := range g.nodes {
		if !visited[nodeID] {
			cluster := g.dfs(nodeID, visited)
			clusters = append(clusters, cluster)
		}
	}

	return clusters
}

// Validate ensures graph invariants
func (g *Graph) Validate() error {
	// Check for orphaned edges
	for _, edge := range g.edges {
		if _, sourceExists := g.nodes[edge.SourceID]; !sourceExists {
			return pkgerrors.NewValidationError("edge references non-existent source node")
		}
		if _, targetExists := g.nodes[edge.TargetID]; !targetExists {
			return pkgerrors.NewValidationError("edge references non-existent target node")
		}
	}

	// Check metadata consistency
	if len(g.nodes) != g.metadata.NodeCount {
		return pkgerrors.NewValidationError("node count mismatch")
	}
	if len(g.edges) != g.metadata.EdgeCount {
		return pkgerrors.NewValidationError("edge count mismatch")
	}

	return nil
}

// GetUncommittedEvents returns all uncommitted domain events
func (g *Graph) GetUncommittedEvents() []events.DomainEvent {
	// Collect events from the graph itself
	allEvents := make([]events.DomainEvent, len(g.events))
	copy(allEvents, g.events)

	// Collect events from all nodes
	for _, node := range g.nodes {
		allEvents = append(allEvents, node.GetUncommittedEvents()...)
	}

	return allEvents
}

// MarkEventsAsCommitted clears all uncommitted events
func (g *Graph) MarkEventsAsCommitted() {
	g.events = []events.DomainEvent{}

	// Also mark node events as committed
	for _, node := range g.nodes {
		node.MarkEventsAsCommitted()
	}
}

// Private helper methods

func (g *Graph) addEvent(event events.DomainEvent) {
	g.events = append(g.events, event)
}

func (g *Graph) makeEdgeKey(sourceID, targetID valueobjects.NodeID) string {
	return sourceID.String() + "->" + targetID.String()
}

func (g *Graph) dfs(nodeID valueobjects.NodeID, visited map[valueobjects.NodeID]bool) []valueobjects.NodeID {
	cluster := []valueobjects.NodeID{nodeID}
	visited[nodeID] = true

	for _, edge := range g.edges {
		var next valueobjects.NodeID

		if edge.SourceID.Equals(nodeID) {
			next = edge.TargetID
		} else if edge.Bidirectional && edge.TargetID.Equals(nodeID) {
			next = edge.SourceID
		} else {
			continue
		}

		if !visited[next] {
			cluster = append(cluster, g.dfs(next, visited)...)
		}
	}

	return cluster
}
