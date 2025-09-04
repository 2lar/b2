package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// EventDetail represents the top-level event detail from EventBridge
type EventDetail struct {
	EventID       string                 `json:"eventId"`
	EventType     string                 `json:"eventType"`
	AggregateID   string                 `json:"aggregateId"`
	AggregateType string                 `json:"aggregateType"`
	Version       int                    `json:"version"`
	OccurredAt    string                 `json:"occurredAt"`
	Data          map[string]interface{} `json:"data"` // The entire NodeCreatedEvent is here
	Metadata      map[string]interface{} `json:"metadata"`
}

func handler(ctx context.Context, event events.EventBridgeEvent) error {
	// First try to unmarshal the raw detail to inspect structure
	var rawDetail map[string]interface{}
	if err := json.Unmarshal(event.Detail, &rawDetail); err != nil {
		log.Printf("ERROR: could not unmarshal raw event detail: %v", err)
		return err
	}
	
	// Log the structure for debugging
	log.Printf("Event detail structure received: %+v", rawDetail)
	
	// Extract the node ID (should be in aggregateId field)
	nodeID, ok := rawDetail["aggregateId"].(string)
	if !ok {
		// Try alternative field names
		if nid, ok := rawDetail["nodeId"].(string); ok {
			nodeID = nid
		} else {
			log.Printf("ERROR: could not extract nodeId from event")
			return fmt.Errorf("missing nodeId in event")
		}
	}
	
	// Extract user ID - try multiple possible locations
	var userID string
	if uid, ok := rawDetail["userId"].(string); ok {
		userID = uid
	} else if uid, ok := rawDetail["user_id"].(string); ok {
		userID = uid
	} else if metadata, ok := rawDetail["metadata"].(map[string]interface{}); ok {
		if uid, ok := metadata["userId"].(string); ok {
			userID = uid
		}
	}
	
	if userID == "" {
		log.Printf("ERROR: could not extract userId from event")
		return fmt.Errorf("missing userId in event")
	}
	
	// Extract keywords - check multiple possible locations
	var keywords []string
	
	// First check if keywords are at the top level
	if keywordsInterface, ok := rawDetail["keywords"]; ok {
		if keywordsArray, ok := keywordsInterface.([]interface{}); ok {
			for _, kw := range keywordsArray {
				if keyword, ok := kw.(string); ok {
					keywords = append(keywords, keyword)
				}
			}
		}
	}
	
	// If not found, check in the data field
	if len(keywords) == 0 {
		if dataField, ok := rawDetail["data"].(map[string]interface{}); ok {
			if keywordsInterface, ok := dataField["keywords"]; ok {
				if keywordsArray, ok := keywordsInterface.([]interface{}); ok {
					for _, kw := range keywordsArray {
						if keyword, ok := kw.(string); ok {
							keywords = append(keywords, keyword)
						}
					}
				}
			}
		}
	}
	
	log.Printf("Processing NodeCreated event for node %s with %d keywords: %v", nodeID, len(keywords), keywords)

	// Find related nodes using keyword-based search
	// For scalability, limit the number of nodes we query
	const maxNodesToQuery = 200
	const maxEdgesToCreate = 100
	const minMatchScore = 0.1 // Minimum relevance score to create an edge

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
		Limit:    maxNodesToQuery,
	}
	
	log.Printf("Searching for related nodes with keywords: %v", keywords)
	relatedNodes, err := repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("ERROR: could not find related nodes: %v", err)
		return err
	}
	
	log.Printf("Found %d potential related nodes for user %s", len(relatedNodes), userID)

	// Score and rank related nodes
	type scoredNode struct {
		nodeID string
		score  float64
	}
	var scoredNodes []scoredNode

	// Create a map of new node's keywords for efficient lookup
	newNodeKeywords := make(map[string]bool)
	for _, kw := range keywords {
		newNodeKeywords[kw] = true
	}

	log.Printf("New node has %d keywords: %v", len(keywords), keywords)

	for _, rn := range relatedNodes {
		if rn.ID().String() == nodeID {
			continue // Skip self
		}

		// Get the related node's keywords
		relatedKeywords := rn.Keywords().ToSlice()
		log.Printf("Comparing with node %s which has %d keywords: %v", rn.ID().String(), len(relatedKeywords), relatedKeywords)
		if len(relatedKeywords) == 0 {
			// If no keywords, skip this node
			continue
		}

		// Count actual keyword matches
		matchCount := 0
		for _, relatedKw := range relatedKeywords {
			if newNodeKeywords[relatedKw] {
				matchCount++
			}
		}

		if matchCount > 0 {
			// Calculate score as percentage of keywords matched
			// Use the maximum of the two keyword sets as denominator for more accurate scoring
			maxKeywords := len(keywords)
			if len(relatedKeywords) > maxKeywords {
				maxKeywords = len(relatedKeywords)
			}
			
			score := float64(matchCount) / float64(maxKeywords)
			
			// Log detailed matching info for debugging
			if matchCount == len(keywords) && len(keywords) == len(relatedKeywords) {
				log.Printf("Found 100%% match! Node %s has identical keywords. Score: %.2f", rn.ID().String(), score)
			} else {
				log.Printf("Node %s: %d/%d keywords matched, score: %.2f", rn.ID().String(), matchCount, maxKeywords, score)
			}
			
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

	// Log scoring results
	log.Printf("Scored %d nodes above threshold %.2f", len(scoredNodes), minMatchScore)
	
	// Limit to top N connections
	if len(scoredNodes) > maxEdgesToCreate {
		log.Printf("Limiting edges from %d to %d (maxEdgesToCreate)", len(scoredNodes), maxEdgesToCreate)
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
		log.Printf("Creating batch of %d edges from node %s to nodes: %v", len(batchNodeIDs), nodeID, batchNodeIDs)
		if err := repo.CreateEdges(ctx, userID, nodeID, batchNodeIDs); err != nil {
			log.Printf("WARNING: failed to create batch of edges: %v", err)
			// Continue with other batches even if one fails
		} else {
			totalCreated += len(batchNodeIDs)
			log.Printf("Successfully created %d edges in this batch", len(batchNodeIDs))
		}
	}

	// Publish EdgesCreated event if we created any edges
	if totalCreated > 0 {
		edgesCreatedDetail, _ := json.Marshal(map[string]interface{}{
			"userId":     userID,
			"nodeId":     nodeID,
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
		nodeID, totalCreated, len(scoredNodes))
	return nil
}

func main() {
	lambda.Start(handler)
}
