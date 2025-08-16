package dynamodb

import (
	"context"

	mainDdb "brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NodeRepository delegates to the main DynamoDB implementation.
// This exists to satisfy the unit of work pattern in the internal infrastructure layer.
type NodeRepository struct {
	base repository.NodeRepository
}

// NewNodeRepository creates a new NodeRepository instance.
func NewNodeRepository(client *dynamodb.Client, tableName, indexName string) repository.NodeRepository {
	// Create the actual DynamoDB repository from the main package
	base := mainDdb.NewNodeRepository(client, tableName, indexName)
	return &NodeRepository{
		base: base,
	}
}

// FindNodeByID delegates to the base implementation
func (r *NodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	return r.base.FindNodeByID(ctx, userID, nodeID)
}

func (r *NodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	return r.base.FindNodes(ctx, query)
}

func (r *NodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.base.GetNodesPage(ctx, query, pagination)
}

func (r *NodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	return r.base.GetNodeNeighborhood(ctx, userID, nodeID, depth)
}

func (r *NodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	return r.base.CountNodes(ctx, userID)
}

func (r *NodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return r.base.FindNodesWithOptions(ctx, query, opts...)
}

func (r *NodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	return r.base.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}

func (r *NodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	return r.base.CreateNodeAndKeywords(ctx, node)
}

func (r *NodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.base.DeleteNode(ctx, userID, nodeID)
}