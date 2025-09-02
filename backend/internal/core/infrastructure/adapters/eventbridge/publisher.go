// Package eventbridge provides EventBridge implementation of the EventBus port
package eventbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// Publisher implements the ports.EventBus interface using AWS EventBridge
type Publisher struct {
	client  *eventbridge.Client
	busName string
	source  string
	logger  ports.Logger
	metrics ports.Metrics
}

// NewPublisher creates a new EventBridge publisher
func NewPublisher(client *eventbridge.Client, busName, source string, logger ports.Logger, metrics ports.Metrics) *Publisher {
	if source == "" {
		source = "brain2.core"
	}
	return &Publisher{
		client:  client,
		busName: busName,
		source:  source,
		logger:  logger,
		metrics: metrics,
	}
}

// Publish publishes domain events to EventBridge
func (p *Publisher) Publish(ctx context.Context, domainEvents ...events.DomainEvent) error {
	if len(domainEvents) == 0 {
		return nil
	}

	// Convert domain events to EventBridge entries
	entries := make([]types.PutEventsRequestEntry, 0, len(domainEvents))
	
	for _, event := range domainEvents {
		entry, err := p.createEventEntry(event)
		if err != nil {
			p.logger.Error("Failed to create event entry", err,
				ports.Field{Key: "event_type", Value: event.GetEventType()},
				ports.Field{Key: "aggregate_id", Value: event.GetAggregateID()})
			p.metrics.IncrementCounter("eventbridge.publish.failed",
				ports.Tag{Key: "event_type", Value: event.GetEventType()})
			continue
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no valid events to publish")
	}

	// Batch publish events (EventBridge supports up to 10 events per request)
	for i := 0; i < len(entries); i += 10 {
		end := i + 10
		if end > len(entries) {
			end = len(entries)
		}
		
		batch := entries[i:end]
		if err := p.publishBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to publish event batch: %w", err)
		}
	}

	p.metrics.IncrementCounter("eventbridge.publish.success",
		ports.Tag{Key: "count", Value: fmt.Sprintf("%d", len(domainEvents))})
	
	return nil
}

// PublishAsync publishes events asynchronously
func (p *Publisher) PublishAsync(ctx context.Context, domainEvents ...events.DomainEvent) {
	go func() {
		// Create a new context with timeout for async operation
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := p.Publish(asyncCtx, domainEvents...); err != nil {
			p.logger.Error("Failed to publish events asynchronously", err,
				ports.Field{Key: "event_count", Value: len(domainEvents)})
		}
	}()
}

// Subscribe is not implemented for EventBridge as it uses rule-based routing
func (p *Publisher) Subscribe(eventType string, handler ports.EventHandler) error {
	return fmt.Errorf("subscribe not implemented: EventBridge uses rule-based routing configured in AWS")
}

// createEventEntry converts a domain event to an EventBridge entry
func (p *Publisher) createEventEntry(event events.DomainEvent) (types.PutEventsRequestEntry, error) {
	// Prepare event detail
	detail := map[string]interface{}{
		"eventId":      event.GetEventID(),
		"eventType":    event.GetEventType(),
		"aggregateId":  event.GetAggregateID(),
		"aggregateType": event.GetAggregateType(),
		"version":      event.GetVersion(),
		"occurredAt":   event.GetOccurredAt().Format(time.RFC3339),
		"data":         event.GetData(),
	}

	// Add metadata if available
	if metadata := event.GetMetadata(); metadata != nil {
		detail["metadata"] = metadata
	}

	// Marshal to JSON
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal event detail: %w", err)
	}

	// Create EventBridge entry
	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.busName),
		Source:       aws.String(p.source),
		DetailType:   aws.String(event.GetEventType()),
		Detail:       aws.String(string(detailJSON)),
		Time:         aws.Time(event.GetOccurredAt()),
	}

	// Add resource ARNs if available (for filtering)
	if aggregateID := event.GetAggregateID(); aggregateID != "" {
		entry.Resources = []string{
			fmt.Sprintf("arn:aws:brain2:%s:aggregate/%s", p.source, aggregateID),
		}
	}

	return entry, nil
}

// publishBatch publishes a batch of events to EventBridge
func (p *Publisher) publishBatch(ctx context.Context, entries []types.PutEventsRequestEntry) error {
	input := &eventbridge.PutEventsInput{
		Entries: entries,
	}

	result, err := p.client.PutEvents(ctx, input)
	if err != nil {
		p.logger.Error("Failed to put events to EventBridge", err,
			ports.Field{Key: "batch_size", Value: len(entries)})
		return err
	}

	// Check for failed entries
	if result.FailedEntryCount != nil && *result.FailedEntryCount > 0 {
		for i, entry := range result.Entries {
			if entry.ErrorCode != nil {
				p.logger.Error("Event entry failed",
					fmt.Errorf("code: %s, message: %s", *entry.ErrorCode, *entry.ErrorMessage),
					ports.Field{Key: "entry_index", Value: i})
			}
		}
		return fmt.Errorf("%d events failed to publish", *result.FailedEntryCount)
	}

	p.logger.Debug("Events published successfully",
		ports.Field{Key: "count", Value: len(entries)})
	
	return nil
}

// Close closes the publisher (no-op for EventBridge)
func (p *Publisher) Close() error {
	p.logger.Info("EventBridge publisher closed")
	return nil
}