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
	categoryRepo    repository.CategoryRepository
	nodeRepo        repository.NodeRepository
	nodeCategoryMap repository.NodeCategoryMapper
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
		categoryRepo:    repo.Categories(),
		nodeRepo:        repo.Nodes(),
		nodeCategoryMap: repo.NodeCategories(),
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

	if err := s.categoryRepo.Save(ctx, &category); err != nil {
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
	domainUserID, err := domain.NewUserID(userID)
	if err != nil {
		return nil, err
	}
	existingCategory, err := s.categoryRepo.FindByID(ctx, domainUserID, categoryID)
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

	if err := s.categoryRepo.Save(ctx, &updatedCategory); err != nil {
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
	domainUserID, err := domain.NewUserID(userID)
	if err != nil {
		return err
	}
	existingCategory, err := s.categoryRepo.FindByID(ctx, domainUserID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to check for existing category")
	}
	if existingCategory == nil {
		return appErrors.NewNotFound("category not found")
	}

	return s.categoryRepo.Delete(ctx, domainUserID, categoryID)
}

// GetCategory retrieves a single category by ID.
func (s *service) GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}

	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "invalid user ID")
	}
	
	category, err := s.categoryRepo.FindByID(ctx, userIDVO, categoryID)
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

	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "invalid user ID")
	}

	categories, err := s.categoryRepo.FindByUser(ctx, userIDVO)
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
	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return appErrors.Wrap(err, "invalid user ID")
	}
	
	category, err := s.categoryRepo.FindByID(ctx, userIDVO, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	// Verify node exists and belongs to user
	nodeIDVO, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return appErrors.Wrap(err, "invalid node ID")
	}
	
	node, err := s.nodeRepo.FindByID(ctx, nodeIDVO)
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
	return s.nodeCategoryMap.AssignNodeToCategory(ctx, &mapping)
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
	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return appErrors.Wrap(err, "invalid user ID")
	}
	
	category, err := s.categoryRepo.FindByID(ctx, userIDVO, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	userIDVO, err2 := domain.NewUserID(userID)
	if err2 != nil {
		return appErrors.Wrap(err2, "invalid user ID")
	}
	
	return s.nodeCategoryMap.RemoveNodeFromCategory(ctx, userIDVO, nodeID, categoryID)
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
	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "invalid user ID")
	}
	
	category, err := s.categoryRepo.FindByID(ctx, userIDVO, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	userIDVO, err2 := domain.NewUserID(userID)
	if err2 != nil {
		return nil, appErrors.Wrap(err2, "invalid user ID")
	}
	
	nodePointers, err := s.nodeCategoryMap.FindNodesByCategory(ctx, userIDVO, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get nodes from repository")
	}
	
	// Convert from []*domain.Node to []domain.Node
	nodes := make([]domain.Node, len(nodePointers))
	for i, nodePtr := range nodePointers {
		nodes[i] = *nodePtr
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
	nodeIDVO, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "invalid node ID")
	}
	
	node, err := s.nodeRepo.FindByID(ctx, nodeIDVO)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "invalid user ID")
	}
	
	categories, err := s.nodeCategoryMap.FindCategoriesForNode(ctx, userIDVO, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get categories from repository")
	}

	// Convert from []*domain.Category to []domain.Category
	result := make([]domain.Category, len(categories))
	for i, cat := range categories {
		result[i] = *cat
	}
	
	return result, nil
}
