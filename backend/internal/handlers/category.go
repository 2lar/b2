// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"brain2-backend/internal/service/category"
	"brain2-backend/pkg/api"

	"github.com/go-chi/chi/v5"
)

// CategoryHandler handles category-related HTTP requests with injected dependencies.
type CategoryHandler struct {
	categoryService category.Service
}

// NewCategoryHandler creates a new category handler with dependency injection.
func NewCategoryHandler(categoryService category.Service) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

// ListCategories handles GET /api/categories
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categories, err := h.categoryService.ListCategories(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	var categoriesResponse []CategoryResponse
	for _, cat := range categories {
		categoriesResponse = append(categoriesResponse, CategoryResponse{
			ID:          cat.ID,
			Title:       cat.Title,
			Description: cat.Description,
			Level:       cat.Level,
			ParentID:    cat.ParentID,
			Color:       cat.Color,
			Icon:        cat.Icon,
			AIGenerated: cat.AIGenerated,
			NoteCount:   cat.NoteCount,
			CreatedAt:   cat.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   cat.UpdatedAt.Format(time.RFC3339),
		})
	}

	api.Success(w, http.StatusOK, map[string][]CategoryResponse{"categories": categoriesResponse})
}

// CreateCategory handles POST /api/categories
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)

	type CreateCategoryRequest struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		api.Error(w, http.StatusBadRequest, "Title cannot be empty")
		return
	}

	cat, err := h.categoryService.CreateCategory(r.Context(), userID, req.Title, req.Description)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	api.Success(w, http.StatusCreated, CategoryResponse{
		ID:          cat.ID,
		Title:       cat.Title,
		Description: cat.Description,
		Level:       cat.Level,
		ParentID:    cat.ParentID,
		Color:       cat.Color,
		Icon:        cat.Icon,
		AIGenerated: cat.AIGenerated,
		NoteCount:   cat.NoteCount,
		CreatedAt:   cat.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   cat.UpdatedAt.Format(time.RFC3339),
	})
}

// GetCategory handles GET /api/categories/{categoryId}
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	category, err := h.categoryService.GetCategory(r.Context(), userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type CategoryResponse struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Level       int     `json:"level"`
		ParentID    *string `json:"parentId"`
		Color       *string `json:"color"`
		Icon        *string `json:"icon"`
		AIGenerated bool    `json:"aiGenerated"`
		NoteCount   int     `json:"noteCount"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	api.Success(w, http.StatusOK, CategoryResponse{
		ID:          category.ID,
		Title:       category.Title,
		Description: category.Description,
		Level:       category.Level,
		ParentID:    category.ParentID,
		Color:       category.Color,
		Icon:        category.Icon,
		AIGenerated: category.AIGenerated,
		NoteCount:   category.NoteCount,
		CreatedAt:   category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   category.UpdatedAt.Format(time.RFC3339),
	})
}

// UpdateCategory handles PUT /api/categories/{categoryId}
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	type UpdateCategoryRequest struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Title == "" {
		api.Error(w, http.StatusBadRequest, "Title cannot be empty")
		return
	}

	category, err := h.categoryService.UpdateCategory(r.Context(), userID, categoryID, req.Title, req.Description)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Category updated successfully",
		"categoryId": category.ID,
	})
}

// DeleteCategory handles DELETE /api/categories/{categoryId}
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	if err := h.categoryService.DeleteCategory(r.Context(), userID, categoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddMemoryToCategory handles POST /api/categories/{categoryId}/memories
func (h *CategoryHandler) AddMemoryToCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	type AddMemoryRequest struct {
		MemoryID string `json:"memoryId"`
	}

	var req AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MemoryID == "" {
		api.Error(w, http.StatusBadRequest, "MemoryID cannot be empty")
		return
	}

	if err := h.categoryService.AddMemoryToCategory(r.Context(), userID, categoryID, req.MemoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Memory added to category successfully"})
}

// GetMemoriesInCategory handles GET /api/categories/{categoryId}/memories
func (h *CategoryHandler) GetMemoriesInCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	memories, err := h.categoryService.GetMemoriesInCategory(r.Context(), userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type MemoryResponse struct {
		NodeID    string   `json:"nodeId"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Timestamp string   `json:"timestamp"`
		Version   int      `json:"version"`
	}

	var memoriesResponse []MemoryResponse
	for _, memory := range memories {
		memoriesResponse = append(memoriesResponse, MemoryResponse{
			NodeID:    memory.ID,
			Content:   memory.Content,
			Tags:      memory.Tags,
			Timestamp: memory.CreatedAt.Format(time.RFC3339),
			Version:   memory.Version,
		})
	}

	api.Success(w, http.StatusOK, map[string][]MemoryResponse{"memories": memoriesResponse})
}

// RemoveMemoryFromCategory handles DELETE /api/categories/{categoryId}/memories/{memoryId}
func (h *CategoryHandler) RemoveMemoryFromCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")
	memoryID := chi.URLParam(r, "memoryId")

	if err := h.categoryService.RemoveMemoryFromCategory(r.Context(), userID, categoryID, memoryID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}