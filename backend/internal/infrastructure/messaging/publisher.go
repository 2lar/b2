// Package messaging provides event publishing infrastructure for the Brain2 backend.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	coreEvents "brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/domain/shared"
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
		source = "brain2.api"
	}
	
	publisher := &EventBridgePublisher{
		client:    client,
		eventBus:  eventBus,
		source:    source,
		batchSize: 10, // EventBridge has a limit of 10 entries per PutEvents call
	}
	
	return publisher
}

// Publish publishes domain events to EventBridge
func (p *EventBridgePublisher) Publish(ctx context.Context, events []shared.DomainEvent) error {
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
func (p *EventBridgePublisher) publishBatch(ctx context.Context, events []shared.DomainEvent) error {
	entries := make([]types.PutEventsRequestEntry, 0, len(events))
	
	// Create entries for each event
	for _, event := range events {
		entry, err := p.createEventEntry(event)
		if err != nil {
			return fmt.Errorf("failed to create event entry for %s: %w", event.EventType(), err)
		}
		entries = append(entries, entry)
	}
	
	// Call EventBridge API
	output, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: entries,
	})
	
	if err != nil {
		return fmt.Errorf("EventBridge API call failed: %w", err)
	}
	
	// Check API response
	if output == nil {
		return fmt.Errorf("EventBridge API returned nil response")
	}
	
	// Check for failed entries
	if output.FailedEntryCount > 0 {
		if output.Entries != nil {
			for i, entry := range output.Entries {
				if entry.ErrorCode != nil {
					// Could add structured logging here if logger was passed
					_ = i // Use index if needed for logging
				}
			}
		}
		return fmt.Errorf("EventBridge rejected %d out of %d events", output.FailedEntryCount, len(entries))
	}
	
	return nil
}

// createEventEntry creates an EventBridge entry from a domain event
func (p *EventBridgePublisher) createEventEntry(event shared.DomainEvent) (types.PutEventsRequestEntry, error) {
	// Marshal event data to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create detail map with all required fields at the top level
	detailMap := make(map[string]interface{})
	
	// Unmarshal event data into the detail map
	if err := json.Unmarshal(eventData, &detailMap); err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to unmarshal event data: %w", err)
	}
	
	// Add critical fields at the top level for EventBridge handlers
	// These fields are needed by cleanup Lambda and other handlers
	detailMap["aggregate_id"] = event.AggregateID()  // Node ID for NodeDeleted events
	detailMap["user_id"] = event.UserID()            // User ID who owns the resource
	detailMap["event_id"] = event.EventID()
	detailMap["event_type"] = event.EventType()
	detailMap["occurred_at"] = time.Now().Format(time.RFC3339)
	detailMap["version"] = event.Version()
	
	// Add camelCase versions for connect-node Lambda compatibility
	// IMPORTANT: These must be set AFTER unmarshaling to avoid being overwritten
	detailMap["nodeId"] = event.AggregateID()  // Expected by connect-node Lambda
	detailMap["userId"] = event.UserID()       // Expected by connect-node Lambda
	
	// For NodeCreated events, ensure keywords are available in the expected format
	// The connect-node Lambda expects "keywords" field to be present
	if event.EventType() == "NodeCreated" {
		// Check if keywords exist in the unmarshaled data
		// The shared.NodeCreatedEvent has a Keywords field (capital K)
		if keywords, ok := detailMap["Keywords"]; ok {
			// Copy to lowercase for Lambda compatibility
			detailMap["keywords"] = keywords
		} else if _, ok := detailMap["keywords"]; !ok {
			// If no keywords at all, create empty array
			detailMap["keywords"] = []string{}
		}
		
		// Also ensure content is available if needed
		if content, ok := detailMap["Content"]; ok {
			detailMap["content"] = content
		}
	}
	
	// Also keep metadata for backward compatibility
	metadata := map[string]interface{}{
		"eventId":      event.EventID(),
		"aggregateId":  event.AggregateID(),
		"userId":       event.UserID(),
		"eventType":    event.EventType(),
		"occurredAt":   time.Now().Format(time.RFC3339),
		"version":      event.Version(),
	}
	detailMap["_metadata"] = metadata
	
	// Marshal the complete detail map
	detailJSON, err := json.Marshal(detailMap)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal detail: %w", err)
	}
	
	// Create EventBridge entry
	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.eventBus),
		Source:       aws.String(p.source),
		DetailType:   aws.String(event.EventType()),
		Detail:       aws.String(string(detailJSON)),
		Time:         aws.Time(time.Now()),
		Resources:    []string{event.AggregateID()},
	}
	
	// Add trace ID if available
	if traceID := getTraceID(event); traceID != "" {
		entry.TraceHeader = aws.String(traceID)
	}
	
	return entry, nil
}

// PublishDomainEvent publishes a single core domain event to EventBridge
func (p *EventBridgePublisher) PublishDomainEvent(ctx context.Context, event coreEvents.DomainEvent) error {
	return p.PublishDomainEvents(ctx, []coreEvents.DomainEvent{event})
}

// PublishDomainEvents publishes multiple core domain events to EventBridge
func (p *EventBridgePublisher) PublishDomainEvents(ctx context.Context, events []coreEvents.DomainEvent) error {
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
		if err := p.publishCoreBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to publish core event batch: %w", err)
		}
	}
	
	return nil
}

// publishCoreBatch publishes a batch of core domain events to EventBridge
func (p *EventBridgePublisher) publishCoreBatch(ctx context.Context, events []coreEvents.DomainEvent) error {
	entries := make([]types.PutEventsRequestEntry, 0, len(events))
	
	// Create entries for each event
	for _, event := range events {
		entry, err := p.createCoreEventEntry(event)
		if err != nil {
			return fmt.Errorf("failed to create core event entry for %s: %w", event.GetEventType(), err)
		}
		entries = append(entries, entry)
	}
	
	// Call EventBridge API
	output, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: entries,
	})
	
	if err != nil {
		return fmt.Errorf("EventBridge API call failed: %w", err)
	}
	
	// Check API response
	if output == nil {
		return fmt.Errorf("EventBridge API returned nil response")
	}
	
	// Check for failed entries
	if output.FailedEntryCount > 0 {
		if output.Entries != nil {
			for i, entry := range output.Entries {
				if entry.ErrorCode != nil {
					// Could add structured logging here if logger was passed
					_ = i // Use index if needed for logging
				}
			}
		}
		return fmt.Errorf("failed to publish %d events to EventBridge", output.FailedEntryCount)
	}
	
	return nil
}

// createCoreEventEntry creates an EventBridge entry from a core domain event
func (p *EventBridgePublisher) createCoreEventEntry(event coreEvents.DomainEvent) (types.PutEventsRequestEntry, error) {
	// Create detail map with all event data
	detailMap := map[string]interface{}{
		"eventId":      event.GetEventID(),
		"eventType":    event.GetEventType(),
		"aggregateId":  event.GetAggregateID(),
		"aggregateType": event.GetAggregateType(),
		"occurredAt":   event.GetOccurredAt().Format(time.RFC3339),
		"version":      event.GetVersion(),
	}
	
	// For NodeCreated events, extract the fields and put them at the top level
	// This is required for the connect-node Lambda to properly receive the data
	if event.GetEventType() == "NodeCreated" {
		// Marshal the event data to extract fields
		eventData := event.GetData()
		eventJSON, err := json.Marshal(eventData)
		if err != nil {
			return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal NodeCreated event: %w", err)
		}
		
		// Unmarshal into a map to extract fields
		var nodeCreatedData map[string]interface{}
		if err := json.Unmarshal(eventJSON, &nodeCreatedData); err != nil {
			return types.PutEventsRequestEntry{}, fmt.Errorf("failed to unmarshal NodeCreated data: %w", err)
		}
		
		// Extract and add fields at the top level with both snake_case and camelCase
		// The connect-node Lambda expects camelCase fields
		if userID, ok := nodeCreatedData["user_id"]; ok {
			detailMap["user_id"] = userID
			detailMap["userId"] = userID  // camelCase for Lambda
		}
		
		if keywords, ok := nodeCreatedData["keywords"]; ok {
			detailMap["keywords"] = keywords
		} else {
			detailMap["keywords"] = []string{}  // Default empty array
		}
		
		// Also add nodeId in camelCase for Lambda (same as aggregateId)
		detailMap["nodeId"] = event.GetAggregateID()
		
		// Keep the full data for backward compatibility
		detailMap["data"] = eventData
	} else {
		// For other events, keep the original structure
		detailMap["data"] = event.GetData()
	}
	
	// Add metadata
	metadata := event.GetMetadata()
	detailMap["metadata"] = map[string]interface{}{
		"correlationId": metadata.CorrelationID,
		"causationId":   metadata.CausationID,
		"userId":        metadata.UserID,
		"ipAddress":     metadata.IPAddress,
		"userAgent":     metadata.UserAgent,
		"custom":        metadata.Custom,
	}
	
	// For NodeCreated, also ensure userId is set from metadata if not already present
	if event.GetEventType() == "NodeCreated" && metadata.UserID != "" {
		if _, hasUserId := detailMap["userId"]; !hasUserId {
			detailMap["userId"] = metadata.UserID
			detailMap["user_id"] = metadata.UserID
		}
	}
	
	// Marshal the complete detail map
	detailJSON, err := json.Marshal(detailMap)
	if err != nil {
		return types.PutEventsRequestEntry{}, fmt.Errorf("failed to marshal detail: %w", err)
	}
	
	// Create EventBridge entry
	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.eventBus),
		Source:       aws.String(p.source),
		DetailType:   aws.String(event.GetEventType()),
		Detail:       aws.String(string(detailJSON)),
		Time:         aws.Time(event.GetOccurredAt()),
		Resources:    []string{event.GetAggregateID()},
	}
	
	return entry, nil
}

// getTraceID extracts trace ID from event if available
func getTraceID(event shared.DomainEvent) string {
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
	queue     chan shared.DomainEvent
	done      chan struct{}
}

// NewAsyncEventPublisher creates a new asynchronous event publisher
func NewAsyncEventPublisher(publisher repository.EventPublisher, queueSize int) *AsyncEventPublisher {
	if queueSize <= 0 {
		queueSize = 1000
	}
	
	p := &AsyncEventPublisher{
		publisher: publisher,
		queue:     make(chan shared.DomainEvent, queueSize),
		done:      make(chan struct{}),
	}
	
	// Start the background worker
	go p.worker()
	
	return p
}

// Publish queues events for asynchronous publishing
func (p *AsyncEventPublisher) Publish(ctx context.Context, events []shared.DomainEvent) error {
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
	batch := make([]shared.DomainEvent, 0, 10)
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
func (p *AsyncEventPublisher) publishBatch(events []shared.DomainEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := p.publisher.Publish(ctx, events); err != nil {
		// In production, use proper structured logging here
		// For now, silently drop errors to avoid fmt.Printf
		_ = err
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
func (p *NoOpEventPublisher) Publish(ctx context.Context, events []shared.DomainEvent) error {
	return nil
}

// BufferedEventPublisher buffers events and publishes them in batches
type BufferedEventPublisher struct {
	publisher     repository.EventPublisher
	buffer        []shared.DomainEvent
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
		buffer:        make([]shared.DomainEvent, 0, bufferSize),
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		lastFlush:     time.Now(),
	}
}

// Publish adds events to the buffer and flushes if necessary
func (p *BufferedEventPublisher) Publish(ctx context.Context, events []shared.DomainEvent) error {
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