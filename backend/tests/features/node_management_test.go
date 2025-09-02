//go:build bdd
// +build bdd

package features

import (
	"testing"

	"brain2-backend/tests/features/framework"
	"brain2-backend/tests/fixtures/builders"
)

// TestNodeCreation tests the node creation workflow
func TestNodeCreation(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().CategoryExists("cat-456").
		When().
			CreatingNode(builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Important meeting notes").
				WithTitle("Q4 Planning Meeting").
				WithTags("meeting", "planning", "q4").
				WithCategories("cat-456").
				Build()).
		Then().
			NodeShouldExist().
			And().EventShouldBePublished("NodeCreated").
			And().NoErrorShouldOccur()
}

// TestNodeCreationWithConnections tests node creation with automatic connections
func TestNodeCreationWithConnections(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().NodesExist(5, "user-123"). // Create 5 existing nodes
		When().
			CreatingNode(builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Related content about meetings").
				WithKeywords("meeting", "planning").
				Build()).
		Then().
			NodeShouldExist().
			And().EventShouldBePublished("NodeCreated").
			And().ConnectionsShouldBeCreated(3). // Expect 3 auto-connections
			And().NoErrorShouldOccur()
}

// TestNodeUpdate tests updating an existing node
func TestNodeUpdate(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().NodeExists("node-1", "user-123").
		When().
			UpdatingNode("node-1", map[string]interface{}{
				"content": "Updated content",
				"title":   "Updated title",
			}).
		Then().
			EventShouldBePublished("NodeUpdated").
			And().NoErrorShouldOccur()
}

// TestNodeDeletion tests deleting a node
func TestNodeDeletion(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().NodeExists("node-1", "user-123").
		When().
			DeletingNode("node-1").
		Then().
			EventShouldBePublished("NodeDeleted").
			And().NodeShouldNotExist("node-1").
			And().NoErrorShouldOccur()
}

// TestNodeConnectionCreation tests creating connections between nodes
func TestNodeConnectionCreation(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().NodeExists("node-1", "user-123").
			And().NodeExists("node-2", "user-123").
		When().
			ConnectingNodes("node-1", "node-2").
		Then().
			EventShouldBePublished("NodeConnected").
			And().NoErrorShouldOccur()
}

// TestBulkNodeDeletion tests bulk deletion of nodes
func TestBulkNodeDeletion(t *testing.T) {
	nodeIDs := []string{"node-1", "node-2", "node-3"}
	
	scenario := framework.NewScenario(t).
		Given().
			UserExists("user-123")
	
	// Create the nodes
	for _, nodeID := range nodeIDs {
		scenario.Given().And().NodeExists(nodeID, "user-123")
	}
	
	scenario.
		When().
			ExecutingBulkOperation("delete", nodeIDs).
		Then().
			CommandShouldBeExecuted("DeleteNode").
			And().NoErrorShouldOccur()
}

// TestNodeCreationValidation tests validation during node creation
func TestNodeCreationValidation(t *testing.T) {
	t.Run("empty content should fail", func(t *testing.T) {
		framework.NewScenario(t).
			Given().
				UserExists("user-123").
			When().
				CreatingNode(builders.NewNodeBuilder().
					WithUserID("user-123").
					WithContent(""). // Empty content
					Build()).
			Then().
				ErrorShouldOccur().
				And().ErrorShouldContain("content is required")
	})
	
	t.Run("missing user ID should fail", func(t *testing.T) {
		framework.NewScenario(t).
			Given().
				UserExists("user-123").
			When().
				CreatingNode(builders.NewNodeBuilder().
					WithUserID(""). // Empty user ID
					WithContent("Valid content").
					Build()).
			Then().
				ErrorShouldOccur().
				And().ErrorShouldContain("user ID is required")
	})
}

// TestNodeArchival tests archiving a node
func TestNodeArchival(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().NodeExists("node-1", "user-123").
		When().
			ExecutingBulkOperation("archive", []string{"node-1"}).
		Then().
			EventShouldBePublished("NodeArchived").
			And().NoErrorShouldOccur()
}

// TestComplexWorkflow tests a complex multi-step workflow
func TestComplexWorkflow(t *testing.T) {
	framework.NewScenario(t).
		Given().
			UserExists("user-123").
			And().CategoryExists("work").
			And().CategoryExists("personal").
			And().NodesExist(3, "user-123").
		When().
			CreatingNode(builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Complex workflow content").
				WithCategories("work").
				Build()).
			And().ConnectingNodes("node-1", "node-2").
			And().UpdatingNode("node-1", map[string]interface{}{
				"categories": []string{"work", "personal"},
			}).
		Then().
			EventsShouldBePublished("NodeCreated", "NodeConnected", "NodeUpdated").
			And().ConnectionsShouldBeCreated(1).
			And().NoErrorShouldOccur()
}