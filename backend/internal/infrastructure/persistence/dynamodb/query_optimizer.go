// Package dynamodb provides query optimization for DynamoDB operations.
package dynamodb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"go.uber.org/zap"
)

// QueryOptimizer optimizes DynamoDB query patterns for better performance.
type QueryOptimizer struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
	
	// Cache for frequently accessed data
	cache      map[string]cachedResult
	cacheMu    sync.RWMutex
	cacheTTL   time.Duration
}

type cachedResult struct {
	items     []map[string]types.AttributeValue
	timestamp time.Time
}

// NewQueryOptimizer creates a new query optimizer.
func NewQueryOptimizer(
	client *dynamodb.Client,
	tableName string,
	indexName string,
	logger *zap.Logger,
) *QueryOptimizer {
	return &QueryOptimizer{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
		cache:     make(map[string]cachedResult),
		cacheTTL:  5 * time.Minute,
	}
}

// OptimizedQuery performs an optimized query with caching and efficient pagination.
func (qo *QueryOptimizer) OptimizedQuery(
	ctx context.Context,
	partitionKey string,
	sortKeyCondition *expression.KeyConditionBuilder,
	limit int,
) ([]map[string]types.AttributeValue, error) {
	
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%v:%d", partitionKey, sortKeyCondition, limit)
	if cached, ok := qo.getFromCache(cacheKey); ok {
		qo.logger.Debug("Cache hit for query", zap.String("key", cacheKey))
		return cached, nil
	}
	
	// Build optimized query
	keyCondition := expression.Key("PK").Equal(expression.Value(partitionKey))
	if sortKeyCondition != nil {
		keyCondition = keyCondition.And(*sortKeyCondition)
	}
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyCondition).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(qo.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(limit)),
		ScanIndexForward:          aws.Bool(false), // Most recent first
		ConsistentRead:            aws.Bool(false), // Eventually consistent for better performance
	}
	
	// Use index if available for better performance
	if qo.indexName != "" {
		input.IndexName = aws.String(qo.indexName)
	}
	
	// Execute query with efficient pagination
	var allItems []map[string]types.AttributeValue
	paginator := dynamodb.NewQueryPaginator(qo.client, input)
	
	itemCount := 0
	for paginator.HasMorePages() && itemCount < limit {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
		
		allItems = append(allItems, output.Items...)
		itemCount += len(output.Items)
		
		// Stop if we have enough items
		if itemCount >= limit {
			allItems = allItems[:limit]
			break
		}
	}
	
	// Cache the result
	qo.putInCache(cacheKey, allItems)
	
	qo.logger.Debug("Query completed",
		zap.String("partition_key", partitionKey),
		zap.Int("items", len(allItems)),
	)
	
	return allItems, nil
}

// ParallelScan performs an optimized parallel scan for large datasets.
func (qo *QueryOptimizer) ParallelScan(
	ctx context.Context,
	filterExpr expression.ConditionBuilder,
	segments int,
) ([]map[string]types.AttributeValue, error) {
	
	if segments < 1 {
		segments = 4 // Default parallel segments
	}
	
	expr, err := expression.NewBuilder().
		WithFilter(filterExpr).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allItems []map[string]types.AttributeValue
	errors := make(chan error, segments)
	
	// Parallel scan segments
	for segment := 0; segment < segments; segment++ {
		wg.Add(1)
		go func(seg int) {
			defer wg.Done()
			
			input := &dynamodb.ScanInput{
				TableName:                 aws.String(qo.tableName),
				FilterExpression:          expr.Filter(),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				Segment:                   aws.Int32(int32(seg)),
				TotalSegments:             aws.Int32(int32(segments)),
				ConsistentRead:            aws.Bool(false),
			}
			
			// Scan with pagination
			paginator := dynamodb.NewScanPaginator(qo.client, input)
			
			var segmentItems []map[string]types.AttributeValue
			for paginator.HasMorePages() {
				output, err := paginator.NextPage(ctx)
				if err != nil {
					errors <- fmt.Errorf("scan segment %d failed: %w", seg, err)
					return
				}
				segmentItems = append(segmentItems, output.Items...)
			}
			
			// Merge results
			mu.Lock()
			allItems = append(allItems, segmentItems...)
			mu.Unlock()
			
			qo.logger.Debug("Scan segment completed",
				zap.Int("segment", seg),
				zap.Int("items", len(segmentItems)),
			)
		}(segment)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	if err := <-errors; err != nil {
		return nil, err
	}
	
	qo.logger.Info("Parallel scan completed",
		zap.Int("segments", segments),
		zap.Int("total_items", len(allItems)),
	)
	
	return allItems, nil
}

// ProjectionQuery performs a query with projection to reduce data transfer.
func (qo *QueryOptimizer) ProjectionQuery(
	ctx context.Context,
	partitionKey string,
	projectionFields []string,
) ([]map[string]types.AttributeValue, error) {
	
	// Build projection expression
	var projection expression.ProjectionBuilder
	for i, field := range projectionFields {
		if i == 0 {
			projection = expression.NamesList(expression.Name(field))
		} else {
			projection = projection.AddNames(expression.Name(field))
		}
	}
	
	keyCondition := expression.Key("PK").Equal(expression.Value(partitionKey))
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyCondition).
		WithProjection(projection).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(qo.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ConsistentRead:            aws.Bool(false),
	}
	
	output, err := qo.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("projection query failed: %w", err)
	}
	
	qo.logger.Debug("Projection query completed",
		zap.String("partition_key", partitionKey),
		zap.Strings("fields", projectionFields),
		zap.Int("items", len(output.Items)),
	)
	
	return output.Items, nil
}

// BatchQueryWithAggregation performs multiple queries and aggregates results.
func (qo *QueryOptimizer) BatchQueryWithAggregation(
	ctx context.Context,
	partitionKeys []string,
	aggregator func([]map[string]types.AttributeValue) map[string]interface{},
) (map[string]interface{}, error) {
	
	// Parallel queries
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	
	var mu sync.Mutex
	var allItems []map[string]types.AttributeValue
	var wg sync.WaitGroup
	
	for _, pk := range partitionKeys {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(partitionKey string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			
			items, err := qo.OptimizedQuery(ctx, partitionKey, nil, 100)
			if err != nil {
				qo.logger.Error("Query failed in batch",
					zap.Error(err),
					zap.String("partition_key", partitionKey),
				)
				return
			}
			
			mu.Lock()
			allItems = append(allItems, items...)
			mu.Unlock()
		}(pk)
	}
	
	wg.Wait()
	
	// Aggregate results
	result := aggregator(allItems)
	
	qo.logger.Debug("Batch query with aggregation completed",
		zap.Int("partitions", len(partitionKeys)),
		zap.Int("total_items", len(allItems)),
	)
	
	return result, nil
}

// getFromCache retrieves a cached result if it exists and is not expired.
func (qo *QueryOptimizer) getFromCache(key string) ([]map[string]types.AttributeValue, bool) {
	qo.cacheMu.RLock()
	defer qo.cacheMu.RUnlock()
	
	if cached, ok := qo.cache[key]; ok {
		if time.Since(cached.timestamp) < qo.cacheTTL {
			return cached.items, true
		}
	}
	
	return nil, false
}

// putInCache stores a result in the cache.
func (qo *QueryOptimizer) putInCache(key string, items []map[string]types.AttributeValue) {
	qo.cacheMu.Lock()
	defer qo.cacheMu.Unlock()
	
	qo.cache[key] = cachedResult{
		items:     items,
		timestamp: time.Now(),
	}
	
	// Clean old cache entries
	qo.cleanCache()
}

// cleanCache removes expired cache entries.
func (qo *QueryOptimizer) cleanCache() {
	now := time.Now()
	for key, cached := range qo.cache {
		if now.Sub(cached.timestamp) > qo.cacheTTL {
			delete(qo.cache, key)
		}
	}
}

// ClearCache clears all cached results.
func (qo *QueryOptimizer) ClearCache() {
	qo.cacheMu.Lock()
	defer qo.cacheMu.Unlock()
	
	qo.cache = make(map[string]cachedResult)
}