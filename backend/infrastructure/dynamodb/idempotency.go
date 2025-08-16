package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ddbIdempotencyItem represents the structure of an idempotency item in DynamoDB
type ddbIdempotencyItem struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	Result    string `dynamodbav:"Result"`
	CreatedAt string `dynamodbav:"CreatedAt"`
	TTL       int64  `dynamodbav:"TTL"`
}

// ddbIdempotencyStore implements IdempotencyStore using DynamoDB
type ddbIdempotencyStore struct {
	dbClient  *dynamodb.Client
	tableName string
	ttl       time.Duration
}

// NewIdempotencyStore creates a new DynamoDB-based idempotency store
func NewIdempotencyStore(dbClient *dynamodb.Client, tableName string, ttl time.Duration) repository.IdempotencyStore {
	if ttl == 0 {
		ttl = 24 * time.Hour // Default to 24 hours
	}
	
	return &ddbIdempotencyStore{
		dbClient:  dbClient,
		tableName: tableName,
		ttl:       ttl,
	}
}

// Store implements IdempotencyStore.Store
func (s *ddbIdempotencyStore) Store(ctx context.Context, key repository.IdempotencyKey, result interface{}) error {
	// Serialize the result to JSON
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return appErrors.Wrap(err, "failed to serialize idempotency result")
	}

	pk := fmt.Sprintf("IDEMPOTENCY#%s#%s", key.UserID, key.Operation)
	sk := key.Hash
	
	expirationTime := time.Now().Add(s.ttl)
	
	item := ddbIdempotencyItem{
		PK:        pk,
		SK:        sk,
		Result:    string(resultBytes),
		CreatedAt: key.CreatedAt.Format(time.RFC3339),
		TTL:       expirationTime.Unix(),
	}

	itemMap, err := attributevalue.MarshalMap(item)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal idempotency item")
	}

	// Use conditional write to prevent overwriting existing keys
	// This handles concurrent requests - only the first request will succeed
	_, err = s.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:               itemMap,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			// Key already exists - this is expected for concurrent duplicate operations
			// The first request succeeded and stored the result, subsequent requests should
			// retrieve and return the existing result
			return nil
		}
		return appErrors.Wrap(err, "failed to store idempotency key")
	}

	return nil
}

// Get implements IdempotencyStore.Get
func (s *ddbIdempotencyStore) Get(ctx context.Context, key repository.IdempotencyKey) (interface{}, bool, error) {
	pk := fmt.Sprintf("IDEMPOTENCY#%s#%s", key.UserID, key.Operation)
	sk := key.Hash

	result, err := s.dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})

	if err != nil {
		return nil, false, appErrors.Wrap(err, "failed to get idempotency key")
	}

	if result.Item == nil {
		return nil, false, nil // Not found
	}

	var item ddbIdempotencyItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, false, appErrors.Wrap(err, "failed to unmarshal idempotency item")
	}

	// Deserialize the result from JSON
	var storedResult interface{}
	if err := json.Unmarshal([]byte(item.Result), &storedResult); err != nil {
		return nil, false, appErrors.Wrap(err, "failed to deserialize idempotency result")
	}

	return storedResult, true, nil
}

// Delete implements IdempotencyStore.Delete
func (s *ddbIdempotencyStore) Delete(ctx context.Context, key repository.IdempotencyKey) error {
	pk := fmt.Sprintf("IDEMPOTENCY#%s#%s", key.UserID, key.Operation)
	sk := key.Hash

	_, err := s.dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})

	if err != nil {
		return appErrors.Wrap(err, "failed to delete idempotency key")
	}

	return nil
}

// Cleanup implements IdempotencyStore.Cleanup
func (s *ddbIdempotencyStore) Cleanup(ctx context.Context, expiration time.Duration) error {
	// DynamoDB TTL handles cleanup automatically, so this is a no-op
	// The TTL attribute on items will cause them to be automatically deleted
	return nil
}

// GetOrStore attempts to get an existing result, or store a new one if it doesn't exist.
// This method provides better concurrent request handling by combining get and store operations.
func (s *ddbIdempotencyStore) GetOrStore(ctx context.Context, key repository.IdempotencyKey, result interface{}) (interface{}, bool, error) {
	// First attempt to get existing result
	existingResult, exists, err := s.Get(ctx, key)
	if err != nil {
		return nil, false, appErrors.Wrap(err, "failed to check for existing idempotency result")
	}
	
	if exists {
		log.Printf("DEBUG: Found existing idempotency result for key %s:%s", key.Operation, key.Hash[:8])
		return existingResult, true, nil
	}
	
	// Try to store the new result
	storeErr := s.Store(ctx, key, result)
	if storeErr != nil {
		// If store failed due to concurrent write, try to get the result again
		// This handles the race condition where another request stored the result
		// between our Get and Store calls
		if retryResult, retryExists, retryErr := s.Get(ctx, key); retryErr == nil && retryExists {
			log.Printf("DEBUG: Concurrent write detected, returning existing result for key %s:%s", key.Operation, key.Hash[:8])
			return retryResult, true, nil
		}
		return nil, false, appErrors.Wrap(storeErr, "failed to store idempotency result")
	}
	
	log.Printf("DEBUG: Stored new idempotency result for key %s:%s", key.Operation, key.Hash[:8])
	return result, false, nil
}

// BatchGet retrieves multiple idempotency keys in a single request for better performance.
func (s *ddbIdempotencyStore) BatchGet(ctx context.Context, keys []repository.IdempotencyKey) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}
	
	// Build batch get request
	requestItems := make(map[string]types.KeysAndAttributes)
	keyMap := make(map[string]repository.IdempotencyKey)
	
	var itemKeys []map[string]types.AttributeValue
	for _, key := range keys {
		pk := fmt.Sprintf("IDEMPOTENCY#%s#%s", key.UserID, key.Operation)
		sk := key.Hash
		
		itemKey := map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		}
		itemKeys = append(itemKeys, itemKey)
		keyMap[pk+":"+sk] = key
	}
	
	requestItems[s.tableName] = types.KeysAndAttributes{
		Keys: itemKeys,
	}
	
	// Execute batch get
	result, err := s.dbClient.BatchGetItem(ctx, &dynamodb.BatchGetItemInput{
		RequestItems: requestItems,
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to batch get idempotency keys")
	}
	
	// Process results
	results := make(map[string]interface{})
	for _, item := range result.Responses[s.tableName] {
		var ddbItem ddbIdempotencyItem
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
			log.Printf("WARN: failed to unmarshal idempotency item: %v", err)
			continue
		}
		
		// Deserialize the result
		var storedResult interface{}
		if err := json.Unmarshal([]byte(ddbItem.Result), &storedResult); err != nil {
			log.Printf("WARN: failed to deserialize idempotency result: %v", err)
			continue
		}
		
		keyStr := ddbItem.PK + ":" + ddbItem.SK
		if originalKey, exists := keyMap[keyStr]; exists {
			results[s.makeKeyString(originalKey)] = storedResult
		}
	}
	
	return results, nil
}

// makeKeyString creates a consistent string representation of an idempotency key.
func (s *ddbIdempotencyStore) makeKeyString(key repository.IdempotencyKey) string {
	return fmt.Sprintf("%s:%s:%s", key.UserID, key.Operation, key.Hash)
}