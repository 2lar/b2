package dynamodb

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// EdgeRepository implements the EdgeRepository interface using DynamoDB
type EdgeRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *zap.Logger
}

// NewEdgeRepository creates a new EdgeRepository
func NewEdgeRepository(client *dynamodb.Client, tableName string, logger *zap.Logger) ports.EdgeRepository {
	return &EdgeRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}

// edgeItem represents the DynamoDB item structure for an edge
type edgeItem struct {
	PK            string                 `dynamodbav:"PK"`
	SK            string                 `dynamodbav:"SK"`
	EntityType    string                 `dynamodbav:"EntityType"`
	EdgeID        string                 `dynamodbav:"EdgeID"`
	GraphID       string                 `dynamodbav:"GraphID"`
	SourceID      string                 `dynamodbav:"SourceID"`
	TargetID      string                 `dynamodbav:"TargetID"`
	Type          string                 `dynamodbav:"Type"`
	Weight        float64                `dynamodbav:"Weight"`
	Bidirectional bool                   `dynamodbav:"Bidirectional"`
	Metadata      map[string]interface{} `dynamodbav:"Metadata"`
	CreatedAt     string                 `dynamodbav:"CreatedAt"`
	UpdatedAt     string                 `dynamodbav:"UpdatedAt"`

	// GSI attributes for querying by node
	GSI2PK string `dynamodbav:"GSI2PK,omitempty"` // NODE#nodeId for source
	GSI2SK string `dynamodbav:"GSI2SK,omitempty"` // EDGE#edgeId
}

// Save persists an edge to DynamoDB
func (r *EdgeRepository) Save(ctx context.Context, graphID string, edge *aggregates.Edge) error {
	r.logger.Info("Saving edge to DynamoDB",
		zap.String("edgeID", edge.ID),
		zap.String("graphID", graphID),
		zap.String("sourceID", edge.SourceID.String()),
		zap.String("targetID", edge.TargetID.String()),
		zap.String("type", string(edge.Type)),
		zap.Float64("weight", edge.Weight),
	)

	item := edgeItem{
		PK:            fmt.Sprintf("GRAPH#%s", graphID),
		SK:            fmt.Sprintf("EDGE#%s#%s", edge.SourceID, edge.TargetID),
		EntityType:    "EDGE",
		EdgeID:        edge.ID,
		GraphID:       graphID,
		SourceID:      edge.SourceID.String(),
		TargetID:      edge.TargetID.String(),
		Type:          string(edge.Type),
		Weight:        edge.Weight,
		Bidirectional: edge.Bidirectional,
		Metadata:      edge.Metadata,
		CreatedAt:     edge.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     time.Now().Format(time.RFC3339),
		GSI2PK:        fmt.Sprintf("NODE#%s", edge.SourceID.String()),
		GSI2SK:        fmt.Sprintf("EDGE#%s", edge.ID),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}

	// Use conditional write to prevent duplicate edges
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	}

	if _, err := r.client.PutItem(ctx, input); err != nil {
		// Check if it's a conditional check failure (edge already exists)
		if err.Error() == "ConditionalCheckFailedException" {
			r.logger.Warn("Edge already exists between nodes",
				zap.String("sourceID", edge.SourceID.String()),
				zap.String("targetID", edge.TargetID.String()),
			)
			return fmt.Errorf("edge already exists between these nodes")
		}
		r.logger.Error("Failed to save edge to DynamoDB",
			zap.Error(err),
			zap.String("edgeID", edge.ID),
			zap.String("graphID", graphID),
		)
		return fmt.Errorf("failed to save edge: %w", err)
	}

	r.logger.Info("Edge successfully saved to DynamoDB",
		zap.String("edgeID", edge.ID),
		zap.String("graphID", graphID),
		zap.String("source", edge.SourceID.String()),
		zap.String("target", edge.TargetID.String()),
		zap.String("PK", fmt.Sprintf("GRAPH#%s", graphID)),
		zap.String("SK", fmt.Sprintf("EDGE#%s#%s", edge.SourceID, edge.TargetID)),
	)

	return nil
}

// SaveWithUoW saves an edge within a unit of work transaction
func (r *EdgeRepository) SaveWithUoW(ctx context.Context, graphID string, edge *aggregates.Edge, uow interface{}) error {
	// Type assert to DynamoDBUnitOfWork
	dynamoUoW, ok := uow.(*DynamoDBUnitOfWork)
	if !ok {
		return fmt.Errorf("invalid unit of work type")
	}

	// Build the edge item
	item := edgeItem{
		PK:            fmt.Sprintf("GRAPH#%s", graphID),
		SK:            fmt.Sprintf("EDGE#%s#%s", edge.SourceID, edge.TargetID),
		EntityType:    "EDGE",
		EdgeID:        edge.ID,
		GraphID:       graphID,
		SourceID:      edge.SourceID.String(),
		TargetID:      edge.TargetID.String(),
		Type:          string(edge.Type),
		Weight:        edge.Weight,
		Bidirectional: edge.Bidirectional,
		Metadata:      edge.Metadata,
		CreatedAt:     edge.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     time.Now().Format(time.RFC3339),
		GSI2PK:        fmt.Sprintf("NODE#%s", edge.SourceID.String()),
		GSI2SK:        fmt.Sprintf("EDGE#%s", edge.ID),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}

	// Register the save operation with the unit of work
	// Use conditional write to prevent duplicate edges
	transactItem := types.TransactWriteItem{
		Put: &types.Put{
			TableName:           aws.String(r.tableName),
			Item:                av,
			ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
		},
	}

	if err := dynamoUoW.RegisterSave(transactItem); err != nil {
		return fmt.Errorf("failed to register edge save: %w", err)
	}

	r.logger.Debug("Edge registered for transactional save",
		zap.String("edgeID", edge.ID),
		zap.String("graphID", graphID),
		zap.String("sourceID", edge.SourceID.String()),
		zap.String("targetID", edge.TargetID.String()),
	)

	return nil
}

// GetByGraphID retrieves all edges for a graph
func (r *EdgeRepository) GetByGraphID(ctx context.Context, graphID string) ([]*aggregates.Edge, error) {
	r.logger.Debug("Querying edges for graph",
		zap.String("graphID", graphID),
		zap.String("PK", fmt.Sprintf("GRAPH#%s", graphID)),
	)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
		},
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query edges: %w", err)
	}

	edges := make([]*aggregates.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		edge, err := r.parseEdgeItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse edge item", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	r.logger.Debug("Retrieved edges for graph",
		zap.String("graphID", graphID),
		zap.Int("edgeCount", len(edges)),
	)

	return edges, nil
}

// GetByNodeID retrieves all edges connected to a node (as source or target)
func (r *EdgeRepository) GetByNodeID(ctx context.Context, nodeID string) ([]*aggregates.Edge, error) {
	edges := make([]*aggregates.Edge, 0)
	edgeMap := make(map[string]bool) // To avoid duplicates

	// First, try to use the EdgeIndex GSI to find edges where this node is the source
	sourceInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("EdgeIndex"),
		KeyConditionExpression: aws.String("GSI2PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", nodeID)},
		},
	}

	sourceResult, err := r.client.Query(ctx, sourceInput)
	if err != nil {
		r.logger.Warn("EdgeIndex query failed, will use scan fallback",
			zap.String("nodeID", nodeID),
			zap.Error(err),
		)
		// If GSI query fails, fall back to scanning
		return r.getByNodeIDWithScan(ctx, nodeID)
	}

	// Process source edges
	for _, item := range sourceResult.Items {
		edge, err := r.parseEdgeItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse edge item", zap.Error(err))
			continue
		}
		if !edgeMap[edge.ID] {
			edges = append(edges, edge)
			edgeMap[edge.ID] = true
		}
	}

	// For target edges, we need to scan since GSI2PK only stores source node
	// This is inefficient but necessary without a second GSI
	// We'll scan with a filter expression to find edges where target matches
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("EntityType = :entityType AND TargetID = :targetID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":entityType": &types.AttributeValueMemberS{Value: "EDGE"},
			":targetID":   &types.AttributeValueMemberS{Value: nodeID},
		},
	}

	scanResult, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		r.logger.Warn("Failed to scan for target edges",
			zap.String("nodeID", nodeID),
			zap.Error(err),
		)
		// Don't fail completely, return what we have
		return edges, nil
	}

	// Process target edges
	for _, item := range scanResult.Items {
		edge, err := r.parseEdgeItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse target edge item", zap.Error(err))
			continue
		}
		if !edgeMap[edge.ID] {
			edges = append(edges, edge)
			edgeMap[edge.ID] = true
		}
	}

	r.logger.Debug("Found edges for node",
		zap.String("nodeID", nodeID),
		zap.Int("edgeCount", len(edges)),
	)

	return edges, nil
}

// getByNodeIDWithScan is a fallback method when GSI is not available
func (r *EdgeRepository) getByNodeIDWithScan(ctx context.Context, nodeID string) ([]*aggregates.Edge, error) {
	// Scan for all edges where this node is either source or target
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("EntityType = :entityType AND (SourceID = :nodeID OR TargetID = :nodeID)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":entityType": &types.AttributeValueMemberS{Value: "EDGE"},
			":nodeID":     &types.AttributeValueMemberS{Value: nodeID},
		},
	}

	result, err := r.client.Scan(ctx, scanInput)
	if err != nil {
		return nil, fmt.Errorf("failed to scan edges by node: %w", err)
	}

	edges := make([]*aggregates.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		edge, err := r.parseEdgeItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse edge item", zap.Error(err))
			continue
		}
		edges = append(edges, edge)
	}

	r.logger.Debug("Found edges for node using scan",
		zap.String("nodeID", nodeID),
		zap.Int("edgeCount", len(edges)),
	)

	return edges, nil
}

// Delete removes an edge
func (r *EdgeRepository) Delete(ctx context.Context, graphID string, sourceID, targetID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s#%s", sourceID, targetID)},
		},
	}

	if _, err := r.client.DeleteItem(ctx, input); err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	r.logger.Debug("Edge deleted",
		zap.String("graphID", graphID),
		zap.String("source", sourceID),
		zap.String("target", targetID),
	)

	return nil
}

// DeleteByNodeID removes all edges connected to a node (for cascade delete)
func (r *EdgeRepository) DeleteByNodeID(ctx context.Context, graphID string, nodeID string) error {
	// First, get all edges connected to this node
	edges, err := r.GetByNodeID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get edges for deletion: %w", err)
	}

	// Delete each edge
	for _, edge := range edges {
		if err := r.Delete(ctx, graphID, edge.SourceID.String(), edge.TargetID.String()); err != nil {
			r.logger.Warn("Failed to delete edge during cascade",
				zap.String("edgeID", edge.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// parseEdgeItem converts a DynamoDB item to an Edge
func (r *EdgeRepository) parseEdgeItem(item map[string]types.AttributeValue) (*aggregates.Edge, error) {
	var ei edgeItem
	if err := attributevalue.UnmarshalMap(item, &ei); err != nil {
		return nil, fmt.Errorf("failed to unmarshal edge item: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, ei.CreatedAt)

	// Note: This is a simplified reconstruction. In production, you'd use proper value objects
	sourceID, _ := valueobjects.NewNodeIDFromString(ei.SourceID)
	targetID, _ := valueobjects.NewNodeIDFromString(ei.TargetID)

	edge := &aggregates.Edge{
		ID:            ei.EdgeID,
		SourceID:      sourceID,
		TargetID:      targetID,
		Type:          entities.EdgeType(ei.Type),
		Weight:        ei.Weight,
		Bidirectional: ei.Bidirectional,
		Metadata:      ei.Metadata,
		CreatedAt:     createdAt,
	}

	return edge, nil
}

// DeleteByNodeIDs deletes all edges connected to multiple nodes efficiently
func (r *EdgeRepository) DeleteByNodeIDs(ctx context.Context, graphID string, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	// Get all edges for all nodes in batch
	allEdges := make([]*aggregates.Edge, 0)
	for _, nodeID := range nodeIDs {
		edges, err := r.GetByNodeID(ctx, nodeID)
		if err != nil {
			r.logger.Warn("Failed to get edges for node during batch delete",
				zap.String("nodeID", nodeID),
				zap.Error(err),
			)
			continue
		}
		allEdges = append(allEdges, edges...)
	}

	if len(allEdges) == 0 {
		return nil // No edges to delete
	}

	// Group edges by their keys for batch deletion
	edgeKeys := make([]map[string]types.AttributeValue, 0, len(allEdges))
	for _, edge := range allEdges {
		key := map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("EDGE#%s#%s", edge.SourceID.String(), edge.TargetID.String())},
		}
		edgeKeys = append(edgeKeys, key)
	}

	// Use batch delete for efficient edge removal
	if len(edgeKeys) > 0 {
		if err := r.batchDeleteEdges(ctx, edgeKeys); err != nil {
			return fmt.Errorf("failed to batch delete edges: %w", err)
		}
	}

	r.logger.Debug("Batch deleted edges for nodes",
		zap.String("graphID", graphID),
		zap.Strings("nodeIDs", nodeIDs),
		zap.Int("edgesDeleted", len(allEdges)),
	)

	return nil
}

// batchDeleteEdges performs efficient batch deletion of edges using DynamoDB BatchWriteItem
func (r *EdgeRepository) batchDeleteEdges(ctx context.Context, edgeKeys []map[string]types.AttributeValue) error {
	if len(edgeKeys) == 0 {
		return nil
	}

	// Process edges in batches of 25 (DynamoDB limit)
	const batchSize = 25
	for i := 0; i < len(edgeKeys); i += batchSize {
		end := i + batchSize
		if end > len(edgeKeys) {
			end = len(edgeKeys)
		}

		// Create delete requests for this batch
		writeRequests := make([]types.WriteRequest, 0, end-i)
		for j := i; j < end; j++ {
			writeRequests = append(writeRequests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: edgeKeys[j],
				},
			})
		}

		// Execute batch write
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: writeRequests,
			},
		}

		_, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("batch delete failed for edges %d-%d: %w", i, end-1, err)
		}
	}

	return nil
}
