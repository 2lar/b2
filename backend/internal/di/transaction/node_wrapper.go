package transaction

import (
	"context"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// txContextKey is the context key for transactions
	txContextKey contextKey = "tx"
)

// transactionalNodeWrapper wraps node repository operations with transaction context
type transactionalNodeWrapper struct {
	base repository.NodeRepository
	tx   repository.Transaction
}

// NewTransactionalNodeWrapper creates a new transactional node repository wrapper
func NewTransactionalNodeWrapper(base repository.NodeRepository, tx repository.Transaction) repository.NodeRepository {
	return &transactionalNodeWrapper{
		base: base,
		tx:   tx,
	}
}

func (w *transactionalNodeWrapper) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	// Mark context with transaction
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateNodeAndKeywords(ctx, node)
}

func (w *transactionalNodeWrapper) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodeByID(ctx, userID, nodeID)
}

func (w *transactionalNodeWrapper) DeleteNode(ctx context.Context, userID, nodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteNode(ctx, userID, nodeID)
}

func (w *transactionalNodeWrapper) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.BatchDeleteNodes(ctx, userID, nodeIDs)
}

func (w *transactionalNodeWrapper) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.BatchGetNodes(ctx, userID, nodeIDs)
}

func (w *transactionalNodeWrapper) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodes(ctx, query)
}

func (w *transactionalNodeWrapper) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetNodesPage(ctx, query, pagination)
}

func (w *transactionalNodeWrapper) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetNodeNeighborhood(ctx, userID, nodeID, depth)
}

func (w *transactionalNodeWrapper) CountNodes(ctx context.Context, userID string) (int, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CountNodes(ctx, userID)
}

func (w *transactionalNodeWrapper) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesWithOptions(ctx, query, opts...)
}

func (w *transactionalNodeWrapper) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}