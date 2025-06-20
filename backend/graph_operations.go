package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type GraphData struct {
	Elements []interface{} `json:"elements"`
}

type GraphNode struct {
	Data NodeData `json:"data"`
}

type NodeData struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type GraphEdge struct {
	Data EdgeData `json:"data"`
}

type EdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func (app *App) getGraphData(ctx context.Context, userID string) (Response, error) {
	graphData := GraphData{
		Elements: []interface{}{},
	}
	nodeMap := make(map[string]string) // nodeID -> content
	edgeMap := make(map[string]bool)   // edge deduplication

	// Use a Scan with a FilterExpression to get all items for a user.
	// This is necessary because Query cannot use 'begins_with' on a Partition Key.
	userPrefix := fmt.Sprintf("USER#%s", userID)
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(app.tableName),
		FilterExpression: aws.String("begins_with(PK, :userPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userPrefix": &types.AttributeValueMemberS{Value: userPrefix},
		},
	}

	// Paginate through all results of the scan
	paginator := dynamodb.NewScanPaginator(app.db, scanInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResponse(500, "Failed to scan graph data page"), nil
		}

		for _, item := range page.Items {
			pkAttr, ok := item["PK"].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			pk := pkAttr.Value

			skAttr, ok := item["SK"].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			sk := skAttr.Value

			// Process node metadata
			if strings.HasPrefix(sk, "METADATA#") {
				var node NodeMetadata
				if err := attributevalue.UnmarshalMap(item, &node); err == nil && node.IsLatest {
					nodeMap[node.NodeID] = node.Content
				}
			}

			// Process edges
			if strings.HasPrefix(sk, "EDGE#RELATES_TO#") {
				if targetAttr, ok := item["TargetID"]; ok {
					if targetID, ok := targetAttr.(*types.AttributeValueMemberS); ok {
						parts := strings.Split(pk, "#")
						if len(parts) >= 4 {
							sourceID := parts[3]
							edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID.Value)
							reverseKey := fmt.Sprintf("%s-%s", targetID.Value, sourceID)

							if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
								edgeMap[edgeKey] = true
								graphData.Elements = append(graphData.Elements, GraphEdge{
									Data: EdgeData{
										ID:     edgeKey,
										Source: sourceID,
										Target: targetID.Value,
									},
								})
							}
						}
					}
				}
			}
		}
	}

	// Create nodes from map
	for nodeID, content := range nodeMap {
		label := content
		if len(label) > 50 {
			label = label[:47] + "..."
		}
		graphData.Elements = append(graphData.Elements, GraphNode{
			Data: NodeData{
				ID:    nodeID,
				Label: label,
			},
		})
	}

	return successResponse(graphData), nil
}

func (app *App) listNodes(ctx context.Context, userID string) (Response, error) {
	nodes := []map[string]interface{}{}

	// Use a Scan with a FilterExpression to get all metadata items for the user.
	userPrefix := fmt.Sprintf("USER#%s", userID)
	skPrefix := "METADATA#"
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(app.tableName),
		FilterExpression: aws.String("begins_with(PK, :userPrefix) AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userPrefix": &types.AttributeValueMemberS{Value: userPrefix},
			":skPrefix":   &types.AttributeValueMemberS{Value: skPrefix},
		},
	}

	paginator := dynamodb.NewScanPaginator(app.db, scanInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return errorResponse(500, "Failed to scan nodes page"), nil
		}
		for _, item := range page.Items {
			var node NodeMetadata
			if err := attributevalue.UnmarshalMap(item, &node); err == nil && node.IsLatest {
				nodes = append(nodes, map[string]interface{}{
					"nodeId":    node.NodeID,
					"content":   node.Content,
					"timestamp": node.Timestamp,
					"version":   node.Version,
				})
			}
		}
	}

	return successResponse(map[string]interface{}{
		"nodes": nodes,
	}), nil
}

func (app *App) getNode(ctx context.Context, userID, nodeID string) (Response, error) {
	// Get the latest version of the node
	// The PK for a specific node is known, so we can use GetItem which is very efficient.
	resp, err := app.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(app.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)},
			"SK": &types.AttributeValueMemberS{Value: "METADATA#v0"}, // Assuming v0 is always the latest for now
		},
	})
	if err != nil {
		return errorResponse(500, "Failed to get node"), nil
	}

	if resp.Item == nil {
		return errorResponse(404, "Node not found"), nil
	}

	var node NodeMetadata
	if err := attributevalue.UnmarshalMap(resp.Item, &node); err != nil {
		return errorResponse(500, "Failed to process node"), nil
	}

	// Get edges for this node using a Query, which is efficient.
	edgeResp, err := app.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(app.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)},
			":skPrefix": &types.AttributeValueMemberS{Value: "EDGE#"},
		},
	})
	if err != nil {
		return errorResponse(500, "Failed to get edges"), nil
	}

	edges := []string{}
	for _, item := range edgeResp.Items {
		if targetAttr, ok := item["TargetID"]; ok {
			if targetID, ok := targetAttr.(*types.AttributeValueMemberS); ok {
				edges = append(edges, targetID.Value)
			}
		}
	}

	return successResponse(map[string]interface{}{
		"nodeId":    node.NodeID,
		"content":   node.Content,
		"timestamp": node.Timestamp,
		"version":   node.Version,
		"edges":     edges,
	}), nil
}

func (app *App) deleteNode(ctx context.Context, userID, nodeID string) (Response, error) {
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)

	// First, query all items with this PK to delete them (this is efficient)
	resp, err := app.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(app.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return errorResponse(500, "Failed to query node items for deletion"), nil
	}

	// Delete all items for this node (metadata, keywords, outgoing edges)
	for _, item := range resp.Items {
		_, err = app.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(app.tableName),
			Key: map[string]types.AttributeValue{
				"PK": item["PK"],
				"SK": item["SK"],
			},
		})
		if err != nil {
			// In a real app, you might want a retry or cleanup mechanism here
			return errorResponse(500, "Failed to delete a node item"), nil
		}
	}

	// Note: Deleting incoming edges from other nodes is complex and not handled here
	// to avoid a full table scan. The graph will have dangling edges pointing to
	// the deleted node, which the frontend should handle gracefully.

	return successResponse(map[string]interface{}{
		"message": "Node and outgoing edges deleted successfully",
	}), nil
}
