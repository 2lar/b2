package abstractions

import (
	"context"
	"backend2/domain/core/entities"
	"backend2/domain/core/valueobjects"
)

// NodeRepositoryAbstraction provides a database-agnostic interface for node persistence
type NodeRepositoryAbstraction interface {
	// Core CRUD operations
	Save(ctx context.Context, node *entities.Node) error
	FindByID(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error)
	Update(ctx context.Context, node *entities.Node) error
	Delete(ctx context.Context, nodeID valueobjects.NodeID) error
	
	// Query operations
	FindByGraphID(ctx context.Context, graphID string) ([]*entities.Node, error)
	FindByUserID(ctx context.Context, userID string) ([]*entities.Node, error)
	FindByTags(ctx context.Context, tags []string) ([]*entities.Node, error)
	SearchByContent(ctx context.Context, query string, limit int) ([]*entities.Node, error)
	
	// Batch operations
	SaveBatch(ctx context.Context, nodes []*entities.Node) error
	DeleteBatch(ctx context.Context, nodeIDs []valueobjects.NodeID) error
	
	// Connection operations (if not handled by EdgeRepository)
	GetConnectedNodes(ctx context.Context, nodeID valueobjects.NodeID) ([]*entities.Node, error)
	CountNodesByGraph(ctx context.Context, graphID string) (int64, error)
	
	// Advanced queries
	FindSimilarNodes(ctx context.Context, nodeID valueobjects.NodeID, threshold float64) ([]*entities.Node, error)
	FindOrphanedNodes(ctx context.Context, graphID string) ([]*entities.Node, error)
}

// NodeFilter provides filtering options for node queries
type NodeFilter struct {
	GraphID   *string
	UserID    *string
	Status    *entities.NodeStatus
	Tags      []string
	CreatedAfter  *string
	CreatedBefore *string
	UpdatedAfter  *string
	UpdatedBefore *string
}

// NodeSortOptions provides sorting options for node queries
type NodeSortOptions struct {
	Field SortField
	Order SortOrder
}

// SortField defines available fields for sorting
type SortField string

const (
	SortByCreatedAt SortField = "created_at"
	SortByUpdatedAt SortField = "updated_at"
	SortByTitle     SortField = "title"
	SortByPosition  SortField = "position"
)

// NodePage represents a paginated result of nodes
type NodePage struct {
	Nodes      []*entities.Node
	Total      int64
	PageSize   int
	PageNumber int
	HasNext    bool
	HasPrev    bool
}