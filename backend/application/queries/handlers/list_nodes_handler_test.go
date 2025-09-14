package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"backend/application/queries"
	"backend/domain/core/entities"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestListNodesHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create test nodes
	node1 := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Node 1").
		WithContent("Content 1").
		MustBuild()

	node2 := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Node 2").
		WithContent("Content 2").
		MustBuild()

	nodes := []*entities.Node{node1, node2}

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  10,
		Offset: 0,
		SortBy: "updated",
		Order:  "desc",
	}

	// Setup mocks
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return(nodes, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount)
	assert.Len(t, result.Nodes, 2)
	assert.Equal(t, node1.ID().String(), result.Nodes[0].ID)
	assert.Equal(t, "Node 1", result.Nodes[0].Title)
	assert.Equal(t, node2.ID().String(), result.Nodes[1].ID)
	assert.Equal(t, "Node 2", result.Nodes[1].Title)
	mockNodeRepo.AssertExpectations(t)
}

func TestListNodesHandler_Handle_EmptyList(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  10,
		Offset: 0,
	}

	// Setup mocks - return empty slice
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return([]*entities.Node{}, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.TotalCount)
	assert.Len(t, result.Nodes, 0)
	mockNodeRepo.AssertExpectations(t)
}

func TestListNodesHandler_Handle_DefaultValues(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	query := queries.ListNodesQuery{
		UserID: "user123",
		// Limit, Offset, SortBy, Order not provided - should use defaults
	}

	// Setup mocks
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return([]*entities.Node{node}, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 20, result.Limit) // Default limit
	assert.Equal(t, 0, result.Offset) // Default offset
	mockNodeRepo.AssertExpectations(t)
}

func TestListNodesHandler_Handle_LimitExceedsMax(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  200, // Exceeds max of 100
	}

	// Setup mocks
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return([]*entities.Node{}, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.Limit) // Should be capped at 100
	mockNodeRepo.AssertExpectations(t)
}

func TestListNodesHandler_Handle_RepositoryError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  10,
	}

	// Setup mocks
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return(nil, errors.New("database error"))

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list nodes")
	assert.Nil(t, result)
	mockNodeRepo.AssertExpectations(t)
}

func TestListNodesHandler_Handle_WithPagination(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create 5 test nodes
	var nodes []*entities.Node
	for i := 1; i <= 5; i++ {
		node := fixtures.NewNodeBuilder().
			WithUserID("user123").
			WithGraphID("graph123").
			WithTitle(fmt.Sprintf("Node %d", i)).
			MustBuild()
		nodes = append(nodes, node)
	}

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  2,
		Offset: 2, // Skip first 2 nodes
	}

	// Setup mocks
	mockNodeRepo.On("GetByUserID", ctx, "user123").Return(nodes, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.TotalCount) // Total nodes
	assert.Equal(t, 2, result.Limit)
	assert.Equal(t, 2, result.Offset)
	// Due to pagination, should return nodes 3 and 4
	assert.Len(t, result.Nodes, 2)
	assert.Equal(t, "Node 3", result.Nodes[0].Title)
	assert.Equal(t, "Node 4", result.Nodes[1].Title)
	mockNodeRepo.AssertExpectations(t)
}

// Benchmarks

func BenchmarkListNodesHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create test nodes
	var nodes []*entities.Node
	for i := 0; i < 100; i++ {
		node := fixtures.NewNodeBuilder().
			WithUserID("user123").
			WithGraphID("graph123").
			MustBuild()
		nodes = append(nodes, node)
	}

	query := queries.ListNodesQuery{
		UserID: "user123",
		Limit:  20,
		Offset: 0,
	}

	mockNodeRepo.On("GetByUserID", mock.Anything, "user123").Return(nodes, nil)

	handler := NewListNodesHandler(mockNodeRepo, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.Handle(ctx, query)
	}
}