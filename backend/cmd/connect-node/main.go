package main

import (
	"context"
	"encoding/json"
	"log"

	"brain2-backend/internal/repository"
	"brain2-backend/internal/repository/ddb"
	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types" // CORRECTED
)

var repo repository.Repository
var eventbridgeClient *eventbridge.Client

func init() {
	cfg := config.New()
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	dbClient := dynamodb.NewFromConfig(awsCfg)
	eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
	repo = ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
}

type NodeCreatedEvent struct {
	UserID   string   `json:"userId"`
	NodeID   string   `json:"nodeId"`
	Keywords []string `json:"keywords"`
}

func handler(ctx context.Context, event events.EventBridgeEvent) error {
	var detail NodeCreatedEvent
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		log.Printf("ERROR: could not unmarshal event detail: %v", err)
		return err
	}

	// Find related nodes
	query := repository.NodeQuery{
		UserID:   detail.UserID,
		Keywords: detail.Keywords,
	}
	relatedNodes, err := repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("ERROR: could not find related nodes: %v", err)
		return err // Returning an error will allow EventBridge to retry
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != detail.NodeID { // Don't connect a node to itself
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	// Create edges if related nodes were found
	if len(relatedNodeIDs) > 0 {
		if err := repo.CreateEdges(ctx, detail.UserID, detail.NodeID, relatedNodeIDs); err != nil {
			log.Printf("ERROR: could not create edges: %v", err)
			return err
		}
	}

	// Publish EdgesCreated event
	edgesCreatedDetail, _ := json.Marshal(map[string]string{"userId": detail.UserID, "nodeId": detail.NodeID})
	_, err = eventbridgeClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{ // CORRECTED
			{
				Source:       aws.String("brain2.connectNode"),
				DetailType:   aws.String("EdgesCreated"),
				Detail:       aws.String(string(edgesCreatedDetail)),
				EventBusName: aws.String("B2EventBus"),
			},
		},
	})
	if err != nil {
		log.Printf("ERROR: could not publish EdgesCreated event: %v", err)
		return err
	}

	log.Printf("Successfully processed node %s for user %s, created %d edges.", detail.NodeID, detail.UserID, len(relatedNodeIDs))
	return nil
}

func main() {
	lambda.Start(handler)
}
