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

// QueryRepository implements ports.QueryRepository for read operations
type QueryRepository struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    ports.Logger
}

// NewQueryRepository creates a new query repository
func NewQueryRepository(client *dynamodb.Client, tableName, indexName string, logger ports.Logger) *QueryRepository {
	return &QueryRepository{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// FindNodesByUser retrieves all nodes for a user
func (r *QueryRepository) FindNodesByUser(ctx context.Context, userID string, options ports.QueryOptions) (*ports.NodeQueryResult, error) {
	// Build the query
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(r.indexName),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		},
	}
	
	// Apply limit if specified
	if options.Limit > 0 {
		input.Limit = aws.Int32(int32(options.Limit))
	}
	
	// Execute query
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	
	// Convert items to NodeView
	nodes := make([]ports.NodeView, 0, len(result.Items))
	for _, item := range result.Items {
		// Extract node data
		nodeView := ports.NodeView{}
		
		if v, ok := item["ID"]; ok {
			if s, ok := v.(*types.AttributeValueMemberS); ok {
				nodeView.ID = s.Value
			}
		}
		
		if v, ok := item["UserID"]; ok {
			if s, ok := v.(*types.AttributeValueMemberS); ok {
				nodeView.UserID = s.Value
			}
		}
		
		if v, ok := item["Content"]; ok {
			if s, ok := v.(*types.AttributeValueMemberS); ok {
				nodeView.Content = s.Value
			}
		}
		
		if v, ok := item["Title"]; ok {
			if s, ok := v.(*types.AttributeValueMemberS); ok {
				nodeView.Title = s.Value
			}
		}
		
		if v, ok := item["Tags"]; ok {
			if ss, ok := v.(*types.AttributeValueMemberSS); ok {
				nodeView.Tags = ss.Value
			}
		}
		
		if v, ok := item["CreatedAt"]; ok {
			if n, ok := v.(*types.AttributeValueMemberN); ok {
				var ts int64
				fmt.Sscanf(n.Value, "%d", &ts)
				nodeView.CreatedAt = ts
			}
		}
		
		if v, ok := item["UpdatedAt"]; ok {
			if n, ok := v.(*types.AttributeValueMemberN); ok {
				var ts int64
				fmt.Sscanf(n.Value, "%d", &ts)
				nodeView.UpdatedAt = ts
			}
		}
		
		nodes = append(nodes, nodeView)
	}
	
	return &ports.NodeQueryResult{
		Nodes:      nodes,
		TotalCount: int64(len(nodes)),
		HasMore:    result.LastEvaluatedKey != nil,
	}, nil
}

// SearchNodes performs full-text search on nodes
func (r *QueryRepository) SearchNodes(ctx context.Context, query string, options ports.QueryOptions) (*ports.NodeQueryResult, error) {
	// For DynamoDB, we'd need to scan with filter or use OpenSearch
	// This is a simplified implementation
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("contains(Content, :query) OR contains(Title, :query)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":query": &types.AttributeValueMemberS{Value: query},
		},
	}
	
	if options.Limit > 0 {
		input.Limit = aws.Int32(int32(options.Limit))
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}
	
	// Convert to NodeView
	nodes := make([]ports.NodeView, 0, len(result.Items))
	for _, item := range result.Items {
		var nodeView ports.NodeView
		if err := attributevalue.UnmarshalMap(item, &nodeView); err != nil {
			r.logger.Warn("Failed to unmarshal node",
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		nodes = append(nodes, nodeView)
	}
	
	return &ports.NodeQueryResult{
		Nodes:      nodes,
		TotalCount: int64(result.Count),
		HasMore:    result.LastEvaluatedKey != nil,
	}, nil
}

// GetNodeGraph retrieves the graph structure around a node
func (r *QueryRepository) GetNodeGraph(ctx context.Context, nodeID string, depth int) (*ports.GraphView, error) {
	// Start with the root node
	nodes := make(map[string]ports.NodeView)
	edges := make([]ports.EdgeView, 0)
	
	// BFS to explore graph
	queue := []struct {
		NodeID string
		Depth  int
	}{{NodeID: nodeID, Depth: 0}}
	
	visited := make(map[string]bool)
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		if current.Depth > depth {
			continue
		}
		
		if visited[current.NodeID] {
			continue
		}
		visited[current.NodeID] = true
		
		// Get the node
		nodeItem, err := r.getNode(ctx, current.NodeID)
		if err != nil {
			r.logger.Warn("Failed to get node in graph",
				ports.Field{Key: "node_id", Value: current.NodeID},
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		
		nodes[current.NodeID] = nodeItem
		
		// Get edges for this node
		edgeItems, err := r.getEdges(ctx, current.NodeID)
		if err != nil {
			r.logger.Warn("Failed to get edges",
				ports.Field{Key: "node_id", Value: current.NodeID},
				ports.Field{Key: "error", Value: err.Error()})
			continue
		}
		
		for _, edge := range edgeItems {
			edges = append(edges, edge)
			
			// Add connected nodes to queue if within depth
			if current.Depth < depth {
				if edge.SourceID == current.NodeID && !visited[edge.TargetID] {
					queue = append(queue, struct {
						NodeID string
						Depth  int
					}{NodeID: edge.TargetID, Depth: current.Depth + 1})
				} else if edge.TargetID == current.NodeID && !visited[edge.SourceID] {
					queue = append(queue, struct {
						NodeID string
						Depth  int
					}{NodeID: edge.SourceID, Depth: current.Depth + 1})
				}
			}
		}
	}
	
	// Convert map to slice
	nodeList := make([]ports.NodeView, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, node)
	}
	
	return &ports.GraphView{
		Nodes: nodeList,
		Edges: edges,
	}, nil
}

// GetStatistics retrieves usage statistics
func (r *QueryRepository) GetStatistics(ctx context.Context, userID string) (*ports.Statistics, error) {
	// Count nodes
	nodeCount, err := r.countUserNodes(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Count connections
	connectionCount, err := r.countUserConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Get recent activity
	recentActivity, err := r.getRecentActivity(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	return &ports.Statistics{
		TotalNodes:       nodeCount,
		TotalConnections: connectionCount,
		TotalCategories:  0, // Would need category count
		RecentActivity:   recentActivity,
	}, nil
}

// Helper methods

func (r *QueryRepository) getNode(ctx context.Context, nodeID string) (ports.NodeView, error) {
	// Simplified - would need to scan or maintain a secondary index
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("ID = :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return ports.NodeView{}, err
	}
	
	if len(result.Items) == 0 {
		return ports.NodeView{}, fmt.Errorf("node not found: %s", nodeID)
	}
	
	var nodeView ports.NodeView
	if err := attributevalue.UnmarshalMap(result.Items[0], &nodeView); err != nil {
		return ports.NodeView{}, err
	}
	
	return nodeView, nil
}

func (r *QueryRepository) getEdges(ctx context.Context, nodeID string) ([]ports.EdgeView, error) {
	// Query edges where node is source or target
	edges := []ports.EdgeView{}
	
	// This is simplified - in production would use GSI
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("SourceID = :id OR TargetID = :id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: nodeID},
		},
	}
	
	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}
	
	for _, item := range result.Items {
		var edge ports.EdgeView
		if err := attributevalue.UnmarshalMap(item, &edge); err != nil {
			continue
		}
		edges = append(edges, edge)
	}
	
	return edges, nil
}

func (r *QueryRepository) countUserNodes(ctx context.Context, userID string) (int64, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(r.indexName),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		},
		Select: types.SelectCount,
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return 0, err
	}
	
	return int64(result.Count), nil
}

func (r *QueryRepository) countUserConnections(ctx context.Context, userID string) (int64, error) {
	// Simplified - count edges for user
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", userID)},
			":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
		},
		Select: types.SelectCount,
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return 0, err
	}
	
	return int64(result.Count), nil
}

func (r *QueryRepository) getRecentActivity(ctx context.Context, userID string) ([]ports.Activity, error) {
	// This would typically query an activity log
	// For now, return empty
	return []ports.Activity{
		{
			Type:      "node_created",
			NodeID:    "",
			Timestamp: time.Now().Unix(),
		},
	}, nil
}