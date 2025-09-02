// Package dynamodb provides DynamoDB implementations of core domain repositories
package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/specifications"
	"brain2-backend/internal/core/domain/valueobjects"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// NodeRepository implements the ports.NodeRepository interface using DynamoDB
type NodeRepository struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    ports.Logger
}

// NewNodeRepository creates a new DynamoDB node repository
func NewNodeRepository(client *dynamodb.Client, tableName, indexName string, logger ports.Logger) *NodeRepository {
	return &NodeRepository{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// nodeItem represents the DynamoDB item structure for a node
type nodeItem struct {
	PK           string                 `dynamodbav:"PK"`
	SK           string                 `dynamodbav:"SK"`
	NodeID       string                 `dynamodbav:"NodeID"`
	UserID       string                 `dynamodbav:"UserID"`
	Content      string                 `dynamodbav:"Content"`
	Title        string                 `dynamodbav:"Title,omitempty"`
	Tags         []string               `dynamodbav:"Tags,omitempty"`
	Keywords     []string               `dynamodbav:"Keywords,omitempty"`
	CategoryIDs  []string               `dynamodbav:"CategoryIDs,omitempty"`
	IsArchived   bool                   `dynamodbav:"IsArchived"`
	Version      int64                  `dynamodbav:"Version"`
	CreatedAt    string                 `dynamodbav:"CreatedAt"`
	UpdatedAt    string                 `dynamodbav:"UpdatedAt"`
	Metadata     map[string]interface{} `dynamodbav:"Metadata,omitempty"`
	EntityType   string                 `dynamodbav:"EntityType"`
	GSI1PK       string                 `dynamodbav:"GSI1PK,omitempty"`
	GSI1SK       string                 `dynamodbav:"GSI1SK,omitempty"`
}

// Save persists a node aggregate to DynamoDB
func (r *NodeRepository) Save(ctx context.Context, aggregate *node.Aggregate) error {
	if aggregate == nil {
		return fmt.Errorf("aggregate cannot be nil")
	}

	// Convert aggregate to DynamoDB item
	item := r.aggregateToItem(aggregate)

	// Marshal to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		r.logger.Error("Failed to marshal node item", err,
			ports.Field{Key: "node_id", Value: aggregate.GetID()})
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	// Prepare put item input with conditional check for optimistic locking
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	}

	// Add version check for updates (optimistic locking)
	if aggregate.GetVersion() > 1 {
		input.ConditionExpression = aws.String("Version = :prev_version OR attribute_not_exists(PK)")
		input.ExpressionAttributeValues = map[string]types.AttributeValue{
			":prev_version": &types.AttributeValueMemberN{
				Value: fmt.Sprintf("%d", aggregate.GetVersion()-1),
			},
		}
	}

	// Execute put operation
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		r.logger.Error("Failed to save node to DynamoDB", err,
			ports.Field{Key: "node_id", Value: aggregate.GetID()})
		return fmt.Errorf("failed to save node: %w", err)
	}

	r.logger.Debug("Node saved successfully",
		ports.Field{Key: "node_id", Value: aggregate.GetID()},
		ports.Field{Key: "version", Value: aggregate.GetVersion()})

	return nil
}

// FindByID retrieves a node by its ID
func (r *NodeRepository) FindByID(ctx context.Context, id string) (*node.Aggregate, error) {
	// For this implementation, we need to know the user ID
	// In a real implementation, you might need to query GSI or scan
	// For now, we'll implement a simplified version
	
	// Build the key
	key := map[string]types.AttributeValue{
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id)},
	}

	// Query using GSI if we have an index on NodeID
	// Otherwise, we'd need to scan (not recommended for production)
	expr, err := expression.NewBuilder().
		WithKeyCondition(expression.Key("NodeID").Equal(expression.Value(id))).
		Build()
	
	if err != nil {
		return nil, fmt.Errorf("failed to build query expression: %w", err)
	}

	// Query the GSI
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		IndexName:                 aws.String("NodeIDIndex"), // Assuming we have this GSI
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(1),
	})

	if err != nil {
		r.logger.Error("Failed to query node by ID", err,
			ports.Field{Key: "node_id", Value: id})
		return nil, fmt.Errorf("failed to find node: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	// Unmarshal the item
	var item nodeItem
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}

	// Convert to aggregate
	return r.itemToAggregate(&item)
}

// GetByID is an alias for FindByID
func (r *NodeRepository) GetByID(ctx context.Context, id string) (*node.Aggregate, error) {
	return r.FindByID(ctx, id)
}

// FindBySpecification retrieves nodes matching a specification
func (r *NodeRepository) FindBySpecification(ctx context.Context, spec specifications.Specification[*node.Aggregate]) ([]*node.Aggregate, error) {
	// This is a simplified implementation
	// In a real implementation, you'd translate specifications to DynamoDB queries
	
	// For now, we'll scan and filter in memory (not recommended for production)
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
		FilterExpression: aws.String("EntityType = :entity_type"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":entity_type": &types.AttributeValueMemberS{Value: "NODE"},
		},
	}

	result, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		r.logger.Error("Failed to scan nodes", err)
		return nil, fmt.Errorf("failed to find nodes: %w", err)
	}

	var aggregates []*node.Aggregate
	for _, item := range result.Items {
		var nodeItem nodeItem
		if err := attributevalue.UnmarshalMap(item, &nodeItem); err != nil {
			r.logger.Warn("Failed to unmarshal node item", 
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		aggregate, err := r.itemToAggregate(&nodeItem)
		if err != nil {
			r.logger.Warn("Failed to convert item to aggregate",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}

		// Apply specification
		if spec.IsSatisfiedBy(aggregate) {
			aggregates = append(aggregates, aggregate)
		}
	}

	return aggregates, nil
}

// Delete removes a node from DynamoDB
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	// Similar to FindByID, we need to construct the proper key
	// This is simplified - in production you'd need the full composite key
	
	key := map[string]types.AttributeValue{
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", id)},
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	})

	if err != nil {
		r.logger.Error("Failed to delete node", err,
			ports.Field{Key: "node_id", Value: id})
		return fmt.Errorf("failed to delete node: %w", err)
	}

	return nil
}

// Exists checks if a node exists
func (r *NodeRepository) Exists(ctx context.Context, id string) (bool, error) {
	aggregate, err := r.FindByID(ctx, id)
	if err != nil {
		if err.Error() == fmt.Sprintf("node not found: %s", id) {
			return false, nil
		}
		return false, err
	}
	return aggregate != nil, nil
}

// Count returns the total number of nodes matching a specification
func (r *NodeRepository) Count(ctx context.Context, spec specifications.Specification[*node.Aggregate]) (int64, error) {
	// Simplified implementation - in production, use DynamoDB COUNT
	nodes, err := r.FindBySpecification(ctx, spec)
	if err != nil {
		return 0, err
	}
	return int64(len(nodes)), nil
}

// aggregateToItem converts a node aggregate to a DynamoDB item
func (r *NodeRepository) aggregateToItem(aggregate *node.Aggregate) *nodeItem {
	now := time.Now().Format(time.RFC3339)
	
	// Extract data from aggregate
	nodeID := aggregate.GetID()
	userID := aggregate.GetUserID()
	
	// Build composite keys
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("NODE#%s", nodeID)
	
	// For GSI (if needed for queries)
	gsi1pk := "NODE"
	gsi1sk := fmt.Sprintf("%s#%s", now, nodeID)

	item := &nodeItem{
		PK:          pk,
		SK:          sk,
		NodeID:      nodeID,
		UserID:      userID,
		Content:     aggregate.GetContent(),
		Title:       aggregate.GetTitle(),
		Tags:        aggregate.GetTags(),
		Keywords:    aggregate.GetKeywords(),
		CategoryIDs: aggregate.GetCategoryIDs(),
		IsArchived:  aggregate.IsArchived(),
		Version:     aggregate.GetVersion(),
		CreatedAt:   aggregate.GetCreatedAt().Format(time.RFC3339),
		UpdatedAt:   aggregate.GetUpdatedAt().Format(time.RFC3339),
		EntityType:  "NODE",
		GSI1PK:      gsi1pk,
		GSI1SK:      gsi1sk,
	}

	// Add metadata if present
	if metadata := aggregate.GetMetadata(); metadata != nil {
		item.Metadata = metadata
	}

	return item
}

// itemToAggregate converts a DynamoDB item to a node aggregate
func (r *NodeRepository) itemToAggregate(item *nodeItem) (*node.Aggregate, error) {
	// Create value objects
	nodeID := valueobjects.NewNodeIDFromString(item.NodeID)
	userID := valueobjects.NewUserID(item.UserID)
	content := valueobjects.NewContent(item.Content)
	title := valueobjects.NewTitle(item.Title)
	tags := valueobjects.NewTags(item.Tags)
	keywords := valueobjects.NewKeywords(item.Keywords)

	// Parse timestamps
	createdAt, _ := time.Parse(time.RFC3339, item.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, item.UpdatedAt)

	// Reconstruct aggregate
	// Note: This is a simplified reconstruction
	// In a real event-sourced system, you'd replay events
	aggregate := node.NewAggregateWithData(
		nodeID,
		userID,
		content,
		title,
		tags,
		keywords,
		item.CategoryIDs,
		item.IsArchived,
		item.Version,
		createdAt,
		updatedAt,
		item.Metadata,
	)

	return aggregate, nil
}