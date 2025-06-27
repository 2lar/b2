package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
)

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

	log.Println("Service initialized successfully")
}

func Handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Request received: Method=%s, Path=%s", request.RequestContext.HTTP.Method, request.RawPath)

	userID, ok := request.RequestContext.Authorizer.Lambda["sub"].(string)
	if !ok || userID == "" {
		return api.Error(401, "Unauthorized"), nil
	}

	method := request.RequestContext.HTTP.Method
	path := request.RawPath

	switch {
	case method == "POST" && path == "/api/nodes":
		return createNodeHandler(ctx, userID, request.Body)
	case method == "GET" && path == "/api/nodes":
		return listNodesHandler(ctx, userID)
	case method == "GET" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return getNodeHandler(ctx, userID, nodeID)
	case method == "PUT" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return updateNodeHandler(ctx, userID, nodeID, request.Body)
	case method == "DELETE" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return deleteNodeHandler(ctx, userID, nodeID)
	case method == "GET" && path == "/api/graph-data":
		return getGraphDataHandler(ctx, userID)
	default:
		return api.Error(404, "Not Found"), nil
	}
}

func main() {
	lambda.Start(Handler)
}

// --- Handler Functions ---

func createNodeHandler(ctx context.Context, userID, body string) (events.APIGatewayProxyResponse, error) {
	var req api.CreateNodeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return api.Error(400, "Invalid request body"), nil
	}
	node, err := memoryService.CreateNode(ctx, userID, req.Content)
	if err != nil {
		return handleServiceError(err)
	}
	return api.Success(201, api.NodeResponse{
		NodeID: node.ID, Content: node.Content, Timestamp: node.CreatedAt.Format(time.RFC3339), Version: node.Version,
	}), nil
}

func listNodesHandler(ctx context.Context, userID string) (events.APIGatewayProxyResponse, error) {
	graph, err := memoryService.GetGraphData(ctx, userID)
	if err != nil {
		return handleServiceError(err)
	}
	var nodesResponse []api.NodeResponse
	for _, node := range graph.Nodes {
		nodesResponse = append(nodesResponse, api.NodeResponse{
			NodeID: node.ID, Content: node.Content, Timestamp: node.CreatedAt.Format(time.RFC3339), Version: node.Version,
		})
	}
	return api.Success(200, map[string][]api.NodeResponse{"nodes": nodesResponse}), nil
}

func getNodeHandler(ctx context.Context, userID, nodeID string) (events.APIGatewayProxyResponse, error) {
	node, edges, err := memoryService.GetNodeDetails(ctx, userID, nodeID)
	if err != nil {
		return handleServiceError(err)
	}
	edgeIDs := make([]string, len(edges))
	for i, edge := range edges {
		edgeIDs[i] = edge.TargetID
	}
	return api.Success(200, api.NodeDetailsResponse{
		NodeID: node.ID, Content: node.Content, Timestamp: node.CreatedAt.Format(time.RFC3339), Version: node.Version, Edges: edgeIDs,
	}), nil
}

func updateNodeHandler(ctx context.Context, userID, nodeID, body string) (events.APIGatewayProxyResponse, error) {
	var req api.UpdateNodeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return api.Error(400, "Invalid request body"), nil
	}
	_, err := memoryService.UpdateNode(ctx, userID, nodeID, req.Content)
	if err != nil {
		return handleServiceError(err)
	}
	return api.Success(200, map[string]string{"message": "Node updated successfully"}), nil
}

func deleteNodeHandler(ctx context.Context, userID, nodeID string) (events.APIGatewayProxyResponse, error) {
	if err := memoryService.DeleteNode(ctx, userID, nodeID); err != nil {
		return handleServiceError(err)
	}
	return api.Success(204, nil), nil // 204 No Content is appropriate for a successful DELETE
}

func getGraphDataHandler(ctx context.Context, userID string) (events.APIGatewayProxyResponse, error) {
	graph, err := memoryService.GetGraphData(ctx, userID)
	if err != nil {
		return handleServiceError(err)
	}

	var elements []api.GraphDataResponse_Elements_Item

	// Add nodes
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

	// Add edges
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

	return api.Success(200, api.GraphDataResponse{Elements: &elements}), nil
}

// handleServiceError translates our custom errors into specific HTTP responses.
func handleServiceError(err error) (events.APIGatewayProxyResponse, error) {
	if appErrors.IsValidation(err) {
		return api.Error(400, err.Error()), nil
	}
	if appErrors.IsNotFound(err) {
		return api.Error(404, err.Error()), nil
	}
	// Default to internal server error
	log.Printf("INTERNAL ERROR: %v", err)
	return api.Error(500, "An internal error occurred"), nil
}
