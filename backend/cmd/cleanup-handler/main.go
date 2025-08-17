// Package main implements the Lambda handler for async cleanup of node-related resources.
// This handler is triggered by EventBridge when a NodeDeletedEvent is published.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/di"
)

// NodeDeletedDetail represents the structure of a NodeDeleted event from EventBridge
type NodeDeletedDetail struct {
	EventType    string `json:"event_type"`
	AggregateID  string `json:"aggregate_id"`  // Node ID
	UserID       string `json:"user_id"`
	Content      string `json:"content"`
	Keywords     []string `json:"keywords"`
	Tags         []string `json:"tags"`
	Version      int    `json:"version"`
	OccurredAt   string `json:"occurred_at"`
}

var (
	cleanupService *services.CleanupService
	container      *di.Container
)

func init() {
	// Initialize DI container
	var err error
	container, err = di.InitializeContainer()
	if err != nil {
		log.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Validate all dependencies are properly initialized
	if err := container.Validate(); err != nil {
		log.Fatalf("Container validation failed: %v", err)
	}

	// Get cleanup service from container
	cleanupService = container.GetCleanupService()
	if cleanupService == nil {
		log.Fatal("Failed to initialize cleanup service")
	}

	log.Println("Cleanup handler initialized successfully")
}

// HandleRequest processes NodeDeleted events from EventBridge
func HandleRequest(ctx context.Context, event events.CloudWatchEvent) error {
	log.Printf("Processing event: ID=%s, DetailType=%s", event.ID, event.DetailType)
	
	// Log the raw event detail for debugging
	log.Printf("DEBUG: Raw event detail: %s", string(event.Detail))

	// Only process NodeDeleted events
	if event.DetailType != "NodeDeleted" {
		log.Printf("Skipping non-NodeDeleted event: %s", event.DetailType)
		return nil
	}

	// Parse the event detail
	var detail NodeDeletedDetail
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		log.Printf("ERROR: Failed to unmarshal event detail: %v", err)
		log.Printf("ERROR: Event detail was: %s", string(event.Detail))
		return fmt.Errorf("failed to unmarshal event detail: %w", err)
	}
	
	// Validate required fields
	if detail.AggregateID == "" || detail.UserID == "" {
		log.Printf("ERROR: Missing required fields - AggregateID: '%s', UserID: '%s'", detail.AggregateID, detail.UserID)
		log.Printf("ERROR: Full event detail struct: %+v", detail)
		return fmt.Errorf("missing required fields: aggregateID=%s, userID=%s", detail.AggregateID, detail.UserID)
	}

	log.Printf("Processing cleanup for node: NodeID=%s, UserID=%s", detail.AggregateID, detail.UserID)

	// Perform async cleanup
	if err := cleanupService.CleanupNodeResiduals(ctx, detail.UserID, detail.AggregateID); err != nil {
		// Log the error but don't fail the Lambda - let it go to DLQ if retries fail
		log.Printf("ERROR: Failed to cleanup node residuals: NodeID=%s, UserID=%s, Error=%v", 
			detail.AggregateID, detail.UserID, err)
		return fmt.Errorf("cleanup failed for node %s: %w", detail.AggregateID, err)
	}

	log.Printf("Successfully cleaned up residuals for node: %s", detail.AggregateID)
	
	// Also cleanup any orphaned idempotency records for this user's node operations
	// This is a best-effort operation, so we don't fail if it errors
	if err := cleanupService.CleanupIdempotencyRecords(ctx, detail.UserID, detail.AggregateID); err != nil {
		log.Printf("WARNING: Failed to cleanup idempotency records: %v", err)
		// Don't return error - this is optional cleanup
	}

	return nil
}

func main() {
	lambda.Start(HandleRequest)
}