package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// Search handles GET /search — hybrid BM25 + semantic search with RRF fusion
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Search query is required"))
		return
	}

	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	searchQuery := &queries.HybridSearchQuery{
		UserID: userCtx.UserID,
		Query:  query,
		Limit:  limit,
		Offset: offset,
	}

	if err := searchQuery.Validate(); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError(err.Error()))
		return
	}

	result, err := h.mediator.Query(r.Context(), searchQuery)
	if err != nil {
		h.logger.Error("Search failed",
			zap.String("query", query),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Search failed").WithCause(err))
		return
	}

	searchResult, ok := result.(*queries.HybridSearchResult)
	if !ok {
		h.errorHandler.Handle(w, r, errors.NewInternalError("Invalid search result"))
		return
	}

	response := map[string]interface{}{
		"query":    searchResult.Query,
		"results":  searchResult.Results,
		"total":    searchResult.Total,
		"offset":   offset,
		"limit":    limit,
		"has_more": offset+limit < searchResult.Total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}
