package main

import (
	"context"
	"encoding/json"
	"log"

	infradynamodb "brain2-backend/internal/infrastructure/persistence/dynamodb"
	"brain2-backend/internal/repository"
	"brain2-backend/pkg/config"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"go.uber.org/zap"
)

var repo repository.Repository
var eventbridgeClient *eventbridge.Client

// compositeRepository implements the repository.Repository interface by embedding all repositories
type compositeRepository struct {
	repository.NodeRepository
	repository.EdgeRepository
	repository.CategoryRepository
	repository.KeywordRepository
	repository.TransactionalRepository
	repository.GraphRepository
}

func init() {
	cfg := config.New()
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		logger, _ = zap.NewDevelopment()
	}
	
	dbClient := dynamodb.NewFromConfig(awsCfg)
	eventbridgeClient = eventbridge.NewFromConfig(awsCfg)
	
	// Create a simple composite repository for backward compatibility
	nodeRepo := infradynamodb.NewNodeRepository(dbClient, cfg.TableName, cfg.KeywordIndexName, logger)
	edgeRepo := infradynamodb.NewEdgeRepository(dbClient, cfg.TableName, cfg.KeywordIndexName, logger)
	categoryRepo := infradynamodb.NewCategoryRepository(dbClient, cfg.TableName, cfg.KeywordIndexName, logger)
	keywordRepo := infradynamodb.NewKeywordRepository(dbClient, cfg.TableName, cfg.KeywordIndexName)
	transactionalRepo := infradynamodb.NewTransactionalRepository(dbClient, cfg.TableName, cfg.KeywordIndexName, logger)
	graphRepo := infradynamodb.NewGraphRepository(dbClient, cfg.TableName, cfg.KeywordIndexName, logger)
	
	// Create composite repository
	repo = &compositeRepository{
		NodeRepository:          nodeRepo,
		EdgeRepository:          edgeRepo,
		CategoryRepository:      categoryRepo,
		KeywordRepository:       keywordRepo,
		TransactionalRepository: transactionalRepo,
		GraphRepository:         graphRepo,
	}
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
		if rn.ID().String() != detail.NodeID { // Don't connect a node to itself
			relatedNodeIDs = append(relatedNodeIDs, rn.ID().String())
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
		Entries: []types.PutEventsRequestEntry{
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

	log.Printf("Successfully processed node, created %d edges.", len(relatedNodeIDs))
	return nil
}

func main() {
	lambda.Start(handler)
}
