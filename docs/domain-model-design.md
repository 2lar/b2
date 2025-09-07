# Domain Model Design

## Bounded Contexts

### 1. Knowledge Graph Context
**Purpose**: Manages the core graph structure of nodes and edges representing knowledge relationships.

**Core Concepts**:
- **Node**: Represents a unit of knowledge (idea, concept, fact)
- **Edge**: Represents relationships between nodes
- **Graph**: Aggregate root containing nodes and edges
- **Cluster**: Groups of related nodes

**Domain Events**:
- NodeCreated
- NodeUpdated
- NodeDeleted
- EdgeCreated
- EdgeDeleted
- GraphStructureChanged

### 2. Content Management Context
**Purpose**: Handles the content within nodes including text, media, and metadata.

**Core Concepts**:
- **Content**: Rich text, markdown, or media
- **Metadata**: Tags, categories, timestamps
- **Version**: Content history and revisions
- **Attachment**: Files, images, links

**Domain Events**:
- ContentUpdated
- MetadataChanged
- AttachmentAdded
- VersionCreated

### 3. User Context
**Purpose**: Manages user accounts, preferences, and permissions.

**Core Concepts**:
- **User**: Account holder
- **Profile**: User preferences and settings
- **Permission**: Access control rules
- **Workspace**: User's knowledge space

**Domain Events**:
- UserRegistered
- ProfileUpdated
- PermissionGranted
- WorkspaceCreated

### 4. Analytics Context
**Purpose**: Tracks usage patterns and provides insights.

**Core Concepts**:
- **Activity**: User actions and interactions
- **Metrics**: Performance and usage statistics
- **Insight**: Derived patterns and recommendations
- **Report**: Aggregated analytics data

**Domain Events**:
- ActivityRecorded
- MetricCalculated
- InsightGenerated

## Core Domain Entities

### Node Entity
```go
package domain

import (
    "time"
    "errors"
)

// NodeID is a value object representing a unique node identifier
type NodeID struct {
    value string
}

// NewNodeID creates a new NodeID with validation
func NewNodeID(id string) (NodeID, error) {
    if id == "" {
        return NodeID{}, errors.New("node ID cannot be empty")
    }
    if !isValidUUID(id) {
        return NodeID{}, errors.New("node ID must be a valid UUID")
    }
    return NodeID{value: id}, nil
}

// String returns the string representation of the NodeID
func (id NodeID) String() string {
    return id.value
}

// Position is a value object representing node coordinates
type Position struct {
    X float64
    Y float64
    Z float64 // For 3D graphs
}

// NewPosition creates a position with validation
func NewPosition(x, y, z float64) (Position, error) {
    if !isValidCoordinate(x) || !isValidCoordinate(y) || !isValidCoordinate(z) {
        return Position{}, errors.New("invalid coordinates")
    }
    return Position{X: x, Y: y, Z: z}, nil
}

// NodeContent is a value object for node content
type NodeContent struct {
    title   string
    body    string
    format  ContentFormat
}

// ContentFormat represents the format of the content
type ContentFormat string

const (
    FormatPlainText ContentFormat = "text"
    FormatMarkdown  ContentFormat = "markdown"
    FormatHTML      ContentFormat = "html"
    FormatJSON      ContentFormat = "json"
)

// NewNodeContent creates content with validation
func NewNodeContent(title, body string, format ContentFormat) (NodeContent, error) {
    if title == "" {
        return NodeContent{}, errors.New("title cannot be empty")
    }
    if len(title) > 200 {
        return NodeContent{}, errors.New("title too long")
    }
    if len(body) > 50000 {
        return NodeContent{}, errors.New("content body too long")
    }
    return NodeContent{
        title:  title,
        body:   body,
        format: format,
    }, nil
}

// Node is the main entity representing a knowledge unit
type Node struct {
    id         NodeID
    userID     string
    content    NodeContent
    position   Position
    metadata   Metadata
    edges      []EdgeReference
    createdAt  time.Time
    updatedAt  time.Time
    version    int
    events     []DomainEvent
}

// EdgeReference is a lightweight reference to connected edges
type EdgeReference struct {
    EdgeID   string
    TargetID NodeID
    Type     EdgeType
}

// Metadata contains additional node information
type Metadata struct {
    Tags       []string
    Categories []string
    Color      string
    Icon       string
    Priority   int
    Status     NodeStatus
    Custom     map[string]interface{}
}

// NodeStatus represents the state of a node
type NodeStatus string

const (
    StatusDraft     NodeStatus = "draft"
    StatusPublished NodeStatus = "published"
    StatusArchived  NodeStatus = "archived"
)

// Factory method for creating a new node
func NewNode(userID string, content NodeContent, position Position) (*Node, error) {
    id, err := generateNodeID()
    if err != nil {
        return nil, err
    }
    
    node := &Node{
        id:        id,
        userID:    userID,
        content:   content,
        position:  position,
        metadata:  Metadata{Status: StatusDraft},
        edges:     []EdgeReference{},
        createdAt: time.Now(),
        updatedAt: time.Now(),
        version:   1,
        events:    []DomainEvent{},
    }
    
    node.addEvent(NodeCreated{
        NodeID:    id,
        UserID:    userID,
        Timestamp: time.Now(),
    })
    
    return node, nil
}

// Business methods
func (n *Node) UpdateContent(content NodeContent) error {
    if n.metadata.Status == StatusArchived {
        return errors.New("cannot update archived node")
    }
    
    n.content = content
    n.updatedAt = time.Now()
    n.version++
    
    n.addEvent(NodeContentUpdated{
        NodeID:    n.id,
        Timestamp: time.Now(),
    })
    
    return nil
}

func (n *Node) MoveTo(position Position) error {
    n.position = position
    n.updatedAt = time.Now()
    
    n.addEvent(NodeMoved{
        NodeID:      n.id,
        NewPosition: position,
        Timestamp:   time.Now(),
    })
    
    return nil
}

func (n *Node) ConnectTo(targetID NodeID, edgeType EdgeType) error {
    // Check for self-reference
    if n.id == targetID {
        return errors.New("cannot connect node to itself")
    }
    
    // Check for duplicate connection
    for _, edge := range n.edges {
        if edge.TargetID == targetID && edge.Type == edgeType {
            return errors.New("connection already exists")
        }
    }
    
    edgeRef := EdgeReference{
        EdgeID:   generateEdgeID(),
        TargetID: targetID,
        Type:     edgeType,
    }
    
    n.edges = append(n.edges, edgeRef)
    n.updatedAt = time.Now()
    
    n.addEvent(NodesConnected{
        SourceID:  n.id,
        TargetID:  targetID,
        EdgeType:  edgeType,
        Timestamp: time.Now(),
    })
    
    return nil
}

func (n *Node) Archive() error {
    if n.metadata.Status == StatusArchived {
        return errors.New("node already archived")
    }
    
    n.metadata.Status = StatusArchived
    n.updatedAt = time.Now()
    
    n.addEvent(NodeArchived{
        NodeID:    n.id,
        Timestamp: time.Now(),
    })
    
    return nil
}

// Domain event management
func (n *Node) addEvent(event DomainEvent) {
    n.events = append(n.events, event)
}

func (n *Node) GetUncommittedEvents() []DomainEvent {
    return n.events
}

func (n *Node) MarkEventsAsCommitted() {
    n.events = []DomainEvent{}
}
```

### Edge Entity
```go
package domain

import (
    "time"
    "errors"
)

// EdgeType defines the type of relationship
type EdgeType string

const (
    EdgeTypeReference   EdgeType = "reference"
    EdgeTypeParentChild EdgeType = "parent_child"
    EdgeTypeSimilar     EdgeType = "similar"
    EdgeTypeOpposite    EdgeType = "opposite"
    EdgeTypeSequential  EdgeType = "sequential"
    EdgeTypeCustom      EdgeType = "custom"
)

// EdgeWeight represents the strength of a connection
type EdgeWeight float64

// NewEdgeWeight creates a weight with validation
func NewEdgeWeight(weight float64) (EdgeWeight, error) {
    if weight < 0 || weight > 1 {
        return EdgeWeight(0), errors.New("weight must be between 0 and 1")
    }
    return EdgeWeight(weight), nil
}

// Edge represents a connection between nodes
type Edge struct {
    id          string
    sourceID    NodeID
    targetID    NodeID
    edgeType    EdgeType
    weight      EdgeWeight
    label       string
    metadata    map[string]interface{}
    createdAt   time.Time
    createdBy   string
    bidirectional bool
    events      []DomainEvent
}

// NewEdge creates a new edge with validation
func NewEdge(sourceID, targetID NodeID, edgeType EdgeType, createdBy string) (*Edge, error) {
    if sourceID == targetID {
        return nil, errors.New("cannot create self-referencing edge")
    }
    
    weight, _ := NewEdgeWeight(1.0) // Default weight
    
    edge := &Edge{
        id:           generateEdgeID(),
        sourceID:     sourceID,
        targetID:     targetID,
        edgeType:     edgeType,
        weight:       weight,
        createdAt:    time.Now(),
        createdBy:    createdBy,
        bidirectional: false,
        metadata:     make(map[string]interface{}),
        events:       []DomainEvent{},
    }
    
    edge.addEvent(EdgeCreated{
        EdgeID:    edge.id,
        SourceID:  sourceID,
        TargetID:  targetID,
        EdgeType:  edgeType,
        Timestamp: time.Now(),
    })
    
    return edge, nil
}

// Business methods
func (e *Edge) UpdateWeight(weight EdgeWeight) error {
    e.weight = weight
    
    e.addEvent(EdgeWeightUpdated{
        EdgeID:    e.id,
        NewWeight: weight,
        Timestamp: time.Now(),
    })
    
    return nil
}

func (e *Edge) MakeBidirectional() {
    if !e.bidirectional {
        e.bidirectional = true
        
        e.addEvent(EdgeMadeBidirectional{
            EdgeID:    e.id,
            Timestamp: time.Now(),
        })
    }
}

func (e *Edge) SetLabel(label string) error {
    if len(label) > 100 {
        return errors.New("label too long")
    }
    
    e.label = label
    return nil
}

// Check if this edge connects the given nodes
func (e *Edge) Connects(nodeA, nodeB NodeID) bool {
    if e.bidirectional {
        return (e.sourceID == nodeA && e.targetID == nodeB) ||
               (e.sourceID == nodeB && e.targetID == nodeA)
    }
    return e.sourceID == nodeA && e.targetID == nodeB
}
```

### Graph Aggregate
```go
package domain

import (
    "errors"
    "time"
)

// GraphID represents a unique graph identifier
type GraphID string

// Graph is the aggregate root for the knowledge graph
type Graph struct {
    id          GraphID
    userID      string
    name        string
    description string
    nodes       map[NodeID]*Node
    edges       map[string]*Edge
    metadata    GraphMetadata
    createdAt   time.Time
    updatedAt   time.Time
    version     int
    events      []DomainEvent
}

// GraphMetadata contains graph-level information
type GraphMetadata struct {
    NodeCount      int
    EdgeCount      int
    MaxDepth       int
    IsPublic       bool
    Tags           []string
    ViewSettings   ViewSettings
}

// ViewSettings contains display preferences
type ViewSettings struct {
    Layout      LayoutType
    Theme       string
    NodeSize    string
    EdgeStyle   string
    ShowLabels  bool
}

// LayoutType defines graph layout algorithms
type LayoutType string

const (
    LayoutForceDirected LayoutType = "force_directed"
    LayoutHierarchical  LayoutType = "hierarchical"
    LayoutCircular      LayoutType = "circular"
    LayoutGrid          LayoutType = "grid"
)

// NewGraph creates a new graph aggregate
func NewGraph(userID, name string) (*Graph, error) {
    if userID == "" {
        return nil, errors.New("userID required")
    }
    if name == "" {
        return nil, errors.New("graph name required")
    }
    
    graph := &Graph{
        id:        GraphID(generateGraphID()),
        userID:    userID,
        name:      name,
        nodes:     make(map[NodeID]*Node),
        edges:     make(map[string]*Edge),
        metadata:  GraphMetadata{},
        createdAt: time.Now(),
        updatedAt: time.Now(),
        version:   1,
        events:    []DomainEvent{},
    }
    
    graph.addEvent(GraphCreated{
        GraphID:   graph.id,
        UserID:    userID,
        Name:      name,
        Timestamp: time.Now(),
    })
    
    return graph, nil
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node) error {
    if node == nil {
        return errors.New("node cannot be nil")
    }
    
    if _, exists := g.nodes[node.id]; exists {
        return errors.New("node already exists in graph")
    }
    
    g.nodes[node.id] = node
    g.metadata.NodeCount++
    g.updatedAt = time.Now()
    g.version++
    
    g.addEvent(NodeAddedToGraph{
        GraphID:   g.id,
        NodeID:    node.id,
        Timestamp: time.Now(),
    })
    
    return nil
}

// ConnectNodes creates an edge between two nodes
func (g *Graph) ConnectNodes(sourceID, targetID NodeID, edgeType EdgeType) (*Edge, error) {
    sourceNode, sourceExists := g.nodes[sourceID]
    targetNode, targetExists := g.nodes[targetID]
    
    if !sourceExists || !targetExists {
        return nil, errors.New("both nodes must exist in graph")
    }
    
    edge, err := NewEdge(sourceID, targetID, edgeType, g.userID)
    if err != nil {
        return nil, err
    }
    
    g.edges[edge.id] = edge
    g.metadata.EdgeCount++
    
    // Update node edge references
    sourceNode.ConnectTo(targetID, edgeType)
    
    g.updatedAt = time.Now()
    g.version++
    
    return edge, nil
}

// RemoveNode removes a node and its edges from the graph
func (g *Graph) RemoveNode(nodeID NodeID) error {
    node, exists := g.nodes[nodeID]
    if !exists {
        return errors.New("node not found")
    }
    
    // Remove all edges connected to this node
    for edgeID, edge := range g.edges {
        if edge.sourceID == nodeID || edge.targetID == nodeID {
            delete(g.edges, edgeID)
            g.metadata.EdgeCount--
        }
    }
    
    delete(g.nodes, nodeID)
    g.metadata.NodeCount--
    g.updatedAt = time.Now()
    g.version++
    
    g.addEvent(NodeRemovedFromGraph{
        GraphID:   g.id,
        NodeID:    nodeID,
        Timestamp: time.Now(),
    })
    
    return nil
}

// FindPath finds a path between two nodes
func (g *Graph) FindPath(startID, endID NodeID) ([]NodeID, error) {
    if _, exists := g.nodes[startID]; !exists {
        return nil, errors.New("start node not found")
    }
    if _, exists := g.nodes[endID]; !exists {
        return nil, errors.New("end node not found")
    }
    
    // Implement BFS or Dijkstra's algorithm
    path := g.breadthFirstSearch(startID, endID)
    if len(path) == 0 {
        return nil, errors.New("no path exists between nodes")
    }
    
    return path, nil
}

// GetClusters identifies clusters of connected nodes
func (g *Graph) GetClusters() [][]NodeID {
    visited := make(map[NodeID]bool)
    var clusters [][]NodeID
    
    for nodeID := range g.nodes {
        if !visited[nodeID] {
            cluster := g.depthFirstSearch(nodeID, visited)
            clusters = append(clusters, cluster)
        }
    }
    
    return clusters
}

// Validate ensures graph invariants
func (g *Graph) Validate() error {
    // Check for orphaned edges
    for _, edge := range g.edges {
        if _, sourceExists := g.nodes[edge.sourceID]; !sourceExists {
            return errors.New("edge references non-existent source node")
        }
        if _, targetExists := g.nodes[edge.targetID]; !targetExists {
            return errors.New("edge references non-existent target node")
        }
    }
    
    // Check metadata consistency
    if len(g.nodes) != g.metadata.NodeCount {
        return errors.New("node count mismatch")
    }
    if len(g.edges) != g.metadata.EdgeCount {
        return errors.New("edge count mismatch")
    }
    
    return nil
}
```

## Value Objects

### Category Value Object
```go
package domain

import (
    "errors"
    "regexp"
    "strings"
)

// Category represents a hierarchical classification
type Category struct {
    name     string
    parent   *Category
    children []*Category
    color    string
    icon     string
}

// NewCategory creates a category with validation
func NewCategory(name string) (*Category, error) {
    if err := validateCategoryName(name); err != nil {
        return nil, err
    }
    
    return &Category{
        name:     name,
        children: []*Category{},
    }, nil
}

func validateCategoryName(name string) error {
    if name == "" {
        return errors.New("category name cannot be empty")
    }
    if len(name) > 50 {
        return errors.New("category name too long")
    }
    
    // Allow alphanumeric, spaces, hyphens, underscores
    validName := regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
    if !validName.MatchString(name) {
        return errors.New("invalid category name format")
    }
    
    return nil
}

func (c *Category) GetFullPath() string {
    if c.parent == nil {
        return c.name
    }
    return c.parent.GetFullPath() + "/" + c.name
}

func (c *Category) AddSubcategory(sub *Category) error {
    if sub == nil {
        return errors.New("subcategory cannot be nil")
    }
    
    // Check for circular reference
    if c.isDescendantOf(sub) {
        return errors.New("circular reference detected")
    }
    
    sub.parent = c
    c.children = append(c.children, sub)
    return nil
}

func (c *Category) isDescendantOf(other *Category) bool {
    current := c.parent
    for current != nil {
        if current == other {
            return true
        }
        current = current.parent
    }
    return false
}
```

### Tag Value Object
```go
package domain

import (
    "errors"
    "regexp"
    "strings"
)

// Tag represents a label for categorization
type Tag struct {
    value string
}

// NewTag creates a tag with validation
func NewTag(value string) (Tag, error) {
    value = strings.TrimSpace(strings.ToLower(value))
    
    if value == "" {
        return Tag{}, errors.New("tag cannot be empty")
    }
    
    if len(value) > 30 {
        return Tag{}, errors.New("tag too long")
    }
    
    // Allow alphanumeric and hyphens
    validTag := regexp.MustCompile(`^[a-z0-9\-]+$`)
    if !validTag.MatchString(value) {
        return Tag{}, errors.New("invalid tag format")
    }
    
    return Tag{value: value}, nil
}

func (t Tag) String() string {
    return t.value
}

func (t Tag) Equals(other Tag) bool {
    return t.value == other.value
}
```

## Domain Services

### Graph Analysis Service
```go
package domain

import (
    "math"
)

// GraphAnalyzer provides graph analysis capabilities
type GraphAnalyzer struct{}

// CalculateCentrality finds the most central nodes
func (ga *GraphAnalyzer) CalculateCentrality(graph *Graph) map[NodeID]float64 {
    centrality := make(map[NodeID]float64)
    
    for nodeID := range graph.nodes {
        // Calculate degree centrality
        degree := float64(len(graph.getNodeEdges(nodeID)))
        centrality[nodeID] = degree / float64(len(graph.nodes)-1)
    }
    
    return centrality
}

// FindCommunities detects communities using modularity
func (ga *GraphAnalyzer) FindCommunities(graph *Graph) [][]NodeID {
    // Implement Louvain algorithm or similar
    return graph.GetClusters()
}

// CalculatePageRank computes PageRank scores
func (ga *GraphAnalyzer) CalculatePageRank(graph *Graph, damping float64, iterations int) map[NodeID]float64 {
    pagerank := make(map[NodeID]float64)
    nodeCount := float64(len(graph.nodes))
    
    // Initialize with equal probability
    for nodeID := range graph.nodes {
        pagerank[nodeID] = 1.0 / nodeCount
    }
    
    // Iterate to convergence
    for i := 0; i < iterations; i++ {
        newPagerank := make(map[NodeID]float64)
        
        for nodeID := range graph.nodes {
            rank := (1 - damping) / nodeCount
            
            // Sum contributions from incoming edges
            for _, edge := range graph.edges {
                if edge.targetID == nodeID {
                    sourceEdges := graph.getNodeEdges(edge.sourceID)
                    if len(sourceEdges) > 0 {
                        rank += damping * pagerank[edge.sourceID] / float64(len(sourceEdges))
                    }
                }
            }
            
            newPagerank[nodeID] = rank
        }
        
        pagerank = newPagerank
    }
    
    return pagerank
}

// GetNodeDensity calculates graph density
func (ga *GraphAnalyzer) GetNodeDensity(graph *Graph) float64 {
    nodeCount := float64(len(graph.nodes))
    if nodeCount <= 1 {
        return 0
    }
    
    edgeCount := float64(len(graph.edges))
    maxEdges := nodeCount * (nodeCount - 1)
    
    return edgeCount / maxEdges
}
```

### Connection Validator Service
```go
package domain

import (
    "errors"
)

// ConnectionValidator validates edge creation rules
type ConnectionValidator struct {
    maxOutgoingEdges int
    maxIncomingEdges int
    allowSelfLoops   bool
    allowDuplicates  bool
}

// NewConnectionValidator creates a validator with rules
func NewConnectionValidator() *ConnectionValidator {
    return &ConnectionValidator{
        maxOutgoingEdges: 50,
        maxIncomingEdges: 50,
        allowSelfLoops:   false,
        allowDuplicates:  false,
    }
}

// ValidateConnection checks if an edge can be created
func (cv *ConnectionValidator) ValidateConnection(graph *Graph, source, target NodeID, edgeType EdgeType) error {
    // Check nodes exist
    if _, exists := graph.nodes[source]; !exists {
        return errors.New("source node does not exist")
    }
    if _, exists := graph.nodes[target]; !exists {
        return errors.New("target node does not exist")
    }
    
    // Check self-loop
    if !cv.allowSelfLoops && source == target {
        return errors.New("self-loops not allowed")
    }
    
    // Check duplicates
    if !cv.allowDuplicates {
        for _, edge := range graph.edges {
            if edge.sourceID == source && edge.targetID == target && edge.edgeType == edgeType {
                return errors.New("duplicate edge not allowed")
            }
        }
    }
    
    // Check edge limits
    outgoing := 0
    incoming := 0
    for _, edge := range graph.edges {
        if edge.sourceID == source {
            outgoing++
        }
        if edge.targetID == target {
            incoming++
        }
    }
    
    if outgoing >= cv.maxOutgoingEdges {
        return errors.New("maximum outgoing edges exceeded")
    }
    if incoming >= cv.maxIncomingEdges {
        return errors.New("maximum incoming edges exceeded")
    }
    
    return nil
}

// CanMergeNodes checks if two nodes can be merged
func (cv *ConnectionValidator) CanMergeNodes(graph *Graph, nodeA, nodeB NodeID) (bool, error) {
    nodeAObj, existsA := graph.nodes[nodeA]
    nodeBObj, existsB := graph.nodes[nodeB]
    
    if !existsA || !existsB {
        return false, errors.New("both nodes must exist")
    }
    
    // Check if nodes are already connected
    for _, edge := range graph.edges {
        if edge.Connects(nodeA, nodeB) {
            return true, nil
        }
    }
    
    // Check if merge would create conflicts
    if nodeAObj.metadata.Status != nodeBObj.metadata.Status {
        return false, errors.New("nodes have different statuses")
    }
    
    return true, nil
}
```

## Domain Events

### Event Definitions
```go
package domain

import (
    "time"
)

// DomainEvent is the base interface for all domain events
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
type NodeCreated struct {
    BaseEvent
    NodeID   NodeID   `json:"node_id"`
    UserID   string   `json:"user_id"`
    Content  NodeContent `json:"content"`
    Position Position `json:"position"`
}

type NodeContentUpdated struct {
    BaseEvent
    NodeID      NodeID      `json:"node_id"`
    OldContent  NodeContent `json:"old_content"`
    NewContent  NodeContent `json:"new_content"`
}

type NodeMoved struct {
    BaseEvent
    NodeID      NodeID   `json:"node_id"`
    OldPosition Position `json:"old_position"`
    NewPosition Position `json:"new_position"`
}

type NodeArchived struct {
    BaseEvent
    NodeID NodeID `json:"node_id"`
}

type NodeDeleted struct {
    BaseEvent
    NodeID NodeID `json:"node_id"`
}

// Edge Events
type EdgeCreated struct {
    BaseEvent
    EdgeID   string   `json:"edge_id"`
    SourceID NodeID   `json:"source_id"`
    TargetID NodeID   `json:"target_id"`
    EdgeType EdgeType `json:"edge_type"`
}

type EdgeWeightUpdated struct {
    BaseEvent
    EdgeID    string     `json:"edge_id"`
    OldWeight EdgeWeight `json:"old_weight"`
    NewWeight EdgeWeight `json:"new_weight"`
}

type EdgeMadeBidirectional struct {
    BaseEvent
    EdgeID string `json:"edge_id"`
}

type EdgeDeleted struct {
    BaseEvent
    EdgeID string `json:"edge_id"`
}

// Graph Events
type GraphCreated struct {
    BaseEvent
    GraphID GraphID `json:"graph_id"`
    UserID  string  `json:"user_id"`
    Name    string  `json:"name"`
}

type NodeAddedToGraph struct {
    BaseEvent
    GraphID GraphID `json:"graph_id"`
    NodeID  NodeID  `json:"node_id"`
}

type NodeRemovedFromGraph struct {
    BaseEvent
    GraphID GraphID `json:"graph_id"`
    NodeID  NodeID  `json:"node_id"`
}

type GraphStructureChanged struct {
    BaseEvent
    GraphID   GraphID `json:"graph_id"`
    NodeCount int     `json:"node_count"`
    EdgeCount int     `json:"edge_count"`
}
```

## Specifications Pattern

### Node Specifications
```go
package domain

// Specification pattern for complex queries
type NodeSpecification interface {
    IsSatisfiedBy(node *Node) bool
    And(spec NodeSpecification) NodeSpecification
    Or(spec NodeSpecification) NodeSpecification
    Not() NodeSpecification
}

// Base specification
type nodeSpec struct {
    predicate func(*Node) bool
}

func (s nodeSpec) IsSatisfiedBy(node *Node) bool {
    return s.predicate(node)
}

func (s nodeSpec) And(other NodeSpecification) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return s.IsSatisfiedBy(n) && other.IsSatisfiedBy(n)
        },
    }
}

func (s nodeSpec) Or(other NodeSpecification) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return s.IsSatisfiedBy(n) || other.IsSatisfiedBy(n)
        },
    }
}

func (s nodeSpec) Not() NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return !s.IsSatisfiedBy(n)
        },
    }
}

// Concrete specifications
func NodeByStatus(status NodeStatus) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return n.metadata.Status == status
        },
    }
}

func NodeByUser(userID string) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return n.userID == userID
        },
    }
}

func NodeWithTag(tag string) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            for _, t := range n.metadata.Tags {
                if t == tag {
                    return true
                }
            }
            return false
        },
    }
}

func NodeCreatedAfter(date time.Time) NodeSpecification {
    return nodeSpec{
        predicate: func(n *Node) bool {
            return n.createdAt.After(date)
        },
    }
}

// Usage example:
// spec := NodeByUser("user123").
//     And(NodeByStatus(StatusPublished)).
//     And(NodeCreatedAfter(time.Now().AddDate(0, -1, 0)))
```

## Invariants and Business Rules

### Graph Invariants
1. **No orphaned edges**: Every edge must connect two existing nodes
2. **Unique node IDs**: No duplicate node IDs within a graph
3. **Edge consistency**: Edge count must match actual edges
4. **User ownership**: Nodes can only be modified by their owner
5. **Version consistency**: Version increments on every change

### Node Invariants
1. **Required fields**: Every node must have ID, content, and position
2. **Content limits**: Title max 200 chars, body max 50KB
3. **Valid coordinates**: Position values must be finite numbers
4. **Status transitions**: Draft → Published → Archived (no reverse)

### Edge Invariants
1. **No self-loops** (configurable): Nodes cannot connect to themselves
2. **Weight range**: Edge weights must be between 0 and 1
3. **Type consistency**: Edge type cannot change after creation
4. **Connection limits**: Max 50 incoming/outgoing edges per node

### Business Rules
1. **Graph creation**: Users can create unlimited private graphs
2. **Public graphs**: Limited to 3 per free user
3. **Node merging**: Only allowed for nodes with compatible metadata
4. **Batch operations**: Limited to 100 nodes/edges per operation
5. **Archive before delete**: Nodes must be archived before deletion