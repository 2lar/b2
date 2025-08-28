// Package dynamodb provides DynamoDB implementation of the Outbox Store.
package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"brain2-backend/internal/domain/events"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ============================================================================
// DYNAMODB OUTBOX STORE - Transactional outbox for guaranteed delivery
// ============================================================================

// DynamoDBOutboxStore implements OutboxStore using DynamoDB.
type DynamoDBOutboxStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBOutboxStore creates a new DynamoDB outbox store.
func NewDynamoDBOutboxStore(client *dynamodb.Client, tableName string) events.OutboxStore {
	return &DynamoDBOutboxStore{
		client:    client,
		tableName: tableName,
	}
}

// outboxRecord represents an outbox entry in DynamoDB.
type outboxRecord struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	EventID     string `dynamodbav:"EventID"`
	EventType   string `dynamodbav:"EventType"`
	Payload     string `dynamodbav:"Payload"`
	Status      string `dynamodbav:"Status"`
	Error       string `dynamodbav:"Error,omitempty"`
	RetryCount  int    `dynamodbav:"RetryCount"`
	CreatedAt   int64  `dynamodbav:"CreatedAt"`
	PublishedAt int64  `dynamodbav:"PublishedAt,omitempty"`
	LastRetryAt int64  `dynamodbav:"LastRetryAt,omitempty"`
	TTL         int64  `dynamodbav:"TTL"`
	GSI1PK      string `dynamodbav:"GSI1PK"` // For status queries
	GSI1SK      string `dynamodbav:"GSI1SK"` // For timestamp ordering
}

// Save saves an entry to the outbox.
func (s *DynamoDBOutboxStore) Save(ctx context.Context, entry *events.OutboxEntry) error {
	// Serialize payload
	payloadData, err := serializeEvent(entry.Payload)
	if err != nil {
		return errors.Internal(errors.CodeInternalError.String(), "failed to serialize event payload").WithCause(err).Build()
	}
	
	// Create record
	record := outboxRecord{
		PK:         fmt.Sprintf("OUTBOX#%s", entry.EventID),
		SK:         "EVENT",
		EventID:    entry.EventID,
		EventType:  entry.EventType,
		Payload:    payloadData,
		Status:     string(entry.Status),
		Error:      entry.Error,
		RetryCount: entry.RetryCount,
		CreatedAt:  entry.CreatedAt.Unix(),
		TTL:        time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days retention
		GSI1PK:     fmt.Sprintf("STATUS#%s", entry.Status),
		GSI1SK:     fmt.Sprintf("%d#%s", entry.CreatedAt.Unix(), entry.EventID),
	}
	
	if entry.PublishedAt != nil {
		record.PublishedAt = entry.PublishedAt.Unix()
	}
	if entry.LastRetryAt != nil {
		record.LastRetryAt = entry.LastRetryAt.Unix()
	}
	
	// Marshal to DynamoDB format
	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return errors.Internal(errors.CodeInternalError.String(), "failed to marshal outbox record").WithCause(err).Build()
	}
	
	// Put item
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	
	if err != nil {
		return errors.Internal(errors.CodeInternalError.String(), "failed to save outbox entry").WithCause(err).Build()
	}
	
	return nil
}

// SaveBatch saves multiple entries atomically.
func (s *DynamoDBOutboxStore) SaveBatch(ctx context.Context, entries []*events.OutboxEntry) error {
	if len(entries) == 0 {
		return nil
	}
	
	// Create write requests
	writeRequests := make([]types.WriteRequest, 0, len(entries))
	
	for _, entry := range entries {
		// Serialize payload
		payloadData, err := serializeEvent(entry.Payload)
		if err != nil {
			return errors.Internal(errors.CodeInternalError.String(), "failed to serialize event payload").WithCause(err).Build()
		}
		
		// Create record
		record := outboxRecord{
			PK:         fmt.Sprintf("OUTBOX#%s", entry.EventID),
			SK:         "EVENT",
			EventID:    entry.EventID,
			EventType:  entry.EventType,
			Payload:    payloadData,
			Status:     string(entry.Status),
			Error:      entry.Error,
			RetryCount: entry.RetryCount,
			CreatedAt:  entry.CreatedAt.Unix(),
			TTL:        time.Now().Add(7 * 24 * time.Hour).Unix(),
			GSI1PK:     fmt.Sprintf("STATUS#%s", entry.Status),
			GSI1SK:     fmt.Sprintf("%d#%s", entry.CreatedAt.Unix(), entry.EventID),
		}
		
		// Marshal to DynamoDB format
		item, err := attributevalue.MarshalMap(record)
		if err != nil {
			return errors.Internal(errors.CodeInternalError.String(), "failed to marshal outbox record").WithCause(err).Build()
		}
		
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}
	
	// Batch write (max 25 items per batch)
	for i := 0; i < len(writeRequests); i += 25 {
		end := i + 25
		if end > len(writeRequests) {
			end = len(writeRequests)
		}
		
		batch := writeRequests[i:end]
		_, err := s.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				s.tableName: batch,
			},
		})
		
		if err != nil {
			return errors.Internal(errors.CodeInternalError.String(), "failed to batch save outbox entries").WithCause(err).Build()
		}
	}
	
	return nil
}

// Update updates an existing entry.
func (s *DynamoDBOutboxStore) Update(ctx context.Context, entry *events.OutboxEntry) error {
	// Build update expression
	updateExpr := "SET #status = :status, RetryCount = :retry_count, GSI1PK = :gsi1pk, GSI1SK = :gsi1sk"
	exprAttrNames := map[string]string{
		"#status": "Status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status":      &types.AttributeValueMemberS{Value: string(entry.Status)},
		":retry_count": &types.AttributeValueMemberN{Value: strconv.Itoa(entry.RetryCount)},
		":gsi1pk":      &types.AttributeValueMemberS{Value: fmt.Sprintf("STATUS#%s", entry.Status)},
		":gsi1sk":      &types.AttributeValueMemberS{Value: fmt.Sprintf("%d#%s", entry.CreatedAt.Unix(), entry.EventID)},
	}
	
	// Add optional fields
	if entry.Error != "" {
		updateExpr += ", #error = :error"
		exprAttrNames["#error"] = "Error"
		exprAttrValues[":error"] = &types.AttributeValueMemberS{Value: entry.Error}
	}
	
	if entry.PublishedAt != nil {
		updateExpr += ", PublishedAt = :published_at"
		exprAttrValues[":published_at"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(entry.PublishedAt.Unix(), 10)}
	}
	
	if entry.LastRetryAt != nil {
		updateExpr += ", LastRetryAt = :last_retry_at"
		exprAttrValues[":last_retry_at"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(entry.LastRetryAt.Unix(), 10)}
	}
	
	// Update item
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("OUTBOX#%s", entry.EventID)},
			"SK": &types.AttributeValueMemberS{Value: "EVENT"},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
	})
	
	if err != nil {
		return errors.Internal(errors.CodeInternalError.String(), "failed to update outbox entry").WithCause(err).Build()
	}
	
	return nil
}

// GetPending gets pending entries.
func (s *DynamoDBOutboxStore) GetPending(ctx context.Context, limit int) ([]*events.OutboxEntry, error) {
	// Query using GSI for PENDING status
	result, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("GSI1PK = :status"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "STATUS#PENDING"},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // Oldest first
	})
	
	if err != nil {
		return nil, errors.Internal(errors.CodeInternalError.String(), "failed to query pending outbox entries").WithCause(err).Build()
	}
	
	// Convert to entries
	entries := make([]*events.OutboxEntry, 0, len(result.Items))
	for _, item := range result.Items {
		var record outboxRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, errors.Internal(errors.CodeInternalError.String(), "failed to unmarshal outbox record").WithCause(err).Build()
		}
		
		entry, err := s.recordToEntry(record)
		if err != nil {
			return nil, err
		}
		
		entries = append(entries, entry)
	}
	
	return entries, nil
}

// GetFailed gets failed entries.
func (s *DynamoDBOutboxStore) GetFailed(ctx context.Context, limit int) ([]*events.OutboxEntry, error) {
	// Query using GSI for FAILED status
	result, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("GSI1PK = :status"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "STATUS#FAILED"},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // Oldest first
	})
	
	if err != nil {
		return nil, errors.Internal(errors.CodeInternalError.String(), "failed to query failed outbox entries").WithCause(err).Build()
	}
	
	// Convert to entries
	entries := make([]*events.OutboxEntry, 0, len(result.Items))
	for _, item := range result.Items {
		var record outboxRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, errors.Internal(errors.CodeInternalError.String(), "failed to unmarshal outbox record").WithCause(err).Build()
		}
		
		entry, err := s.recordToEntry(record)
		if err != nil {
			return nil, err
		}
		
		entries = append(entries, entry)
	}
	
	return entries, nil
}

// CleanupPublished removes old published entries.
func (s *DynamoDBOutboxStore) CleanupPublished(ctx context.Context, olderThan time.Time) error {
	// Query for published entries older than the specified time
	result, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("GSI1PK = :status AND GSI1SK < :timestamp"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":    &types.AttributeValueMemberS{Value: "STATUS#PUBLISHED"},
			":timestamp": &types.AttributeValueMemberS{Value: fmt.Sprintf("%d", olderThan.Unix())},
		},
		Limit: aws.Int32(100), // Process in batches
	})
	
	if err != nil {
		return errors.Internal(errors.CodeInternalError.String(), "failed to query old published entries").WithCause(err).Build()
	}
	
	// Delete entries
	for _, item := range result.Items {
		pk := item["PK"].(*types.AttributeValueMemberS).Value
		sk := item["SK"].(*types.AttributeValueMemberS).Value
		
		_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
		})
		
		if err != nil {
			// Log error but continue cleanup
			continue
		}
	}
	
	return nil
}

// recordToEntry converts a DynamoDB record to an OutboxEntry.
func (s *DynamoDBOutboxStore) recordToEntry(record outboxRecord) (*events.OutboxEntry, error) {
	// Deserialize payload
	event, err := deserializeEvent(record.Payload)
	if err != nil {
		return nil, errors.Internal(errors.CodeInternalError.String(), "failed to deserialize event").WithCause(err).Build()
	}
	
	entry := &events.OutboxEntry{
		EventID:    record.EventID,
		EventType:  record.EventType,
		Payload:    event,
		Status:     events.OutboxStatus(record.Status),
		Error:      record.Error,
		RetryCount: record.RetryCount,
		CreatedAt:  time.Unix(record.CreatedAt, 0),
	}
	
	if record.PublishedAt != 0 {
		t := time.Unix(record.PublishedAt, 0)
		entry.PublishedAt = &t
	}
	
	if record.LastRetryAt != 0 {
		t := time.Unix(record.LastRetryAt, 0)
		entry.LastRetryAt = &t
	}
	
	return entry, nil
}

// serializeEvent serializes a domain event to JSON.
func serializeEvent(event shared.DomainEvent) (string, error) {
	// This is a simplified serialization
	// In production, you'd want a more sophisticated serialization strategy
	data := map[string]interface{}{
		"eventID":     event.EventID(),
		"eventType":   event.EventType(),
		"aggregateID": event.AggregateID(),
		"userID":      event.UserID(),
		"timestamp":   event.Timestamp().Format(time.RFC3339),
		"data":        event.EventData(),
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	
	return string(jsonData), nil
}

// deserializeEvent deserializes a JSON event back to a domain event.
func deserializeEvent(jsonData string) (shared.DomainEvent, error) {
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &eventData); err != nil {
		return nil, err
	}
	
	timestamp, _ := time.Parse(time.RFC3339, eventData["timestamp"].(string))
	
	// Type assert data field
	var payloadData map[string]interface{}
	if d, ok := eventData["data"].(map[string]interface{}); ok {
		payloadData = d
	}
	
	// Return a generic event - in production, you'd use a factory
	return &GenericDomainEvent{
		eventID:       eventData["eventID"].(string),
		eventType:     eventData["eventType"].(string),
		aggregateID:   eventData["aggregateID"].(string),
		aggregateType: "", // Would be in eventData
		userID:        eventData["userID"].(string),
		timestamp:     timestamp,
		version:       0, // Would be in eventData
		data:          payloadData,
	}, nil
}