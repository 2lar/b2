package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSafeUpdateNode_BasicFunctionality(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	// Create a test node
	userID := "test-user"
	nodeID := uuid.New().String()
	initialNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   "Initial content",
		Keywords:  []string{"initial", "content"},
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Create the node in the mock repository
	err := mockRepo.CreateNodeAndKeywords(ctx, initialNode)
	assert.NoError(t, err)

	t.Run("Should successfully update node content", func(t *testing.T) {
		updatedNode, err := service.SafeUpdateNode(ctx, userID, nodeID, "Updated content", []string{"updated"})
		
		assert.NoError(t, err)
		assert.NotNil(t, updatedNode)
		assert.Equal(t, "Updated content", updatedNode.Content)
		assert.Contains(t, updatedNode.Tags, "updated")
	})

	t.Run("Should handle node not found error", func(t *testing.T) {
		nonExistentNodeID := uuid.New().String()
		
		_, err := service.SafeUpdateNode(ctx, userID, nonExistentNodeID, "Updated", []string{})
		
		assert.Error(t, err)
		// Should contain "not found" in the error message
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestSafeUpdateNodeWithConnections_BasicFunctionality(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	// Create test nodes
	userID := "test-user"
	nodeID := uuid.New().String()
	relatedNodeID := uuid.New().String()

	initialNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   "Initial content",
		Keywords:  []string{"initial"},
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	relatedNode := domain.Node{
		ID:        relatedNodeID,
		UserID:    userID,
		Content:   "Related node",
		Keywords:  []string{"related"},
		Tags:      []string{"related"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Create nodes in the mock repository
	err := mockRepo.CreateNodeAndKeywords(ctx, initialNode)
	assert.NoError(t, err)
	err = mockRepo.CreateNodeAndKeywords(ctx, relatedNode)
	assert.NoError(t, err)

	t.Run("Should successfully update node with connections", func(t *testing.T) {
		updatedNode, err := service.SafeUpdateNodeWithConnections(
			ctx, userID, nodeID,
			"Updated with connections",
			[]string{"updated", "connected"},
			[]string{relatedNodeID},
		)

		assert.NoError(t, err)
		assert.NotNil(t, updatedNode)
		assert.Equal(t, "Updated with connections", updatedNode.Content)
		assert.Contains(t, updatedNode.Tags, "updated")
		assert.Contains(t, updatedNode.Tags, "connected")
	})
}

func TestUpdateNodeWithRetry_CustomFunction(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	userID := "test-user"
	nodeID := uuid.New().String()
	initialNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   "Initial content",
		Keywords:  []string{"initial"},
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Create the node
	err := mockRepo.CreateNodeAndKeywords(ctx, initialNode)
	assert.NoError(t, err)

	t.Run("Should apply custom update function", func(t *testing.T) {
		customUpdateFn := func(node *domain.Node) error {
			node.Content = "Custom updated content"
			node.Tags = append(node.Tags, "custom")
			node.Keywords = ExtractKeywords(node.Content)
			return nil
		}

		updatedNode, err := service.UpdateNodeWithRetry(ctx, userID, nodeID, customUpdateFn)

		assert.NoError(t, err)
		assert.NotNil(t, updatedNode)
		assert.Equal(t, "Custom updated content", updatedNode.Content)
		assert.Contains(t, updatedNode.Tags, "custom")
		assert.Contains(t, updatedNode.Tags, "test") // Original tag should remain
	})

	t.Run("Should handle update function error", func(t *testing.T) {
		errorUpdateFn := func(node *domain.Node) error {
			return assert.AnError // Return a test error
		}

		_, err := service.UpdateNodeWithRetry(ctx, userID, nodeID, errorUpdateFn)

		assert.Error(t, err)
	})
}

func TestOptimisticLocking_ErrorHandling(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	userID := "test-user"
	nodeID := uuid.New().String()
	initialNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   "Initial content",
		Keywords:  []string{"initial"},
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Create the node
	err := mockRepo.CreateNodeAndKeywords(ctx, initialNode)
	assert.NoError(t, err)

	t.Run("Should handle optimistic lock error with retry", func(t *testing.T) {
		// First, let's test that the mock can simulate an optimistic lock error
		// Configure mock to fail on UpdateNodeAndEdges
		mockRepo.SetError("UpdateNodeAndEdges", repository.NewOptimisticLockError(nodeID, 1, 2))

		_, err := service.SafeUpdateNode(ctx, userID, nodeID, "Updated content", []string{"updated"})

		// Since our retry logic is designed to handle optimistic lock errors,
		// but the mock is configured to always return this error, it should fail
		// The error could be either the original optimistic lock error or max retries exceeded
		assert.Error(t, err)
		assert.True(t, 
			err.Error() == repository.NewOptimisticLockError(nodeID, 1, 2).Error() || 
			strings.Contains(err.Error(), "max retries exceeded"),
			"Expected either optimistic lock error or max retries exceeded error, got: %v", err)
	})

	t.Run("Should handle repository FindNodeByID error", func(t *testing.T) {
		// Clear previous errors and set a new error for FindNodeByID
		mockRepo.ClearErrors()
		mockRepo.SetError("FindNodeByID", assert.AnError)

		_, err := service.SafeUpdateNode(ctx, userID, nodeID, "Updated content", []string{"updated"})

		assert.Error(t, err)
	})
}

func TestVersionHandling(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewService(mockRepo)
	ctx := context.Background()

	userID := "test-user"

	t.Run("Should initialize new nodes with version 1", func(t *testing.T) {
		node, _, err := service.CreateNodeWithEdges(ctx, userID, "Test content", []string{"test"})
		
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, 1, node.Version, "New nodes should start with version 1")
	})

	t.Run("Should extract keywords correctly", func(t *testing.T) {
		content := "This is a test content with multiple keywords"
		keywords := ExtractKeywords(content)
		
		// Should extract meaningful keywords, not stop words
		assert.Contains(t, keywords, "test")
		assert.Contains(t, keywords, "content")
		assert.Contains(t, keywords, "multiple")
		assert.Contains(t, keywords, "keywords")
		
		// Should not contain stop words
		assert.NotContains(t, keywords, "this")
		assert.NotContains(t, keywords, "is")
		assert.NotContains(t, keywords, "a")
		assert.NotContains(t, keywords, "with")
	})
}