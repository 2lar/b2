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
	
	// AddMemoryToCategory associates a memory with a category
	AddMemoryToCategory(ctx context.Context, userID, categoryID, memoryID string) error
	
	// RemoveMemoryFromCategory removes a memory from a category
	RemoveMemoryFromCategory(ctx context.Context, userID, categoryID, memoryID string) error
	
	// GetMemoriesInCategory retrieves all memories in a specific category
	GetMemoriesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error)
	
	// GetCategoriesForMemory retrieves all categories that contain a specific memory
	GetCategoriesForMemory(ctx context.Context, userID, memoryID string) ([]domain.Category, error)
}

// service implements the Service interface with concrete business logic.
type service struct {
	repo repository.Repository
}

// NewService creates a new category service with the provided repository.
func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
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

	if err := s.repo.CreateCategory(ctx, category); err != nil {
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
	existingCategory, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
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

	if err := s.repo.UpdateCategory(ctx, updatedCategory); err != nil {
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
	existingCategory, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to check for existing category")
	}
	if existingCategory == nil {
		return appErrors.NewNotFound("category not found")
	}

	return s.repo.DeleteCategory(ctx, userID, categoryID)
}

// GetCategory retrieves a single category by ID.
func (s *service) GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}

	category, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
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

	categories, err := s.repo.FindCategories(ctx, query)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to list categories from repository")
	}

	return categories, nil
}

// AddMemoryToCategory associates a memory with a category with validation.
func (s *service) AddMemoryToCategory(ctx context.Context, userID, categoryID, memoryID string) error {
	if userID == "" {
		return appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return appErrors.NewValidation("categoryID cannot be empty")
	}
	if memoryID == "" {
		return appErrors.NewValidation("memoryID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	// Verify memory exists and belongs to user
	memory, err := s.repo.FindNodeByID(ctx, userID, memoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify memory")
	}
	if memory == nil {
		return appErrors.NewNotFound("memory not found")
	}

	return s.repo.AddMemoryToCategory(ctx, userID, categoryID, memoryID)
}

// RemoveMemoryFromCategory removes a memory from a category with validation.
func (s *service) RemoveMemoryFromCategory(ctx context.Context, userID, categoryID, memoryID string) error {
	if userID == "" {
		return appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return appErrors.NewValidation("categoryID cannot be empty")
	}
	if memoryID == "" {
		return appErrors.NewValidation("memoryID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return appErrors.NewNotFound("category not found")
	}

	return s.repo.RemoveMemoryFromCategory(ctx, userID, categoryID, memoryID)
}

// GetMemoriesInCategory retrieves all memories in a specific category.
func (s *service) GetMemoriesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if categoryID == "" {
		return nil, appErrors.NewValidation("categoryID cannot be empty")
	}

	// Verify category exists and belongs to user
	category, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	memories, err := s.repo.FindMemoriesInCategory(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get memories from repository")
	}

	return memories, nil
}

// GetCategoriesForMemory retrieves all categories that contain a specific memory.
func (s *service) GetCategoriesForMemory(ctx context.Context, userID, memoryID string) ([]domain.Category, error) {
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if memoryID == "" {
		return nil, appErrors.NewValidation("memoryID cannot be empty")
	}

	// Verify memory exists and belongs to user
	memory, err := s.repo.FindNodeByID(ctx, userID, memoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to verify memory")
	}
	if memory == nil {
		return nil, appErrors.NewNotFound("memory not found")
	}

	categories, err := s.repo.FindCategoriesForMemory(ctx, userID, memoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get categories from repository")
	}

	return categories, nil
}