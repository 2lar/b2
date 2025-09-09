package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"backend/application/queries"
	querybus "backend/application/queries/bus"
	"backend/pkg/auth"
	"go.uber.org/zap"
)

// SearchHandler handles search-related HTTP requests
type SearchHandler struct {
	queryBus *querybus.QueryBus
	logger   *zap.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(queryBus *querybus.QueryBus, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{
		queryBus: queryBus,
		logger:   logger,
	}
}

// Search handles GET /search
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.respondError(w, http.StatusBadRequest, "Search query is required")
		return
	}

	// Get user from context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	// Parse tags from comma-separated list
	tagsParam := r.URL.Query().Get("tags")
	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	// Create search query
	searchQuery := queries.SearchNodesQuery{
		UserID:   userCtx.UserID,
		Keywords: strings.Fields(query), // Split query into keywords
		Tags:     tags,
		Limit:    limit,
		Offset:   offset,
	}

	// Validate query
	if err := searchQuery.Validate(); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Execute query
	result, err := h.queryBus.Ask(r.Context(), &searchQuery)
	if err != nil {
		h.logger.Error("Search failed",
			zap.String("query", query),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "Search failed")
		return
	}

	// Format response
	searchResult, ok := result.(*queries.SearchNodesResult)
	if !ok {
		h.respondError(w, http.StatusInternalServerError, "Invalid search result")
		return
	}

	// Convert to API response format
	response := map[string]interface{}{
		"query":    query,
		"results":  searchResult.Nodes,
		"total":    searchResult.TotalCount,
		"offset":   searchResult.Offset,
		"limit":    limit,
		"has_more": searchResult.Offset+limit < searchResult.TotalCount,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *SearchHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

func (h *SearchHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
