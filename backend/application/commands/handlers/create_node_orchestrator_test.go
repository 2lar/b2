package handlers

import (
	"context"
	"errors"
	"testing"

	"backend/application/commands"
	"backend/application/services"
	"backend/domain/core/aggregates"
	"backend/infrastructure/config"
	"backend/infrastructure/persistence/dynamodb"
	"backend/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockLogger implements the Logger interface for testing
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *MockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {}

func TestCreateNodeOrchestrator_Handle_Success(t *testing.T) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command with proper structure
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Create test graph
	graph, _ := aggregates.NewGraph("user123", "Default Graph")

	// Setup expectations
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil).Maybe()
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)

	// The orchestrator will save the node with UoW
	mockNodeRepo.On("SaveWithUoW", ctx, mock.AnythingOfType("*entities.Node"), mockUoW).Return(nil)

	// The orchestrator will save the graph with UoW
	mockGraphRepo.On("SaveWithUoW", ctx, graph, mockUoW).Return(nil)

	mockUoW.On("Commit", ctx).Return(nil)
	mockEventPublisher.On("PublishBatch", ctx, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)
	mockGraphRepo.On("UpdateGraphMetadata", ctx, mock.AnythingOfType("string")).Return(nil)

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	distributedLock := &dynamodb.DistributedLock{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	// Execute - Handle returns only error
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
	mockEventPublisher.AssertExpectations(t)
}

func TestCreateNodeOrchestrator_Handle_TransactionBeginError(t *testing.T) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Setup expectations - transaction begin fails
	mockUoW.On("Begin", ctx).Return(errors.New("transaction error"))

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	distributedLock := &dynamodb.DistributedLock{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	// Execute
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction error")
	mockUoW.AssertExpectations(t)
}

func TestCreateNodeOrchestrator_Handle_GraphCreationError(t *testing.T) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Setup expectations
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(nil, errors.New("graph not found"))
	// The orchestrator will create both a graph and save the node, so we need to mock both calls
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(nil, errors.New("graph not found")).Maybe() // Double-check call after lock
	mockGraphRepo.On("SaveWithUoW", ctx, mock.AnythingOfType("*aggregates.Graph"), mock.AnythingOfType("*mocks.MockUnitOfWork")).Return(errors.New("graph save failed"))
	mockNodeRepo.On("SaveWithUoW", ctx, mock.AnythingOfType("*entities.Node"), mock.AnythingOfType("*mocks.MockUnitOfWork")).Return(nil)

	// Mock the distributed lock for graph creation - use nil to skip distributed locking in tests
	var distributedLock *dynamodb.DistributedLock = nil

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	// Execute
	err := handler.Handle(ctx, cmd)

	// Assert - since GetUserDefaultGraph fails, it will try to acquire lock and create
	// But for this test we want to simulate the error path
	assert.Error(t, err)
	mockUoW.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

func TestCreateNodeOrchestrator_Handle_NodeSaveError(t *testing.T) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Create test graph
	graph, _ := aggregates.NewGraph("user123", "Default Graph")

	// Setup expectations
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockNodeRepo.On("SaveWithUoW", ctx, mock.AnythingOfType("*entities.Node"), mockUoW).Return(errors.New("save error"))

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	distributedLock := &dynamodb.DistributedLock{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	// Execute
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

func TestCreateNodeOrchestrator_Handle_CommitError(t *testing.T) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Create test graph
	graph, _ := aggregates.NewGraph("user123", "Default Graph")

	// Setup expectations
	mockUoW.On("Begin", ctx).Return(nil)
	mockUoW.On("Rollback").Return(nil)
	mockGraphRepo.On("GetUserDefaultGraph", ctx, "user123").Return(graph, nil)
	mockNodeRepo.On("SaveWithUoW", ctx, mock.AnythingOfType("*entities.Node"), mockUoW).Return(nil)
	mockGraphRepo.On("SaveWithUoW", ctx, graph, mockUoW).Return(nil)
	mockUoW.On("Commit", ctx).Return(errors.New("commit error"))

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	distributedLock := &dynamodb.DistributedLock{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	// Execute
	err := handler.Handle(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "commit error")
	mockUoW.AssertExpectations(t)
	mockNodeRepo.AssertExpectations(t)
	mockGraphRepo.AssertExpectations(t)
}

// Benchmarks

func BenchmarkCreateNodeOrchestrator_Handle(b *testing.B) {
	ctx := context.Background()

	// Create mocks
	mockUoW := new(mocks.MockUnitOfWork)
	mockNodeRepo := new(mocks.MockNodeRepository)
	mockGraphRepo := new(mocks.MockGraphRepository)
	mockEdgeRepo := new(mocks.MockEdgeRepository)
	mockEventPublisher := new(mocks.MockEventPublisher)

	// Create test command
	cmd := commands.CreateNodeCommand{
		NodeID:  "node123",
		UserID:  "user123",
		Title:   "Test Node",
		Content: "Test content",
		Format:  "markdown",
		X:       1.0,
		Y:       2.0,
		Z:       3.0,
		Tags:    []string{"test"},
	}

	// Create test graph
	graph, _ := aggregates.NewGraph("user123", "Default Graph")

	// Setup expectations
	mockUoW.On("Begin", mock.Anything).Return(nil)
	mockUoW.On("Rollback").Return(nil).Maybe()
	mockGraphRepo.On("GetUserDefaultGraph", mock.Anything, "user123").Return(graph, nil)
	mockNodeRepo.On("SaveWithUoW", mock.Anything, mock.AnythingOfType("*entities.Node"), mockUoW).Return(nil)
	mockGraphRepo.On("SaveWithUoW", mock.Anything, graph, mockUoW).Return(nil)
	mockUoW.On("Commit", mock.Anything).Return(nil)
	mockEventPublisher.On("PublishBatch", mock.Anything, mock.AnythingOfType("[]events.DomainEvent")).Return(nil)
	mockGraphRepo.On("UpdateGraphMetadata", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	// Create service dependencies
	edgeConfig := &config.EdgeCreationConfig{
		AsyncEnabled:        true,
		SimilarityThreshold: 0.7,
		MaxEdgesPerNode:     10,
		SyncEdgeLimit:       5,
	}

	logger := &MockLogger{}
	edgeService := services.NewEdgeService(mockNodeRepo, mockGraphRepo, mockEdgeRepo, edgeConfig, zap.NewNop())
	graphLazyService := &services.GraphLazyService{}
	distributedLock := &dynamodb.DistributedLock{}
	appConfig := &config.Config{
		EnableLazyLoading: false,
	}

	// Create handler
	handler := NewCreateNodeOrchestrator(
		mockUoW,
		mockNodeRepo,
		mockGraphRepo,
		mockEdgeRepo,
		edgeService,
		graphLazyService,
		mockEventPublisher,
		distributedLock,
		edgeConfig,
		appConfig,
		logger,
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.Handle(ctx, cmd)
	}
}