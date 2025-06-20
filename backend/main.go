package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// DBClient defines the interface for DynamoDB operations, making the app testable.
type DBClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

type App struct {
	db        DBClient // Use the interface
	tableName string
}

// This struct can be used for clarity but is not strictly necessary for this function
type AuthorizerContext struct {
	Subject string `json:"sub"` // Using 'sub' to match the JWT standard
	Email   string `json:"email"`
	Role    string `json:"role"`
}

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		log.Fatalf("TABLE_NAME environment variable is not set")
	}

	// The concrete dynamodb.Client satisfies the DBClient interface
	app := &App{
		db:        dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}

	log.Printf("Initialized app with table name: %s", tableName)

	lambda.Start(app.Handler)
}

func (app *App) Handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (Response, error) {
	log.Printf("Request received: Method=%s, RouteKey=%s, Body=%s",
		request.RequestContext.HTTP.Method, request.RouteKey, request.Body)

	// Extract user ID from JWT claims
	userID, err := getUserID(request)
	if err != nil {
		log.Printf("Failed to get user ID: %v", err)
		return errorResponse(401, "Unauthorized"), nil
	}

	log.Printf("User ID extracted: %s", userID)

	// Route based on path and method
	path := request.RawPath
	method := request.RequestContext.HTTP.Method

	log.Printf("Routing: Method=%s, Path=%s", method, path)

	switch {
	case method == "POST" && strings.HasPrefix(path, "/api/nodes"):
		log.Printf("Creating node for user %s", userID)
		return app.createNode(ctx, userID, request.Body)
	case method == "GET" && path == "/api/nodes":
		return app.listNodes(ctx, userID)
	case method == "GET" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return app.getNode(ctx, userID, nodeID)
	case method == "DELETE" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return app.deleteNode(ctx, userID, nodeID)
	case method == "GET" && path == "/api/graph-data":
		return app.getGraphData(ctx, userID)
	case method == "PUT" && strings.HasPrefix(path, "/api/nodes/"):
		nodeID := strings.TrimPrefix(path, "/api/nodes/")
		return app.updateNode(ctx, userID, nodeID, request.Body)
	default:
		log.Printf("Route not found: %s %s", method, path)
		return errorResponse(404, "Not Found"), nil
	}
}

func getUserID(request events.APIGatewayV2HTTPRequest) (string, error) {
	authorizer := request.RequestContext.Authorizer
	if authorizer == nil || authorizer.Lambda == nil {
		return "", fmt.Errorf("authorizer context is missing or lambda context is nil")
	}

	// The context from a SIMPLE response is in the `Lambda` field, which is a map[string]interface{}
	lambdaContext := authorizer.Lambda

	// Access the 'sub' key from the map and perform a type assertion to string
	sub, ok := lambdaContext["sub"].(string)
	if !ok || sub == "" {
		// For debugging, log the actual context received
		contextData, _ := json.Marshal(lambdaContext)
		return "", fmt.Errorf("user ID ('sub') not found or not a string in authorizer context: %s", string(contextData))
	}

	return sub, nil
}

func successResponse(data interface{}) Response {
	body, _ := json.Marshal(data)
	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}

func errorResponse(code int, message string) Response {
	body, _ := json.Marshal(map[string]string{"error": message})
	return Response{
		StatusCode: code,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}
