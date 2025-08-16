// Package handlers provides clean HTTP handlers following best practices.
package handlers

import (
	"encoding/json"
	"net/http"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/services"
	"brain2-backend/pkg/api"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// CategoryHandler handles HTTP requests for category operations following clean architecture.
type CategoryHandler struct {
	categoryService *services.CategoryService
	logger          *zap.Logger
}

// NewCategoryHandler creates a new category handler with dependency injection.
func NewCategoryHandler(categoryService *services.CategoryService, logger *zap.Logger) *CategoryHandler {
	if categoryService == nil {
		panic("categoryService is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &CategoryHandler{
		categoryService: categoryService,
		logger:          logger.Named("CategoryHandler"),
	}
}

// CreateCategory handles POST /api/categories
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		api.Error(w, http.StatusBadRequest, "Title is required")
		return
	}

	// Create command
	cmd, err := commands.NewCreateCategoryCommand(userID, req.Title, req.Description)
	if err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid category data: "+err.Error())
		return
	}
	
	if req.Color != "" {
		cmd.WithColor(req.Color)
	}

	// Execute command
	result, err := h.categoryService.CreateCategory(r.Context(), cmd)
	if err != nil {
		h.logger.Error("Failed to create category", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	api.Success(w, http.StatusCreated, result)
}

// UpdateCategory handles PUT /api/categories/{id}
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	categoryID := chi.URLParam(r, "id")
	if categoryID == "" {
		api.Error(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Create update command
	cmd, err := commands.NewUpdateCategoryCommand(userID, categoryID)
	if err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid update data: "+err.Error())
		return
	}
	
	if req.Title != "" {
		cmd.WithTitle(req.Title)
	}
	if req.Description != "" {
		cmd.WithDescription(req.Description)
	}
	if req.Color != "" {
		cmd.WithColor(req.Color)
	}

	// Execute command
	result, err := h.categoryService.UpdateCategory(r.Context(), cmd)
	if err != nil {
		h.logger.Error("Failed to update category", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	api.Success(w, http.StatusOK, result)
}

// DeleteCategory handles DELETE /api/categories/{id}
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)
	if userID == "" {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	categoryID := chi.URLParam(r, "id")
	if categoryID == "" {
		api.Error(w, http.StatusBadRequest, "Category ID is required")
		return
	}

	// Create delete command
	cmd, err := commands.NewDeleteCategoryCommand(userID, categoryID)
	if err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid delete data: "+err.Error())
		return
	}

	// Execute command
	_, err = h.categoryService.DeleteCategory(r.Context(), cmd)
	if err != nil {
		h.logger.Error("Failed to delete category", zap.Error(err))
		api.Error(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}