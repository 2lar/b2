// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/pkg/api"

	"github.com/go-chi/chi/v5"
)

// CategoryHandler handles category-related HTTP requests with CQRS services.
type CategoryHandler struct {
	// CQRS services for clean separation of concerns
	categoryService      *services.CategoryService      // Write operations (commands)
	categoryQueryService *queries.CategoryQueryService  // Read operations (queries)
}

// NewCategoryHandler creates a new category handler with CQRS services.
func NewCategoryHandler(
	categoryService *services.CategoryService,
	categoryQueryService *queries.CategoryQueryService,
) *CategoryHandler {
	return &CategoryHandler{
		categoryService:      categoryService,
		categoryQueryService: categoryQueryService,
	}
}

// NewCategoryHandlerLegacy creates a new category handler with legacy service (temporary).
func NewCategoryHandlerLegacy(legacyService interface{}) *CategoryHandler {
	// For now, return a handler that will fail gracefully
	// This is a temporary solution until we complete the CQRS migration
	return &CategoryHandler{
		categoryService:      nil,
		categoryQueryService: nil,
	}
}

// ListCategories handles GET /api/categories
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.categoryQueryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		log.Printf("ERROR: ListCategories - Authentication failed, getUserID returned false")
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	log.Printf("DEBUG: ListCategories called for userID: %s", userID)
	
	// Create query for CQRS pattern
	listQuery, err := queries.NewListCategoriesQuery(userID)
	if err != nil {
		log.Printf("ERROR: ListCategories - failed to create query: %v", err)
		handleServiceError(w, err)
		return
	}
	
	// Include node counts in the response
	listQuery.WithNodeCounts()
	
	result, err := h.categoryQueryService.ListCategories(r.Context(), listQuery)
	if err != nil {
		log.Printf("ERROR: ListCategories - categoryQueryService.ListCategories failed: %v", err)
		handleServiceError(w, err)
		return
	}
	
	log.Printf("DEBUG: ListCategories - retrieved %d categories", len(result.Categories))

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
	for _, catView := range result.Categories {
		// CategoryView structure is simpler than domain Category
		var color *string
		if catView.Color != "" {
			color = &catView.Color
		}
		
		categoriesResponse = append(categoriesResponse, CategoryResponse{
			ID:          catView.ID,
			Title:       catView.Title,
			Description: catView.Description,
			Level:       0,         // CategoryView doesn't have Level field
			ParentID:    nil,       // CategoryView doesn't have ParentID field  
			Color:       color,
			Icon:        nil,       // CategoryView doesn't have Icon field
			AIGenerated: false,     // CategoryView doesn't have AIGenerated field
			NoteCount:   catView.NodeCount,
			CreatedAt:   catView.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   catView.UpdatedAt.Format(time.RFC3339),
		})
	}

	api.Success(w, http.StatusOK, map[string][]CategoryResponse{"categories": categoriesResponse})
}

// CreateCategory handles POST /api/categories
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.categoryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
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

	// Create command for CQRS pattern
	cmd, err := commands.NewCreateCategoryCommand(userID, req.Title, req.Description)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	result, err := h.categoryService.CreateCategory(r.Context(), cmd)
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

	// Convert CategoryView from CQRS result to response format
	var color *string
	if result.Category.Color != "" {
		color = &result.Category.Color
	}
	
	api.Success(w, http.StatusCreated, CategoryResponse{
		ID:          result.Category.ID,
		Title:       result.Category.Title,
		Description: result.Category.Description,
		Level:       0,       // CategoryView doesn't have Level field
		ParentID:    nil,     // CategoryView doesn't have ParentID field
		Color:       color,
		Icon:        nil,     // CategoryView doesn't have Icon field
		AIGenerated: false,   // CategoryView doesn't have AIGenerated field
		NoteCount:   result.Category.NodeCount,
		CreatedAt:   result.Category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   result.Category.UpdatedAt.Format(time.RFC3339),
	})
}

// GetCategory handles GET /api/categories/{categoryId}
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.categoryQueryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	categoryID := chi.URLParam(r, "categoryId")

	// Create query for CQRS pattern
	categoryQuery, err := queries.NewGetCategoryQuery(userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	result, err := h.categoryQueryService.GetCategory(r.Context(), categoryQuery)
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

	// Convert CategoryView from CQRS result to response format
	var color *string
	if result.Category.Color != "" {
		color = &result.Category.Color
	}
	
	api.Success(w, http.StatusOK, CategoryResponse{
		ID:          result.Category.ID,
		Title:       result.Category.Title,
		Description: result.Category.Description,
		Level:       0,       // CategoryView doesn't have Level field
		ParentID:    nil,     // CategoryView doesn't have ParentID field
		Color:       color,
		Icon:        nil,     // CategoryView doesn't have Icon field
		AIGenerated: false,   // CategoryView doesn't have AIGenerated field
		NoteCount:   result.Category.NodeCount,
		CreatedAt:   result.Category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   result.Category.UpdatedAt.Format(time.RFC3339),
	})
}

// UpdateCategory handles PUT /api/categories/{categoryId}
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.categoryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
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

	// Create command for CQRS pattern
	cmd, err := commands.NewUpdateCategoryCommand(userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	
	cmd.WithTitle(req.Title).WithDescription(req.Description)

	result, err := h.categoryService.UpdateCategory(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Category updated successfully",
		"categoryId": result.Category.ID,
	})
}

// DeleteCategory handles DELETE /api/categories/{categoryId}
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.categoryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	categoryID := chi.URLParam(r, "categoryId")

	// Create command for CQRS pattern
	cmd, err := commands.NewDeleteCategoryCommand(userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	_, err = h.categoryService.DeleteCategory(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AssignNodeToCategory handles POST /api/categories/{categoryId}/nodes
// TODO: Implement after adding corresponding command handler
func (h *CategoryHandler) AssignNodeToCategory(w http.ResponseWriter, r *http.Request) {
	api.Error(w, http.StatusNotImplemented, "AssignNodeToCategory not yet implemented")
}

// GetNodesInCategory handles GET /api/categories/{categoryId}/nodes
func (h *CategoryHandler) GetNodesInCategory(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	categoryID := chi.URLParam(r, "categoryId")

	// Create query for CQRS pattern
	nodesQuery, err := queries.NewGetNodesInCategoryQuery(userID, categoryID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	result, err := h.categoryQueryService.GetNodesInCategory(r.Context(), nodesQuery)
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
	for _, nodeView := range result.Nodes {
		nodesResponse = append(nodesResponse, NodeResponse{
			NodeID:    nodeView.ID,
			Content:   nodeView.Content,
			Tags:      nodeView.Tags,
			Timestamp: nodeView.CreatedAt.Format(time.RFC3339),
			Version:   nodeView.Version,
		})
	}

	api.Success(w, http.StatusOK, map[string][]NodeResponse{"nodes": nodesResponse})
}

// RemoveNodeFromCategory handles DELETE /api/categories/{categoryId}/nodes/{nodeId}
// TODO: Implement after adding corresponding command handler
func (h *CategoryHandler) RemoveNodeFromCategory(w http.ResponseWriter, r *http.Request) {
	api.Error(w, http.StatusNotImplemented, "RemoveNodeFromCategory not yet implemented")
}

// GetNodeCategories handles GET /api/nodes/{nodeId}/categories
func (h *CategoryHandler) GetNodeCategories(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// Create query for CQRS pattern
	categoriesQuery, err := queries.NewGetCategoriesForNodeQuery(userID, nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Get categories for this node
	result, err := h.categoryQueryService.GetCategoriesForNode(r.Context(), categoriesQuery)
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
	for _, catView := range result.Categories {
		// Convert CategoryView to response format
		var color *string
		if catView.Color != "" {
			color = &catView.Color
		}
		
		categoriesResponse = append(categoriesResponse, CategoryResponse{
			ID:          catView.ID,
			Title:       catView.Title,
			Description: catView.Description,
			Level:       0,       // CategoryView doesn't have Level field
			ParentID:    nil,     // CategoryView doesn't have ParentID field
			Color:       color,
			Icon:        nil,     // CategoryView doesn't have Icon field
			AIGenerated: false,   // CategoryView doesn't have AIGenerated field
			NoteCount:   catView.NodeCount,
			CreatedAt:   catView.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   catView.UpdatedAt.Format(time.RFC3339),
		})
	}

	api.Success(w, http.StatusOK, map[string][]CategoryResponse{"categories": categoriesResponse})
}

// CategorizeNode handles POST /api/nodes/{nodeId}/categories
func (h *CategoryHandler) CategorizeNode(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
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
