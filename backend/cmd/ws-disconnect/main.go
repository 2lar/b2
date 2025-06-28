package main

import (
	"context"
	"fmt"
	"log"

	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dbClient *dynamodb.Client
var cfg *config.Config

func init() {
	cfg = config.New()

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient = dynamodb.NewFromConfig(awsCfg)
	log.Println("WebSocket Disconnect service initialized successfully")
}

func Handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID
	log.Printf("WebSocket disconnection: %s", connectionID)

	// Find and delete the connection record
	if err := deleteConnection(ctx, connectionID); err != nil {
		log.Printf("Failed to delete connection: %v", err)
		// Don't return error - disconnection should succeed even if cleanup fails
	}

	log.Printf("WebSocket disconnected: %s", connectionID)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Disconnected",
	}, nil
}

func deleteConnection(ctx context.Context, connectionID string) error {
	tableName := cfg.TableName + "-Connections"
	
	// First, we need to find the connection to get the user ID
	userID, err := findUserByConnection(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to find user for connection %s: %v", connectionID, err)
	}

	if userID == "" {
		log.Printf("Connection %s not found in database", connectionID)
		return nil // Connection not found, nothing to delete
	}

	// Delete the connection record
	_, err = dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CONN#%s", connectionID)},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete connection item: %v", err)
	}

	log.Printf("Deleted connection %s for user %s", connectionID, userID)
	return nil
}

func findUserByConnection(ctx context.Context, connectionID string) (string, error) {
	tableName := cfg.TableName + "-Connections"
	
	// Since we don't know the user ID, we need to scan the table to find the connection
	// This is not ideal for large datasets, but for a demo it's acceptable
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		FilterExpression: aws.String("ConnectionID = :connId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":connId": &types.AttributeValueMemberS{Value: connectionID},
		},
	}

	result, err := dbClient.Scan(ctx, scanInput)
	if err != nil {
		return "", fmt.Errorf("failed to scan for connection: %v", err)
	}

	if len(result.Items) == 0 {
		return "", nil // Connection not found
	}

	// Extract user ID from the first matching item
	userIDAttr, exists := result.Items[0]["UserID"]
	if !exists {
		return "", fmt.Errorf("UserID not found in connection record")
	}

	userIDValue, ok := userIDAttr.(*types.AttributeValueMemberS)
	if !ok {
		return "", fmt.Errorf("UserID is not a string")
	}

	return userIDValue.Value, nil
}

func main() {
	lambda.Start(Handler)
}