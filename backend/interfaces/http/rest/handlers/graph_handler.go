package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"backend/application/mediator"
	"backend/application/queries"
	"backend/pkg/auth"
	"backend/pkg/errors"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// GraphHandler handles graph-related HTTP requests
type GraphHandler struct {
	mediator     mediator.IMediator
	logger       *zap.Logger
	errorHandler *errors.ErrorHandler
}

// NewGraphHandler creates a new graph handler
func NewGraphHandler(med mediator.IMediator, logger *zap.Logger, errorHandler *errors.ErrorHandler) *GraphHandler {
	return &GraphHandler{
		mediator:     med,
		logger:       logger,
		errorHandler: errorHandler,
	}
}

// GetGraph handles GET /graphs/{graphID}
func (h *GraphHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	graphID := chi.URLParam(r, "graphID")
	if graphID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Graph ID is required"))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Create query
	query := queries.GetGraphByIDQuery{
		UserID:  userCtx.UserID,
		GraphID: graphID,
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get graph",
			zap.String("graphID", graphID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewNotFoundError("Graph not found"))
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// ListGraphs handles GET /graphs
func (h *GraphHandler) ListGraphs(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	sortBy := r.URL.Query().Get("sort_by")
	order := r.URL.Query().Get("order")

	// Create query
	query := queries.ListGraphsQuery{
		UserID: userCtx.UserID,
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
		Order:  order,
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to list graphs",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to list graphs").WithCause(err))
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// GetGraphData handles GET /graph-data
func (h *GraphHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Get optional graph ID from query params
	graphID := r.URL.Query().Get("graph_id")

	// Create query
	query := queries.GetGraphDataQuery{
		UserID:  userCtx.UserID,
		GraphID: graphID,
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get graph data",
			zap.String("userID", userCtx.UserID),
			zap.String("graphID", graphID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to get graph data").WithCause(err))
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// Helper methods

func (h *GraphHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// GetGraphStats handles GET /graphs/{graphID}/stats
func (h *GraphHandler) GetGraphStats(w http.ResponseWriter, r *http.Request) {
	graphID := chi.URLParam(r, "graphID")
	if graphID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Graph ID is required"))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Create and execute query
	query := queries.GetGraphStatsQuery{
		UserID:  userCtx.UserID,
		GraphID: graphID,
	}

	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get graph stats",
			zap.String("graphID", graphID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Graph not found"))
		} else if strings.Contains(err.Error(), "unauthorized") {
			h.errorHandler.Handle(w, r, errors.NewForbiddenError("Access denied"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to retrieve graph statistics").WithCause(err))
		}
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

