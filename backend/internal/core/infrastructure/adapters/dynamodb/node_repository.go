// Package dynamodb provides DynamoDB implementations of core domain repositories
package dynamodb

import (
	"context"
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

// FindByUserAndID retrieves a node by UserID and NodeID using direct GetItem
func (r *NodeRepository) FindByUserAndID(ctx context.Context, userID, nodeID string) (*node.Aggregate, error) {
	// Direct GetItem using composite key
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("NODE#%s", nodeID)
	
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	
	if err != nil {
		r.logger.Error("Failed to get node by UserID and NodeID", err,
			ports.Field{Key: "user_id", Value: userID},
			ports.Field{Key: "node_id", Value: nodeID})
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	
	if result.Item == nil {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	
	// Unmarshal the item
	var item nodeItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}
	
	// Convert to aggregate
	return r.itemToAggregate(&item)
}

// FindByID retrieves a node by its ID
func (r *NodeRepository) FindByID(ctx context.Context, id string) (*node.Aggregate, error) {
	// Check if we have UserID in context (if we add this feature later)
	// For now, we'll need to scan the GSI which is not ideal
	// The proper solution is to use FindByUserAndID when UserID is available
	
	// Query GSI1 with a filter on NodeID
	// This is more efficient than scanning the main table
	expr, err := expression.NewBuilder().
		WithFilter(expression.Name("NodeID").Equal(expression.Value(id))).
		Build()
	
	if err != nil {
		return nil, fmt.Errorf("failed to build query expression: %w", err)
	}

	// Scan the GSI1 index
	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(r.tableName),
		IndexName:                 aws.String(r.indexName), // Use GSI1 index
		FilterExpression:          expr.Filter(),
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
	// Optimize for UserOwnedNodeSpecification which is the most common case
	// Check if spec has a GetUserID method to extract user context
	var userID string
	if userSpec, ok := spec.(interface{ GetUserID() string }); ok {
		userID = userSpec.GetUserID()
	}
	
	if userID != "" {
		// Use GSI1 to query all nodes for a specific user
		// This avoids a full table scan
		expr, err := expression.NewBuilder().
			WithKeyCondition(
				expression.Key("GSI1PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID))),
			).
			Build()
		
		if err != nil {
			return nil, fmt.Errorf("failed to build query expression: %w", err)
		}
		
		queryInput := &dynamodb.QueryInput{
			TableName:                 aws.String(r.tableName),
			IndexName:                 aws.String(r.indexName), // GSI1
			KeyConditionExpression:    expr.KeyCondition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			Limit:                     aws.Int32(100), // Reasonable limit for related nodes
		}
		
		result, err := r.client.Query(ctx, queryInput)
		if err != nil {
			r.logger.Error("Failed to query nodes for user", err,
				ports.Field{Key: "user_id", Value: userID})
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
			
			// Apply specification filter for additional criteria
			satisfied, err := spec.IsSatisfiedBy(ctx, aggregate)
			if err != nil {
				r.logger.Warn("Failed to check specification",
					ports.Field{Key: "error", Value: err.Error()})
				continue
			}
			if satisfied {
				aggregates = append(aggregates, aggregate)
			}
		}
		
		r.logger.Debug("Queried nodes using GSI for user",
			ports.Field{Key: "user_id", Value: userID},
			ports.Field{Key: "count", Value: len(aggregates)})
		
		return aggregates, nil
	}
	
	// Fallback to scan for other specification types
	// This should be rare and we should log a warning
	r.logger.Warn("Using scan for non-user specification - consider optimizing",
		ports.Field{Key: "spec_type", Value: fmt.Sprintf("%T", spec)})
	
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
		FilterExpression: aws.String("EntityType = :entity_type"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":entity_type": &types.AttributeValueMemberS{Value: "NODE"},
		},
		Limit: aws.Int32(50), // Reduce limit to minimize scan impact
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
		satisfied, err := spec.IsSatisfiedBy(ctx, aggregate)
		if err != nil {
			r.logger.Warn("Failed to check specification",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		if satisfied {
			aggregates = append(aggregates, aggregate)
		}
	}

	return aggregates, nil
}

// Delete removes a node from DynamoDB
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	// We need both PK and SK to delete - this requires UserID
	// This method should be called with UserID context
	return fmt.Errorf("Delete requires UserID - use DeleteByUserAndID instead")
}

// DeleteByUserAndID removes a node from DynamoDB using both UserID and NodeID
func (r *NodeRepository) DeleteByUserAndID(ctx context.Context, userID, nodeID string) error {
	// Construct the composite key
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("NODE#%s", nodeID)
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	})

	if err != nil {
		r.logger.Error("Failed to delete node", err,
			ports.Field{Key: "user_id", Value: userID},
			ports.Field{Key: "node_id", Value: nodeID})
		return fmt.Errorf("failed to delete node: %w", err)
	}

	r.logger.Info("Node deleted successfully",
		ports.Field{Key: "user_id", Value: userID},
		ports.Field{Key: "node_id", Value: nodeID})

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
	gsi1pk := fmt.Sprintf("USER#%s", userID)
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
	nodeID := valueobjects.NewNodeID(item.NodeID)
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