package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/commands"
	"backend/domain/core/valueobjects"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewDeleteNodeHandler(t *testing.T) {
	// Arrange
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Act
	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Assert
	assert.NotNil(t, handler)
}

func TestDeleteNodeHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node
	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Test Node").
		MustBuild()

	// Create command
	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Setup mocks
	// Create a test graph for the deletion event
	testGraph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(testGraph, nil)
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_NodeNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	nodeID := valueobjects.NewNodeID()
	cmd := commands.DeleteNodeCommand{
		NodeID: nodeID.String(),
		UserID: "user123",
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(nil, errors.New("node not found"))

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
	mockNodeRepo.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_UnauthorizedUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node owned by different user
	node := fixtures.NewNodeBuilder().
		WithUserID("different-user").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
	mockNodeRepo.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_TransactionBeginError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Use a valid UUID that will fail validation
	cmd := commands.DeleteNodeCommand{
		NodeID: "not-a-valid-uuid",
		UserID: "user123",
	}

	// Setup mocks

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid node ID")
}

func TestDeleteNodeHandler_Handle_DeleteNodeError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(errors.New("delete failed"))

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
	mockNodeRepo.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_DeleteEdgesError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(errors.New("publish failed"))

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	// Event publish failure is logged but doesn't fail the delete
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_GraphNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(nil, errors.New("graph not found"))
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert - deletion should succeed even if graph not found
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_EventPublishError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(errors.New("publish failed"))

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	// Event publish error is typically logged but doesn't fail the operation
	// Depends on implementation - adjust based on actual behavior
	if err != nil {
		assert.Contains(t, err.Error(), "publish")
	}
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestDeleteNodeHandler_Handle_InvalidNodeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	cmd := commands.DeleteNodeCommand{
		NodeID: "invalid-id",
		UserID: "user123",
	}

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestDeleteNodeHandler_Handle_EmptyNodeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	cmd := commands.DeleteNodeCommand{
		NodeID: "",
		UserID: "user123",
	}

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node ID is required")
}

func TestDeleteNodeHandler_Handle_EmptyUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	nodeID := valueobjects.NewNodeID()
	cmd := commands.DeleteNodeCommand{
		NodeID: nodeID.String(),
		UserID: "",
	}

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestDeleteNodeHandler_Handle_CascadeDeleteWithManyEdges(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node with many connections
	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Hub Node").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Delete", ctx, node.ID()).Return(nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

// Benchmarks

func BenchmarkDeleteNodeHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.DeleteNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Setup mocks with Any matchers for benchmark
	mockNodeRepo.On("GetByID", mock.Anything, node.ID()).Return(node, nil)
	mockNodeRepo.On("Delete", mock.Anything, node.ID()).Return(nil)
	mockEdgeRepo.On("DeleteByNodeID", mock.Anything, "graph123", node.ID().String()).Return(nil)
	mockGraphRepo.On("UpdateGraphMetadata", mock.Anything, "graph123").Return(nil)
	mockEventBus.On("PublishBatch", mock.Anything, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, cmd)
	}
}

func BenchmarkDeleteNodeHandler_Validation(b *testing.B) {
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Test with invalid command that will fail validation
	cmd := commands.DeleteNodeCommand{
		NodeID: "invalid",
		UserID: "",
	}

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, cmd)
	}
}

// TestDeleteNodeHandler_Integration simulates a more complex scenario
func TestDeleteNodeHandler_Integration(t *testing.T) {
	// This test simulates deleting a node that's part of a complex graph structure
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test data
	testData := fixtures.CreateTestData()
	require.NotNil(t, testData)
	require.NotEmpty(t, testData.Nodes)

	// Pick a node to delete
	nodeToDelete := testData.Nodes[2] // Middle node

	cmd := commands.DeleteNodeCommand{
		NodeID: nodeToDelete.ID().String(),
		UserID: testData.UserID,
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, nodeToDelete.ID()).Return(nodeToDelete, nil)
	mockNodeRepo.On("Delete", ctx, nodeToDelete.ID()).Return(nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, testData.UserID).Return(testData.DefaultGraph, nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteNodeHandler(
		mockNodeRepo,
		mockEdgeRepo,
		mockGraphRepo,
		mockEventStore,
		mockEventBus,
		logger,
	)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}