package eventbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/domain/events"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"go.uber.org/zap"
)

// EventBridgePublisher implements the EventBus interface using AWS EventBridge
type EventBridgePublisher struct {
	client       *eventbridge.Client
	eventBusName string
	source       string
	logger       *zap.Logger
}

// NewEventBridgePublisher creates a new EventBridge publisher
func NewEventBridgePublisher(
	client *eventbridge.Client,
	eventBusName string,
	logger *zap.Logger,
) ports.EventBus {
	return &EventBridgePublisher{
		client:       client,
		eventBusName: eventBusName,
		source:       events.SourceBackend, // Use constant from events package
		logger:       logger,
	}
}

// Publish sends a single event to EventBridge
func (p *EventBridgePublisher) Publish(ctx context.Context, event events.DomainEvent) error {
	return p.PublishBatch(ctx, []events.DomainEvent{event})
}

// PublishBatch sends multiple events to EventBridge
func (p *EventBridgePublisher) PublishBatch(ctx context.Context, domainEvents []events.DomainEvent) error {
	if len(domainEvents) == 0 {
		return nil
	}

	// EventBridge limits to 10 events per PutEvents call
	const batchSize = 10

	for i := 0; i < len(domainEvents); i += batchSize {
		end := i + batchSize
		if end > len(domainEvents) {
			end = len(domainEvents)
		}

		batch := domainEvents[i:end]
		if err := p.publishBatch(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

// publishBatch publishes a batch of events (max 10)
func (p *EventBridgePublisher) publishBatch(ctx context.Context, domainEvents []events.DomainEvent) error {
	entries := make([]types.PutEventsRequestEntry, 0, len(domainEvents))

	for _, event := range domainEvents {
		// Serialize event to JSON
		eventData, err := json.Marshal(event)
		if err != nil {
			p.logger.Error("Failed to marshal event",
				zap.Error(err),
				zap.String("eventType", event.GetEventType()),
			)
			continue
		}

		// Create EventBridge entry
		entry := types.PutEventsRequestEntry{
			EventBusName: aws.String(p.eventBusName),
			Source:       aws.String(p.source),
			DetailType:   aws.String(event.GetEventType()),
			Detail:       aws.String(string(eventData)),
			Time:         aws.Time(event.GetTimestamp()),
			Resources: []string{
				fmt.Sprintf("arn:aws:brain2::%s", event.GetAggregateID()),
			},
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil
	}

	// Send events to EventBridge
	input := &eventbridge.PutEventsInput{
		Entries: entries,
	}

	result, err := p.client.PutEvents(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to publish events to EventBridge: %w", err)
	}

	// Check for failures
	if result.FailedEntryCount > 0 {
		for i, entry := range result.Entries {
			if entry.ErrorCode != nil {
				p.logger.Error("Failed to publish event",
					zap.String("eventType", domainEvents[i].GetEventType()),
					zap.String("errorCode", *entry.ErrorCode),
					zap.String("errorMessage", aws.ToString(entry.ErrorMessage)),
				)
			}
		}
		return fmt.Errorf("%d events failed to publish", result.FailedEntryCount)
	}

	p.logger.Debug("Events published to EventBridge",
		zap.Int("count", len(entries)),
		zap.String("eventBus", p.eventBusName),
	)

	return nil
}

// Subscribe registers a handler for an event type
// Note: In EventBridge, subscriptions are handled through Rules and Targets,
// not directly in the publisher. This would typically be configured via
// infrastructure as code (CloudFormation/Terraform) or AWS Console.
func (p *EventBridgePublisher) Subscribe(eventType string, handler ports.EventHandler) error {
	// EventBridge subscriptions are managed externally
	// This method is here to satisfy the interface but would typically
	// configure Lambda functions or other targets via AWS APIs
	p.logger.Warn("Subscribe called but EventBridge subscriptions are managed externally",
		zap.String("eventType", eventType),
	)
	return nil
}

// Unsubscribe removes a handler
func (p *EventBridgePublisher) Unsubscribe(eventType string, handler ports.EventHandler) error {
	// EventBridge subscriptions are managed externally
	p.logger.Warn("Unsubscribe called but EventBridge subscriptions are managed externally",
		zap.String("eventType", eventType),
	)
	return nil
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

// publishWithRetry publishes events with exponential backoff retry
func (p *EventBridgePublisher) publishWithRetry(ctx context.Context, domainEvents []events.DomainEvent) error {
	const maxRetries = 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := p.publishBatch(ctx, domainEvents)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		if attempt < maxRetries-1 {
			p.logger.Warn("Retrying event publication",
				zap.Int("attempt", attempt+1),
				zap.Error(err),
				zap.Duration("backoff", backoff),
			)

			select {
			case <-time.After(backoff):
				backoff *= 2
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("failed to publish events after %d attempts", maxRetries)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	// In production, check for specific AWS error codes
	// that indicate transient failures
	return true // Simplified for this implementation
}
