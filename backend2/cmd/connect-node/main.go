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
	"github.com/google/uuid"
	
	"backend2/application/commands"
	commandbus "backend2/application/commands/bus"
	"backend2/application/ports"
	querybus "backend2/application/queries/bus"
	"backend2/infrastructure/config"
	"backend2/infrastructure/di"
)

// Global dependencies for Lambda performance optimization
var (
	commandBus  *commandbus.CommandBus
	queryBus    *querybus.QueryBus
	publisher   ports.EventBus
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
	
	commandBus = container.CommandBus
	queryBus = container.QueryBus
	publisher = container.EventBus
	
	log.Println("Connect-node handler initialized successfully")
}

// ConnectionRequest represents the input for connection discovery
type ConnectionRequest struct {
	NodeID            string   `json:"node_id"`
	UserID            string   `json:"user_id"`
	MaxConnections    int      `json:"max_connections,omitempty"`
	ConnectionType    string   `json:"connection_type,omitempty"`    // semantic, keyword, temporal
	SimilarityThreshold float64 `json:"similarity_threshold,omitempty"`
	Keywords          []string `json:"keywords,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}

// ConnectionResponse represents the discovered connections
type ConnectionResponse struct {
	NodeID      string                  `json:"node_id"`
	Connections []DiscoveredConnection  `json:"connections"`
	TotalFound  int                     `json:"total_found"`
	Applied     int                     `json:"applied"`
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
	
	// Set defaults
	if request.MaxConnections == 0 {
		request.MaxConnections = 10
	}
	if request.SimilarityThreshold == 0 {
		request.SimilarityThreshold = 0.7
	}
	if request.ConnectionType == "" {
		request.ConnectionType = "semantic"
	}
	
	// Query for similar nodes based on connection type
	var discoveries []DiscoveredConnection
	
	switch request.ConnectionType {
	case "semantic":
		discoveries = discoverSemanticConnections(ctx, request)
	case "keyword":
		discoveries = discoverKeywordConnections(ctx, request)
	case "temporal":
		discoveries = discoverTemporalConnections(ctx, request)
	default:
		return nil, fmt.Errorf("unknown connection type: %s", request.ConnectionType)
	}
	
	// Apply connections (create edges)
	applied := 0
	for i, discovery := range discoveries {
		if i >= request.MaxConnections {
			break
		}
		
		if discovery.Confidence >= request.SimilarityThreshold {
			// Create edge command
			createEdgeCmd := commands.CreateEdgeCommand{
				EdgeID:   uuid.New().String(),
				UserID:   request.UserID,
				SourceID: request.NodeID,
				TargetID: discovery.TargetNodeID,
				Type:     discovery.Type,
				Weight:   discovery.Confidence,
				Metadata: map[string]interface{}{
					"reason":       discovery.Reason,
					"auto_created": true,
				},
			}
			
			// Execute command
			if err := commandBus.Send(ctx, createEdgeCmd); err != nil {
				log.Printf("Failed to create edge to %s: %v", discovery.TargetNodeID, err)
				discoveries[i].Created = false
			} else {
				discoveries[i].Created = true
				applied++
				
				// Publish EdgeCreatedEvent
				// This would trigger WebSocket notifications and other handlers
			}
		}
	}
	
	response := &ConnectionResponse{
		NodeID:      request.NodeID,
		Connections: discoveries,
		TotalFound:  len(discoveries),
		Applied:     applied,
	}
	
	log.Printf("Found %d connections, applied %d for node %s", 
		len(discoveries), applied, request.NodeID)
	
	return response, nil
}

// discoverSemanticConnections finds semantically similar nodes
func discoverSemanticConnections(ctx context.Context, request ConnectionRequest) []DiscoveredConnection {
	// Simplified implementation - would use query bus in production
	// query := &queries.FindSimilarNodesQuery{
	//     NodeID:         request.NodeID,
	//     UserID:         request.UserID,
	//     MaxResults:     request.MaxConnections * 2, // Get extra for filtering
	//     SimilarityType: "semantic",
	// }
	// result, err := queryBus.Query(ctx, query)
	// if err != nil {
	//     log.Printf("Failed to find similar nodes: %v", err)
	//     return nil
	// }
	
	// For now, return empty discoveries
	var discoveries []DiscoveredConnection
	
	return discoveries
}

// discoverKeywordConnections finds nodes with matching keywords
func discoverKeywordConnections(ctx context.Context, request ConnectionRequest) []DiscoveredConnection {
	// Implementation for keyword-based discovery
	// Would query nodes with overlapping keywords
	return []DiscoveredConnection{}
}

// discoverTemporalConnections finds nodes created around the same time
func discoverTemporalConnections(ctx context.Context, request ConnectionRequest) []DiscoveredConnection {
	// Implementation for temporal-based discovery
	// Would query nodes created in similar time windows
	return []DiscoveredConnection{}
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
		if cloudWatchEvent.DetailType == "NodeCreated" {
			// Auto-discover connections for newly created nodes
			var nodeCreatedEvent struct {
				NodeID string   `json:"aggregate_id"`
				UserID string   `json:"user_id"`
				Keywords []string `json:"keywords"`
				Tags []string `json:"tags"`
			}
			
			if err := json.Unmarshal(cloudWatchEvent.Detail, &nodeCreatedEvent); err != nil {
				return fmt.Errorf("failed to parse NodeCreated event: %w", err)
			}
			
			request := ConnectionRequest{
				NodeID:         nodeCreatedEvent.NodeID,
				UserID:         nodeCreatedEvent.UserID,
				Keywords:       nodeCreatedEvent.Keywords,
				Tags:           nodeCreatedEvent.Tags,
				MaxConnections: 5,
				ConnectionType: "semantic",
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