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
	sharedContext "brain2-backend/internal/context"
	
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
	_ repository.EdgeRepository = (*EdgeRepositoryCQRS)(nil)
)

// ============================================================================
// EDGE READER INTERFACE - Read Operations
// ============================================================================

// FindByID retrieves an edge by its ID.
func (r *EdgeRepositoryCQRS) FindByID(ctx context.Context, id shared.NodeID) (*edge.Edge, error) {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	// Build the composite key for DynamoDB
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", id.String())},
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
	edge, err := r.parseEdgeFromItem(result.Item, userID)
	if err != nil {
		return nil, err
	}
	
	return edge, nil
}

// Exists checks if an edge exists.
func (r *EdgeRepositoryCQRS) Exists(ctx context.Context, id shared.NodeID) (bool, error) {
	edge, err := r.FindByID(ctx, id)
	if err == repository.ErrEdgeNotFound {
		return false, nil
	}
	return edge != nil, err
}

// FindByUser retrieves all edges for a user.
func (r *EdgeRepositoryCQRS) FindByUser(ctx context.Context, userID shared.UserID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	// Apply query options
	options := repository.ApplyQueryOptions(opts...)
	
	// Scan for all edges belonging to this user
	// Filter: PK begins with USER#<userID>#NODE# AND SK begins with EDGE#
	filterEx := expression.And(
		expression.Name("PK").BeginsWith(fmt.Sprintf("USER#%s#NODE#", userID.String())),
		expression.Name("SK").BeginsWith("EDGE#"),
	)
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithFilter(filterEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.ScanInput{
		TableName:                 aws.String(r.tableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(options.Limit)),
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan edges: %w", err)
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

// FindBySourceNode finds edges originating from a specific node.
func (r *EdgeRepositoryCQRS) FindBySourceNode(ctx context.Context, sourceID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
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
		edge, err := r.parseEdgeFromItem(item, userID)
		if err != nil {
			r.logger.Warn("Failed to parse edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}
	
	return edges, nil
}

// FindByTargetNode finds edges pointing to a specific node.
func (r *EdgeRepositoryCQRS) FindByTargetNode(ctx context.Context, targetID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	// Apply query options
	options := repository.ApplyQueryOptions(opts...)
	
	// We need to scan because target node is in the SK
	// Filter: PK begins with USER#<userID>#NODE# AND SK = EDGE#RELATES_TO#<targetID>
	filterEx := expression.And(
		expression.Name("PK").BeginsWith(fmt.Sprintf("USER#%s#NODE#", userID)),
		expression.Name("SK").Equal(expression.Value(fmt.Sprintf("EDGE#RELATES_TO#%s", targetID.String()))),
	)
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithFilter(filterEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.ScanInput{
		TableName:                 aws.String(r.tableName),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(options.Limit)),
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan edges: %w", err)
	}
	
	edges := make([]*edge.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		edge, err := r.parseEdgeFromItem(item, userID)
		if err != nil {
			r.logger.Warn("Failed to parse edge", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}
	
	return edges, nil
}

// FindByNode finds all edges connected to a specific node.
func (r *EdgeRepositoryCQRS) FindByNode(ctx context.Context, nodeID shared.NodeID, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	uid, _ := shared.NewUserID(userID)
	edges, err := r.FindByUser(ctx, uid, opts...)
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

// FindBetweenNodes finds edges between two specific nodes.
func (r *EdgeRepositoryCQRS) FindBetweenNodes(ctx context.Context, node1ID, node2ID shared.NodeID) ([]*edge.Edge, error) {
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user ID not found in context")
	}
	
	uid, _ := shared.NewUserID(userID)
	edges, err := r.FindByUser(ctx, uid)
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
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
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

// FindEdges is a compatibility method for query service.
func (r *EdgeRepositoryCQRS) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	// Check if we have a source node filter
	if query.SourceID != "" {
		sourceID, err := shared.ParseNodeID(query.SourceID)
		if err != nil {
			return nil, fmt.Errorf("invalid source node ID: %w", err)
		}
		return r.FindBySourceNode(ctx, sourceID)
	}
	
	// Check if we have a target node filter
	if query.TargetID != "" {
		targetID, err := shared.ParseNodeID(query.TargetID)
		if err != nil {
			return nil, fmt.Errorf("invalid target node ID: %w", err)
		}
		return r.FindByTargetNode(ctx, targetID)
	}
	
	// Otherwise return all edges for the user
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	return r.FindByUser(ctx, userID)
}

// CountBySourceID counts edges from a source node.
func (r *EdgeRepositoryCQRS) CountBySourceID(ctx context.Context, sourceID shared.NodeID) (int, error) {
	edges, err := r.FindBySourceNode(ctx, sourceID)
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
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
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

// SaveBatch saves multiple edges in a batch.
func (r *EdgeRepositoryCQRS) SaveBatch(ctx context.Context, edges []*edge.Edge) error {
	// Process in batches of 25 (DynamoDB limit)
	const batchSize = 25
	
	for i := 0; i < len(edges); i += batchSize {
		end := i + batchSize
		if end > len(edges) {
			end = len(edges)
		}
		
		batch := edges[i:end]
		for _, edge := range batch {
			if err := r.Save(ctx, edge); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// UpdateWeight updates the weight of an edge.
func (r *EdgeRepositoryCQRS) UpdateWeight(ctx context.Context, id shared.NodeID, newWeight float64, expectedVersion shared.Version) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
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
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", id.String())},
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

// Delete deletes an edge using the actual edge storage pattern.
// The edges are stored with PK=USER#userID#NODE#sourceID and SK=EDGE#RELATES_TO#targetID
func (r *EdgeRepositoryCQRS) Delete(ctx context.Context, id shared.NodeID) error {
	// This method can't work with just an edge ID because we need the full key structure
	// We need to know the source and target node IDs to construct the proper keys
	return fmt.Errorf("Delete by ID not supported - use DeleteEdgeByNodes instead")
}

// DeleteEdgeByNodes deletes an edge between two specific nodes.
// This uses the actual storage pattern where edges are stored with composite keys.
func (r *EdgeRepositoryCQRS) DeleteEdgeByNodes(ctx context.Context, sourceNodeID, targetNodeID string) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
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

// DeleteBatch deletes multiple edges in a batch.
func (r *EdgeRepositoryCQRS) DeleteBatch(ctx context.Context, ids []shared.NodeID) error {
	for _, id := range ids {
		if err := r.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByNode deletes all edges connected to a node.
func (r *EdgeRepositoryCQRS) DeleteByNode(ctx context.Context, nodeID shared.NodeID) error {
	// Extract userID from context
	userID, ok := sharedContext.GetUserIDFromContext(ctx)
	if !ok {
		return fmt.Errorf("user ID not found in context")
	}
	
	log.Printf("DeleteByNode: Starting deletion of edges for node %s", nodeID.String())
	
	deletedCount := 0
	failedCount := 0
	
	// Method 1: Delete edges where this node is the source
	// PK = USER#<userID>#NODE#<nodeID>, SK begins with EDGE#RELATES_TO#
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID.String())))
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
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		log.Printf("WARNING: Failed to query source edges: %v", err)
	} else {
		// Delete each edge found
		for _, item := range result.Items {
			if pk, ok := item["PK"].(*types.AttributeValueMemberS); ok {
				if sk, ok := item["SK"].(*types.AttributeValueMemberS); ok {
					key := map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk.Value},
						"SK": &types.AttributeValueMemberS{Value: sk.Value},
					}
					
					delInput := &dynamodb.DeleteItemInput{
						TableName: aws.String(r.tableName),
						Key:       key,
					}
					
					_, err := r.client.DeleteItem(ctx, delInput)
					if err != nil {
						log.Printf("Failed to delete edge PK=%s SK=%s: %v", pk.Value, sk.Value, err)
						failedCount++
					} else {
						log.Printf("Deleted edge PK=%s SK=%s", pk.Value, sk.Value)
						deletedCount++
					}
				}
			}
		}
	}
	
	// Method 2: Delete edges where this node is the target
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
	}
	
	scanResult, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		log.Printf("WARNING: Failed to scan target edges: %v", err)
	} else {
		// Delete each edge found
		for _, item := range scanResult.Items {
			if pk, ok := item["PK"].(*types.AttributeValueMemberS); ok {
				if sk, ok := item["SK"].(*types.AttributeValueMemberS); ok {
					key := map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk.Value},
						"SK": &types.AttributeValueMemberS{Value: sk.Value},
					}
					
					delInput := &dynamodb.DeleteItemInput{
						TableName: aws.String(r.tableName),
						Key:       key,
					}
					
					_, err := r.client.DeleteItem(ctx, delInput)
					if err != nil {
						log.Printf("Failed to delete edge PK=%s SK=%s: %v", pk.Value, sk.Value, err)
						failedCount++
					} else {
						log.Printf("Deleted edge PK=%s SK=%s", pk.Value, sk.Value)
						deletedCount++
					}
				}
			}
		}
	}
	
	log.Printf("DeleteByNode complete: deleted=%d, failed=%d", deletedCount, failedCount)
	
	return nil
}

// SaveManyToOne creates multiple edges from many sources to one target.
func (r *EdgeRepositoryCQRS) SaveManyToOne(ctx context.Context, sourceID shared.NodeID, targetIDs []shared.NodeID, weights []float64) error {
	if len(targetIDs) != len(weights) {
		return fmt.Errorf("targetIDs and weights must have the same length")
	}
	
	// Get userID from context
	userID, _ := sharedContext.GetUserIDFromContext(ctx)
	uid, _ := shared.NewUserID(userID)
	
	edges := make([]*edge.Edge, len(targetIDs))
	for i, targetID := range targetIDs {
		edge, err := edge.NewEdge(sourceID, targetID, uid, weights[i])
		if err != nil {
			return fmt.Errorf("failed to create edge: %w", err)
		}
		edges[i] = edge
	}
	
	return r.SaveBatch(ctx, edges)
}

// SaveOneToMany creates multiple edges from one source to many targets.
func (r *EdgeRepositoryCQRS) SaveOneToMany(ctx context.Context, sourceIDs []shared.NodeID, targetID shared.NodeID, weights []float64) error {
	if len(sourceIDs) != len(weights) {
		return fmt.Errorf("sourceIDs and weights must have the same length")
	}
	
	// Get userID from context
	userID, _ := sharedContext.GetUserIDFromContext(ctx)
	uid, _ := shared.NewUserID(userID)
	
	edges := make([]*edge.Edge, len(sourceIDs))
	for i, sourceID := range sourceIDs {
		edge, err := edge.NewEdge(sourceID, targetID, uid, weights[i])
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
	ctx = sharedContext.WithUserID(ctx, userID)
	
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
	// Add userID to context if available from edge
	userID := edge.UserID()
	if userID.String() != "" {
		ctx = sharedContext.WithUserID(ctx, userID.String())
	}
	return r.Save(ctx, edge)
}

// GetEdgesPage retrieves a paginated list of edges.
func (r *EdgeRepositoryCQRS) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return r.FindPage(ctx, query, pagination)
}

// FindEdgesWithOptions retrieves edges with query options.
func (r *EdgeRepositoryCQRS) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	
	userID, err := shared.NewUserID(query.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	
	return r.FindByUser(ctx, userID, opts...)
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