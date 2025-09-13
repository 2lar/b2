package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"backend/application/commands"
	"backend/application/mediator"
	"backend/application/queries"
	"backend/pkg/auth"
	"backend/pkg/errors"
	"backend/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodeHandler handles node-related HTTP requests
type NodeHandler struct {
	mediator     mediator.IMediator
	logger       *zap.Logger
	errorHandler *errors.ErrorHandler
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(
	med mediator.IMediator,
	logger *zap.Logger,
	errorHandler *errors.ErrorHandler,
) *NodeHandler {
	return &NodeHandler{
		mediator:     med,
		logger:       logger,
		errorHandler: errorHandler,
	}
}

// CreateNodeRequest represents the request body for creating a node
type CreateNodeRequest struct {
	Title   string   `json:"title,omitempty" validate:"omitempty,min=1,max=200"` // Optional, auto-generated from content if not provided
	Content string   `json:"content" validate:"required"`
	Format  string   `json:"format,omitempty" validate:"omitempty,oneof=text markdown html code"`
	X       *float64 `json:"x,omitempty"` // Optional, will be auto-generated if not provided
	Y       *float64 `json:"y,omitempty"` // Optional, will be auto-generated if not provided
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
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body: "+err.Error()))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context from auth middleware
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
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
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to create node",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "validation") {
			h.errorHandler.Handle(w, r, errors.NewValidationError(err.Error()))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to create node").WithCause(err))
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
		h.errorHandler.Handle(w, r, errors.NewValidationError("Node ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid node ID format"))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Create query
	query := queries.GetNodeQuery{
		UserID: userCtx.UserID,
		NodeID: nodeID,
	}

	// Execute query
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to get node",
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Node not found"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to retrieve node").WithCause(err))
		}
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// UpdateNode handles PUT /nodes/{nodeID}
func (h *NodeHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Node ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid node ID format"))
		return
	}

	var req UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body: "+err.Error()))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
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
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to update node",
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Node not found"))
		} else if strings.Contains(err.Error(), "validation") {
			h.errorHandler.Handle(w, r, errors.NewValidationError(err.Error()))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to update node").WithCause(err))
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
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body"))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Generate operation ID for async tracking
	operationID := uuid.New().String()

	// Use the bulk delete command handler
	bulkDeleteCmd := commands.BulkDeleteNodesCommand{
		OperationID: operationID,
		UserID:      userCtx.UserID,
		NodeIDs:     req.NodeIDs,
	}

	// Send command (returns void per CQRS)
	err = h.mediator.Send(r.Context(), bulkDeleteCmd)
	if err != nil {
		h.logger.Error("Failed to execute bulk delete",
			zap.String("operationID", operationID),
			zap.String("userID", userCtx.UserID),
			zap.Int("nodeCount", len(req.NodeIDs)),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to delete nodes").WithCause(err))
		return
	}

	// Return 202 Accepted with operation ID for async tracking
	response := map[string]interface{}{
		"operation_id": operationID,
		"status":       "pending",
		"message":      "Bulk delete operation initiated",
		"status_url":   fmt.Sprintf("/api/v1/operations/%s", operationID),
	}

	h.respondJSON(w, http.StatusAccepted, response)
}

// DeleteNode handles DELETE /nodes/{nodeID}
func (h *NodeHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Node ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(nodeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid node ID format"))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Create command
	cmd := commands.DeleteNodeCommand{
		UserID: userCtx.UserID,
		NodeID: nodeID,
	}

	// Execute command
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to delete node",
			zap.String("nodeID", nodeID),
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Node not found"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to delete node").WithCause(err))
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
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
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
	result, err := h.mediator.Query(r.Context(), query)
	if err != nil {
		h.logger.Error("Failed to list nodes",
			zap.String("userID", userCtx.UserID),
			zap.Error(err),
		)
		h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to list nodes").WithCause(err))
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

// ConnectNodes handles POST /nodes/{id}/connect
func (h *NodeHandler) ConnectNodes(w http.ResponseWriter, r *http.Request) {
	sourceNodeID := chi.URLParam(r, "id")
	if sourceNodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Source node ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(sourceNodeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid source node ID format"))
		return
	}

	var req struct {
		TargetNodeID string  `json:"targetNodeId" validate:"required,uuid"`
		EdgeType     string  `json:"edgeType,omitempty" validate:"omitempty,oneof=similarity semantic reference dependency"`
		Weight       float64 `json:"weight,omitempty" validate:"omitempty,min=0,max=1"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body: "+err.Error()))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context
	userCtx, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Default edge type if not provided
	if req.EdgeType == "" {
		req.EdgeType = "reference"
	}

	// Create edge command
	edgeID := uuid.New().String()
	cmd := commands.CreateEdgeCommand{
		EdgeID:   edgeID,
		UserID:   userCtx.UserID,
		GraphID:  "", // Will be determined from nodes
		SourceID: sourceNodeID,
		TargetID: req.TargetNodeID,
		Type:     req.EdgeType,
		Weight:   req.Weight,
	}

	// Execute command
	if err := h.mediator.Send(r.Context(), cmd); err != nil {
		h.logger.Error("Failed to connect nodes",
			zap.String("userID", userCtx.UserID),
			zap.String("sourceNodeID", sourceNodeID),
			zap.String("targetNodeID", req.TargetNodeID),
			zap.Error(err),
		)
		if strings.Contains(err.Error(), "not found") {
			h.errorHandler.Handle(w, r, errors.NewNotFoundError("Node not found"))
		} else if strings.Contains(err.Error(), "already exists") {
			h.errorHandler.Handle(w, r, errors.NewConflictError("Edge already exists"))
		} else {
			h.errorHandler.Handle(w, r, errors.NewInternalError("Failed to connect nodes").WithCause(err))
		}
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Nodes connected successfully",
		"edge": map[string]interface{}{
			"sourceNodeId": sourceNodeID,
			"targetNodeId": req.TargetNodeID,
			"type":         req.EdgeType,
			"weight":       req.Weight,
		},
	})
}

// DisconnectNodes handles POST /nodes/{id}/disconnect
// Note: This is a simplified implementation that requires the edge ID to be computed
// In production, you might want to create a specific DisconnectNodesCommand
func (h *NodeHandler) DisconnectNodes(w http.ResponseWriter, r *http.Request) {
	sourceNodeID := chi.URLParam(r, "id")
	if sourceNodeID == "" {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Source node ID is required"))
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(sourceNodeID); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid source node ID format"))
		return
	}

	var req struct {
		TargetNodeID string `json:"targetNodeId" validate:"required,uuid"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Invalid request body: "+err.Error()))
		return
	}

	// Validate request
	if err := utils.ValidateStruct(req); err != nil {
		h.errorHandler.Handle(w, r, errors.NewValidationError("Validation error: "+err.Error()))
		return
	}

	// Get user context
	_, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		h.errorHandler.Handle(w, r, errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// TODO: Query for the edge between these nodes to get the edge ID
	// For now, return not implemented
	h.errorHandler.HandleStatus(w, r, http.StatusNotImplemented, "Disconnect nodes endpoint not fully implemented yet")
}

