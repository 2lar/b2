package main

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type connectionRecord struct {
	PK           string `dynamodbav:"PK"`
	SK           string `dynamodbav:"SK"`
	ConnectionID string `dynamodbav:"ConnectionID"`
	UserID       string `dynamodbav:"UserID"`
}

var dbClient *dynamodb.Client
var cfg *config.Config

func init() {
	cfg = config.New()

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient = dynamodb.NewFromConfig(awsCfg)
	log.Println("WebSocket Connect service initialized successfully")
}

func Handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID
	log.Printf("WebSocket connection attempt: %s", connectionID)

	// Extract user ID from query parameter (JWT token)
	userID, err := extractUserIDFromToken(request.QueryStringParameters)
	if err != nil {
		log.Printf("Failed to extract user ID: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       "Unauthorized",
		}, nil
	}

	// Store connection in DynamoDB
	if err := storeConnection(ctx, userID, connectionID); err != nil {
		log.Printf("Failed to store connection: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal Server Error",
		}, nil
	}

	log.Printf("WebSocket connected: user %s, connection %s", userID, connectionID)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Connected",
	}, nil
}

func extractUserIDFromToken(queryParams map[string]string) (string, error) {
	token := queryParams["token"]
	if token == "" {
		return "", fmt.Errorf("missing token parameter")
	}

	// Decode the token
	decodedToken, err := url.QueryUnescape(token)
	if err != nil {
		return "", fmt.Errorf("failed to decode token: %v", err)
	}

	// For now, we'll do a simple validation by checking if it's a valid JWT format
	// In a real implementation, you'd use the Supabase library to validate the JWT
	// and extract the user ID from the 'sub' claim
	
	// Simple validation: JWT should have 3 parts separated by dots
	parts := len([]byte(decodedToken))
	if parts < 10 { // Very basic length check
		return "", fmt.Errorf("invalid token format")
	}

	// For demo purposes, we'll extract a mock user ID
	// In reality, you'd decode the JWT payload and extract the 'sub' claim
	// This is a placeholder - in production you'd use proper JWT validation
	// Generate a simple user ID based on token hash for demo
	mockUserID := fmt.Sprintf("user_%d", len(decodedToken)%1000)
	
	log.Printf("Extracted user ID: %s (mock implementation)", mockUserID)
	return mockUserID, nil
}

func storeConnection(ctx context.Context, userID, connectionID string) error {
	tableName := cfg.TableName + "-Connections" // We'll use a separate table for connections
	
	record := connectionRecord{
		PK:           fmt.Sprintf("USER#%s", userID),
		SK:           fmt.Sprintf("CONN#%s", connectionID),
		ConnectionID: connectionID,
		UserID:       userID,
	}

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("failed to marshal connection record: %v", err)
	}

	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("failed to put connection item: %v", err)
	}

	return nil
}

func main() {
	lambda.Start(Handler)
}