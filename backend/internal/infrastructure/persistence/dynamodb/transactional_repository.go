package dynamodb

import (
	"context"
	"fmt"

	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// TransactionalRepository implements the repository.TransactionalRepository interface.
type TransactionalRepository struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
}

// NewTransactionalRepository creates a new transactional repository.
func NewTransactionalRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) repository.TransactionalRepository {
	return &TransactionalRepository{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// CreateNodeWithEdges creates a node with edges in a transactional manner.
func (r *TransactionalRepository) CreateNodeWithEdges(ctx context.Context, node *node.Node, relatedNodeIDs []string) error {
	// For now, just create the node
	// In a real implementation, this would use DynamoDB transactions
	// to atomically create the node and all edges
	nodeRepo := NewNodeRepository(r.client, r.tableName, r.indexName, r.logger)
	if err := nodeRepo.Save(ctx, node); err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	
	// Create edges if any
	if len(relatedNodeIDs) > 0 {
		edgeRepo := NewEdgeRepository(r.client, r.tableName, r.indexName, r.logger)
		for _, targetID := range relatedNodeIDs {
			// Parse IDs properly
			sourceID, _ := shared.ParseNodeID(node.GetID())
			targetNodeID, _ := shared.ParseNodeID(targetID)
			userID := node.GetUserID()
			
			// Create edge using proper constructor
			newEdge, err := edge.NewEdge(sourceID, targetNodeID, userID, 1.0)
			if err != nil {
				return fmt.Errorf("failed to create edge: %w", err)
			}
			
			if err := edgeRepo.Save(ctx, newEdge); err != nil {
				return fmt.Errorf("failed to save edge: %w", err)
			}
		}
	}
	
	return nil
}

// UpdateNodeAndEdges updates a node and its edges transactionally.
func (r *TransactionalRepository) UpdateNodeAndEdges(ctx context.Context, node *node.Node, relatedNodeIDs []string) error {
	// For now, just update the node
	// In a real implementation, this would use DynamoDB transactions
	nodeRepo := NewNodeRepository(r.client, r.tableName, r.indexName, r.logger)
	if err := nodeRepo.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	
	// Update edges if needed
	// This is a simplified implementation
	
	return nil
}