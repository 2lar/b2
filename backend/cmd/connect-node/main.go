package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/repository/ddb"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/config"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridge_types "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

type NodeCreatedEvent struct {
	UserID string `json:"userId"`
	NodeID string `json:"nodeId"`
}

var memoryService memory.Service
var repo repository.Repository
var eventBridgeClient *eventbridge.Client
var cfg *config.Config

func init() {
	cfg = config.New()

	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	dbClient := dynamodb.NewFromConfig(awsCfg)
	eventBridgeClient = eventbridge.NewFromConfig(awsCfg)
	repo = ddb.NewRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	memoryService = memory.NewService(repo)

	log.Println("ConnectNode service initialized successfully")
}

func Handler(ctx context.Context, event events.EventBridgeEvent) error {
	log.Printf("Received EventBridge event: %s", event.DetailType)

	if event.DetailType != "NodeCreated" {
		log.Printf("Ignoring event with detail type: %s", event.DetailType)
		return nil
	}

	var nodeEvent NodeCreatedEvent
	if err := json.Unmarshal(event.Detail, &nodeEvent); err != nil {
		log.Printf("Failed to unmarshal event detail: %v", err)
		return err
	}

	log.Printf("Processing connections for node %s owned by user %s", nodeEvent.NodeID, nodeEvent.UserID)

	// Find the newly created node
	node, _, err := memoryService.GetNodeDetails(ctx, nodeEvent.UserID, nodeEvent.NodeID)
	if err != nil {
		log.Printf("Failed to get node details: %v", err)
		return err
	}

	if node == nil {
		log.Printf("Node %s not found", nodeEvent.NodeID)
		return appErrors.NewNotFound("node not found")
	}

	// Find related nodes using keywords
	query := repository.NodeQuery{
		UserID:   nodeEvent.UserID,
		Keywords: node.Keywords,
	}
	relatedNodes, err := repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("Non-critical error finding related nodes: %v", err)
		relatedNodes = []domain.Node{} // Continue with empty list
	}

	// Extract related node IDs (excluding the current node)
	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != nodeEvent.NodeID {
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	// Create edges to related nodes
	if err := createEdgesForNode(ctx, nodeEvent.UserID, nodeEvent.NodeID, relatedNodeIDs); err != nil {
		log.Printf("Failed to create edges: %v", err)
		return err
	}

	log.Printf("Successfully created %d edges for node %s", len(relatedNodeIDs), nodeEvent.NodeID)

	// Publish EdgesCreated event
	if err := publishEdgesCreatedEvent(ctx, nodeEvent.UserID, nodeEvent.NodeID); err != nil {
		log.Printf("Failed to publish EdgesCreated event: %v", err)
		return err
	}

	return nil
}

func createEdgesForNode(ctx context.Context, userID, nodeID string, relatedNodeIDs []string) error {
	return repo.CreateEdgesOnly(ctx, userID, nodeID, relatedNodeIDs)
}

func publishEdgesCreatedEvent(ctx context.Context, userID, nodeID string) error {
	eventDetail := fmt.Sprintf(`{"userId": "%s", "nodeId": "%s"}`, userID, nodeID)
	
	_, err := eventBridgeClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []eventbridge_types.PutEventsRequestEntry{
			{
				Source:       aws.String("brain2.edges"),
				DetailType:   aws.String("EdgesCreated"),
				Detail:       aws.String(eventDetail),
				EventBusName: aws.String(cfg.EventBusName),
			},
		},
	})
	
	return err
}

func main() {
	lambda.Start(Handler)
}