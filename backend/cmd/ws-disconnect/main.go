package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dbClient *dynamodb.Client
var connectionsTable string
var gsiName string

func init() {
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	gsiName = os.Getenv("CONNECTIONS_GSI_NAME")
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	dbClient = dynamodb.NewFromConfig(awsCfg)
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := req.RequestContext.ConnectionID
	sk := "CONN#" + connectionID

	// Query GSI to find the PK for the given connectionId
	result, err := dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &connectionsTable,
		IndexName:              &gsiName,
		KeyConditionExpression: aws.String("GSI1PK = :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sk": &types.AttributeValueMemberS{Value: sk},
		},
	})

	if err != nil {
		log.Printf("ERROR: Failed to query GSI for disconnect: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	if len(result.Items) == 0 {
		log.Printf("WARN: Connection ID %s not found for disconnect.", connectionID)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
	}

	var item struct {
		PK string `dynamodbav:"PK"`
		SK string `dynamodbav:"SK"`
	}
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		log.Printf("ERROR: Failed to unmarshal item: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// Delete the connection from the table
	_, err = dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &connectionsTable,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: item.PK},
			"SK": &types.AttributeValueMemberS{Value: item.SK},
		},
	})

	if err != nil {
		log.Printf("ERROR: Failed to delete connection: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

func main() {
	lambda.Start(handler)
}
