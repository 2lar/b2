package memory

import (
	"context"
	"testing"

	"brain2-backend/internal/repository/mocks"

	"github.com/stretchr/testify/assert"
)

// Test the consolidated service functionality
func TestConsolidatedServiceFlow(t *testing.T) {
	mockRepo := mocks.NewMockRepository()
	service := NewServiceFromRepository(mockRepo)
	ctx := context.Background()
	userID := "test-user"

	t.Run("CreateNode should use consolidated method", func(t *testing.T) {
		// Test
		node, edges, err := service.CreateNode(ctx, userID, "Test content", []string{"test"})
		
		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, 0, node.Version) // Should start at version 0
		assert.Equal(t, "Test content", node.Content)
		assert.NotNil(t, edges)
	})

	t.Run("UpdateNode should work with mock repository", func(t *testing.T) {
		mockRepo2 := mocks.NewMockRepository()
		service2 := NewServiceFromRepository(mockRepo2)

		// First create a node
		node, _, err := service2.CreateNode(ctx, userID, "Original content", []string{"original"})
		assert.NoError(t, err)

		// Then update it
		updatedNode, err := service2.UpdateNode(ctx, userID, node.ID, "New content", []string{"updated"})
		
		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, updatedNode)
		assert.Equal(t, "New content", updatedNode.Content)
	})

	t.Run("DeleteNode should work", func(t *testing.T) {
		mockRepo3 := mocks.NewMockRepository()
		service3 := NewServiceFromRepository(mockRepo3)

		// First create a node
		node, _, err := service3.CreateNode(ctx, userID, "To be deleted", []string{})
		assert.NoError(t, err)

		// Then delete it
		err = service3.DeleteNode(ctx, userID, node.ID)
		assert.NoError(t, err)
	})

	t.Run("Idempotency key should work with context", func(t *testing.T) {
		idempotencyKey := "test-key-123"
		ctx := WithIdempotencyKey(context.Background(), idempotencyKey)

		retrievedKey := GetIdempotencyKeyFromContext(ctx)
		assert.Equal(t, idempotencyKey, retrievedKey)
	})
}

func TestExtractKeywords(t *testing.T) {
	t.Run("should extract meaningful keywords", func(t *testing.T) {
		content := "This is a test content with some meaningful words"
		keywords := ExtractKeywords(content)
		
		// Should exclude stop words and include meaningful ones
		assert.Contains(t, keywords, "test")
		assert.Contains(t, keywords, "content")
		assert.Contains(t, keywords, "meaningful")
		assert.Contains(t, keywords, "words")
		assert.NotContains(t, keywords, "this")
		assert.NotContains(t, keywords, "is")
		assert.NotContains(t, keywords, "a")
	})
}