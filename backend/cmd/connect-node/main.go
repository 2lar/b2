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

	log.Printf("Processing NodeCreated event for node %s with %d keywords", detail.NodeID, len(detail.Keywords))

	// Find related nodes using keyword-based search
	// For scalability, limit the number of nodes we query
	const maxNodesToQuery = 200
	const maxEdgesToCreate = 100
	const minMatchScore = 0.1 // Minimum relevance score to create an edge

	query := repository.NodeQuery{
		UserID:   detail.UserID,
		Keywords: detail.Keywords,
		Limit:    maxNodesToQuery,
	}
	
	relatedNodes, err := repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("ERROR: could not find related nodes: %v", err)
		return err
	}

	// Score and rank related nodes
	type scoredNode struct {
		nodeID string
		score  float64
	}
	var scoredNodes []scoredNode

	for _, rn := range relatedNodes {
		if rn.ID().String() == detail.NodeID {
			continue // Skip self
		}

		// Calculate relevance score based on keyword matches
		// For simplicity, we'll give each related node found a basic score
		// The FindNodes query already filters by keywords
		matchCount := 1 // Basic match since it was returned by keyword query

		if matchCount > 0 {
			// Calculate score as percentage of keywords matched
			score := float64(matchCount) / float64(len(detail.Keywords))
			if score >= minMatchScore {
				scoredNodes = append(scoredNodes, scoredNode{
					nodeID: rn.ID().String(),
					score:  score,
				})
			}
		}
	}

	// Sort by score (highest first)
	for i := 0; i < len(scoredNodes); i++ {
		for j := i + 1; j < len(scoredNodes); j++ {
			if scoredNodes[j].score > scoredNodes[i].score {
				scoredNodes[i], scoredNodes[j] = scoredNodes[j], scoredNodes[i]
			}
		}
	}

	// Limit to top N connections
	if len(scoredNodes) > maxEdgesToCreate {
		scoredNodes = scoredNodes[:maxEdgesToCreate]
	}

	// Create edges in batches for efficiency
	const batchSize = 25 // DynamoDB batch write limit
	totalCreated := 0

	for i := 0; i < len(scoredNodes); i += batchSize {
		end := i + batchSize
		if end > len(scoredNodes) {
			end = len(scoredNodes)
		}

		batch := scoredNodes[i:end]
		var batchNodeIDs []string
		for _, sn := range batch {
			batchNodeIDs = append(batchNodeIDs, sn.nodeID)
		}

		// Create edges for this batch
		if err := repo.CreateEdges(ctx, detail.UserID, detail.NodeID, batchNodeIDs); err != nil {
			log.Printf("WARNING: failed to create batch of edges: %v", err)
			// Continue with other batches even if one fails
		} else {
			totalCreated += len(batchNodeIDs)
		}
	}

	// Publish EdgesCreated event if we created any edges
	if totalCreated > 0 {
		edgesCreatedDetail, _ := json.Marshal(map[string]interface{}{
			"userId":     detail.UserID,
			"nodeId":     detail.NodeID,
			"edgeCount":  totalCreated,
		})
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
			log.Printf("WARNING: could not publish EdgesCreated event: %v", err)
			// Don't fail the handler for this
		}
	}

	log.Printf("Successfully processed node %s, created %d edges from %d candidates", 
		detail.NodeID, totalCreated, len(scoredNodes))
	return nil
}

func main() {
	lambda.Start(handler)
}
