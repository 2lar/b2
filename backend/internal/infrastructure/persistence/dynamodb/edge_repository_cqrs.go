// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file implements EdgeReader and EdgeWriter interfaces using direct CQRS patterns.
package dynamodb

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"
)

// EdgeRepositoryCQRS implements both EdgeReader and EdgeWriter interfaces directly.
type EdgeRepositoryCQRS struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
}

// NewEdgeRepositoryCQRS creates a new edge repository with direct CQRS support.
func NewEdgeRepositoryCQRS(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *EdgeRepositoryCQRS {
	return &EdgeRepositoryCQRS{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// Ensure interfaces are implemented
var (
	_ repository.EdgeReader     = (*EdgeRepositoryCQRS)(nil)
	_ repository.EdgeWriter     = (*EdgeRepositoryCQRS)(nil)
	_ repository.EdgeRepository = (*EdgeRepositoryCQRS)(nil) // For backward compatibility
)

// ============================================================================
// EDGE READER INTERFACE - Read Operations
// ============================================================================

// FindByID retrieves an edge by its ID with explicit userID.
func (r *EdgeRepositoryCQRS) FindByID(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (*edge.Edge, error) {
	
	// Build the composite key for DynamoDB
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID.String())},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edgeID.String())},
	}
	
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrEdgeNotFound
	}
	
	// Parse the edge from DynamoDB item
	edge, err := r.parseEdgeFromItem(result.Item, userID.String())
	if err != nil {
		return nil, err
	}
	
	return edge, nil
}

// Exists checks if an edge exists with explicit userID.
func (r *EdgeRepositoryCQRS) Exists(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) (bool, error) {
	edge, err := r.FindByID(ctx, userID, edgeID)
	if err == repository.ErrEdgeNotFound {
		return false, nil
	}
	return edge != nil, err
}

// FindByUser retrieves all edges for a user.
func (r *EdgeRepositoryCQRS) FindByUser(ctx context.Context, userID shared.UserID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// Use EdgeIndex GSI to efficiently query all edges for the user
	// This avoids scan operations and provides consistent performance
	
	// Build key condition expression for GSI2 (EdgeIndex)
	keyEx := expression.Key("GSI2PK").Equal(expression.Value(fmt.Sprintf("USER#%s#EDGE", userID.String())))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build GSI expression: %w", err)
	}
	
	var allEdges []*edge.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	
	// Paginate through all results to ensure we get all edges
	for {
		input := &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String("EdgeIndex"), // Use EdgeIndex GSI (hardcoded like other repos)
			KeyConditionExpression:    expr.KeyCondition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			ExclusiveStartKey:         lastEvaluatedKey,
		}
		
		result, err := r.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to query EdgeIndex GSI: %w", err)
		}
		
		// Parse all edges for this user
		for _, item := range result.Items {
			edge, err := r.parseEdgeFromItem(item, userID.String())
			if err != nil {
				r.logger.Warn("Failed to parse edge", zap.Error(err))
				continue
			}
			allEdges = append(allEdges, edge)
		}
		
		// Check if there are more results to fetch
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}
	
	return allEdges, nil
}

// CountByUser counts edges for a user.
func (r *EdgeRepositoryCQRS) CountByUser(ctx context.Context, userID shared.UserID) (int, error) {
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("EDGE#"))
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return 0, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Select:                    types.SelectCount,
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}
	
	return int(result.Count), nil
}

// FindBySourceNode finds edges originating from a specific node with explicit userID.
func (r *EdgeRepositoryCQRS) FindBySourceNode(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	
	// Apply query options
	options := repository.ApplyQueryOptions(opts...)
	
	// Build key condition expression for edges from this source node
	// PK = USER#<userID>#NODE#<sourceID>, SK begins with EDGE#
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s#NODE#%s", userID, sourceID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("EDGE#"))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(options.Limit)),
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}
	
	edges := make([]*edge.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		edge, err := r.parseEdgeFromItem(item, userID.String())
		if err != nil {
			r.logger.Warn("Failed to parse edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}
	
	return edges, nil
}

// FindByTargetNode finds edges pointing to a specific node with explicit userID.
func (r *EdgeRepositoryCQRS) FindByTargetNode(ctx context.Context, userID shared.UserID, targetID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	
	// Use EdgeIndex GSI to efficiently query all edges for the user, then filter for target
	// This avoids the scan limit issue and matches what EdgeRepository does
	
	// Build key condition expression for GSI2 (EdgeIndex)
	keyEx := expression.Key("GSI2PK").Equal(expression.Value(fmt.Sprintf("USER#%s#EDGE", userID)))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build GSI expression: %w", err)
	}
	
	var allEdges []*edge.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	
	// Paginate through all results to ensure we get all edges
	for {
		input := &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String("EdgeIndex"), // Use EdgeIndex GSI (hardcoded like other repos)
			KeyConditionExpression:    expr.KeyCondition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			ExclusiveStartKey:         lastEvaluatedKey,
		}
		
		result, err := r.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to query EdgeIndex GSI: %w", err)
		}
		
		// Parse edges and filter for target node
		for _, item := range result.Items {
			edge, err := r.parseEdgeFromItem(item, userID.String())
			if err != nil {
				r.logger.Warn("Failed to parse edge", zap.Error(err))
				continue
			}
			
			// Check if this edge points to our target node
			if edge.TargetID.String() == targetID.String() {
				allEdges = append(allEdges, edge)
			}
		}
		
		// Check if there are more results to fetch
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}
	
	return allEdges, nil
}

// FindByNode finds all edges connected to a specific node with explicit userID.
func (r *EdgeRepositoryCQRS) FindByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	
	edges, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter by node (either source or target)
	filtered := make([]*edge.Edge, 0)
	for _, edge := range edges {
		if edge.SourceID == nodeID || edge.TargetID == nodeID {
			filtered = append(filtered, edge)
		}
	}
	
	return filtered, nil
}

// FindBetweenNodes finds edges between two specific nodes with explicit userID.
func (r *EdgeRepositoryCQRS) FindBetweenNodes(ctx context.Context, userID shared.UserID, node1ID, node2ID shared.NodeID) ([]*edge.Edge, error) {
	
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Filter edges between the two nodes (bidirectional)
	filtered := make([]*edge.Edge, 0)
	for _, edge := range edges {
		if (edge.SourceID == node1ID && edge.TargetID == node2ID) ||
		   (edge.SourceID == node2ID && edge.TargetID == node1ID) {
			filtered = append(filtered, edge)
		}
	}
	
	return filtered, nil
}

// FindStrongConnections finds edges with weight above threshold.
func (r *EdgeRepositoryCQRS) FindStrongConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	edges, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter by weight threshold
	filtered := make([]*edge.Edge, 0)
	for _, edge := range edges {
		if edge.Weight() > threshold {
			filtered = append(filtered, edge)
		}
	}
	
	return filtered, nil
}

// FindWeakConnections finds edges with weight below threshold.
func (r *EdgeRepositoryCQRS) FindWeakConnections(ctx context.Context, userID shared.UserID, threshold float64, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	edges, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter by weight threshold
	filtered := make([]*edge.Edge, 0)
	for _, edge := range edges {
		if edge.Weight() <= threshold {
			filtered = append(filtered, edge)
		}
	}
	
	return filtered, nil
}

// FindBySpecification finds edges matching a specification.
func (r *EdgeRepositoryCQRS) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// This would be implemented based on the specification pattern
	// For now, return empty result
	return []*edge.Edge{}, nil
}

// CountBySpecification counts edges matching a specification.
func (r *EdgeRepositoryCQRS) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

// FindPage retrieves a page of edges.
func (r *EdgeRepositoryCQRS) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	// No need to add to context - userID is in the query
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	opts := []repository.QueryOption{
		repository.WithLimit(pagination.Limit),
	}
	
	if pagination.Cursor != "" {
		opts = append(opts, repository.WithCursor(pagination.Cursor))
	}
	
	edges, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Apply query filters
	if query.SourceID != "" {
		sourceID, _ := shared.ParseNodeID(query.SourceID)
		filtered := make([]*edge.Edge, 0)
		for _, edge := range edges {
			if edge.SourceID == sourceID {
				filtered = append(filtered, edge)
			}
		}
		edges = filtered
	}
	
	if query.TargetID != "" {
		targetID, _ := shared.ParseNodeID(query.TargetID)
		filtered := make([]*edge.Edge, 0)
		for _, edge := range edges {
			if edge.TargetID == targetID {
				filtered = append(filtered, edge)
			}
		}
		edges = filtered
	}
	
	// Generate next cursor if we have a full page
	nextCursor := ""
	if len(edges) == pagination.Limit && len(edges) > 0 {
		lastEdge := edges[len(edges)-1]
		nextCursor = lastEdge.ID.String()
	}
	
	return &repository.EdgePage{
		Items:      edges,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// FindEdges finds edges based on query criteria - part of EdgeReader interface.
func (r *EdgeRepositoryCQRS) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	// No need to add to context - userID is in the query
	// Parse userID first
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	// Check if we have a source node filter
	if query.SourceID != "" {
		sourceID, err := shared.ParseNodeID(query.SourceID)
		if err != nil {
			return nil, fmt.Errorf("invalid source node ID: %w", err)
		}
		return r.FindBySourceNode(ctx, userID, sourceID)
	}
	
	// Check if we have a target node filter
	if query.TargetID != "" {
		targetID, err := shared.ParseNodeID(query.TargetID)
		if err != nil {
			return nil, fmt.Errorf("invalid target node ID: %w", err)
		}
		return r.FindByTargetNode(ctx, userID, targetID)
	}
	
	// Otherwise return all edges for the user
	
	return r.FindByUser(ctx, userID)
}

// CountBySourceID counts edges from a source node.
func (r *EdgeRepositoryCQRS) CountBySourceID(ctx context.Context, sourceID shared.NodeID) (int, error) {
	// TODO: This should accept explicit userID but interface needs updating
	// For now, use a default userID or extract from edge data
	// This is a technical debt that should be addressed
	userID := shared.UserID{}
	
	edges, err := r.FindBySourceNode(ctx, userID, sourceID)
	if err != nil {
		return 0, err
	}
	return len(edges), nil
}

// ============================================================================
// EDGE WRITER INTERFACE - Write Operations
// ============================================================================

// Save creates a new edge.
func (r *EdgeRepositoryCQRS) Save(ctx context.Context, edge *edge.Edge) error {
	// Get userID from the edge entity itself
	userID := edge.UserID().String()
	if userID == "" {
		return fmt.Errorf("edge must have a valid user ID")
	}
	
	// Build the item with composite keys
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.ID.String())},
		"EntityType": &types.AttributeValueMemberS{Value: "EDGE"},
		"EdgeID":     &types.AttributeValueMemberS{Value: edge.ID.String()},
		"SourceID":   &types.AttributeValueMemberS{Value: edge.SourceID.String()},
		"TargetID":   &types.AttributeValueMemberS{Value: edge.TargetID.String()},
		"Weight":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", edge.Weight())},
		"CreatedAt":  &types.AttributeValueMemberS{Value: edge.CreatedAt.Format(time.RFC3339)},
		"UpdatedAt":  &types.AttributeValueMemberS{Value: edge.UpdatedAt.Format(time.RFC3339)},
		"Version":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", edge.Version)},
	}
	
	// Add metadata if present (edge doesn't have metadata field currently)
	
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	}
	
	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to save edge: %w", err)
	}
	
	return nil
}

// SaveBatch saves multiple edges using BatchWriteItem for optimal performance.
func (r *EdgeRepositoryCQRS) SaveBatch(ctx context.Context, edges []*edge.Edge) error {
	if len(edges) == 0 {
		return nil
	}
	
	// Validate all edges have the same userID (for security)
	var userID string
	for i, edge := range edges {
		edgeUserID := edge.UserID().String()
		if edgeUserID == "" {
			return fmt.Errorf("edge at index %d must have a valid user ID", i)
		}
		if i == 0 {
			userID = edgeUserID
		} else if edgeUserID != userID {
			return fmt.Errorf("all edges in batch must belong to the same user")
		}
	}
	
	// Process in batches of 25 (DynamoDB BatchWriteItem limit)
	const batchSize = 25
	
	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		
		batch := edges[i:end]
		if err := r.saveBatchChunk(ctx, userID, batch); err != nil {
			return fmt.Errorf("failed to save batch chunk: %w", err)
		}
	}
	
	return nil
}

// saveBatchChunk saves a chunk of up to 25 edges using BatchWriteItem
func (r *EdgeRepositoryCQRS) saveBatchChunk(ctx context.Context, userID string, edges []*edge.Edge) error {
	writeRequests := make([]types.WriteRequest, 0, len(edges)*2) // *2 for bidirectional edges
	
	for _, edge := range edges {
		// Create items for both directions (bidirectional storage)
		// Direction 1: source -> target
		item1 := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, edge.SourceID.String())},
			"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#%s", edge.TargetID.String())},
			"EdgeID":    &types.AttributeValueMemberS{Value: edge.ID.String()},
			"SourceID":  &types.AttributeValueMemberS{Value: edge.SourceID.String()},
			"TargetID":  &types.AttributeValueMemberS{Value: edge.TargetID.String()},
			"Weight":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", edge.Weight())},
			"CreatedAt": &types.AttributeValueMemberS{Value: edge.CreatedAt.Format(time.RFC3339)},
			"UpdatedAt": &types.AttributeValueMemberS{Value: edge.UpdatedAt.Format(time.RFC3339)},
			"Version":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", edge.Version)},
			"GSI2PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", userID)},
			"GSI2SK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.ID.String())},
		}
		
		// Direction 2: target -> source (reverse edge for bidirectional queries)
		item2 := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, edge.TargetID.String())},
			"SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#%s", edge.SourceID.String())},
			"EdgeID":    &types.AttributeValueMemberS{Value: edge.ID.String()},
			"SourceID":  &types.AttributeValueMemberS{Value: edge.SourceID.String()},
			"TargetID":  &types.AttributeValueMemberS{Value: edge.TargetID.String()},
			"Weight":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", edge.Weight())},
			"CreatedAt": &types.AttributeValueMemberS{Value: edge.CreatedAt.Format(time.RFC3339)},
			"UpdatedAt": &types.AttributeValueMemberS{Value: edge.UpdatedAt.Format(time.RFC3339)},
			"Version":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", edge.Version)},
		}
		
		writeRequests = append(writeRequests,
			types.WriteRequest{PutRequest: &types.PutRequest{Item: item1}},
			types.WriteRequest{PutRequest: &types.PutRequest{Item: item2}},
		)
	}
	
	// Execute batch write with retry logic
	maxRetries := 3
	retryDelay := 100 * time.Millisecond
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2
		}
		
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: writeRequests,
			},
		}
		
		output, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("batch write failed after %d attempts: %w", maxRetries, err)
			}
			r.logger.Warn("batch write attempt failed, retrying",
				zap.Int("attempt", attempt),
				zap.Error(err))
			continue
		}
		
		// Check for unprocessed items
		if output.UnprocessedItems != nil && len(output.UnprocessedItems[r.tableName]) > 0 {
			// Update writeRequests to only contain unprocessed items for retry
			writeRequests = output.UnprocessedItems[r.tableName]
			r.logger.Debug("retrying unprocessed items",
				zap.Int("count", len(writeRequests)))
		} else {
			// All items processed successfully
			return nil
		}
	}
	
	return fmt.Errorf("failed to process all items after %d attempts", maxRetries)
}

// UpdateWeight updates the weight of an edge with explicit userID.
func (r *EdgeRepositoryCQRS) UpdateWeight(ctx context.Context, userID shared.UserID, edgeID shared.NodeID, newWeight float64, expectedVersion shared.Version) error {
	
	// Build update expression
	update := expression.Set(expression.Name("Weight"), expression.Value(newWeight)).
		Set(expression.Name("UpdatedAt"), expression.Value(time.Now().Format(time.RFC3339))).
		Set(expression.Name("Version"), expression.Value(expectedVersion.Int()+1))
	
	// Build condition expression for optimistic locking
	condition := expression.Equal(expression.Name("Version"), expression.Value(expectedVersion.Int()))
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(condition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID.String())},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edgeID.String())},
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
		return fmt.Errorf("failed to update edge weight: %w", err)
	}
	
	return nil
}

// Delete deletes an edge with explicit userID.
// The edges are stored with PK=USER#userID#NODE#sourceID and SK=EDGE#RELATES_TO#targetID
func (r *EdgeRepositoryCQRS) Delete(ctx context.Context, userID shared.UserID, edgeID shared.NodeID) error {
	// This method can't work with just an edge ID because we need the full key structure
	// We need to know the source and target node IDs to construct the proper keys
	return fmt.Errorf("Delete by ID not supported - use DeleteEdgeByNodes instead")
}

// DeleteEdgeByNodes deletes an edge between two specific nodes.
// This uses the actual storage pattern where edges are stored with composite keys.
func (r *EdgeRepositoryCQRS) DeleteEdgeByNodes(ctx context.Context, userID string, sourceNodeID, targetNodeID string) error {
	
	// Edges are stored bidirectionally, so we need to delete both directions
	// First direction: source -> target
	key1 := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, sourceNodeID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#%s", targetNodeID)},
	}
	
	input1 := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key1,
	}
	
	_, err := r.client.DeleteItem(ctx, input1)
	if err != nil {
		r.logger.Warn("Failed to delete edge in first direction", 
			zap.String("source", sourceNodeID),
			zap.String("target", targetNodeID),
			zap.Error(err))
	}
	
	// Second direction: target -> source (reverse edge)
	key2 := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, targetNodeID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#RELATES_TO#%s", sourceNodeID)},
	}
	
	input2 := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key2,
	}
	
	_, err = r.client.DeleteItem(ctx, input2)
	if err != nil {
		r.logger.Warn("Failed to delete edge in reverse direction",
			zap.String("source", targetNodeID),
			zap.String("target", sourceNodeID),
			zap.Error(err))
	}
	
	return nil
}

// DeleteBatch deletes multiple edges in a batch with explicit userID.
func (r *EdgeRepositoryCQRS) DeleteBatch(ctx context.Context, userID shared.UserID, edgeIDs []shared.NodeID) error {
	for _, edgeID := range edgeIDs {
		if err := r.Delete(ctx, userID, edgeID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByNode deletes all edges connected to a node using batch operations with explicit userID.
func (r *EdgeRepositoryCQRS) DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error {
	
	log.Printf("DeleteByNode: Starting optimized batch deletion of edges for node %s", nodeID.String())
	
	// Collect all edge keys to delete
	keysToDelete := make([]map[string]types.AttributeValue, 0)
	
	// Method 1: Find edges where this node is the source
	// PK = USER#<userID>#NODE#<nodeID>, SK begins with EDGE#RELATES_TO#
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s#NODE#%s", userID.String(), nodeID.String())))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("EDGE#RELATES_TO#"))
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      aws.String("PK, SK"), // Only fetch keys for deletion
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		log.Printf("WARNING: Failed to query source edges: %v", err)
	} else {
		// Collect keys for batch deletion
		for _, item := range result.Items {
			if pk, ok := item["PK"].(*types.AttributeValueMemberS); ok {
				if sk, ok := item["SK"].(*types.AttributeValueMemberS); ok {
					keysToDelete = append(keysToDelete, map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk.Value},
						"SK": &types.AttributeValueMemberS{Value: sk.Value},
					})
				}
			}
		}
	}
	
	// Method 2: Find edges where this node is the target
	// Need to scan for SK = EDGE#RELATES_TO#<nodeID>
	filterEx := expression.And(
		expression.Name("PK").BeginsWith(fmt.Sprintf("USER#%s#NODE#", userID)),
		expression.Name("SK").Equal(expression.Value(fmt.Sprintf("EDGE#RELATES_TO#%s", nodeID.String()))),
	)
	
	expr2, err := expression.NewBuilder().
		WithFilter(filterEx).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build scan expression: %w", err)
	}
	
	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(r.tableName),
		FilterExpression:          expr2.Filter(),
		ExpressionAttributeNames:  expr2.Names(),
		ExpressionAttributeValues: expr2.Values(),
		ProjectionExpression:      aws.String("PK, SK"), // Only fetch keys for deletion
		Limit:                     aws.Int32(100), // Limit scan for performance
	}
	
	scanResult, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		log.Printf("WARNING: Failed to scan target edges: %v", err)
	} else {
		// Collect keys for batch deletion
		for _, item := range scanResult.Items {
			if pk, ok := item["PK"].(*types.AttributeValueMemberS); ok {
				if sk, ok := item["SK"].(*types.AttributeValueMemberS); ok {
					keysToDelete = append(keysToDelete, map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk.Value},
						"SK": &types.AttributeValueMemberS{Value: sk.Value},
					})
				}
			}
		}
	}
	
	// Now perform batch deletion of all collected keys
	if len(keysToDelete) == 0 {
		log.Printf("DeleteByNode: No edges found for node %s", nodeID.String())
		return nil
	}
	
	log.Printf("DeleteByNode: Deleting %d edges for node %s", len(keysToDelete), nodeID.String())
	
	// Process in batches of 25 (DynamoDB BatchWriteItem limit)
	const batchSize = 25
	deletedCount := 0
	failedCount := 0
	
	for i := 0; i < len(keysToDelete); i += batchSize {
		end := i + batchSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}
		
		batch := keysToDelete[i:end]
		writeRequests := make([]types.WriteRequest, 0, len(batch))
		
		for _, key := range batch {
			writeRequests = append(writeRequests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: key,
				},
			})
		}
		
		// Execute batch write with retry
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: writeRequests,
			},
		}
		
		output, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			log.Printf("ERROR: BatchWriteItem failed: %v", err)
			failedCount += len(batch)
		} else {
			deletedCount += len(batch)
			
			// Check for unprocessed items
			if output.UnprocessedItems != nil && len(output.UnprocessedItems[r.tableName]) > 0 {
				unprocessed := len(output.UnprocessedItems[r.tableName])
				deletedCount -= unprocessed
				failedCount += unprocessed
				log.Printf("WARNING: %d unprocessed items in batch", unprocessed)
			}
		}
	}
	
	log.Printf("DeleteByNode complete: deleted=%d, failed=%d", deletedCount, failedCount)
	
	return nil
}

// SaveManyToOne creates multiple edges from many sources to one target.
func (r *EdgeRepositoryCQRS) SaveManyToOne(ctx context.Context, userID shared.UserID, sourceID shared.NodeID, targetIDs []shared.NodeID, weights []float64) error {
	if len(targetIDs) != len(weights) {
		return fmt.Errorf("targetIDs and weights must have the same length")
	}
	
	edges := make([]*edge.Edge, len(targetIDs))
	for i, targetID := range targetIDs {
		edge, err := edge.NewEdge(sourceID, targetID, userID, weights[i])
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
		edges[i] = edge
	}
	
	return r.SaveBatch(ctx, edges)
}

// SaveOneToMany creates multiple edges from one source to many targets.
func (r *EdgeRepositoryCQRS) SaveOneToMany(ctx context.Context, userID shared.UserID, sourceIDs []shared.NodeID, targetID shared.NodeID, weights []float64) error {
	if len(sourceIDs) != len(weights) {
		return fmt.Errorf("sourceIDs and weights must have the same length")
	}
	
	edges := make([]*edge.Edge, len(sourceIDs))
	for i, sourceID := range sourceIDs {
		edge, err := edge.NewEdge(sourceID, targetID, userID, weights[i])
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
		edges[i] = edge
	}
	
	return r.SaveBatch(ctx, edges)
}

// ============================================================================
// EDGE REPOSITORY INTERFACE - Additional Methods for Compatibility
// ============================================================================

// CreateEdges creates bidirectional edges between a source node and multiple related nodes.
func (r *EdgeRepositoryCQRS) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	// No need to add to context - edges will contain userID
	
	sourceID, err := shared.ParseNodeID(sourceNodeID)
	if err != nil {
		return fmt.Errorf("invalid source node ID: %w", err)
	}
	
	uid, err := shared.NewUserID(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	
	edges := make([]*edge.Edge, 0, len(relatedNodeIDs))
	for _, relatedID := range relatedNodeIDs {
		targetID, err := shared.ParseNodeID(relatedID)
		if err != nil {
			continue
		}
		
		edge, err := edge.NewEdge(sourceID, targetID, uid, 1.0)
		if err != nil {
			continue
		}
		edges = append(edges, edge)
	}
	
	return r.SaveBatch(ctx, edges)
}

// CreateEdge creates a single edge.
func (r *EdgeRepositoryCQRS) CreateEdge(ctx context.Context, edge *edge.Edge) error {
	// Simply delegate to Save which uses the edge's userID
	return r.Save(ctx, edge)
}

// GetEdgesPage retrieves a paginated list of edges.
func (r *EdgeRepositoryCQRS) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return r.FindPage(ctx, query, pagination)
}

// FindEdgesWithOptions retrieves edges with query options.
func (r *EdgeRepositoryCQRS) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// No need to add to context - userID is in the query
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	return r.FindByUser(ctx, userID, opts...)
}

// DeleteEdge deletes a single edge by ID.
func (r *EdgeRepositoryCQRS) DeleteEdge(ctx context.Context, userID, edgeID string) error {
	uid, err := shared.NewUserID(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	
	eid, err := shared.ParseNodeID(edgeID)
	if err != nil {
		return fmt.Errorf("invalid edge ID: %w", err)
	}
	
	return r.Delete(ctx, uid, eid)
}

// DeleteEdgesByNode deletes all edges connected to a specific node.
func (r *EdgeRepositoryCQRS) DeleteEdgesByNode(ctx context.Context, userID, nodeID string) error {
	uid, err := shared.NewUserID(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	
	nid, err := shared.ParseNodeID(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %w", err)
	}
	
	return r.DeleteByNode(ctx, uid, nid)
}

// DeleteEdgesBetweenNodes deletes all edges between two specific nodes.
func (r *EdgeRepositoryCQRS) DeleteEdgesBetweenNodes(ctx context.Context, userID, sourceNodeID, targetNodeID string) error {
	return r.DeleteEdgeByNodes(ctx, userID, sourceNodeID, targetNodeID)
}

// ============================================================================
// HELPER METHODS
// ============================================================================


// parseEdgeFromItem parses a DynamoDB item into an Edge domain object.
func (r *EdgeRepositoryCQRS) parseEdgeFromItem(item map[string]types.AttributeValue, userID string) (*edge.Edge, error) {
	// Extract IDs from the item
	var edgeID, sourceID, targetID string
	var weight float64 = 1.0
	var version int = 1
	
	// Extract source ID from PK (format: USER#<userID>#NODE#<sourceID>)
	if pk, ok := item["PK"].(*types.AttributeValueMemberS); ok {
		pkParts := strings.Split(pk.Value, "#")
		if len(pkParts) == 4 && pkParts[0] == "USER" && pkParts[2] == "NODE" {
			sourceID = pkParts[3]
		}
	}
	
	// Try to get SourceID from direct field if available (for newer records)
	if v, ok := item["SourceID"].(*types.AttributeValueMemberS); ok && v.Value != "" {
		sourceID = v.Value
	}
	
	// Extract target ID from TargetID field or SK
	if v, ok := item["TargetID"].(*types.AttributeValueMemberS); ok {
		targetID = v.Value
	} else if sk, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		// SK format: EDGE#RELATES_TO#<targetID>
		skParts := strings.Split(sk.Value, "#")
		if len(skParts) >= 3 {
			targetID = skParts[2]
		}
	}
	
	// Try to get EdgeID from direct field
	if v, ok := item["EdgeID"].(*types.AttributeValueMemberS); ok {
		edgeID = v.Value
	}
	
	// Generate edge ID if not available
	if edgeID == "" && sourceID != "" && targetID != "" {
		edgeID = fmt.Sprintf("%s-%s", sourceID, targetID)
	}
	
	if v, ok := item["Weight"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%f", &weight)
	}
	if v, ok := item["Version"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%d", &version)
	}
	
	// Parse timestamps
	createdAt := time.Now()
	
	if v, ok := item["CreatedAt"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			createdAt = t
		}
	}
	
	// Validate we have the required IDs
	if sourceID == "" || targetID == "" {
		return nil, fmt.Errorf("missing source or target ID in edge item")
	}
	
	// Create domain objects
	eid, err := shared.ParseNodeID(edgeID)
	if err != nil {
		// Generate a new edge ID if parsing fails
		eid = shared.NewNodeID()
	}
	
	sid, err := shared.ParseNodeID(sourceID)
	if err != nil {
		return nil, fmt.Errorf("invalid source ID: %w", err)
	}
	
	tid, err := shared.ParseNodeID(targetID)
	if err != nil {
		return nil, fmt.Errorf("invalid target ID: %w", err)
	}
	
	uid, _ := shared.NewUserID(userID)
	
	// Reconstruct edge using domain reconstruction method
	edge := edge.ReconstructEdge(
		eid,
		sid,
		tid,
		uid,
		weight,
		createdAt,
		shared.ParseVersion(version),
	)
	
	// Note: Edge doesn't currently have metadata field in domain model
	// Metadata parsing would go here if added to domain model
	
	return edge, nil
}