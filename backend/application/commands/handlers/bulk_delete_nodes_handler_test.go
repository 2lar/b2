package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/commands"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestNewBulkDeleteNodesHandler(t *testing.T) {
	// Arrange
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Act
	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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

func TestBulkDeleteNodesHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test nodes with valid UUIDs
	nodes := make([]*entities.Node, 3)
	nodeIDs := make([]valueobjects.NodeID, 3)
	nodeIDStrings := make([]string, 3)
	for i := 0; i < 3; i++ {
		content, _ := valueobjects.NewNodeContent("Node "+string(rune('A'+i)), "Content", valueobjects.FormatMarkdown)
		position, _ := valueobjects.NewPosition3D(0, 0, 0)
		node, _ := entities.NewNode("user123", content, position)
		node.SetGraphID("graph123")
		nodes[i] = node
		nodeIDs[i] = node.ID()
		nodeIDStrings[i] = node.ID().String()
	}

	// Create test command
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     nodeIDStrings,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Commit", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil).Maybe()

	// Mock node retrieval
	for i, nodeID := range nodeIDs {
		mockNodeRepo.On("GetByID", ctx, nodeID).Return(nodes[i], nil)
	}

	// Mock batch delete
	mockNodeRepo.On("DeleteBatch", ctx, nodeIDs).Return(nil)

	// Mock edge deletion
	mockEdgeRepo.On("DeleteByNodeIDs", ctx, "graph123", nodeIDStrings).Return(nil)

	// Mock event publishing - handler now uses batch publishing
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_InvalidNodeIDs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test command with invalid node IDs that pass command validation but fail UUID parsing
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     []string{"not-a-uuid-1", "not-a-uuid-2", "not-a-uuid-3"},
	}

	// Mock event publishing for failure event
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "all node IDs are invalid")
	mockEventBus.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_NodeNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node with valid UUID
	content, _ := valueobjects.NewNodeContent("Test Node", "Content", valueobjects.FormatMarkdown)
	position, _ := valueobjects.NewPosition3D(0, 0, 0)
	node, _ := entities.NewNode("user123", content, position)
	nodeID := node.ID()
	nodeIDStr := nodeID.String()

	// Create test command
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     []string{nodeIDStr},
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)

	// Mock node not found
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(nil, errors.New("node not found"))

	// Mock event publishing for failure
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "no valid nodes to delete")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_UnauthorizedUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node with valid UUID owned by different user
	content, _ := valueobjects.NewNodeContent("Test Node", "Content", valueobjects.FormatMarkdown)
	position, _ := valueobjects.NewPosition3D(0, 0, 0)
	node, _ := entities.NewNode("differentUser", content, position) // Different user
	node.SetGraphID("graph123")
	nodeID := node.ID()
	nodeIDStr := nodeID.String()

	// Create test command
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123", // Different user trying to delete
		NodeIDs:     []string{nodeIDStr},
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)

	// Mock node retrieval
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(node, nil)

	// Mock event publishing for failure
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "no valid nodes to delete")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_TransactionBeginError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test command
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     []string{valueobjects.NewNodeID().String()},
	}

	// Setup mocks - transaction begin fails
	mockUoW.On("Begin", ctx).Return(errors.New("transaction error"))
	// Command validation might still trigger event publishing
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil).Maybe()

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "failed to begin transaction")
	mockUoW.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_DeleteBatchError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test command
	// Create test node with valid UUID
	content, _ := valueobjects.NewNodeContent("Test Node", "Content", valueobjects.FormatMarkdown)
	position, _ := valueobjects.NewPosition3D(0, 0, 0)
	testNode, _ := entities.NewNode("user123", content, position)
	testNode.SetGraphID("graph123")
	nodeID := testNode.ID()
	nodeIDStr := nodeID.String()
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     []string{nodeIDStr},
	}

	// Use the existing testNode and nodeID from above
	node := testNode

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)

	// Mock node retrieval
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(node, nil)

	// Mock batch delete failure
	mockNodeRepo.On("DeleteBatch", ctx, []valueobjects.NodeID{nodeID}).Return(errors.New("delete failed"))

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "failed to delete nodes in batch")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
}

func TestBulkDeleteNodesHandler_Handle_CommitError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test command
	// Create test node with valid UUID
	content, _ := valueobjects.NewNodeContent("Test Node", "Content", valueobjects.FormatMarkdown)
	position, _ := valueobjects.NewPosition3D(0, 0, 0)
	testNode, _ := entities.NewNode("user123", content, position)
	testNode.SetGraphID("graph123")
	nodeID := testNode.ID()
	nodeIDStr := nodeID.String()
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     []string{nodeIDStr},
	}

	// Use the existing testNode and nodeID from above
	node := testNode

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)

	// Mock node retrieval
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(node, nil)

	// Mock batch delete success
	mockNodeRepo.On("DeleteBatch", ctx, []valueobjects.NodeID{nodeID}).Return(nil)

	// Mock edge deletion
	mockEdgeRepo.On("DeleteByNodeIDs", ctx, "graph123", []string{nodeIDStr}).Return(nil)

	// Mock commit failure
	mockUoW.On("Commit", ctx).Return(errors.New("commit failed"))

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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
	assert.Contains(t, err.Error(), "failed to commit bulk delete transaction")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
}

// Benchmarks

func BenchmarkBulkDeleteNodesHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventStore := new(mocks.MockEventStore)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test nodes with valid UUIDs
	nodes := make([]*entities.Node, 3)
	nodeIDs := make([]valueobjects.NodeID, 3)
	nodeIDStrings := make([]string, 3)
	for i := 0; i < 3; i++ {
		content, _ := valueobjects.NewNodeContent("Node "+string(rune('A'+i)), "Content", valueobjects.FormatMarkdown)
		position, _ := valueobjects.NewPosition3D(0, 0, 0)
		node, _ := entities.NewNode("user123", content, position)
		node.SetGraphID("graph123")
		nodes[i] = node
		nodeIDs[i] = node.ID()
		nodeIDStrings[i] = node.ID().String()
	}

	// Create test command
	cmd := commands.BulkDeleteNodesCommand{
		OperationID: "op123",
		UserID:      "user123",
		NodeIDs:     nodeIDStrings,
	}

	// Setup mocks with Any matchers for benchmark
	mockUoW.On("Begin", mock.Anything).Return(nil)
	mockUoW.On("Commit", mock.Anything).Return(nil)
	mockUoW.On("Rollback").Return(nil).Maybe()

	for i, nodeID := range nodeIDs {
		mockNodeRepo.On("GetByID", mock.Anything, nodeID).Return(nodes[i], nil)
	}

	mockNodeRepo.On("DeleteBatch", mock.Anything, nodeIDs).Return(nil)
	mockEdgeRepo.On("DeleteByNodeIDs", mock.Anything, "graph123", nodeIDStrings).Return(nil)
	mockEventBus.On("PublishBatch", mock.Anything, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewBulkDeleteNodesHandler(
		mockUoW,
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