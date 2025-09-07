// Package main implements the WebSocket connection Lambda handler.
// This handler manages WebSocket connection establishment with JWT authentication.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Global DynamoDB client for Lambda performance optimization
var dynamoClient *dynamodb.Client

// Connection represents a WebSocket connection record
type Connection struct {
	ConnectionID string    `json:"connection_id"`
	UserID       string    `json:"user_id"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastPingAt   time.Time `json:"last_ping_at"`
	Endpoint     string    `json:"endpoint"`
	TTL          int64     `json:"ttl"`
}

func init() {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	
	dynamoClient = dynamodb.NewFromConfig(cfg)
	
	log.Println("WebSocket connect handler initialized")
}

// validateToken validates the JWT token from query parameters
func validateToken(token string) (string, error) {
	// In a real implementation, this would:
	// 1. Validate JWT signature
	// 2. Check expiration
	// 3. Extract user ID from claims
	
	// For now, simplified validation
	if token == "" {
		return "", fmt.Errorf("missing authentication token")
	}
	
	// Mock user ID extraction
	// In production, decode JWT and extract user ID from claims
	userID := "user-" + token[:8]
	
	return userID, nil
}

// storeConnection saves the connection information to DynamoDB
func storeConnection(ctx context.Context, conn Connection) error {
	tableName := os.Getenv("CONNECTIONS_TABLE_NAME")
	if tableName == "" {
		// Fallback to old env var name for backwards compatibility
		tableName = os.Getenv("CONNECTIONS_TABLE")
		if tableName == "" {
			tableName = "B2-Connections"
		}
	}
	
	// Set TTL to 24 hours from now
	conn.TTL = time.Now().Add(24 * time.Hour).Unix()
	
	// Use composite key structure: PK=CONNECTION#<id>, SK=METADATA
	item := map[string]types.AttributeValue{
		"PK":           &types.AttributeValueMemberS{Value: fmt.Sprintf("CONNECTION#%s", conn.ConnectionID)},
		"SK":           &types.AttributeValueMemberS{Value: "METADATA"},
		"ConnectionID": &types.AttributeValueMemberS{Value: conn.ConnectionID},
		"UserID":       &types.AttributeValueMemberS{Value: conn.UserID},
		"GSI1PK":       &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", conn.UserID)},
		"GSI1SK":       &types.AttributeValueMemberS{Value: fmt.Sprintf("CONNECTION#%s", conn.ConnectionID)},
		"ConnectedAt":  &types.AttributeValueMemberS{Value: conn.ConnectedAt.Format(time.RFC3339)},
		"LastPingAt":   &types.AttributeValueMemberS{Value: conn.LastPingAt.Format(time.RFC3339)},
		"Endpoint":     &types.AttributeValueMemberS{Value: conn.Endpoint},
		"TTL":          &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", conn.TTL)},
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}
	
	_, err := dynamoClient.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to store connection: %w", err)
	}
	
	log.Printf("Stored connection %s for user %s", conn.ConnectionID, conn.UserID)
	return nil
}

// handler processes WebSocket connection requests
func handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("WebSocket connect request from connection: %s", request.RequestContext.ConnectionID)
	
	// Extract token from query parameters
	token := request.QueryStringParameters["token"]
	if token == "" {
		// Try Authorization header as fallback
		if auth := request.Headers["Authorization"]; auth != "" {
			token = auth
		}
	}
	
	// Validate token and extract user ID
	userID, err := validateToken(token)
	if err != nil {
		log.Printf("Authentication failed: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       `{"error": "unauthorized"}`,
		}, nil
	}
	
	// Create connection record
	connection := Connection{
		ConnectionID: request.RequestContext.ConnectionID,
		UserID:       userID,
		ConnectedAt:  time.Now(),
		LastPingAt:   time.Now(),
		Endpoint:     fmt.Sprintf("%s/%s", request.RequestContext.DomainName, request.RequestContext.Stage),
	}
	
	// Store connection in DynamoDB
	if err := storeConnection(ctx, connection); err != nil {
		log.Printf("Failed to store connection: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       `{"error": "internal server error"}`,
		}, nil
	}
	
	// Send welcome message
	welcomeMsg := map[string]interface{}{
		"type":         "connection_established",
		"connectionId": connection.ConnectionID,
		"userId":       userID,
		"timestamp":    time.Now().Unix(),
		"message":      "Welcome to Brain2 WebSocket API",
	}
	
	welcomeJSON, _ := json.Marshal(welcomeMsg)
	
	log.Printf("WebSocket connection established for user %s", userID)
	
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(welcomeJSON),
	}, nil
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting WebSocket connect Lambda")
		lambda.Start(handler)
	} else {
		// Local testing mode
		log.Println("Running in local test mode")
		
		// Create a test request
		testRequest := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "test-connection-123",
				DomainName:   "test.execute-api.us-east-1.amazonaws.com",
				Stage:        "dev",
			},
			QueryStringParameters: map[string]string{
				"token": "test-token-12345678",
			},
		}
		
		// Process the test request
		response, err := handler(context.Background(), testRequest)
		if err != nil {
			log.Fatalf("Test request processing failed: %v", err)
		}
		
		log.Printf("Test response: %+v", response)
	}
}