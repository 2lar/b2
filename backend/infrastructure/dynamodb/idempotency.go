package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	_, err = s.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:               itemMap,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})

	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			// Key already exists - this is expected for duplicate operations
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