// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"encoding/json"
	"log"
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
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
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
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

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

// AssignNodeToCategory handles POST /api/categories/{categoryId}/nodes
func (h *CategoryHandler) AssignNodeToCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	type AssignNodeRequest struct {
		NodeID string `json:"nodeId"`
	}

	var req AssignNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NodeID == "" {
		api.Error(w, http.StatusBadRequest, "NodeID cannot be empty")
		return
	}

	if err := h.categoryService.AssignNodeToCategory(r.Context(), userID, categoryID, req.NodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node assigned to category successfully"})
}

// GetNodesInCategory handles GET /api/categories/{categoryId}/nodes
func (h *CategoryHandler) GetNodesInCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")

	nodes, err := h.categoryService.GetNodesInCategory(r.Context(), userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	type NodeResponse struct {
		NodeID    string   `json:"nodeId"`
		Content   string   `json:"content"`
		Tags      []string `json:"tags"`
		Timestamp string   `json:"timestamp"`
		Version   int      `json:"version"`
	}

	var nodesResponse []NodeResponse
	for _, node := range nodes {
		nodesResponse = append(nodesResponse, NodeResponse{
			NodeID:    node.ID,
			Content:   node.Content,
			Tags:      node.Tags,
			Timestamp: node.CreatedAt.Format(time.RFC3339),
			Version:   node.Version,
		})
	}

	api.Success(w, http.StatusOK, map[string][]NodeResponse{"nodes": nodesResponse})
}

// RemoveNodeFromCategory handles DELETE /api/categories/{categoryId}/nodes/{nodeId}
func (h *CategoryHandler) RemoveNodeFromCategory(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	categoryID := chi.URLParam(r, "categoryId")
	nodeID := chi.URLParam(r, "nodeId")

	if err := h.categoryService.RemoveNodeFromCategory(r.Context(), userID, categoryID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetNodeCategories handles GET /api/nodes/{nodeId}/categories
func (h *CategoryHandler) GetNodeCategories(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// Get categories for this node
	categories, err := h.categoryService.GetCategoriesForNode(r.Context(), userID, nodeID)
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

// CategorizeNode handles POST /api/nodes/{nodeId}/categories
func (h *CategoryHandler) CategorizeNode(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// TODO: Implement AI-powered categorization when LLM service infrastructure is ready
	// For now, return success with empty categories to prevent frontend errors

	// Future implementation should:
	// 1. Get the node content using the memory service
	// 2. Use enhanced category service with AI categorization
	// 3. Automatically assign relevant categories based on content analysis

	// Log for development visibility
	log.Printf("Auto-categorization requested for nodeID %s by user %s - not yet implemented", nodeID, userID)

	// Return empty categories array for now
	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Auto-categorization not yet implemented",
		"categories": []interface{}{},
		"nodeId":     nodeID,
	})
}
