// WebSocket connection Lambda handles connection establishment with JWT authentication
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
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/supabase-community/supabase-go"
)

// Global clients for Lambda performance optimization

var dbClient *dynamodb.Client
var supabaseClient *supabase.Client
var connectionsTable string

// init initializes global clients and configuration
func init() {
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if connectionsTable == "" || supabaseURL == "" || supabaseKey == "" {
		log.Fatalf("FATAL: Environment variables CONNECTIONS_TABLE_NAME, SUPABASE_URL, and SUPABASE_SERVICE_ROLE_KEY must be set.")
	}

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	
	dbClient = dynamodb.NewFromConfig(awsCfg)

	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		log.Fatalf("Unable to create Supabase client: %v", err)
	}
	supabaseClient = client
}

// handler processes WebSocket connection requests with JWT authentication
func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	token, ok := req.QueryStringParameters["token"]
	if !ok || token == "" {
		log.Println("WARN: Connection request missing token.")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	user, err := supabaseClient.Auth.WithToken(token).GetUser()
	if err != nil {
		log.Printf("ERROR: Invalid token provided. %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	connectionID := req.RequestContext.ConnectionID
	userID := user.ID.String()
	expireAt := time.Now().Add(2 * time.Hour).Unix()

	pk := "USER#" + userID
	sk := "CONN#" + connectionID

	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(connectionsTable),
		Item: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
			"GSI1PK": &types.AttributeValueMemberS{Value: sk},
			"GSI1SK": &types.AttributeValueMemberS{Value: pk},
			"expireAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expireAt)},
		},
	})

	if err != nil {
		log.Printf("ERROR: Failed to save connection to DynamoDB: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	log.Println("WebSocket connection established successfully")
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

// main registers the WebSocket connection handler with Lambda runtime
func main() {
	lambda.Start(handler)
}
