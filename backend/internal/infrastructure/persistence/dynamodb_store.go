package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// DynamoDBStore implements the Store interface for AWS DynamoDB.
type DynamoDBStore struct {
	client *dynamodb.Client
	config StoreConfig
	logger *zap.Logger
	metrics *StoreMetrics
}

// NewDynamoDBStore creates a new DynamoDB store implementation.
func NewDynamoDBStore(client *dynamodb.Client, config StoreConfig, logger *zap.Logger) *DynamoDBStore {
	return &DynamoDBStore{
		client: client,
		config: config,
		logger: logger,
		metrics: &StoreMetrics{
			OperationCount: make(map[string]int64),
			LatencyMs:      make(map[string]int64),
			ErrorCount:     make(map[string]int64),
			ConnectionStatus: "connected",
		},
	}
}

// Get retrieves a single record by key.
func (s *DynamoDBStore) Get(ctx context.Context, key Key) (*Record, error) {
	start := time.Now()
	defer s.recordMetrics("Get", start, nil)

	// Build DynamoDB key
	dynamoKey := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: key.PartitionKey},
		"SK": &types.AttributeValueMemberS{Value: key.SortKey},
	}

	// Execute GetItem
	input := &dynamodb.GetItemInput{
		TableName:      aws.String(s.config.TableName),
		Key:            dynamoKey,
		ConsistentRead: aws.Bool(s.config.ConsistentRead),
	}

	result, err := s.client.GetItem(ctx, input)
	if err != nil {
		s.recordMetrics("Get", start, err)
		return nil, fmt.Errorf("DynamoDB GetItem failed: %w", err)
	}

	if result.Item == nil {
		return nil, nil // Record not found
	}

	// Convert DynamoDB item to Record
	record, err := s.dynamoItemToRecord(result.Item)
	if err != nil {
		s.recordMetrics("Get", start, err)
		return nil, fmt.Errorf("failed to convert DynamoDB item: %w", err)
	}

	s.logger.Debug("retrieved record",
		zap.String("partition_key", key.PartitionKey),
		zap.String("sort_key", key.SortKey))

	return record, nil
}

// Put stores a record.
func (s *DynamoDBStore) Put(ctx context.Context, record Record) error {
	start := time.Now()
	defer s.recordMetrics("Put", start, nil)

	// Convert Record to DynamoDB item
	item, err := s.recordToDynamoItem(record)
	if err != nil {
		s.recordMetrics("Put", start, err)
		return fmt.Errorf("failed to convert record to DynamoDB item: %w", err)
	}

	// Execute PutItem
	input := &dynamodb.PutItemInput{
		TableName: aws.String(s.config.TableName),
		Item:      item,
	}

	_, err = s.client.PutItem(ctx, input)
	if err != nil {
		s.recordMetrics("Put", start, err)
		return fmt.Errorf("DynamoDB PutItem failed: %w", err)
	}

	s.logger.Debug("stored record",
		zap.String("partition_key", record.Key.PartitionKey),
		zap.String("sort_key", record.Key.SortKey))

	return nil
}

// Delete removes a record by key.
func (s *DynamoDBStore) Delete(ctx context.Context, key Key) error {
	start := time.Now()
	defer s.recordMetrics("Delete", start, nil)

	// Build DynamoDB key
	dynamoKey := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: key.PartitionKey},
		"SK": &types.AttributeValueMemberS{Value: key.SortKey},
	}

	// Execute DeleteItem
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(s.config.TableName),
		Key:       dynamoKey,
	}

	_, err := s.client.DeleteItem(ctx, input)
	if err != nil {
		s.recordMetrics("Delete", start, err)
		return fmt.Errorf("DynamoDB DeleteItem failed: %w", err)
	}

	s.logger.Debug("deleted record",
		zap.String("partition_key", key.PartitionKey),
		zap.String("sort_key", key.SortKey))

	return nil
}

// Update modifies a record with the given updates.
func (s *DynamoDBStore) Update(ctx context.Context, key Key, updates map[string]interface{}, conditionExpr *string) error {
	start := time.Now()
	defer s.recordMetrics("Update", start, nil)

	// Build DynamoDB key
	dynamoKey := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: key.PartitionKey},
		"SK": &types.AttributeValueMemberS{Value: key.SortKey},
	}

	// Build update expression
	updateExpr, exprAttrNames, exprAttrValues, err := s.buildUpdateExpression(updates)
	if err != nil {
		s.recordMetrics("Update", start, err)
		return fmt.Errorf("failed to build update expression: %w", err)
	}

	// Execute UpdateItem
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.config.TableName),
		Key:                       dynamoKey,
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
	}

	if conditionExpr != nil {
		input.ConditionExpression = conditionExpr
	}

	_, err = s.client.UpdateItem(ctx, input)
	if err != nil {
		s.recordMetrics("Update", start, err)
		return fmt.Errorf("DynamoDB UpdateItem failed: %w", err)
	}

	s.logger.Debug("updated record",
		zap.String("partition_key", key.PartitionKey),
		zap.String("sort_key", key.SortKey))

	return nil
}

// Query performs a query operation.
func (s *DynamoDBStore) Query(ctx context.Context, query Query) (*QueryResult, error) {
	start := time.Now()
	defer s.recordMetrics("Query", start, nil)

	// Build query input
	input := &dynamodb.QueryInput{
		TableName:      aws.String(s.config.TableName),
		ConsistentRead: aws.Bool(s.config.ConsistentRead),
	}

	// Handle different index types with appropriate key names
	if query.IndexName != nil && *query.IndexName == "EdgeIndex" {
		// EdgeIndex uses GSI2PK/GSI2SK
		input.IndexName = query.IndexName
		input.KeyConditionExpression = aws.String("GSI2PK = :pk")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: query.PartitionKey},
		}
		
		// Add sort key condition if specified for GSI2
		if query.SortKeyPrefix != nil {
			input.KeyConditionExpression = aws.String("GSI2PK = :pk AND begins_with(GSI2SK, :sk)")
			input.ExpressionAttributeValues[":sk"] = &types.AttributeValueMemberS{Value: *query.SortKeyPrefix}
		}
	} else {
		// Main table or other indexes use PK/SK
		input.KeyConditionExpression = aws.String("PK = :pk")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: query.PartitionKey},
		}
		
		// Add sort key condition if specified for main table
		if query.SortKeyPrefix != nil {
			input.KeyConditionExpression = aws.String("PK = :pk AND begins_with(SK, :sk)")
			input.ExpressionAttributeValues[":sk"] = &types.AttributeValueMemberS{Value: *query.SortKeyPrefix}
		}
		
		// Add index if specified (for non-EdgeIndex GSIs)
		if query.IndexName != nil {
			input.IndexName = query.IndexName
		}
	}

	// Add filter expression if specified
	if query.FilterExpr != nil {
		input.FilterExpression = query.FilterExpr
	}

	// Add limit if specified
	if query.Limit != nil {
		input.Limit = query.Limit
	}

	// Add pagination if specified
	if query.LastEvaluated != nil {
		lastKey, err := attributevalue.MarshalMap(query.LastEvaluated)
		if err != nil {
			s.recordMetrics("Query", start, err)
			return nil, fmt.Errorf("failed to marshal last evaluated key: %w", err)
		}
		input.ExclusiveStartKey = lastKey
	}

	// Execute query
	result, err := s.client.Query(ctx, input)
	if err != nil {
		s.recordMetrics("Query", start, err)
		return nil, fmt.Errorf("DynamoDB Query failed: %w", err)
	}

	// Convert results
	records := make([]Record, 0, len(result.Items))
	for _, item := range result.Items {
		record, err := s.dynamoItemToRecord(item)
		if err != nil {
			s.logger.Warn("failed to convert query result item", zap.Error(err))
			continue
		}
		records = append(records, *record)
	}

	// Build query result
	queryResult := &QueryResult{
		Records:      records,
		Count:        result.Count,
		ScannedCount: result.ScannedCount,
	}

	// Add last evaluated key for pagination
	if result.LastEvaluatedKey != nil {
		var lastEval map[string]interface{}
		err := attributevalue.UnmarshalMap(result.LastEvaluatedKey, &lastEval)
		if err != nil {
			s.logger.Warn("failed to unmarshal last evaluated key", zap.Error(err))
		} else {
			queryResult.LastEvaluated = lastEval
		}
	}

	s.logger.Debug("query completed",
		zap.String("partition_key", query.PartitionKey),
		zap.Int32("count", result.Count))

	return queryResult, nil
}

// Scan performs a scan operation.
func (s *DynamoDBStore) Scan(ctx context.Context, query Query) (*QueryResult, error) {
	start := time.Now()
	defer s.recordMetrics("Scan", start, nil)

	// Build scan input
	input := &dynamodb.ScanInput{
		TableName:      aws.String(s.config.TableName),
		ConsistentRead: aws.Bool(s.config.ConsistentRead),
	}

	// Add filter expression if specified
	if query.FilterExpr != nil {
		input.FilterExpression = query.FilterExpr
		
		// Add expression attribute values if specified
		if len(query.Attributes) > 0 {
			exprAttrValues := make(map[string]types.AttributeValue)
			for key, value := range query.Attributes {
				attrValue, err := attributevalue.Marshal(value)
				if err != nil {
					s.recordMetrics("Scan", start, err)
					return nil, fmt.Errorf("failed to marshal attribute value for key %s: %w", key, err)
				}
				exprAttrValues[key] = attrValue
			}
			input.ExpressionAttributeValues = exprAttrValues
		}
	}

	// Add index if specified
	if query.IndexName != nil {
		input.IndexName = query.IndexName
	}

	// Add limit if specified
	if query.Limit != nil {
		input.Limit = query.Limit
	}

	// Execute scan
	result, err := s.client.Scan(ctx, input)
	if err != nil {
		s.recordMetrics("Scan", start, err)
		return nil, fmt.Errorf("DynamoDB Scan failed: %w", err)
	}

	// Convert results
	records := make([]Record, 0, len(result.Items))
	for _, item := range result.Items {
		record, err := s.dynamoItemToRecord(item)
		if err != nil {
			s.logger.Warn("failed to convert scan result item", zap.Error(err))
			continue
		}
		records = append(records, *record)
	}

	return &QueryResult{
		Records:      records,
		Count:        result.Count,
		ScannedCount: result.ScannedCount,
	}, nil
}

// BatchGet retrieves multiple records by keys.
func (s *DynamoDBStore) BatchGet(ctx context.Context, keys []Key) ([]Record, error) {
	start := time.Now()
	defer s.recordMetrics("BatchGet", start, nil)

	// Build keys
	dynamoKeys := make([]map[string]types.AttributeValue, len(keys))
	for i, key := range keys {
		dynamoKeys[i] = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: key.PartitionKey},
			"SK": &types.AttributeValueMemberS{Value: key.SortKey},
		}
	}

	// Execute BatchGetItem
	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			s.config.TableName: {
				Keys:           dynamoKeys,
				ConsistentRead: aws.Bool(s.config.ConsistentRead),
			},
		},
	}

	result, err := s.client.BatchGetItem(ctx, input)
	if err != nil {
		s.recordMetrics("BatchGet", start, err)
		return nil, fmt.Errorf("DynamoDB BatchGetItem failed: %w", err)
	}

	// Convert results
	items := result.Responses[s.config.TableName]
	records := make([]Record, 0, len(items))
	for _, item := range items {
		record, err := s.dynamoItemToRecord(item)
		if err != nil {
			s.logger.Warn("failed to convert batch get result item", zap.Error(err))
			continue
		}
		records = append(records, *record)
	}

	return records, nil
}

// BatchPut stores multiple records.
func (s *DynamoDBStore) BatchPut(ctx context.Context, records []Record) error {
	start := time.Now()
	defer s.recordMetrics("BatchPut", start, nil)

	// Convert records to write requests
	writeRequests := make([]types.WriteRequest, len(records))
	for i, record := range records {
		item, err := s.recordToDynamoItem(record)
		if err != nil {
			s.recordMetrics("BatchPut", start, err)
			return fmt.Errorf("failed to convert record to DynamoDB item: %w", err)
		}

		writeRequests[i] = types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		}
	}

	// Execute BatchWriteItem
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			s.config.TableName: writeRequests,
		},
	}

	_, err := s.client.BatchWriteItem(ctx, input)
	if err != nil {
		s.recordMetrics("BatchPut", start, err)
		return fmt.Errorf("DynamoDB BatchWriteItem failed: %w", err)
	}

	return nil
}

// BatchDelete removes multiple records by keys.
func (s *DynamoDBStore) BatchDelete(ctx context.Context, keys []Key) error {
	start := time.Now()
	defer s.recordMetrics("BatchDelete", start, nil)

	// Convert keys to delete requests
	writeRequests := make([]types.WriteRequest, len(keys))
	for i, key := range keys {
		writeRequests[i] = types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: key.PartitionKey},
					"SK": &types.AttributeValueMemberS{Value: key.SortKey},
				},
			},
		}
	}

	// Execute BatchWriteItem
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			s.config.TableName: writeRequests,
		},
	}

	_, err := s.client.BatchWriteItem(ctx, input)
	if err != nil {
		s.recordMetrics("BatchDelete", start, err)
		return fmt.Errorf("DynamoDB BatchWriteItem failed: %w", err)
	}

	return nil
}

// Transaction executes multiple operations atomically.
func (s *DynamoDBStore) Transaction(ctx context.Context, operations []Operation) error {
	start := time.Now()
	defer s.recordMetrics("Transaction", start, nil)

	// Convert operations to transact items
	transactItems := make([]types.TransactWriteItem, len(operations))
	for i, op := range operations {
		switch op.Type {
		case OperationTypePut:
			item, err := attributevalue.MarshalMap(op.Data)
			if err != nil {
				s.recordMetrics("Transaction", start, err)
				return fmt.Errorf("failed to marshal transaction item: %w", err)
			}
			transactItems[i] = types.TransactWriteItem{
				Put: &types.Put{
					TableName: aws.String(s.config.TableName),
					Item:      item,
				},
			}
		case OperationTypeDelete:
			key := map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: op.Key.PartitionKey},
				"SK": &types.AttributeValueMemberS{Value: op.Key.SortKey},
			}
			transactItems[i] = types.TransactWriteItem{
				Delete: &types.Delete{
					TableName: aws.String(s.config.TableName),
					Key:       key,
				},
			}
		default:
			return fmt.Errorf("unsupported operation type: %s", op.Type)
		}

		// Add condition expression if specified
		if op.ConditionExpr != nil {
			switch op.Type {
			case OperationTypePut:
				transactItems[i].Put.ConditionExpression = op.ConditionExpr
			case OperationTypeDelete:
				transactItems[i].Delete.ConditionExpression = op.ConditionExpr
			}
		}
	}

	// Execute transaction
	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	}

	_, err := s.client.TransactWriteItems(ctx, input)
	if err != nil {
		s.recordMetrics("Transaction", start, err)
		return fmt.Errorf("DynamoDB TransactWriteItems failed: %w", err)
	}

	s.logger.Debug("transaction completed",
		zap.Int("operation_count", len(operations)))

	return nil
}

// HealthCheck verifies the store is accessible.
func (s *DynamoDBStore) HealthCheck(ctx context.Context) error {
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.config.TableName),
	})
	if err != nil {
		s.metrics.ConnectionStatus = "disconnected"
		return fmt.Errorf("DynamoDB health check failed: %w", err)
	}
	s.metrics.ConnectionStatus = "connected"
	return nil
}

// GetStatistics returns store usage statistics.
func (s *DynamoDBStore) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"operation_counts": s.metrics.OperationCount,
		"latency_ms":       s.metrics.LatencyMs,
		"error_counts":     s.metrics.ErrorCount,
		"last_operation":   s.metrics.LastOperation,
		"connection_status": s.metrics.ConnectionStatus,
	}, nil
}

// Helper methods

func (s *DynamoDBStore) dynamoItemToRecord(item map[string]types.AttributeValue) (*Record, error) {
	// Extract key components
	pk, ok := item["PK"].(*types.AttributeValueMemberS)
	if !ok {
		return nil, fmt.Errorf("missing or invalid PK attribute")
	}

	sk, ok := item["SK"].(*types.AttributeValueMemberS)
	if !ok {
		return nil, fmt.Errorf("missing or invalid SK attribute")
	}

	// Convert to map
	data := make(map[string]interface{})
	err := attributevalue.UnmarshalMap(item, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal DynamoDB item: %w", err)
	}

	// Extract timestamps
	var createdAt, updatedAt time.Time
	if ts, ok := data["CreatedAt"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, ts)
	}
	if ts, ok := data["UpdatedAt"].(string); ok {
		updatedAt, _ = time.Parse(time.RFC3339, ts)
	}

	// Extract version
	var version int64
	if v, ok := data["Version"].(int); ok {
		version = int64(v)
	}

	return &Record{
		Key: Key{
			PartitionKey: pk.Value,
			SortKey:      sk.Value,
		},
		Data:      data,
		Version:   version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (s *DynamoDBStore) recordToDynamoItem(record Record) (map[string]types.AttributeValue, error) {
	// Add key to data
	data := make(map[string]interface{})
	for k, v := range record.Data {
		data[k] = v
	}

	data["PK"] = record.Key.PartitionKey
	data["SK"] = record.Key.SortKey
	data["Version"] = record.Version
	data["CreatedAt"] = record.CreatedAt.Format(time.RFC3339)
	data["UpdatedAt"] = record.UpdatedAt.Format(time.RFC3339)

	// Convert to DynamoDB item
	item, err := attributevalue.MarshalMap(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record to DynamoDB item: %w", err)
	}

	return item, nil
}

func (s *DynamoDBStore) buildUpdateExpression(updates map[string]interface{}) (string, map[string]string, map[string]types.AttributeValue, error) {
	expr := "SET "
	attrNames := make(map[string]string)
	attrValues := make(map[string]types.AttributeValue)

	i := 0
	for key, value := range updates {
		if i > 0 {
			expr += ", "
		}

		attrNameKey := "#" + key
		attrValueKey := ":" + key

		attrNames[attrNameKey] = key
		attrVal, err := attributevalue.Marshal(value)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to marshal attribute value: %w", err)
		}
		attrValues[attrValueKey] = attrVal

		expr += attrNameKey + " = " + attrValueKey
		i++
	}

	return expr, attrNames, attrValues, nil
}

func (s *DynamoDBStore) recordMetrics(operation string, start time.Time, err error) {
	duration := time.Since(start).Milliseconds()
	s.metrics.OperationCount[operation]++
	s.metrics.LatencyMs[operation] = duration
	s.metrics.LastOperation = time.Now()

	if err != nil {
		s.metrics.ErrorCount[operation]++
	}
}