package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/repository/ddb"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/api"
	"brain2-backend/pkg/config"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Define a custom type for our context key.
// The empty struct `struct{}` is used because it allocates no memory.
type contextKey struct {
	name string
}

// Create a package-level variable for our user ID key.
var userIDKey = contextKey{"userID"}

var chiLambda *chiadapter.ChiLambdaV2
var memoryService memory.Service

func init() {
	cfg := config.New()

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient := dynamodb.NewFromConfig(awsCfg)
	repo := ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	memoryService = memory.NewService(repo)

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Use(Authenticator)

		r.Get("/nodes", listNodesHandler)
		r.Post("/nodes", createNodeHandler)
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

		// Use our custom key instead of a raw string.
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Use the correct V2 proxy method
	return chiLambda.ProxyWithContextV2(ctx, req)
}

func main() {
	lambda.Start(Handler)
}

// --- Handler Functions ---

func createNodeHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the user ID using our custom key.
	userID := r.Context().Value(userIDKey).(string)

	var req api.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	node, err := memoryService.CreateNode(r.Context(), userID, req.Content)
	if err != nil {
		handleServiceError(w, err)
		return
	}

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

	var req api.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := memoryService.UpdateNode(r.Context(), userID, nodeID, req.Content)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	api.Success(w, http.StatusOK, map[string]string{"message": "Node updated successfully"})
}

func deleteNodeHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	nodeID := chi.URLParam(r, "nodeId")

	if err := memoryService.DeleteNode(r.Context(), userID, nodeID); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
		return
	}
	if appErrors.IsNotFound(err) {
		api.Error(w, http.StatusNotFound, err.Error())
		return
	}
	log.Printf("INTERNAL ERROR: %v", err)
	api.Error(w, http.StatusInternalServerError, "An internal error occurred")
}
