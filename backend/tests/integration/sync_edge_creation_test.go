package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"backend/application/commands"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncEdgeCreation(t *testing.T) {
	// Load configuration
	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	// Initialize the application container
	container, err := di.InitializeContainer()
	require.NoError(t, err)

	ctx := context.Background()
	commandBus := container.CommandBus
	nodeRepo := container.NodeRepository
	edgeRepo := container.EdgeRepository

	// Create a test user ID
	userID := "test-user-" + time.Now().Format("20060102150405")

	// Create first node
	createCmd1 := commands.CreateNodeCommand{
		NodeID:  "node1",
		UserID:  userID,
		Title:   "Test Node 1 - Machine Learning",
		Content: "This node is about machine learning, neural networks, deep learning, and artificial intelligence.",
		Format:  "text",
		X:       1.0,
		Y:       1.0,
		Z:       0.0,
		Tags:    []string{"ml", "ai"},
	}

	err = commandBus.Send(ctx, createCmd1)
	require.NoError(t, err)

	// Create second node
	createCmd2 := commands.CreateNodeCommand{
		NodeID:  "node2",
		UserID:  userID,
		Title:   "Test Node 2 - Deep Learning",
		Content: "This node focuses on deep learning, convolutional neural networks, and computer vision.",
		Format:  "text",
		X:       2.0,
		Y:       2.0,
		Z:       0.0,
		Tags:    []string{"deep-learning", "cnn"},
	}

	err = commandBus.Send(ctx, createCmd2)
	require.NoError(t, err)

	// Create third node - this should trigger edge discovery
	createCmd3 := commands.CreateNodeCommand{
		NodeID:  "node3",
		UserID:  userID,
		Title:   "Test Node 3 - Neural Networks",
		Content: "Neural networks, backpropagation, gradient descent, and machine learning fundamentals.",
		Format:  "text",
		X:       3.0,
		Y:       3.0,
		Z:       0.0,
		Tags:    []string{"neural-networks", "ml"},
	}

	err = commandBus.Send(ctx, createCmd3)
	require.NoError(t, err)

	// Give a small delay for edge creation to complete
	time.Sleep(500 * time.Millisecond)

	// Check that edges were created synchronously
	// Get the graph ID (assuming default graph)
	graphs, err := container.GraphRepository.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, graphs, "User should have at least one graph")

	graphID := graphs[0].ID().String()

	// Get edges for the graph
	edges, err := edgeRepo.GetByGraphID(ctx, graphID)
	require.NoError(t, err)

	// Assert that edges were created
	assert.Greater(t, len(edges), 0, "Edges should have been created synchronously")

	// Log the edges for debugging
	t.Logf("Created %d edges synchronously", len(edges))
	for _, edge := range edges {
		t.Logf("Edge: %s -> %s (weight: %.2f)", edge.SourceID, edge.TargetID, edge.Weight)
	}

	// Verify that the edges connect related nodes (high similarity content)
	// We expect edges between nodes with similar content
	hasRelevantEdge := false
	for _, edge := range edges {
		// Check if any edge connects nodes about similar topics
		if (edge.SourceID == "node3" && (edge.TargetID == "node1" || edge.TargetID == "node2")) ||
			(edge.TargetID == "node3" && (edge.SourceID == "node1" || edge.SourceID == "node2")) {
			hasRelevantEdge = true
			break
		}
	}

	assert.True(t, hasRelevantEdge, "Should have edges connecting nodes with similar content")

	// Clean up - delete test nodes
	for _, nodeID := range []string{"node1", "node2", "node3"} {
		deleteCmd := commands.DeleteNodeCommand{
			UserID: userID,
			NodeID: nodeID,
		}
		_ = commandBus.Send(ctx, deleteCmd)
	}
}

func TestSyncEdgeLimitRespected(t *testing.T) {
	// Load configuration
	cfg, err := config.LoadConfig()
	require.NoError(t, err)

	// Verify sync edge limit is configured
	assert.Equal(t, 20, cfg.EdgeCreation.SyncEdgeLimit, "Sync edge limit should be 20")

	// Initialize the application container
	container, err := di.InitializeContainer()
	require.NoError(t, err)

	ctx := context.Background()
	commandBus := container.CommandBus
	edgeRepo := container.EdgeRepository

	// Create a test user ID
	userID := "test-user-limit-" + time.Now().Format("20060102150405")

	// Create 25 nodes to potentially exceed the sync limit
	for i := 1; i <= 25; i++ {
		createCmd := commands.CreateNodeCommand{
			NodeID:  fmt.Sprintf("node%d", i),
			UserID:  userID,
			Title:   fmt.Sprintf("Test Node %d - Similar Content", i),
			Content: "This node has similar content about programming, software, and development.",
			Format:  "text",
			X:       float64(i),
			Y:       float64(i),
			Z:       0.0,
			Tags:    []string{"programming"},
		}

		err = commandBus.Send(ctx, createCmd)
		require.NoError(t, err)

		// Small delay to avoid overwhelming the system
		time.Sleep(50 * time.Millisecond)
	}

	// Give time for any async processing
	time.Sleep(1 * time.Second)

	// Get the graph
	graphs, err := container.GraphRepository.GetByUserID(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, graphs)

	graphID := graphs[0].ID().String()

	// Get edges created for the last node
	edges, err := edgeRepo.GetByNodeID(ctx, "node25")
	require.NoError(t, err)

	// The number of sync edges for a single node should not exceed the limit
	assert.LessOrEqual(t, len(edges), cfg.EdgeCreation.SyncEdgeLimit,
		"Number of sync edges should not exceed the configured limit")

	t.Logf("Node 25 has %d edges (limit: %d)", len(edges), cfg.EdgeCreation.SyncEdgeLimit)

	// Clean up
	bulkDelete := commands.BulkDeleteNodesCommand{
		OperationID: "cleanup",
		UserID:      userID,
		NodeIDs:     make([]string, 25),
	}
	for i := 1; i <= 25; i++ {
		bulkDelete.NodeIDs[i-1] = fmt.Sprintf("node%d", i)
	}
	_ = commandBus.Send(ctx, bulkDelete)
}