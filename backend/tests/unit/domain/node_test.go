package domain_test

import (
	"testing"

	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNode_Creation(t *testing.T) {
	// Arrange
	content, err := valueobjects.NewNodeContent(
		"Test Node",
		"This is test content",
		valueobjects.FormatMarkdown,
	)
	require.NoError(t, err)

	position, err := valueobjects.NewPosition2D(10.5, 20.5)
	require.NoError(t, err)

	// Act
	node, err := entities.NewNode("user-123", content, position)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, node)
	assert.Equal(t, "user-123", node.UserID())
	assert.Equal(t, entities.StatusDraft, node.Status())
	assert.Equal(t, 1, node.Version())
	assert.Equal(t, "Test Node", node.Content().Title())
	assert.Equal(t, "This is test content", node.Content().Body())
}

func TestNode_UpdateContent(t *testing.T) {
	// Arrange
	node := createTestNode(t)
	newContent, err := valueobjects.NewNodeContent(
		"Updated Title",
		"Updated content",
		valueobjects.FormatPlainText,
	)
	require.NoError(t, err)

	// Act
	err = node.UpdateContent(newContent)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Updated Title", node.Content().Title())
	assert.Equal(t, "Updated content", node.Content().Body())
	assert.Equal(t, 2, node.Version())
}

func TestNode_CannotUpdateArchivedNode(t *testing.T) {
	// Arrange
	node := createTestNode(t)
	err := node.Archive()
	require.NoError(t, err)

	newContent, err := valueobjects.NewNodeContent(
		"New Title",
		"New content",
		valueobjects.FormatPlainText,
	)
	require.NoError(t, err)

	// Act
	err = node.UpdateContent(newContent)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update archived node")
}

func TestNode_ConnectTo(t *testing.T) {
	// Arrange
	node := createTestNode(t)
	targetID := valueobjects.NewNodeID()

	// Act
	err := node.ConnectTo(targetID, entities.EdgeTypeReference)

	// Assert
	assert.NoError(t, err)
	connections := node.GetConnections()
	assert.Len(t, connections, 1)
	assert.Equal(t, targetID, connections[0].TargetID)
	assert.Equal(t, entities.EdgeTypeReference, connections[0].Type)
}

func TestNode_CannotConnectToSelf(t *testing.T) {
	// Arrange
	node := createTestNode(t)

	// Act
	err := node.ConnectTo(node.ID(), entities.EdgeTypeReference)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect node to itself")
}

func TestNode_AddTag(t *testing.T) {
	// Arrange
	node := createTestNode(t)

	// Act
	err := node.AddTag("important")
	require.NoError(t, err)
	err = node.AddTag("project-x")
	require.NoError(t, err)

	// Assert
	tags := node.GetTags()
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "important")
	assert.Contains(t, tags, "project-x")
}

func TestNode_RemoveTag(t *testing.T) {
	// Arrange
	node := createTestNode(t)
	node.AddTag("tag1")
	node.AddTag("tag2")
	node.AddTag("tag3")

	// Act
	err := node.RemoveTag("tag2")

	// Assert
	assert.NoError(t, err)
	tags := node.GetTags()
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "tag1")
	assert.Contains(t, tags, "tag3")
	assert.NotContains(t, tags, "tag2")
}

func TestNode_PublishChangesStatus(t *testing.T) {
	// Arrange
	node := createTestNode(t)
	assert.Equal(t, entities.StatusDraft, node.Status())

	// Act
	err := node.Publish()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, entities.StatusPublished, node.Status())
	assert.Equal(t, 2, node.Version())
}

func TestNode_DomainEvents(t *testing.T) {
	// Arrange & Act
	node := createTestNode(t)

	// Assert - should have creation event
	events := node.GetUncommittedEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, "node.created", events[0].GetEventType())

	// Act - update content
	newContent, _ := valueobjects.NewNodeContent("Updated", "Content", valueobjects.FormatPlainText)
	node.UpdateContent(newContent)

	// Assert - should have two events
	events = node.GetUncommittedEvents()
	assert.Len(t, events, 2)
	assert.Equal(t, "node.content_updated", events[1].GetEventType())

	// Act - mark as committed
	node.MarkEventsAsCommitted()

	// Assert - should have no uncommitted events
	events = node.GetUncommittedEvents()
	assert.Len(t, events, 0)
}

// Helper function to create a test node
func createTestNode(t *testing.T) *entities.Node {
	content, err := valueobjects.NewNodeContent(
		"Test Node",
		"Test content",
		valueobjects.FormatMarkdown,
	)
	require.NoError(t, err)

	position, err := valueobjects.NewPosition2D(0, 0)
	require.NoError(t, err)

	node, err := entities.NewNode("test-user", content, position)
	require.NoError(t, err)

	// Clear initial events for cleaner tests
	node.MarkEventsAsCommitted()

	return node
}
