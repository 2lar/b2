package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend2/domain/core/valueobjects"
	"backend2/domain/events"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// DynamoDBEventStore implements the EventStore interface using DynamoDB
type DynamoDBEventStore struct {
	client    *dynamodb.Client
	tableName string
}

// PublishStatus represents the publishing status of an event
type PublishStatus string

const (
	PublishStatusPending   PublishStatus = "pending"   // Event is saved but not yet published
	PublishStatusPublished PublishStatus = "published" // Event successfully published
	PublishStatusFailed    PublishStatus = "failed"    // Event publishing failed
)

// EventRecord represents how events are stored in DynamoDB with Outbox pattern
type EventRecord struct {
	PK            string                 `dynamodbav:"PK"`     // EVENTS#<aggregate_id>
	SK            string                 `dynamodbav:"SK"`     // EVENT#<timestamp>#<event_id>
	EventID       string                 `dynamodbav:"EventID"`
	EventType     string                 `dynamodbav:"EventType"`
	AggregateID   string                 `dynamodbav:"AggregateID"`
	AggregateType string                 `dynamodbav:"AggregateType"`
	EventData     map[string]interface{} `dynamodbav:"EventData"`
	Metadata      map[string]interface{} `dynamodbav:"Metadata"`
	Timestamp     string                 `dynamodbav:"Timestamp"`
	Version       int                    `dynamodbav:"Version"`
	UserID        string                 `dynamodbav:"UserID"`
	
	// Outbox pattern fields
	PublishStatus    string    `dynamodbav:"PublishStatus"`    // pending/published/failed
	PublishAttempts  int       `dynamodbav:"PublishAttempts"`  // Number of publish attempts
	LastPublishTry   string    `dynamodbav:"LastPublishTry,omitempty"` // RFC3339 timestamp
	PublishedAt      string    `dynamodbav:"PublishedAt,omitempty"`    // RFC3339 timestamp when published
	ErrorMessage     string    `dynamodbav:"ErrorMessage,omitempty"`   // Last error message if failed
	
	// GSI attributes for querying
	GSI1PK        string                 `dynamodbav:"GSI1PK"` // USER#<user_id>
	GSI1SK        string                 `dynamodbav:"GSI1SK"` // EVENT#<timestamp>
	GSI2PK        string                 `dynamodbav:"GSI2PK"` // EVENTTYPE#<type>
	GSI2SK        string                 `dynamodbav:"GSI2SK"` // EVENT#<timestamp>
	
	// TTL for automatic cleanup (optional)
	TTL           int64                  `dynamodbav:"TTL,omitempty"`
}

// NewDynamoDBEventStore creates a new DynamoDB event store
func NewDynamoDBEventStore(client *dynamodb.Client, tableName string) *DynamoDBEventStore {
	return &DynamoDBEventStore{
		client:    client,
		tableName: tableName,
	}
}

// SaveEvents persists domain events to the event store
func (es *DynamoDBEventStore) SaveEvents(ctx context.Context, domainEvents []events.DomainEvent) error {
	if len(domainEvents) == 0 {
		return nil
	}
	
	writeRequests := make([]types.WriteRequest, 0, len(domainEvents))
	
	for _, event := range domainEvents {
		record, err := es.eventToRecord(event)
		if err != nil {
			return fmt.Errorf("failed to convert event to record: %w", err)
		}
		
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			return fmt.Errorf("failed to marshal event record: %w", err)
		}
		
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}
	
	// Batch write events (DynamoDB limit is 25 items per batch)
	for i := 0; i < len(writeRequests); i += 25 {
		end := i + 25
		if end > len(writeRequests) {
			end = len(writeRequests)
		}
		
		batch := writeRequests[i:end]
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				es.tableName: batch,
			},
		}
		
		result, err := es.client.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to write events batch: %w", err)
		}
		
		// Handle unprocessed items (retry logic could be added here)
		if len(result.UnprocessedItems) > 0 {
			// For now, return an error. In production, implement retry with backoff
			return fmt.Errorf("failed to write %d events", len(result.UnprocessedItems[es.tableName]))
		}
	}
	
	return nil
}

// GetEvents retrieves all events for an aggregate
func (es *DynamoDBEventStore) GetEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("EVENTS#%s", aggregateID)},
		},
		ScanIndexForward: aws.Bool(true), // Order by timestamp ascending
	}
	
	var allEvents []events.DomainEvent
	
	// Handle pagination
	for {
		result, err := es.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to query events: %w", err)
		}
		
		for _, item := range result.Items {
			var record EventRecord
			if err := attributevalue.UnmarshalMap(item, &record); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event record: %w", err)
			}
			
			event, err := es.recordToEvent(record)
			if err != nil {
				return nil, fmt.Errorf("failed to convert record to event: %w", err)
			}
			
			allEvents = append(allEvents, event)
		}
		
		// Check if there are more pages
		if result.LastEvaluatedKey == nil {
			break
		}
		input.ExclusiveStartKey = result.LastEvaluatedKey
	}
	
	return allEvents, nil
}

// GetEventsByType retrieves events of a specific type
func (es *DynamoDBEventStore) GetEventsByType(ctx context.Context, eventType string, limit int) ([]events.DomainEvent, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		IndexName:              aws.String("GSI2"),
		KeyConditionExpression: aws.String("GSI2PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("EVENTTYPE#%s", eventType)},
		},
		ScanIndexForward: aws.Bool(false), // Order by timestamp descending (most recent first)
	}
	
	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}
	
	result, err := es.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by type: %w", err)
	}
	
	domainEvents := make([]events.DomainEvent, 0, len(result.Items))
	for _, item := range result.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event record: %w", err)
		}
		
		event, err := es.recordToEvent(record)
		if err != nil {
			return nil, fmt.Errorf("failed to convert record to event: %w", err)
		}
		
		domainEvents = append(domainEvents, event)
	}
	
	return domainEvents, nil
}

// GetEventsAfter retrieves events for an aggregate after a specific version
func (es *DynamoDBEventStore) GetEventsAfter(ctx context.Context, aggregateID string, version int) ([]events.DomainEvent, error) {
	// First get all events for the aggregate
	allEvents, err := es.GetEvents(ctx, aggregateID)
	if err != nil {
		return nil, err
	}
	
	// Filter events after the specified version
	var filteredEvents []events.DomainEvent
	for _, event := range allEvents {
		if event.GetVersion() > version {
			filteredEvents = append(filteredEvents, event)
		}
	}
	
	return filteredEvents, nil
}

// GetEventsByUser retrieves events for a specific user
func (es *DynamoDBEventStore) GetEventsByUser(ctx context.Context, userID string, since time.Time, limit int) ([]events.DomainEvent, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND GSI1SK > :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk": &types.AttributeValueMemberS{Value: fmt.Sprintf("EVENT#%s", since.Format(time.RFC3339Nano))},
		},
		ScanIndexForward: aws.Bool(true), // Order by timestamp ascending
	}
	
	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}
	
	result, err := es.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by user: %w", err)
	}
	
	domainEvents := make([]events.DomainEvent, 0, len(result.Items))
	for _, item := range result.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event record: %w", err)
		}
		
		event, err := es.recordToEvent(record)
		if err != nil {
			return nil, fmt.Errorf("failed to convert record to event: %w", err)
		}
		
		domainEvents = append(domainEvents, event)
	}
	
	return domainEvents, nil
}

// PrepareEventItem prepares an event for transactional write
// This is used by the UnitOfWork to include events in transactions
func (es *DynamoDBEventStore) PrepareEventItem(event events.DomainEvent) (types.TransactWriteItem, error) {
	record, err := es.eventToRecord(event)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	
	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(es.tableName),
			Item:      item,
		},
	}, nil
}

// eventToRecord converts a domain event to a DynamoDB record
func (es *DynamoDBEventStore) eventToRecord(event events.DomainEvent) (*EventRecord, error) {
	// Get event data as a map
	eventData := make(map[string]interface{})
	
	// Try to marshal the event to JSON and then to a map
	// This allows us to store the event data in a flexible format
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	if err := json.Unmarshal(eventBytes, &eventData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event to map: %w", err)
	}
	
	timestamp := event.GetTimestamp()
	// Generate a unique event ID since DomainEvent doesn't have GetEventID
	eventID := uuid.New().String()
	
	// Calculate TTL (optional - events older than 1 year are automatically deleted)
	// You can adjust or remove this based on your requirements
	ttl := timestamp.Add(365 * 24 * time.Hour).Unix()
	
	// Extract user ID from event data if available
	userID := ""
	if userData, ok := eventData["user_id"].(string); ok {
		userID = userData
	}
	
	// Determine aggregate type from event type
	aggregateType := "unknown"
	if strings.HasPrefix(event.GetEventType(), "node.") {
		aggregateType = "node"
	} else if strings.HasPrefix(event.GetEventType(), "graph.") {
		aggregateType = "graph"
	} else if strings.HasPrefix(event.GetEventType(), "nodes.") {
		aggregateType = "edge"
	}
	
	return &EventRecord{
		PK:            fmt.Sprintf("EVENTS#%s", event.GetAggregateID()),
		SK:            fmt.Sprintf("EVENT#%s#%s", timestamp.Format(time.RFC3339Nano), eventID),
		EventID:       eventID,
		EventType:     event.GetEventType(),
		AggregateID:   event.GetAggregateID(),
		AggregateType: aggregateType,
		EventData:     eventData,
		Metadata:      make(map[string]interface{}), // Events don't have metadata method
		Timestamp:     timestamp.Format(time.RFC3339),
		Version:       event.GetVersion(),
		UserID:        userID,
		
		// Outbox pattern fields - events start as pending
		PublishStatus:   string(PublishStatusPending),
		PublishAttempts: 0,
		
		GSI1PK:        fmt.Sprintf("USER#%s", userID),
		GSI1SK:        fmt.Sprintf("EVENT#%s", timestamp.Format(time.RFC3339Nano)),
		GSI2PK:        fmt.Sprintf("EVENTTYPE#%s", event.GetEventType()),
		GSI2SK:        fmt.Sprintf("EVENT#%s", timestamp.Format(time.RFC3339Nano)),
		TTL:           ttl,
	}, nil
}

// recordToEvent converts a DynamoDB record back to a domain event
func (es *DynamoDBEventStore) recordToEvent(record EventRecord) (events.DomainEvent, error) {
	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, record.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	
	// Create a base event
	// In a real implementation, you would use a factory pattern
	// to create the specific event type based on EventType
	baseEvent := &events.BaseEvent{
		AggregateID: record.AggregateID,
		EventType:   record.EventType,
		Timestamp:   timestamp,
		Version:     record.Version,
	}
	
	// Depending on the event type, create the appropriate concrete event
	// This is a simplified version - in production, use a proper event factory
	switch record.EventType {
	case "node.created":
		// Parse node ID from event data
		nodeIDStr, _ := record.EventData["node_id"].(string)
		nodeID, _ := valueobjects.NewNodeIDFromString(nodeIDStr)
		userID, _ := record.EventData["user_id"].(string)
		
		return &events.NodeCreated{
			BaseEvent: *baseEvent,
			NodeID:    nodeID,
			UserID:    userID,
		}, nil
		
	case "node.archived":
		nodeIDStr, _ := record.EventData["node_id"].(string)
		nodeID, _ := valueobjects.NewNodeIDFromString(nodeIDStr)
		
		return &events.NodeArchived{
			BaseEvent: *baseEvent,
			NodeID:    nodeID,
		}, nil
		
	case "NodeDeleted":
		nodeIDStr, _ := record.EventData["node_id"].(string)
		nodeID, _ := valueobjects.NewNodeIDFromString(nodeIDStr)
		userID, _ := record.EventData["user_id"].(string)
		content, _ := record.EventData["content"].(string)
		
		// Handle keywords and tags arrays
		var keywords []string
		if kwInterface, ok := record.EventData["keywords"].([]interface{}); ok {
			for _, kw := range kwInterface {
				if str, ok := kw.(string); ok {
					keywords = append(keywords, str)
				}
			}
		}
		
		var tags []string
		if tagsInterface, ok := record.EventData["tags"].([]interface{}); ok {
			for _, tag := range tagsInterface {
				if str, ok := tag.(string); ok {
					tags = append(tags, str)
				}
			}
		}
		
		return &events.NodeDeletedEvent{
			BaseEvent: *baseEvent,
			NodeID:    nodeID,
			UserID:    userID,
			Content:   content,
			Keywords:  keywords,
			Tags:      tags,
		}, nil
		
	case "graph.created":
		graphID, _ := record.EventData["graph_id"].(string)
		userID, _ := record.EventData["user_id"].(string)
		name, _ := record.EventData["name"].(string)
		
		return &events.GraphCreated{
			BaseEvent: *baseEvent,
			GraphID:   graphID,
			UserID:    userID,
			Name:      name,
		}, nil
		
	default:
		// For unknown event types, return the base event
		// In production, you might want to handle this differently
		return baseEvent, nil
	}
}

// GetSnapshot retrieves the latest snapshot for an aggregate
func (es *DynamoDBEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*EventSnapshot, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(es.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("SNAPSHOT#%s", aggregateID)},
			"SK": &types.AttributeValueMemberS{Value: "LATEST"},
		},
	}
	
	result, err := es.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	
	if result.Item == nil {
		return nil, nil // No snapshot exists
	}
	
	var snapshot EventSnapshot
	if err := attributevalue.UnmarshalMap(result.Item, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	
	return &snapshot, nil
}

// SaveSnapshot saves a snapshot of an aggregate's state
func (es *DynamoDBEventStore) SaveSnapshot(ctx context.Context, snapshot *EventSnapshot) error {
	item, err := attributevalue.MarshalMap(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(es.tableName),
		Item:      item,
	}
	
	_, err = es.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	
	return nil
}

// EventSnapshot represents a snapshot of an aggregate's state
type EventSnapshot struct {
	PK            string                 `dynamodbav:"PK"`      // SNAPSHOT#<aggregate_id>
	SK            string                 `dynamodbav:"SK"`      // LATEST
	AggregateID   string                 `dynamodbav:"AggregateID"`
	AggregateType string                 `dynamodbav:"AggregateType"`
	Version       int                    `dynamodbav:"Version"`
	State         map[string]interface{} `dynamodbav:"State"`
	Timestamp     string                 `dynamodbav:"Timestamp"`
}

// Outbox Pattern Methods

// GetPendingEvents retrieves events that haven't been published yet
func (es *DynamoDBEventStore) GetPendingEvents(ctx context.Context, limit int32) ([]*EventRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	// Query events with PublishStatus = "pending"
	// We'll use a scan with filter for simplicity, but in production, consider adding a GSI
	input := &dynamodb.ScanInput{
		TableName:        aws.String(es.tableName),
		FilterExpression: aws.String("PublishStatus = :status AND begins_with(PK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(PublishStatusPending)},
			":prefix": &types.AttributeValueMemberS{Value: "EVENTS#"},
		},
		Limit: aws.Int32(limit),
	}

	result, err := es.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan pending events: %w", err)
	}

	records := make([]*EventRecord, 0, len(result.Items))
	for _, item := range result.Items {
		var record EventRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			continue // Skip malformed records
		}
		records = append(records, &record)
	}

	return records, nil
}

// MarkEventAsPublished marks an event as successfully published
func (es *DynamoDBEventStore) MarkEventAsPublished(ctx context.Context, eventPK, eventSK string) error {
	now := time.Now().Format(time.RFC3339)
	
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(es.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: eventPK},
			"SK": &types.AttributeValueMemberS{Value: eventSK},
		},
		UpdateExpression: aws.String("SET PublishStatus = :published, PublishedAt = :publishedAt"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":published":   &types.AttributeValueMemberS{Value: string(PublishStatusPublished)},
			":publishedAt": &types.AttributeValueMemberS{Value: now},
		},
		ConditionExpression: aws.String("attribute_exists(PK)"), // Ensure event exists
	}

	_, err := es.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to mark event as published: %w", err)
	}

	return nil
}

// MarkEventAsFailed marks an event as failed to publish with error details
func (es *DynamoDBEventStore) MarkEventAsFailed(ctx context.Context, eventPK, eventSK string, errorMsg string, attempts int) error {
	now := time.Now().Format(time.RFC3339)
	
	// Determine status based on attempt count
	status := string(PublishStatusFailed)
	if attempts < 3 { // Max 3 attempts before marking as permanently failed
		status = string(PublishStatusPending) // Keep as pending for retry
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(es.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: eventPK},
			"SK": &types.AttributeValueMemberS{Value: eventSK},
		},
		UpdateExpression: aws.String("SET PublishStatus = :status, PublishAttempts = :attempts, LastPublishTry = :lastTry, ErrorMessage = :error"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":    &types.AttributeValueMemberS{Value: status},
			":attempts":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", attempts)},
			":lastTry":   &types.AttributeValueMemberS{Value: now},
			":error":     &types.AttributeValueMemberS{Value: errorMsg},
		},
		ConditionExpression: aws.String("attribute_exists(PK)"), // Ensure event exists
	}

	_, err := es.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	return nil
}