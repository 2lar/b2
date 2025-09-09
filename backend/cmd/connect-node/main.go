// Package main implements the Lambda handler for node connection discovery.
// This handler finds and establishes connections between nodes based on various criteria.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"backend/application/services"
	"backend/domain/events"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"go.uber.org/zap"
)

// Global dependencies for Lambda performance optimization
var (
	edgeService *services.EdgeService
	logger      *zap.Logger
)

func init() {
	// Initialize dependencies using Wire
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	container, err := di.InitializeContainer(context.Background(), cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependency container: %v", err)
	}

	// Initialize EdgeService with dependencies from container
	logger = container.Logger
	edgeService = services.NewEdgeService(
		container.NodeRepo,
		container.GraphRepo,
		container.EdgeRepo,
		&cfg.EdgeCreation,
		logger,
	)

	log.Println("Connect-node handler initialized successfully")
}

// ConnectionRequest represents the input for connection discovery
type ConnectionRequest struct {
	NodeID              string   `json:"node_id"`
	UserID              string   `json:"user_id"`
	GraphID             string   `json:"graph_id"`
	Title               string   `json:"title,omitempty"`
	Content             string   `json:"content,omitempty"`
	MaxConnections      int      `json:"max_connections,omitempty"`
	ConnectionType      string   `json:"connection_type,omitempty"` // semantic, keyword, temporal
	SimilarityThreshold float64  `json:"similarity_threshold,omitempty"`
	Keywords            []string `json:"keywords,omitempty"`
	Tags                []string `json:"tags,omitempty"`
}

// ConnectionResponse represents the discovered connections
type ConnectionResponse struct {
	NodeID      string                 `json:"node_id"`
	Connections []DiscoveredConnection `json:"connections"`
	TotalFound  int                    `json:"total_found"`
	Applied     int                    `json:"applied"`
}

// DiscoveredConnection represents a potential or established connection
type DiscoveredConnection struct {
	TargetNodeID string  `json:"target_node_id"`
	Confidence   float64 `json:"confidence"`
	Type         string  `json:"type"`
	Reason       string  `json:"reason"`
	Created      bool    `json:"created"`
}

// HandleConnectionDiscovery processes connection discovery requests
func HandleConnectionDiscovery(ctx context.Context, request ConnectionRequest) (*ConnectionResponse, error) {
	log.Printf("Discovering connections for node %s", request.NodeID)

	// Use the EdgeService to create edges for the new node
	createdEdgeIDs, err := edgeService.CreateEdgesForNewNode(
		ctx,
		request.NodeID,
		request.UserID,
		request.GraphID,
		request.Keywords,
		request.Tags,
	)

	if err != nil {
		log.Printf("Failed to create edges for node %s: %v", request.NodeID, err)
		return &ConnectionResponse{
			NodeID:      request.NodeID,
			Connections: []DiscoveredConnection{},
			TotalFound:  0,
			Applied:     0,
		}, err
	}

	// Build response with created edges
	connections := make([]DiscoveredConnection, 0, len(createdEdgeIDs))
	for range createdEdgeIDs {
		// For simplicity, we're not fetching the full edge details here
		// In production, you might want to fetch and include more details
		connections = append(connections, DiscoveredConnection{
			Created: true,
			Type:    "similar",
			Reason:  "Semantic similarity based on keywords and tags",
		})
	}

	response := &ConnectionResponse{
		NodeID:      request.NodeID,
		Connections: connections,
		TotalFound:  len(connections),
		Applied:     len(connections),
	}

	log.Printf("Created %d edges for node %s", len(createdEdgeIDs), request.NodeID)

	return response, nil
}

// handler is the main Lambda handler for different invocation types
func handler(ctx context.Context, event json.RawMessage) error {
	log.Printf("Received event: %s", string(event))

	// Try to parse as API Gateway event (direct invocation)
	var apiEvent awsevents.APIGatewayProxyRequest
	if err := json.Unmarshal(event, &apiEvent); err == nil && apiEvent.Body != "" {
		var request ConnectionRequest
		if err := json.Unmarshal([]byte(apiEvent.Body), &request); err != nil {
			log.Printf("Failed to parse request body: %v", err)
			return err
		}

		response, err := HandleConnectionDiscovery(ctx, request)
		if err != nil {
			return err
		}

		// For API Gateway, we'd return the response
		// but Lambda handles this differently
		responseJSON, _ := json.Marshal(response)
		log.Printf("Response: %s", responseJSON)
		return nil
	}

	// Try to parse as EventBridge event (async invocation)
	var cloudWatchEvent awsevents.CloudWatchEvent
	if err := json.Unmarshal(event, &cloudWatchEvent); err == nil {
		// Handle enhanced event with async edge candidates
		if cloudWatchEvent.DetailType == events.TypeNodeCreatedWithPending {
			var enhancedEvent struct {
				NodeID           string                 `json:"nodeId"`
				UserID           string                 `json:"userId"`
				GraphID          string                 `json:"graphId"`
				Title            string                 `json:"title"`
				Keywords         []string               `json:"keywords"`
				Tags             []string               `json:"tags"`
				SyncEdgesCreated int                    `json:"syncEdgesCreated"`
				AsyncCandidates  []events.EdgeCandidate `json:"asyncCandidates"`
			}

			if err := json.Unmarshal(cloudWatchEvent.Detail, &enhancedEvent); err != nil {
				return fmt.Errorf("failed to parse enhanced NodeCreated event: %w", err)
			}

			log.Printf("Processing async edge candidates for node %s (sync edges created: %d, async pending: %d)",
				enhancedEvent.NodeID, enhancedEvent.SyncEdgesCreated, len(enhancedEvent.AsyncCandidates))

			// Process the async edge candidates
			// Since edges were already discovered in the main process,
			// we just need to create them based on the candidates
			for _, candidate := range enhancedEvent.AsyncCandidates {
				_, err := edgeService.CreateEdge(
					ctx,
					candidate.SourceID,
					candidate.TargetID,
					enhancedEvent.GraphID,
					candidate.Type,
					candidate.Similarity,
				)
				if err != nil {
					log.Printf("Failed to create async edge from %s to %s: %v",
						candidate.SourceID, candidate.TargetID, err)
					// Continue with other edges even if one fails
				}
			}

			log.Printf("Completed processing async edges for node %s", enhancedEvent.NodeID)
			return nil
		}

		// Handle regular node.created event (backward compatibility)
		if cloudWatchEvent.DetailType == events.TypeNodeCreated {
			// Auto-discover connections for newly created nodes
			var nodeCreatedEvent struct {
				NodeID   string   `json:"node_id"`
				UserID   string   `json:"user_id"`
				GraphID  string   `json:"graph_id"`
				Title    string   `json:"title"`
				Content  string   `json:"content"`
				Keywords []string `json:"keywords"`
				Tags     []string `json:"tags"`
			}

			if err := json.Unmarshal(cloudWatchEvent.Detail, &nodeCreatedEvent); err != nil {
				return fmt.Errorf("failed to parse NodeCreated event: %w", err)
			}

			request := ConnectionRequest{
				NodeID:              nodeCreatedEvent.NodeID,
				UserID:              nodeCreatedEvent.UserID,
				GraphID:             nodeCreatedEvent.GraphID,
				Title:               nodeCreatedEvent.Title,
				Content:             nodeCreatedEvent.Content,
				Keywords:            nodeCreatedEvent.Keywords,
				Tags:                nodeCreatedEvent.Tags,
				MaxConnections:      10, // Allow more connections for automatic discovery
				ConnectionType:      "semantic",
				SimilarityThreshold: 0.3, // Lower threshold for automatic connections
			}

			_, err := HandleConnectionDiscovery(ctx, request)
			return err
		}
	}

	// Try to parse as direct invocation
	var request ConnectionRequest
	if err := json.Unmarshal(event, &request); err == nil {
		_, err := HandleConnectionDiscovery(ctx, request)
		return err
	}

	return fmt.Errorf("unable to parse event")
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting connect-node Lambda")
		lambda.Start(handler)
	} else {
		// Local testing mode
		log.Println("Running in local test mode")

		// Create a test request
		testRequest := ConnectionRequest{
			NodeID:         "test-node-123",
			UserID:         "test-user-456",
			MaxConnections: 3,
			ConnectionType: "semantic",
			Keywords:       []string{"test", "example"},
		}

		// Process the test request
		response, err := HandleConnectionDiscovery(context.Background(), testRequest)
		if err != nil {
			log.Fatalf("Test request processing failed: %v", err)
		}

		responseJSON, _ := json.MarshalIndent(response, "", "  ")
		log.Printf("Test response:\n%s", responseJSON)
	}
}
