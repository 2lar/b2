package integration

import (
	"context"
	"testing"
	"time"

	"backend/domain/config"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockNodeLoader implements the NodeLoader interface for testing
type MockNodeLoader struct {
	nodes map[valueobjects.NodeID]*entities.Node
}

func NewMockNodeLoader() *MockNodeLoader {
	return &MockNodeLoader{
		nodes: make(map[valueobjects.NodeID]*entities.Node),
	}
}

func (m *MockNodeLoader) LoadNode(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error) {
	if node, exists := m.nodes[nodeID]; exists {
		return node, nil
	}
	return nil, nil
}

func (m *MockNodeLoader) LoadNodes(ctx context.Context, nodeIDs []valueobjects.NodeID) ([]*entities.Node, error) {
	result := make([]*entities.Node, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if node, exists := m.nodes[id]; exists {
			result = append(result, node)
		}
	}
	return result, nil
}

// MockEdgeLoader implements the EdgeLoader interface for testing
type MockEdgeLoader struct {
	edges map[string]*aggregates.Edge
}

func NewMockEdgeLoader() *MockEdgeLoader {
	return &MockEdgeLoader{
		edges: make(map[string]*aggregates.Edge),
	}
}

func (m *MockEdgeLoader) LoadEdge(ctx context.Context, edgeKey string) (*aggregates.Edge, error) {
	if edge, exists := m.edges[edgeKey]; exists {
		return edge, nil
	}
	return nil, nil
}

func (m *MockEdgeLoader) LoadEdges(ctx context.Context, edgeKeys []string) ([]*aggregates.Edge, error) {
	result := make([]*aggregates.Edge, 0, len(edgeKeys))
	for _, key := range edgeKeys {
		if edge, exists := m.edges[key]; exists {
			result = append(result, edge)
		}
	}
	return result, nil
}

func (m *MockEdgeLoader) LoadEdgesByNodeID(ctx context.Context, nodeID valueobjects.NodeID) ([]*aggregates.Edge, error) {
	result := make([]*aggregates.Edge, 0)
	for _, edge := range m.edges {
		if edge.SourceID.Equals(nodeID) || edge.TargetID.Equals(nodeID) {
			result = append(result, edge)
		}
	}
	return result, nil
}

func TestGraphLazy_Creation(t *testing.T) {
	// Test creating a new lazy graph
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, "user123", graph.UserID())
	assert.Equal(t, "Test Graph", graph.Name())
	assert.Equal(t, 0, graph.NodeCount())
	assert.Equal(t, 0, graph.EdgeCount())
}

func TestGraphLazy_AddNodeID(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	// Add node IDs without loading full nodes
	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()

	err = graph.AddNodeID(nodeID1)
	assert.NoError(t, err)
	assert.Equal(t, 1, graph.NodeCount())
	assert.True(t, graph.HasNode(nodeID1))

	err = graph.AddNodeID(nodeID2)
	assert.NoError(t, err)
	assert.Equal(t, 2, graph.NodeCount())
	assert.True(t, graph.HasNode(nodeID2))

	// Try adding duplicate
	err = graph.AddNodeID(nodeID1)
	assert.Error(t, err)
	assert.Equal(t, 2, graph.NodeCount())
}

func TestGraphLazy_NodeLimitEnforcement(t *testing.T) {
	// Create graph with low node limit
	cfg := &config.DomainConfig{
		MaxNodesPerGraph: 3,
	}
	graph, err := aggregates.NewGraphLazyWithConfig("user123", "Limited Graph", cfg)
	require.NoError(t, err)

	// Add nodes up to limit
	for i := 0; i < 3; i++ {
		err = graph.AddNodeID(valueobjects.NewNodeID())
		assert.NoError(t, err)
	}

	// Try to exceed limit
	err = graph.AddNodeID(valueobjects.NewNodeID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum nodes reached")
	assert.Equal(t, 3, graph.NodeCount())
}

func TestGraphLazy_EdgeManagement(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()
	nodeID3 := valueobjects.NewNodeID()

	// Add nodes first
	err = graph.AddNodeID(nodeID1)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID2)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID3)
	require.NoError(t, err)

	// Add edges
	err = graph.AddEdgeKey(nodeID1, nodeID2)
	assert.NoError(t, err)
	assert.Equal(t, 1, graph.EdgeCount())

	err = graph.AddEdgeKey(nodeID2, nodeID3)
	assert.NoError(t, err)
	assert.Equal(t, 2, graph.EdgeCount())

	// Try adding duplicate edge
	err = graph.AddEdgeKey(nodeID1, nodeID2)
	assert.Error(t, err)
	assert.Equal(t, 2, graph.EdgeCount())

	// Try self-reference
	err = graph.AddEdgeKey(nodeID1, nodeID1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect node to itself")

	// Try edge with non-existent node
	err = graph.AddEdgeKey(nodeID1, valueobjects.NewNodeID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both nodes must exist")
}

func TestGraphLazy_LazyLoading(t *testing.T) {
	ctx := context.Background()
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	// Create mock loaders
	nodeLoader := NewMockNodeLoader()
	edgeLoader := NewMockEdgeLoader()
	graph.SetLoaders(nodeLoader, edgeLoader)

	// Create test nodes
	content1, _ := valueobjects.NewNodeContent("Node 1", "Content 1", valueobjects.FormatPlainText)
	content2, _ := valueobjects.NewNodeContent("Node 2", "Content 2", valueobjects.FormatPlainText)
	position, _ := valueobjects.NewPosition3D(0, 0, 0)

	node1, _ := entities.NewNode("user123", content1, position)
	node2, _ := entities.NewNode("user123", content2, position)

	// Add to mock loader
	nodeLoader.nodes[node1.ID()] = node1
	nodeLoader.nodes[node2.ID()] = node2

	// Add node IDs to graph
	err = graph.AddNodeID(node1.ID())
	require.NoError(t, err)
	err = graph.AddNodeID(node2.ID())
	require.NoError(t, err)

	// Load single node on demand
	loadedNode, err := graph.GetNode(ctx, node1.ID())
	assert.NoError(t, err)
	assert.NotNil(t, loadedNode)
	assert.Equal(t, "Node 1", loadedNode.Content().Title())

	// Load nodes with pagination
	nodes, hasMore, err := graph.GetNodesPaginated(ctx, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, nodes, 2)
	assert.False(t, hasMore)
}

func TestGraphLazy_RemoveNode(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()
	nodeID3 := valueobjects.NewNodeID()

	// Add nodes
	err = graph.AddNodeID(nodeID1)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID2)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID3)
	require.NoError(t, err)

	// Add edges
	err = graph.AddEdgeKey(nodeID1, nodeID2)
	require.NoError(t, err)
	err = graph.AddEdgeKey(nodeID2, nodeID3)
	require.NoError(t, err)

	assert.Equal(t, 3, graph.NodeCount())
	assert.Equal(t, 2, graph.EdgeCount())

	// Remove node2 (should also remove its edges)
	err = graph.RemoveNodeID(nodeID2)
	assert.NoError(t, err)
	assert.Equal(t, 2, graph.NodeCount())
	assert.Equal(t, 0, graph.EdgeCount()) // Both edges involved node2
	assert.False(t, graph.HasNode(nodeID2))

	// Try removing non-existent node
	err = graph.RemoveNodeID(valueobjects.NewNodeID())
	assert.Error(t, err)
}

func TestGraphLazy_Events(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	// Initially should have creation event
	events := graph.GetUncommittedEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, "graph.created", events[0].GetEventType())

	// Clear events
	graph.MarkEventsAsCommitted()
	events = graph.GetUncommittedEvents()
	assert.Len(t, events, 0)

	// Add node - should generate event
	nodeID := valueobjects.NewNodeID()
	err = graph.AddNodeID(nodeID)
	require.NoError(t, err)

	events = graph.GetUncommittedEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, "graph.node_added", events[0].GetEventType())

	// Remove node - should generate event
	graph.MarkEventsAsCommitted()
	err = graph.RemoveNodeID(nodeID)
	require.NoError(t, err)

	events = graph.GetUncommittedEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, "graph.node_removed", events[0].GetEventType())
}

func TestGraphLazy_Reconstruction(t *testing.T) {
	// Create node IDs
	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()
	
	// Create edge keys
	edgeKeys := []string{
		nodeID1.String() + "->" + nodeID2.String(),
	}

	// Create metadata
	metadata := aggregates.GraphMetadata{
		NodeCount: 2,
		EdgeCount: 1,
		ViewSettings: aggregates.ViewSettings{
			Layout:     aggregates.LayoutForceDirected,
			ShowLabels: true,
		},
	}

	// Reconstruct graph
	graph, err := aggregates.ReconstructGraphLazy(
		"graph123",
		"user123",
		"Reconstructed Graph",
		"Test description",
		[]valueobjects.NodeID{nodeID1, nodeID2},
		edgeKeys,
		metadata,
		time.Now().Add(-24*time.Hour),
		time.Now(),
		5,
	)

	require.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, "graph123", graph.ID().String())
	assert.Equal(t, "user123", graph.UserID())
	assert.Equal(t, "Reconstructed Graph", graph.Name())
	assert.Equal(t, 2, graph.NodeCount())
	assert.Equal(t, 1, graph.EdgeCount())
	assert.True(t, graph.HasNode(nodeID1))
	assert.True(t, graph.HasNode(nodeID2))
}

func TestGraphLazy_Validation(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	// Initially valid
	err = graph.Validate()
	assert.NoError(t, err)

	// Add nodes and validate
	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()
	
	err = graph.AddNodeID(nodeID1)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID2)
	require.NoError(t, err)

	err = graph.Validate()
	assert.NoError(t, err)

	// Add edge and validate
	err = graph.AddEdgeKey(nodeID1, nodeID2)
	require.NoError(t, err)

	err = graph.Validate()
	assert.NoError(t, err)
}

func TestGraphLazy_NodeConnectivity(t *testing.T) {
	graph, err := aggregates.NewGraphLazy("user123", "Test Graph")
	require.NoError(t, err)

	nodeID1 := valueobjects.NewNodeID()
	nodeID2 := valueobjects.NewNodeID()
	nodeID3 := valueobjects.NewNodeID()
	nodeID4 := valueobjects.NewNodeID()

	// Add nodes
	err = graph.AddNodeID(nodeID1)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID2)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID3)
	require.NoError(t, err)
	err = graph.AddNodeID(nodeID4)
	require.NoError(t, err)

	// Create a hub node (node2 connects to all others)
	err = graph.AddEdgeKey(nodeID2, nodeID1)
	require.NoError(t, err)
	err = graph.AddEdgeKey(nodeID2, nodeID3)
	require.NoError(t, err)
	err = graph.AddEdgeKey(nodeID2, nodeID4)
	require.NoError(t, err)

	// Check connectivity
	connectivity := graph.GetNodeConnectivity(nodeID2)
	assert.Equal(t, 3, connectivity) // node2 is connected to 3 edges

	connectivity = graph.GetNodeConnectivity(nodeID1)
	assert.Equal(t, 1, connectivity) // node1 has 1 edge

	connectivity = graph.GetNodeConnectivity(valueobjects.NewNodeID())
	assert.Equal(t, 0, connectivity) // non-existent node has 0 edges
}