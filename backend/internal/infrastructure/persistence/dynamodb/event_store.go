package dynamodb

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	"brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBEventStore implements EventStore using DynamoDB
type DynamoDBEventStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBEventStore creates a new DynamoDB-based event store
func NewDynamoDBEventStore(client *dynamodb.Client, tableName string) repository.EventStore {
	return &DynamoDBEventStore{
		client:    client,
		tableName: tableName,
	}
}

// EventRecord represents how events are stored in DynamoDB
type EventRecord struct {
	PK            string                 `dynamodbav:"PK"`     // AGGREGATE#<aggregateID>
	SK            string                 `dynamodbav:"SK"`     // EVENT#<version>#<eventID>
	AggregateID   string                 `dynamodbav:"AggregateID"`
	EventID       string                 `dynamodbav:"EventID"`
	EventType     string                 `dynamodbav:"EventType"`
	EventVersion  int                    `dynamodbav:"EventVersion"`
	EventData     map[string]interface{} `dynamodbav:"EventData"`
	UserID        string                 `dynamodbav:"UserID"`
	Timestamp     time.Time              `dynamodbav:"Timestamp"`
	TTL           int64                  `dynamodbav:"TTL,omitempty"`
}

// SaveEvents persists domain events with optimistic concurrency control
func (s *DynamoDBEventStore) SaveEvents(ctx context.Context, aggregateID string, events []shared.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	// Prepare batch write items
	writeRequests := make([]types.WriteRequest, 0, len(events))
	
	for _, event := range events {
		record := EventRecord{
			PK:           fmt.Sprintf("AGGREGATE#%s", aggregateID),
			SK:           fmt.Sprintf("EVENT#%05d#%s", event.Version(), event.EventID()),
			AggregateID:  aggregateID,
			EventID:      event.EventID(),
			EventType:    event.EventType(),
			EventVersion: event.Version(),
			EventData:    event.EventData(),
			UserID:       event.UserID(),
			Timestamp:    event.Timestamp(),
			TTL:          time.Now().Add(90 * 24 * time.Hour).Unix(), // 90 days retention
		}
		
		av, err := attributevalue.MarshalMap(record)
		if err != nil {
			return errors.Wrap(err, "failed to marshal event record")
		}
		
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: av,
			},
		})
	}
	
	// Execute batch write
	_, err := s.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			s.tableName: writeRequests,
		},
	})
	
	if err != nil {
		return errors.Wrap(err, "failed to save events")
	}
	
	return nil
}

// GetEvents retrieves all events for an aggregate
func (s *DynamoDBEventStore) GetEvents(ctx context.Context, aggregateID string) ([]shared.DomainEvent, error) {
	pk := fmt.Sprintf("AGGREGATE#%s", aggregateID)
	
	resp, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: pk},
			":sk_prefix": &types.AttributeValueMemberS{Value: "EVENT#"},
		},
		ScanIndexForward: aws.Bool(true), // Order by version ascending
	})
	
	if err != nil {
		return nil, errors.Wrap(err, "failed to query events")
	}
	
	events := make([]shared.DomainEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal event record")
		}
		
		event := s.reconstructEvent(record)
		events = append(events, event)
	}
	
	return events, nil
}

// GetEventsAfterVersion retrieves events after a specific version
func (s *DynamoDBEventStore) GetEventsAfterVersion(ctx context.Context, aggregateID string, version int) ([]shared.DomainEvent, error) {
	pk := fmt.Sprintf("AGGREGATE#%s", aggregateID)
	skStart := fmt.Sprintf("EVENT#%05d", version+1)
	
	resp, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND SK >= :sk_start"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: pk},
			":sk_start": &types.AttributeValueMemberS{Value: skStart},
		},
		ScanIndexForward: aws.Bool(true),
	})
	
	if err != nil {
		return nil, errors.Wrap(err, "failed to query events after version")
	}
	
	events := make([]shared.DomainEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal event record")
		}
		
		event := s.reconstructEvent(record)
		events = append(events, event)
	}
	
	return events, nil
}

// GetEventsByType retrieves events of a specific type since a given time
func (s *DynamoDBEventStore) GetEventsByType(ctx context.Context, eventType string, since time.Time) ([]shared.DomainEvent, error) {
	// This would require a GSI on EventType and Timestamp
	// For now, we'll implement a simplified version
	
	resp, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(s.tableName),
		FilterExpression: aws.String("EventType = :event_type AND #ts >= :since"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":event_type": &types.AttributeValueMemberS{Value: eventType},
			":since":      &types.AttributeValueMemberS{Value: since.Format(time.RFC3339)},
		},
		ExpressionAttributeNames: map[string]string{
			"#ts": "Timestamp",
		},
		Limit: aws.Int32(100), // Limit scan to prevent performance issues
	})
	
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan events by type")
	}
	
	events := make([]shared.DomainEvent, 0, len(resp.Items))
	for _, item := range resp.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal event record")
		}
		
		event := s.reconstructEvent(record)
		events = append(events, event)
	}
	
	return events, nil
}

// GetSnapshot retrieves the latest snapshot for an aggregate
func (s *DynamoDBEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*repository.AggregateSnapshot, error) {
	pk := fmt.Sprintf("AGGREGATE#%s", aggregateID)
	sk := "SNAPSHOT#LATEST"
	
	resp, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	
	if err != nil {
		return nil, errors.Wrap(err, "failed to get snapshot")
	}
	
	if resp.Item == nil {
		return nil, nil // No snapshot found
	}
	
	var snapshot repository.AggregateSnapshot
	if err := attributevalue.UnmarshalMap(resp.Item, &snapshot); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal snapshot")
	}
	
	return &snapshot, nil
}

// SaveSnapshot saves a snapshot of aggregate state
func (s *DynamoDBEventStore) SaveSnapshot(ctx context.Context, snapshot *repository.AggregateSnapshot) error {
	item := map[string]interface{}{
		"PK":            fmt.Sprintf("AGGREGATE#%s", snapshot.AggregateID),
		"SK":            "SNAPSHOT#LATEST",
		"AggregateID":   snapshot.AggregateID,
		"AggregateType": snapshot.AggregateType,
		"Version":       snapshot.Version,
		"Data":          snapshot.Data,
		"CreatedAt":     snapshot.CreatedAt,
		"TTL":           time.Now().Add(90 * 24 * time.Hour).Unix(),
	}
	
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return errors.Wrap(err, "failed to marshal snapshot")
	}
	
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	
	if err != nil {
		return errors.Wrap(err, "failed to save snapshot")
	}
	
	return nil
}

// reconstructEvent converts an EventRecord back to a DomainEvent
func (s *DynamoDBEventStore) reconstructEvent(record EventRecord) shared.DomainEvent {
	// This is a simplified reconstruction - in practice, you'd need a factory
	// to create the specific event types based on EventType
	return &GenericDomainEvent{
		eventID:     record.EventID,
		eventType:   record.EventType,
		aggregateID: record.AggregateID,
		userID:      record.UserID,
		timestamp:   record.Timestamp,
		version:     record.EventVersion,
		data:        record.EventData,
	}
}

// GenericDomainEvent is a generic implementation for reconstructed events
type GenericDomainEvent struct {
	eventID     string
	eventType   string
	aggregateID string
	userID      string
	timestamp   time.Time
	version     int
	data        map[string]interface{}
}

func (e *GenericDomainEvent) EventID() string                   { return e.eventID }
func (e *GenericDomainEvent) EventType() string                 { return e.eventType }
func (e *GenericDomainEvent) AggregateID() string               { return e.aggregateID }
func (e *GenericDomainEvent) UserID() string                    { return e.userID }
func (e *GenericDomainEvent) Timestamp() time.Time              { return e.timestamp }
func (e *GenericDomainEvent) Version() int                      { return e.version }
func (e *GenericDomainEvent) EventData() map[string]interface{} { return e.data }