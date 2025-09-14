package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/commands"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/tests/fixtures"
	"backend/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateEdgeHandler_Handle_Success(t *testing.T) {
	// Fixed: Removed duplicate ConnectTo calls from handler

	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	// Create test nodes
	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Source Node").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Target Node").
		MustBuild()

	// Create test graph without adding nodes - they'll be added when connected
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		MustBuild()

	// Manually add nodes to the graph so they exist but aren't connected
	graph.AddNode(sourceNode)
	graph.AddNode(targetNode)

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "normal",
		Weight:   1.0,
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", ctx, targetNode.ID()).Return(targetNode, nil)
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)

	// Setup UoW mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Commit", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)

	// Save expectations
	mockGraphRepo.On("SaveWithUoW", ctx, graph, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, sourceNode, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, targetNode, mockUoW).Return(nil)

	// Event publishing
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestCreateEdgeHandler_Handle_NodesNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	sourceID := valueobjects.NewNodeID()
	targetID := valueobjects.NewNodeID()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceID.String(),
		TargetID: targetID.String(),
		Type:     "normal",
		Weight:   1.0,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	// Note: Graph repo is not called since source node retrieval fails first
	mockNodeRepo.On("GetByID", ctx, sourceID).Return(nil, errors.New("node not found"))

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
	mockNodeRepo.AssertExpectations(t)
	// Graph repo is not called since source node retrieval fails first
}

func TestCreateEdgeHandler_Handle_SelfReferenceEdge(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	// Create test graph
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithNodes(node).
		MustBuild()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: node.ID().String(),
		TargetID: node.ID().String(), // Same as source - self-reference
		Type:     "normal",
		Weight:   1.0,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot connect node to itself")
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

func TestCreateEdgeHandler_Handle_DuplicateEdge(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	// Create graph with existing edge
	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithNodes(sourceNode, targetNode).
		MustBuild()

	// Add edge to graph
	_, _ = graph.ConnectNodes(sourceNode.ID(), targetNode.ID(), entities.EdgeTypeNormal)

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "normal",
		Weight:   1.0,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByID", ctx, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", ctx, targetNode.ID()).Return(targetNode, nil)

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

func TestCreateEdgeHandler_Handle_UnauthorizedNodes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	// Nodes owned by different user
	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("other-user").
		WithGraphID("other-graph").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "normal",
		Weight:   1.0,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockNodeRepo.On("GetByID", ctx, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", ctx, targetNode.ID()).Return(targetNode, nil)

	// Note: No need to mock GraphRepo.GetByID because the handler should fail
	// on "nodes belong to different graphs" before trying to get the graph

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.Error(t, err)
	// Note: The error comes from early validation when nodes are in different graphs
	assert.Contains(t, err.Error(), "nodes belong to different graphs")
	mockNodeRepo.AssertExpectations(t)
	// Note: GraphRepo should not be called since validation fails early
}

func TestCreateEdgeHandler_Handle_InvalidEdgeType(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "invalid-type", // Invalid edge type
		Weight:   0.5,
	}

	// Setup mocks - Note: validation fails early so only UoW is called
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	// No other repo calls expected since validation fails before reaching them

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid edge type")
	// No repo assertions since they're not called due to early validation failure
}

func TestCreateEdgeHandler_Handle_BidirectionalEdge(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithNodes(sourceNode, targetNode).
		MustBuild()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "normal", // Use a valid edge type
		Weight:   1.0,
	}

	// Setup mocks
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Commit", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetByID", ctx, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByID", ctx, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", ctx, targetNode.ID()).Return(targetNode, nil)

	// Save expectations using UoW pattern
	mockGraphRepo.On("SaveWithUoW", ctx, graph, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, sourceNode, mockUoW).Return(nil)
	mockNodeRepo.On("SaveWithUoW", ctx, targetNode, mockUoW).Return(nil)

	// Event publishing expects batch
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	// Act
	err := handler.Handle(ctx, &cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEdgeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

// Benchmarks

func BenchmarkCreateEdgeHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEventBus := new(mocks.MockEventBus)

	sourceNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	targetNode := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	graph := fixtures.NewGraphBuilder().
		WithUserID("user123").
		WithID("graph123").
		WithNodes(sourceNode, targetNode).
		MustBuild()

	cmd := commands.CreateEdgeCommand{
		EdgeID:   uuid.New().String(),
		UserID:   "user123",
		GraphID:  "graph123",
		SourceID: sourceNode.ID().String(),
		TargetID: targetNode.ID().String(),
		Type:     "normal",
		Weight:   1.0,
	}

	mockGraphRepo.On("GetByID", mock.Anything, aggregates.GraphID("graph123")).Return(graph, nil)
	mockNodeRepo.On("GetByID", mock.Anything, sourceNode.ID()).Return(sourceNode, nil)
	mockNodeRepo.On("GetByID", mock.Anything, targetNode.ID()).Return(targetNode, nil)
	mockEdgeRepo.On("Save", mock.Anything, "graph123", mock.AnythingOfType("*aggregates.Edge")).Return(nil)
	mockGraphRepo.On("Save", mock.Anything, graph).Return(nil)
	mockEventBus.On("Publish", mock.Anything, mock.AnythingOfType("events.EdgeCreatedEvent")).Return(nil)

	handler := NewCreateEdgeHandler(mockUoW, mockNodeRepo, mockGraphRepo, mockEdgeRepo, mockEventBus)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, &cmd)
	}
}