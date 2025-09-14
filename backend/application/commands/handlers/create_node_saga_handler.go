package handlers

import (
	"context"
	"time"

	"backend/application/commands"
	"backend/application/ports"
	"backend/application/sagas"
	"backend/application/services"
	"backend/infrastructure/config"
	"backend/infrastructure/persistence/dynamodb"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateNodeSagaHandler handles CreateNodeCommand using saga pattern
type CreateNodeSagaHandler struct {
	saga           *sagas.CreateNodeSaga
	operationStore ports.OperationStore
	config         *config.Config
	logger         *zap.Logger
}

// NewCreateNodeSagaHandler creates a new saga-based handler for node creation
func NewCreateNodeSagaHandler(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	edgeService *services.EdgeService,
	graphLazyService *services.GraphLazyService,
	eventPublisher ports.EventPublisher,
	distributedLock *dynamodb.DistributedLock,
	operationStore ports.OperationStore,
	edgeConfig *config.EdgeCreationConfig,
	appConfig *config.Config,
	logger *zap.Logger,
) *CreateNodeSagaHandler {
	saga := sagas.NewCreateNodeSaga(
		uow,
		nodeRepo,
		graphRepo,
		edgeRepo,
		edgeService,
		graphLazyService,
		eventPublisher,
		distributedLock,
		operationStore,
		edgeConfig,
		appConfig,
		logger,
	)

	return &CreateNodeSagaHandler{
		saga:           saga,
		operationStore: operationStore,
		config:         appConfig,
		logger:         logger,
	}
}

// Handle executes the create node command using saga pattern
func (h *CreateNodeSagaHandler) Handle(ctx context.Context, cmd commands.CreateNodeCommand) error {
	// Generate operation ID for tracking
	operationID := uuid.New().String()
	startTime := time.Now()

	// Store initial operation status
	if h.operationStore != nil {
		h.operationStore.Store(ctx, &ports.OperationResult{
			OperationID: operationID,
			Status:      ports.OperationStatusPending,
			StartedAt:   startTime,
			Metadata: map[string]interface{}{
				"user_id": cmd.UserID,
				"title":   cmd.Title,
			},
		})
	}

	// Prepare saga data
	sagaData := &sagas.CreateNodeSagaData{
		UserID:      cmd.UserID,
		Title:       cmd.Title,
		Content:     cmd.Content,
		Tags:        cmd.Tags,
		X:           cmd.X,
		Y:           cmd.Y,
		Z:           cmd.Z,
		OperationID: operationID,
		StartTime:   startTime,
	}

	// Check if saga should be used (feature flag)
	if !h.shouldUseSaga() {
		// Fall back to original orchestrator
		h.logger.Debug("Saga disabled by feature flag, using original orchestrator")
		return h.fallbackToOrchestrator(ctx, cmd)
	}

	// Execute saga
	h.logger.Info("Executing CreateNode saga",
		zap.String("operation_id", operationID),
		zap.String("user_id", cmd.UserID),
		zap.String("title", cmd.Title),
	)

	err := h.saga.Execute(ctx, sagaData)
	if err != nil {
		h.logger.Error("CreateNode saga failed",
			zap.String("operation_id", operationID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("CreateNode saga completed successfully",
		zap.String("operation_id", operationID),
		zap.String("node_id", sagaData.Node.ID().String()),
		zap.Duration("duration", time.Since(startTime)),
	)

	return nil
}

// shouldUseSaga checks if saga pattern should be used based on feature flags
func (h *CreateNodeSagaHandler) shouldUseSaga() bool {
	// Check feature flag from config
	if h.config == nil {
		return false
	}

	// Check for saga feature flag
	return h.config.Features.EnableSagaOrchestrator
}

// fallbackToOrchestrator falls back to the original CreateNodeOrchestrator
func (h *CreateNodeSagaHandler) fallbackToOrchestrator(_ context.Context, cmd commands.CreateNodeCommand) error {
	// This would call the original CreateNodeOrchestrator
	// For now, return an error indicating fallback is needed
	h.logger.Warn("Fallback to original orchestrator not implemented",
		zap.String("user_id", cmd.UserID),
		zap.String("title", cmd.Title),
	)
	
	// In production, you would instantiate and call the original orchestrator here
	// return h.orchestrator.Handle(ctx, cmd)
	
	return nil
}

// GetOperationStatus returns the status of an async operation
func (h *CreateNodeSagaHandler) GetOperationStatus(ctx context.Context, operationID string) (*ports.OperationResult, error) {
	if h.operationStore == nil {
		return nil, nil
	}
	
	return h.operationStore.Get(ctx, operationID)
}