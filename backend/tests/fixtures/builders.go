package fixtures

import (
	"fmt"
	"time"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	"github.com/google/uuid"
)

// NodeBuilder helps create test nodes with default values
type NodeBuilder struct {
	id       valueobjects.NodeID
	userID   string
	graphID  string
	title    string
	content  string
	format   valueobjects.ContentFormat
	x, y, z  float64
	status   string
	tags     []string
	metadata map[string]interface{}
}

func NewNodeBuilder() *NodeBuilder {
	return &NodeBuilder{
		id:      valueobjects.NewNodeID(),
		userID:  "test-user-123",
		graphID: "test-graph-123",
		title:   "Test Node",
		content: "Test content",
		format:  valueobjects.FormatMarkdown,
		x:       0,
		y:       0,
		z:       0,
		status:  "published",
		tags:    []string{"test"},
	}
}

func (b *NodeBuilder) WithID(id string) *NodeBuilder {
	b.id, _ = valueobjects.NewNodeIDFromString(id)
	return b
}

func (b *NodeBuilder) WithUserID(userID string) *NodeBuilder {
	b.userID = userID
	return b
}

func (b *NodeBuilder) WithGraphID(graphID string) *NodeBuilder {
	b.graphID = graphID
	return b
}

func (b *NodeBuilder) WithTitle(title string) *NodeBuilder {
	b.title = title
	return b
}

func (b *NodeBuilder) WithContent(content string) *NodeBuilder {
	b.content = content
	return b
}

func (b *NodeBuilder) WithPosition(x, y, z float64) *NodeBuilder {
	b.x, b.y, b.z = x, y, z
	return b
}

func (b *NodeBuilder) WithStatus(status string) *NodeBuilder {
	b.status = status
	return b
}

func (b *NodeBuilder) WithTags(tags ...string) *NodeBuilder {
	b.tags = tags
	return b
}

func (b *NodeBuilder) Build() (*entities.Node, error) {
	content, err := valueobjects.NewNodeContent(b.title, b.content, b.format)
	if err != nil {
		return nil, err
	}

	position, err := valueobjects.NewPosition3D(b.x, b.y, b.z)
	if err != nil {
		return nil, err
	}

	node, err := entities.NewNode(b.userID, content, position)
	if err != nil {
		return nil, err
	}

	node.SetGraphID(b.graphID)
	for _, tag := range b.tags {
		node.AddTag(tag)
	}

	return node, nil
}

func (b *NodeBuilder) MustBuild() *entities.Node {
	node, err := b.Build()
	if err != nil {
		panic(err)
	}
	// Mark creation events as committed so tests don't see them
	node.MarkEventsAsCommitted()
	return node
}

// GraphBuilder helps create test graphs
type GraphBuilder struct {
	id          string
	userID      string
	name        string
	description string
	isDefault   bool
	isPublic    bool
	nodes       []*entities.Node
	edges       []*aggregates.Edge
}

func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		id:          uuid.New().String(),
		userID:      "test-user-123",
		name:        "Test Graph",
		description: "Test graph description",
		isDefault:   false,
		isPublic:    false,
		nodes:       []*entities.Node{},
		edges:       []*aggregates.Edge{},
	}
}

func (b *GraphBuilder) WithID(id string) *GraphBuilder {
	b.id = id
	return b
}

func (b *GraphBuilder) WithUserID(userID string) *GraphBuilder {
	b.userID = userID
	return b
}

func (b *GraphBuilder) WithName(name string) *GraphBuilder {
	b.name = name
	return b
}

func (b *GraphBuilder) WithDescription(desc string) *GraphBuilder {
	b.description = desc
	return b
}

func (b *GraphBuilder) AsDefault() *GraphBuilder {
	b.isDefault = true
	return b
}

func (b *GraphBuilder) AsPublic() *GraphBuilder {
	b.isPublic = true
	return b
}

func (b *GraphBuilder) WithNodes(nodes ...*entities.Node) *GraphBuilder {
	b.nodes = append(b.nodes, nodes...)
	return b
}

func (b *GraphBuilder) Build() (*aggregates.Graph, error) {
	var graph *aggregates.Graph
	var err error

	// If we have explicit ID or description, use ReconstructGraph
	if b.id != "" || b.description != "" {
		id := b.id
		if id == "" {
			id = uuid.New().String()
		}
		description := b.description
		if description == "" {
			description = "Knowledge graph for " + b.name
		}

		// Use current time for created/updated timestamps in test fixtures
		now := time.Now().Format(time.RFC3339)
		graph, err = aggregates.ReconstructGraph(
			id,
			b.userID,
			b.name,
			description,
			b.isDefault,
			now,
			now,
		)
	} else {
		// Use regular constructor for simple cases
		graph, err = aggregates.NewGraph(b.userID, b.name)
	}

	if err != nil {
		return nil, err
	}

	// Add nodes
	for _, node := range b.nodes {
		if err := graph.AddNode(node); err != nil {
			return nil, err
		}
	}

	return graph, nil
}

func (b *GraphBuilder) MustBuild() *aggregates.Graph {
	graph, err := b.Build()
	if err != nil {
		panic(err)
	}
	// Mark creation events as committed so tests don't see them
	graph.MarkEventsAsCommitted()
	return graph
}

// EdgeBuilder helps create test edges
type EdgeBuilder struct {
	id            string
	sourceID      valueobjects.NodeID
	targetID      valueobjects.NodeID
	edgeType      entities.EdgeType
	weight        float64
	bidirectional bool
	metadata      map[string]interface{}
}

func NewEdgeBuilder() *EdgeBuilder {
	return &EdgeBuilder{
		id:            uuid.New().String(),
		sourceID:      valueobjects.NewNodeID(),
		targetID:      valueobjects.NewNodeID(),
		edgeType:      entities.EdgeTypeNormal,
		weight:        1.0,
		bidirectional: false,
		metadata:      make(map[string]interface{}),
	}
}

func (b *EdgeBuilder) WithID(id string) *EdgeBuilder {
	b.id = id
	return b
}

func (b *EdgeBuilder) WithSource(nodeID valueobjects.NodeID) *EdgeBuilder {
	b.sourceID = nodeID
	return b
}

func (b *EdgeBuilder) WithTarget(nodeID valueobjects.NodeID) *EdgeBuilder {
	b.targetID = nodeID
	return b
}

func (b *EdgeBuilder) WithType(edgeType entities.EdgeType) *EdgeBuilder {
	b.edgeType = edgeType
	return b
}

func (b *EdgeBuilder) WithWeight(weight float64) *EdgeBuilder {
	b.weight = weight
	return b
}

func (b *EdgeBuilder) AsBidirectional() *EdgeBuilder {
	b.bidirectional = true
	return b
}

func (b *EdgeBuilder) Build() *aggregates.Edge {
	return &aggregates.Edge{
		ID:            b.id,
		SourceID:      b.sourceID,
		TargetID:      b.targetID,
		Type:          b.edgeType,
		Weight:        b.weight,
		Bidirectional: b.bidirectional,
		Metadata:      b.metadata,
		CreatedAt:     time.Now(),
	}
}

// EventBuilder helps create test events
type EventBuilder struct {
	eventType    string
	aggregateID  string
	userID       string
	version      int
	data         map[string]interface{}
	occurredAt   time.Time
}

func NewEventBuilder(eventType string) *EventBuilder {
	return &EventBuilder{
		eventType:   eventType,
		aggregateID: uuid.New().String(),
		userID:      "test-user-123",
		version:     1,
		data:        make(map[string]interface{}),
		occurredAt:  time.Now(),
	}
}

func (b *EventBuilder) WithAggregateID(id string) *EventBuilder {
	b.aggregateID = id
	return b
}

func (b *EventBuilder) WithUserID(userID string) *EventBuilder {
	b.userID = userID
	return b
}

func (b *EventBuilder) WithVersion(version int) *EventBuilder {
	b.version = version
	return b
}

func (b *EventBuilder) WithData(key string, value interface{}) *EventBuilder {
	b.data[key] = value
	return b
}

func (b *EventBuilder) BuildNodeCreatedEvent() *events.NodeCreatedEvent {
	return &events.NodeCreatedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			EventType:   "NodeCreated",
			Timestamp:   b.occurredAt,
			Version:     b.version,
		},
		NodeID:  b.aggregateID,
		GraphID: b.data["graphID"].(string),
		Title:   b.data["title"].(string),
		Content: b.data["content"].(string),
	}
}

func (b *EventBuilder) BuildEdgeCreatedEvent() *events.EdgeCreatedEvent {
	return &events.EdgeCreatedEvent{
		BaseEvent: events.BaseEvent{
			AggregateID: b.aggregateID,
			EventType:   "EdgeCreated",
			Timestamp:   b.occurredAt,
			Version:     b.version,
		},
		EdgeID:   b.aggregateID,
		GraphID:  b.data["graphID"].(string),
		SourceID: b.data["sourceID"].(string),
		TargetID: b.data["targetID"].(string),
	}
}

// TestData provides pre-built test objects
type TestData struct {
	UserID       string
	GraphID      string
	DefaultGraph *aggregates.Graph
	Nodes        []*entities.Node
	Edges        []*aggregates.Edge
}

// CreateTestData creates a complete test dataset
func CreateTestData() *TestData {
	userID := "test-user-" + uuid.New().String()[:8]
	graphID := "test-graph-" + uuid.New().String()[:8]

	// Create nodes
	nodes := make([]*entities.Node, 5)
	for i := 0; i < 5; i++ {
		nodes[i] = NewNodeBuilder().
			WithUserID(userID).
			WithGraphID(graphID).
			WithTitle(fmt.Sprintf("Node %d", i+1)).
			WithContent(fmt.Sprintf("Content for node %d", i+1)).
			WithPosition(float64(i*100), float64(i*100), 0).
			WithTags("test", fmt.Sprintf("node%d", i+1)).
			MustBuild()
	}

	// Create edges
	edges := make([]*aggregates.Edge, 4)
	for i := 0; i < 4; i++ {
		edges[i] = NewEdgeBuilder().
			WithSource(nodes[i].ID()).
			WithTarget(nodes[i+1].ID()).
			WithType(entities.EdgeTypeNormal).
			Build()
	}

	// Create graph
	graph := NewGraphBuilder().
		WithID(graphID).
		WithUserID(userID).
		WithName("Test Graph").
		AsDefault().
		WithNodes(nodes...).
		MustBuild()

	return &TestData{
		UserID:       userID,
		GraphID:      graphID,
		DefaultGraph: graph,
		Nodes:        nodes,
		Edges:        edges,
	}
}

// CreateSimpleTestGraph creates a simple graph with 3 nodes and 2 edges
func CreateSimpleTestGraph(userID string) (*aggregates.Graph, []*entities.Node, []*aggregates.Edge) {
	nodes := []*entities.Node{
		NewNodeBuilder().WithUserID(userID).WithTitle("Node 1").MustBuild(),
		NewNodeBuilder().WithUserID(userID).WithTitle("Node 2").MustBuild(),
		NewNodeBuilder().WithUserID(userID).WithTitle("Node 3").MustBuild(),
	}

	edges := []*aggregates.Edge{
		NewEdgeBuilder().WithSource(nodes[0].ID()).WithTarget(nodes[1].ID()).Build(),
		NewEdgeBuilder().WithSource(nodes[1].ID()).WithTarget(nodes[2].ID()).Build(),
	}

	graph := NewGraphBuilder().
		WithUserID(userID).
		WithNodes(nodes...).
		MustBuild()

	return graph, nodes, edges
}

// CreateComplexTestGraph creates a complex graph with hub-and-spoke pattern
func CreateComplexTestGraph(userID string) (*aggregates.Graph, []*entities.Node, []*aggregates.Edge) {
	hub := NewNodeBuilder().WithUserID(userID).WithTitle("Hub").MustBuild()

	spokes := make([]*entities.Node, 8)
	for i := 0; i < 8; i++ {
		spokes[i] = NewNodeBuilder().
			WithUserID(userID).
			WithTitle(fmt.Sprintf("Spoke %d", i+1)).
			MustBuild()
	}

	edges := make([]*aggregates.Edge, 8)
	for i := 0; i < 8; i++ {
		edges[i] = NewEdgeBuilder().
			WithSource(hub.ID()).
			WithTarget(spokes[i].ID()).
			Build()
	}

	allNodes := append([]*entities.Node{hub}, spokes...)

	graph := NewGraphBuilder().
		WithUserID(userID).
		WithNodes(allNodes...).
		MustBuild()

	return graph, allNodes, edges
}