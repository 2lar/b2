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
	"go.uber.org/zap"
)

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}

// Helper function to create string slice pointer
func strSlicePtr(s []string) *[]string {
	return &s
}

func TestUpdateNodeHandler_Handle_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node
	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Original Title").
		WithContent("Original content").
		MustBuild()

	cmd := commands.UpdateNodeCommand{
		NodeID:  node.ID().String(),
		UserID:  "user123",
		Title:   strPtr("Updated Title"),
		Content: strPtr("Updated content"),
		X:       floatPtr(100),
		Y:       floatPtr(200),
		Z:       floatPtr(0),
		Tags:    strSlicePtr([]string{"updated", "test"}),
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Save", ctx, mock.AnythingOfType("*entities.Node")).Return(nil)
	// The handler publishes events in batch
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestUpdateNodeHandler_Handle_NodeNotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	nodeID := valueobjects.NewNodeID()
	cmd := commands.UpdateNodeCommand{
		NodeID:  nodeID.String(),
		UserID:  "user123",
		Title:   strPtr("Updated Title"),
		Content: strPtr("Updated content"),
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, nodeID).Return(nil, errors.New("node not found"))

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node not found")
	mockNodeRepo.AssertExpectations(t)
}

func TestUpdateNodeHandler_Handle_UnauthorizedUser(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	// Create test node owned by different user
	node := fixtures.NewNodeBuilder().
		WithUserID("different-user").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.UpdateNodeCommand{
		NodeID:  node.ID().String(),
		UserID:  "user123", // Different user trying to update
		Title:   strPtr("Hacker Title"),
		Content: strPtr("Hacked content"),
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node does not belong to user")
	mockNodeRepo.AssertExpectations(t)
}

func TestUpdateNodeHandler_Handle_InvalidNodeID(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	cmd := commands.UpdateNodeCommand{
		NodeID:  "invalid-uuid",
		UserID:  "user123",
		Title:   strPtr("Title"),
		Content: strPtr("Content"),
	}

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid node ID")
}

func TestUpdateNodeHandler_Handle_SaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.UpdateNodeCommand{
		NodeID:  node.ID().String(),
		UserID:  "user123",
		Title:   strPtr("Updated Title"),
		Content: strPtr("Updated content"),
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Save", ctx, mock.AnythingOfType("*entities.Node")).Return(errors.New("save failed"))

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save failed")
	mockNodeRepo.AssertExpectations(t)
}

func TestUpdateNodeHandler_Handle_PartialUpdate(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTitle("Original Title").
		WithContent("Original content").
		WithPosition(10, 20, 30).
		MustBuild()

	// Only update title, keep other fields unchanged
	cmd := commands.UpdateNodeCommand{
		NodeID: node.ID().String(),
		UserID: "user123",
		Title:  strPtr("New Title Only"),
		// Content, position, and tags not provided - should keep original values
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Save", ctx, mock.AnythingOfType("*entities.Node")).Return(nil)
	// The handler publishes events in batch
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestUpdateNodeHandler_Handle_UpdateTags(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		WithTags("old", "tags").
		MustBuild()

	cmd := commands.UpdateNodeCommand{
		NodeID:  node.ID().String(),
		UserID:  "user123",
		Title:   strPtr("Title"),
		Content: strPtr("Content"),
		Tags:    strSlicePtr([]string{"new", "tags", "updated"}),
	}

	// Setup mocks
	mockNodeRepo.On("GetByID", ctx, node.ID()).Return(node, nil)
	mockNodeRepo.On("Save", ctx, mock.AnythingOfType("*entities.Node")).Return(nil)
	// The handler publishes events in batch
	mockEventBus.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	// Act
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockNodeRepo.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

// Benchmarks

func BenchmarkUpdateNodeHandler_Handle(b *testing.B) {
	ctx := context.Background()
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockEventBus := new(mocks.MockEventBus)
	logger := zap.NewNop()

	node := fixtures.NewNodeBuilder().
		WithUserID("user123").
		WithGraphID("graph123").
		MustBuild()

	cmd := commands.UpdateNodeCommand{
		NodeID:  node.ID().String(),
		UserID:  "user123",
		Title:   strPtr("Updated"),
		Content: strPtr("Updated"),
	}

	mockNodeRepo.On("GetByID", mock.Anything, node.ID()).Return(node, nil)
	mockNodeRepo.On("Save", mock.Anything, mock.AnythingOfType("*entities.Node")).Return(nil)
	mockEventBus.On("PublishBatch", mock.Anything, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)

	mockEventStore := new(mocks.MockEventStore)
	handler := NewUpdateNodeHandler(mockNodeRepo, mockEventStore, mockEventBus, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, cmd)
	}
}