// Package main implements the Lambda handler for async cleanup of node-related resources.
// This handler is triggered by EventBridge when a NodeDeletedEvent is published.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"backend/application/commands"
	"backend/application/commands/bus"
	"backend/application/ports"
	"backend/domain/events"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
)

// Global dependencies for Lambda performance optimization
var (
	commandBus *bus.CommandBus
	edgeRepo   ports.EdgeRepository
	eventStore ports.EventStore
	graphRepo  ports.GraphRepository
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
	edgeRepo = container.EdgeRepo
	eventStore = container.EventStore
	graphRepo = container.GraphRepo

	log.Println("Cleanup handler initialized successfully")
}

// HandleNodeDeleted processes the NodeDeletedEvent from EventBridge
func HandleNodeDeleted(ctx context.Context, event awsevents.CloudWatchEvent) error {
	log.Printf("Processing NodeDeletedEvent: %s", event.ID)

	// Parse the event detail
	var nodeEvent events.NodeDeletedEvent
	if err := json.Unmarshal(event.Detail, &nodeEvent); err != nil {
		return fmt.Errorf("failed to unmarshal event detail: %w", err)
	}

	log.Printf("Node %s deleted by user %s, performing async cleanup",
		nodeEvent.AggregateID, nodeEvent.UserID)

	// 1. Delete all edges connected to this node
	if nodeEvent.GraphID != "" {
		if err := edgeRepo.DeleteByNodeID(ctx, nodeEvent.GraphID, nodeEvent.AggregateID); err != nil {
			log.Printf("Failed to delete edges for node %s: %v", nodeEvent.AggregateID, err)
			// Continue with other cleanup tasks even if edge deletion fails
		} else {
			log.Printf("Successfully deleted edges for node %s", nodeEvent.AggregateID)
		}
	}

	// 2. Delete all events for this node from event store
	if err := eventStore.DeleteEvents(ctx, nodeEvent.AggregateID); err != nil {
		log.Printf("Failed to delete events for node %s: %v", nodeEvent.AggregateID, err)
		// Continue with other cleanup tasks
	} else {
		log.Printf("Successfully deleted events for node %s", nodeEvent.AggregateID)
	}

	// 3. Update graph metadata to reflect the new node/edge counts
	if nodeEvent.GraphID != "" {
		if err := graphRepo.UpdateGraphMetadata(ctx, nodeEvent.GraphID); err != nil {
			log.Printf("Failed to update graph metadata for graph %s: %v", nodeEvent.GraphID, err)
			// Non-critical error, don't fail the operation
		} else {
			log.Printf("Successfully updated graph metadata for graph %s", nodeEvent.GraphID)
		}
	}

	// 4. Execute any additional cleanup commands (search index, cache, etc.)
	cleanupCmd := &commands.CleanupNodeResourcesCommand{
		NodeID:   nodeEvent.AggregateID,
		UserID:   nodeEvent.UserID,
		Keywords: nodeEvent.Keywords,
		Tags:     nodeEvent.Tags,
	}

	if err := commandBus.Send(ctx, cleanupCmd); err != nil {
		log.Printf("Additional cleanup command failed: %v", err)
		// Non-critical, continue
	}

	log.Printf("Successfully cleaned up resources for node %s", nodeEvent.AggregateID)
	return nil
}

// HandleEdgeDeleted processes the EdgeDeletedEvent from EventBridge
func HandleEdgeDeleted(ctx context.Context, event awsevents.CloudWatchEvent) error {
	log.Printf("Processing EdgeDeletedEvent: %s", event.ID)

	var edgeEvent events.EdgeDeletedEvent
	if err := json.Unmarshal(event.Detail, &edgeEvent); err != nil {
		return fmt.Errorf("failed to unmarshal event detail: %w", err)
	}

	log.Printf("Edge %s deleted, performing cleanup", edgeEvent.AggregateID)

	// Create cleanup command for edge
	cleanupCmd := &commands.CleanupEdgeResourcesCommand{
		EdgeID:   edgeEvent.AggregateID,
		SourceID: edgeEvent.SourceNodeID.String(),
		TargetID: edgeEvent.TargetNodeID.String(),
		UserID:   edgeEvent.UserID,
	}

	// Execute cleanup
	if err := commandBus.Send(ctx, cleanupCmd); err != nil {
		return fmt.Errorf("edge cleanup command failed: %w", err)
	}

	log.Printf("Successfully cleaned up resources for edge %s", edgeEvent.AggregateID)
	return nil
}

// handler is the main Lambda handler that routes events based on detail-type
func handler(ctx context.Context, event awsevents.CloudWatchEvent) error {
	log.Printf("Received event: %s (detail-type: %s)", event.ID, event.DetailType)

	// Route based on event type
	switch event.DetailType {
	case "NodeDeleted":
		return HandleNodeDeleted(ctx, event)
	case "EdgeDeleted":
		return HandleEdgeDeleted(ctx, event)
	default:
		log.Printf("Unhandled event type: %s", event.DetailType)
		// Return nil to acknowledge the event even if we don't process it
		// This prevents event retry loops
		return nil
	}
}

func main() {
	// Check if running in Lambda environment
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting cleanup handler Lambda")
		lambda.Start(handler)
	} else {
		// Local testing mode
		log.Println("Running in local test mode")

		// Create a test event
		testEvent := awsevents.CloudWatchEvent{
			ID:         "test-event-1",
			DetailType: "NodeDeleted",
			Detail: json.RawMessage(`{
				"event_type": "NodeDeleted",
				"aggregate_id": "test-node-123",
				"user_id": "test-user-456",
				"graph_id": "test-graph-789",
				"content": "Test content",
				"keywords": ["test", "example"],
				"tags": ["cleanup", "test"],
				"version": 1,
				"occurred_at": "2024-01-01T00:00:00Z"
			}`),
		}

		// Process the test event
		if err := handler(context.Background(), testEvent); err != nil {
			log.Fatalf("Test event processing failed: %v", err)
		}

		log.Println("Test event processed successfully")
	}
}
