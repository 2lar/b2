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

var dbClient *dynamodb.Client
var supabaseClient *supabase.Client
var connectionsTable string

func init() {
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	supabaseURL := os.Getenv("SUPABASE_URL")
	// For backend validation, we should use the service role key.
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

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	token, ok := req.QueryStringParameters["token"]
	if !ok || token == "" {
		log.Println("WARN: Connection request missing token.")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	// The GetUser method, when chained with WithToken, does not take a context argument.
	// The context is implicitly used in the underlying HTTP request made by the client.
	user, err := supabaseClient.Auth.WithToken(token).GetUser()
	if err != nil {
		log.Printf("ERROR: Invalid token provided. %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	connectionID := req.RequestContext.ConnectionID
	// Correctly convert the UUID type to a string for concatenation.
	userID := user.ID.String()
	// Set a TTL for the connection to ensure automatic cleanup of stale connections.
	expireAt := time.Now().Add(2 * time.Hour).Unix()

	pk := "USER#" + userID
	sk := "CONN#" + connectionID

	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(connectionsTable),
		Item: map[string]types.AttributeValue{
			"PK":       &types.AttributeValueMemberS{Value: pk},
			"SK":       &types.AttributeValueMemberS{Value: sk},
			"GSI1PK":   &types.AttributeValueMemberS{Value: sk}, // For disconnect lookup
			"GSI1SK":   &types.AttributeValueMemberS{Value: pk},
			"expireAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expireAt)},
		},
	})

	if err != nil {
		log.Printf("ERROR: Failed to save connection to DynamoDB: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	log.Printf("Successfully connected user %s with connection ID %s", userID, connectionID)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

func main() {
	lambda.Start(handler)
}
