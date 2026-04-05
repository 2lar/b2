package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"backend/application/services"
	"backend/pkg/auth"
	"backend/pkg/errors"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// AnalysisHandler handles thought chain and impact analysis HTTP requests.
type AnalysisHandler struct {
	analysisService *services.AnalysisService
	logger          *zap.Logger
	errorHandler    *errors.ErrorHandler
}

// NewAnalysisHandler creates a new handler.
func NewAnalysisHandler(
	analysisService *services.AnalysisService,
	logger *zap.Logger,
	errorHandler *errors.ErrorHandler,
) *AnalysisHandler {
	return &AnalysisHandler{
		analysisService: analysisService,
		logger:          logger,
		errorHandler:    errorHandler,
	}
}

// GetThoughtChains handles GET /nodes/{nodeID}/chains
func (h *AnalysisHandler) GetThoughtChains(w http.ResponseWriter, r *http.Request) {
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("nodeID is required"))
		return
	}

	maxDepth := queryInt(r, "maxDepth", 10)
	maxBranches := queryInt(r, "maxBranches", 4)

	result, err := h.analysisService.GetThoughtChains(r.Context(), userCtx.UserID, nodeID, maxDepth, maxBranches)
	if err != nil {
		h.logger.Error("Thought chain tracing failed",
			zap.String("userID", userCtx.UserID),
			zap.String("nodeID", nodeID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to trace thought chains").WithCause(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// GetImpactAnalysis handles GET /nodes/{nodeID}/impact
func (h *AnalysisHandler) GetImpactAnalysis(w http.ResponseWriter, r *http.Request) {
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("nodeID is required"))
		return
	}

	maxDepth := queryInt(r, "maxDepth", 3)

	result, err := h.analysisService.GetImpactAnalysis(r.Context(), userCtx.UserID, nodeID, maxDepth)
	if err != nil {
		h.logger.Error("Impact analysis failed",
			zap.String("userID", userCtx.UserID),
			zap.String("nodeID", nodeID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to analyze impact").WithCause(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
