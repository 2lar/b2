package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/queries"
	"backend/domain/core/valueobjects"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetNodeHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create test node
	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Test Node").
		WithContent("Test content").
		MustBuild()

	query := queries.GetNodeQuery{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, node.ID().String(), result.ID)
	assert.Equal(t, "Test Node", result.Title)
	assert.Equal(t, "Test content", result.Content)
	assert.Equal(t, "user123", result.UserID)
	mockNodeRepo.AssertExpectations(t)
}

func TestGetNodeHandler_Handle_NodeNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	nodeID := valueobjects.NewNodeID()
	query := queries.GetNodeQuery{
		NodeID: nodeID.String(),
		UserID: "user123",
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(nil, errors.New("node not found"))

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
	assert.Nil(t, result)
	mockNodeRepo.AssertExpectations(t)
}

func TestGetNodeHandler_Handle_UnauthorizedAccess(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create test node owned by different user
	node := fixtures.NewNodeBuilder().
		WithUserID("different-user").
		WithGraphID("graph123").
		WithTitle("Private Node").
		MustBuild()

	query := queries.GetNodeQuery{
		NodeID: node.ID().String(),
		UserID: "user123", // Different user trying to access
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
	assert.Nil(t, result)
	mockNodeRepo.AssertExpectations(t)
}

func TestGetNodeHandler_Handle_InvalidNodeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.GetNodeQuery{
		NodeID: "invalid-uuid",
		UserID: "user123",
	}

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid node ID")
	assert.Nil(t, result)
}

func TestGetNodeHandler_Handle_EmptyNodeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.GetNodeQuery{
		NodeID: "",
		UserID: "user123",
	}

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid node ID")
	assert.Nil(t, result)
}

func TestGetNodeHandler_Handle_EmptyUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	nodeID := valueobjects.NewNodeID()
	query := queries.GetNodeQuery{
		NodeID: nodeID.String(),
		UserID: "",
	}

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid node ID")
	assert.Nil(t, result)
	// No repository calls should be made since UserID validation fails first
}

// Benchmarks

func BenchmarkGetNodeHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	query := queries.GetNodeQuery{
		NodeID: node.ID().String(),
		UserID: "user123",
	}

	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	handler := NewGetNodeHandler(mockNodeRepo, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.Handle(ctx, query)
	}
}