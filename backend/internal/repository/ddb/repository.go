// Package ddb implements the repository interface using AWS DynamoDB.
// This is the only layer that should have knowledge of DynamoDB specifics.
package ddb

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	appErrors "brain2-backend/pkg/errors" // ALIAS for our custom errors

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ddbNode represents the structure of a node item in DynamoDB.
type ddbNode struct {
	PK        string   `dynamodbav:"PK"`
	SK        string   `dynamodbav:"SK"`
	NodeID    string   `dynamodbav:"NodeID"`
	UserID    string   `dynamodbav:"UserID"`
	Content   string   `dynamodbav:"Content"`
	Keywords  []string `dynamodbav:"Keywords"`
	IsLatest  bool     `dynamodbav:"IsLatest"`
	Version   int      `dynamodbav:"Version"`
	Timestamp string   `dynamodbav:"Timestamp"`
}

// ddbKeyword represents a keyword index item in DynamoDB.
type ddbKeyword struct {
	PK     string `dynamodbav:"PK"`
	SK     string `dynamodbav:"SK"`
	GSI1PK string `dynamodbav:"GSI1PK"`
	GSI1SK string `dynamodbav:"GSI1SK"`
}

// ddbEdge represents an edge item in DynamoDB.
type ddbEdge struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"`
	TargetID string `dynamodbav:"TargetID"`
}

// Repository defines the contract for database operations.
type Repository interface {
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	DeleteNode(ctx context.Context, userID, nodeID string) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error)
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
	GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

// ddbRepository is the concrete implementation for DynamoDB.
type ddbRepository struct {
	dbClient  *dynamodb.Client
	tableName string
	indexName string
}

// NewRepository creates a new instance of the DynamoDB repository.
func NewRepository(dbClient *dynamodb.Client, tableName, indexName string) Repository {
	return &ddbRepository{
		dbClient:  dbClient,
		tableName: tableName,
		indexName: indexName,
	}
}

// CreateNodeWithEdges saves a node, its keywords, and its connections in a single transaction.
func (r *ddbRepository) CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	transactItems := []types.TransactWriteItem{}

	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID, UserID: node.UserID, Content: node.Content,
		Keywords: node.Keywords, IsLatest: true, Version: 0, Timestamp: node.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: nodeItem}})

	for _, keyword := range node.Keywords {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID, keyword), GSI1SK: fmt.Sprintf("NODE#%s", node.ID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: keywordItem}})
	}

	for _, relatedNodeID := range relatedNodeIDs {
		edge1Item, err := attributevalue.MarshalMap(ddbEdge{PK: pk, SK: fmt.Sprintf("EDGE#RELATES_TO#%s", relatedNodeID), TargetID: relatedNodeID})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal outgoing edge item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: edge1Item}})

		edge2Item, err := attributevalue.MarshalMap(ddbEdge{PK: fmt.Sprintf("USER#%s#NODE#%s", node.UserID, relatedNodeID), SK: fmt.Sprintf("EDGE#RELATES_TO#%s", node.ID), TargetID: node.ID})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal incoming edge item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: edge2Item}})
	}

	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
	if err != nil {
		return appErrors.Wrap(err, "transaction to create node with edges failed")
	}
	return nil
}

// UpdateNodeAndEdges transactionally updates a node and its connections.
func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error {
	if err := r.clearNodeConnections(ctx, node.UserID, node.ID); err != nil {
		return appErrors.Wrap(err, "failed to clear old connections for update")
	}

	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID, node.ID)
	transactItems := []types.TransactWriteItem{}

	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(r.tableName),
		Key:              map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"}},
		UpdateExpression: aws.String("SET Content = :c, Keywords = :k, Timestamp = :t, Version = :v"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: node.Content},
			":k": &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords)},
			":t": &types.AttributeValueMemberS{Value: node.CreatedAt.Format(time.RFC3339)},
			":v": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version)},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to update node metadata")
	}

	for _, keyword := range node.Keywords {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID, keyword), GSI1SK: fmt.Sprintf("NODE#%s", node.ID)})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: keywordItem}})
	}

	for _, relatedNodeID := range relatedNodeIDs {
		edge1Item, err := attributevalue.MarshalMap(ddbEdge{PK: pk, SK: fmt.Sprintf("EDGE#RELATES_TO#%s", relatedNodeID), TargetID: relatedNodeID})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal outgoing edge item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: edge1Item}})
		edge2Item, err := attributevalue.MarshalMap(ddbEdge{PK: fmt.Sprintf("USER#%s#NODE#%s", node.UserID, relatedNodeID), SK: fmt.Sprintf("EDGE#RELATES_TO#%s", node.ID), TargetID: node.ID})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal incoming edge item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.tableName), Item: edge2Item}})
	}

	if len(transactItems) > 0 {
		_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
		if err != nil {
			return appErrors.Wrap(err, "transaction to update node edges failed")
		}
	}
	return nil
}

// FindNodesByKeywords uses the GSI to find nodes with matching keywords.
func (r *ddbRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error) {
	nodeIdMap := make(map[string]bool)
	var nodes []domain.Node
	for _, keyword := range keywords {
		gsiPK := fmt.Sprintf("USER#%s#KEYWORD#%s", userID, keyword)
		result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
			TableName: aws.String(r.tableName), IndexName: aws.String(r.indexName), KeyConditionExpression: aws.String("GSI1PK = :gsiPK"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":gsiPK": &types.AttributeValueMemberS{Value: gsiPK}},
		})
		if err != nil {
			log.Printf("failed to query GSI for keyword %s: %v", keyword, err)
			continue
		}
		for _, item := range result.Items {
			pkValue := item["PK"].(*types.AttributeValueMemberS).Value
			nodeID := strings.Split(pkValue, "#")[3]
			if _, exists := nodeIdMap[nodeID]; !exists {
				nodeIdMap[nodeID] = true
				node, err := r.FindNodeByID(ctx, userID, nodeID)
				if err != nil {
					log.Printf("failed to find node by ID %s found from keyword search: %v", nodeID, err)
					continue
				}
				if node != nil {
					nodes = append(nodes, *node)
				}
			}
		}
	}
	return nodes, nil
}

// FindNodeByID retrieves a single node's metadata.
func (r *ddbRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	sk := "METADATA#v0"
	result, err := r.dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: sk}},
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get item from dynamodb")
	}
	if result.Item == nil {
		return nil, nil // Not found
	}
	var ddbItem ddbNode
	if err := attributevalue.UnmarshalMap(result.Item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal node item")
	}
	createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
	return &domain.Node{
		ID: ddbItem.NodeID, UserID: ddbItem.UserID, Content: ddbItem.Content,
		Keywords: ddbItem.Keywords, CreatedAt: createdAt, Version: ddbItem.Version,
	}, nil
}

// FindEdgesByNode queries for all outgoing edges from a given node.
func (r *ddbRepository) FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error) {
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName: aws.String(r.tableName), KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk}, ":skPrefix": &types.AttributeValueMemberS{Value: "EDGE#RELATES_TO#"},
		},
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query edges")
	}
	var edges []domain.Edge
	for _, item := range result.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			edges = append(edges, domain.Edge{SourceID: nodeID, TargetID: ddbItem.TargetID})
		}
	}
	return edges, nil
}

// DeleteNode transactionally deletes a node, its keywords, and outgoing edges.
func (r *ddbRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.clearNodeConnections(ctx, userID, nodeID)
}

// GetAllGraphData scans the entire table for a user's data to build the graph.
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(r.tableName), FilterExpression: aws.String("begins_with(PK, :pkPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pkPrefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#", userID)}},
	}
	paginator := dynamodb.NewScanPaginator(r.dbClient, scanInput)
	var nodes []domain.Node
	var edges []domain.Edge
	edgeMap := make(map[string]bool)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan graph data page")
		}
		for _, item := range page.Items {
			skValue := item["SK"].(*types.AttributeValueMemberS).Value
			pkValue := item["PK"].(*types.AttributeValueMemberS).Value
			if strings.HasPrefix(skValue, "METADATA#") {
				var ddbItem ddbNode
				if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
					createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
					nodes = append(nodes, domain.Node{ID: ddbItem.NodeID, UserID: ddbItem.UserID, Content: ddbItem.Content, CreatedAt: createdAt, Version: ddbItem.Version})
				}
			} else if strings.HasPrefix(skValue, "EDGE#RELATES_TO#") {
				var ddbItem ddbEdge
				if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
					sourceID := strings.Split(pkValue, "#")[3]
					edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
					reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
					if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
						edgeMap[edgeKey] = true
						edges = append(edges, domain.Edge{SourceID: sourceID, TargetID: ddbItem.TargetID})
					}
				}
			}
		}
	}
	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
}

func (r *ddbRepository) clearNodeConnections(ctx context.Context, userID, nodeID string) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	queryResult, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName: aws.String(r.tableName), KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query items for deletion")
	}
	if len(queryResult.Items) == 0 {
		return nil // Nothing to delete
	}
	var writeRequests []types.WriteRequest
	for _, item := range queryResult.Items {
		writeRequests = append(writeRequests, types.WriteRequest{DeleteRequest: &types.DeleteRequest{Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]}}})
	}
	if len(writeRequests) > 0 {
		_, err = r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{RequestItems: map[string][]types.WriteRequest{r.tableName: writeRequests}})
		if err != nil {
			return appErrors.Wrap(err, "failed to batch delete node items")
		}
	}
	return nil
}

func toAttributeValueList(ss []string) []types.AttributeValue {
	var avs []types.AttributeValue
	for _, s := range ss {
		avs = append(avs, &types.AttributeValueMemberS{Value: s})
	}
	return avs
}
