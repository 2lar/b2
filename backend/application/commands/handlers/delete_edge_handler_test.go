package handlers

import (
	"context"
	"testing"

	"backend/application/commands"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeleteEdgeHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)

	edgeID := uuid.New().String()
	cmd := commands.DeleteEdgeCommand{
		UserID:  "user123",
		GraphID: "graph123",
		EdgeID:  edgeID,
	}

	// Create test fixtures with nodes and graph containing the edge
	sourceNode := fixtures.NewNodeBuilder().WithUserID("user123").MustBuild()
	targetNode := fixtures.NewNodeBuilder().WithUserID("user123").MustBuild()

	// Create a graph with nodes first
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithNodes(sourceNode, targetNode).
		MustBuild()

	// Create edge manually for test
	edge := &aggregates.Edge{
		ID:       edgeID,
		SourceID: sourceNode.ID(),
		TargetID: targetNode.ID(),
		Type:     entities.EdgeTypeNormal,
		Weight:   1.0,
	}

	// Load edge into graph
	_ = graph.LoadEdge(edge)

	// Setup mock expectations
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil).Maybe()
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByID", ctx, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", ctx, targetNode.ID()).Return(targetNode, nil)
	mockGraphRepo.On("SaveWithUoW", ctx, graph, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, sourceNode, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, targetNode, mockUoW).Return(nil)
	mockUoW.On("Commit", ctx).Return(nil)
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewDeleteEdgeHandler(mockUoW, mockGraphRepo, mockNodeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.NoError(t, err)
	mockUoW.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestDeleteEdgeHandler_Handle_InvalidCommandType(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)

	// Wrong command type
	invalidCmd := "not a command"

	handler := NewDeleteEdgeHandler(mockUoW, mockGraphRepo, mockNodeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, invalidCmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid command type")
}

func TestDeleteEdgeHandler_Handle_EmptyEdgeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)

	cmd := commands.DeleteEdgeCommand{
		UserID: "user123",
		EdgeID: "",
	}

	handler := NewDeleteEdgeHandler(mockUoW, mockGraphRepo, mockNodeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	// The handler validates the command and should return an error for empty EdgeID
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestDeleteEdgeHandler_Handle_EmptyUserID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)

	cmd := commands.DeleteEdgeCommand{
		UserID: "",
		EdgeID: uuid.New().String(),
	}

	handler := NewDeleteEdgeHandler(mockUoW, mockGraphRepo, mockNodeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	// The handler validates the command and should return an error for empty UserID
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// Benchmarks

func BenchmarkDeleteEdgeHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)

	cmd := commands.DeleteEdgeCommand{
		UserID:  "user123",
		GraphID: "graph123",
		EdgeID:  uuid.New().String(),
	}

	handler := NewDeleteEdgeHandler(mockUoW, mockGraphRepo, mockNodeRepo, mockEventBus)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, &cmd)
	}
}