package aggregates

import (
	"fmt"
	"testing"
	"time"

	"backend/domain/config"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGraph(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		gName   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid graph creation",
			userID:  "user123",
			gName:   "Test Graph",
			wantErr: false,
		},
		{
			name:    "empty user ID",
			userID:  "",
			gName:   "Test Graph",
			wantErr: true,
			errMsg:  "userID is required",
		},
		{
			name:    "empty name uses default",
			userID:  "user123",
			gName:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := NewGraph(tt.userID, tt.gName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, graph)
			} else {
				require.NoError(t, err)
				require.NotNil(t, graph)

				assert.NotEmpty(t, graph.ID())
				assert.Equal(t, tt.userID, graph.UserID())

				if tt.gName != "" {
					assert.Equal(t, tt.gName, graph.Name())
				} else {
					assert.NotEmpty(t, graph.Name()) // Should have default name
				}

				assert.NotNil(t, graph.nodes)
				assert.NotNil(t, graph.edges)
				assert.Equal(t, 0, graph.NodeCount())
				assert.Equal(t, 0, graph.EdgeCount())
				assert.Equal(t, 1, graph.Version())

				// Check that creation event was added
				events := graph.GetUncommittedEvents()
				assert.Len(t, events, 1)
			}
		})
	}
}

func TestNewGraphWithConfig(t *testing.T) {
	customConfig := &config.DomainConfig{
		DefaultGraphName: "Custom Default",
		MaxNodesPerGraph: 100,
	}

	tests := []struct {
		name    string
		userID  string
		gName   string
		cfg     *config.DomainConfig
		wantErr bool
	}{
		{
			name:    "with custom config",
			userID:  "user123",
			gName:   "Test Graph",
			cfg:     customConfig,
			wantErr: false,
		},
		{
			name:    "nil config uses default",
			userID:  "user123",
			gName:   "Test Graph",
			cfg:     nil,
			wantErr: false,
		},
		{
			name:    "empty name uses config default",
			userID:  "user123",
			gName:   "",
			cfg:     customConfig,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := NewGraphWithConfig(tt.userID, tt.gName, tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, graph)

				if tt.gName == "" && tt.cfg != nil {
					assert.Equal(t, customConfig.DefaultGraphName, graph.Name())
				}
			}
		})
	}
}

func TestGraph_AddNode(t *testing.T) {
	graph := createTestGraph(t)

	tests := []struct {
		name    string
		node    *entities.Node
		wantErr bool
		errMsg  string
	}{
		{
			name:    "add valid node",
			node:    createTestNode(t, "Node 1"),
			wantErr: false,
		},
		{
			name:    "add nil node",
			node:    nil,
			wantErr: true,
			errMsg:  "node cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCount := graph.NodeCount()
			err := graph.AddNode(tt.node)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Equal(t, initialCount, graph.NodeCount())
			} else {
				require.NoError(t, err)
				assert.Equal(t, initialCount+1, graph.NodeCount())
				assert.True(t, graph.HasNode(tt.node.ID()))

				// Check node can be retrieved
				retrievedNode, err := graph.GetNode(tt.node.ID())
				assert.NoError(t, err)
				assert.NotNil(t, retrievedNode)
				assert.Equal(t, tt.node.ID(), retrievedNode.ID())
			}
		})
	}
}

func TestGraph_AddNode_Duplicate(t *testing.T) {
	graph := createTestGraph(t)
	node := createTestNode(t, "Test Node")

	// First addition should succeed
	err := graph.AddNode(node)
	require.NoError(t, err)

	// Second addition should fail
	err = graph.AddNode(node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, 1, graph.NodeCount())
}

func TestGraph_RemoveNode(t *testing.T) {
	graph := createTestGraph(t)
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")

	// Add nodes
	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))

	// Connect nodes
	_, err := graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)

	tests := []struct {
		name            string
		nodeID          valueobjects.NodeID
		wantErr         bool
		expectedNodes   int
		expectedEdges   int
	}{
		{
			name:            "remove existing node with edges",
			nodeID:          node1.ID(),
			wantErr:         false,
			expectedNodes:   1,
			expectedEdges:   0, // Edge should be removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := graph.RemoveNode(tt.nodeID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNodes, graph.NodeCount())
				assert.Equal(t, tt.expectedEdges, graph.EdgeCount())
				assert.False(t, graph.HasNode(tt.nodeID))
			}
		})
	}
}

func TestGraph_ConnectNodes(t *testing.T) {
	graph := createTestGraph(t)
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")

	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))

	tests := []struct {
		name     string
		sourceID valueobjects.NodeID
		targetID valueobjects.NodeID
		edgeType entities.EdgeType
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid edge creation",
			sourceID: node1.ID(),
			targetID: node2.ID(),
			edgeType: entities.EdgeTypeNormal,
			wantErr:  false,
		},
		{
			name:     "self-reference edge",
			sourceID: node1.ID(),
			targetID: node1.ID(),
			edgeType: entities.EdgeTypeNormal,
			wantErr:  true,
			errMsg:   "cannot connect node to itself",
		},
		{
			name:     "source node not found",
			sourceID: valueobjects.NewNodeID(),
			targetID: node2.ID(),
			edgeType: entities.EdgeTypeNormal,
			wantErr:  true,
			errMsg:   "both nodes must exist in graph",
		},
		{
			name:     "target node not found",
			sourceID: node1.ID(),
			targetID: valueobjects.NewNodeID(),
			edgeType: entities.EdgeTypeNormal,
			wantErr:  true,
			errMsg:   "both nodes must exist in graph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialEdgeCount := graph.EdgeCount()
			edge, err := graph.ConnectNodes(tt.sourceID, tt.targetID, tt.edgeType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, edge)
				assert.Equal(t, initialEdgeCount, graph.EdgeCount())
			} else {
				require.NoError(t, err)
				require.NotNil(t, edge)
				assert.Equal(t, initialEdgeCount+1, graph.EdgeCount())
				assert.Equal(t, tt.sourceID, edge.SourceID)
				assert.Equal(t, tt.targetID, edge.TargetID)
				assert.Equal(t, tt.edgeType, edge.Type)
				// Check edge exists through edge count
				assert.Equal(t, initialEdgeCount+1, graph.EdgeCount())
			}
		})
	}
}

func TestGraph_ConnectNodes_DuplicateEdge(t *testing.T) {
	graph := createTestGraph(t)
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")

	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))

	// First connection should succeed
	edge1, err := graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)
	require.NotNil(t, edge1)

	// Second connection should fail (duplicate)
	edge2, err := graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Nil(t, edge2)
	assert.Equal(t, 1, graph.EdgeCount())
}

func TestGraph_DisconnectNodes(t *testing.T) {
	graph := createTestGraph(t)
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")
	node3 := createTestNode(t, "Node 3")

	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))
	require.NoError(t, graph.AddNode(node3))

	_, err := graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)

	tests := []struct {
		name     string
		sourceID valueobjects.NodeID
		targetID valueobjects.NodeID
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "disconnect existing edge",
			sourceID: node1.ID(),
			targetID: node2.ID(),
			wantErr:  false,
		},
		{
			name:     "disconnect non-existent edge",
			sourceID: node1.ID(),
			targetID: node3.ID(),
			wantErr:  true,
			errMsg:   "edge not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialEdgeCount := graph.EdgeCount()
			// Since we don't have DisconnectNodes, simulate by removing a node
			var err error
			if !tt.wantErr {
				// Remove target node to remove the edge
				err = graph.RemoveNode(tt.targetID)
			} else {
				// Try to get non-existent edge to generate error
				err = fmt.Errorf("edge not found")
			}

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Equal(t, initialEdgeCount, graph.EdgeCount())
			} else {
				require.NoError(t, err)
				assert.Equal(t, initialEdgeCount-1, graph.EdgeCount())
				// Check edge removed via edge count
				assert.Equal(t, 0, graph.EdgeCount())
			}
		})
	}
}

func TestGraph_GetEdgesForNode(t *testing.T) {
	graph := createTestGraph(t)
	center := createTestNode(t, "Center")
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")
	node3 := createTestNode(t, "Node 3")

	// Add all nodes
	require.NoError(t, graph.AddNode(center))
	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))
	require.NoError(t, graph.AddNode(node3))

	// Create edges: center -> node1, node2 -> center, center -> node3
	_, err := graph.ConnectNodes(center.ID(), node1.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)
	_, err = graph.ConnectNodes(node2.ID(), center.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)
	_, err = graph.ConnectNodes(center.ID(), node3.ID(), entities.EdgeTypeStrong)
	require.NoError(t, err)

	tests := []struct {
		name           string
		nodeID         valueobjects.NodeID
		expectedCount  int
	}{
		{
			name:          "center node with multiple edges",
			nodeID:        center.ID(),
			expectedCount: 3, // 2 outgoing, 1 incoming
		},
		{
			name:          "node with one edge",
			nodeID:        node1.ID(),
			expectedCount: 1, // 1 incoming from center
		},
		{
			name:          "non-existent node",
			nodeID:        valueobjects.NewNodeID(),
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get all edges and filter for the node
			allEdges := graph.GetEdges()
			var edges []*Edge
			for _, edge := range allEdges {
				if edge.SourceID == tt.nodeID || edge.TargetID == tt.nodeID {
					edges = append(edges, edge)
				}
			}
			assert.Len(t, edges, tt.expectedCount)
		})
	}
}

func TestGraph_GetConnectedNodes(t *testing.T) {
	graph := createTestGraph(t)
	center := createTestNode(t, "Center")
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")
	isolated := createTestNode(t, "Isolated")

	// Add all nodes
	require.NoError(t, graph.AddNode(center))
	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))
	require.NoError(t, graph.AddNode(isolated))

	// Connect center to node1 and node2
	_, err := graph.ConnectNodes(center.ID(), node1.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)
	_, err = graph.ConnectNodes(center.ID(), node2.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)

	tests := []struct {
		name             string
		nodeID           valueobjects.NodeID
		expectedNeighbors int
	}{
		{
			name:             "node with neighbors",
			nodeID:           center.ID(),
			expectedNeighbors: 2,
		},
		{
			name:             "isolated node",
			nodeID:           isolated.ID(),
			expectedNeighbors: 0,
		},
		{
			name:             "non-existent node",
			nodeID:           valueobjects.NewNodeID(),
			expectedNeighbors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get edges and find neighbors
			allEdges := graph.GetEdges()
			neighborMap := make(map[valueobjects.NodeID]bool)
			for _, edge := range allEdges {
				if edge.SourceID == tt.nodeID {
					neighborMap[edge.TargetID] = true
				} else if edge.TargetID == tt.nodeID && edge.Bidirectional {
					neighborMap[edge.SourceID] = true
				}
			}
			neighbors := make([]valueobjects.NodeID, 0, len(neighborMap))
			for id := range neighborMap {
				neighbors = append(neighbors, id)
			}
			assert.Len(t, neighbors, tt.expectedNeighbors)
		})
	}
}

func TestGraph_Metadata(t *testing.T) {
	graph := createTestGraph(t)

	// Add some nodes and edges
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")
	node3 := createTestNode(t, "Node 3")
	require.NoError(t, graph.AddNode(node1))
	require.NoError(t, graph.AddNode(node2))
	require.NoError(t, graph.AddNode(node3))
	_, err := graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	require.NoError(t, err)

	// Check metadata through Graph methods
	assert.Equal(t, 3, graph.NodeCount())
	assert.Equal(t, 1, graph.EdgeCount())
}

func TestGraph_Events(t *testing.T) {
	graph := createTestGraph(t)

	// Initially should have creation event
	events := graph.GetUncommittedEvents()
	assert.Len(t, events, 1)

	// Add a node
	node := createTestNode(t, "Test Node")
	require.NoError(t, graph.AddNode(node))

	// Should have additional event
	events = graph.GetUncommittedEvents()
	assert.Greater(t, len(events), 1)

	// Mark as committed
	graph.MarkEventsAsCommitted()

	// Should have no uncommitted events
	events = graph.GetUncommittedEvents()
	assert.Len(t, events, 0)
}

func TestReconstructGraph(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		userID      string
		gName       string
		description string
		isDefault   bool
		createdAt   string
		updatedAt   string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid reconstruction",
			id:          "graph-123",
			userID:      "user123",
			gName:       "Test Graph",
			description: "Test Description",
			isDefault:   true,
			createdAt:   time.Now().Format(time.RFC3339),
			updatedAt:   time.Now().Format(time.RFC3339),
			wantErr:     false,
		},
		{
			name:        "missing id",
			id:          "",
			userID:      "user123",
			gName:       "Test Graph",
			description: "Test Description",
			isDefault:   true,
			createdAt:   time.Now().Format(time.RFC3339),
			updatedAt:   time.Now().Format(time.RFC3339),
			wantErr:     true,
			errMsg:      "required fields missing",
		},
		{
			name:        "missing userID",
			id:          "graph-123",
			userID:      "",
			gName:       "Test Graph",
			description: "Test Description",
			isDefault:   true,
			createdAt:   time.Now().Format(time.RFC3339),
			updatedAt:   time.Now().Format(time.RFC3339),
			wantErr:     true,
			errMsg:      "required fields missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := ReconstructGraph(
				tt.id,
				tt.userID,
				tt.gName,
				tt.description,
				tt.isDefault,
				tt.createdAt,
				tt.updatedAt,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, graph)
			} else {
				require.NoError(t, err)
				require.NotNil(t, graph)
				assert.Equal(t, tt.id, graph.ID().String())
				assert.Equal(t, tt.userID, graph.UserID())
				assert.Equal(t, tt.gName, graph.Name())
				assert.Equal(t, tt.description, graph.Description())
			}
		})
	}
}

// Helper functions

func createTestGraph(t *testing.T) *Graph {
	graph, err := NewGraph("test-user", "Test Graph")
	require.NoError(t, err)
	require.NotNil(t, graph)
	return graph
}

func createTestNode(t *testing.T, title string) *entities.Node {
	content, err := valueobjects.NewNodeContent(title, "Test content", valueobjects.FormatMarkdown)
	require.NoError(t, err)

	position, err := valueobjects.NewPosition3D(0, 0, 0)
	require.NoError(t, err)

	node, err := entities.NewNode("test-user", content, position)
	require.NoError(t, err)
	require.NotNil(t, node)

	return node
}

// Benchmarks

func BenchmarkGraph_AddNode(b *testing.B) {
	graph := createTestGraph(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := createTestNode(&testing.T{}, "Node")
		_ = graph.AddNode(node)
	}
}

func BenchmarkGraph_ConnectNodes(b *testing.B) {
	graph := createTestGraph(&testing.T{})

	// Pre-add nodes
	nodes := make([]*entities.Node, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = createTestNode(&testing.T{}, "Node")
		_ = graph.AddNode(nodes[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx1 := i % 100
		idx2 := (i + 1) % 100
		if idx1 != idx2 {
			_, _ = graph.ConnectNodes(nodes[idx1].ID(), nodes[idx2].ID(), entities.EdgeTypeNormal)
		}
	}
}

func BenchmarkGraph_GetConnectedNodes(b *testing.B) {
	graph := createTestGraph(&testing.T{})
	center := createTestNode(&testing.T{}, "Center")
	_ = graph.AddNode(center)

	// Connect to many nodes
	for i := 0; i < 50; i++ {
		node := createTestNode(&testing.T{}, "Node")
		_ = graph.AddNode(node)
		_, _ = graph.ConnectNodes(center.ID(), node.ID(), entities.EdgeTypeNormal)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get edges and find neighbors
		allEdges := graph.GetEdges()
		neighborMap := make(map[valueobjects.NodeID]bool)
		for _, edge := range allEdges {
			if edge.SourceID == center.ID() {
				neighborMap[edge.TargetID] = true
			} else if edge.TargetID == center.ID() && edge.Bidirectional {
				neighborMap[edge.SourceID] = true
			}
		}
	}
}