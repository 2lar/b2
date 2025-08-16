// Package handlers provides clean HTTP handlers following best practices.
package handlers

import (
	"encoding/json"
	"net/http"

	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/api"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// MemoryHandler handles HTTP requests for memory operations following clean architecture.
// This handler demonstrates proper separation of concerns by delegating business logic
// to the memory service while handling only HTTP-specific concerns.
type MemoryHandler struct {
	memoryService memory.Service
	logger        *zap.Logger
}

// NewMemoryHandler creates a new memory handler with dependency injection.
func NewMemoryHandler(memoryService memory.Service, logger *zap.Logger) *MemoryHandler {
	if memoryService == nil {
		panic("memoryService is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &MemoryHandler{
		memoryService: memoryService,
		logger:        logger.Named("MemoryHandler"),
	}
}

// CreateMemory handles POST /api/memories
func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req api.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	node, _, err := h.memoryService.CreateNode(r.Context(), userID, req.Content, tags)
	if err != nil {
		h.logger.Error("Failed to create memory", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to create memory")
		return
	}

	response := api.NodeResponse{
		NodeID:    node.ID.String(),
		Content:   node.Content.String(),
		Tags:      node.Tags.ToSlice(),
		Timestamp: node.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Version:   node.Version,
	}

	api.Success(w, http.StatusCreated, response)
}

// GetMemory handles GET /api/memories/{id}
func (h *MemoryHandler) GetMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	memoryID := chi.URLParam(r, "id")
	if memoryID == "" {
		api.Error(w, http.StatusBadRequest, "Memory ID is required")
		return
	}

	node, _, err := h.memoryService.GetNodeDetails(r.Context(), userID, memoryID)
	if err != nil {
		h.logger.Error("Failed to get memory", zap.Error(err))
		api.Error(w, http.StatusNotFound, "Memory not found")
		return
	}

	response := api.NodeResponse{
		NodeID:    node.ID.String(),
		Content:   node.Content.String(),
		Tags:      node.Tags.ToSlice(),
		Timestamp: node.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Version:   node.Version,
	}

	api.Success(w, http.StatusOK, response)
}

// UpdateMemory handles PUT /api/memories/{id}
func (h *MemoryHandler) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	memoryID := chi.URLParam(r, "id")
	if memoryID == "" {
		api.Error(w, http.StatusBadRequest, "Memory ID is required")
		return
	}

	var req api.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	node, err := h.memoryService.UpdateNode(r.Context(), userID, memoryID, req.Content, req.Tags)
	if err != nil {
		h.logger.Error("Failed to update memory", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to update memory")
		return
	}

	response := api.NodeResponse{
		NodeID:    node.ID.String(),
		Content:   node.Content.String(),
		Tags:      node.Tags.ToSlice(),
		Timestamp: node.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Version:   node.Version,
	}

	api.Success(w, http.StatusOK, response)
}

// DeleteMemory handles DELETE /api/memories/{id}
func (h *MemoryHandler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	memoryID := chi.URLParam(r, "id")
	if memoryID == "" {
		api.Error(w, http.StatusBadRequest, "Memory ID is required")
		return
	}

	err := h.memoryService.DeleteNode(r.Context(), userID, memoryID)
	if err != nil {
		h.logger.Error("Failed to delete memory", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to delete memory")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}