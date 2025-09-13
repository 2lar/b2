package integration_test

import (
	"context"
	"testing"
	"time"

	"backend/application/commands"
	"backend/application/commands/handlers"
	"backend/application/ports"
	"backend/application/sagas"
	"backend/application/services"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/infrastructure/config"
	"backend/infrastructure/persistence/dynamodb"
	"backend/infrastructure/persistence/memory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Mock implementations for testing

type MockUnitOfWork struct {
	mock.Mock
}

func (m *MockUnitOfWork) Begin(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUnitOfWork) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUnitOfWork) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

type MockNodeRepository struct {
	mock.Mock
	nodes map[valueobjects.NodeID]*entities.Node
}

func NewMockNodeRepository() *MockNodeRepository {
	return &MockNodeRepository{
		nodes: make(map[valueobjects.NodeID]*entities.Node),
	}
}

func (m *MockNodeRepository) Save(ctx context.Context, node *entities.Node) error {
	args := m.Called(ctx, node)
	if args.Error(0) == nil {
		m.nodes[node.ID()] = node
	}
	return args.Error(0)
}

func (m *MockNodeRepository) SaveWithUoW(ctx context.Context, node *entities.Node, uow interface{}) error {
	args := m.Called(ctx, node, uow)
	if args.Error(0) == nil {
		m.nodes[node.ID()] = node
	}
	return args.Error(0)
}

func (m *MockNodeRepository) GetByID(ctx context.Context, id valueobjects.NodeID) (*entities.Node, error) {
	args := m.Called(ctx, id)
	if node, ok := m.nodes[id]; ok {
		return node, args.Error(1)
	}
	return args.Get(0).(*entities.Node), args.Error(1)
}

func (m *MockNodeRepository) Delete(ctx context.Context, id valueobjects.NodeID) error {
	args := m.Called(ctx, id)
	if args.Error(0) == nil {
		delete(m.nodes, id)
	}
	return args.Error(0)
}

func (m *MockNodeRepository) GetByUserID(ctx context.Context, userID string) ([]*entities.Node, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*entities.Node), args.Error(1)
}

func (m *MockNodeRepository) GetByGraphID(ctx context.Context, graphID string) ([]*entities.Node, error) {
	args := m.Called(ctx, graphID)
	return args.Get(0).([]*entities.Node), args.Error(1)
}

func (m *MockNodeRepository) Update(ctx context.Context, node *entities.Node) error {
	args := m.Called(ctx, node)
	return args.Error(0)
}

type MockGraphRepository struct {
	mock.Mock
	graphs map[aggregates.GraphID]*aggregates.Graph
}

func NewMockGraphRepository() *MockGraphRepository {
	return &MockGraphRepository{
		graphs: make(map[aggregates.GraphID]*aggregates.Graph),
	}
}

func (m *MockGraphRepository) GetUserDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) != nil {
		return args.Get(0).(*aggregates.Graph), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGraphRepository) GetOrCreateDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*aggregates.Graph), args.Error(1)
}

func (m *MockGraphRepository) Save(ctx context.Context, graph *aggregates.Graph) error {
	args := m.Called(ctx, graph)
	if args.Error(0) == nil {
		m.graphs[graph.ID()] = graph
	}
	return args.Error(0)
}

func (m *MockGraphRepository) SaveWithUoW(ctx context.Context, graph *aggregates.Graph, uow interface{}) error {
	args := m.Called(ctx, graph, uow)
	if args.Error(0) == nil {
		m.graphs[graph.ID()] = graph
	}
	return args.Error(0)
}

func (m *MockGraphRepository) GetByID(ctx context.Context, id aggregates.GraphID) (*aggregates.Graph, error) {
	args := m.Called(ctx, id)
	if graph, ok := m.graphs[id]; ok {
		return graph, args.Error(1)
	}
	return args.Get(0).(*aggregates.Graph), args.Error(1)
}

func (m *MockGraphRepository) UpdateGraphMetadata(ctx context.Context, graphID string) error {
	args := m.Called(ctx, graphID)
	return args.Error(0)
}

func (m *MockGraphRepository) GetByUserID(ctx context.Context, userID string) ([]*aggregates.Graph, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*aggregates.Graph), args.Error(1)
}

type MockEdgeRepository struct {
	mock.Mock
}

func (m *MockEdgeRepository) Save(ctx context.Context, graphID string, edge *aggregates.Edge) error {
	args := m.Called(ctx, graphID, edge)
	return args.Error(0)
}

func (m *MockEdgeRepository) GetByGraphID(ctx context.Context, graphID string) ([]*aggregates.Edge, error) {
	args := m.Called(ctx, graphID)
	return args.Get(0).([]*aggregates.Edge), args.Error(1)
}

func (m *MockEdgeRepository) Delete(ctx context.Context, graphID string, edgeID string) error {
	args := m.Called(ctx, graphID, edgeID)
	return args.Error(0)
}

func (m *MockEdgeRepository) GetBySourceNode(ctx context.Context, graphID string, sourceID valueobjects.NodeID) ([]*aggregates.Edge, error) {
	args := m.Called(ctx, graphID, sourceID)
	return args.Get(0).([]*aggregates.Edge), args.Error(1)
}

func (m *MockEdgeRepository) GetByTargetNode(ctx context.Context, graphID string, targetID valueobjects.NodeID) ([]*aggregates.Edge, error) {
	args := m.Called(ctx, graphID, targetID)
	return args.Get(0).([]*aggregates.Edge), args.Error(1)
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishBatch(ctx context.Context, events []interface{}) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

// Test the full saga integration
func TestCreateNodeSagaHandler_Integration(t *testing.T) {
	// Arrange
	ctx := context.Background()
	logger := zap.NewNop()
	
	// Create mocks
	uow := new(MockUnitOfWork)
	nodeRepo := NewMockNodeRepository()
	graphRepo := NewMockGraphRepository()
	edgeRepo := new(MockEdgeRepository)
	eventPublisher := new(MockEventPublisher)
	
	// Create real services
	edgeService := &services.EdgeService{}
	operationStore := memory.NewInMemoryOperationStore(1 * time.Hour)
	
	// Create config
	cfg := &config.Config{
		Environment: "test",
		EnableLazyLoading: false,
		EdgeCreation: config.EdgeCreationConfig{
			SyncEdgeLimit: 5,
			AsyncEnabled: false,
		},
		Features: config.Features{
			EnableSagaOrchestrator: true,
		},
	}
	
	// Create default graph
	defaultGraph, _ := aggregates.NewGraph("user-123", "Default Graph")
	
	// Setup mock expectations
	uow.On("Begin", ctx).Return(nil)
	uow.On("Commit", ctx).Return(nil)
	uow.On("Rollback").Return(nil)
	
	graphRepo.On("GetUserDefaultGraph", ctx, "user-123").Return(defaultGraph, nil)
	graphRepo.On("SaveWithUoW", ctx, mock.Anything, uow).Return(nil)
	graphRepo.On("UpdateGraphMetadata", ctx, mock.Anything).Return(nil)
	
	nodeRepo.On("SaveWithUoW", ctx, mock.Anything, uow).Return(nil)
	
	eventPublisher.On("PublishBatch", ctx, mock.Anything).Return(nil)
	
	// Create saga handler
	handler := handlers.NewCreateNodeSagaHandler(
		uow,
		nodeRepo,
		graphRepo,
		edgeRepo,
		edgeService,
		nil, // graphLazyService
		eventPublisher,
		nil, // distributedLock
		operationStore,
		&cfg.EdgeCreation,
		cfg,
		logger,
	)
	
	// Create command
	cmd := commands.CreateNodeCommand{
		UserID:  "user-123",
		Title:   "Integration Test Node",
		Content: "This is a test node created by saga",
		Tags:    []string{"test", "integration"},
		X:       100,
		Y:       200,
		Z:       0,
	}
	
	// Act
	err := handler.Handle(ctx, cmd)
	
	// Assert
	require.NoError(t, err)
	
	// Verify all mocks were called as expected
	uow.AssertExpectations(t)
	graphRepo.AssertExpectations(t)
	nodeRepo.AssertExpectations(t)
	eventPublisher.AssertExpectations(t)
	
	// Verify operation was tracked
	// Note: In a real test, we'd check the operation store
}

func TestCreateNodeSagaHandler_Rollback(t *testing.T) {
	// Arrange
	ctx := context.Background()
	logger := zap.NewNop()
	
	// Create mocks
	uow := new(MockUnitOfWork)
	nodeRepo := NewMockNodeRepository()
	graphRepo := NewMockGraphRepository()
	edgeRepo := new(MockEdgeRepository)
	eventPublisher := new(MockEventPublisher)
	
	// Create config with saga enabled
	cfg := &config.Config{
		Features: config.Features{
			EnableSagaOrchestrator: true,
		},
	}
	
	// Create saga
	saga := sagas.NewCreateNodeSaga(
		uow,
		nodeRepo,
		graphRepo,
		edgeRepo,
		nil, // edgeService
		nil, // graphLazyService
		eventPublisher,
		nil, // distributedLock
		nil, // operationStore
		&cfg.EdgeCreation,
		cfg,
		logger,
	)
	
	// Setup mock expectations for failure scenario
	uow.On("Begin", ctx).Return(nil)
	uow.On("Rollback").Return(nil)
	
	// Graph creation fails, triggering compensation
	graphRepo.On("GetUserDefaultGraph", ctx, "user-123").Return(nil, assert.AnError)
	
	// Prepare saga data
	data := &sagas.CreateNodeSagaData{
		UserID:      "user-123",
		Title:       "Test Node",
		Content:     "Test content",
		OperationID: "op-rollback",
		StartTime:   time.Now(),
	}
	
	// Act
	err := saga.Execute(ctx, data)
	
	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create default graph")
	
	// Verify rollback was called
	uow.AssertCalled(t, "Rollback")
	uow.AssertExpectations(t)
	graphRepo.AssertExpectations(t)
}

func TestCreateNodeSaga_FeatureFlagDisabled(t *testing.T) {
	// Arrange
	ctx := context.Background()
	logger := zap.NewNop()
	
	// Create config with saga disabled
	cfg := &config.Config{
		Features: config.Features{
			EnableSagaOrchestrator: false, // Disabled
		},
	}
	
	// Create handler
	handler := handlers.NewCreateNodeSagaHandler(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		&cfg.EdgeCreation,
		cfg,
		logger,
	)
	
	// Create command
	cmd := commands.CreateNodeCommand{
		UserID: "user-123",
		Title:  "Test Node",
	}
	
	// Act
	err := handler.Handle(ctx, cmd)
	
	// Assert
	// When saga is disabled, it should fall back to orchestrator
	// In this test, we expect it to return nil (no-op in fallback)
	require.NoError(t, err)
}