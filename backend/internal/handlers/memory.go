// Package handlers provides HTTP handlers with clean dependency injection.
package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
)

// MemoryHandler handles memory-related HTTP requests with injected dependencies.
type MemoryHandler struct {
	memoryService     memory.Service
	eventBridgeClient *eventbridge.Client
	container         interface {
		IsPostColdStartRequest() bool
		GetTimeSinceColdStart() time.Duration
	}
}

// ColdStartContainer interface for cold start detection
type ColdStartContainer interface {
	IsPostColdStartRequest() bool
	GetTimeSinceColdStart() time.Duration
}

// NewMemoryHandler creates a new memory handler with dependency injection.
func NewMemoryHandler(memoryService memory.Service, eventBridgeClient *eventbridge.Client, container ColdStartContainer) *MemoryHandler {
	return &MemoryHandler{
		memoryService:     memoryService,
		eventBridgeClient: eventBridgeClient,
		container:         container,
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

	// Add idempotency key to context
	ctx := r.Context()
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		ctx = memory.WithIdempotencyKey(ctx, idempotencyKey)
	} else {
		// Generate automatic key
		key := generateIdempotencyKey(userID, "CREATE_NODE", req)
		ctx = memory.WithIdempotencyKey(ctx, key)
	}

	// Call simplified service method
	createdNode, edges, err := h.memoryService.CreateNode(ctx, userID, req.Content, tags)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Publish "NodeCreated" event to EventBridge with complete graph update
	eventDetail, err := json.Marshal(map[string]any{
		"type":      "nodeCreated",
		"userId":    createdNode.UserID,
		"nodeId":    createdNode.ID,
		"content":   createdNode.Content,
		"keywords":  createdNode.Keywords,
		"edges":     edges,
		"timestamp": time.Now(),
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	_, err = h.eventBridgeClient.PutEvents(r.Context(), &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:       aws.String("brain2.api"),
				DetailType:   aws.String("GraphUpdate"),
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
		NodeID:    createdNode.ID,
		Content:   createdNode.Content,
		Tags:      createdNode.Tags,
		Timestamp: createdNode.CreatedAt.Format(time.RFC3339),
		Version:   createdNode.Version,
	})
}

// ListNodes handles GET /api/nodes
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit := 20
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	pageReq := repository.PageRequest{
		Limit:     limit,
		NextToken: query.Get("nextToken"),
	}

	response, err := h.memoryService.GetNodes(r.Context(), userID, pageReq)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Convert PageResponse to the format expected by the frontend
	// Transform raw domain objects to properly formatted API response objects
	nodes, ok := response.Items.([]domain.Node)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Invalid data format")
		return
	}

	// Convert each domain.Node to API response format matching CreateNode/GetNode endpoints
	apiNodes := make([]api.Node, len(nodes))
	for i, node := range nodes {
		apiNodes[i] = api.Node{
			NodeId:    node.ID,        // id → nodeId 
			Content:   node.Content,
			Tags:      &node.Tags,
			Timestamp: node.CreatedAt, // created_at → timestamp
			Version:   node.Version,
		}
	}

	nodesResponse := map[string]interface{}{
		"nodes":     apiNodes,
		"total":     response.Total,
		"hasMore":   response.HasMore,
		"nextToken": response.NextToken,
	}

	api.Success(w, http.StatusOK, nodesResponse)
}

// GetNode handles GET /api/nodes/{nodeId}
func (h *MemoryHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Check if this is a post-cold-start request for conditional behavior
	isPostColdStart := h.container.IsPostColdStartRequest()
	if isPostColdStart {
		timeSince := h.container.GetTimeSinceColdStart()
		log.Printf("GetNode: Processing post-cold-start request (%v after cold start) for node %s", timeSince, nodeID)
		
		// Add cold start headers to response
		w.Header().Set("X-Cold-Start", "true")
		w.Header().Set("X-Cold-Start-Age", timeSince.String())
	}

	node, edges, err := h.memoryService.GetNodeDetails(r.Context(), userID, nodeID)
	if err != nil {
		if isPostColdStart {
			log.Printf("GetNode: Error during post-cold-start request for node %s: %v", nodeID, err)
		}
		handleServiceError(w, err)
		return
	}

	edgeIDs := make([]string, len(edges))
	for i, edge := range edges {
		edgeIDs[i] = edge.TargetID
	}

	response := api.NodeDetailsResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Tags:      node.Tags,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
		Edges:     edgeIDs,
	}

	if isPostColdStart {
		log.Printf("GetNode: Successfully completed post-cold-start request for node %s", nodeID)
	}

	api.Success(w, http.StatusOK, response)
}

// UpdateNode handles PUT /api/nodes/{nodeId}
func (h *MemoryHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Verify ownership before proceeding
	_, err := h.checkOwnership(r.Context(), userID, nodeID)
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

	// Add idempotency key to context
	ctx := r.Context()
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		ctx = memory.WithIdempotencyKey(ctx, idempotencyKey)
	} else {
		key := generateIdempotencyKey(userID, "UPDATE_NODE", map[string]interface{}{
			"nodeId": nodeID,
			"content": req.Content,
			"tags": tags,
		})
		ctx = memory.WithIdempotencyKey(ctx, key)
	}

	_, err = h.memoryService.UpdateNode(ctx, userID, nodeID, req.Content, tags)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

// DeleteNode handles DELETE /api/nodes/{nodeId}
func (h *MemoryHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	_, err := h.checkOwnership(r.Context(), userID, nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := h.memoryService.DeleteNode(r.Context(), userID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node deleted successfully"})
}

// BulkDeleteNodes handles POST /api/nodes/bulk-delete
func (h *MemoryHandler) BulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Add idempotency key to context
	ctx := r.Context()
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		ctx = memory.WithIdempotencyKey(ctx, idempotencyKey)
	} else {
		key := generateIdempotencyKey(userID, "BULK_DELETE", req)
		ctx = memory.WithIdempotencyKey(ctx, key)
	}

	deletedCount, failedNodeIds, err := h.memoryService.BulkDeleteNodes(ctx, userID, req.NodeIds)
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

	log.Printf("DEBUG: Calling memoryService.GetGraphData")
	graph, err := h.memoryService.GetGraphData(r.Context(), userID)
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
func (h *MemoryHandler) checkOwnership(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
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



// generateIdempotencyKey creates an idempotency key from request data if not provided
func generateIdempotencyKey(userID, operation string, payload interface{}) string {
	hasher := sha256.New()
	
	// Include user ID and operation in the hash
	hasher.Write([]byte(userID))
	hasher.Write([]byte(operation))
	
	// Include payload in the hash
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err == nil {
			hasher.Write(payloadBytes)
		}
	}
	
	return hex.EncodeToString(hasher.Sum(nil))
}

// getIdempotencyKey extracts idempotency key from header or generates one
func getIdempotencyKey(r *http.Request, userID, operation string, payload interface{}) string {
	// Check for client-provided idempotency key
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		return key
	}
	
	// Generate automatic key based on operation and payload
	return generateIdempotencyKey(userID, operation, payload)
}
