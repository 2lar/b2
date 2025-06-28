// backend/cmd/ws-disconnect/main.go
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

	// The userID is passed from the connection's context, which was set by the authorizer on connect.
	userID, ok := request.RequestContext.Authorizer.(map[string]interface{})["sub"].(string)
	if !ok || userID == "" {
		log.Printf("Could not determine user for disconnection a of connection %s. It may have already been cleaned up.", connectionID)
		// Return 200 OK because the connection is already gone from the client's perspective.
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Disconnected"}, nil
	}

	log.Printf("WebSocket disconnecting: user %s, connection %s", userID, connectionID)

	if err := deleteConnection(ctx, userID, connectionID); err != nil {
		log.Printf("Failed to delete connection from DB: %v", err)
		// Don't return an error to the client, as the connection is closed anyway.
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Disconnected",
	}, nil
}

// deleteConnection performs a targeted delete, which is much more efficient than scanning.
func deleteConnection(ctx context.Context, userID, connectionID string) error {
	tableName := cfg.TableName + "-Connections"
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CONN#%s", connectionID)

	_, err := dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete connection item: %w", err)
	}

	log.Printf("Successfully deleted connection %s for user %s", connectionID, userID)
	return nil
}

func main() {
	lambda.Start(Handler)
}
