// Package memory provides unit tests for the memory service using mock repositories.
package memory

import (
	"context"
	"testing"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository/mocks"
	appErrors "brain2-backend/pkg/errors"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateNodeAndKeywords(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("SuccessfulCreation", func(t *testing.T) {
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    "test-user",
			Content:   "This is a test node with various keywords",
			Keywords:  []string{"test", "node", "keywords"},
			Tags:      []string{"testing"},
			CreatedAt: time.Now(),
			Version:   0,
		}

		err := service.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		// Verify the node was stored
		storedNode, err := mockRepo.FindNodeByID(ctx, node.UserID, node.ID)
		require.NoError(t, err)
		require.NotNil(t, storedNode)

		assert.Equal(t, node.ID, storedNode.ID)
		assert.Equal(t, node.Content, storedNode.Content)
		assert.Equal(t, node.Keywords, storedNode.Keywords)
	})

	t.Run("EmptyContent", func(t *testing.T) {
		node := domain.Node{
			ID:      uuid.New().String(),
			UserID:  "test-user",
			Content: "", // Empty content should fail
		}

		err := service.CreateNodeAndKeywords(ctx, node)
		require.Error(t, err)
		assert.True(t, appErrors.IsValidation(err))
	})

	t.Run("RepositoryError", func(t *testing.T) {
		// Configure mock to return error
		mockRepo.SetError("CreateNodeAndKeywords", appErrors.NewInternal("database error", nil))

		node := domain.Node{
			ID:      uuid.New().String(),
			UserID:  "test-user",
			Content: "Valid content",
		}

		err := service.CreateNodeAndKeywords(ctx, node)
		require.Error(t, err)
	})
}

func TestCreateNodeWithEdges(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("SuccessfulCreationWithEdges", func(t *testing.T) {
		userID := "test-user"
		
		// First create some existing nodes to connect to
		existingNode1 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Existing node 1",
			Keywords:  []string{"existing"},
			CreatedAt: time.Now(),
		}
		
		existingNode2 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Existing node 2",
			Keywords:  []string{"existing"},
			CreatedAt: time.Now(),
		}

		err := mockRepo.CreateNodeAndKeywords(ctx, existingNode1)
		require.NoError(t, err)
		
		err = mockRepo.CreateNodeAndKeywords(ctx, existingNode2)
		require.NoError(t, err)

		// Now create a new node with edges to existing nodes
		newNode, err := service.CreateNodeWithEdges(ctx, userID, "New node content with connections")
		require.NoError(t, err)
		require.NotNil(t, newNode)

		assert.Equal(t, userID, newNode.UserID)
		assert.Equal(t, "New node content with connections", newNode.Content)
		assert.NotEmpty(t, newNode.Keywords)
		assert.NotEmpty(t, newNode.ID)
	})

	t.Run("EmptyContent", func(t *testing.T) {
		node, err := service.CreateNodeWithEdges(ctx, "test-user", "")
		require.Error(t, err)
		assert.Nil(t, node)
		assert.True(t, appErrors.IsValidation(err))
	})
}

func TestGetNodeDetails(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("NodeExists", func(t *testing.T) {
		userID := "test-user"
		
		// Create a node with some edges
		node1 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Node 1",
			Keywords:  []string{"node1"},
			CreatedAt: time.Now(),
		}
		
		node2 := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Node 2",
			Keywords:  []string{"node2"},
			CreatedAt: time.Now(),
		}

		// Store both nodes
		err := mockRepo.CreateNodeAndKeywords(ctx, node1)
		require.NoError(t, err)
		
		err = mockRepo.CreateNodeAndKeywords(ctx, node2)
		require.NoError(t, err)
		
		// Create edge between them
		err = mockRepo.CreateEdges(ctx, userID, node1.ID, []string{node2.ID})
		require.NoError(t, err)

		// Get node details
		retrievedNode, edges, err := service.GetNodeDetails(ctx, userID, node1.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedNode)

		assert.Equal(t, node1.ID, retrievedNode.ID)
		assert.Equal(t, node1.Content, retrievedNode.Content)
		assert.NotEmpty(t, edges)
		assert.Equal(t, node2.ID, edges[0].TargetID)
	})

	t.Run("NodeNotFound", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		
		node, edges, err := service.GetNodeDetails(ctx, "test-user", nonExistentID)
		require.Error(t, err)
		assert.Nil(t, node)
		assert.Nil(t, edges)
		assert.True(t, appErrors.IsNotFound(err))
	})
}

func TestDeleteNode(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("SuccessfulDeletion", func(t *testing.T) {
		userID := "test-user"
		
		node := domain.Node{
			ID:        uuid.New().String(),
			UserID:    userID,
			Content:   "Node to delete",
			Keywords:  []string{"delete"},
			CreatedAt: time.Now(),
		}

		// Create the node
		err := mockRepo.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		// Verify it exists
		storedNode, err := mockRepo.FindNodeByID(ctx, userID, node.ID)
		require.NoError(t, err)
		require.NotNil(t, storedNode)

		// Delete it
		err = service.DeleteNode(ctx, userID, node.ID)
		require.NoError(t, err)

		// Verify it's gone
		deletedNode, err := mockRepo.FindNodeByID(ctx, userID, node.ID)
		require.NoError(t, err)
		assert.Nil(t, deletedNode)
	})

	t.Run("NodeNotFound", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		
		err := service.DeleteNode(ctx, "test-user", nonExistentID)
		require.Error(t, err)
		assert.True(t, appErrors.IsNotFound(err))
	})
}

func TestUpdateNode(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("SuccessfulUpdate", func(t *testing.T) {
		userID := "test-user"
		
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

		err := mockRepo.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)

		// Update the node
		updatedNode, err := service.UpdateNode(ctx, userID, node.ID, "Updated content", []string{"tag1", "tag2"})
		require.NoError(t, err)
		require.NotNil(t, updatedNode)

		assert.Equal(t, "Updated content", updatedNode.Content)
		assert.Equal(t, []string{"tag1", "tag2"}, updatedNode.Tags)
		assert.Equal(t, 1, updatedNode.Version) // Version should increment
		assert.NotEqual(t, node.Keywords, updatedNode.Keywords) // Keywords should be re-extracted
	})

	t.Run("NodeNotFound", func(t *testing.T) {
		nonExistentID := uuid.New().String()
		
		node, err := service.UpdateNode(ctx, "test-user", nonExistentID, "New content", []string{})
		require.Error(t, err)
		assert.Nil(t, node)
		assert.True(t, appErrors.IsNotFound(err))
	})
}

func TestGetGraphData(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("EmptyGraph", func(t *testing.T) {
		graph, err := service.GetGraphData(ctx, "test-user")
		require.NoError(t, err)
		require.NotNil(t, graph)

		assert.Empty(t, graph.Nodes)
		assert.Empty(t, graph.Edges)
	})

	t.Run("GraphWithNodes", func(t *testing.T) {
		userID := "test-user"
		
		// Create multiple nodes
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
		}

		for _, node := range nodes {
			err := mockRepo.CreateNodeAndKeywords(ctx, node)
			require.NoError(t, err)
		}

		// Create edge between nodes
		err := mockRepo.CreateEdges(ctx, userID, nodes[0].ID, []string{nodes[1].ID})
		require.NoError(t, err)

		// Get graph data
		graph, err := service.GetGraphData(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, graph)

		assert.Len(t, graph.Nodes, 2)
		assert.NotEmpty(t, graph.Edges)
	})
}

// TestServiceWithMockErrors demonstrates how to test error scenarios
func TestServiceWithMockErrors(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	t.Run("RepositoryFailure", func(t *testing.T) {
		// Configure mock to fail on CreateNodeAndKeywords
		mockRepo.SetError("CreateNodeAndKeywords", appErrors.NewInternal("simulated database failure", nil))

		node := domain.Node{
			ID:      uuid.New().String(),
			UserID:  "test-user",
			Content: "Valid content",
		}

		err := service.CreateNodeAndKeywords(ctx, node)
		require.Error(t, err)

		// Clear the error for subsequent tests
		mockRepo.ClearErrors()

		// Should work now
		err = service.CreateNodeAndKeywords(ctx, node)
		require.NoError(t, err)
	})
}