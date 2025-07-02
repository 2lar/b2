/**
 * =============================================================================
 * Brain2 Main HTTP API - RESTful Memory Management Server
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This is the primary HTTP API server for the Brain2 memory management system.
 * It demonstrates modern serverless architecture patterns, clean API design,
 * and enterprise-grade Go web service development using AWS Lambda.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. SERVERLESS HTTP API PATTERNS:
 *    - Lambda function with HTTP proxy integration
 *    - Chi router for elegant HTTP request routing
 *    - API Gateway v2 integration for modern HTTP APIs
 *    - Cold start optimization strategies
 * 
 * 2. CLEAN ARCHITECTURE IMPLEMENTATION:
 *    - Layered architecture (API → Service → Repository → Database)
 *    - Dependency injection for testability
 *    - Domain-driven design principles
 *    - Separation of concerns between layers
 * 
 * 3. EVENT-DRIVEN ARCHITECTURE:
 *    - EventBridge integration for decoupled event processing
 *    - Asynchronous memory processing workflows
 *    - Real-time notification system
 *    - Event sourcing patterns for audit trails
 * 
 * 4. AUTHENTICATION & AUTHORIZATION:
 *    - JWT token validation via API Gateway Lambda Authorizer
 *    - User context propagation through middleware
 *    - Resource ownership verification
 *    - Multi-tenant data isolation
 * 
 * 5. API DESIGN BEST PRACTICES:
 *    - RESTful resource naming and HTTP verb usage
 *    - Consistent error handling and status codes
 *    - Request/response validation and serialization
 *    - CORS configuration for web client support
 * 
 * 6. ENTERPRISE ERROR HANDLING:
 *    - Structured error types with business context
 *    - Graceful error propagation across layers
 *    - Security-conscious error messages
 *    - Comprehensive logging for monitoring
 * 
 * 🔄 REQUEST LIFECYCLE:
 * 1. API Gateway receives HTTP request
 * 2. Lambda Authorizer validates JWT token
 * 3. Request routed to appropriate handler
 * 4. Authentication middleware extracts user context
 * 5. Business logic executed via service layer
 * 6. Events published to EventBridge for async processing
 * 7. Response returned to client
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Serverless HTTP API development patterns
 * - Clean architecture in Go web services
 * - Event-driven microservice communication
 * - Enterprise authentication and authorization
 * - RESTful API design and implementation
 * - Error handling and middleware patterns
 */
package main

import (
	"context"     // Request lifecycle and cancellation
	"encoding/json" // JSON serialization for API requests/responses
	"fmt"          // String formatting for responses
	"log"          // Structured logging for monitoring
	"net/http"     // HTTP status codes and request handling
	"time"         // Timestamp management for domain entities

	// Internal packages - Clean architecture dependency flow
	"brain2-backend/internal/domain"     // Core business entities
	"brain2-backend/internal/repository/ddb" // DynamoDB repository implementation
	"brain2-backend/internal/service/memory" // Business logic service layer
	"brain2-backend/pkg/api"             // API types and utilities
	"brain2-backend/pkg/config"          // Configuration management
	appErrors "brain2-backend/pkg/errors" // Structured error handling

	// AWS Lambda and API Gateway integration
	"github.com/aws/aws-lambda-go/events" // Lambda event structures
	"github.com/aws/aws-lambda-go/lambda" // Lambda runtime integration

	// AWS SDK v2 for modern, efficient AWS service integration
	"github.com/aws/aws-sdk-go-v2/aws"               // Core AWS configuration
	awsConfig "github.com/aws/aws-sdk-go-v2/config" // Configuration loading
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"   // DynamoDB client
	"github.com/aws/aws-sdk-go-v2/service/eventbridge" // EventBridge client
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types" // EventBridge types

	// HTTP framework and middleware
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi" // Lambda-Chi integration
	"github.com/awslabs/aws-lambda-go-api-proxy/core"           // Lambda proxy utilities
	"github.com/go-chi/chi/v5"            // Modern HTTP router
	"github.com/go-chi/chi/v5/middleware" // Standard HTTP middleware
	"github.com/go-chi/cors"              // CORS handling for web clients

	// Utilities
	"github.com/google/uuid" // UUID generation for unique identifiers
)

/**
 * =============================================================================
 * Context Key Type - Type-Safe Context Value Storage
 * =============================================================================
 * 
 * CONTEXT SECURITY PATTERN:
 * Custom types for context keys prevent accidental key collisions and
 * provide compile-time safety when storing/retrieving values from context.
 * This is a Go best practice for request-scoped data.
 * 
 * WHY CUSTOM TYPES:
 * - Prevents string key collisions between packages
 * - Provides type safety at compile time
 * - Makes context usage more explicit and discoverable
 * - Follows Go standard library patterns
 */
type contextKey struct {
	name string // Descriptive name for debugging
}

/**
 * =============================================================================
 * Global Application State - Lambda Container Reuse Optimization
 * =============================================================================
 * 
 * LAMBDA PERFORMANCE OPTIMIZATION:
 * These global variables are initialized once per Lambda container and
 * reused across function invocations, providing significant performance
 * benefits by avoiding repeated initialization overhead.
 * 
 * SINGLETON PATTERN BENEFITS:
 * - Database clients reuse connection pools
 * - HTTP router initialization happens once
 * - Service layer dependencies injected once
 * - Dramatically reduces cold start impact
 */

// Context key for storing authenticated user ID
// SECURITY: Used to propagate user identity through request processing
var userIDKey = contextKey{"userID"}

// Lambda-Chi router adapter for HTTP request processing
// SERVERLESS INTEGRATION: Bridges AWS Lambda events to Chi HTTP router
var chiLambda *chiadapter.ChiLambdaV2

// Memory service for business logic operations
// DEPENDENCY INJECTION: Service layer with repository dependencies
var memoryService memory.Service

// EventBridge client for asynchronous event publishing
// EVENT-DRIVEN: Enables decoupled processing and real-time notifications
var eventbridgeClient *eventbridge.Client

/**
 * Lambda Container Initialization - Complete Application Bootstrap
 * 
 * This function runs once per Lambda container lifecycle, performing all
 * necessary initialization for the HTTP API server. It demonstrates
 * dependency injection, clean architecture setup, and performance
 * optimization patterns for serverless applications.
 * 
 * INITIALIZATION WORKFLOW:
 * 1. Load application configuration from environment
 * 2. Initialize AWS SDK clients with proper configuration
 * 3. Set up clean architecture dependency chain
 * 4. Configure HTTP router with middleware and routes
 * 5. Create Lambda-HTTP adapter for serverless execution
 * 
 * PERFORMANCE CONSIDERATIONS:
 * - One-time initialization reduces per-request latency
 * - Connection pooling for AWS services
 * - Router compilation happens once
 * - Middleware chain setup optimized for reuse
 * 
 * ERROR HANDLING STRATEGY:
 * - Fail fast with log.Fatalf for critical initialization errors
 * - Prevent partially initialized Lambda from accepting requests
 * - Clear error messages for operational debugging
 */
func init() {
	
	// ==========================================================================
	// Step 1: Application Configuration Loading
	// ==========================================================================
	//
	// CONFIGURATION MANAGEMENT:
	// Centralized configuration loading from environment variables
	// Provides type safety and validation for required settings
	// Enables different configurations per deployment environment
	
	cfg := config.New()
	
	// ==========================================================================
	// Step 2: AWS SDK Configuration and Authentication
	// ==========================================================================
	//
	// AWS CREDENTIAL CHAIN:
	// 1. Lambda execution role (preferred for serverless)
	// 2. Environment variables
	// 3. AWS credentials file
	// 4. EC2 instance metadata (not applicable for Lambda)
	//
	// REGION CONFIGURATION:
	// Explicit region setting ensures services operate in correct AWS region
	// Important for data locality and compliance requirements
	
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	
	// ==========================================================================
	// Step 3: AWS Service Client Initialization
	// ==========================================================================
	//
	// CLIENT REUSE PATTERN:
	// Initialize clients once and reuse across Lambda invocations
	// AWS SDK clients are thread-safe and include connection pooling
	// Significant performance improvement over per-request initialization
	
	// DynamoDB client for memory and connection state persistence
	dbClient := dynamodb.NewFromConfig(awsCfg)
	
	// EventBridge client for asynchronous event publishing
	// Enables real-time notifications and decoupled processing
	eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
	
	// ==========================================================================
	// Step 4: Clean Architecture Dependency Injection
	// ==========================================================================
	//
	// DEPENDENCY FLOW (Clean Architecture):
	// Infrastructure → Repository → Service → Handler
	// 
	// BENEFITS:
	// - Testability (can inject mock repositories)
	// - Flexibility (can swap implementations)
	// - Separation of concerns
	// - Domain-driven design principles
	
	// Repository layer: DynamoDB implementation with table configuration
	repo := ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	
	// Service layer: Business logic with injected repository dependency
	memoryService = memory.NewService(repo)
	
	// ==========================================================================
	// Step 5: HTTP Router Configuration and Middleware Setup
	// ==========================================================================
	//
	// CHI ROUTER BENEFITS:
	// - Lightweight and fast HTTP router
	// - Excellent middleware support
	// - RESTful route patterns
	// - Context-aware request handling
	// - Easy integration with Lambda
	
	r := chi.NewRouter()
	
	// ==========================================================================
	// Step 6: CORS Middleware - Web Browser Security
	// ==========================================================================
	//
	// CORS (Cross-Origin Resource Sharing):
	// Enables web browsers to make API calls from different domains
	// Required for modern single-page applications
	//
	// SECURITY CONSIDERATIONS:
	// - AllowedOrigins: "*" is permissive (consider restricting in production)
	// - AllowCredentials: true enables cookie/auth header sharing
	// - AllowedHeaders: includes common headers for API communication
	//
	// PRODUCTION SECURITY:
	// Consider restricting AllowedOrigins to specific frontend domains
	// Monitor for CORS-related security issues
	
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // SECURITY: Consider restricting in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true, // Enables authentication cookies/headers
	}))
	
	// ==========================================================================
	// Step 7: Standard HTTP Middleware - Observability and Resilience
	// ==========================================================================
	//
	// MIDDLEWARE CHAIN EXECUTION ORDER:
	// Middleware executes in the order added, wrapping each subsequent handler
	// Order matters for security, logging, and error handling
	
	// Request logging middleware for observability
	// Logs HTTP method, path, response status, and timing
	r.Use(middleware.Logger)
	
	// Panic recovery middleware for resilience
	// Catches panics and returns 500 errors instead of crashing
	// Essential for production stability
	r.Use(middleware.Recoverer)
	
	// ==========================================================================
	// Step 8: API Route Configuration - RESTful Resource Design
	// ==========================================================================
	//
	// RESTFUL API DESIGN PATTERNS:
	// - Resource-based URLs (/api/nodes)
	// - HTTP verbs for operations (GET, POST, PUT, DELETE)
	// - Hierarchical resource structure
	// - Consistent error handling and status codes
	//
	// AUTHENTICATION STRATEGY:
	// All /api routes require authentication via custom middleware
	// User context extracted from JWT and propagated through request chain
	
	r.Route("/api", func(r chi.Router) {
		// Authentication middleware applied to all API routes
		// Extracts user ID from JWT token and adds to request context
		r.Use(Authenticator)
		
		// MEMORY NODE CRUD OPERATIONS:
		r.Get("/nodes", listNodesHandler)           // GET    /api/nodes
		r.Post("/nodes", createNodeHandler)         // POST   /api/nodes
		r.Get("/nodes/{nodeId}", getNodeHandler)    // GET    /api/nodes/{id}
		r.Put("/nodes/{nodeId}", updateNodeHandler) // PUT    /api/nodes/{id}
		r.Delete("/nodes/{nodeId}", deleteNodeHandler) // DELETE /api/nodes/{id}
		
		// BULK OPERATIONS:
		r.Post("/nodes/bulk-delete", bulkDeleteNodesHandler) // POST /api/nodes/bulk-delete
		
		// GRAPH VISUALIZATION DATA:
		r.Get("/graph-data", getGraphDataHandler) // GET /api/graph-data
	})
	
	// ==========================================================================
	// Step 9: Lambda-HTTP Adapter Initialization
	// ==========================================================================
	//
	// LAMBDA INTEGRATION PATTERN:
	// ChiLambdaV2 adapter bridges AWS Lambda events to Chi HTTP router
	// Handles event parsing, context propagation, and response formatting
	//
	// API GATEWAY V2 INTEGRATION:
	// Supports modern HTTP API features like JWT authorizers
	// Better performance and lower cost than REST API v1
	
	chiLambda = chiadapter.NewV2(r)
	
	// ==========================================================================
	// Step 10: Initialization Completion Logging
	// ==========================================================================
	//
	// OPERATIONAL VISIBILITY:
	// Log successful initialization for monitoring and debugging
	// Helps identify cold start events and initialization timing
	
	log.Println("Service initialized successfully")
}

func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCtx, ok := core.GetAPIGatewayV2ContextFromContext(r.Context())
		if !ok {
			log.Println("Error: could not get proxy request context from context")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		userID, ok := proxyCtx.Authorizer.Lambda["sub"].(string)
		if !ok || userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func checkOwnership(ctx context.Context, nodeID string) (*domain.Node, error) {
	userID := ctx.Value(userIDKey).(string)
	node, _, err := memoryService.GetNodeDetails(ctx, userID, nodeID)
	if err != nil {
		// If the underlying error is a "not found" error, we return that.
		if appErrors.IsNotFound(err) {
			return nil, err
		}
		// Otherwise, it's an internal server error.
		return nil, appErrors.NewInternal("failed to verify node ownership", err)
	}

	// This check is redundant if GetNodeDetails is implemented correctly,
	// but it provides an explicit layer of defense-in-depth.
	if node.UserID != userID {
		return nil, appErrors.NewNotFound("node not found") // Obscure the reason for security
	}

	return node, nil
}

func createNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req api.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		api.Error(w, http.StatusBadRequest, "Content cannot be empty")
		return
	}

	// Create the node object
	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   req.Content,
		Keywords:  memory.ExtractKeywords(req.Content),
		CreatedAt: time.Now(),
		Version:   0,
	}

	// Save the node and its keywords to DynamoDB
	if err := memoryService.CreateNodeAndKeywords(r.Context(), node); err != nil {
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

	_, err = eventbridgeClient.PutEvents(r.Context(), &eventbridge.PutEventsInput{
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

	// Return immediate success to the client
	api.Success(w, http.StatusCreated, api.NodeResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
	})
}

func listNodesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	graph, err := memoryService.GetGraphData(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	var nodesResponse []api.NodeResponse
	for _, node := range graph.Nodes {
		nodesResponse = append(nodesResponse, api.NodeResponse{
			NodeID:    node.ID,
			Content:   node.Content,
			Timestamp: node.CreatedAt.Format(time.RFC3339),
			Version:   node.Version,
		})
	}
	api.Success(w, http.StatusOK, map[string][]api.NodeResponse{"nodes": nodesResponse})
}

func getNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	node, edges, err := memoryService.GetNodeDetails(r.Context(), userID, nodeID)
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
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
		Edges:     edgeIDs,
	})
}

func updateNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	// **SECURITY: Verify ownership before proceeding.**
	_, err := checkOwnership(r.Context(), nodeID)
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

	_, err = memoryService.UpdateNode(r.Context(), userID, nodeID, req.Content)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

func deleteNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	_, err := checkOwnership(r.Context(), nodeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := memoryService.DeleteNode(r.Context(), userID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func bulkDeleteNodesHandler(w http.ResponseWriter, r *http.Request) {
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

	deletedCount, failedNodeIds, err := memoryService.BulkDeleteNodes(r.Context(), userID, req.NodeIds)
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

func getGraphDataHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	graph, err := memoryService.GetGraphData(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	var elements []api.GraphDataResponse_Elements_Item

	for _, node := range graph.Nodes {
		label := node.Content
		if len(label) > 50 {
			label = label[:47] + "..."
		}

		graphNode := api.GraphNode{
			Data: &api.NodeData{
				Id:    &node.ID,
				Label: &label,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphNode(graphNode); err != nil {
			log.Printf("Error converting graph node: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	for _, edge := range graph.Edges {
		edgeID := fmt.Sprintf("%s-%s", edge.SourceID, edge.TargetID)
		graphEdge := api.GraphEdge{
			Data: &api.EdgeData{
				Id:     &edgeID,
				Source: &edge.SourceID,
				Target: &edge.TargetID,
			},
		}

		var element api.GraphDataResponse_Elements_Item
		if err := element.FromGraphEdge(graphEdge); err != nil {
			log.Printf("Error converting graph edge: %v", err)
			continue
		}
		elements = append(elements, element)
	}

	api.Success(w, http.StatusOK, api.GraphDataResponse{Elements: &elements})
}

func handleServiceError(w http.ResponseWriter, err error) {
	if appErrors.IsValidation(err) {
		api.Error(w, http.StatusBadRequest, err.Error())
	} else if appErrors.IsNotFound(err) {
		api.Error(w, http.StatusNotFound, err.Error())
	} else {
		log.Printf("INTERNAL ERROR: %v", err)
		api.Error(w, http.StatusInternalServerError, "An internal error occurred")
	}
}

func main() {
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return chiLambda.ProxyWithContextV2(ctx, req)
	})
}
