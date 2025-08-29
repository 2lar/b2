// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"net/http"
	"time"

	"brain2-backend/internal/application/commands"
	appdto "brain2-backend/internal/application/dto"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/interfaces/http/v1/dto"
	"brain2-backend/internal/interfaces/http/v1/middleware"
	"brain2-backend/internal/interfaces/http/v1/validation"
	"brain2-backend/pkg/api"

	"github.com/go-chi/chi/v5"
)

// CategoryHandler handles category-related HTTP requests with clean separation of concerns.
// This handler follows the Single Responsibility Principle by delegating
// specific responsibilities to focused components.
type CategoryHandler struct {
	// Core CQRS services
	categoryService      *services.CategoryService      // Write operations (commands)
	categoryQueryService *queries.CategoryQueryService  // Read operations (queries)
	
	// Focused components following SRP
	validator         *validation.CategoryValidator    // Input validation
	converter         *dto.CategoryConverter          // Data transformation
	middleware        *middleware.HandlerMiddleware   // Cross-cutting concerns
	serviceChecker    *middleware.ServiceAvailabilityCheck // Service availability
	userExtractor     *middleware.UserIDExtractor     // Authentication
	errorHandler      *middleware.ErrorHandler        // Error handling
	logger            *middleware.HandlerLoggingHelper    // Request logging
}

// NewCategoryHandler creates a new category handler with focused dependencies.
func NewCategoryHandler(
	categoryService *services.CategoryService,
	categoryQueryService *queries.CategoryQueryService,
) *CategoryHandler {
	return &CategoryHandler{
		categoryService:      categoryService,
		categoryQueryService: categoryQueryService,
		validator:           validation.NewCategoryValidator(),
		converter:           dto.NewCategoryConverter(),
		middleware:          middleware.NewHandlerMiddleware(),
		serviceChecker:      middleware.NewServiceAvailabilityCheck("CategoryQuery", func() bool {
			return categoryQueryService != nil
		}),
		userExtractor:       middleware.NewUserIDExtractor(),
		errorHandler:        middleware.NewErrorHandler(),
		logger:              middleware.NewHandlerLoggingHelper(),
	}
}

// ListCategories handles GET /api/categories with clean separation of concerns.
// @Summary List all categories for the authenticated user
// @Description Retrieves all categories belonging to the authenticated user with optional filtering by level
// @Tags Category Management
// @Produce json
// @Security Bearer
// @Param level query int false "Filter by category level" example(1)
// @Param limit query int false "Maximum number of categories to return" default(50) example(20)
// @Param offset query int false "Number of categories to skip" default(0) example(0)
// @Success 200 {array} api.CategoryResponse "Successfully retrieved categories"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /categories [get]
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	// Apply middleware chain for cross-cutting concerns
	handler := h.serviceChecker.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("ListCategories", h.listCategoriesCore),
		),
	)
	handler(w, r)
}

// listCategoriesCore contains the core business logic for listing categories.
func (h *CategoryHandler) listCategoriesCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	// Create query using CQRS pattern
	listQuery, err := queries.NewListCategoriesQuery(userID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Include node counts in the response
	listQuery.WithNodeCounts()
	
	// Execute query
	result, err := h.categoryQueryService.ListCategories(r.Context(), listQuery)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Convert to response format using focused converter
	convertedViews := convertAppCategoryViewsToInterfaceViews(result.Categories)
	response := h.converter.FromCategoryViews(convertedViews)
	
	api.Success(w, http.StatusOK, response)
}

// CreateCategory handles POST /api/categories with focused validation and conversion.
// @Summary Create a new category
// @Description Creates a new category for organizing memory nodes with optional parent hierarchy
// @Tags Category Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body api.CreateCategoryRequest true "Category creation request"
// @Success 201 {object} api.CategoryResponse "Successfully created category"
// @Failure 400 {object} api.ErrorResponse "Invalid request body or validation failed"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /categories [post]
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	// Apply middleware chain
	serviceCheck := middleware.NewServiceAvailabilityCheck("CategoryService", func() bool {
		return h.categoryService != nil
	})
	
	handler := serviceCheck.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("CreateCategory", h.createCategoryCore),
		),
	)
	handler(w, r)
}

// createCategoryCore contains the core business logic for creating categories.
func (h *CategoryHandler) createCategoryCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	// Validate request using focused validator
	req, validationResult := h.validator.ValidateCreateCategoryRequest(r)
	if !validationResult.IsValid {
		message := h.validator.FormatValidationErrors(validationResult.Errors)
		api.Error(w, http.StatusBadRequest, message)
		return
	}
	
	// Create command using CQRS pattern
	cmd, err := commands.NewCreateCategoryCommand(userID, req.Title, req.Description)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Execute command
	result, err := h.categoryService.CreateCategory(r.Context(), cmd)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Convert to response format using focused converter
	// Create a CategoryView from the CategoryDTO
	categoryView := &appdto.CategoryView{
		ID:          result.ID,
		Title:       result.Name,
		Description: result.Description,
		UserID:      result.UserID,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
		NodeCount:   result.NodeCount,
	}
	convertedView := convertAppCategoryViewToInterfaceView(categoryView)
	response := h.converter.FromCategoryView(convertedView)
	
	api.Success(w, http.StatusCreated, response)
}

// GetCategory handles GET /api/categories/{categoryId} with clean separation.
// @Summary Get a specific category by ID
// @Description Retrieves a specific category by its ID including node count and hierarchy information
// @Tags Category Management
// @Produce json
// @Security Bearer
// @Param categoryId path string true "Category ID"
// @Success 200 {object} api.CategoryResponse "Successfully retrieved category"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Category not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /categories/{categoryId} [get]
func (h *CategoryHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	handler := h.serviceChecker.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("GetCategory", h.getCategoryCore),
		),
	)
	handler(w, r)
}

// getCategoryCore contains the core business logic for getting a category.
func (h *CategoryHandler) getCategoryCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	categoryID := chi.URLParam(r, "categoryId")
	
	// Create query using CQRS pattern
	categoryQuery, err := queries.NewGetCategoryQuery(userID, categoryID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Execute query
	result, err := h.categoryQueryService.GetCategory(r.Context(), categoryQuery)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Convert to response format using focused converter
	// GetCategoryResult has a Category field
	convertedView := convertAppCategoryViewToInterfaceView(result.Category)
	response := h.converter.FromCategoryView(convertedView)
	
	api.Success(w, http.StatusOK, response)
}

// UpdateCategory handles PUT /api/categories/{categoryId} with focused validation.
// @Summary Update an existing category
// @Description Updates an existing category's title, description, or color
// @Tags Category Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param categoryId path string true "Category ID"
// @Param request body api.UpdateCategoryRequest true "Category update request"
// @Success 200 {object} api.CategoryResponse "Successfully updated category"
// @Failure 400 {object} api.ErrorResponse "Invalid request body or validation failed"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Category not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /categories/{categoryId} [put]
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	serviceCheck := middleware.NewServiceAvailabilityCheck("CategoryService", func() bool {
		return h.categoryService != nil
	})
	
	handler := serviceCheck.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("UpdateCategory", h.updateCategoryCore),
		),
	)
	handler(w, r)
}

// updateCategoryCore contains the core business logic for updating categories.
func (h *CategoryHandler) updateCategoryCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	categoryID := chi.URLParam(r, "categoryId")
	
	// Validate request using focused validator
	req, validationResult := h.validator.ValidateUpdateCategoryRequest(r)
	if !validationResult.IsValid {
		message := h.validator.FormatValidationErrors(validationResult.Errors)
		api.Error(w, http.StatusBadRequest, message)
		return
	}
	
	// Create command using CQRS pattern
	cmd, err := commands.NewUpdateCategoryCommand(userID, categoryID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	cmd.WithTitle(req.Title).WithDescription(req.Description)
	
	// Execute command
	result, err := h.categoryService.UpdateCategory(r.Context(), cmd)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Category updated successfully",
		"categoryId": result.ID,
	})
}

// DeleteCategory handles DELETE /api/categories/{categoryId} with clean separation.
// @Summary Delete a category
// @Description Deletes a category and optionally removes it from all associated nodes
// @Tags Category Management
// @Security Bearer
// @Param categoryId path string true "Category ID"
// @Success 204 "Category successfully deleted"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Category not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /categories/{categoryId} [delete]
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	serviceCheck := middleware.NewServiceAvailabilityCheck("CategoryService", func() bool {
		return h.categoryService != nil
	})
	
	handler := serviceCheck.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("DeleteCategory", h.deleteCategoryCore),
		),
	)
	handler(w, r)
}

// deleteCategoryCore contains the core business logic for deleting categories.
func (h *CategoryHandler) deleteCategoryCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	categoryID := chi.URLParam(r, "categoryId")
	
	// Execute delete directly with userID and categoryID
	err := h.categoryService.DeleteCategory(r.Context(), userID, categoryID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// GetNodesInCategory handles GET /api/categories/{categoryId}/nodes with clean separation.
func (h *CategoryHandler) GetNodesInCategory(w http.ResponseWriter, r *http.Request) {
	handler := h.serviceChecker.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("GetNodesInCategory", h.getNodesInCategoryCore),
		),
	)
	handler(w, r)
}

// getNodesInCategoryCore contains the core business logic for getting nodes in a category.
func (h *CategoryHandler) getNodesInCategoryCore(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	categoryID := chi.URLParam(r, "categoryId")
	
	// Create query using CQRS pattern
	nodesQuery, err := queries.NewGetNodesInCategoryQuery(userID, categoryID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Execute query
	result, err := h.categoryQueryService.GetNodesInCategory(r.Context(), nodesQuery)
	if err != nil {
		h.errorHandler.HandleServiceError(w, err)
		return
	}
	
	// Convert to response format (this could be extracted to a node converter)
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

// GetNodeCategories handles GET /api/nodes/{nodeId}/categories with clean separation.
func (h *CategoryHandler) GetNodeCategories(w http.ResponseWriter, r *http.Request) {
	handler := h.serviceChecker.Check(
		h.userExtractor.Extract(
			h.logger.LogHandlerCall("GetNodeCategories", h.getNodeCategoriesCore),
		),
	)
	handler(w, r)
}

// getNodeCategoriesCore contains the core business logic for getting node categories.
func (h *CategoryHandler) getNodeCategoriesCore(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	nodeID := chi.URLParam(r, "nodeId")
	
	// TODO: Implement GetNodeCategories query when ready
	// For now, return empty categories to prevent errors
	api.Success(w, http.StatusOK, map[string]interface{}{
		"categories": []interface{}{},
		"nodeId":     nodeID,
	})
}

// AssignNodeToCategory handles POST /api/categories/{categoryId}/nodes
func (h *CategoryHandler) AssignNodeToCategory(w http.ResponseWriter, r *http.Request) {
	api.Error(w, http.StatusNotImplemented, "AssignNodeToCategory not yet implemented")
}

// RemoveNodeFromCategory handles DELETE /api/categories/{categoryId}/nodes/{nodeId}
func (h *CategoryHandler) RemoveNodeFromCategory(w http.ResponseWriter, r *http.Request) {
	api.Error(w, http.StatusNotImplemented, "RemoveNodeFromCategory not yet implemented")
}

// CategorizeNode handles POST /api/nodes/{nodeId}/categories
func (h *CategoryHandler) CategorizeNode(w http.ResponseWriter, r *http.Request) {
	handler := h.userExtractor.Extract(
		h.logger.LogHandlerCall("CategorizeNode", h.categorizeNodeCore),
	)
	handler(w, r)
}

// categorizeNodeCore contains the core business logic for auto-categorizing nodes.
func (h *CategoryHandler) categorizeNodeCore(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
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
	
	api.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Auto-categorization not yet implemented",
		"categories": []interface{}{},
		"nodeId":     nodeID,
	})
}

// Helper function to convert between different CategoryView types
func convertAppCategoryViewsToInterfaceViews(appViews []*appdto.CategoryView) []dto.CategoryView {
	if appViews == nil {
		return nil
	}
	
	result := make([]dto.CategoryView, len(appViews))
	for i, appView := range appViews {
		if appView != nil {
			result[i] = dto.CategoryView{
				ID:          appView.ID,
				Title:       appView.Title,
				Description: appView.Description,
				Level:       0, // appdto.CategoryView doesn't have Level
				NoteCount:   appView.NodeCount,
			}
		}
	}
	return result
}

func convertAppCategoryViewToInterfaceView(appView *appdto.CategoryView) dto.CategoryView {
	if appView == nil {
		return dto.CategoryView{}
	}
	
	return dto.CategoryView{
		ID:          appView.ID,
		Title:       appView.Title,
		Description: appView.Description,
		Level:       0, // appdto.CategoryView doesn't have Level
		NoteCount:   appView.NodeCount,
	}
}