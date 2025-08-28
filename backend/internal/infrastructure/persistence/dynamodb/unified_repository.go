// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file implements a unified repository that directly implements all
// repository interfaces using direct CQRS patterns.
package dynamodb

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/aws"
)

// UnifiedRepository implements all repository interfaces directly.
type UnifiedRepository struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	config    *config.Database
}

// NewUnifiedRepository creates a new unified repository.
func NewUnifiedRepository(
	client *dynamodb.Client,
	config *config.Database,
) *UnifiedRepository {
	return &UnifiedRepository{
		client:    client,
		tableName: config.TableName,
		indexName: config.IndexName,
		config:    config,
	}
}

// ============================================================================
// NODE REPOSITORY INTERFACE
// ============================================================================

// CreateNode creates a new node in DynamoDB.
func (r *UnifiedRepository) CreateNode(ctx context.Context, node *node.Node) error {
	item, err := attributevalue.MarshalMap(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	
	return nil
}

// GetNodeByID retrieves a node by its ID.
func (r *UnifiedRepository) GetNodeByID(ctx context.Context, nodeID string) (*node.Node, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrNodeNotFound("", "")
	}
	
	var node node.Node
	if err := attributevalue.UnmarshalMap(result.Item, &node); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}
	
	return &node, nil
}

// UpdateNode updates an existing node.
func (r *UnifiedRepository) UpdateNode(ctx context.Context, node *node.Node) error {
	item, err := attributevalue.MarshalMap(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	
	return nil
}

// DeleteNode deletes a node by ID.
func (r *UnifiedRepository) DeleteNode(ctx context.Context, nodeID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	
	return nil
}

// ListNodesByUserID lists nodes for a specific user with pagination.
func (r *UnifiedRepository) ListNodesByUserID(ctx context.Context, userID string, limit, offset int) ([]*node.Node, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(r.indexName),
		KeyConditionExpression: aws.String("user_id = :userId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userID},
		},
		Limit: aws.Int32(int32(limit)),
	}
	
	// Handle pagination with ExclusiveStartKey if offset > 0
	// This is simplified - in production, use proper cursor-based pagination
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	
	nodes := make([]*node.Node, 0, len(result.Items))
	for _, item := range result.Items {
		var node node.Node
		if err := attributevalue.UnmarshalMap(item, &node); err != nil {
			continue // Skip invalid items
		}
		nodes = append(nodes, &node)
	}
	
	return nodes, nil
}

// ============================================================================
// EDGE REPOSITORY INTERFACE
// ============================================================================

// CreateEdge creates a new edge in DynamoDB.
func (r *UnifiedRepository) CreateEdge(ctx context.Context, edge *edge.Edge) error {
	item, err := attributevalue.MarshalMap(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create edge: %w", err)
	}
	
	return nil
}

// GetEdgeByID retrieves an edge by its ID.
func (r *UnifiedRepository) GetEdgeByID(ctx context.Context, edgeID string) (*edge.Edge, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: edgeID},
		},
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrEdgeNotFound("", "")
	}
	
	var edge edge.Edge
	if err := attributevalue.UnmarshalMap(result.Item, &edge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal edge: %w", err)
	}
	
	return &edge, nil
}

// UpdateEdge updates an existing edge.
func (r *UnifiedRepository) UpdateEdge(ctx context.Context, edge *edge.Edge) error {
	item, err := attributevalue.MarshalMap(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update edge: %w", err)
	}
	
	return nil
}

// DeleteEdge deletes an edge by ID.
func (r *UnifiedRepository) DeleteEdge(ctx context.Context, edgeID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: edgeID},
		},
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}
	
	return nil
}

// GetEdgesByNodeID gets all edges connected to a specific node.
func (r *UnifiedRepository) GetEdgesByNodeID(ctx context.Context, nodeID string) ([]*edge.Edge, error) {
	// Query for edges where node is source
	sourceEdges, err := r.queryEdges(ctx, "source_id", nodeID)
	if err != nil {
		return nil, err
	}
	
	// Query for edges where node is target
	targetEdges, err := r.queryEdges(ctx, "target_id", nodeID)
	if err != nil {
		return nil, err
	}
	
	// Combine results
	edges := append(sourceEdges, targetEdges...)
	return edges, nil
}

// GetEdgesBetweenNodes gets edges between two specific nodes.
func (r *UnifiedRepository) GetEdgesBetweenNodes(ctx context.Context, sourceID, targetID string) ([]*edge.Edge, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("source-target-index"),
		KeyConditionExpression: aws.String("source_id = :source AND target_id = :target"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":source": &types.AttributeValueMemberS{Value: sourceID},
			":target": &types.AttributeValueMemberS{Value: targetID},
		},
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}
	
	edges := make([]*edge.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		var edge edge.Edge
		if err := attributevalue.UnmarshalMap(item, &edge); err != nil {
			continue
		}
		edges = append(edges, &edge)
	}
	
	return edges, nil
}

// ============================================================================
// CATEGORY REPOSITORY INTERFACE
// ============================================================================

// CreateCategory creates a new category.
func (r *UnifiedRepository) CreateCategory(ctx context.Context, category *category.Category) error {
	item, err := attributevalue.MarshalMap(category)
	if err != nil {
		return fmt.Errorf("failed to marshal category: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	
	return nil
}

// GetCategoryByID retrieves a category by ID.
func (r *UnifiedRepository) GetCategoryByID(ctx context.Context, categoryID string) (*category.Category, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: categoryID},
		},
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrCategoryNotFound("", "")
	}
	
	var category category.Category
	if err := attributevalue.UnmarshalMap(result.Item, &category); err != nil {
		return nil, fmt.Errorf("failed to unmarshal category: %w", err)
	}
	
	return &category, nil
}

// UpdateCategory updates an existing category.
func (r *UnifiedRepository) UpdateCategory(ctx context.Context, category *category.Category) error {
	item, err := attributevalue.MarshalMap(category)
	if err != nil {
		return fmt.Errorf("failed to marshal category: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	
	return nil
}

// DeleteCategory deletes a category.
func (r *UnifiedRepository) DeleteCategory(ctx context.Context, categoryID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: categoryID},
		},
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	
	return nil
}

// ListCategoriesByUserID lists all categories for a user.
func (r *UnifiedRepository) ListCategoriesByUserID(ctx context.Context, userID string) ([]*category.Category, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(r.indexName),
		KeyConditionExpression: aws.String("user_id = :userId AND entity_type = :type"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userId": &types.AttributeValueMemberS{Value: userID},
			":type":   &types.AttributeValueMemberS{Value: "category"},
		},
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	
	categories := make([]*category.Category, 0, len(result.Items))
	for _, item := range result.Items {
		var category category.Category
		if err := attributevalue.UnmarshalMap(item, &category); err != nil {
			continue
		}
		categories = append(categories, &category)
	}
	
	return categories, nil
}

// AddNodeToCategory adds a node to a category.
func (r *UnifiedRepository) AddNodeToCategory(ctx context.Context, categoryID string, nodeID shared.NodeID) error {
	// Create a node-category relationship record
	relationship := map[string]interface{}{
		"id":          fmt.Sprintf("nc_%s_%s", categoryID, nodeID),
		"category_id": categoryID,
		"node_id":     nodeID.String(),
		"entity_type": "node_category",
		"created_at":  time.Now().Unix(),
	}
	
	item, err := attributevalue.MarshalMap(relationship)
	if err != nil {
		return fmt.Errorf("failed to marshal relationship: %w", err)
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to add node to category: %w", err)
	}
	
	return nil
}

// RemoveNodeFromCategory removes a node from a category.
func (r *UnifiedRepository) RemoveNodeFromCategory(ctx context.Context, categoryID string, nodeID shared.NodeID) error {
	relationshipID := fmt.Sprintf("nc_%s_%s", categoryID, nodeID)
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: relationshipID},
		},
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to remove node from category: %w", err)
	}
	
	return nil
}

// GetNodesInCategory gets all nodes in a specific category.
func (r *UnifiedRepository) GetNodesInCategory(ctx context.Context, categoryID string) ([]*node.Node, error) {
	// First, get all node IDs in the category
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("category-index"),
		KeyConditionExpression: aws.String("category_id = :categoryId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":categoryId": &types.AttributeValueMemberS{Value: categoryID},
		},
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes in category: %w", err)
	}
	
	// Extract node IDs
	nodeIDs := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if nodeID, ok := item["node_id"]; ok {
			if s, ok := nodeID.(*types.AttributeValueMemberS); ok {
				nodeIDs = append(nodeIDs, s.Value)
			}
		}
	}
	
	// Batch get nodes
	nodes := make([]*node.Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		node, err := r.GetNodeByID(ctx, nodeID)
		if err != nil {
			continue // Skip nodes that can't be fetched
		}
		nodes = append(nodes, node)
	}
	
	return nodes, nil
}

// GetCategoriesByNodeID gets all categories for a specific node.
func (r *UnifiedRepository) GetCategoriesByNodeID(ctx context.Context, nodeID string) ([]*category.Category, error) {
	// Query for all category relationships for this node
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("node-category-index"),
		KeyConditionExpression: aws.String("node_id = :nodeId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":nodeId": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories for node: %w", err)
	}
	
	// Extract category IDs
	categoryIDs := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if catID, ok := item["category_id"]; ok {
			if s, ok := catID.(*types.AttributeValueMemberS); ok {
				categoryIDs = append(categoryIDs, s.Value)
			}
		}
	}
	
	// Batch get categories
	categories := make([]*category.Category, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		category, err := r.GetCategoryByID(ctx, categoryID)
		if err != nil {
			continue
		}
		categories = append(categories, category)
	}
	
	return categories, nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// queryEdges is a helper method to query edges by a specific attribute.
func (r *UnifiedRepository) queryEdges(ctx context.Context, attributeName, value string) ([]*edge.Edge, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(fmt.Sprintf("%s-index", attributeName)),
		KeyConditionExpression: aws.String(fmt.Sprintf("%s = :value", attributeName)),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":value": &types.AttributeValueMemberS{Value: value},
		},
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}
	
	edges := make([]*edge.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		var edge edge.Edge
		if err := attributevalue.UnmarshalMap(item, &edge); err != nil {
			continue
		}
		edges = append(edges, &edge)
	}
	
	return edges, nil
}