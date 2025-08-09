// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/api"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MemoryHandler handles memory-related HTTP requests with injected dependencies.
type MemoryHandler struct {
	memoryService     memory.Service
	eventBridgeClient *eventbridge.Client
}

// NewMemoryHandler creates a new memory handler with dependency injection.
func NewMemoryHandler(memoryService memory.Service, eventBridgeClient *eventbridge.Client) *MemoryHandler {
	return &MemoryHandler{
		memoryService:     memoryService,
		eventBridgeClient: eventBridgeClient,
	}
}

// CreateNode handles POST /api/nodes
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
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

	tags := []string{}
	if req.Tags != nil {
		tags = *req.Tags
	}
	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   req.Content,
		Keywords:  memory.ExtractKeywords(req.Content),
		Tags:      tags,
		CreatedAt: time.Now(),
		Version:   0,
	}

	if err := h.memoryService.CreateNodeAndKeywords(r.Context(), node); err != nil {
		handleServiceError(w, err)
		return
	}

	// Publish "NodeCreated" event to EventBridge
	eventDetail, err := json.Marshal(map[string]interface{}{
		"userId":   node.UserID,
		"nodeId":   node.ID,
		"content":  node.Content,
		"keywords": node.Keywords,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	_, err = h.eventBridgeClient.PutEvents(r.Context(), &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:       aws.String("brain2.api"),
				DetailType:   aws.String("NodeCreated"),
				Detail:       aws.String(string(eventDetail)),
				EventBusName: aws.String("B2EventBus"),
			},
		},
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusCreated, api.NodeResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Tags:      node.Tags,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
	})
}

// ListNodes handles GET /api/nodes
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Use paginated version for better performance
	pagination := repository.Pagination{
		Limit:  1000, // Reasonable limit for node listing
		Offset: 0,
	}

	graph, _, err := h.memoryService.GetGraphDataPaginated(r.Context(), userID, pagination)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	var nodesResponse []api.NodeResponse
	for _, node := range graph.Nodes {
		nodesResponse = append(nodesResponse, api.NodeResponse{
			NodeID:    node.ID,
			Content:   node.Content,
			Tags:      node.Tags,
			Timestamp: node.CreatedAt.Format(time.RFC3339),
			Version:   node.Version,
		})
	}
	api.Success(w, http.StatusOK, map[string][]api.NodeResponse{"nodes": nodesResponse})
}

// GetNode handles GET /api/nodes/{nodeId}
func (h *MemoryHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	node, edges, err := h.memoryService.GetNodeDetails(r.Context(), userID, nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	edgeIDs := make([]string, len(edges))
	for i, edge := range edges {
		edgeIDs[i] = edge.TargetID
	}

	api.Success(w, http.StatusOK, api.NodeDetailsResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Tags:      node.Tags,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
		Edges:     edgeIDs,
	})
}

// UpdateNode handles PUT /api/nodes/{nodeId}
func (h *MemoryHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	// Verify ownership before proceeding
	_, err := h.checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var req api.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Add server-side validation
	if len(req.Content) == 0 || len(req.Content) > 5000 {
		api.Error(w, http.StatusBadRequest, "Content must be between 1 and 5000 characters.")
		return
	}

	tags := []string{}
	if req.Tags != nil {
		tags = *req.Tags
	}
	_, err = h.memoryService.UpdateNode(r.Context(), userID, nodeID, req.Content, tags)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

// DeleteNode handles DELETE /api/nodes/{nodeId}
func (h *MemoryHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	_, err := h.checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.memoryService.DeleteNode(r.Context(), userID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BulkDeleteNodes handles POST /api/nodes/bulk-delete
func (h *MemoryHandler) BulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)

	var req api.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.NodeIds) == 0 {
		api.Error(w, http.StatusBadRequest, "NodeIds cannot be empty")
		return
	}

	if len(req.NodeIds) > 100 {
		api.Error(w, http.StatusBadRequest, "Cannot delete more than 100 nodes at once")
		return
	}

	deletedCount, failedNodeIds, err := h.memoryService.BulkDeleteNodes(r.Context(), userID, req.NodeIds)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	message := fmt.Sprintf("Successfully deleted %d nodes", deletedCount)
	if len(failedNodeIds) > 0 {
		message += fmt.Sprintf(", failed to delete %d nodes", len(failedNodeIds))
	}

	api.Success(w, http.StatusOK, api.BulkDeleteResponse{
		DeletedCount:  &deletedCount,
		FailedNodeIds: &failedNodeIds,
		Message:       &message,
	})
}

// GetGraphData handles GET /api/graph-data
func (h *MemoryHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		log.Printf("ERROR: GetGraphData - Authentication required, getUserID returned false")
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	log.Printf("DEBUG: GetGraphData called for userID: %s", userID)

	// Use paginated version for better performance, but maintain backward compatibility
	// For existing API, we'll load a large page by default to maintain compatibility
	pagination := repository.Pagination{
		Limit:  1000, // Maximum allowed limit for graph data
		Offset: 0,
	}

	log.Printf("DEBUG: Calling memoryService.GetGraphDataPaginated with pagination limit: %d", pagination.Limit)
	graph, _, err := h.memoryService.GetGraphDataPaginated(r.Context(), userID, pagination)
	if err != nil {
		log.Printf("ERROR: GetGraphDataPaginated failed: %v", err)
		handleServiceError(w, err)
		return
	}

	log.Printf("DEBUG: GetGraphDataPaginated succeeded, graph has %d nodes and %d edges", len(graph.Nodes), len(graph.Edges))

	var elements []api.GraphDataResponse_Elements_Item

	// Handle case where graph is nil (should not happen, but defensive programming)
	if graph == nil {
		log.Printf("WARN: GetGraphDataPaginated returned nil graph, returning empty response")
		api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
		return
	}

	for _, node := range graph.Nodes {
		label := node.Content
		if len(label) > 50 {
			label = label[:47] + "..."
		}

		graphNode := api.GraphNode{
			Data: &api.NodeData{
				Id:    node.ID,
				Label: label,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphNode(graphNode); err != nil {
			log.Printf("WARN: Failed to convert node to GraphDataResponse element: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	for _, edge := range graph.Edges {
		edgeID := fmt.Sprintf("%s-%s", edge.SourceID, edge.TargetID)
		graphEdge := api.GraphEdge{
			Data: &api.EdgeData{
				Id:     edgeID,
				Source: edge.SourceID,
				Target: edge.TargetID,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphEdge(graphEdge); err != nil {
			log.Printf("WARN: Failed to convert edge to GraphDataResponse element: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	log.Printf("DEBUG: GetGraphData completed successfully - returning %d elements", len(elements))
	api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
}

// Helper method for ownership check
func (h *MemoryHandler) checkOwnership(ctx context.Context, nodeID string) (*domain.Node, error) {
	userID := ctx.Value(userIDKey).(string)
	node, _, err := h.memoryService.GetNodeDetails(ctx, userID, nodeID)
	if err != nil {
		if appErrors.IsNotFound(err) {
			return nil, err
		}
		return nil, appErrors.NewInternal("failed to verify node ownership", err)
	}

	if node.UserID != userID {
		return nil, appErrors.NewNotFound("node not found") // Obscure the reason for security
	}

	return node, nil
}

// GetNodesPage handles GET /api/nodes/page with pagination support
func (h *MemoryHandler) GetNodesPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse pagination parameters
	pagination := repository.Pagination{
		Limit:  50, // Default limit
		Offset: 0,  // Default offset
	}

	// Parse query parameters for pagination
	query := r.URL.Query()
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 1000 {
			pagination.Limit = limit
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			pagination.Offset = offset
		}
	}
	if cursor := query.Get("cursor"); cursor != "" {
		pagination.Cursor = cursor
	}

	page, err := h.memoryService.GetNodesPage(r.Context(), userID, pagination)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Convert nodes to API response format
	nodes := make([]api.EnhancedNode, len(page.Items))
	for i, node := range page.Items {
		createdAt := node.CreatedAt.Format(time.RFC3339)
		nodes[i] = api.EnhancedNode{
			Node: api.Node{
				Content:   node.Content,
				NodeId:    node.ID,
				Tags:      &node.Tags,
				Timestamp: node.CreatedAt,
				Version:   node.Version,
			},
			Keywords:  &node.Keywords,
			CreatedAt: &createdAt,
		}
	}

	response := api.NodePageResponse{
		Items:      &nodes,
		HasMore:    &page.HasMore,
		NextCursor: &page.NextCursor,
		PageInfo: &api.PageInfo{
			CurrentPage: &page.PageInfo.CurrentPage,
			PageSize:    &page.PageInfo.PageSize,
			ItemsInPage: &page.PageInfo.ItemsInPage,
		},
	}

	api.Success(w, http.StatusOK, response)
}

// GetNodeNeighborhood handles GET /api/nodes/{nodeId}/neighborhood with depth parameter
func (h *MemoryHandler) GetNodeNeighborhood(w http.ResponseWriter, r *http.Request) {
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

	// Parse depth parameter (default: 2, max: 3)
	depth := 2
	if depthStr := r.URL.Query().Get("depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d >= 1 && d <= 3 {
			depth = d
		}
	}

	graph, err := h.memoryService.GetNodeNeighborhood(r.Context(), userID, nodeID, depth)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Convert to graph data response format (reuse existing logic)
	var elements []api.GraphDataResponse_Elements_Item

	for _, node := range graph.Nodes {
		label := node.Content
		if len(label) > 50 {
			label = label[:47] + "..."
		}

		graphNode := api.GraphNode{
			Data: &api.NodeData{
				Id:    node.ID,
				Label: label,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphNode(graphNode); err != nil {
			log.Printf("WARN: Failed to convert node to GraphDataResponse element: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	for _, edge := range graph.Edges {
		edgeID := fmt.Sprintf("%s-%s", edge.SourceID, edge.TargetID)
		graphEdge := api.GraphEdge{
			Data: &api.EdgeData{
				Id:     edgeID,
				Source: edge.SourceID,
				Target: edge.TargetID,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphEdge(graphEdge); err != nil {
			log.Printf("WARN: Failed to convert edge to GraphDataResponse element: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	response := api.NodeNeighborhoodResponse{
		Elements: &elements,
		Depth:    &depth,
		CenterNode: &api.Node{
			NodeId:    nodeID,
			Content:   "", // Will be filled from the graph data
			Timestamp: time.Now(),
			Version:   0,
		},
	}

	api.Success(w, http.StatusOK, response)
}
