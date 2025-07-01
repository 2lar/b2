package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	apigwTypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dbClient *dynamodb.Client
var apiGatewayManagementClient *apigatewaymanagementapi.Client
var connectionsTable string

func init() {
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	wsApiEndpoint := os.Getenv("WEBSOCKET_API_ENDPOINT")
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	dbClient = dynamodb.NewFromConfig(awsCfg)
	apiGatewayManagementClient = apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &wsApiEndpoint
	})
}

type EdgesCreatedEvent struct {
	UserID string `json:"userId"`
	NodeID string `json:"nodeId"`
}

func handler(ctx context.Context, event events.EventBridgeEvent) error {
	var detail EdgesCreatedEvent
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		log.Printf("ERROR: could not unmarshal event detail: %v", err)
		return err
	}

	pk := "USER#" + detail.UserID
	result, err := dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(connectionsTable),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: pk},
			":sk_prefix": &types.AttributeValueMemberS{Value: "CONN#"},
		},
	})

	if err != nil {
		log.Printf("ERROR: Failed to query connections for user %s: %v", detail.UserID, err)
		return err
	}

	message := []byte(`{"action": "graphUpdated"}`)
	for _, item := range result.Items {
		connectionID := strings.TrimPrefix(item["SK"].(*types.AttributeValueMemberS).Value, "CONN#")
		_, err := apiGatewayManagementClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionID,
			Data:         message,
		})

		if err != nil {
			var goneErr *apigwTypes.GoneException
			if errors.As(err, &goneErr) {
				log.Printf("Found stale connection, deleting: %s", connectionID)
				dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
					TableName: aws.String(connectionsTable),
					Key: map[string]types.AttributeValue{
						"PK": item["PK"],
						"SK": item["SK"],
					},
				})
			} else {
				log.Printf("ERROR: Failed to post to connection %s: %v", connectionID, err)
			}
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
