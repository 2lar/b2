// Package category provides business logic for category management and memory organization.
package category

import (
	"context"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/google/uuid"
)

// Service defines the interface for category-related business operations.
type Service interface {
	// CreateCategory creates a new category with validation
	CreateCategory(ctx context.Context, userID, title, description string) (*domain.Category, error)

	// UpdateCategory modifies an existing category
	UpdateCategory(ctx context.Context, userID, categoryID, title, description string) (*domain.Category, error)

	// DeleteCategory removes a category and all its memory associations
	DeleteCategory(ctx context.Context, userID, categoryID string) error

	// GetCategory retrieves a single category by ID
	GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error)

	// ListCategories retrieves all categories for a user
	ListCategories(ctx context.Context, userID string) ([]domain.Category, error)

	// AssignNodeToCategory associates a node with a category
	AssignNodeToCategory(ctx context.Context, userID, categoryID, nodeID string) error

	// RemoveNodeFromCategory removes a node from a category
	RemoveNodeFromCategory(ctx context.Context, userID, categoryID, nodeID string) error

	// GetNodesInCategory retrieves all nodes in a specific category
	GetNodesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)

	// GetCategoriesForNode retrieves all categories that contain a specific node
	GetCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error)
}

// service implements the Service interface with concrete business logic using segregated repositories.
type service struct {
	// Segregated repository dependencies - only what this service needs  
	categoryRepo repository.CategoryRepository
	nodeRepo     repository.NodeRepository
}

// NewService creates a new category service with segregated repositories.
func NewService(categoryRepo repository.CategoryRepository, nodeRepo repository.NodeRepository) Service {
	return &service{
		categoryRepo: categoryRepo,
		nodeRepo:     nodeRepo,
	}
}

// NewServiceFromRepository creates a category service from a monolithic repository (for backward compatibility).
func NewServiceFromRepository(repo repository.Repository) Service {
	return &service{
		categoryRepo: repo,
		nodeRepo:     repo,
	}
}

// CreateCategory creates a new category with validation.
func (s *service) CreateCategory(ctx context.Context, userID, title, description string) (*domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if title == "" {
		return nil, appErrors.NewValidation("title cannot be empty")
	}
	if len(title) > 100 {
		return nil, appErrors.NewValidation("title cannot exceed 100 characters")
	}
	if len(description) > 500 {
		return nil, appErrors.NewValidation("description cannot exceed 500 characters")
	}

	category := domain.Category{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       title,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if err := s.categoryRepo.CreateCategory(ctx, category); err != nil {
		return nil, appErrors.Wrap(err, "failed to create category in repository")
	}

	return &category, nil
}

// UpdateCategory modifies an existing category with validation.
func (s *service) UpdateCategory(ctx context.Context, userID, categoryID, title, description string) (*domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}
	if title == "" {
		return nil, appErrors.NewValidation("title cannot be empty")
	}
	if len(title) > 100 {
		return nil, appErrors.NewValidation("title cannot exceed 100 characters")
	}
	if len(description) > 500 {
		return nil, appErrors.NewValidation("description cannot exceed 500 characters")
	}

	// Check if category exists and belongs to user
	existingCategory, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to check for existing category")
	}
	if existingCategory == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	updatedCategory := domain.Category{
		ID:          categoryID,
		UserID:      userID,
		Title:       title,
		Description: description,
		CreatedAt:   time.Now(), // Update timestamp
	}

	if err := s.categoryRepo.UpdateCategory(ctx, updatedCategory); err != nil {
		return nil, appErrors.Wrap(err, "failed to update category in repository")
	}

	return &updatedCategory, nil
}

// DeleteCategory removes a category and all its memory associations.
func (s *service) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	if userID == "" {
		return appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return appErrors.NewValidation("categoryID cannot be empty")
	}

	// Check if category exists and belongs to user
	existingCategory, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to check for existing category")
	}
	if existingCategory == nil {
		return appErrors.NewNotFound("category not found")
	}

	return s.categoryRepo.DeleteCategory(ctx, userID, categoryID)
}

// GetCategory retrieves a single category by ID.
func (s *service) GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}

	category, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get category from repository")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	return category, nil
}

// ListCategories retrieves all categories for a user.
func (s *service) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}

	query := repository.CategoryQuery{
		UserID: userID,
	}

	categories, err := s.categoryRepo.FindCategories(ctx, query)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to list categories from repository")
	}

	return categories, nil
}

// AssignNodeToCategory associates a node with a category with validation.
func (s *service) AssignNodeToCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	if userID == "" {
		return appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return appErrors.NewValidation("categoryID cannot be empty")
	}
	if nodeID == "" {
		return appErrors.NewValidation("nodeID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	// Verify node exists and belongs to user
	node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify node")
	}
	if node == nil {
		return appErrors.NewNotFound("node not found")
	}

	mapping := domain.NodeCategory{
		UserID:     userID,
		NodeID:     nodeID,
		CategoryID: categoryID,
		Confidence: 1.0,
		Method:     "manual",
		CreatedAt:  time.Now(),
	}
	return s.categoryRepo.AssignNodeToCategory(ctx, mapping)
}

// RemoveNodeFromCategory removes a node from a category with validation.
func (s *service) RemoveNodeFromCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	if userID == "" {
		return appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return appErrors.NewValidation("categoryID cannot be empty")
	}
	if nodeID == "" {
		return appErrors.NewValidation("nodeID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	return s.categoryRepo.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

// GetNodesInCategory retrieves all nodes in a specific category.
func (s *service) GetNodesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	nodes, err := s.categoryRepo.FindNodesByCategory(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get nodes from repository")
	}

	return nodes, nil
}

// GetCategoriesForNode retrieves all categories that contain a specific node.
func (s *service) GetCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if nodeID == "" {
		return nil, appErrors.NewValidation("nodeID cannot be empty")
	}

	// Verify node exists and belongs to user
	node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	categories, err := s.categoryRepo.FindCategoriesForNode(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get categories from repository")
	}

	return categories, nil
}
