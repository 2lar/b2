package abstractions

import (
	"context"
	"backend2/domain/core/aggregates"
)

// GraphRepositoryAbstraction provides a database-agnostic interface for graph persistence
type GraphRepositoryAbstraction interface {
	// Core CRUD operations
	Save(ctx context.Context, graph *aggregates.Graph) error
	FindByID(ctx context.Context, graphID string) (*aggregates.Graph, error)
	Update(ctx context.Context, graph *aggregates.Graph) error
	Delete(ctx context.Context, graphID string) error
	
	// Query operations
	FindByUserID(ctx context.Context, userID string) ([]*aggregates.Graph, error)
	FindPublicGraphs(ctx context.Context, limit int) ([]*aggregates.Graph, error)
	FindSharedGraphs(ctx context.Context, userID string) ([]*aggregates.Graph, error)
	
	// Graph statistics
	GetGraphStatistics(ctx context.Context, graphID string) (*GraphStatistics, error)
	CountGraphsByUser(ctx context.Context, userID string) (int64, error)
	
	// Version management
	GetGraphVersion(ctx context.Context, graphID string, version int) (*aggregates.Graph, error)
	ListGraphVersions(ctx context.Context, graphID string) ([]GraphVersion, error)
	CreateGraphSnapshot(ctx context.Context, graphID string) error
	
	// Search and discovery
	SearchGraphs(ctx context.Context, query string, filters GraphFilter) ([]*aggregates.Graph, error)
	GetRecommendedGraphs(ctx context.Context, userID string, limit int) ([]*aggregates.Graph, error)
}

// GraphStatistics contains statistical information about a graph
type GraphStatistics struct {
	GraphID      string
	NodeCount    int64
	EdgeCount    int64
	MaxDepth     int
	Components   int
	Density      float64
	LastModified string
	TotalViews   int64
	UniqueUsers  int64
}

// GraphFilter provides filtering options for graph queries
type GraphFilter struct {
	UserID        *string
	IsPublic      *bool
	Tags          []string
	MinNodes      *int
	MaxNodes      *int
	CreatedAfter  *string
	CreatedBefore *string
	UpdatedAfter  *string
	UpdatedBefore *string
}

// GraphVersion represents a specific version of a graph
type GraphVersion struct {
	GraphID     string
	Version     int
	CreatedAt   string
	CreatedBy   string
	Description string
	NodeCount   int
	EdgeCount   int
	Checksum    string
}

// GraphPage represents a paginated result of graphs
type GraphPage struct {
	Graphs     []*aggregates.Graph
	Total      int64
	PageSize   int
	PageNumber int
	HasNext    bool
	HasPrev    bool
}