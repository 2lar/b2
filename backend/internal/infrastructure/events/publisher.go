// Package events provides event publishing infrastructure for the Brain2 backend.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// EventBridgePublisher implements EventPublisher using AWS EventBridge
type EventBridgePublisher struct {
	client    *eventbridge.Client
	eventBus  string
	source    string
	batchSize int
}

// NewEventBridgePublisher creates a new EventBridge publisher
func NewEventBridgePublisher(client *eventbridge.Client, eventBus, source string) repository.EventPublisher {
	if eventBus == "" {
		eventBus = "default"
	}
	if source == "" {
		source = "brain2-backend"
	}
	
	return &EventBridgePublisher{
		client:    client,
		eventBus:  eventBus,
		source:    source,
		batchSize: 10, // EventBridge has a limit of 10 entries per PutEvents call
	}
}

// Publish publishes domain events to EventBridge
func (p *EventBridgePublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	
	// Process events in batches
	for i := 0; i < len(events); i += p.batchSize {
		end := i + p.batchSize
		if end > len(events) {
			end = len(events)
		}
		
		batch := events[i:end]
		if err := p.publishBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to publish event batch: %w", err)
		}
	}
	
	return nil
}

// publishBatch publishes a batch of events to EventBridge
func (p *EventBridgePublisher) publishBatch(ctx context.Context, events []domain.DomainEvent) error {
	entries := make([]types.PutEventsRequestEntry, 0, len(events))
	
	for _, event := range events {
		entry, err := p.createEventEntry(event)
		if err != nil {
			return fmt.Errorf("failed to create event entry: %w", err)
		}
		entries = append(entries, entry)
	}
	
	output, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: entries,
	})
	
	if err != nil {
		return fmt.Errorf("failed to put events: %w", err)
	}
	
	// Check for failed entries
	if output.FailedEntryCount > 0 {
		return fmt.Errorf("%d events failed to publish", output.FailedEntryCount)
	}
	
	return nil
}

// createEventEntry creates an EventBridge entry from a domain event
func (p *EventBridgePublisher) createEventEntry(event domain.DomainEvent) (types.PutEventsRequestEntry, error) {
	// Marshal event data to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Extract metadata
	metadata := map[string]interface{}{
		"eventId":      event.EventID(),
		"aggregateId":  event.AggregateID(),
		"eventType":    event.EventType(),
		"occurredAt":   time.Now().Format(time.RFC3339),
	}
	
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Create EventBridge entry
	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.eventBus),
		Source:       aws.String(p.source),
		DetailType:   aws.String(event.EventType()),
		Detail:       aws.String(string(eventData)),
		Time:         aws.Time(time.Now()),
		Resources:    []string{event.AggregateID()},
	}
	
	// Add trace ID if available
	if traceID := getTraceID(event); traceID != "" {
		entry.TraceHeader = aws.String(traceID)
	}
	
	// Add metadata as part of the detail
	detailMap := make(map[string]interface{})
	if err := json.Unmarshal(eventData, &detailMap); err == nil {
		detailMap["_metadata"] = json.RawMessage(metadataJSON)
		if updatedDetail, err := json.Marshal(detailMap); err == nil {
			entry.Detail = aws.String(string(updatedDetail))
		}
	}
	
	return entry, nil
}

// getTraceID extracts trace ID from event if available
func getTraceID(event domain.DomainEvent) string {
	// Check if event implements TraceableEvent interface
	type traceableEvent interface {
		GetTraceID() string
	}
	
	if traceable, ok := event.(traceableEvent); ok {
		return traceable.GetTraceID()
	}
	
	return ""
}

// AsyncEventPublisher wraps an EventPublisher to provide asynchronous publishing
type AsyncEventPublisher struct {
	publisher repository.EventPublisher
	queue     chan domain.DomainEvent
	done      chan struct{}
}

// NewAsyncEventPublisher creates a new asynchronous event publisher
func NewAsyncEventPublisher(publisher repository.EventPublisher, queueSize int) *AsyncEventPublisher {
	if queueSize <= 0 {
		queueSize = 1000
	}
	
	p := &AsyncEventPublisher{
		publisher: publisher,
		queue:     make(chan domain.DomainEvent, queueSize),
		done:      make(chan struct{}),
	}
	
	// Start the background worker
	go p.worker()
	
	return p
}

// Publish queues events for asynchronous publishing
func (p *AsyncEventPublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
	for _, event := range events {
		select {
		case p.queue <- event:
			// Event queued successfully
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Queue is full
			return fmt.Errorf("event queue is full")
		}
	}
	return nil
}

// worker processes events from the queue
func (p *AsyncEventPublisher) worker() {
	batch := make([]domain.DomainEvent, 0, 10)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-p.queue:
			batch = append(batch, event)
			
			// Publish when batch is full
			if len(batch) >= 10 {
				p.publishBatch(batch)
				batch = batch[:0]
			}
			
		case <-ticker.C:
			// Publish any pending events
			if len(batch) > 0 {
				p.publishBatch(batch)
				batch = batch[:0]
			}
			
		case <-p.done:
			// Publish any remaining events before shutting down
			if len(batch) > 0 {
				p.publishBatch(batch)
			}
			return
		}
	}
}

// publishBatch publishes a batch of events
func (p *AsyncEventPublisher) publishBatch(events []domain.DomainEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := p.publisher.Publish(ctx, events); err != nil {
		// Log error (in production, use proper logging)
		fmt.Printf("Failed to publish events: %v\n", err)
	}
}

// Close stops the async publisher
func (p *AsyncEventPublisher) Close() {
	close(p.done)
}

// NoOpEventPublisher is a no-op implementation of EventPublisher for testing
type NoOpEventPublisher struct{}

// NewNoOpEventPublisher creates a new no-op event publisher
func NewNoOpEventPublisher() repository.EventPublisher {
	return &NoOpEventPublisher{}
}

// Publish does nothing
func (p *NoOpEventPublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
	return nil
}

// BufferedEventPublisher buffers events and publishes them in batches
type BufferedEventPublisher struct {
	publisher     repository.EventPublisher
	buffer        []domain.DomainEvent
	bufferSize    int
	flushInterval time.Duration
	lastFlush     time.Time
}

// NewBufferedEventPublisher creates a new buffered event publisher
func NewBufferedEventPublisher(publisher repository.EventPublisher, bufferSize int, flushInterval time.Duration) *BufferedEventPublisher {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 1 * time.Second
	}
	
	return &BufferedEventPublisher{
		publisher:     publisher,
		buffer:        make([]domain.DomainEvent, 0, bufferSize),
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		lastFlush:     time.Now(),
	}
}

// Publish adds events to the buffer and flushes if necessary
func (p *BufferedEventPublisher) Publish(ctx context.Context, events []domain.DomainEvent) error {
	p.buffer = append(p.buffer, events...)
	
	// Check if we should flush
	shouldFlush := len(p.buffer) >= p.bufferSize ||
		time.Since(p.lastFlush) >= p.flushInterval
	
	if shouldFlush {
		return p.Flush(ctx)
	}
	
	return nil
}

// Flush publishes all buffered events
func (p *BufferedEventPublisher) Flush(ctx context.Context) error {
	if len(p.buffer) == 0 {
		return nil
	}
	
	err := p.publisher.Publish(ctx, p.buffer)
	if err != nil {
		return err
	}
	
	// Clear buffer
	p.buffer = p.buffer[:0]
	p.lastFlush = time.Now()
	
	return nil
}