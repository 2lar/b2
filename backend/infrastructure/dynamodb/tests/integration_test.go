//go:build ignore

// Package tests provides integration tests for the DynamoDB infrastructure layer.
// These tests require a running DynamoDB instance (local or AWS).
package tests

import (
	"context"
	"os"
	"testing"
	"time"

	infraDynamoDB "brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"github.com/stretchr/testify/require"
)

const (
	testTableName = "brain2-test"
	testIndexName = "KeywordIndex"
)

// setupTestRepo creates a DynamoDB repository for testing.
// This assumes either:
// 1. AWS credentials are configured and a test table exists
// 2. DynamoDB Local is running on localhost:8000
func setupTestRepo(t *testing.T) repository.Repository {
	// Check if we should skip integration tests
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration tests")
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	require.NoError(t, err)

	// If LOCAL_DYNAMODB is set, use local endpoint
	if os.Getenv("LOCAL_DYNAMODB") == "true" {
		cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: "http://localhost:8000",
			}, nil
		})
	}

	client := dynamodb.NewFromConfig(cfg)
	logger, _ := zap.NewDevelopment()
	return infraDynamoDB.NewRepository(client, testTableName, testIndexName, logger)
}

func TestNodeOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repo := setupTestRepo(t)
	ctx := context.Background()
	userID := "test-user-" + uuid.New().String()

	t.Run("CreateAndRetrieveNode", func(t *testing.T) {
		// Create a test node
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "This is a test node for integration testing",
			Keywords:  []string{"test", "integration", "node"},
			Tags:      []string{"testing"},
			CreatedAt: time.Now(),
			Version:   0,
		}

		// Create the node
		err := repo.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		// Retrieve the node
		retrievedNode, err := repo.FindNodeByID(ctx, userID, node.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedNode)

		// Verify node data
		assert.Equal(t, node.ID, retrievedNode.ID)
		assert.Equal(t, node.UserID, retrievedNode.UserID)
		assert.Equal(t, node.Content, retrievedNode.Content)
		assert.Equal(t, node.Keywords, retrievedNode.Keywords)
		assert.Equal(t, node.Tags, retrievedNode.Tags)
		assert.Equal(t, node.Version, retrievedNode.Version)

		// Clean up
		err = repo.DeleteNode(ctx, userID, node.ID)
		require.NoError(t, err)
	})

	t.Run("NodeNotFound", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		node, err := repo.FindNodeByID(ctx, userID, nonExistentID)
		require.NoError(t, err)
		assert.Nil(t, node)
	})

	t.Run("UpdateNode", func(t *testing.T) {
		// Create initial node
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Original content",
			Keywords:  []string{"original"},
			Tags:      []string{"tag1"},
			CreatedAt: time.Now(),
			Version:   0,
		}

		err := repo.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		// Update the node
		node.Content = "Updated content"
		node.Keywords = []string{"updated", "content"}
		node.Tags = []string{"tag1", "tag2"}
		node.Version = 1

		err = repo.UpdateNodeAndEdges(ctx, node, []string{})
		require.NoError(t, err)

		// Verify update
		retrievedNode, err := repo.FindNodeByID(ctx, userID, node.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedNode)

		assert.Equal(t, "Updated content", retrievedNode.Content)
		assert.Equal(t, []string{"updated", "content"}, retrievedNode.Keywords)
		assert.Equal(t, []string{"tag1", "tag2"}, retrievedNode.Tags)
		assert.Equal(t, 1, retrievedNode.Version)

		// Clean up
		err = repo.DeleteNode(ctx, userID, node.ID)
		require.NoError(t, err)
	})
}

func TestEdgeOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repo := setupTestRepo(t)
	ctx := context.Background()
	userID := "test-user-" + uuid.New().String()

	t.Run("CreateEdges", func(t *testing.T) {
		// Create two nodes
		node1 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "First node",
			Keywords:  []string{"first"},
			CreatedAt: time.Now(),
		}
		
		node2 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Second node",
			Keywords:  []string{"second"},
			CreatedAt: time.Now(),
		}

		// Create both nodes
		err := repo.CreateNodeAndKeywords(ctx, node1)
		require.NoError(t, err)
		
		err = repo.CreateNodeAndKeywords(ctx, node2)
		require.NoError(t, err)

		// Create edge between them
		err = repo.CreateEdges(ctx, userID, node1.ID, []string{node2.ID})
		require.NoError(t, err)

		// Verify edge exists
		edgeQuery := repository.EdgeQuery{
			UserID:   userID,
			SourceID: node1.ID,
		}
		edges, err := repo.FindEdges(ctx, edgeQuery)
		require.NoError(t, err)
		assert.Len(t, edges, 1)
		assert.Equal(t, node1.ID, edges[0].SourceID)
		assert.Equal(t, node2.ID, edges[0].TargetID)

		// Clean up
		err = repo.DeleteNode(ctx, userID, node1.ID)
		require.NoError(t, err)
		err = repo.DeleteNode(ctx, userID, node2.ID)
		require.NoError(t, err)
	})
}

func TestCategoryOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repo := setupTestRepo(t)
	ctx := context.Background()
	userID := "test-user-" + uuid.New().String()

	t.Run("CreateAndRetrieveCategory", func(t *testing.T) {
		// Create a test category
		category := domain.Category{
			ID:          uuid.New().String(),
			UserID:      userID,
			Title:       "Test Category",
			Description: "A category for testing",
			CreatedAt:   time.Now(),
		}

		// Create the category
		err := repo.CreateCategory(ctx, category)
		require.NoError(t, err)

		// Retrieve the category
		retrievedCategory, err := repo.FindCategoryByID(ctx, userID, category.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedCategory)

		// Verify category data
		assert.Equal(t, category.ID, retrievedCategory.ID)
		assert.Equal(t, category.UserID, retrievedCategory.UserID)
		assert.Equal(t, category.Title, retrievedCategory.Title)
		assert.Equal(t, category.Description, retrievedCategory.Description)

		// Clean up
		err = repo.DeleteCategory(ctx, userID, category.ID)
		require.NoError(t, err)
	})

	t.Run("CategoryNodeAssociation", func(t *testing.T) {
		// Create a node and category
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Test node for category",
			Keywords:  []string{"test"},
			CreatedAt: time.Now(),
		}

		category := domain.Category{
			ID:          uuid.New().String(),
			UserID:      userID,
			Title:       "Test Category",
			Description: "A category for testing",
			CreatedAt:   time.Now(),
		}

		// Create both
		err := repo.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		err = repo.CreateCategory(ctx, category)
		require.NoError(t, err)

		// Associate node with category
		mapping := domain.NodeCategory{
			UserID:     userID,
			NodeID:     node.ID,
			CategoryID: category.ID,
			Confidence: 0.95,
			Method:     "test",
			CreatedAt:  time.Now(),
		}

		err = repo.AssignNodeToCategory(ctx, mapping)
		require.NoError(t, err)

		// Verify association
		categories, err := repo.FindCategoriesForNode(ctx, userID, node.ID)
		require.NoError(t, err)
		assert.Len(t, categories, 1)
		assert.Equal(t, category.ID, categories[0].ID)

		nodes, err := repo.FindNodesByCategory(ctx, userID, category.ID)
		require.NoError(t, err)
		assert.Len(t, nodes, 1)
		assert.Equal(t, node.ID, nodes[0].ID)

		// Clean up
		err = repo.RemoveNodeFromCategory(ctx, userID, node.ID, category.ID)
		require.NoError(t, err)

		err = repo.DeleteNode(ctx, userID, node.ID)
		require.NoError(t, err)

		err = repo.DeleteCategory(ctx, userID, category.ID)
		require.NoError(t, err)
	})
}

func TestGraphOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	repo := setupTestRepo(t)
	ctx := context.Background()
	userID := "test-user-" + uuid.New().String()

	t.Run("GetGraphData", func(t *testing.T) {
		// Create multiple nodes with edges
		nodes := []domain.Node{
			{
				ID:        uuid.New().String(),
				UserID:    userID,
				Content:   "Node 1",
				Keywords:  []string{"node1"},
				CreatedAt: time.Now(),
			},
			{
				ID:        uuid.New().String(),
				UserID:    userID,
				Content:   "Node 2",
				Keywords:  []string{"node2"},
				CreatedAt: time.Now(),
			},
			{
				ID:        uuid.New().String(),
				UserID:    userID,
				Content:   "Node 3",
				Keywords:  []string{"node3"},
				CreatedAt: time.Now(),
			},
		}

		// Create all nodes
		for _, node := range nodes {
			err := repo.CreateNodeAndKeywords(ctx, node)
			require.NoError(t, err)
		}

		// Create edges: 1->2, 2->3
		err := repo.CreateEdges(ctx, userID, nodes[0].ID, []string{nodes[1].ID})
		require.NoError(t, err)

		err = repo.CreateEdges(ctx, userID, nodes[1].ID, []string{nodes[2].ID})
		require.NoError(t, err)

		// Get graph data
		graphQuery := repository.GraphQuery{
			UserID:       userID,
			IncludeEdges: true,
		}

		graph, err := repo.GetGraphData(ctx, graphQuery)
		require.NoError(t, err)
		require.NotNil(t, graph)

		// Verify we have all nodes
		assert.Len(t, graph.Nodes, 3)

		// Verify we have edges (bidirectional, so should be 4 total)
		assert.True(t, len(graph.Edges) >= 2, "Should have at least 2 edges")

		// Clean up
		for _, node := range nodes {
			err = repo.DeleteNode(ctx, userID, node.ID)
			require.NoError(t, err)
		}
	})
}

// BenchmarkNodeCreation benchmarks node creation performance
func BenchmarkNodeCreation(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	repo := setupTestRepo(&testing.T{})
	ctx := context.Background()
	userID := "bench-user-" + uuid.New().String()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Benchmark node content",
			Keywords:  []string{"benchmark", "performance"},
			CreatedAt: time.Now(),
		}

		err := repo.CreateNodeAndKeywords(ctx, node)
		if err != nil {
			b.Fatal(err)
		}

		// Clean up immediately to avoid accumulating test data
		err = repo.DeleteNode(ctx, userID, node.ID)
		if err != nil {
			b.Fatal(err)
		}
	}
}