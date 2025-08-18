package node

import (
	"context"

	"brain2-backend/internal/domain/shared"
)

// Repository defines the persistence methods for a Node.
type Repository interface {
	FindByID(ctx context.Context, userID string, id shared.NodeID) (*Node, error)
	FindByGraphID(ctx context.Context, userID string, graphID shared.GraphID) ([]*Node, error)
	Save(ctx context.Context, node *Node) error
	Delete(ctx context.Context, userID string, id shared.NodeID) error
}