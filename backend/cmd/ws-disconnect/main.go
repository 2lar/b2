// Package main implements the WebSocket disconnect Lambda handler.
// This handler manages WebSocket connection cleanup and state management.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Global DynamoDB client for Lambda performance optimization
var dynamoClient *dynamodb.Client

// Connection represents a WebSocket connection record
type Connection struct {
	ConnectionID   string    `json:"connection_id"`
	UserID         string    `json:"user_id"`
	ConnectedAt    time.Time `json:"connected_at"`
	DisconnectedAt time.Time `json:"disconnected_at,omitempty"`
	LastPingAt     time.Time `json:"last_ping_at"`
	Endpoint       string    `json:"endpoint"`
}

func init() {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)

	log.Println("WebSocket disconnect handler initialized")
}

// getConnection retrieves connection information from DynamoDB
func getConnection(ctx context.Context, connectionID string) (*Connection, error) {
	tableName := os.Getenv("CONNECTIONS_TABLE_NAME")
	if tableName == "" {
		// Fallback to old env var name for backwards compatibility
		tableName = os.Getenv("CONNECTIONS_TABLE")
		if tableName == "" {
			tableName = "B2-Connections"
		}
	}

	// Use composite key structure: PK=CONNECTION#<id>, SK=METADATA
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CONNECTION#%s", connectionID)},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	}

	result, err := dynamoClient.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("connection not found")
	}

	var conn Connection
	if err := attributevalue.UnmarshalMap(result.Item, &conn); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection: %w", err)
	}

	return &conn, nil
}

// removeConnection deletes the connection from DynamoDB
func removeConnection(ctx context.Context, connectionID string) error {
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
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	log.Printf("Removed connection %s from table", connectionID)
	return nil
}

// archiveConnection moves the connection to an archive table for analytics
func archiveConnection(ctx context.Context, conn *Connection) error {
	archiveTable := os.Getenv("CONNECTIONS_ARCHIVE_TABLE")
	if archiveTable == "" {
		// Skip archiving if no archive table is configured
		return nil
	}

	conn.DisconnectedAt = time.Now()

	item, err := attributevalue.MarshalMap(conn)
	if err != nil {
		return fmt.Errorf("failed to marshal connection for archive: %w", err)
	}

	// Add session duration for analytics
	duration := conn.DisconnectedAt.Sub(conn.ConnectedAt).Minutes()
	item["SessionDurationMinutes"] = &types.AttributeValueMemberN{
		Value: fmt.Sprintf("%.2f", duration),
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(archiveTable),
		Item:      item,
	}

	_, err = dynamoClient.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to archive connection: %w", err)
	}

	log.Printf("Archived connection %s (duration: %.2f minutes)", conn.ConnectionID, duration)
	return nil
}

// cleanupUserState performs any user-specific cleanup
func cleanupUserState(ctx context.Context, userID string) error {
	// This could include:
	// - Clearing user presence status
	// - Updating last seen timestamp
	// - Notifying other connected clients
	// - Clearing temporary user state

	log.Printf("Cleaning up state for user %s", userID)

	// Update user's online status
	userTable := os.Getenv("USERS_TABLE")
	if userTable != "" {
		input := &dynamodb.UpdateItemInput{
			TableName: aws.String(userTable),
			Key: map[string]types.AttributeValue{
				"UserID": &types.AttributeValueMemberS{Value: userID},
			},
			UpdateExpression: aws.String("SET OnlineStatus = :status, LastSeenAt = :timestamp"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":status":    &types.AttributeValueMemberS{Value: "offline"},
				":timestamp": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			},
		}

		_, err := dynamoClient.UpdateItem(ctx, input)
		if err != nil {
			log.Printf("Failed to update user status: %v", err)
			// Non-critical error, continue
		}
	}

	return nil
}

// handler processes WebSocket disconnect requests
func handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID
	log.Printf("WebSocket disconnect request for connection: %s", connectionID)

	// Retrieve connection information
	conn, err := getConnection(ctx, connectionID)
	if err != nil {
		log.Printf("Failed to retrieve connection %s: %v", connectionID, err)
		// Continue with cleanup even if we can't find the connection
		// It might have been already cleaned up
	}

	// Archive connection for analytics (if configured)
	if conn != nil {
		if err := archiveConnection(ctx, conn); err != nil {
			log.Printf("Failed to archive connection: %v", err)
			// Non-critical error, continue
		}

		// Cleanup user-specific state
		if err := cleanupUserState(ctx, conn.UserID); err != nil {
			log.Printf("Failed to cleanup user state: %v", err)
			// Non-critical error, continue
		}
	}

	// Remove connection from active connections table
	if err := removeConnection(ctx, connectionID); err != nil {
		log.Printf("Failed to remove connection: %v", err)
		// Return success anyway - the connection is already closed
	}

	log.Printf("WebSocket connection %s disconnected and cleaned up", connectionID)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message": "disconnected"}`,
	}, nil
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting WebSocket disconnect Lambda")
		lambda.Start(handler)
	} else {
		// Local testing mode
		log.Println("Running in local test mode")

		// First, simulate a connection being stored
		testConn := Connection{
			ConnectionID: "test-connection-123",
			UserID:       "test-user-456",
			ConnectedAt:  time.Now().Add(-10 * time.Minute),
			LastPingAt:   time.Now().Add(-1 * time.Minute),
			Endpoint:     "test.execute-api.us-east-1.amazonaws.com/dev",
		}

		// Store test connection
		ctx := context.Background()
		tableName := "B2-Connections"
		item, _ := attributevalue.MarshalMap(testConn)
		_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      item,
		})
		if err != nil {
			log.Printf("Failed to store test connection: %v", err)
		}

		// Create a test disconnect request
		testRequest := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "test-connection-123",
				DomainName:   "test.execute-api.us-east-1.amazonaws.com",
				Stage:        "dev",
			},
		}

		// Process the test request
		response, err := handler(ctx, testRequest)
		if err != nil {
			log.Fatalf("Test request processing failed: %v", err)
		}

		log.Printf("Test response: %+v", response)
	}
}
