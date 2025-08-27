// Package dynamodb provides a generic base repository implementation for DynamoDB operations.
//
// This file implements a reusable base repository pattern using Go generics to eliminate
// code duplication across entity-specific repositories. It provides:
//   - Standard CRUD operations (Create, Read, Update, Delete)
//   - Batch operations with automatic chunking
//   - Consistent error handling and retry logic
//   - Query and scan operations with pagination support
//   - Optimistic locking support
//
// The BaseRepository is designed to be embedded in specific repository implementations,
// providing a foundation of common functionality that can be extended or overridden.
package dynamodb

import (
	"context"
	"fmt"
	"time"

	errorContext "brain2-backend/internal/errors"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// ============================================================================
// INTERFACES AND CONSTRAINTS
// ============================================================================

// DomainEntity represents the contract that domain entities must fulfill
// to work with the BaseRepository.
type DomainEntity interface {
	// GetID returns the unique identifier of the entity
	GetID() string
	// GetUserID returns the user ID associated with the entity
	GetUserID() string
	// GetVersion returns the version number for optimistic locking
	Version() int
}

// EntityParser defines the interface for converting between DynamoDB items
// and domain entities.
type EntityParser[T DomainEntity] interface {
	// ToItem converts a domain entity to a DynamoDB item
	ToItem(entity T) (map[string]types.AttributeValue, error)
	// FromItem converts a DynamoDB item to a domain entity
	FromItem(item map[string]types.AttributeValue) (T, error)
}

// ============================================================================
// BASE REPOSITORY IMPLEMENTATION
// ============================================================================

// BaseRepository provides generic CRUD operations for DynamoDB entities.
// It uses Go generics to provide type-safe operations while eliminating
// code duplication across repositories.
type BaseRepository[T DomainEntity] struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
	parser    EntityParser[T]
	
	// Configuration options
	maxRetries     int
	batchSize      int
	queryBatchSize int
}

// BaseRepositoryConfig holds configuration options for BaseRepository
type BaseRepositoryConfig struct {
	MaxRetries     int
	BatchSize      int  // For batch write operations (max 25)
	QueryBatchSize int  // For batch get operations (max 100)
}

// NewBaseRepository creates a new instance of BaseRepository with the given configuration.
func NewBaseRepository[T DomainEntity](
	client *dynamodb.Client,
	tableName string,
	indexName string,
	logger *zap.Logger,
	parser EntityParser[T],
	config *BaseRepositoryConfig,
) *BaseRepository[T] {
	// Apply defaults if config not provided
	if config == nil {
		config = &BaseRepositoryConfig{
			MaxRetries:     3,
			BatchSize:      25,
			QueryBatchSize: 100,
		}
	}
	
	return &BaseRepository[T]{
		client:         client,
		tableName:      tableName,
		indexName:      indexName,
		logger:         logger,
		parser:         parser,
		maxRetries:     config.MaxRetries,
		batchSize:      config.BatchSize,
		queryBatchSize: config.QueryBatchSize,
	}
}

// ============================================================================
// SINGLE ITEM OPERATIONS
// ============================================================================

// GetItem retrieves a single item from DynamoDB using the provided key.
func (r *BaseRepository[T]) GetItem(ctx context.Context, key map[string]types.AttributeValue) (T, error) {
	var zero T
	
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return zero, errorContext.WrapWithContext(err, "DynamoDB GetItem failed")
	}
	
	if result.Item == nil {
		return zero, repository.ErrNodeNotFound
	}
	
	return r.parser.FromItem(result.Item)
}

// PutItem creates or replaces an item in DynamoDB.
func (r *BaseRepository[T]) PutItem(ctx context.Context, entity T) error {
	item, err := r.parser.ToItem(entity)
	if err != nil {
		return errorContext.WrapWithContext(err, "failed to marshal entity")
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}
	
	// Add condition for create-only operations (prevent overwrites)
	if r.shouldPreventOverwrite(entity) {
		input.ConditionExpression = aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)")
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return errorContext.WrapWithContext(err, "DynamoDB PutItem failed")
	}
	
	return nil
}

// UpdateItem updates an existing item in DynamoDB with optimistic locking.
func (r *BaseRepository[T]) UpdateItem(
	ctx context.Context,
	key map[string]types.AttributeValue,
	updateExpr expression.UpdateBuilder,
	conditionExpr *expression.ConditionBuilder,
) error {
	builder := expression.NewBuilder().WithUpdate(updateExpr)
	
	if conditionExpr != nil {
		builder = builder.WithCondition(*conditionExpr)
	}
	
	expr, err := builder.Build()
	if err != nil {
		return errorContext.WrapWithContext(err, "failed to build expression")
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	if conditionExpr != nil {
		input.ConditionExpression = expr.Condition()
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return errorContext.WrapWithContext(err, "DynamoDB UpdateItem failed")
	}
	
	return nil
}

// DeleteItem removes an item from DynamoDB.
func (r *BaseRepository[T]) DeleteItem(ctx context.Context, key map[string]types.AttributeValue) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return errorContext.WrapWithContext(err, "DynamoDB DeleteItem failed")
	}
	
	return nil
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// BatchGetItems retrieves multiple items in parallel using BatchGetItem.
// Automatically handles chunking (100 items max per request) and retries.
func (r *BaseRepository[T]) BatchGetItems(ctx context.Context, keys []map[string]types.AttributeValue) ([]T, error) {
	if len(keys) == 0 {
		return []T{}, nil
	}
	
	var results []T
	
	// Process in chunks
	for i := 0; i < len(keys); i += r.queryBatchSize {
		end := i + r.queryBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		
		chunk := keys[i:end]
		chunkResults, err := r.batchGetChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		
		results = append(results, chunkResults...)
	}
	
	return results, nil
}

// batchGetChunk processes a single chunk of keys
func (r *BaseRepository[T]) batchGetChunk(ctx context.Context, keys []map[string]types.AttributeValue) ([]T, error) {
	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			r.tableName: {
				Keys: keys,
			},
		},
	}
	
	var results []T
	retryCount := 0
	
	for {
		output, err := r.client.BatchGetItem(ctx, input)
		if err != nil {
			return nil, errorContext.WrapWithContext(err, "BatchGetItem failed")
		}
		
		// Process successful items
		for _, item := range output.Responses[r.tableName] {
			entity, err := r.parser.FromItem(item)
			if err != nil {
				r.logger.Warn("failed to parse item", zap.Error(err))
				continue
			}
			results = append(results, entity)
		}
		
		// Handle unprocessed keys with retry
		if len(output.UnprocessedKeys) == 0 || len(output.UnprocessedKeys[r.tableName].Keys) == 0 {
			break
		}
		
		if retryCount >= r.maxRetries {
			r.logger.Warn("max retries exceeded for batch get",
				zap.Int("unprocessed", len(output.UnprocessedKeys[r.tableName].Keys)))
			break
		}
		
		// Exponential backoff
		time.Sleep(time.Duration(1<<retryCount) * 100 * time.Millisecond)
		
		// Retry with unprocessed keys
		input.RequestItems = map[string]types.KeysAndAttributes{
			r.tableName: {
				Keys: output.UnprocessedKeys[r.tableName].Keys,
			},
		}
		retryCount++
	}
	
	return results, nil
}

// BatchWriteItems performs batch write operations (put/delete) with automatic chunking.
// Handles the DynamoDB limit of 25 items per batch write request.
func (r *BaseRepository[T]) BatchWriteItems(ctx context.Context, requests []types.WriteRequest) error {
	if len(requests) == 0 {
		return nil
	}
	
	// Process in chunks of 25 (DynamoDB limit)
	for i := 0; i < len(requests); i += r.batchSize {
		end := i + r.batchSize
		if end > len(requests) {
			end = len(requests)
		}
		
		chunk := requests[i:end]
		if err := r.batchWriteChunk(ctx, chunk); err != nil {
			return err
		}
	}
	
	return nil
}

// batchWriteChunk processes a single chunk of write requests
func (r *BaseRepository[T]) batchWriteChunk(ctx context.Context, requests []types.WriteRequest) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			r.tableName: requests,
		},
	}
	
	retryCount := 0
	
	for {
		output, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			return errorContext.WrapWithContext(err, "BatchWriteItem failed")
		}
		
		// Check for unprocessed items
		if len(output.UnprocessedItems) == 0 || len(output.UnprocessedItems[r.tableName]) == 0 {
			break
		}
		
		if retryCount >= r.maxRetries {
			r.logger.Error("max retries exceeded for batch write",
				zap.Int("unprocessed", len(output.UnprocessedItems[r.tableName])))
			return fmt.Errorf("failed to process all items after %d retries", r.maxRetries)
		}
		
		// Exponential backoff
		time.Sleep(time.Duration(1<<retryCount) * 100 * time.Millisecond)
		
		// Retry with unprocessed items
		input.RequestItems[r.tableName] = output.UnprocessedItems[r.tableName]
		retryCount++
	}
	
	return nil
}

// BatchPutItems saves multiple entities using batch write operations.
func (r *BaseRepository[T]) BatchPutItems(ctx context.Context, entities []T) error {
	var requests []types.WriteRequest
	
	for _, entity := range entities {
		item, err := r.parser.ToItem(entity)
		if err != nil {
			return errorContext.WrapWithContext(err, "failed to marshal entity")
		}
		
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}
	
	return r.BatchWriteItems(ctx, requests)
}

// BatchDeleteItems deletes multiple items using batch write operations.
func (r *BaseRepository[T]) BatchDeleteItems(ctx context.Context, keys []map[string]types.AttributeValue) error {
	var requests []types.WriteRequest
	
	for _, key := range keys {
		requests = append(requests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: key,
			},
		})
	}
	
	return r.BatchWriteItems(ctx, requests)
}

// ============================================================================
// QUERY AND SCAN OPERATIONS
// ============================================================================

// QueryItems performs a query operation with the given key condition.
func (r *BaseRepository[T]) QueryItems(
	ctx context.Context,
	keyCondition expression.KeyConditionBuilder,
	filterCondition *expression.ConditionBuilder,
	limit *int32,
	exclusiveStartKey map[string]types.AttributeValue,
) ([]T, map[string]types.AttributeValue, error) {
	builder := expression.NewBuilder().WithKeyCondition(keyCondition)
	
	if filterCondition != nil {
		builder = builder.WithFilter(*filterCondition)
	}
	
	expr, err := builder.Build()
	if err != nil {
		return nil, nil, errorContext.WrapWithContext(err, "failed to build expression")
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	if filterCondition != nil {
		input.FilterExpression = expr.Filter()
	}
	
	if limit != nil {
		input.Limit = limit
	}
	
	if exclusiveStartKey != nil {
		input.ExclusiveStartKey = exclusiveStartKey
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, nil, errorContext.WrapWithContext(err, "DynamoDB Query failed")
	}
	
	var entities []T
	for _, item := range result.Items {
		entity, err := r.parser.FromItem(item)
		if err != nil {
			r.logger.Warn("failed to parse item", zap.Error(err))
			continue
		}
		entities = append(entities, entity)
	}
	
	return entities, result.LastEvaluatedKey, nil
}

// QueryAllItems performs a query and automatically handles pagination to retrieve all results.
func (r *BaseRepository[T]) QueryAllItems(
	ctx context.Context,
	keyCondition expression.KeyConditionBuilder,
	filterCondition *expression.ConditionBuilder,
) ([]T, error) {
	var allEntities []T
	var lastEvaluatedKey map[string]types.AttributeValue
	
	for {
		entities, nextKey, err := r.QueryItems(ctx, keyCondition, filterCondition, nil, lastEvaluatedKey)
		if err != nil {
			return nil, err
		}
		
		allEntities = append(allEntities, entities...)
		
		if nextKey == nil {
			break
		}
		lastEvaluatedKey = nextKey
	}
	
	return allEntities, nil
}

// ScanItems performs a scan operation with optional filtering.
func (r *BaseRepository[T]) ScanItems(
	ctx context.Context,
	filterCondition *expression.ConditionBuilder,
	limit *int32,
	exclusiveStartKey map[string]types.AttributeValue,
) ([]T, map[string]types.AttributeValue, error) {
	builder := expression.NewBuilder()
	
	if filterCondition != nil {
		builder = builder.WithFilter(*filterCondition)
	}
	
	expr, err := builder.Build()
	if err != nil {
		return nil, nil, errorContext.WrapWithContext(err, "failed to build expression")
	}
	
	input := &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	}
	
	if filterCondition != nil {
		input.FilterExpression = expr.Filter()
		input.ExpressionAttributeNames = expr.Names()
		input.ExpressionAttributeValues = expr.Values()
	}
	
	if limit != nil {
		input.Limit = limit
	}
	
	if exclusiveStartKey != nil {
		input.ExclusiveStartKey = exclusiveStartKey
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, nil, errorContext.WrapWithContext(err, "DynamoDB Scan failed")
	}
	
	var entities []T
	for _, item := range result.Items {
		entity, err := r.parser.FromItem(item)
		if err != nil {
			r.logger.Warn("failed to parse item", zap.Error(err))
			continue
		}
		entities = append(entities, entity)
	}
	
	return entities, result.LastEvaluatedKey, nil
}

// ============================================================================
// TRANSACTION OPERATIONS
// ============================================================================

// TransactWriteItems performs multiple write operations atomically.
func (r *BaseRepository[T]) TransactWriteItems(ctx context.Context, items []types.TransactWriteItem) error {
	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	}
	
	_, err := r.client.TransactWriteItems(ctx, input)
	if err != nil {
		return errorContext.WrapWithContext(err, "TransactWriteItems failed")
	}
	
	return nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// shouldPreventOverwrite determines if we should prevent overwriting existing items.
// This is typically used for create operations to ensure uniqueness.
func (r *BaseRepository[T]) shouldPreventOverwrite(entity T) bool {
	// Only prevent overwrites for new entities (version 0 or 1)
	return entity.Version() <= 1
}

// BuildKey constructs a DynamoDB key from user ID and entity ID.
// This is a common pattern for single table design.
func (r *BaseRepository[T]) BuildKey(userID, entityID, entityType string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": StringAttr(BuildUserPK(userID)),
		"SK": StringAttr(fmt.Sprintf("%s#%s", entityType, entityID)),
	}
}

// BuildOptimisticLockCondition creates a condition expression for optimistic locking.
func (r *BaseRepository[T]) BuildOptimisticLockCondition(expectedVersion int) expression.ConditionBuilder {
	return expression.Equal(expression.Name("Version"), expression.Value(expectedVersion))
}

// GetTableName returns the table name for this repository.
func (r *BaseRepository[T]) GetTableName() string {
	return r.tableName
}

// GetClient returns the DynamoDB client for advanced operations.
func (r *BaseRepository[T]) GetClient() *dynamodb.Client {
	return r.client
}

// GetLogger returns the logger instance.
func (r *BaseRepository[T]) GetLogger() *zap.Logger {
	return r.logger
}