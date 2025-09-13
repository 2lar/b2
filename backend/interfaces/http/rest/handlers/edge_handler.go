package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend/application/commands"
	"backend/application/mediator"
	"backend/pkg/auth"
	"backend/pkg/errors"
	"backend/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EdgeHandler handles edge-related HTTP requests
type EdgeHandler struct {
	mediator     mediator.IMediator
	logger       *zap.Logger
	errorHandler *errors.ErrorHandler
}

// NewEdgeHandler creates a new edge handler
func NewEdgeHandler(med mediator.IMediator, logger *zap.Logger, errorHandler *errors.ErrorHandler) *EdgeHandler {
	return &EdgeHandler{
		mediator:     med,
		logger:       logger,
		errorHandler: errorHandler,
	}
}

// CreateEdgeRequest represents the request body for creating an edge
type CreateEdgeRequest struct {
	SourceID string  `json:"source_id" validate:"required,uuid"`
	TargetID string  `json:"target_id" validate:"required,uuid"`
	Type     string  `json:"type,omitempty" validate:"omitempty,oneof=reference dependency parent child related"`
	Weight   float64 `json:"weight,omitempty" validate:"omitempty,min=0,max=1"`
}

// CreateEdge handles POST /edges
func (h *EdgeHandler) CreateEdge(w http.ResponseWriter, r *http.Request) {
	var req CreateEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body: "+err.Error()))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Set defaults
	if req.Type == "" {
		req.Type = "related"
	}
	if req.Weight == 0 {
		req.Weight = 1.0
	}

	// Generate edge ID
	edgeID := uuid.New().String()

	// Create command
	cmd := commands.CreateEdgeCommand{
		EdgeID:   edgeID,
		UserID:   userCtx.UserID,
		SourceID: req.SourceID,
		TargetID: req.TargetID,
		Type:     req.Type,
		Weight:   req.Weight,
	}

	// Execute command
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to create edge",
			zap.String("userID", userCtx.UserID),
			zap.String("sourceID", req.SourceID),
			zap.String("targetID", req.TargetID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "validation") {
			h.errorHandler.Handle(w, r, errors.NewValidationError(err.Error()))
		} else if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("One or both nodes not found"))
		} else if strings.Contains(err.Error(), "already exists") {
			h.errorHandler.Handle(w, r, errors.NewConflictError("Edge already exists"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to create edge").WithCause(err))
		}
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":        edgeID,
		"message":   "Edge created successfully",
		"createdAt": utils.NowRFC3339(),
	})
}

// DeleteEdge handles DELETE /edges/{edgeID}
func (h *EdgeHandler) DeleteEdge(w http.ResponseWriter, r *http.Request) {
	edgeID := chi.URLParam(r, "edgeID")
	if edgeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Edge ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(edgeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid edge ID format"))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Create command
	cmd := commands.DeleteEdgeCommand{
		UserID: userCtx.UserID,
		EdgeID: edgeID,
	}

	// Execute command
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to delete edge",
			zap.String("edgeID", edgeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Edge not found"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to delete edge").WithCause(err))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods

func (h *EdgeHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

