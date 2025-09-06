// Package main implements the WebSocket message broadcasting Lambda.
// This handler distributes real-time events to connected WebSocket clients.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	apigwTypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Global AWS clients for Lambda performance optimization
var (
	dynamoClient *dynamodb.Client
	apiGwClient  *apigatewaymanagementapi.Client
)

// BroadcastMessage represents a message to be sent to WebSocket clients
type BroadcastMessage struct {
	EventType    string                 `json:"event_type"`
	TargetUserID string                 `json:"target_user_id,omitempty"` // Optional: send to specific user
	TargetUsers  []string               `json:"target_users,omitempty"`   // Optional: send to multiple users
	Broadcast    bool                   `json:"broadcast,omitempty"`      // Send to all connected users
	Payload      map[string]interface{} `json:"payload"`
}

// WebSocketMessage represents the message format sent to clients
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func init() {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	
	dynamoClient = dynamodb.NewFromConfig(cfg)
	
	// API Gateway Management API client will be initialized per request
	// as it needs the specific endpoint
	
	log.Println("WebSocket send-message handler initialized")
}

// initializeAPIGatewayClient creates an API Gateway Management API client for the specific endpoint
func initializeAPIGatewayClient(endpoint string) *apigatewaymanagementapi.Client {
	cfg, _ := config.LoadDefaultConfig(context.Background())
	
	return apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s", endpoint))
	})
}

// getConnectionsForUser retrieves all active connections for a user
func getConnectionsForUser(ctx context.Context, userID string) ([]string, error) {
	tableName := os.Getenv("CONNECTIONS_TABLE_NAME")
	if tableName == "" {
		// Fallback to old env var name for backwards compatibility
		tableName = os.Getenv("CONNECTIONS_TABLE")
		if tableName == "" {
			tableName = "B2-Connections"
		}
	}
	
	// Query by GSI1 (User index) - GSI1PK=USER#<userID>
	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("connection-id-index"),
		KeyConditionExpression: aws.String("GSI1PK = :userpk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userpk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		},
	}
	
	result, err := dynamoClient.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %w", err)
	}
	
	var connectionIDs []string
	for _, item := range result.Items {
		if connID, ok := item["ConnectionID"].(*types.AttributeValueMemberS); ok {
			connectionIDs = append(connectionIDs, connID.Value)
		}
	}
	
	return connectionIDs, nil
}

// getAllConnections retrieves all active connections for broadcast
func getAllConnections(ctx context.Context) (map[string]string, error) {
	tableName := os.Getenv("CONNECTIONS_TABLE_NAME")
	if tableName == "" {
		// Fallback to old env var name for backwards compatibility
		tableName = os.Getenv("CONNECTIONS_TABLE")
		if tableName == "" {
			tableName = "B2-Connections"
		}
	}
	
	connections := make(map[string]string) // connectionID -> endpoint
	
	// Scan all connections
	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}
	
	paginator := dynamodb.NewScanPaginator(dynamoClient, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connections: %w", err)
		}
		
		for _, item := range page.Items {
			connID, _ := item["ConnectionID"].(*types.AttributeValueMemberS)
			endpoint, _ := item["Endpoint"].(*types.AttributeValueMemberS)
			if connID != nil && endpoint != nil {
				connections[connID.Value] = endpoint.Value
			}
		}
	}
	
	return connections, nil
}

// sendMessageToConnection sends a message to a specific WebSocket connection
func sendMessageToConnection(ctx context.Context, apiClient *apigatewaymanagementapi.Client, 
	connectionID string, message []byte) error {
	
	input := &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         message,
	}
	
	_, err := apiClient.PostToConnection(ctx, input)
	if err != nil {
		var goneErr *apigwTypes.GoneException
		if errors.As(err, &goneErr) {
			// Connection is stale, should be removed
			log.Printf("Connection %s is gone, marking for cleanup", connectionID)
			removeStaleConnection(ctx, connectionID)
			return nil // Don't treat as error
		}
		return fmt.Errorf("failed to send message: %w", err)
	}
	
	return nil
}

// removeStaleConnection removes a stale connection from DynamoDB
func removeStaleConnection(ctx context.Context, connectionID string) {
	tableName := os.Getenv("CONNECTIONS_TABLE_NAME")
	if tableName == "" {
		// Fallback to old env var name for backwards compatibility
		tableName = os.Getenv("CONNECTIONS_TABLE")
		if tableName == "" {
			tableName = "B2-Connections"
		}
	}
	
	// Use composite key structure: PK=CONNECTION#<id>, SK=METADATA
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CONNECTION#%s", connectionID)},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	}
	
	_, err := dynamoClient.DeleteItem(ctx, input)
	if err != nil {
		log.Printf("Failed to remove stale connection %s: %v", connectionID, err)
	} else {
		log.Printf("Removed stale connection %s", connectionID)
	}
}

// handleBroadcast sends a message to multiple WebSocket connections
func handleBroadcast(ctx context.Context, msg BroadcastMessage) error {
	// Prepare WebSocket message
	wsMessage := WebSocketMessage{
		Type:      msg.EventType,
		Timestamp: events.SecondsEpochTime{}.Unix(),
		Data:      msg.Payload,
	}
	
	messageJSON, err := json.Marshal(wsMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Determine target connections
	targetConnections := make(map[string]string) // connectionID -> endpoint
	
	if msg.Broadcast {
		// Send to all connections
		targetConnections, err = getAllConnections(ctx)
		if err != nil {
			return fmt.Errorf("failed to get all connections: %w", err)
		}
	} else if msg.TargetUserID != "" {
		// Send to specific user
		connectionIDs, err := getConnectionsForUser(ctx, msg.TargetUserID)
		if err != nil {
			return fmt.Errorf("failed to get user connections: %w", err)
		}
		
		// Get endpoints for these connections
		for _, connID := range connectionIDs {
			// Simplified - in production, batch get items
			endpoint := os.Getenv("WEBSOCKET_ENDPOINT")
			if endpoint == "" {
				endpoint = "execute-api.us-east-1.amazonaws.com/prod"
			}
			targetConnections[connID] = endpoint
		}
	} else if len(msg.TargetUsers) > 0 {
		// Send to multiple users
		for _, userID := range msg.TargetUsers {
			connectionIDs, err := getConnectionsForUser(ctx, userID)
			if err != nil {
				log.Printf("Failed to get connections for user %s: %v", userID, err)
				continue
			}
			
			for _, connID := range connectionIDs {
				endpoint := os.Getenv("WEBSOCKET_ENDPOINT")
				if endpoint == "" {
					endpoint = "execute-api.us-east-1.amazonaws.com/prod"
				}
				targetConnections[connID] = endpoint
			}
		}
	}
	
	// Group connections by endpoint
	endpointGroups := make(map[string][]string)
	for connID, endpoint := range targetConnections {
		endpointGroups[endpoint] = append(endpointGroups[endpoint], connID)
	}
	
	// Send messages
	successCount := 0
	failCount := 0
	
	for endpoint, connectionIDs := range endpointGroups {
		apiClient := initializeAPIGatewayClient(endpoint)
		
		for _, connID := range connectionIDs {
			if err := sendMessageToConnection(ctx, apiClient, connID, messageJSON); err != nil {
				log.Printf("Failed to send to connection %s: %v", connID, err)
				failCount++
			} else {
				successCount++
			}
		}
	}
	
	log.Printf("Broadcast complete: %d successful, %d failed", successCount, failCount)
	
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("all message sends failed")
	}
	
	return nil
}

// handler processes different types of events for WebSocket broadcasting
func handler(ctx context.Context, event json.RawMessage) error {
	log.Printf("Received event for broadcasting")
	
	// Try to parse as EventBridge event (domain events)
	var cloudWatchEvent events.CloudWatchEvent
	if err := json.Unmarshal(event, &cloudWatchEvent); err == nil {
		// Handle domain events
		log.Printf("Processing domain event: %s", cloudWatchEvent.DetailType)
		
		// Convert domain event to broadcast message
		var payload map[string]interface{}
		if err := json.Unmarshal(cloudWatchEvent.Detail, &payload); err != nil {
			return fmt.Errorf("failed to parse event detail: %w", err)
		}
		
		// Determine target based on event type
		msg := BroadcastMessage{
			EventType: cloudWatchEvent.DetailType,
			Payload:   payload,
		}
		
		// Extract user ID from event if available
		if userID, ok := payload["user_id"].(string); ok && userID != "" {
			msg.TargetUserID = userID
		} else {
			// Broadcast to all if no specific user
			msg.Broadcast = true
		}
		
		return handleBroadcast(ctx, msg)
	}
	
	// Try to parse as direct broadcast message
	var broadcastMsg BroadcastMessage
	if err := json.Unmarshal(event, &broadcastMsg); err == nil {
		return handleBroadcast(ctx, broadcastMsg)
	}
	
	// Try to parse as SQS event (for batched messages)
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err == nil {
		for _, record := range sqsEvent.Records {
			var msg BroadcastMessage
			if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
				log.Printf("Failed to parse SQS message: %v", err)
				continue
			}
			
			if err := handleBroadcast(ctx, msg); err != nil {
				log.Printf("Failed to broadcast message: %v", err)
				// Continue processing other messages
			}
		}
		return nil
	}
	
	return fmt.Errorf("unable to parse event")
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting WebSocket send-message Lambda")
		lambda.Start(handler)
	} else {
		// Local testing mode
		log.Println("Running in local test mode")
		
		// Create a test broadcast message
		testMsg := BroadcastMessage{
			EventType:    "NodeCreated",
			TargetUserID: "test-user-456",
			Payload: map[string]interface{}{
				"node_id": "test-node-123",
				"title":   "Test Node",
				"content": "This is a test node",
			},
		}
		
		testJSON, _ := json.Marshal(testMsg)
		
		// Process the test message
		if err := handler(context.Background(), testJSON); err != nil {
			log.Fatalf("Test message processing failed: %v", err)
		}
		
		log.Println("Test message processed successfully")
	}
}