package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository/ddb"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/api"
	"brain2-backend/pkg/config"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

// Define a custom type for our context key.
type contextKey struct {
	name string
}

var userIDKey = contextKey{"userID"}
var chiLambda *chiadapter.ChiLambdaV2
var memoryService memory.Service
var eventbridgeClient *eventbridge.Client

func init() {
	cfg := config.New()
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient := dynamodb.NewFromConfig(awsCfg)
	eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
	repo := ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	memoryService = memory.NewService(repo)

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Use(Authenticator)
		r.Get("/nodes", listNodesHandler)
		r.Post("/nodes", createNodeHandler)
		r.Post("/nodes/bulk-delete", bulkDeleteNodesHandler)
		r.Get("/nodes/{nodeId}", getNodeHandler)
		r.Put("/nodes/{nodeId}", updateNodeHandler)
		r.Delete("/nodes/{nodeId}", deleteNodeHandler)
		r.Get("/graph-data", getGraphDataHandler)
	})

	chiLambda = chiadapter.NewV2(r)
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
