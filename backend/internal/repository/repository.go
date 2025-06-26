package repository

import (
	"brain2-backend/internal/domain"
	"context"
)

// Repository is the interface that defines the contract for all data storage operations.
// Any concrete implementation (like DynamoDB or an in-memory mock) must satisfy this interface.
type Repository interface {
	// Node operations
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	DeleteNode(ctx context.Context, userID, nodeID string) error
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)

	// Enhanced query methods using new query types
	FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
	FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)

	// Legacy methods (maintained for backward compatibility)
	FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error)
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
	GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}
