// Package dynamodb provides DynamoDB implementations of repository interfaces
package dynamodb

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/core/application/ports"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// EdgeRepository implements ports.EdgeRepository using DynamoDB
type EdgeRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    ports.Logger
}

// NewEdgeRepository creates a new DynamoDB edge repository
func NewEdgeRepository(client *dynamodb.Client, tableName string, logger ports.Logger) *EdgeRepository {
	return &EdgeRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// CreateEdge creates a new edge between nodes
func (r *EdgeRepository) CreateEdge(ctx context.Context, edge *ports.Edge) error {
	if edge.ID == "" {
		edge.ID = fmt.Sprintf("%s-%s", edge.SourceID, edge.TargetID)
	}
	
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = time.Now()
	}
	edge.UpdatedAt = time.Now()
	
	item, err := attributevalue.MarshalMap(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}
	
	// Add composite keys for querying
	item["PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", edge.UserID)}
	item["SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.ID)}
	item["GSI1PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", edge.SourceID)}
	item["GSI1SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.TargetID)}
	
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:               item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		if _, ok := err.(*types.ConditionalCheckFailedException); ok {
			return fmt.Errorf("edge already exists: %w", err)
		}
		return fmt.Errorf("failed to create edge: %w", err)
	}
	
	r.logger.Debug("Edge created",
		ports.Field{Key: "source_id", Value: edge.SourceID},
		ports.Field{Key: "target_id", Value: edge.TargetID})
	
	return nil
}

// DeleteEdge removes an edge
func (r *EdgeRepository) DeleteEdge(ctx context.Context, sourceID, targetID string) error {
	edgeID := fmt.Sprintf("%s-%s", sourceID, targetID)
	
	// We need the UserID to construct the key, so first get the edge
	edge, err := r.GetEdge(ctx, sourceID, targetID)
	if err != nil {
		return err
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", edge.UserID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edgeID)},
	}
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err = r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}
	
	r.logger.Debug("Edge deleted",
		ports.Field{Key: "source_id", Value: sourceID},
		ports.Field{Key: "target_id", Value: targetID})
	
	return nil
}

// FindEdgesByNode finds all edges connected to a node
func (r *EdgeRepository) FindEdgesByNode(ctx context.Context, nodeID string) ([]ports.Edge, error) {
	var edges []ports.Edge
	
	// Since we don't have the GSI1 index yet, we need to use scan with filters
	// This is less efficient but works without the index
	// Scan for edges where node is either source or target
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("SourceID = :nodeId OR TargetID = :nodeId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":nodeId": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	scanResult, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		return nil, fmt.Errorf("failed to scan edges: %w", err)
	}
	
	for _, item := range scanResult.Items {
		var edge ports.Edge
		if err := attributevalue.UnmarshalMap(item, &edge); err != nil {
			r.logger.Warn("Failed to unmarshal edge",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		edges = append(edges, edge)
	}
	
	return edges, nil
}

// GetEdge retrieves a specific edge
func (r *EdgeRepository) GetEdge(ctx context.Context, sourceID, targetID string) (*ports.Edge, error) {
	edgeID := fmt.Sprintf("%s-%s", sourceID, targetID)
	
	// We need to scan since we don't have the UserID
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("ID = :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: edgeID},
		},
	}
	
	result, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge: %w", err)
	}
	
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("edge not found: %s -> %s", sourceID, targetID)
	}
	
	var edge ports.Edge
	if err := attributevalue.UnmarshalMap(result.Items[0], &edge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal edge: %w", err)
	}
	
	return &edge, nil
}

// UpdateEdge updates an edge
func (r *EdgeRepository) UpdateEdge(ctx context.Context, edge *ports.Edge) error {
	edge.UpdatedAt = time.Now()
	
	// Marshal the edge
	item, err := attributevalue.MarshalMap(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}
	
	// Add composite keys
	item["PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", edge.UserID)}
	item["SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.ID)}
	item["GSI1PK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", edge.SourceID)}
	item["GSI1SK"] = &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edge.TargetID)}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}
	
	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update edge: %w", err)
	}
	
	r.logger.Debug("Edge updated",
		ports.Field{Key: "edge_id", Value: edge.ID})
	
	return nil
}

// UpdateStrength updates the strength of an edge
func (r *EdgeRepository) UpdateStrength(ctx context.Context, sourceID, targetID string, strength float64) error {
	// First get the edge to get the UserID
	edge, err := r.GetEdge(ctx, sourceID, targetID)
	if err != nil {
		return err
	}
	
	edgeID := fmt.Sprintf("%s-%s", sourceID, targetID)
	
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", edge.UserID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s", edgeID)},
		},
		UpdateExpression: aws.String("SET Strength = :strength, UpdatedAt = :updated"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":strength": &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", strength)},
			":updated":  &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update edge strength: %w", err)
	}
	
	r.logger.Debug("Edge strength updated",
		ports.Field{Key: "source_id", Value: sourceID},
		ports.Field{Key: "target_id", Value: targetID},
		ports.Field{Key: "strength", Value: strength})
	
	return nil
}