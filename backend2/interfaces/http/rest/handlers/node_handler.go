package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"backend2/application/commands"
	"backend2/application/commands/bus"
	"backend2/application/queries"
	querybus "backend2/application/queries/bus"
	"backend2/pkg/auth"
	"backend2/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodeHandler handles node-related HTTP requests
type NodeHandler struct {
	commandBus *bus.CommandBus
	queryBus   *querybus.QueryBus
	logger     *zap.Logger
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(
	commandBus *bus.CommandBus,
	queryBus *querybus.QueryBus,
	logger *zap.Logger,
) *NodeHandler {
	return &NodeHandler{
		commandBus: commandBus,
		queryBus:   queryBus,
		logger:     logger,
	}
}

// CreateNodeRequest represents the request body for creating a node
type CreateNodeRequest struct {
	Title   string   `json:"title,omitempty" validate:"omitempty,min=1,max=200"`  // Optional, auto-generated from content if not provided
	Content string   `json:"content" validate:"required"`
	Format  string   `json:"format,omitempty" validate:"omitempty,oneof=text markdown html code"`
	X       *float64 `json:"x,omitempty"`  // Optional, will be auto-generated if not provided
	Y       *float64 `json:"y,omitempty"`  // Optional, will be auto-generated if not provided
	Z       *float64 `json:"z,omitempty"`
	Tags    []string `json:"tags,omitempty" validate:"omitempty,max=10,dive,max=50"`
}

// UpdateNodeRequest represents the request body for updating a node
type UpdateNodeRequest struct {
	Title   *string   `json:"title,omitempty" validate:"omitempty,min=1,max=200"`
	Content *string   `json:"content,omitempty"`
	Format  *string   `json:"format,omitempty" validate:"omitempty,oneof=text markdown html code"`
	X       *float64  `json:"x,omitempty"`
	Y       *float64  `json:"y,omitempty"`
	Z       *float64  `json:"z,omitempty"`
	Tags    *[]string `json:"tags,omitempty" validate:"omitempty,max=10,dive,max=50"`
}

// CreateNodeResponse represents the response for creating a node
type CreateNodeResponse struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

// CreateNode handles POST /nodes
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}
	
	// Get user context from auth middleware
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// Set default format if not provided
	if req.Format == "" {
		req.Format = "text"
	}
	
	// Generate title from content if not provided
	if req.Title == "" {
		// Take first 50 characters of content or less
		titleLen := len(req.Content)
		if titleLen > 50 {
			titleLen = 50
		}
		req.Title = strings.TrimSpace(req.Content[:titleLen])
		if req.Title == "" {
			req.Title = "Untitled"
		}
		// Add ellipsis if truncated
		if len(req.Content) > 50 {
			req.Title = req.Title + "..."
		}
	}
	
	// Generate node ID
	nodeID := uuid.New().String()
	
	// Generate random positions if not provided
	var x, y, z float64
	if req.X != nil {
		x = *req.X
	} else {
		// Generate random X position between -500 and 500
		x = (rand.Float64() * 1000) - 500
	}
	if req.Y != nil {
		y = *req.Y
	} else {
		// Generate random Y position between -500 and 500
		y = (rand.Float64() * 1000) - 500
	}
	if req.Z != nil {
		z = *req.Z
	} else {
		z = 0 // Default Z to 0
	}
	
	// Create command
	cmd := commands.CreateNodeCommand{
		NodeID:  nodeID,
		UserID:  userCtx.UserID,
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
		X:       x,
		Y:       y,
		Z:       z,
		Tags:    req.Tags,
	}
	
	// Execute command
	if err := h.commandBus.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to create node", 
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "validation") {
			h.respondError(w, http.StatusBadRequest, err.Error())
		} else {
			h.respondError(w, http.StatusInternalServerError, "Failed to create node")
		}
		return
	}
	
	response := CreateNodeResponse{
		ID:        nodeID,
		Message:   "Node created successfully",
		CreatedAt: utils.NowRFC3339(),
	}
	
	h.respondJSON(w, http.StatusCreated, response)
}

// GetNode handles GET /nodes/{nodeID}
func (h *NodeHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.respondError(w, http.StatusBadRequest, "Node ID is required")
		return
	}
	
	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid node ID format")
		return
	}
	
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// Create query
	query := queries.GetNodeQuery{
		UserID: userCtx.UserID,
		NodeID: nodeID,
	}
	
	// Execute query
	result, err := h.queryBus.Ask(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get node", 
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Node not found")
		} else {
			h.respondError(w, http.StatusInternalServerError, "Failed to retrieve node")
		}
		return
	}
	
	h.respondJSON(w, http.StatusOK, result)
}

// UpdateNode handles PUT /nodes/{nodeID}
func (h *NodeHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.respondError(w, http.StatusBadRequest, "Node ID is required")
		return
	}
	
	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid node ID format")
		return
	}
	
	var req UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}
	
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// Create command
	cmd := commands.UpdateNodeCommand{
		UserID:  userCtx.UserID,
		NodeID:  nodeID,
		Title:   req.Title,
		Content: req.Content,
		Format:  req.Format,
		X:       req.X,
		Y:       req.Y,
		Z:       req.Z,
		Tags:    req.Tags,
	}
	
	// Execute command
	if err := h.commandBus.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to update node",
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Node not found")
		} else if strings.Contains(err.Error(), "validation") {
			h.respondError(w, http.StatusBadRequest, err.Error())
		} else {
			h.respondError(w, http.StatusInternalServerError, "Failed to update node")
		}
		return
	}
	
	h.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Node updated successfully",
		"id":      nodeID,
	})
}

// BulkDeleteNodes handles POST /nodes/bulk-delete
func (h *NodeHandler) BulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		NodeIDs []string `json:"node_ids" validate:"required,min=1,max=100"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Validation error: "+err.Error())
		return
	}
	
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// For now, delete nodes individually since we don't have a bulk handler registered
	// TODO: Register bulk delete handler in DI container
	deletedCount := 0
	var failedIDs []string
	var errors []string
	
	for _, nodeID := range req.NodeIDs {
		deleteCmd := commands.DeleteNodeCommand{
			UserID: userCtx.UserID,
			NodeID: nodeID,
		}
		
		if err := h.commandBus.Send(r.Context(), deleteCmd); err != nil {
			failedIDs = append(failedIDs, nodeID)
			errors = append(errors, fmt.Sprintf("Failed to delete node %s: %v", nodeID, err))
			h.logger.Error("Failed to delete node in bulk operation",
				zap.String("nodeID", nodeID),
				zap.Error(err),
			)
		} else {
			deletedCount++
		}
	}
	
	// Build response
	response := map[string]interface{}{
		"deleted_count": deletedCount,
		"failed_ids":    failedIDs,
		"errors":        errors,
	}
	
	h.respondJSON(w, http.StatusOK, response)
}

// DeleteNode handles DELETE /nodes/{nodeID}
func (h *NodeHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.respondError(w, http.StatusBadRequest, "Node ID is required")
		return
	}
	
	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid node ID format")
		return
	}
	
	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	
	// Create command
	cmd := commands.DeleteNodeCommand{
		UserID: userCtx.UserID,
		NodeID: nodeID,
	}
	
	// Execute command
	if err := h.commandBus.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to delete node",
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Node not found")
		} else {
			h.respondError(w, http.StatusInternalServerError, "Failed to delete node")
		}
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// ListNodes handles GET /nodes
func (h *NodeHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
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
	query := queries.ListNodesQuery{
		UserID: userCtx.UserID,
		Limit:  limit,
		Offset: offset,
		SortBy: sortBy,
		Order:  order,
	}
	
	// Execute query
	result, err := h.queryBus.Ask(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to list nodes",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "Failed to list nodes")
		return
	}
	
	h.respondJSON(w, http.StatusOK, result)
}

// Helper methods

func (h *NodeHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

func (h *NodeHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]interface{}{
		"error":   true,
		"message": message,
		"code":    status,
	})
}