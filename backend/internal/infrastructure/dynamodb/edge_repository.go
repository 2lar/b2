package dynamodb

import (
	"context"
	"fmt"
	"strings"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ddbEdge represents an edge item in DynamoDB.
type ddbEdge struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"`
	TargetID string `dynamodbav:"TargetID"`
	GSI2PK   string `dynamodbav:"GSI2PK"`   // USER#{userId}#EDGE
	GSI2SK   string `dynamodbav:"GSI2SK"`   // NODE#{sourceId}#TARGET#{targetId}
}

// getCanonicalEdge returns the canonical ordering of two node IDs for consistent edge storage.
func getCanonicalEdge(nodeA, nodeB string) (owner, target string) {
	if nodeA < nodeB {
		return nodeA, nodeB
	}
	return nodeB, nodeA
}

// EdgeRepository implements the repository.EdgeRepository interface using DynamoDB.
// This is a dedicated implementation that follows the Single Responsibility Principle.
type EdgeRepository struct {
	dbClient  *dynamodb.Client
	tableName string
	indexName string
}

// NewEdgeRepository creates a new EdgeRepository instance.
func NewEdgeRepository(client *dynamodb.Client, tableName, indexName string) repository.EdgeRepository {
	return &EdgeRepository{
		dbClient:  client,
		tableName: tableName,
		indexName: indexName,
	}
}

// CreateEdges creates bidirectional edges between a source node and multiple related nodes.
// This implements efficient batch edge creation with canonical edge storage pattern.
func (r *EdgeRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	if len(relatedNodeIDs) == 0 {
		return nil // Nothing to create
	}

	var transactItems []types.TransactWriteItem

	// Create canonical edges - only one edge per connection pair
	// This prevents duplicate edges and maintains consistent storage
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(sourceNodeID, relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, targetID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item")
		}

		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(r.tableName),
				Item:      edgeItem,
			},
		})

	}

	// Execute transaction
	_, err := r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if err != nil {
		return appErrors.Wrap(err, "transaction to create edges failed")
	}

	return nil
}

// CreateEdge creates a single edge between two nodes using canonical storage pattern.
func (r *EdgeRepository) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	if edge == nil {
		return appErrors.NewValidation("edge cannot be nil")
	}

	// Use canonical edge storage pattern (lexicographically ordered IDs)
	sourceID := edge.SourceID.String()
	targetID := edge.TargetID.String()
	userID := edge.UserID().String()

	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)

	edgeItem := ddbEdge{
		PK:       ownerPK,
		SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", canonicalTargetID),
		TargetID: canonicalTargetID,
		GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
		GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID),
	}

	item, err := attributevalue.MarshalMap(edgeItem)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal edge item")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to create edge in DynamoDB")
	}

	return nil
}

// FindEdges retrieves edges based on the provided query parameters.
func (r *EdgeRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	if err := query.Validate(); err != nil {
		return nil, appErrors.Wrap(err, "edge query validation failed")
	}

	var edges []*domain.Edge

	// If specific node IDs are requested, find edges for each
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			nodeEdges, err := r.findEdgesByNode(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, appErrors.Wrap(err, fmt.Sprintf("failed to find edges for node %s", nodeID))
			}
			edges = append(edges, nodeEdges...)
		}
		return r.deduplicateEdges(edges), nil
	}

	// If source node is specified, find outgoing edges
	if query.HasSourceFilter() {
		nodeEdges, err := r.findEdgesByNode(ctx, query.UserID, query.SourceID)
		if err != nil {
			return nil, appErrors.Wrap(err, fmt.Sprintf("failed to find edges for source node %s", query.SourceID))
		}
		return r.deduplicateEdges(nodeEdges), nil
	}

	// If target node is specified, find incoming edges (requires scan)
	if query.HasTargetFilter() {
		nodeEdges, err := r.findEdgesByTargetNode(ctx, query.UserID, query.TargetID)
		if err != nil {
			return nil, appErrors.Wrap(err, fmt.Sprintf("failed to find edges for target node %s", query.TargetID))
		}
		return r.deduplicateEdges(nodeEdges), nil
	}

	// Otherwise, get all edges for the user using GSI2
	allEdges, err := r.findAllEdgesForUser(ctx, query.UserID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find all edges for user")
	}

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(allEdges) {
			return []*domain.Edge{}, nil
		}

		end := len(allEdges)
		if query.Limit > 0 && start+query.Limit < len(allEdges) {
			end = start + query.Limit
		}

		allEdges = allEdges[start:end]
	}

	return allEdges, nil
}

// GetEdgesPage retrieves a paginated list of edges based on the query.
func (r *EdgeRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	if err := query.Validate(); err != nil {
		return nil, appErrors.Wrap(err, "edge query validation failed")
	}
	if err := pagination.Validate(); err != nil {
		return nil, appErrors.Wrap(err, "pagination validation failed")
	}

	var queryInput *dynamodb.QueryInput

	if query.HasSourceFilter() {
		// Query edges from a specific source node
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", query.UserID, query.SourceID)},
				":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
			},
		}
	} else {
		// Query all edges for user using GSI2
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			IndexName:              aws.String("EdgeIndex"), // Assuming EdgeIndex exists
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#EDGE", query.UserID)},
			},
		}
	}

	queryInput.Limit = aws.Int32(int32(pagination.GetEffectiveLimit()))

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			queryInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query edges page")
	}

	// Convert DynamoDB items to domain edges
	edges := make([]*domain.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		edge, err := r.ddbItemToEdge(item, query.UserID)
		if err != nil {
			// Skip items that can't be converted - consider adding structured logging here
			continue
		}
		if edge != nil {
			edges = append(edges, edge)
		}
	}

	return &repository.EdgePage{
		Items:      edges,
		HasMore:    result.LastEvaluatedKey != nil,
		NextCursor: repository.EncodeCursor(result.LastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(edges), result.LastEvaluatedKey != nil),
	}, nil
}

// FindEdgesWithOptions implements enhanced edge queries with options (Phase 2 enhancement).
func (r *EdgeRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// For now, delegate to existing method - options can be implemented later
	return r.FindEdges(ctx, query)
}

// Helper methods

// findEdgesByNode finds all edges connected to a specific node.
func (r *EdgeRepository) findEdgesByNode(ctx context.Context, userID, nodeID string) ([]*domain.Edge, error) {
	var edges []*domain.Edge
	edgeMap := make(map[string]bool)

	// Use GSI2 to find all edges for this user, then filter for those involving the specific node
	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			IndexName:              aws.String("EdgeIndex"), // Assuming EdgeIndex exists for GSI2
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges using GSI2")
		}

		// Process edges - keep only those involving the specified node
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source node ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					targetID := ddbItem.TargetID

					// Check if this edge involves the requested node
					if sourceID == nodeID || targetID == nodeID {
						// Prevent duplicate edges using canonical key
						edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID)
						reverseKey := fmt.Sprintf("%s-%s", targetID, sourceID)
						if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
							edgeMap[edgeKey] = true
							// Create rich domain edge using factory method
							userIDVO, _ := domain.NewUserID(userID)
							sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
							targetNodeIDVO, _ := domain.ParseNodeID(targetID)
							edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
							if err == nil {
								edges = append(edges, edge)
							}
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

// findEdgesByTargetNode finds all edges that have the specified node as target.
func (r *EdgeRepository) findEdgesByTargetNode(ctx context.Context, userID, targetNodeID string) ([]*domain.Edge, error) {
	// This requires a scan since we don't have a GSI on target node
	// In a production system, consider adding a GSI for reverse lookups
	var edges []*domain.Edge
	var lastEvaluatedKey map[string]types.AttributeValue

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)

	for {
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(r.tableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix) AND TargetID = :target_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "EDGE#"},
				":target_id": &types.AttributeValueMemberS{Value: targetNodeID},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan for edges with target node")
		}

		for _, item := range result.Items {
			edge, err := r.ddbItemToEdge(item, userID)
			if err != nil {
				// Skip items that can't be converted - consider adding structured logging here
				continue
			}
			if edge != nil {
				edges = append(edges, edge)
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

// findAllEdgesForUser retrieves all edges for a user using GSI2.
func (r *EdgeRepository) findAllEdgesForUser(ctx context.Context, userID string) ([]*domain.Edge, error) {
	var edges []*domain.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	edgeMap := make(map[string]bool)

	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.tableName),
			IndexName:              aws.String("EdgeIndex"), // Assuming EdgeIndex exists for GSI2
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query all edges")
		}

		// Process edges
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					// Prevent duplicate edges using canonical key
					edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
					reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
					if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
						edgeMap[edgeKey] = true
						// Create rich domain edge using factory method
						userIDVO, _ := domain.NewUserID(userID)
						sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
						targetNodeIDVO, _ := domain.ParseNodeID(ddbItem.TargetID)
						edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
						if err == nil {
							edges = append(edges, edge)
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

// ddbItemToEdge converts a DynamoDB item to a domain Edge.
func (r *EdgeRepository) ddbItemToEdge(item map[string]types.AttributeValue, userID string) (*domain.Edge, error) {
	var ddbItem ddbEdge
	if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal edge item")
	}

	// Extract source node ID from PK pattern: USER#<userID>#NODE#<sourceID>
	pkParts := strings.Split(ddbItem.PK, "#")
	if len(pkParts) < 4 {
		return nil, appErrors.NewValidation("invalid PK format for edge")
	}

	sourceID := pkParts[3]
	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create user ID")
	}

	sourceNodeIDVO, err := domain.ParseNodeID(sourceID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to parse source node ID")
	}

	targetNodeIDVO, err := domain.ParseNodeID(ddbItem.TargetID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to parse target node ID")
	}

	edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create edge")
	}

	return edge, nil
}

// deduplicateEdges removes duplicate edges from a slice.
func (r *EdgeRepository) deduplicateEdges(edges []*domain.Edge) []*domain.Edge {
	if len(edges) == 0 {
		return edges
	}

	edgeMap := make(map[string]bool)
	var uniqueEdges []*domain.Edge

	for _, edge := range edges {
		sourceID := edge.SourceID.String()
		targetID := edge.TargetID.String()

		// Create canonical key for deduplication
		ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
		canonicalKey := fmt.Sprintf("%s-%s", ownerID, canonicalTargetID)

		if !edgeMap[canonicalKey] {
			edgeMap[canonicalKey] = true
			uniqueEdges = append(uniqueEdges, edge)
		}
	}

	return uniqueEdges
}