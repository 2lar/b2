package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/queries"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetGraphHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create test nodes
	node1 := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Node 1").
		MustBuild()

	node2 := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Node 2").
		MustBuild()

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithName("Test Graph").
		WithDescription("Test Description").
		WithNodes(node1, node2).
		MustBuild()

	query := queries.GetGraphByIDQuery{
		UserID:  "user123",
		GraphID: "graph123",
	}

	// Setup mocks
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByGraphID", ctx, "graph123").Return([]*entities.Node{node1, node2}, nil)

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "graph123", result.ID)
	assert.Equal(t, "user123", result.UserID)
	assert.Equal(t, "Test Graph", result.Name)
	assert.Equal(t, "Test Description", result.Description)
	assert.Equal(t, 2, result.NodeCount)
	assert.Len(t, result.Nodes, 2)
	mockGraphRepo.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
}

func TestGetGraphHandler_Handle_GraphNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	query := queries.GetGraphByIDQuery{
		UserID:  "user123",
		GraphID: "nonexistent",
	}

	// Setup mocks
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("nonexistent")).Return(nil, errors.New("graph not found"))

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get graph")
	assert.Nil(t, result)
	mockGraphRepo.AssertExpectations(t)
}

func TestGetGraphHandler_Handle_UnauthorizedAccess(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create graph owned by different user
	graph := fixtures.NewGraphBuilder().
		WithUserID("different-user").
		WithID("graph123").
		MustBuild()

	query := queries.GetGraphByIDQuery{
		UserID:  "user123", // Different user trying to access
		GraphID: "graph123",
	}

	// Setup mocks
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
	assert.Nil(t, result)
	mockGraphRepo.AssertExpectations(t)
}

func TestGetGraphHandler_Handle_InvalidQuery(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	testCases := []struct {
		name     string
		query    queries.GetGraphByIDQuery
		expected string
	}{
		{
			name: "Empty UserID",
			query: queries.GetGraphByIDQuery{
				UserID:  "",
				GraphID: "graph123",
			},
			expected: "user ID is required",
		},
		{
			name: "Empty GraphID",
			query: queries.GetGraphByIDQuery{
				UserID:  "user123",
				GraphID: "",
			},
			expected: "graph ID is required",
		},
	}

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result, err := handler.Handle(ctx, tc.query)

			// Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expected)
			assert.Nil(t, result)
		})
	}
}

func TestGetGraphHandler_Handle_EmptyGraph(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	// Create empty graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithName("Empty Graph").
		MustBuild()

	query := queries.GetGraphByIDQuery{
		UserID:  "user123",
		GraphID: "graph123",
	}

	// Setup mocks
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByGraphID", ctx, "graph123").Return([]*entities.Node{}, nil)

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	// Act
	result, err := handler.Handle(ctx, query)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "graph123", result.ID)
	assert.Equal(t, 0, result.NodeCount)
	assert.Equal(t, 0, result.EdgeCount)
	assert.Len(t, result.Nodes, 0)
	assert.Len(t, result.Edges, 0)
	mockGraphRepo.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
}

// Benchmarks

func BenchmarkGetGraphHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	logger := zap.NewNop()

	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	query := queries.GetGraphByIDQuery{
		UserID:  "user123",
		GraphID: "graph123",
	}

	mockGraphRepo.On("GetByID", mock.Anything, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByGraphID", mock.Anything, "graph123").Return([]*entities.Node{}, nil)

	handler := NewGetGraphHandler(mockGraphRepo, mockNodeRepo, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.Handle(ctx, query)
	}
}