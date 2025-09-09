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
	"backend/domain/events"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
)

// Global command bus for Lambda performance optimization
var commandBus *bus.CommandBus

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

	log.Printf("Node %s deleted by user %s, performing cleanup",
		nodeEvent.AggregateID, nodeEvent.UserID)

	// Create cleanup command
	cleanupCmd := &commands.CleanupNodeResourcesCommand{
		NodeID:   nodeEvent.AggregateID,
		UserID:   nodeEvent.UserID,
		Keywords: nodeEvent.Keywords,
		Tags:     nodeEvent.Tags,
	}

	// Execute cleanup through command bus
	if err := commandBus.Send(ctx, cleanupCmd); err != nil {
		return fmt.Errorf("cleanup command failed: %w", err)
	}

	// Additional cleanup tasks can be added here
	// For example:
	// - Remove from search index
	// - Clear cache entries
	// - Update analytics
	// - Notify connected clients via WebSocket

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
