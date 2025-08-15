package category

import (
	"context"
	"fmt"
	"brain2-backend/internal/domain"
)

// AIService defines the interface for AI-powered categorization features
type AIService interface {
	SuggestCategories(ctx context.Context, content string, userID string) ([]domain.CategorySuggestion, error)
	CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error)
}

// Service defines the interface for basic category operations
type Service interface {
	GetCategoryDetails(ctx context.Context, userID string, id domain.CategoryID) (*domain.Category, error)
	CreateCategory(ctx context.Context, userID, title, description string) (*domain.Category, error)
	ListCategories(ctx context.Context, userID string) ([]domain.Category, error)
	GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error)
	UpdateCategory(ctx context.Context, userID, categoryID, title, description string) (*domain.Category, error)
	DeleteCategory(ctx context.Context, userID, categoryID string) error
	AssignNodeToCategory(ctx context.Context, userID, categoryID, nodeID string) error
	GetNodesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)
	RemoveNodeFromCategory(ctx context.Context, userID, categoryID, nodeID string) error
	GetCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)
}

// BasicService encapsulates the business logic for categories.
type BasicService struct {
	// The service now depends on the specific, domain-defined interface.
	// This makes the dependency explicit and clear.
	categoryRepo domain.CategoryRepository
	aiService    AIService // Assuming AIService is still needed
}

// NewService creates a new category service.
func NewService(repo domain.CategoryRepository, aiSvc AIService) Service {
	return &BasicService{
		categoryRepo: repo,
		aiService:    aiSvc,
	}
}

// GetCategoryDetails retrieves a category using the repository.
// The service logic is now simpler and more focused on orchestration.
func (s *BasicService) GetCategoryDetails(ctx context.Context, userID string, id domain.CategoryID) (*domain.Category, error) {
	// The call is now simple, direct, and type-safe.
	return s.categoryRepo.FindByID(ctx, userID, id)
}

// CreateCategory creates a new category with the given details
func (s *BasicService) CreateCategory(ctx context.Context, userID, title, description string) (*domain.Category, error) {
	// For now, return an error since the basic service doesn't have a repository implementation
	// that matches the expected methods. This will be handled by the enhanced service.
	return nil, fmt.Errorf("CreateCategory not implemented in BasicService - use EnhancedService")
}

// ListCategories lists all categories for a user
func (s *BasicService) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	return nil, fmt.Errorf("ListCategories not implemented in BasicService - use EnhancedService")
}

// GetCategory retrieves a category by ID
func (s *BasicService) GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	categoryIDTyped := domain.CategoryID(categoryID)
	return s.categoryRepo.FindByID(ctx, userID, categoryIDTyped)
}

// UpdateCategory updates an existing category
func (s *BasicService) UpdateCategory(ctx context.Context, userID, categoryID, title, description string) (*domain.Category, error) {
	return nil, fmt.Errorf("UpdateCategory not implemented in BasicService - use EnhancedService")
}

// DeleteCategory deletes a category
func (s *BasicService) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	categoryIDTyped := domain.CategoryID(categoryID)
	return s.categoryRepo.Delete(ctx, userID, categoryIDTyped)
}

// AssignNodeToCategory assigns a node to a category
func (s *BasicService) AssignNodeToCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	return fmt.Errorf("AssignNodeToCategory not implemented in BasicService - use EnhancedService")
}

// GetNodesInCategory retrieves all nodes in a category
func (s *BasicService) GetNodesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	return nil, fmt.Errorf("GetNodesInCategory not implemented in BasicService - use EnhancedService")
}

// RemoveNodeFromCategory removes a node from a category
func (s *BasicService) RemoveNodeFromCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	return fmt.Errorf("RemoveNodeFromCategory not implemented in BasicService - use EnhancedService")
}

// GetCategoriesForNode retrieves all categories for a node
func (s *BasicService) GetCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	return nil, fmt.Errorf("GetCategoriesForNode not implemented in BasicService - use EnhancedService")
}
