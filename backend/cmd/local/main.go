// Brain2 Local HTTP Server - For local development
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/repository/ddb"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/api"
	"brain2-backend/pkg/config"
	appErrors "brain2-backend/pkg/errors"
	"brain2-backend/pkg/tagger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/google/uuid"
)

type contextKey struct {
	name string
}

var userIDKey = contextKey{"userID"}
var memoryService memory.Service
var eventbridgeClient *eventbridge.Client
var tagService tagger.Tagger

func main() {
	log.Println("Starting Brain2 Backend in local mode...")

	cfg := config.New()
	
	// For local development, we'll use a mock user ID if no auth is present
	mockUserID := "local-dev-user"
	log.Printf("Using mock user ID for local development: %s", mockUserID)

	var repo repository.Repository
	
	// Check if we should use AWS services or local mock
	if os.Getenv("USE_LOCAL_STORAGE") == "true" {
		log.Println("Using local storage (not implemented yet - would use in-memory or local DB)")
		// TODO: Implement local storage for development
		log.Fatal("Local storage not implemented yet. Please set up DynamoDB for development.")
	} else {
		log.Println("Connecting to DynamoDB...")
		awsCfgOptions := []func(*awsConfig.LoadOptions) error{awsConfig.WithRegion(cfg.Region)}
		
		// If custom endpoint is specified, use it (for local DynamoDB)
		if cfg.DynamoDBEndpoint != "" {
			log.Printf("Using custom DynamoDB endpoint: %s", cfg.DynamoDBEndpoint)
			awsCfgOptions = append(awsCfgOptions, awsConfig.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					if service == dynamodb.ServiceID {
						return aws.Endpoint{
							URL: cfg.DynamoDBEndpoint,
						}, nil
					}
					return aws.Endpoint{}, &aws.EndpointNotFoundError{}
				}),
			))
		}
		
		awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsCfgOptions...)
		if err != nil {
			log.Fatalf("unable to load SDK config: %v", err)
		}
		
		dbClient := dynamodb.NewFromConfig(awsCfg)
		eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
		
		repo = ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	}
	
	// Initialize tagger service
	taggerConfig := tagger.Config{
		Type:          tagger.TaggerType(cfg.TaggerType),
		LocalLLMURL:   cfg.TaggerServiceURL,
		MaxTags:       cfg.TaggerMaxTags,
		EnableFallback: cfg.TaggerFallback,
	}
	
	if cfg.TaggerFallback {
		tagService = tagger.NewWithFallback(taggerConfig)
	} else {
		var err error
		tagService, err = tagger.NewTagger(taggerConfig)
		if err != nil {
			log.Fatalf("Failed to initialize tagger: %v", err)
		}
	}
	
	log.Printf("Tagger service initialized: type=%s, url=%s", cfg.TaggerType, cfg.TaggerServiceURL)
	
	memoryService = memory.NewService(repo)
	
	r := chi.NewRouter()
	
	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // SECURITY: Consider restricting in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}))
	
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	r.Route("/api", func(r chi.Router) {
		// For local development, inject mock user ID
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), userIDKey, mockUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		
		r.Get("/nodes", listNodesHandler)
		r.Post("/nodes", createNodeHandler)
		r.Get("/nodes/{nodeId}", getNodeHandler)
		r.Put("/nodes/{nodeId}", updateNodeHandler)
		r.Delete("/nodes/{nodeId}", deleteNodeHandler)
		
		r.Post("/nodes/bulk-delete", bulkDeleteNodesHandler)
		
		r.Get("/graph-data", getGraphDataHandler)
	})
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	
	log.Printf("Service initialized successfully")
	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func checkOwnership(ctx context.Context, nodeID string) (*domain.Node, error) {
	userID := ctx.Value(userIDKey).(string)
	node, _, err := memoryService.GetNodeDetails(ctx, userID, nodeID)
	if err != nil {
		if appErrors.IsNotFound(err) {
			return nil, err
		}
		return nil, appErrors.NewInternal("failed to verify node ownership", err)
	}

	if node.UserID != userID {
		return nil, appErrors.NewNotFound("node not found")
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

	// Generate tags using the tagger service
	keywords, err := tagService.GenerateTags(r.Context(), req.Content)
	if err != nil {
		log.Printf("Warning: Failed to generate tags, using fallback: %v", err)
		// Fallback to simple extraction if tagger fails
		keywords = memory.ExtractKeywords(req.Content)
	}

	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   req.Content,
		Keywords:  keywords,
		CreatedAt: time.Now(),
		Version:   0,
	}

	if err := memoryService.CreateNodeAndKeywords(r.Context(), node); err != nil {
		handleServiceError(w, err)
		return
	}

	// For local development, we'll skip EventBridge events
	if eventbridgeClient != nil {
		eventDetail, err := json.Marshal(map[string]interface{}{
			"userId":   node.UserID,
			"nodeId":   node.ID,
			"content":  node.Content,
			"keywords": node.Keywords,
		})
		if err == nil {
			eventbridgeClient.PutEvents(r.Context(), &eventbridge.PutEventsInput{
				Entries: []types.PutEventsRequestEntry{
					{
						Source:       aws.String("brain2.api"),
						DetailType:   aws.String("NodeCreated"),
						Detail:       aws.String(string(eventDetail)),
						EventBusName: aws.String("B2EventBus"),
					},
				},
			})
		}
	}

	api.Success(w, http.StatusCreated, api.NodeResponse{
		NodeID:    node.ID,
		Content:   node.Content,
		Timestamp: node.CreatedAt.Format(time.RFC3339),
		Version:   node.Version,
		Tags:      node.Keywords,
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
			Tags:      node.Keywords,
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
		Tags:      node.Keywords,
		Edges:     edgeIDs,
	})
}

func updateNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

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