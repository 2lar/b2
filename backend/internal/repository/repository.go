package repository

import (
	"context"
	"brain2-backend/internal/domain/shared"
)

// Repository is the main repository interface that composes all segregated interfaces.
// This maintains backward compatibility while providing access to all repository operations.
type Repository interface {
	NodeRepository
	EdgeRepository
	KeywordRepository
	TransactionalRepository
	// Note: CategoryRepository and GraphRepository are accessed through methods,
	// not embedded, to avoid duplicate method declarations
	
	// GetGraphData is needed for consistency checks
	GetGraphData(ctx context.Context, query shared.GraphQuery) (*shared.Graph, error)
}
