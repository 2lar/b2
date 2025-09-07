package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"backend2/application/queries"
	querybus "backend2/application/queries/bus"
	"backend2/pkg/auth"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// GraphHandler handles graph-related HTTP requests
type GraphHandler struct {
	queryBus *querybus.QueryBus
	logger   *zap.Logger
}

// NewGraphHandler creates a new graph handler
func NewGraphHandler(queryBus *querybus.QueryBus, logger *zap.Logger) *GraphHandler {
	return &GraphHandler{
		queryBus: queryBus,
		logger:   logger,
	}
}

// GetGraph handles GET /graphs/{graphID}
func (h *GraphHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	graphID := chi.URLParam(r, "graphID")
	if graphID == "" {
		h.respondError(w, http.StatusBadRequest, "Graph ID is required")
		return
	}
	
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// Create query
	query := queries.GetGraphByIDQuery{
		UserID:  userCtx.UserID,
		GraphID: graphID,
	}
	
	// Execute query
	result, err := h.queryBus.Ask(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get graph",
			zap.String("graphID", graphID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.respondError(w, http.StatusNotFound, "Graph not found")
		return
	}
	
	h.respondJSON(w, http.StatusOK, result)
}

// ListGraphs handles GET /graphs
func (h *GraphHandler) ListGraphs(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
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
	result, err := h.queryBus.Ask(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to list graphs",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to list graphs")
		return
	}
	
	h.respondJSON(w, http.StatusOK, result)
}

// GetGraphData handles GET /graph-data
func (h *GraphHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
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
	result, err := h.queryBus.Ask(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get graph data",
			zap.String("userID", userCtx.UserID),
			zap.String("graphID", graphID),
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to get graph data")
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

func (h *GraphHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]interface{}{
		"error":   true,
		"message": message,
		"code":    status,
	})
}