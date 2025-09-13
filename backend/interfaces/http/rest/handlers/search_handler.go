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
	"go.uber.org/zap"
)

// SearchHandler handles search-related HTTP requests
type SearchHandler struct {
	mediator     mediator.IMediator
	logger       *zap.Logger
	errorHandler *errors.ErrorHandler
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(mediator mediator.IMediator, logger *zap.Logger, errorHandler *errors.ErrorHandler) *SearchHandler {
	return &SearchHandler{
		mediator:     mediator,
		logger:       logger,
		errorHandler: errorHandler,
	}
}

// Search handles GET /search
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Search query is required"))
		return
	}

	// Get user from context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
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
		h.errorHandler.Handle(w, r, errors.NewValidationError(err.Error()))
		return
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), &searchQuery)
	if err != nil {
		h.logger.Error("Search failed",
			zap.String("query", query),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Search failed").WithCause(err))
		return
	}

	// Format response
	searchResult, ok := result.(*queries.SearchNodesResult)
	if !ok {
		h.errorHandler.Handle(w, r, errors.NewInternalError("Invalid search result"))
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

