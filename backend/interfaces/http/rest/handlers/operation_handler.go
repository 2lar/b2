package handlers

import (
	"net/http"

	"backend/application/mediator"
	"backend/application/queries"
	"backend/pkg/auth"
	"backend/pkg/common"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// OperationHandler handles operation status endpoints
type OperationHandler struct {
	mediator mediator.IMediator
	logger   *zap.Logger
}

// NewOperationHandler creates a new operation handler
func NewOperationHandler(mediator mediator.IMediator, logger *zap.Logger) *OperationHandler {
	return &OperationHandler{
		mediator: mediator,
		logger:   logger,
	}
}

// GetOperationStatus handles GET /operations/{operationID}
func (h *OperationHandler) GetOperationStatus(w http.ResponseWriter, r *http.Request) {
	operationID := chi.URLParam(r, "operationID")
	if operationID == "" {
		common.RespondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Operation ID is required")
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		common.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
		return
	}

	// Create query
	query := queries.GetOperationStatusQuery{
		OperationID: operationID,
		UserID:      userCtx.UserID,
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Debug("Failed to get operation status",
			zap.String("operationID", operationID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		common.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Operation not found")
		return
	}

	// Type assert and respond
	statusResult, ok := result.(*queries.OperationStatusResult)
	if !ok {
		h.logger.Error("Invalid result type from operation status query")
		common.RespondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to process operation status")
		return
	}

	common.RespondJSON(w, http.StatusOK, statusResult)
}