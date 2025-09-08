package dynamodb

import (
	"context"
	"fmt"
	"time"

	"backend2/application/ports"
	
	"go.uber.org/zap"
)

// OutboxProcessor handles the background processing of unpublished events
// using the Outbox pattern to ensure eventual consistency
type OutboxProcessor struct {
	eventStore     *DynamoDBEventStore
	eventPublisher ports.EventPublisher
	logger         *zap.Logger
	
	// Configuration
	batchSize      int32
	processingInterval time.Duration
	maxRetries     int
	
	// Control channels
	stopChan       chan struct{}
	stoppedChan    chan struct{}
}

// NewOutboxProcessor creates a new outbox processor
func NewOutboxProcessor(
	eventStore *DynamoDBEventStore,
	eventPublisher ports.EventPublisher,
	logger *zap.Logger,
) *OutboxProcessor {
	return &OutboxProcessor{
		eventStore:         eventStore,
		eventPublisher:     eventPublisher,
		logger:            logger,
		batchSize:         50, // Process 50 events at a time
		processingInterval: 5 * time.Second, // Process every 5 seconds
		maxRetries:        3, // Maximum 3 attempts per event
		stopChan:          make(chan struct{}),
		stoppedChan:       make(chan struct{}),
	}
}

// Start begins the background processing of outbox events
func (op *OutboxProcessor) Start(ctx context.Context) {
	op.logger.Info("Starting outbox processor",
		zap.Int32("batchSize", op.batchSize),
		zap.Duration("interval", op.processingInterval),
	)
	
	go op.processLoop(ctx)
}

// Stop gracefully stops the outbox processor
func (op *OutboxProcessor) Stop() {
	op.logger.Info("Stopping outbox processor")
	close(op.stopChan)
	<-op.stoppedChan
	op.logger.Info("Outbox processor stopped")
}

// processLoop is the main processing loop
func (op *OutboxProcessor) processLoop(ctx context.Context) {
	defer close(op.stoppedChan)
	
	ticker := time.NewTicker(op.processingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			op.logger.Info("Context cancelled, stopping outbox processor")
			return
		case <-op.stopChan:
			op.logger.Info("Stop signal received")
			return
		case <-ticker.C:
			if err := op.processBatch(ctx); err != nil {
				op.logger.Error("Error processing outbox batch", zap.Error(err))
			}
		}
	}
}

// processBatch processes a batch of pending events
func (op *OutboxProcessor) processBatch(ctx context.Context) error {
	// Get pending events from the outbox
	pendingEvents, err := op.eventStore.GetPendingEvents(ctx, op.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending events: %w", err)
	}
	
	if len(pendingEvents) == 0 {
		return nil // No events to process
	}
	
	op.logger.Debug("Processing outbox batch",
		zap.Int("eventCount", len(pendingEvents)),
	)
	
	successCount := 0
	failureCount := 0
	
	for _, eventRecord := range pendingEvents {
		if err := op.processEvent(ctx, eventRecord); err != nil {
			op.logger.Error("Failed to process event",
				zap.String("eventID", eventRecord.EventID),
				zap.String("eventType", eventRecord.EventType),
				zap.Error(err),
			)
			failureCount++
		} else {
			successCount++
		}
	}
	
	op.logger.Debug("Completed outbox batch processing",
		zap.Int("successCount", successCount),
		zap.Int("failureCount", failureCount),
	)
	
	return nil
}

// processEvent processes a single event from the outbox
func (op *OutboxProcessor) processEvent(ctx context.Context, eventRecord *EventRecord) error {
	// Convert the event record back to a domain event
	domainEvent, err := op.eventStore.recordToEvent(*eventRecord)
	if err != nil {
		// Mark as failed - malformed events can't be processed
		return op.markEventFailed(ctx, eventRecord, fmt.Sprintf("Failed to convert to domain event: %v", err))
	}
	
	// Try to publish the event
	if err := op.eventPublisher.Publish(ctx, domainEvent); err != nil {
		// Mark as failed or retry
		return op.markEventFailed(ctx, eventRecord, fmt.Sprintf("Publish failed: %v", err))
	}
	
	// Mark as successfully published
	return op.markEventPublished(ctx, eventRecord)
}

// markEventPublished marks an event as successfully published
func (op *OutboxProcessor) markEventPublished(ctx context.Context, eventRecord *EventRecord) error {
	err := op.eventStore.MarkEventAsPublished(ctx, eventRecord.PK, eventRecord.SK)
	if err != nil {
		op.logger.Error("Failed to mark event as published",
			zap.String("eventID", eventRecord.EventID),
			zap.Error(err),
		)
		return err
	}
	
	op.logger.Debug("Event published successfully",
		zap.String("eventID", eventRecord.EventID),
		zap.String("eventType", eventRecord.EventType),
	)
	
	return nil
}

// markEventFailed marks an event as failed with appropriate retry logic
func (op *OutboxProcessor) markEventFailed(ctx context.Context, eventRecord *EventRecord, errorMsg string) error {
	newAttempts := eventRecord.PublishAttempts + 1
	
	err := op.eventStore.MarkEventAsFailed(ctx, eventRecord.PK, eventRecord.SK, errorMsg, newAttempts)
	if err != nil {
		op.logger.Error("Failed to mark event as failed",
			zap.String("eventID", eventRecord.EventID),
			zap.Error(err),
		)
		return err
	}
	
	if newAttempts >= op.maxRetries {
		op.logger.Warn("Event permanently failed after max retries",
			zap.String("eventID", eventRecord.EventID),
			zap.String("eventType", eventRecord.EventType),
			zap.Int("attempts", newAttempts),
			zap.String("error", errorMsg),
		)
	} else {
		op.logger.Debug("Event marked for retry",
			zap.String("eventID", eventRecord.EventID),
			zap.String("eventType", eventRecord.EventType),
			zap.Int("attempts", newAttempts),
			zap.String("error", errorMsg),
		)
	}
	
	return fmt.Errorf("event processing failed: %s", errorMsg)
}

// GetStats returns processing statistics
func (op *OutboxProcessor) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// This could be enhanced to track more detailed statistics
	pendingEvents, err := op.eventStore.GetPendingEvents(ctx, 1)
	if err != nil {
		return nil, err
	}
	
	hasPendingEvents := len(pendingEvents) > 0
	
	return map[string]interface{}{
		"hasPendingEvents": hasPendingEvents,
		"batchSize":        op.batchSize,
		"processingInterval": op.processingInterval.String(),
		"maxRetries":       op.maxRetries,
	}, nil
}