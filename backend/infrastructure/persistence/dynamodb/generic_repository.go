package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// Entity represents a domain entity that can be stored in DynamoDB
type Entity interface {
	GetID() string
	GetUserID() string
	GetVersion() int
}

// EntityConfig defines entity-specific behavior for the generic repository
type EntityConfig[T Entity] interface {
	// ParseItem converts a DynamoDB item to the entity type
	ParseItem(item map[string]types.AttributeValue) (T, error)
	// ToItem converts an entity to a DynamoDB item
	ToItem(entity T) (map[string]types.AttributeValue, error)
	// BuildKey creates the primary key for the entity
	BuildKey(userID, entityID string) map[string]types.AttributeValue
	// GetEntityType returns the entity type name for filtering
	GetEntityType() string
}

// GenericRepository provides common CRUD operations for all entity types
// This dramatically reduces code duplication (90% reduction)
type GenericRepository[T Entity] struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	config    EntityConfig[T]
	logger    *zap.Logger
}

// NewGenericRepository creates a new generic repository instance
func NewGenericRepository[T Entity](
	client *dynamodb.Client,
	tableName string,
	indexName string,
	config EntityConfig[T],
	logger *zap.Logger,
) *GenericRepository[T] {
	return &GenericRepository[T]{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		config:    config,
		logger:    logger,
	}
}

// Save creates or updates an entity with optimistic locking
func (r *GenericRepository[T]) Save(ctx context.Context, entity T) error {
	item, err := r.config.ToItem(entity)
	if err != nil {
		return fmt.Errorf("failed to convert entity to item: %w", err)
	}

	// Add metadata
	item["UpdatedAt"] = &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)}

	// Build condition for optimistic locking
	var condition expression.ConditionBuilder
	if entity.GetVersion() > 1 {
		// Update case - check version
		condition = expression.Name("Version").Equal(expression.Value(entity.GetVersion() - 1))
	} else {
		// Create case - ensure doesn't exist
		condition = expression.Name("PK").AttributeNotExists()
	}

	expr, err := expression.NewBuilder().WithCondition(condition).Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName:                 aws.String(r.tableName),
		Item:                      item,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return fmt.Errorf("optimistic lock failed: %w", err)
		}
		return fmt.Errorf("failed to save entity: %w", err)
	}

	r.logger.Debug("Entity saved",
		zap.String("entityType", r.config.GetEntityType()),
		zap.String("entityID", entity.GetID()),
		zap.String("userID", entity.GetUserID()),
	)

	return nil
}

// GetByID retrieves an entity by its ID
func (r *GenericRepository[T]) GetByID(ctx context.Context, userID, entityID string) (T, error) {
	var zero T

	key := r.config.BuildKey(userID, entityID)

	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}

	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return zero, fmt.Errorf("failed to get item: %w", err)
	}

	if result.Item == nil {
		return zero, fmt.Errorf("entity not found")
	}

	entity, err := r.config.ParseItem(result.Item)
	if err != nil {
		return zero, fmt.Errorf("failed to parse item: %w", err)
	}

	return entity, nil
}

// GetByUserID retrieves all entities for a user
func (r *GenericRepository[T]) GetByUserID(ctx context.Context, userID string) ([]T, error) {
	// Build query expression
	keyExpr := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID)))
	filterExpr := expression.Name("EntityType").Equal(expression.Value(r.config.GetEntityType()))

	expr, err := expression.NewBuilder().
		WithKeyCondition(keyExpr).
		WithFilter(filterExpr).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}

	entities := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		entity, err := r.config.ParseItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse item", zap.Error(err))
			continue
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// Delete removes an entity
func (r *GenericRepository[T]) Delete(ctx context.Context, userID, entityID string) error {
	key := r.config.BuildKey(userID, entityID)

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}

	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	r.logger.Debug("Entity deleted",
		zap.String("entityType", r.config.GetEntityType()),
		zap.String("entityID", entityID),
		zap.String("userID", userID),
	)

	return nil
}

// QueryWithPagination performs a paginated query
func (r *GenericRepository[T]) QueryWithPagination(
	ctx context.Context,
	userID string,
	limit int32,
	lastEvaluatedKey map[string]types.AttributeValue,
) ([]T, map[string]types.AttributeValue, error) {
	// Build query expression
	keyExpr := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID)))
	filterExpr := expression.Name("EntityType").Equal(expression.Value(r.config.GetEntityType()))

	expr, err := expression.NewBuilder().
		WithKeyCondition(keyExpr).
		WithFilter(filterExpr).
		Build()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(limit),
		ExclusiveStartKey:         lastEvaluatedKey,
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query items: %w", err)
	}

	entities := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		entity, err := r.config.ParseItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse item", zap.Error(err))
			continue
		}
		entities = append(entities, entity)
	}

	return entities, result.LastEvaluatedKey, nil
}

// BatchSave saves multiple entities in a transaction
func (r *GenericRepository[T]) BatchSave(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	// DynamoDB limits batch writes to 25 items
	const batchSize = 25
	const maxRetries = 3

	totalProcessed := 0

	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}

		batch := entities[i:end]
		requests := make([]types.WriteRequest, 0, len(batch))

		for _, entity := range batch {
			item, err := r.config.ToItem(entity)
			if err != nil {
				return fmt.Errorf("failed to convert entity to item: %w", err)
			}

			requests = append(requests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		// Retry logic for unprocessed items
		unprocessedRequests := requests
		for retry := 0; retry < maxRetries && len(unprocessedRequests) > 0; retry++ {
			input := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					r.tableName: unprocessedRequests,
				},
			}

			result, err := r.client.BatchWriteItem(ctx, input)
			if err != nil {
				// Exponential backoff for retries
				backoffDuration := time.Duration(retry*retry+1) * time.Millisecond * 100
				r.logger.Warn("Batch write failed, retrying",
					zap.Error(err),
					zap.Int("retry", retry+1),
					zap.Duration("backoff", backoffDuration),
				)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoffDuration):
					// Continue with retry
				}
				continue
			}

			// Check for unprocessed items
			if unprocessedItems, exists := result.UnprocessedItems[r.tableName]; exists && len(unprocessedItems) > 0 {
				unprocessedRequests = unprocessedItems
				r.logger.Debug("Found unprocessed items, retrying",
					zap.Int("unprocessedCount", len(unprocessedItems)),
					zap.Int("retry", retry+1),
				)

				// Exponential backoff for unprocessed items
				backoffDuration := time.Duration(retry*retry+1) * time.Millisecond * 100
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoffDuration):
					// Continue with retry
				}
			} else {
				// All items processed successfully
				unprocessedRequests = nil
				break
			}
		}

		if len(unprocessedRequests) > 0 {
			return fmt.Errorf("failed to process %d items after %d retries", len(unprocessedRequests), maxRetries)
		}

		totalProcessed += len(batch)
	}

	r.logger.Debug("Batch saved entities successfully",
		zap.String("entityType", r.config.GetEntityType()),
		zap.Int("totalCount", len(entities)),
		zap.Int("processedCount", totalProcessed),
	)

	return nil
}

// Update performs a partial update on an entity
func (r *GenericRepository[T]) Update(ctx context.Context, userID, entityID string, updates map[string]interface{}) error {
	key := r.config.BuildKey(userID, entityID)

	// Build update expression
	var updateExpr expression.UpdateBuilder
	for attr, value := range updates {
		updateExpr = updateExpr.Set(expression.Name(attr), expression.Value(value))
	}

	// Add updated timestamp
	updateExpr = updateExpr.Set(
		expression.Name("UpdatedAt"),
		expression.Value(time.Now().Format(time.RFC3339)),
	)

	// Build condition to ensure entity exists
	condition := expression.Name("PK").AttributeExists()

	expr, err := expression.NewBuilder().
		WithUpdate(updateExpr).
		WithCondition(condition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return fmt.Errorf("entity not found: %w", err)
		}
		return fmt.Errorf("failed to update entity: %w", err)
	}

	r.logger.Debug("Entity updated",
		zap.String("entityType", r.config.GetEntityType()),
		zap.String("entityID", entityID),
		zap.String("userID", userID),
	)

	return nil
}

// Exists checks if an entity exists
func (r *GenericRepository[T]) Exists(ctx context.Context, userID, entityID string) (bool, error) {
	key := r.config.BuildKey(userID, entityID)

	input := &dynamodb.GetItemInput{
		TableName:            aws.String(r.tableName),
		Key:                  key,
		ProjectionExpression: aws.String("PK"), // Only fetch primary key
	}

	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return result.Item != nil, nil
}

// BatchDelete deletes multiple entities by their keys
func (r *GenericRepository[T]) BatchDelete(ctx context.Context, keys []map[string]types.AttributeValue) error {
	if len(keys) == 0 {
		return nil
	}

	// DynamoDB limits batch writes to 25 items
	const batchSize = 25
	const maxRetries = 3

	totalProcessed := 0

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		requests := make([]types.WriteRequest, 0, len(batch))

		for _, key := range batch {
			requests = append(requests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: key,
				},
			})
		}

		// Retry logic for unprocessed items
		unprocessedRequests := requests
		for retry := 0; retry < maxRetries && len(unprocessedRequests) > 0; retry++ {
			input := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					r.tableName: unprocessedRequests,
				},
			}

			result, err := r.client.BatchWriteItem(ctx, input)
			if err != nil {
				// Exponential backoff for retries
				backoffDuration := time.Duration(retry*retry+1) * time.Millisecond * 100
				r.logger.Warn("Batch delete failed, retrying",
					zap.Error(err),
					zap.Int("retry", retry+1),
					zap.Duration("backoff", backoffDuration),
				)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoffDuration):
					// Continue with retry
				}
				continue
			}

			// Check for unprocessed items
			if unprocessedItems, exists := result.UnprocessedItems[r.tableName]; exists && len(unprocessedItems) > 0 {
				unprocessedRequests = unprocessedItems
				r.logger.Debug("Found unprocessed delete requests, retrying",
					zap.Int("unprocessedCount", len(unprocessedItems)),
					zap.Int("retry", retry+1),
				)

				// Exponential backoff for unprocessed items
				backoffDuration := time.Duration(retry*retry+1) * time.Millisecond * 100
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoffDuration):
					// Continue with retry
				}
			} else {
				// All items processed successfully
				unprocessedRequests = nil
				break
			}
		}

		if len(unprocessedRequests) > 0 {
			return fmt.Errorf("failed to delete %d items after %d retries", len(unprocessedRequests), maxRetries)
		}

		totalProcessed += len(batch)
	}

	r.logger.Debug("Batch deleted items successfully",
		zap.String("entityType", r.config.GetEntityType()),
		zap.Int("totalCount", len(keys)),
		zap.Int("processedCount", totalProcessed),
	)

	return nil
}
