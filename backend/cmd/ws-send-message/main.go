package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type EdgesCreatedEvent struct {
	UserID string `json:"userId"`
	NodeID string `json:"nodeId"`
}

type WebSocketMessage struct {
	Action string `json:"action"`
	NodeID string `json:"nodeId,omitempty"`
}

var dbClient *dynamodb.Client
var apiGWClient *apigatewaymanagementapi.Client
var cfg *config.Config

func init() {
	cfg = config.New()

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient = dynamodb.NewFromConfig(awsCfg)
	
	// Configure API Gateway Management API client with WebSocket endpoint
	websocketEndpoint := os.Getenv("WEBSOCKET_API_ENDPOINT")
	if websocketEndpoint == "" {
		log.Fatalf("WEBSOCKET_API_ENDPOINT environment variable is required")
	}
	
	// Create custom endpoint resolver for API Gateway Management API
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == apigatewaymanagementapi.ServiceID {
			return aws.Endpoint{
				URL: websocketEndpoint,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown service %s", service)
	})
	
	awsCfgWithEndpoint, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(cfg.Region),
		awsConfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config with custom endpoint: %v", err)
	}
	
	apiGWClient = apigatewaymanagementapi.NewFromConfig(awsCfgWithEndpoint)
	
	log.Println("WebSocket SendMessage service initialized successfully")
}

func Handler(ctx context.Context, event events.EventBridgeEvent) error {
	log.Printf("🚀 WebSocket SendMessage Lambda triggered!")
	log.Printf("Received EventBridge event: %s from source: %s", event.DetailType, event.Source)
	log.Printf("Event detail: %s", string(event.Detail))

	if event.DetailType != "EdgesCreated" {
		log.Printf("Ignoring event with detail type: %s", event.DetailType)
		return nil
	}

	var edgeEvent EdgesCreatedEvent
	if err := json.Unmarshal(event.Detail, &edgeEvent); err != nil {
		log.Printf("Failed to unmarshal event detail: %v", err)
		return err
	}

	log.Printf("Sending WebSocket message for user %s about node %s", edgeEvent.UserID, edgeEvent.NodeID)

	// Find all active connections for the user
	log.Printf("Looking for active WebSocket connections for user %s", edgeEvent.UserID)
	connectionIDs, err := getActiveConnections(ctx, edgeEvent.UserID)
	if err != nil {
		log.Printf("Failed to get active connections: %v", err)
		return err
	}

	log.Printf("Found %d active connections for user %s: %v", len(connectionIDs), edgeEvent.UserID, connectionIDs)
	if len(connectionIDs) == 0 {
		log.Printf("No active connections found for user %s", edgeEvent.UserID)
		return nil
	}

	// Prepare the message to send
	message := WebSocketMessage{
		Action: "graphUpdated",
		NodeID: edgeEvent.NodeID,
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return err
	}

	// Send message to all active connections
	successCount := 0
	for _, connectionID := range connectionIDs {
		if err := sendMessage(ctx, connectionID, messageData); err != nil {
			log.Printf("Failed to send message to connection %s: %v", connectionID, err)
			// Clean up stale connection
			if err := removeStaleConnection(ctx, edgeEvent.UserID, connectionID); err != nil {
				log.Printf("Failed to remove stale connection %s: %v", connectionID, err)
			}
		} else {
			successCount++
		}
	}

	log.Printf("Successfully sent message to %d out of %d connections for user %s", 
		successCount, len(connectionIDs), edgeEvent.UserID)

	return nil
}

func getActiveConnections(ctx context.Context, userID string) ([]string, error) {
	tableName := cfg.TableName + "-Connections"
	
	queryInput := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		},
	}

	result, err := dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %v", err)
	}

	var connectionIDs []string
	for _, item := range result.Items {
		connIDAttr, exists := item["ConnectionID"]
		if !exists {
			continue
		}

		connIDValue, ok := connIDAttr.(*types.AttributeValueMemberS)
		if !ok {
			continue
		}

		connectionIDs = append(connectionIDs, connIDValue.Value)
	}

	return connectionIDs, nil
}

func sendMessage(ctx context.Context, connectionID string, messageData []byte) error {
	log.Printf("Sending WebSocket message to connection %s: %s", connectionID, string(messageData))
	
	_, err := apiGWClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         messageData,
	})

	if err != nil {
		log.Printf("Failed to send message to connection %s: %v", connectionID, err)
		return fmt.Errorf("failed to post to connection %s: %v", connectionID, err)
	}

	log.Printf("✅ Successfully sent message to connection %s", connectionID)
	return nil
}

func removeStaleConnection(ctx context.Context, userID, connectionID string) error {
	tableName := cfg.TableName + "-Connections"
	
	_, err := dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CONN#%s", connectionID)},
		},
	})

	return err
}

func main() {
	lambda.Start(Handler)
}