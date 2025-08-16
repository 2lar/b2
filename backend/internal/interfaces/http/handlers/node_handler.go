// Package handlers provides clean HTTP handlers following best practices.
// This package demonstrates the Handler Layer Excellence pattern with proper
// separation of concerns, consistent error handling, and clean architecture.
//
// Key Concepts Illustrated:
//   - Single Responsibility: Handlers only handle HTTP concerns
//   - Dependency Injection: Services are injected, not created
//   - Command/Query Separation: Different services for reads and writes
//   - DTO Pattern: Request/Response transformation at boundaries
//   - Error Handling: Consistent error responses using error types
//   - Validation: Input validation before business logic
//   - Logging: Structured logging for observability
//   - Security: Authentication, authorization, and input sanitization
//
// Design Principles:
//   - Handlers are thin: They orchestrate but don't contain business logic
//   - Clear boundaries: HTTP concerns stay in the handler layer
//   - Testability: Dependencies are injected for easy mocking
//   - Consistency: All handlers follow the same patterns
//   - Documentation: Clear comments explain the why, not just the what
package handlers

import (
	"brain2-backend/internal/application/commands"
	appDto "brain2-backend/internal/application/dto"
	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/interfaces/http/dto"
	httpErrors "brain2-backend/internal/interfaces/http/errors"
	"brain2-backend/internal/interfaces/http/response"
	"brain2-backend/internal/interfaces/http/validation"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// NodeHandler handles HTTP requests for node operations.
// It demonstrates clean separation between HTTP concerns and business logic.
//
// This handler:
//   1. Parses and validates HTTP requests
//   2. Converts requests to application commands/queries
//   3. Delegates to application services
//   4. Converts results to HTTP responses
//   5. Handles errors consistently
//
// Notice how this handler has NO business logic - it's purely an adapter
// between HTTP and the application layer.
type NodeHandler struct {
	// Application services (injected dependencies)
	nodeService  *services.NodeService           // Handles write operations (commands)
	queryService *queries.NodeQueryService       // Handles read operations (queries)
	
	// Infrastructure services
	validator    *validation.Validator           // Validates requests
	logger       *zap.Logger                     // Structured logging
	
	// Configuration
	isProduction bool                            // Controls error detail exposure
}

// NewNodeHandler creates a new node handler with dependency injection.
// This constructor ensures all dependencies are provided and valid.
func NewNodeHandler(
	nodeService *services.NodeService,
	queryService *queries.NodeQueryService,
	validator *validation.Validator,
	logger *zap.Logger,
	isProduction bool,
) *NodeHandler {
	if nodeService == nil {
		panic("nodeService is required")
	}
	if queryService == nil {
		panic("queryService is required")
	}
	if validator == nil {
		validator = validation.GetValidator() // Use default if not provided
	}
	if logger == nil {
		logger = zap.NewNop() // Use no-op logger if not provided
	}
	
	return &NodeHandler{
		nodeService:  nodeService,
		queryService: queryService,
		validator:    validator,
		logger:       logger.Named("NodeHandler"),
		isProduction: isProduction,
	}
}

// CreateNode handles POST /api/nodes
//
// This endpoint demonstrates:
//   - Request parsing and validation
//   - Command pattern for write operations
//   - Proper HTTP status codes (201 Created)
//   - Location header for created resource
//   - Structured error handling
func (h *NodeHandler) CreateNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID from authenticated context
		userID, err := h.getUserID(r)
		if err != nil {
			h.logger.Debug("Authentication failed", zap.Error(err))
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Parse request body
		var request dto.CreateNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			h.logger.Debug("Failed to decode request", zap.Error(err))
			httpErrors.NewBadRequest("Invalid request body").Write(w)
			return
		}
		
		// 3. Validate request
		if err := h.validator.Validate(&request); err != nil {
			h.logger.Debug("Validation failed", zap.Error(err))
			httpErrors.NewValidationError(err).Write(w)
			return
		}
		
		// 4. Sanitize input
		request.Sanitize()
		
		// 5. Convert to application command
		command, err := request.ToCommand(userID)
		if err != nil {
			h.logger.Debug("Failed to create command", zap.Error(err))
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// Add idempotency key if provided
		if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
			command.WithIdempotencyKey(idempotencyKey)
		}
		
		// 6. Execute use case through application service
		result, err := h.nodeService.CreateNode(ctx, command)
		if err != nil {
			h.logger.Error("Failed to create node", 
				zap.String("user_id", userID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 7. Build successful response
		location := r.URL.Path + "/" + result.Node.ID
		
		response.New(w, r).
			Status(http.StatusCreated).
			Header("Location", location).
			Data(result).
			WithRequestID(h.getRequestID(r)).
			Send()
		
		h.logger.Info("Node created successfully",
			zap.String("user_id", userID),
			zap.String("node_id", result.Node.ID))
	}
}

// GetNode handles GET /api/nodes/{nodeId}
//
// This endpoint demonstrates:
//   - Path parameter extraction
//   - Query pattern for read operations
//   - Caching headers (ETag, Cache-Control)
//   - Conditional requests (If-None-Match)
func (h *NodeHandler) GetNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Extract path parameter
		nodeID := chi.URLParam(r, "nodeId")
		if nodeID == "" {
			httpErrors.NewBadRequest("Node ID is required").Write(w)
			return
		}
		
		// 3. Parse query parameters for includes
		includeConnections := r.URL.Query().Get("include_connections") == "true"
		includeMetadata := r.URL.Query().Get("include_metadata") == "true"
		
		// 4. Create query
		query, err := queries.NewGetNodeQuery(userID, nodeID)
		if err != nil {
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		if includeConnections {
			query.WithConnections()
		}
		if includeMetadata {
			query.WithMetadata()
		}
		
		// 5. Execute query through query service
		result, err := h.queryService.GetNode(ctx, query)
		if err != nil {
			h.logger.Debug("Failed to get node",
				zap.String("node_id", nodeID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 6. Build response with caching
		response.New(w, r).
			Status(http.StatusOK).
			Data(result).
			Cache(300). // Cache for 5 minutes
			WithRequestID(h.getRequestID(r)).
			Send()
	}
}

// UpdateNode handles PUT /api/nodes/{nodeId}
//
// This endpoint demonstrates:
//   - Partial updates with optional fields
//   - Optimistic locking with version checking
//   - Idempotency support
func (h *NodeHandler) UpdateNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Extract path parameter
		nodeID := chi.URLParam(r, "nodeId")
		if nodeID == "" {
			httpErrors.NewBadRequest("Node ID is required").Write(w)
			return
		}
		
		// 3. Parse request
		var request dto.UpdateNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			httpErrors.NewBadRequest("Invalid request body").Write(w)
			return
		}
		
		// 4. Validate request
		if err := h.validator.Validate(&request); err != nil {
			httpErrors.NewValidationError(err).Write(w)
			return
		}
		
		// 5. Check if there are any changes
		if !request.HasChanges() {
			httpErrors.NewBadRequest("No changes provided").Write(w)
			return
		}
		
		// 6. Sanitize input
		request.Sanitize()
		
		// 7. Convert to command
		command, err := request.ToCommand(userID, nodeID)
		if err != nil {
			h.logger.Debug("Failed to create update command", zap.Error(err))
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// Add idempotency key if provided
		if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
			command.WithIdempotencyKey(idempotencyKey)
		}
		
		// 8. Execute update
		result, err := h.nodeService.UpdateNode(ctx, command)
		if err != nil {
			h.logger.Error("Failed to update node",
				zap.String("node_id", nodeID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 9. Send response
		response.New(w, r).
			Status(http.StatusOK).
			Data(result).
			WithRequestID(h.getRequestID(r)).
			Send()
		
		h.logger.Info("Node updated successfully",
			zap.String("user_id", userID),
			zap.String("node_id", nodeID))
	}
}

// DeleteNode handles DELETE /api/nodes/{nodeId}
//
// This endpoint demonstrates:
//   - Proper DELETE semantics
//   - 204 No Content response
//   - Idempotent deletion
func (h *NodeHandler) DeleteNode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Extract path parameter
		nodeID := chi.URLParam(r, "nodeId")
		if nodeID == "" {
			httpErrors.NewBadRequest("Node ID is required").Write(w)
			return
		}
		
		// 3. Create command
		command, err := commands.NewDeleteNodeCommand(userID, nodeID)
		if err != nil {
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// Add idempotency key if provided
		if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
			command.WithIdempotencyKey(idempotencyKey)
		}
		
		// 4. Execute deletion
		_, err = h.nodeService.DeleteNode(ctx, command)
		if err != nil {
			h.logger.Error("Failed to delete node",
				zap.String("node_id", nodeID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 5. Send 204 No Content
		response.NoContent(w, r)
		
		h.logger.Info("Node deleted successfully",
			zap.String("user_id", userID),
			zap.String("node_id", nodeID))
	}
}

// ListNodes handles GET /api/nodes
//
// This endpoint demonstrates:
//   - Query parameter parsing
//   - Pagination with next tokens
//   - Filtering and sorting
//   - List response format
func (h *NodeHandler) ListNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Parse query parameters
		var request dto.ListNodesRequest
		
		// Parse limit
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil {
				httpErrors.NewBadRequest("Invalid limit parameter").Write(w)
				return
			}
			request.Limit = limit
		}
		
		// Parse pagination token
		request.NextToken = r.URL.Query().Get("next_token")
		
		// Parse filters
		if tags := r.URL.Query()["tag"]; len(tags) > 0 {
			request.Tags = tags
		}
		
		// Parse sorting
		request.SortBy = r.URL.Query().Get("sort_by")
		request.Order = r.URL.Query().Get("order")
		
		// 3. Set defaults
		request.SetDefaults()
		
		// 4. Create query
		query, err := request.ToQuery(userID)
		if err != nil {
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// 5. Execute query
		result, err := h.queryService.ListNodes(ctx, query)
		if err != nil {
			h.logger.Error("Failed to list nodes",
				zap.String("user_id", userID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 6. Build paginated response
		builder := response.New(w, r).
			Status(http.StatusOK).
			Data(result.Nodes).
			WithRequestID(h.getRequestID(r))
		
		// Add pagination metadata
		if result.NextToken != "" {
			builder.WithNextToken(result.NextToken, result.HasMore)
		}
		
		builder.Send()
	}
}

// BulkDeleteNodes handles POST /api/nodes/bulk-delete
//
// This endpoint demonstrates:
//   - Bulk operations with limits
//   - Partial success handling
//   - Detailed error reporting
func (h *NodeHandler) BulkDeleteNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Parse request
		var request dto.BulkDeleteNodesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			httpErrors.NewBadRequest("Invalid request body").Write(w)
			return
		}
		
		// 3. Validate request
		if err := h.validator.Validate(&request); err != nil {
			httpErrors.NewValidationError(err).Write(w)
			return
		}
		
		// 4. Convert to command
		command, err := request.ToCommand(userID)
		if err != nil {
			h.logger.Debug("Failed to create bulk delete command", zap.Error(err))
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// Add idempotency key if provided
		if idempotencyKey := r.Header.Get("Idempotency-Key"); idempotencyKey != "" {
			command.WithIdempotencyKey(idempotencyKey)
		}
		
		// 5. Execute bulk deletion
		result, err := h.nodeService.BulkDeleteNodes(ctx, command)
		if err != nil {
			h.logger.Error("Failed to bulk delete nodes",
				zap.String("user_id", userID),
				zap.Int("count", len(request.NodeIDs)),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 6. Send response
		status := http.StatusOK
		if result.DeletedCount == 0 {
			status = http.StatusBadRequest
		} else if len(result.FailedIDs) > 0 {
			status = http.StatusPartialContent
		}
		
		response.New(w, r).
			Status(status).
			Data(result).
			WithRequestID(h.getRequestID(r)).
			Send()
		
		h.logger.Info("Bulk delete completed",
			zap.String("user_id", userID),
			zap.Int("deleted", result.DeletedCount),
			zap.Int("failed", len(result.FailedIDs)))
	}
}

// ConnectNodes handles POST /api/nodes/connect
//
// This endpoint demonstrates:
//   - Creating relationships between entities
//   - Weight/strength parameters
//   - Validation of entity relationships
func (h *NodeHandler) ConnectNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ctx := r.Context() // Commented out as it's not used yet
		_ = r.Context() // temporary to avoid unused variable
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Parse request
		var request dto.ConnectNodesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			httpErrors.NewBadRequest("Invalid request body").Write(w)
			return
		}
		
		// 3. Validate request
		if err := h.validator.Validate(&request); err != nil {
			httpErrors.NewValidationError(err).Write(w)
			return
		}
		
		// 4. Convert to command
		command, err := request.ToCommand(userID)
		if err != nil {
			h.logger.Debug("Failed to create connect command", zap.Error(err))
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// 5. Execute connection
		// TODO: Implement ConnectNodes in NodeService
		// result, err := h.nodeService.ConnectNodes(ctx, command)
		_ = command // temporary
		result := &appDto.CreateConnectionResult{Success: true} // temporary stub
		err = nil // temporary
		if err != nil {
			h.logger.Error("Failed to connect nodes",
				zap.String("source", request.SourceNodeID),
				zap.String("target", request.TargetNodeID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 6. Send response
		response.New(w, r).
			Status(http.StatusCreated).
			Data(result).
			WithRequestID(h.getRequestID(r)).
			Send()
		
		h.logger.Info("Nodes connected successfully",
			zap.String("user_id", userID),
			zap.String("source", request.SourceNodeID),
			zap.String("target", request.TargetNodeID))
	}
}

// GetGraphData handles GET /api/graph-data
//
// This endpoint demonstrates:
//   - Complex data aggregation
//   - Performance optimization with limits
//   - Graph visualization support
func (h *NodeHandler) GetGraphData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// 1. Extract user ID
		userID, err := h.getUserID(r)
		if err != nil {
			httpErrors.NewUnauthorized("Authentication required").Write(w)
			return
		}
		
		// 2. Create query with defaults
		query, err := queries.NewGetGraphDataQuery(userID)
		if err != nil {
			httpErrors.NewBadRequest(err.Error()).Write(w)
			return
		}
		
		// 3. Parse optional parameters
		if includeArchived := r.URL.Query().Get("include_archived"); includeArchived == "true" {
			query.WithArchived()
		}
		
		// 4. Execute query
		result, err := h.queryService.GetGraphData(ctx, query)
		if err != nil {
			h.logger.Error("Failed to get graph data",
				zap.String("user_id", userID),
				zap.Error(err))
			httpErrors.FromError(err, h.isProduction).Write(w)
			return
		}
		
		// 5. Use the graph view from result
		graphView := result.Graph
		
		// 6. Send response with caching
		response.New(w, r).
			Status(http.StatusOK).
			Data(graphView).
			Cache(60). // Cache for 1 minute
			WithRequestID(h.getRequestID(r)).
			Send()
		
		h.logger.Debug("Graph data retrieved",
			zap.String("user_id", userID),
			zap.Int("nodes", len(graphView.Nodes)),
			zap.Int("edges", len(graphView.Connections)))
	}
}

// Helper methods

// getUserID extracts the authenticated user ID from the request context
func (h *NodeHandler) getUserID(r *http.Request) (string, error) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		return "", httpErrors.NewUnauthorized("User ID not found in context")
	}
	
	userIDStr, ok := userID.(string)
	if !ok {
		return "", httpErrors.NewUnauthorized("Invalid user ID in context")
	}
	
	if userIDStr == "" {
		return "", httpErrors.NewUnauthorized("Empty user ID")
	}
	
	return userIDStr, nil
}

// getRequestID extracts the request ID from the context for tracing
func (h *NodeHandler) getRequestID(r *http.Request) string {
	if reqID := r.Context().Value("request_id"); reqID != nil {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	return r.Header.Get("X-Request-ID")
}