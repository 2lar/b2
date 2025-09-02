// Package handlers implements HTTP request handlers following Clean Architecture principles.
//
// PURPOSE: Serves as the Interface Adapter layer that transforms HTTP requests into
// application use cases and converts application responses back to HTTP format.
// This is the entry point for all REST API operations in the Brain2 system.
//
// CLEAN ARCHITECTURE ROLE: This layer handles external communication concerns:
//   • Request/Response transformation and validation
//   • HTTP status code mapping from domain errors
//   • Authentication and authorization enforcement
//   • API versioning and backward compatibility
//   • Request tracing and observability integration
//
// KEY HANDLERS:
//   • MemoryHandler: CRUD operations for memory nodes and graph queries
//   • CategoryHandler: Category management and auto-categorization
//   • HealthHandler: System health checks and readiness probes
//
// DESIGN PRINCIPLES:
//   • Thin Layer: No business logic, only coordination and transformation
//   • Dependency Injection: All dependencies injected via constructor
//   • Error Handling: Consistent error responses across all endpoints
//   • Observability: Request tracing, metrics, and structured logging
//
// This package ensures HTTP concerns remain separate from business logic,
// enabling easy testing and potential protocol changes in the future.
package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	coreCommands "brain2-backend/internal/core/application/commands"
	"brain2-backend/internal/core/application/cqrs"
	coreQueries "brain2-backend/internal/core/application/queries"
	"brain2-backend/pkg/api"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/go-chi/chi/v5"
)

// MemoryHandler handles memory-related HTTP requests with CQRS services.
type MemoryHandler struct {
	// CQRS buses for clean separation of concerns
	commandBus *cqrs.CommandBus // Write operations (commands)
	queryBus   *cqrs.QueryBus   // Read operations (queries)

	// Infrastructure dependencies
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

// NewMemoryHandler creates a new memory handler with CQRS buses.
func NewMemoryHandler(
	commandBus *cqrs.CommandBus,
	queryBus *cqrs.QueryBus,
	eventBridgeClient *eventbridge.Client,
	container ColdStartContainer,
) *MemoryHandler {
	return &MemoryHandler{
		commandBus:        commandBus,
		queryBus:          queryBus,
		eventBridgeClient: eventBridgeClient,
		container:         container,
	}
}

// CreateNode handles POST /api/nodes
// @Summary Create a new memory node
// @Description Creates a new memory node with content, optional title and tags. The system automatically extracts keywords and establishes connections to existing nodes.
// @Tags Memory Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body api.CreateNodeRequest true "Memory node creation request"
// @Success 201 {object} api.Node "Successfully created memory node"
// @Failure 400 {object} api.ErrorResponse "Invalid request body or validation failed"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes [post]
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	// Process create node request
	// Check if CQRS buses are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

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
	// Request decoded successfully

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	// Create command for CQRS pattern
	cmd := &coreCommands.CreateNodeCommand{}
	cmd.UserID = userID
	cmd.Content = req.Content
	cmd.Title = req.Title
	cmd.Tags = tags
	cmd.Timestamp = time.Now()
	// Command created for node creation

	// Add idempotency key if provided
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		cmd.IdempotencyKey = idempotencyKey
	} else {
		// Generate automatic key
		cmd.IdempotencyKey = generateIdempotencyKey(userID, "CREATE_NODE", req)
	}

	// Execute command through CommandBus
	err := h.commandBus.Send(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Generate a node ID for the response (would be returned via event or separate query)
	// In a pure CQRS system, we'd return just an acknowledgment
	// For compatibility, we're generating a temporary ID
	nodeID := generateNodeID(userID, req.Content)

	// Publish "NodeCreated" event to EventBridge
	eventDetail, err := json.Marshal(map[string]any{
		"type":      "nodeCreated",
		"userId":    userID,
		"nodeId":    nodeID,
		"content":   req.Content,
		"title":     req.Title,
		"tags":      tags,
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
		NodeID:    nodeID,
		Content:   req.Content,
		Title:     req.Title,
		Tags:      tags,
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   1,
	})
}

// ListNodes handles GET /api/nodes
// @Summary List all memory nodes for the authenticated user
// @Description Retrieves all memory nodes belonging to the authenticated user, including content, tags, and metadata
// @Tags Memory Management
// @Produce json
// @Security Bearer
// @Success 200 {array} api.Node "Successfully retrieved user's memory nodes"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes [get]
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS buses are available
	if h.queryBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		// Authentication failed
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse query parameters
	urlQuery := r.URL.Query()
	limit := 20
	if l := urlQuery.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Processing list nodes request

	// Create query for CQRS pattern
	listQuery := &coreQueries.GetNodesByUserQuery{}
	listQuery.UserID = userID
	listQuery.Limit = limit
	listQuery.Offset = 0

	// Parse offset from nextToken if provided
	if nextToken := urlQuery.Get("nextToken"); nextToken != "" {
		if offset, err := strconv.Atoi(nextToken); err == nil {
			listQuery.Offset = offset
		}
	}

	// Add sorting if provided
	if sortBy := urlQuery.Get("sortBy"); sortBy != "" {
		listQuery.SortBy = sortBy
	}
	if sortDirection := urlQuery.Get("sortDirection"); sortDirection != "" {
		listQuery.Order = sortDirection
	} else {
		listQuery.Order = "desc"
	}

	// Add search filter if provided
	if searchQuery := urlQuery.Get("search"); searchQuery != "" {
		if listQuery.Filters == nil {
			listQuery.Filters = make(map[string]interface{})
		}
		listQuery.Filters["search"] = searchQuery
	}

	result, err := h.queryBus.Send(r.Context(), listQuery)
	if err != nil {
		// Query service failed
		handleServiceError(w, err)
		return
	}

	// Type assert the result to GetNodesByUserResult
	response, ok := result.(*coreQueries.GetNodesByUserResult)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Invalid response type from query")
		return
	}

	if response == nil {
		// Received nil response from service
		api.Error(w, http.StatusInternalServerError, "Service returned no data")
		return
	}

	// Query executed successfully

	// Convert NodeView DTOs to API response format
	apiNodes := make([]api.Node, len(response.Nodes))
	for i, nodeView := range response.Nodes {
		// Convert Unix timestamps to time.Time for formatting
		createdAt := time.Unix(nodeView.CreatedAt, 0)
		updatedAt := time.Unix(nodeView.UpdatedAt, 0)
		
		apiNodes[i] = api.Node{
			NodeID:    nodeView.ID,
			UserID:    nodeView.UserID,
			Content:   nodeView.Content,
			Title:     nodeView.Title,
			Tags:      nodeView.Tags,
			Metadata:  nil, // NodeView doesn't include metadata field
			Timestamp: createdAt.Format(time.RFC3339),
			CreatedAt: createdAt.Format(time.RFC3339),
			UpdatedAt: updatedAt.Format(time.RFC3339),
		}
	}

	// Calculate next token for pagination
	nextToken := ""
	if response.HasMore {
		nextToken = strconv.Itoa(listQuery.Offset + listQuery.Limit)
	}

	// Response prepared successfully

	nodesResponse := map[string]interface{}{
		"nodes":     apiNodes,
		"total":     response.TotalCount,
		"hasMore":   response.HasMore,
		"nextToken": nextToken,
	}

	// Response prepared successfully
	api.Success(w, http.StatusOK, nodesResponse)
}

// GetNode handles GET /api/nodes/{nodeId}
// @Summary Get a specific memory node by ID
// @Description Retrieves a specific memory node by its ID, including full details and connected edges
// @Tags Memory Management
// @Produce json
// @Security Bearer
// @Param nodeId path string true "Memory node ID"
// @Success 200 {object} api.NodeDetailsResponse "Successfully retrieved memory node details"
// @Failure 400 {object} api.ErrorResponse "Invalid node ID format"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Memory node not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/{nodeId} [get]
func (h *MemoryHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.queryBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

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
		// Processing post-cold-start request

		// Add cold start headers to response
		w.Header().Set("X-Cold-Start", "true")
		w.Header().Set("X-Cold-Start-Age", timeSince.String())
	}

	// Create query for CQRS pattern
	nodeQuery := &coreQueries.GetNodeByIDQuery{}
	nodeQuery.UserID = userID
	nodeQuery.NodeID = nodeID

	result, err := h.queryBus.Send(r.Context(), nodeQuery)
	if err != nil {
		if isPostColdStart {
			// Error during post-cold-start request
		}
		handleServiceError(w, err)
		return
	}

	// Type assert the result to GetNodeByIDResult
	response, ok := result.(*coreQueries.GetNodeByIDResult)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Invalid response type from query")
		return
	}

	if response == nil || response.Node == nil {
		api.Error(w, http.StatusNotFound, "Node not found")
		return
	}

	// For now, we'll return empty connections since GetNodeByIDResult doesn't include them
	// This would need to be enhanced with a separate query for connections
	connectedNodes := []string{}

	apiResponse := api.NodeDetailsResponse{
		NodeID:    response.Node.GetID(),
		Content:   response.Node.GetContent(),
		Title:     response.Node.GetTitle(),
		Tags:      response.Node.GetTags(),
		Timestamp: response.Node.GetCreatedAt().Format(time.RFC3339),
		Version:   int(response.Node.GetVersion()),
		Edges:     connectedNodes, // Edges field contains connected node IDs, not edge IDs
	}

	if isPostColdStart {
		log.Printf("GetNode: Successfully completed post-cold-start request for node %s", nodeID)
	}

	api.Success(w, http.StatusOK, apiResponse)
}

// UpdateNode handles PUT /api/nodes/{nodeId}
// @Summary Update an existing memory node
// @Description Updates an existing memory node's content, title, and tags. The system will re-analyze connections.
// @Tags Memory Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param nodeId path string true "Memory node ID"
// @Param request body api.UpdateNodeRequest true "Memory node update request"
// @Success 200 {object} api.NodeResponse "Successfully updated memory node"
// @Failure 400 {object} api.ErrorResponse "Invalid request body or node ID"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Memory node not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/{nodeId} [put]
func (h *MemoryHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	var req api.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Add server-side validation
	if req.Content != "" && len(req.Content) > 5000 {
		api.Error(w, http.StatusBadRequest, "Content must not exceed 5000 characters.")
		return
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	// Create command for CQRS pattern
	cmd := &coreCommands.UpdateNodeCommand{}
	cmd.UserID = userID
	cmd.NodeID = nodeID
	cmd.Content = req.Content
	cmd.Title = req.Title
	cmd.Tags = tags
	cmd.Timestamp = time.Now()

	// Note: req.Version field doesn't exist in api.UpdateNodeRequest
	// For optimistic locking, this would need to be added to the API request struct

	err := h.commandBus.Send(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

// DeleteNode handles DELETE /api/nodes/{nodeId}
// @Summary Delete a memory node
// @Description Deletes a memory node and all its connections. This operation cannot be undone.
// @Tags Memory Management
// @Security Bearer
// @Param nodeId path string true "Memory node ID"
// @Success 204 "Memory node successfully deleted"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Memory node not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/{nodeId} [delete]
func (h *MemoryHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Create command for CQRS pattern
	cmd := &coreCommands.DeleteNodeCommand{}
	cmd.UserID = userID
	cmd.NodeID = nodeID
	cmd.Timestamp = time.Now()

	err := h.commandBus.Send(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node deleted successfully"})
}

// BulkDeleteNodes handles POST /api/nodes/bulk-delete
// @Summary Delete multiple memory nodes
// @Description Deletes multiple memory nodes and all their connections in a single operation
// @Tags Memory Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body api.BulkDeleteRequest true "Bulk delete request with node IDs"
// @Success 200 {object} api.BulkDeleteResponse "Bulk delete operation completed"
// @Failure 400 {object} api.ErrorResponse "Invalid request body"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/bulk-delete [post]
func (h *MemoryHandler) BulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

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

	if len(req.NodeIDs) == 0 {
		api.Error(w, http.StatusBadRequest, "NodeIds cannot be empty")
		return
	}

	if len(req.NodeIDs) > 100 {
		api.Error(w, http.StatusBadRequest, "Cannot delete more than 100 nodes at once")
		return
	}

	// Delete nodes one by one using individual delete commands
	var deletedCount int
	var failedIDs []string

	for _, nodeID := range req.NodeIDs {
		cmd := &coreCommands.DeleteNodeCommand{}
		cmd.UserID = userID
		cmd.NodeID = nodeID
		cmd.Timestamp = time.Now()

		err := h.commandBus.Send(r.Context(), cmd)
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
			log.Printf("Failed to delete node %s: %v", nodeID, err)
		} else {
			deletedCount++
		}
	}

	api.Success(w, http.StatusOK, api.BulkDeleteResponse{
		DeletedCount: deletedCount,
		FailedIDs:    failedIDs,
	})
}

// GetGraphData handles GET /api/graph-data
// @Summary Get graph visualization data
// @Description Retrieves graph data for visualization including nodes and edges in a format suitable for graph libraries
// @Tags Graph Operations
// @Produce json
// @Security Bearer
// @Success 200 {object} api.GraphDataResponse "Graph data for visualization"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /graph-data [get]
func (h *MemoryHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.queryBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		log.Printf("ERROR: GetGraphData - Authentication required, getUserID returned false")
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	log.Printf("DEBUG: GetGraphData called for userID: %s", userID)

	// Create query for CQRS pattern  
	graphQuery := &coreQueries.GetGraphQuery{}
	graphQuery.UserID = userID

	log.Printf("DEBUG: Calling queryBus.Send with GetGraphDataQuery")
	result, err := h.queryBus.Send(r.Context(), graphQuery)
	if err != nil {
		log.Printf("ERROR: GetGraphData failed: %v", err)
		handleServiceError(w, err)
		return
	}

	// Type assert the result to GetGraphResult
	response, ok := result.(*coreQueries.GetGraphResult)
	if !ok {
		api.Error(w, http.StatusInternalServerError, "Invalid response type from query")
		return
	}

	var elements []api.GraphDataResponse_Elements_Item

	// Handle case where result is nil (should not happen, but defensive programming)
	if response == nil || len(response.Nodes) == 0 && len(response.Edges) == 0 {
		log.Printf("WARN: GetGraphData returned empty graph, returning empty response")
		api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
		return
	}

	log.Printf("DEBUG: GetGraphData succeeded, graph has %d nodes and %d edges", len(response.Nodes), len(response.Edges))

	for _, nodeView := range response.Nodes {
		label := nodeView.Title
		if label == "" {
			label = nodeView.Content
		}
		if len(label) > 50 {
			label = label[:47] + "..."
		}

		graphNode := api.GraphNode{
			Data: &api.NodeData{
				Id:    nodeView.ID,
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

	for _, edgeView := range response.Edges {
		edgeID := fmt.Sprintf("%s-%s", edgeView.SourceID, edgeView.TargetID)
		graphEdge := api.GraphEdge{
			Data: &api.EdgeData{
				Id:     edgeID,
				Source: edgeView.SourceID,
				Target: edgeView.TargetID,
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

// checkOwnership is no longer needed as CQRS services handle ownership internally

// ConnectNodes handles POST /api/nodes/{nodeId}/connections
// @Summary Connect two memory nodes
// @Description Creates a connection between two memory nodes to establish their relationship
// @Tags Graph Operations
// @Accept json
// @Produce json
// @Security Bearer
// @Param nodeId path string true "Source memory node ID"
// @Param request body api.ConnectNodesRequest true "Connection request with target node ID"
// @Success 200 {object} api.ConnectionResponse "Successfully created connection"
// @Failure 400 {object} api.ErrorResponse "Invalid request or nodes"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Node not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/{nodeId}/connections [post]
func (h *MemoryHandler) ConnectNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS buses are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sourceNodeID := chi.URLParam(r, "nodeId")
	
	var req api.ConnectNodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TargetNodeID == "" {
		api.Error(w, http.StatusBadRequest, "Target node ID is required")
		return
	}

	if sourceNodeID == req.TargetNodeID {
		api.Error(w, http.StatusBadRequest, "Cannot connect a node to itself")
		return
	}

	// Create command for CQRS pattern
	cmd := &coreCommands.ConnectNodesCommand{}
	cmd.UserID = userID
	cmd.SourceNodeID = sourceNodeID
	cmd.TargetNodeID = req.TargetNodeID
	cmd.Weight = 1.0 // Default weight
	cmd.Timestamp = time.Now()

	err := h.commandBus.Send(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Return success response
	api.Success(w, http.StatusOK, api.ConnectionResponse{
		Message: "Nodes connected successfully",
		EdgeID:  fmt.Sprintf("%s-%s", sourceNodeID, req.TargetNodeID),
	})
}

// DisconnectNodes handles DELETE /api/nodes/{nodeId}/connections/{targetNodeId}
// @Summary Disconnect two memory nodes
// @Description Removes the connection between two memory nodes
// @Tags Graph Operations
// @Security Bearer
// @Param nodeId path string true "Source memory node ID"
// @Param targetNodeId path string true "Target memory node ID"
// @Success 204 "Successfully removed connection"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "Connection not found"
// @Failure 503 {object} api.ErrorResponse "Service temporarily unavailable"
// @Router /nodes/{nodeId}/connections/{targetNodeId} [delete]
func (h *MemoryHandler) DisconnectNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS buses are available
	if h.commandBus == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	sourceNodeID := chi.URLParam(r, "nodeId")
	targetNodeID := chi.URLParam(r, "targetNodeId")

	if sourceNodeID == "" || targetNodeID == "" {
		api.Error(w, http.StatusBadRequest, "Both source and target node IDs are required")
		return
	}

	// Create command for CQRS pattern
	cmd := &coreCommands.DisconnectNodesCommand{}
	cmd.UserID = userID
	cmd.SourceNodeID = sourceNodeID
	cmd.TargetNodeID = targetNodeID
	cmd.Timestamp = time.Now()

	err := h.commandBus.Send(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

// generateNodeID generates a deterministic node ID
func generateNodeID(userID, content string) string {
	h := sha256.New()
	h.Write([]byte(userID))
	h.Write([]byte(content))
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))[:16] // Use first 16 chars for shorter IDs
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
