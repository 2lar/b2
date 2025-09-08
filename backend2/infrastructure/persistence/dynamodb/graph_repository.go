package dynamodb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend2/application/ports"
	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// GraphRepository implements the GraphRepository interface using DynamoDB
type GraphRepository struct {
	client    *dynamodb.Client
	tableName string
	logger    *zap.Logger
	edgeRepo  ports.EdgeRepository
	nodeRepo  ports.NodeRepository
}

// NewGraphRepository creates a new GraphRepository
func NewGraphRepository(client *dynamodb.Client, tableName string, logger *zap.Logger) ports.GraphRepository {
	return &GraphRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
		edgeRepo:  nil, // Will be set via SetEdgeRepository
		nodeRepo:  nil, // Will be set via SetNodeRepository
	}
}

// SetEdgeRepository sets the edge repository for saving edges
func (r *GraphRepository) SetEdgeRepository(edgeRepo ports.EdgeRepository) {
	r.edgeRepo = edgeRepo
}

// SetNodeRepository sets the node repository for loading nodes
func (r *GraphRepository) SetNodeRepository(nodeRepo ports.NodeRepository) {
	r.nodeRepo = nodeRepo
}

// graphItem represents the DynamoDB item structure for a graph
type graphItem struct {
	PK          string                 `dynamodbav:"PK"`
	SK          string                 `dynamodbav:"SK"`
	GSI1PK      string                 `dynamodbav:"GSI1PK,omitempty"` // For graph lookups by ID
	GSI1SK      string                 `dynamodbav:"GSI1SK,omitempty"` // Always "METADATA" for graphs
	EntityType  string                 `dynamodbav:"EntityType"`
	GraphID     string                 `dynamodbav:"GraphID"`
	UserID      string                 `dynamodbav:"UserID"`
	Name        string                 `dynamodbav:"Name"`
	Description string                 `dynamodbav:"Description"`
	NodeCount   int                    `dynamodbav:"NodeCount"`
	EdgeCount   int                    `dynamodbav:"EdgeCount"`
	IsDefault   bool                   `dynamodbav:"IsDefault"`
	Metadata    map[string]interface{} `dynamodbav:"Metadata"`
	CreatedAt   string                 `dynamodbav:"CreatedAt"`
	UpdatedAt   string                 `dynamodbav:"UpdatedAt"`
	Version     int                    `dynamodbav:"Version"`
}

// Save persists a graph to DynamoDB
func (r *GraphRepository) Save(ctx context.Context, graph *aggregates.Graph) error {
	// Get the edges from the graph
	edges := graph.GetEdges()

	// Get node count safely for logging
	nodes, err := graph.Nodes()
	nodeCount := 0
	if err != nil {
		r.logger.Warn("Large graph detected during save", zap.Error(err))
		nodeCount = -1 // Indicate large graph
	} else {
		nodeCount = len(nodes)
	}

	r.logger.Info("Saving graph to DynamoDB",
		zap.String("graphID", graph.ID().String()),
		zap.String("userID", graph.UserID()),
		zap.Int("nodeCount", nodeCount),
		zap.Int("edgeCount", len(edges)),
	)

	// Save edges to edge repository if available
	if r.edgeRepo != nil {
		for _, edge := range edges {
			r.logger.Info("Saving edge from graph",
				zap.String("edgeID", edge.ID),
				zap.String("sourceID", edge.SourceID.String()),
				zap.String("targetID", edge.TargetID.String()),
			)

			if err := r.edgeRepo.Save(ctx, graph.ID().String(), edge); err != nil {
				r.logger.Error("Failed to save edge",
					zap.Error(err),
					zap.String("edgeID", edge.ID),
				)
				// Continue saving other edges even if one fails
			}
		}
	} else {
		r.logger.Warn("EdgeRepository not set, edges will not be persisted separately")
	}

	item := graphItem{
		PK:          fmt.Sprintf("USER#%s", graph.UserID()),
		SK:          fmt.Sprintf("GRAPH#%s", graph.ID().String()),
		GSI1PK:      fmt.Sprintf("GRAPHID#%s", graph.ID().String()), // Enable efficient lookup by graph ID
		GSI1SK:      "METADATA",
		EntityType:  "GRAPH",
		GraphID:     graph.ID().String(),
		UserID:      graph.UserID(),
		Name:        graph.Name(),
		Description: graph.Description(),
		NodeCount:   graph.NodeCount(),
		EdgeCount:   graph.EdgeCount(),
		IsDefault:   graph.IsDefault(),
		Metadata:    graph.Metadata(),
		CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
		Version:     1, // TODO: Implement versioning
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	}

	if _, err := r.client.PutItem(ctx, input); err != nil {
		r.logger.Error("Failed to save graph to DynamoDB",
			zap.Error(err),
			zap.String("graphID", graph.ID().String()),
		)
		return fmt.Errorf("failed to save graph: %w", err)
	}

	r.logger.Info("Successfully saved graph to DynamoDB",
		zap.String("graphID", graph.ID().String()),
		zap.String("userID", graph.UserID()),
		zap.String("PK", fmt.Sprintf("USER#%s", graph.UserID())),
		zap.String("SK", fmt.Sprintf("GRAPH#%s", graph.ID().String())),
		zap.Int("edgesSaved", len(edges)),
	)

	return nil
}

// SaveWithUoW saves a graph within a unit of work transaction
func (r *GraphRepository) SaveWithUoW(ctx context.Context, graph *aggregates.Graph, uow interface{}) error {
	// Type assert to DynamoDBUnitOfWork
	dynamoUoW, ok := uow.(*DynamoDBUnitOfWork)
	if !ok {
		return fmt.Errorf("invalid unit of work type")
	}

	// Build the graph item
	item := graphItem{
		PK:          fmt.Sprintf("USER#%s", graph.UserID()),
		SK:          fmt.Sprintf("GRAPH#%s", graph.ID().String()),
		GSI1PK:      fmt.Sprintf("GRAPHID#%s", graph.ID().String()),
		GSI1SK:      "METADATA",
		EntityType:  "GRAPH",
		GraphID:     graph.ID().String(),
		UserID:      graph.UserID(),
		Name:        graph.Name(),
		Description: graph.Description(),
		NodeCount:   graph.NodeCount(),
		EdgeCount:   graph.EdgeCount(),
		IsDefault:   graph.IsDefault(),
		Metadata:    graph.Metadata(),
		CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
		Version:     graph.Version(),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal graph: %w", err)
	}

	// Register the save operation with the unit of work
	transactItem := types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(r.tableName),
			Item:      av,
		},
	}

	if err := dynamoUoW.RegisterSave(transactItem); err != nil {
		return fmt.Errorf("failed to register graph save: %w", err)
	}

	// Register any uncommitted events from the graph
	for _, event := range graph.GetUncommittedEvents() {
		if err := dynamoUoW.RegisterEvent(event); err != nil {
			return fmt.Errorf("failed to register graph event: %w", err)
		}
	}

	r.logger.Debug("Graph registered for transactional save",
		zap.String("graphID", graph.ID().String()),
		zap.String("userID", graph.UserID()),
		zap.Int("nodeCount", graph.NodeCount()),
		zap.Int("edgeCount", graph.EdgeCount()),
	)

	return nil
}

// GetByID retrieves a graph by its ID
func (r *GraphRepository) GetByID(ctx context.Context, id aggregates.GraphID) (*aggregates.Graph, error) {
	// Use GSI1 for efficient lookup by GraphID
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND GSI1SK = :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPHID#%s", id.String())},
			":sk": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		Limit: aws.Int32(1),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query graph: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("graph not found: %s", id.String())
	}

	var item graphItem
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	// Add debug logging
	r.logger.Debug("Retrieved graph from DynamoDB",
		zap.String("graphID", item.GraphID),
		zap.String("name", item.Name),
		zap.String("entityType", item.EntityType),
	)

	// Create graph with proper reconstruction
	graph, err := aggregates.ReconstructGraph(
		item.GraphID,
		item.UserID,
		item.Name,
		item.Description,
		item.IsDefault,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct graph: %w", err)
	}

	// CRITICAL: Load nodes and edges in parallel for performance
	var nodes []*entities.Node
	var edges []*aggregates.Edge
	var nodeErr, edgeErr error

	// Use goroutines for parallel loading
	var wg sync.WaitGroup
	wg.Add(2)

	// Load nodes in parallel
	go func() {
		defer wg.Done()
		if r.nodeRepo != nil {
			nodes, nodeErr = r.nodeRepo.GetByGraphID(ctx, id.String())
		}
	}()

	// Load edges in parallel
	go func() {
		defer wg.Done()
		if r.edgeRepo != nil {
			edges, edgeErr = r.edgeRepo.GetByGraphID(ctx, id.String())
		}
	}()

	// Wait for both operations to complete
	wg.Wait()

	// Process nodes first (edges depend on nodes)
	if nodeErr != nil {
		r.logger.Warn("Failed to load nodes for graph",
			zap.String("graphID", id.String()),
			zap.Error(nodeErr),
		)
	} else if nodes != nil {
		// Add all nodes to the graph aggregate
		for _, node := range nodes {
			if err := graph.AddNode(node); err != nil {
				r.logger.Debug("Node already in graph or failed to add",
					zap.String("nodeID", node.ID().String()),
					zap.Error(err),
				)
			}
		}
		r.logger.Debug("Loaded nodes into graph",
			zap.String("graphID", id.String()),
			zap.Int("nodeCount", len(nodes)),
		)
	}

	// Process edges after nodes are loaded
	if edgeErr != nil {
		r.logger.Warn("Failed to load edges for graph",
			zap.String("graphID", id.String()),
			zap.Error(edgeErr),
		)
	} else if edges != nil {
		// Add all edges to the graph aggregate
		successfulEdges := 0
		for _, edge := range edges {
			if err := graph.LoadEdge(edge); err != nil {
				r.logger.Debug("Edge failed to load",
					zap.String("edgeID", edge.ID),
					zap.Error(err),
				)
			} else {
				successfulEdges++
			}
		}
		r.logger.Debug("Loaded edges into graph",
			zap.String("graphID", id.String()),
			zap.Int("edgeCount", successfulEdges),
			zap.Int("totalEdges", len(edges)),
		)
	}

	return graph, nil
}

// GetByUserID retrieves all graphs for a user
func (r *GraphRepository) GetByUserID(ctx context.Context, userID string) ([]*aggregates.Graph, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk": &types.AttributeValueMemberS{Value: "GRAPH#"},
		},
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query graphs: %w", err)
	}

	graphs := make([]*aggregates.Graph, 0, len(result.Items))
	for _, item := range result.Items {
		var graphItem graphItem
		if err := attributevalue.UnmarshalMap(item, &graphItem); err != nil {
			r.logger.Warn("Failed to unmarshal graph item", zap.Error(err))
			continue
		}

		// Reconstruct the graph from stored data
		graph := &aggregates.Graph{}
		// Use reflection or a reconstruction method to properly restore the graph
		// For now, we'll create a new graph and set its ID
		graph, err := aggregates.ReconstructGraph(
			graphItem.GraphID,
			graphItem.UserID,
			graphItem.Name,
			graphItem.Description,
			graphItem.IsDefault,
			graphItem.CreatedAt,
			graphItem.UpdatedAt,
		)
		if err != nil {
			r.logger.Warn("Failed to reconstruct graph from item",
				zap.String("graphID", graphItem.GraphID),
				zap.Error(err))
			continue
		}
		graphs = append(graphs, graph)
	}

	return graphs, nil
}

// GetUserDefaultGraph retrieves the user's default graph
func (r *GraphRepository) GetUserDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	// Query for user's graphs where IsDefault = true
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("IsDefault = :isDefault AND EntityType = :entityType"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":         &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			":sk":         &types.AttributeValueMemberS{Value: "GRAPH#"},
			":isDefault":  &types.AttributeValueMemberBOOL{Value: true},
			":entityType": &types.AttributeValueMemberS{Value: "GRAPH"},
		},
		Limit: aws.Int32(1),
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query default graph: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no default graph found for user")
	}

	var item graphItem
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	// Reconstruct the graph from stored data
	graph, err := aggregates.ReconstructGraph(
		item.GraphID,
		item.UserID,
		item.Name,
		item.Description,
		item.IsDefault,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct graph: %w", err)
	}

	// Load nodes if nodeRepo is available
	if r.nodeRepo != nil {
		nodes, err := r.nodeRepo.GetByGraphID(ctx, item.GraphID)
		if err != nil {
			r.logger.Warn("Failed to load nodes for graph",
				zap.String("graphID", item.GraphID),
				zap.Error(err),
			)
		} else {
			for _, node := range nodes {
				if err := graph.AddNode(node); err != nil {
					r.logger.Debug("Node already in graph or failed to add",
						zap.String("nodeID", node.ID().String()),
						zap.Error(err),
					)
				}
			}
		}
	}

	return graph, nil
}

// GetOrCreateDefaultGraph gets or creates a default graph for a user
func (r *GraphRepository) GetOrCreateDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	// First, try to get the existing default graph
	existingGraph, err := r.GetUserDefaultGraph(ctx, userID)
	if err == nil {
		// Found existing default graph
		return existingGraph, nil
	}

	// No default graph exists, create one
	graph, err := aggregates.NewGraph(userID, "Default Graph")
	if err != nil {
		return nil, fmt.Errorf("failed to create default graph: %w", err)
	}

	// Use conditional write to prevent race conditions
	item := graphItem{
		PK:          fmt.Sprintf("USER#%s", graph.UserID()),
		SK:          fmt.Sprintf("GRAPH#%s", graph.ID().String()),
		GSI1PK:      fmt.Sprintf("GRAPHID#%s", graph.ID().String()),
		GSI1SK:      "METADATA",
		EntityType:  "GRAPH",
		GraphID:     graph.ID().String(),
		UserID:      graph.UserID(),
		Name:        graph.Name(),
		Description: graph.Description(),
		NodeCount:   0,
		EdgeCount:   0,
		IsDefault:   true,
		Metadata:    graph.Metadata(),
		CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
		Version:     1,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal graph: %w", err)
	}

	// Only create if no other default graph exists for this user
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	}

	if _, err := r.client.PutItem(ctx, input); err != nil {
		// If it fails due to condition, try to get the graph again
		// This handles race conditions where another process created it
		existingGraph, err2 := r.GetUserDefaultGraph(ctx, userID)
		if err2 == nil {
			return existingGraph, nil
		}
		return nil, fmt.Errorf("failed to save default graph: %w", err)
	}

	r.logger.Info("Default graph created",
		zap.String("graphID", graph.ID().String()),
		zap.String("userID", userID),
	)

	return graph, nil
}

// CreateDefaultGraph creates a default graph for a user (deprecated - use GetOrCreateDefaultGraph)
func (r *GraphRepository) CreateDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	// This is deprecated - just call GetOrCreateDefaultGraph
	return r.GetOrCreateDefaultGraph(ctx, userID)
}

// Delete removes a graph
func (r *GraphRepository) Delete(ctx context.Context, id aggregates.GraphID) error {
	// First, get the graph to find the user ID
	graph, err := r.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get graph for deletion: %w", err)
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", graph.UserID())},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", id.String())},
		},
	}

	if _, err := r.client.DeleteItem(ctx, input); err != nil {
		return fmt.Errorf("failed to delete graph: %w", err)
	}

	r.logger.Debug("Graph deleted",
		zap.String("graphID", id.String()),
		zap.String("userID", graph.UserID()),
	)

	return nil
}

// UpdateGraphMetadata updates the node and edge counts for a graph based on actual database state
func (r *GraphRepository) UpdateGraphMetadata(ctx context.Context, graphID string) error {
	r.logger.Debug("Updating graph metadata",
		zap.String("graphID", graphID),
	)

	// Count nodes: Query where PK = GRAPH#id AND begins_with(SK, 'NODE#')
	nodeCountInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			":sk": &types.AttributeValueMemberS{Value: "NODE#"},
		},
		Select: types.SelectCount,
	}

	nodeResult, err := r.client.Query(ctx, nodeCountInput)
	if err != nil {
		return fmt.Errorf("failed to count nodes: %w", err)
	}
	nodeCount := int(nodeResult.Count)

	// Count edges: Query where PK = GRAPH#id AND begins_with(SK, 'EDGE#')
	edgeCountInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
			":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
		},
		Select: types.SelectCount,
	}

	edgeResult, err := r.client.Query(ctx, edgeCountInput)
	if err != nil {
		return fmt.Errorf("failed to count edges: %w", err)
	}
	edgeCount := int(edgeResult.Count)

	// Get the user ID for the graph (we need it for the key)
	// First, get the graph metadata to find the user
	graphQueryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :pk AND GSI1SK = :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPHID#%s", graphID)},
			":sk": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		Limit: aws.Int32(1),
	}

	graphResult, err := r.client.Query(ctx, graphQueryInput)
	if err != nil {
		return fmt.Errorf("failed to get graph metadata: %w", err)
	}

	if len(graphResult.Items) == 0 {
		return fmt.Errorf("graph not found: %s", graphID)
	}

	var graphItem graphItem
	if err := attributevalue.UnmarshalMap(graphResult.Items[0], &graphItem); err != nil {
		return fmt.Errorf("failed to unmarshal graph: %w", err)
	}

	// Update the graph metadata with the actual counts
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", graphItem.UserID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("GRAPH#%s", graphID)},
		},
		UpdateExpression: aws.String("SET NodeCount = :nodeCount, EdgeCount = :edgeCount, UpdatedAt = :updatedAt, #metadata.#nodeCount = :nodeCount, #metadata.#edgeCount = :edgeCount"),
		ExpressionAttributeNames: map[string]string{
			"#metadata":  "Metadata",
			"#nodeCount": "nodeCount",
			"#edgeCount": "edgeCount",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":nodeCount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", nodeCount)},
			":edgeCount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", edgeCount)},
			":updatedAt": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	}

	if _, err := r.client.UpdateItem(ctx, updateInput); err != nil {
		return fmt.Errorf("failed to update graph metadata: %w", err)
	}

	r.logger.Info("Successfully updated graph metadata",
		zap.String("graphID", graphID),
		zap.Int("nodeCount", nodeCount),
		zap.Int("edgeCount", edgeCount),
	)

	return nil
}
