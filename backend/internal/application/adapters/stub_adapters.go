package adapters

import (
	"context"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// StubEdgeRepositoryAdapter provides a non-nil implementation that returns empty results
type StubEdgeRepositoryAdapter struct{}

func (s *StubEdgeRepositoryAdapter) Save(ctx context.Context, edge *domain.Edge) error {
	return nil
}

func (s *StubEdgeRepositoryAdapter) DeleteByNodeID(ctx context.Context, nodeID domain.NodeID) error {
	return nil
}

// StubCategoryRepositoryAdapter provides a non-nil implementation
type StubCategoryRepositoryAdapter struct{}

func (s *StubCategoryRepositoryAdapter) Save(ctx context.Context, category *domain.Category) error {
	return nil
}

func (s *StubCategoryRepositoryAdapter) GetByID(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) (*domain.Category, error) {
	return nil, repository.ErrCategoryNotFound
}

func (s *StubCategoryRepositoryAdapter) FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return nil, repository.ErrCategoryNotFound
}

func (s *StubCategoryRepositoryAdapter) Delete(ctx context.Context, userID domain.UserID, categoryID domain.CategoryID) error {
	return nil
}

func (s *StubCategoryRepositoryAdapter) GetCategoriesForUser(ctx context.Context, userID domain.UserID) ([]*domain.Category, error) {
	return []*domain.Category{}, nil
}

func (s *StubCategoryRepositoryAdapter) AssignNodeToCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error {
	return nil
}

func (s *StubCategoryRepositoryAdapter) RemoveNodeFromCategory(ctx context.Context, userID domain.UserID, nodeID domain.NodeID, categoryID domain.CategoryID) error {
	return nil
}

// StubGraphRepositoryAdapter provides a non-nil implementation
type StubGraphRepositoryAdapter struct{}

func (s *StubGraphRepositoryAdapter) GetGraphForUser(ctx context.Context, userID domain.UserID) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}

func (s *StubGraphRepositoryAdapter) GetSubGraph(ctx context.Context, userID domain.UserID, nodeIDs []domain.NodeID) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}

// StubNodeCategoryRepositoryAdapter provides a non-nil implementation
type StubNodeCategoryRepositoryAdapter struct{}

func (s *StubNodeCategoryRepositoryAdapter) Assign(ctx context.Context, mapping *domain.NodeCategory) error {
	return nil
}

func (s *StubNodeCategoryRepositoryAdapter) Remove(ctx context.Context, userID, nodeID, categoryID string) error {
	return nil
}

func (s *StubNodeCategoryRepositoryAdapter) RemoveAllFromCategory(ctx context.Context, categoryID string) error {
	return nil
}

func (s *StubNodeCategoryRepositoryAdapter) Save(ctx context.Context, mapping *domain.NodeCategory) error {
	return nil
}