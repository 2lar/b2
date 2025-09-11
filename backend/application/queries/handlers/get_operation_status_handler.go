package handlers

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/application/queries"
	queries_bus "backend/application/queries/bus"
	"go.uber.org/zap"
)

// GetOperationStatusHandler handles operation status queries
type GetOperationStatusHandler struct {
	operationStore ports.OperationStore
	logger         *zap.Logger
}

// NewGetOperationStatusHandler creates a new operation status handler
func NewGetOperationStatusHandler(
	operationStore ports.OperationStore,
	logger *zap.Logger,
) *GetOperationStatusHandler {
	return &GetOperationStatusHandler{
		operationStore: operationStore,
		logger:         logger,
	}
}

// Handle executes the operation status query
func (h *GetOperationStatusHandler) Handle(ctx context.Context, query queries_bus.Query) (interface{}, error) {
	// Type assert the query
	q, ok := query.(queries.GetOperationStatusQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	// Validate query
	if err := q.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Get operation from store
	operation, err := h.operationStore.Get(ctx, q.OperationID)
	if err != nil {
		h.logger.Debug("Operation not found",
			zap.String("operationID", q.OperationID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("operation not found: %s", q.OperationID)
	}

	// Verify ownership (operation should have userID in metadata)
	if userID, ok := operation.Metadata["user_id"].(string); ok {
		if userID != q.UserID {
			return nil, fmt.Errorf("operation does not belong to user")
		}
	}

	// Map to result
	result := &queries.OperationStatusResult{
		OperationID: operation.OperationID,
		Status:      string(operation.Status),
		StartedAt:   operation.StartedAt,
		CompletedAt: operation.CompletedAt,
		Result:      operation.Result,
		Error:       operation.Error,
		Metadata:    operation.Metadata,
	}

	h.logger.Debug("Operation status retrieved",
		zap.String("operationID", q.OperationID),
		zap.String("status", result.Status),
	)

	return result, nil
}