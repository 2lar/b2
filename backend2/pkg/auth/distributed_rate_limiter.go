package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DistributedRateLimiter implements rate limiting using DynamoDB as the state store
// This allows rate limiting to work correctly across Lambda invocations
type DistributedRateLimiter struct {
	client    *dynamodb.Client
	tableName string
	limit     int
	window    time.Duration
	keyPrefix string
}

// RateLimitEntry represents a rate limit entry in DynamoDB
type RateLimitEntry struct {
	PK        string    `dynamodbav:"PK"`
	Count     int       `dynamodbav:"Count"`
	WindowEnd time.Time `dynamodbav:"WindowEnd"`
	TTL       int64     `dynamodbav:"TTL"`
}

// NewDistributedIPRateLimiter creates a rate limiter for IP addresses
func NewDistributedIPRateLimiter(client *dynamodb.Client, tableName string, requestsPerMinute int) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		client:    client,
		tableName: tableName,
		limit:     requestsPerMinute,
		window:    time.Minute,
		keyPrefix: "IP",
	}
}

// NewDistributedUserRateLimiter creates a rate limiter for user IDs
func NewDistributedUserRateLimiter(client *dynamodb.Client, tableName string, requestsPerMinute int) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		client:    client,
		tableName: tableName,
		limit:     requestsPerMinute,
		window:    time.Minute,
		keyPrefix: "USER",
	}
}

// NewDistributedRateLimiter creates a generic distributed rate limiter
func NewDistributedRateLimiter(client *dynamodb.Client, tableName string, limit int, window time.Duration, keyPrefix string) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		client:    client,
		tableName: tableName,
		limit:     limit,
		window:    window,
		keyPrefix: keyPrefix,
	}
}

// Allow checks if a request is allowed under the rate limit
func (r *DistributedRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if r.client == nil {
		// Fallback to allowing all requests if DynamoDB is not configured
		// This is useful for local development
		return true, nil
	}

	now := time.Now()
	windowStart := now.Truncate(r.window)
	windowEnd := windowStart.Add(r.window)
	
	// Create composite key with prefix, key, and window timestamp
	pk := fmt.Sprintf("RATELIMIT#%s#%s#%d", r.keyPrefix, key, windowStart.Unix())
	
	// Atomic increment with conditional check
	// This uses DynamoDB's conditional update to atomically increment the counter
	// only if it's below the limit
	update := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
		UpdateExpression: aws.String("SET #count = if_not_exists(#count, :zero) + :incr, WindowEnd = :window_end, #ttl = :ttl"),
		ConditionExpression: aws.String("attribute_not_exists(#count) OR #count < :limit"),
		ExpressionAttributeNames: map[string]string{
			"#count": "Count",
			"#ttl":   "TTL",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":       &types.AttributeValueMemberN{Value: "0"},
			":incr":       &types.AttributeValueMemberN{Value: "1"},
			":limit":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", r.limit)},
			":window_end": &types.AttributeValueMemberS{Value: windowEnd.Format(time.RFC3339)},
			":ttl":        &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowEnd.Add(time.Hour).Unix())},
		},
		ReturnValues: types.ReturnValueAllNew,
	}
	
	result, err := r.client.UpdateItem(ctx, update)
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			// Rate limit exceeded
			return false, nil
		}
		// For other errors, fail open (allow the request) to avoid blocking legitimate traffic
		// Log the error for monitoring
		return true, fmt.Errorf("rate limiter error (failing open): %w", err)
	}
	
	// Parse the count from result to verify
	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
		// If we can't parse, fail open
		return true, fmt.Errorf("failed to parse rate limit entry (failing open): %w", err)
	}
	
	return entry.Count <= r.limit, nil
}

// GetRemaining returns the number of requests remaining in the current window
func (r *DistributedRateLimiter) GetRemaining(ctx context.Context, key string) (int, time.Duration, error) {
	if r.client == nil {
		return r.limit, r.window, nil
	}

	now := time.Now()
	windowStart := now.Truncate(r.window)
	windowEnd := windowStart.Add(r.window)
	
	pk := fmt.Sprintf("RATELIMIT#%s#%s#%d", r.keyPrefix, key, windowStart.Unix())
	
	get := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
	}
	
	result, err := r.client.GetItem(ctx, get)
	if err != nil {
		// On error, return full limit
		return r.limit, time.Until(windowEnd), nil
	}
	
	if result.Item == nil {
		// No entry yet, full limit available
		return r.limit, time.Until(windowEnd), nil
	}
	
	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Item, &entry); err != nil {
		return r.limit, time.Until(windowEnd), fmt.Errorf("failed to parse rate limit entry: %w", err)
	}
	
	remaining := r.limit - entry.Count
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, time.Until(entry.WindowEnd), nil
}

// Reset clears the rate limit for a given key (useful for testing or admin operations)
func (r *DistributedRateLimiter) Reset(ctx context.Context, key string) error {
	if r.client == nil {
		return nil
	}

	now := time.Now()
	windowStart := now.Truncate(r.window)
	pk := fmt.Sprintf("RATELIMIT#%s#%s#%d", r.keyPrefix, key, windowStart.Unix())
	
	delete := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
		},
	}
	
	_, err := r.client.DeleteItem(ctx, delete)
	return err
}

// GetLimit returns the configured rate limit
func (r *DistributedRateLimiter) GetLimit() int {
	return r.limit
}

// GetWindow returns the configured time window
func (r *DistributedRateLimiter) GetWindow() time.Duration {
	return r.window
}

// SetHeaders adds rate limit headers to an HTTP response
func (r *DistributedRateLimiter) SetHeaders(ctx context.Context, key string, headers map[string]string) error {
	remaining, resetIn, err := r.GetRemaining(ctx, key)
	if err != nil {
		return err
	}
	
	headers["X-RateLimit-Limit"] = fmt.Sprintf("%d", r.limit)
	headers["X-RateLimit-Remaining"] = fmt.Sprintf("%d", remaining)
	headers["X-RateLimit-Reset"] = fmt.Sprintf("%d", time.Now().Add(resetIn).Unix())
	
	return nil
}