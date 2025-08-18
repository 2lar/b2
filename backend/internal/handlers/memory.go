// Package handlers provides HTTP handlers with clean dependency injection.
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

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/pkg/api"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/go-chi/chi/v5"
)

// MemoryHandler handles memory-related HTTP requests with CQRS services.
type MemoryHandler struct {
	// CQRS services for clean separation of concerns
	nodeService       *services.NodeService      // Write operations (commands)
	nodeQueryService  *queries.NodeQueryService  // Read operations (queries)
	graphQueryService *queries.GraphQueryService // Graph operations (queries)
	
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

// NewMemoryHandler creates a new memory handler with CQRS services.
func NewMemoryHandler(
	nodeService *services.NodeService,
	nodeQueryService *queries.NodeQueryService,
	graphQueryService *queries.GraphQueryService,
	eventBridgeClient *eventbridge.Client,
	container ColdStartContainer,
) *MemoryHandler {
	return &MemoryHandler{
		nodeService:       nodeService,
		nodeQueryService:  nodeQueryService,
		graphQueryService: graphQueryService,
		eventBridgeClient: eventBridgeClient,
		container:         container,
	}
}


// CreateNode handles POST /api/nodes
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeService == nil {
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

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	// Create command for CQRS pattern
	cmd := &commands.CreateNodeCommand{
		UserID:  userID,
		Content: req.Content,
		Tags:    tags,
	}

	// Add idempotency key if provided
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		cmd.IdempotencyKey = idempotencyKey
	} else {
		// Generate automatic key
		cmd.IdempotencyKey = generateIdempotencyKey(userID, "CREATE_NODE", req)
	}

	// Execute command through CQRS service
	result, err := h.nodeService.CreateNode(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// Validate result is not nil (defensive programming for idempotency issues)
	if result == nil || result.Node == nil {
		log.Printf("ERROR: CreateNode returned nil result or nil Node for user %s", userID)
		api.Error(w, http.StatusInternalServerError, "Failed to create node - invalid response from service")
		return
	}

	// Publish "NodeCreated" event to EventBridge with complete graph update
	eventDetail, err := json.Marshal(map[string]any{
		"type":      "nodeCreated",
		"userId":    result.Node.UserID,
		"nodeId":    result.Node.ID,
		"content":   result.Node.Content,
		"keywords":  result.Node.Keywords,
		"edges":     result.Connections, // Use connections from CQRS result
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
		NodeID:    result.Node.ID,
		Content:   result.Node.Content,
		Tags:      result.Node.Tags,
		Timestamp: result.Node.CreatedAt.Format(time.RFC3339),
		Version:   result.Node.Version,
	})
}

// ListNodes handles GET /api/nodes
func (h *MemoryHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeQueryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		log.Printf("ERROR: ListNodes - Authentication failed, getUserID returned false")
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

	log.Printf("DEBUG: ListNodes called for userID: %s, limit: %d, nextToken: %s", userID, limit, urlQuery.Get("nextToken"))

	// Create query for CQRS pattern
	listQuery, err := queries.NewListNodesQuery(userID)
	if err != nil {
		log.Printf("ERROR: ListNodes - failed to create query: %v", err)
		handleServiceError(w, err)
		return
	}
	
	listQuery.WithPagination(limit, urlQuery.Get("nextToken"))
	
	// Add search filter if provided
	if searchQuery := urlQuery.Get("search"); searchQuery != "" {
		listQuery.WithSearch(searchQuery)
	}
	
	// Add sorting if provided
	if sortBy := urlQuery.Get("sortBy"); sortBy != "" {
		sortDirection := urlQuery.Get("sortDirection")
		if sortDirection == "" {
			sortDirection = "desc"
		}
		listQuery.WithSort(sortBy, sortDirection)
	}

	response, err := h.nodeQueryService.ListNodes(r.Context(), listQuery)
	if err != nil {
		log.Printf("ERROR: ListNodes - nodeQueryService.ListNodes failed: %v", err)
		handleServiceError(w, err)
		return
	}

	if response == nil {
		log.Printf("ERROR: ListNodes - received nil response from service")
		api.Error(w, http.StatusInternalServerError, "Service returned no data")
		return
	}

	log.Printf("DEBUG: ListNodes - received response with %d total items, hasMore: %v", response.Total, response.HasMore)

	// Convert NodeView DTOs to API response format
	apiNodes := make([]api.Node, len(response.Nodes))
	for i, nodeView := range response.Nodes {
		apiNodes[i] = api.Node{
			NodeID:    nodeView.ID,
			UserID:    nodeView.UserID,
			Content:   nodeView.Content,
			Tags:      nodeView.Tags,
			Metadata:  nil, // NodeView doesn't include metadata field
			Timestamp: nodeView.CreatedAt.Format(time.RFC3339),
			CreatedAt: nodeView.CreatedAt.Format(time.RFC3339),
			UpdatedAt: nodeView.UpdatedAt.Format(time.RFC3339),
		}
	}

	log.Printf("DEBUG: ListNodes - successfully converted %d nodes to API format", len(apiNodes))

	nodesResponse := map[string]interface{}{
		"nodes":     apiNodes,
		"total":     response.Total,
		"hasMore":   response.HasMore,
		"nextToken": response.NextToken,
	}

	log.Printf("DEBUG: ListNodes - returning response with %d nodes, total: %d, hasMore: %v", len(apiNodes), response.Total, response.HasMore)
	api.Success(w, http.StatusOK, nodesResponse)
}

// GetNode handles GET /api/nodes/{nodeId}
func (h *MemoryHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeQueryService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Validate nodeID
	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// Check for common JavaScript undefined-to-string conversion
	if nodeID == "undefined" || nodeID == "null" {
		api.Error(w, http.StatusBadRequest, "Node must be created before operation")
		return
	}

	// Check if this is a post-cold-start request for conditional behavior
	isPostColdStart := h.container.IsPostColdStartRequest()
	if isPostColdStart {
		timeSince := h.container.GetTimeSinceColdStart()
		log.Printf("GetNode: Processing post-cold-start request (%v after cold start) for node %s", timeSince, nodeID)
		
		// Add cold start headers to response
		w.Header().Set("X-Cold-Start", "true")
		w.Header().Set("X-Cold-Start-Age", timeSince.String())
	}

	// Create query for CQRS pattern
	nodeQuery, err := queries.NewGetNodeQuery(userID, nodeID)
	if err != nil {
		if isPostColdStart {
			log.Printf("GetNode: Error creating query during post-cold-start request for node %s: %v", nodeID, err)
		}
		handleServiceError(w, err)
		return
	}
	
	// Include connections in the query
	nodeQuery.WithConnections()

	result, err := h.nodeQueryService.GetNode(r.Context(), nodeQuery)
	if err != nil {
		if isPostColdStart {
			log.Printf("GetNode: Error during post-cold-start request for node %s: %v", nodeID, err)
		}
		handleServiceError(w, err)
		return
	}

	// Extract connected node IDs from connections
	connectedNodeIDs := make(map[string]bool) // Use map to avoid duplicates
	for _, connection := range result.Connections {
		// Add the "other" node ID
		if connection.SourceNodeID == nodeID {
			// Current node is source, so target is the connected node
			if connection.TargetNodeID != nodeID { // Avoid self-references
				connectedNodeIDs[connection.TargetNodeID] = true
			}
		} else if connection.TargetNodeID == nodeID {
			// Current node is target, so source is the connected node
			if connection.SourceNodeID != nodeID { // Avoid self-references
				connectedNodeIDs[connection.SourceNodeID] = true
			}
		}
	}
	
	// Convert map to slice
	edgeIDs := make([]string, 0, len(connectedNodeIDs))
	for id := range connectedNodeIDs {
		edgeIDs = append(edgeIDs, id)
	}

	response := api.NodeDetailsResponse{
		NodeID:    result.Node.ID,
		Content:   result.Node.Content,
		Tags:      result.Node.Tags,
		Timestamp: result.Node.CreatedAt.Format(time.RFC3339),
		Version:   result.Node.Version,
		Edges:     edgeIDs,
	}

	if isPostColdStart {
		log.Printf("GetNode: Successfully completed post-cold-start request for node %s", nodeID)
	}

	api.Success(w, http.StatusOK, response)
}

// UpdateNode handles PUT /api/nodes/{nodeId}
func (h *MemoryHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Validate nodeID
	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// Check for common JavaScript undefined-to-string conversion
	if nodeID == "undefined" || nodeID == "null" {
		api.Error(w, http.StatusBadRequest, "Node must be created before operation")
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

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	// Create command for CQRS pattern
	cmd := &commands.UpdateNodeCommand{
		NodeID:  nodeID,
		UserID:  userID,
		Content: req.Content,
		Tags:    tags,
	}

	// Add idempotency key if provided
	if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
		// Note: UpdateNodeCommand doesn't have IdempotencyKey field, 
		// but we'll handle it in the service layer if needed
	}

	_, err := h.nodeService.UpdateNode(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

// DeleteNode handles DELETE /api/nodes/{nodeId}
func (h *MemoryHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeService == nil {
		api.Error(w, http.StatusServiceUnavailable, "Service temporarily unavailable - CQRS migration in progress")
		return
	}
	
	userID, ok := getUserID(r)
	if !ok {
		api.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	nodeID := chi.URLParam(r, "nodeId")

	// Validate nodeID
	if nodeID == "" {
		api.Error(w, http.StatusBadRequest, "Node ID is required")
		return
	}

	// Check for common JavaScript undefined-to-string conversion
	if nodeID == "undefined" || nodeID == "null" {
		api.Error(w, http.StatusBadRequest, "Node must be created before operation")
		return
	}

	// Create command for CQRS pattern
	cmd := &commands.DeleteNodeCommand{
		NodeID: nodeID,
		UserID: userID,
	}

	_, err := h.nodeService.DeleteNode(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node deleted successfully"})
}

// BulkDeleteNodes handles POST /api/nodes/bulk-delete
func (h *MemoryHandler) BulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.nodeService == nil {
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

	// Create command for CQRS pattern
	cmd := &commands.BulkDeleteNodesCommand{
		NodeIDs: req.NodeIDs,
		UserID:  userID,
	}

	result, err := h.nodeService.BulkDeleteNodes(r.Context(), cmd)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	message := fmt.Sprintf("Successfully deleted %d nodes", result.DeletedCount)
	if len(result.FailedIDs) > 0 {
		message += fmt.Sprintf(", failed to delete %d nodes", len(result.FailedIDs))
	}

	api.Success(w, http.StatusOK, api.BulkDeleteResponse{
		DeletedCount: result.DeletedCount,
		FailedIDs:    result.FailedIDs,
	})
}

// GetGraphData handles GET /api/graph-data
func (h *MemoryHandler) GetGraphData(w http.ResponseWriter, r *http.Request) {
	// Check if CQRS services are available
	if h.graphQueryService == nil {
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
	graphQuery := &queries.GetGraphQuery{
		UserID:         userID,
		Limit:          0, // No limit for now
		IncludeMetrics: false,
	}

	log.Printf("DEBUG: Calling graphQueryService.GetGraph")
	result, err := h.graphQueryService.GetGraph(r.Context(), graphQuery)
	if err != nil {
		log.Printf("ERROR: GetGraphData failed: %v", err)
		handleServiceError(w, err)
		return
	}

	log.Printf("DEBUG: GetGraphData succeeded, graph has %d nodes and %d edges", len(result.Nodes), len(result.Edges))

	var elements []api.GraphDataResponse_Elements_Item

	// Handle case where result is nil (should not happen, but defensive programming)
	if result == nil {
		log.Printf("WARN: GetGraphData returned nil result, returning empty response")
		api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
		return
	}

	for _, nodeView := range result.Nodes {
		label := nodeView.Content
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

	for _, edgeView := range result.Edges {
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
