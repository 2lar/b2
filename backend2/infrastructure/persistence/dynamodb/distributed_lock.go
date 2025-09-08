package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// DistributedLock provides distributed locking using DynamoDB conditional writes
type DistributedLock struct {
	client    *dynamodb.Client
	tableName string
	logger    *zap.Logger
}

// LockRecord represents a lock record in DynamoDB
type LockRecord struct {
	PK         string `dynamodbav:"PK"`         // LOCK#<resource_name>
	SK         string `dynamodbav:"SK"`         // LOCK
	LockID     string `dynamodbav:"LockID"`     // Unique lock identifier
	Owner      string `dynamodbav:"Owner"`      // Lock owner identifier
	AcquiredAt string `dynamodbav:"AcquiredAt"` // RFC3339 timestamp
	ExpiresAt  string `dynamodbav:"ExpiresAt"`  // RFC3339 timestamp
	TTL        int64  `dynamodbav:"TTL"`        // Unix timestamp for DynamoDB TTL
}

// NewDistributedLock creates a new distributed lock instance
func NewDistributedLock(client *dynamodb.Client, tableName string, logger *zap.Logger) *DistributedLock {
	return &DistributedLock{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// AcquireLock attempts to acquire a distributed lock for the given resource
func (dl *DistributedLock) AcquireLock(ctx context.Context, resourceName, ownerID string, lockDuration time.Duration) (*Lock, error) {
	lockID := fmt.Sprintf("%s_%d", ownerID, time.Now().UnixNano())
	now := time.Now()
	expiresAt := now.Add(lockDuration)
	
	lockRecord := LockRecord{
		PK:         fmt.Sprintf("LOCK#%s", resourceName),
		SK:         "LOCK",
		LockID:     lockID,
		Owner:      ownerID,
		AcquiredAt: now.Format(time.RFC3339),
		ExpiresAt:  expiresAt.Format(time.RFC3339),
		TTL:        expiresAt.Unix(),
	}
	
	// Convert to DynamoDB item
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: lockRecord.PK},
		"SK":         &types.AttributeValueMemberS{Value: lockRecord.SK},
		"LockID":     &types.AttributeValueMemberS{Value: lockRecord.LockID},
		"Owner":      &types.AttributeValueMemberS{Value: lockRecord.Owner},
		"AcquiredAt": &types.AttributeValueMemberS{Value: lockRecord.AcquiredAt},
		"ExpiresAt":  &types.AttributeValueMemberS{Value: lockRecord.ExpiresAt},
		"TTL":        &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", lockRecord.TTL)},
	}
	
	// Try to acquire the lock using conditional write
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(dl.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) OR ExpiresAt < :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
	}
	
	_, err := dl.client.PutItem(ctx, input)
	if err != nil {
		var conditionalCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckFailed) {
			dl.logger.Debug("Failed to acquire lock - already held",
				zap.String("resource", resourceName),
				zap.String("owner", ownerID),
			)
			return nil, fmt.Errorf("lock already held for resource: %s", resourceName)
		}
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	
	dl.logger.Debug("Lock acquired successfully",
		zap.String("resource", resourceName),
		zap.String("lockID", lockID),
		zap.String("owner", ownerID),
		zap.Duration("duration", lockDuration),
	)
	
	return &Lock{
		distributedLock: dl,
		resourceName:    resourceName,
		lockID:          lockID,
		ownerID:         ownerID,
		expiresAt:       expiresAt,
	}, nil
}

// TryAcquireLock attempts to acquire a lock with a timeout
func (dl *DistributedLock) TryAcquireLock(ctx context.Context, resourceName, ownerID string, lockDuration, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	retryInterval := 100 * time.Millisecond
	
	for time.Now().Before(deadline) {
		lock, err := dl.AcquireLock(ctx, resourceName, ownerID, lockDuration)
		if err == nil {
			return lock, nil
		}
		
		// If it's not a lock contention error, return immediately
		if err.Error() != fmt.Sprintf("lock already held for resource: %s", resourceName) {
			return nil, err
		}
		
		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
			// Exponential backoff with jitter
			if retryInterval < time.Second {
				retryInterval = time.Duration(float64(retryInterval) * 1.5)
			}
		}
	}
	
	return nil, fmt.Errorf("timeout acquiring lock for resource: %s", resourceName)
}

// ReleaseLock releases the specified lock
func (dl *DistributedLock) ReleaseLock(ctx context.Context, resourceName, lockID, ownerID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(dl.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("LOCK#%s", resourceName)},
			"SK": &types.AttributeValueMemberS{Value: "LOCK"},
		},
		ConditionExpression: aws.String("LockID = :lockId AND Owner = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":lockId": &types.AttributeValueMemberS{Value: lockID},
			":owner":  &types.AttributeValueMemberS{Value: ownerID},
		},
	}
	
	_, err := dl.client.DeleteItem(ctx, input)
	if err != nil {
		var conditionalCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &conditionalCheckFailed) {
			dl.logger.Warn("Lock already released or owned by someone else",
				zap.String("resource", resourceName),
				zap.String("lockID", lockID),
				zap.String("owner", ownerID),
			)
			return nil // Lock is already gone, which is what we wanted
		}
		return fmt.Errorf("failed to release lock: %w", err)
	}
	
	dl.logger.Debug("Lock released successfully",
		zap.String("resource", resourceName),
		zap.String("lockID", lockID),
		zap.String("owner", ownerID),
	)
	
	return nil
}

// Lock represents an acquired distributed lock
type Lock struct {
	distributedLock *DistributedLock
	resourceName    string
	lockID          string
	ownerID         string
	expiresAt       time.Time
}

// Release releases the lock
func (l *Lock) Release(ctx context.Context) error {
	return l.distributedLock.ReleaseLock(ctx, l.resourceName, l.lockID, l.ownerID)
}

// IsExpired checks if the lock has expired
func (l *Lock) IsExpired() bool {
	return time.Now().After(l.expiresAt)
}

// TimeUntilExpiry returns the time until the lock expires
func (l *Lock) TimeUntilExpiry() time.Duration {
	if l.IsExpired() {
		return 0
	}
	return time.Until(l.expiresAt)
}

// Extend extends the lock duration (not implemented in this basic version)
func (l *Lock) Extend(ctx context.Context, additionalDuration time.Duration) error {
	// This would require updating the lock record in DynamoDB
	// For simplicity, this is not implemented in this basic version
	return fmt.Errorf("lock extension not implemented")
}

// LockInfo returns information about the lock
func (l *Lock) LockInfo() map[string]interface{} {
	return map[string]interface{}{
		"resourceName":    l.resourceName,
		"lockID":          l.lockID,
		"ownerID":         l.ownerID,
		"expiresAt":       l.expiresAt,
		"isExpired":       l.IsExpired(),
		"timeUntilExpiry": l.TimeUntilExpiry().String(),
	}
}