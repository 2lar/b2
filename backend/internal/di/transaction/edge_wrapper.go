package transaction

import (
	"context"

	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/repository"
)

// transactionalEdgeWrapper wraps edge repository operations with transaction context
type transactionalEdgeWrapper struct {
	base repository.EdgeRepository
	tx   repository.Transaction
}

// NewTransactionalEdgeWrapper creates a new transactional edge repository wrapper
func NewTransactionalEdgeWrapper(base repository.EdgeRepository, tx repository.Transaction) repository.EdgeRepository {
	return &transactionalEdgeWrapper{
		base: base,
		tx:   tx,
	}
}

func (w *transactionalEdgeWrapper) CreateEdge(ctx context.Context, edge *edge.Edge) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateEdge(ctx, edge)
}

func (w *transactionalEdgeWrapper) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
}

func (w *transactionalEdgeWrapper) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*edge.Edge, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindEdges(ctx, query)
}

func (w *transactionalEdgeWrapper) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetEdgesPage(ctx, query, pagination)
}

func (w *transactionalEdgeWrapper) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*edge.Edge, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindEdgesWithOptions(ctx, query, opts...)
}

func (w *transactionalEdgeWrapper) DeleteEdge(ctx context.Context, userID, edgeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdge(ctx, userID, edgeID)
}

func (w *transactionalEdgeWrapper) DeleteEdgesByNode(ctx context.Context, userID, nodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdgesByNode(ctx, userID, nodeID)
}

func (w *transactionalEdgeWrapper) DeleteEdgesBetweenNodes(ctx context.Context, userID, sourceNodeID, targetNodeID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteEdgesBetweenNodes(ctx, userID, sourceNodeID, targetNodeID)
}