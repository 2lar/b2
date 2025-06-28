// backend/cmd/ws-connect/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/supabase-community/gotrue-go"
)

type connectionRecord struct {
	PK           string `dynamodbav:"PK"`
	SK           string `dynamodbav:"SK"`
	ConnectionID string `dynamodbav:"ConnectionID"`
	UserID       string `dynamodbav:"UserID"`
}

var dbClient *dynamodb.Client
var supabaseClient gotrue.Client
var cfg *config.Config

func init() {
	cfg = config.New()

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY must be set")
	}

	supabaseClient = gotrue.New(supabaseURL, supabaseKey)

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	dbClient = dynamodb.NewFromConfig(awsCfg)
	log.Println("WebSocket Connect service initialized successfully")
}

func Handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := request.RequestContext.ConnectionID
	token := request.QueryStringParameters["token"]

	if token == "" {
		log.Println("Connect request missing token")
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized"}, nil
	}

	authedClient := supabaseClient.WithToken(token)

	// FINAL FIX: The GetUser method on the authedClient takes no arguments.
	user, err := authedClient.GetUser()
	if err != nil {
		log.Printf("Token validation failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Unauthorized"}, nil
	}

	userID := user.ID.String()
	log.Printf("Successfully authenticated user %s for connection %s", userID, connectionID)

	if err := storeConnection(ctx, userID, connectionID); err != nil {
		log.Printf("Failed to store connection: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Internal Server Error"}, nil
	}

	log.Printf("WebSocket connected: user %s, connection %s", userID, connectionID)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "Connected",
	}, nil
}

func storeConnection(ctx context.Context, userID, connectionID string) error {
	tableName := os.Getenv("TABLE_NAME")

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

	return err
}

func main() {
	lambda.Start(Handler)
}
