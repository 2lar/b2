// Package dynamodb provides DynamoDB implementation of the EventStore
package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// EventStore implements the ports.EventStore interface using DynamoDB
type EventStore struct {
	client    *dynamodb.Client
	tableName string
	logger    ports.Logger
}

// NewEventStore creates a new DynamoDB event store
func NewEventStore(client *dynamodb.Client, tableName string, logger ports.Logger) *EventStore {
	return &EventStore{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// eventItem represents the DynamoDB item structure for an event
type eventItem struct {
	PK            string                 `dynamodbav:"PK"`
	SK            string                 `dynamodbav:"SK"`
	EventID       string                 `dynamodbav:"EventID"`
	EventType     string                 `dynamodbav:"EventType"`
	AggregateID   string                 `dynamodbav:"AggregateID"`
	AggregateType string                 `dynamodbav:"AggregateType"`
	Version       int64                  `dynamodbav:"Version"`
	OccurredAt    string                 `dynamodbav:"OccurredAt"`
	Data          string                 `dynamodbav:"Data"`
	Metadata      map[string]interface{} `dynamodbav:"Metadata,omitempty"`
	EntityType    string                 `dynamodbav:"EntityType"`
	TTL           int64                  `dynamodbav:"TTL,omitempty"`
}

// SaveEvents saves events to the event store with optimistic concurrency control
func (es *EventStore) SaveEvents(ctx context.Context, aggregateID string, domainEvents []events.DomainEvent, expectedVersion int64) error {
	if len(domainEvents) == 0 {
		return nil
	}

	// Prepare batch write items
	writeRequests := make([]types.WriteRequest, 0, len(domainEvents))
	
	for _, event := range domainEvents {
		item, err := es.eventToItem(aggregateID, event)
		if err != nil {
			es.logger.Error("Failed to convert event to item", err,
				ports.Field{Key: "event_type", Value: event.GetEventType()},
				ports.Field{Key: "aggregate_id", Value: aggregateID})
			return fmt.Errorf("failed to convert event: %w", err)
		}

		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			es.logger.Error("Failed to marshal event item", err,
				ports.Field{Key: "event_id", Value: event.GetEventID()})
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: av,
			},
		})
	}

	// Execute batch write (DynamoDB supports up to 25 items per batch)
	for i := 0; i < len(writeRequests); i += 25 {
		end := i + 25
		if end > len(writeRequests) {
			end = len(writeRequests)
		}
		
		batch := writeRequests[i:end]
		if err := es.writeBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to write event batch: %w", err)
		}
	}

	es.logger.Debug("Events saved successfully",
		ports.Field{Key: "aggregate_id", Value: aggregateID},
		ports.Field{Key: "event_count", Value: len(domainEvents)},
		ports.Field{Key: "expected_version", Value: expectedVersion})

	return nil
}

// GetEvents retrieves all events for an aggregate
func (es *EventStore) GetEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error) {
	// Query all events for the aggregate
	input := &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("EVENT#%s", aggregateID)},
		},
		ScanIndexForward: aws.Bool(true), // Sort by version ascending
	}

	result, err := es.client.Query(ctx, input)
	if err != nil {
		es.logger.Error("Failed to query events", err,
			ports.Field{Key: "aggregate_id", Value: aggregateID})
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	// Convert items to domain events
	domainEvents := make([]events.DomainEvent, 0, len(result.Items))
	for _, item := range result.Items {
		var eventItem eventItem
		if err := attributevalue.UnmarshalMap(item, &eventItem); err != nil {
			es.logger.Warn("Failed to unmarshal event item",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		domainEvent, err := es.itemToEvent(&eventItem)
		if err != nil {
			es.logger.Warn("Failed to convert item to event",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		domainEvents = append(domainEvents, domainEvent)
	}

	return domainEvents, nil
}

// GetEventsAfterVersion retrieves events after a specific version
func (es *EventStore) GetEventsAfterVersion(ctx context.Context, aggregateID string, version int64) ([]events.DomainEvent, error) {
	// Query events with version greater than specified
	input := &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND SK > :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("EVENT#%s", aggregateID)},
			":sk": &types.AttributeValueMemberS{Value: fmt.Sprintf("VERSION#%020d", version)},
		},
		ScanIndexForward: aws.Bool(true), // Sort by version ascending
	}

	result, err := es.client.Query(ctx, input)
	if err != nil {
		es.logger.Error("Failed to query events after version", err,
			ports.Field{Key: "aggregate_id", Value: aggregateID},
			ports.Field{Key: "version", Value: version})
		return nil, fmt.Errorf("failed to get events after version: %w", err)
	}

	// Convert items to domain events
	domainEvents := make([]events.DomainEvent, 0, len(result.Items))
	for _, item := range result.Items {
		var eventItem eventItem
		if err := attributevalue.UnmarshalMap(item, &eventItem); err != nil {
			es.logger.Warn("Failed to unmarshal event item",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		domainEvent, err := es.itemToEvent(&eventItem)
		if err != nil {
			es.logger.Warn("Failed to convert item to event",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		domainEvents = append(domainEvents, domainEvent)
	}

	return domainEvents, nil
}

// GetSnapshot retrieves the latest snapshot for an aggregate
func (es *EventStore) GetSnapshot(ctx context.Context, aggregateID string) (*events.AggregateSnapshot, error) {
	// Query for snapshot (stored with SK = "SNAPSHOT")
	input := &dynamodb.GetItemInput{
		TableName: aws.String(es.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("SNAPSHOT#%s", aggregateID)},
			"SK": &types.AttributeValueMemberS{Value: "LATEST"},
		},
	}

	result, err := es.client.GetItem(ctx, input)
	if err != nil {
		es.logger.Error("Failed to get snapshot", err,
			ports.Field{Key: "aggregate_id", Value: aggregateID})
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	if result.Item == nil {
		return nil, nil // No snapshot found
	}

	// Unmarshal snapshot
	var snapshotData map[string]interface{}
	if err := attributevalue.UnmarshalMap(result.Item, &snapshotData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	// Convert to AggregateSnapshot
	snapshot := &events.AggregateSnapshot{
		AggregateID:   aggregateID,
		AggregateType: snapshotData["AggregateType"].(string),
		Data:          snapshotData["Data"].(map[string]interface{}),
		Version:       int64(snapshotData["Version"].(float64)),
	}

	if timestamp, ok := snapshotData["Timestamp"].(string); ok {
		snapshot.Timestamp, _ = time.Parse(time.RFC3339, timestamp)
	}

	return snapshot, nil
}

// SaveSnapshot saves a snapshot of an aggregate
func (es *EventStore) SaveSnapshot(ctx context.Context, snapshot *events.AggregateSnapshot) error {
	// Prepare snapshot item
	item := map[string]interface{}{
		"PK":            fmt.Sprintf("SNAPSHOT#%s", snapshot.AggregateID),
		"SK":            "LATEST",
		"AggregateID":   snapshot.AggregateID,
		"AggregateType": snapshot.AggregateType,
		"Data":          snapshot.Data,
		"Version":       snapshot.Version,
		"Timestamp":     snapshot.Timestamp.Format(time.RFC3339),
		"EntityType":    "SNAPSHOT",
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		es.logger.Error("Failed to marshal snapshot", err,
			ports.Field{Key: "aggregate_id", Value: snapshot.AggregateID})
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Save snapshot
	_, err = es.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(es.tableName),
		Item:      av,
	})

	if err != nil {
		es.logger.Error("Failed to save snapshot", err,
			ports.Field{Key: "aggregate_id", Value: snapshot.AggregateID})
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	es.logger.Debug("Snapshot saved successfully",
		ports.Field{Key: "aggregate_id", Value: snapshot.AggregateID},
		ports.Field{Key: "version", Value: snapshot.Version})

	return nil
}

// eventToItem converts a domain event to a DynamoDB item
func (es *EventStore) eventToItem(aggregateID string, event events.DomainEvent) (*eventItem, error) {
	// Serialize event data
	dataJSON, err := json.Marshal(event.GetData())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Build composite keys
	pk := fmt.Sprintf("EVENT#%s", aggregateID)
	sk := fmt.Sprintf("VERSION#%020d", event.GetVersion())

	// Calculate TTL (e.g., 90 days retention)
	ttl := time.Now().Add(90 * 24 * time.Hour).Unix()

	item := &eventItem{
		PK:            pk,
		SK:            sk,
		EventID:       event.GetEventID(),
		EventType:     event.GetEventType(),
		AggregateID:   aggregateID,
		AggregateType: event.GetAggregateType(),
		Version:       event.GetVersion(),
		OccurredAt:    event.GetOccurredAt().Format(time.RFC3339),
		Data:          string(dataJSON),
		Metadata:      event.GetMetadata(),
		EntityType:    "EVENT",
		TTL:           ttl,
	}

	return item, nil
}

// itemToEvent converts a DynamoDB item to a domain event
func (es *EventStore) itemToEvent(item *eventItem) (events.DomainEvent, error) {
	// Parse event data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(item.Data), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	// Parse timestamp
	occurredAt, err := time.Parse(time.RFC3339, item.OccurredAt)
	if err != nil {
		occurredAt = time.Now()
	}

	// Create base event
	baseEvent := events.NewBaseEvent(
		item.AggregateID,
		item.AggregateType,
		item.EventType,
		data,
		item.Version,
	)

	// Set additional fields
	baseEvent.EventID = item.EventID
	baseEvent.OccurredAt = occurredAt
	baseEvent.Metadata = item.Metadata

	return baseEvent, nil
}

// writeBatch writes a batch of events to DynamoDB
func (es *EventStore) writeBatch(ctx context.Context, requests []types.WriteRequest) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			es.tableName: requests,
		},
	}

	output, err := es.client.BatchWriteItem(ctx, input)
	if err != nil {
		es.logger.Error("Failed to batch write events", err,
			ports.Field{Key: "batch_size", Value: len(requests)})
		return err
	}

	// Check for unprocessed items
	if len(output.UnprocessedItems) > 0 {
		es.logger.Warn("Some events were not processed",
			ports.Field{Key: "unprocessed_count", Value: len(output.UnprocessedItems)})
		// In production, you'd want to retry these
		return fmt.Errorf("failed to process all events")
	}

	return nil
}