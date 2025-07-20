package repository

import (
	"brain2-backend/internal/domain"
	"context"
)

type Repository interface {
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	DeleteNode(ctx context.Context, userID, nodeID string) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
	FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
}
