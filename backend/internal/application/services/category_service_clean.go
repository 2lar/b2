package services

import (
	"context"
	"time"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// CategoryServiceClean implements category operations with PURE CQRS.
type CategoryServiceClean struct {
	// CQRS: Separate readers and writers
	categoryReader repository.CategoryReader
	categoryWriter repository.CategoryWriter
	
	// Supporting dependencies
	uowFactory       repository.UnitOfWorkFactory
	eventBus         shared.EventBus
	idempotencyStore repository.IdempotencyStore
}

// NewCategoryServiceClean creates a new CategoryService with CQRS interfaces.
func NewCategoryServiceClean(
	categoryReader repository.CategoryReader,
	categoryWriter repository.CategoryWriter,
	uowFactory repository.UnitOfWorkFactory,
	eventBus shared.EventBus,
	idempotencyStore repository.IdempotencyStore,
) *CategoryServiceClean {
	return &CategoryServiceClean{
		categoryReader:   categoryReader,
		categoryWriter:   categoryWriter,
		uowFactory:       uowFactory,
		eventBus:         eventBus,
		idempotencyStore: idempotencyStore,
	}
}

// CreateCategory creates a new category - WRITE OPERATION.
func (s *CategoryServiceClean) CreateCategory(ctx context.Context, cmd *commands.CreateCategoryCommand) (*dto.CategoryDTO, error) {
	// Create unit of work
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	// Handle idempotency
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		key := repository.IdempotencyKey{
			UserID:    cmd.UserID,
			Operation: "CreateCategory",
			Hash:      *cmd.IdempotencyKey,
			CreatedAt: time.Now(),
		}
		result, found, err := s.idempotencyStore.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if found {
			if res, ok := result.(*dto.CategoryDTO); ok {
				return res, nil
			}
		}
	}
	
	// Create domain entity
	userID, err := shared.NewUserID(cmd.UserID)
	if err != nil {
		uow.Rollback()
		return nil, appErrors.Wrap(err, "invalid user ID")
	}
	
	newCategory, err := category.NewCategory(
		userID,
		cmd.Title,        // Use Title field instead of Name
		cmd.Description,  // Pass description
	)
	if err != nil {
		uow.Rollback()
		return nil, appErrors.Wrap(err, "failed to create category")
	}
	
	// Save through writer
	writer := uow.Categories()
	if err := writer.Save(ctx, newCategory); err != nil {
		uow.Rollback()
		return nil, appErrors.Wrap(err, "failed to save category")
	}
	
	// Handle hierarchy if parent is specified
	if cmd.ParentID != nil && *cmd.ParentID != "" {
		// Set parent on the category
		parentID := shared.CategoryID(*cmd.ParentID)
		newCategory.ParentID = &parentID
		// Update the category with parent using Save
		if err := writer.Save(ctx, newCategory); err != nil {
			uow.Rollback()
			return nil, appErrors.Wrap(err, "failed to set parent category")
		}
	}
	
	// Register domain event - commented out for now as method doesn't exist
	// uow.RegisterDomainEvent(category.NewCategoryCreatedEvent(newCategory))
	
	// Commit transaction
	if err := uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}
	
	// Store idempotency result
	result := dto.CategoryFromDomain(*newCategory)
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		key := repository.IdempotencyKey{
			UserID:    cmd.UserID,
			Operation: "CreateCategory",
			Hash:      *cmd.IdempotencyKey,
			CreatedAt: time.Now(),
		}
		s.idempotencyStore.Store(ctx, key, result)
	}
	
	// Publish event - commented out for now as NewCategoryCreatedEvent doesn't exist
	// s.eventBus.Publish(ctx, category.NewCategoryCreatedEvent(newCategory))
	
	return result, nil
}

// GetCategory retrieves a category by ID - READ OPERATION.
func (s *CategoryServiceClean) GetCategory(ctx context.Context, userID, categoryID string) (*dto.CategoryDTO, error) {
	// Use reader directly - no transaction for reads
	cat, err := s.categoryReader.FindByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	
	if cat == nil {
		return nil, appErrors.NewNotFound("category not found")
	}
	
	return dto.CategoryFromDomain(*cat), nil
}

// UpdateCategory updates a category - WRITE OPERATION.
func (s *CategoryServiceClean) UpdateCategory(ctx context.Context, cmd *commands.UpdateCategoryCommand) (*dto.CategoryDTO, error) {
	// Create unit of work
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create unit of work")
	}
	
	if err := uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	// Read existing category
	reader := uow.Categories()
	existingCategory, err := reader.FindByID(ctx, cmd.UserID, cmd.CategoryID)
	if err != nil {
		uow.Rollback()
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	
	if existingCategory == nil {
		uow.Rollback()
		return nil, appErrors.NewNotFound("category not found")
	}
	
	// Apply updates using the correct field names
	if cmd.Title != nil && *cmd.Title != "" {
		existingCategory.Name = *cmd.Title
		existingCategory.Title = *cmd.Title
	}
	if cmd.Description != nil {
		existingCategory.Description = *cmd.Description
	}
	if cmd.Color != nil {
		existingCategory.Color = cmd.Color
	}
	if cmd.Icon != nil {
		existingCategory.Icon = cmd.Icon
	}
	
	// Save through writer
	writer := uow.Categories()
	if err := writer.Save(ctx, existingCategory); err != nil {
		uow.Rollback()
		return nil, appErrors.Wrap(err, "failed to update category")
	}
	
	// Register event - commented out until UnitOfWork supports it
	// uow.RegisterDomainEvent(category.NewCategoryUpdatedEvent(existingCategory))
	
	// Commit
	if err := uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}
	
	// Publish event - commented out for now
	// s.eventBus.Publish(ctx, category.NewCategoryUpdatedEvent(existingCategory))
	
	return dto.CategoryFromDomain(*existingCategory), nil
}

// DeleteCategory deletes a category - WRITE OPERATION.
func (s *CategoryServiceClean) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	// Create unit of work
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return appErrors.Wrap(err, "failed to create unit of work")
	}
	
	if err := uow.Begin(ctx); err != nil {
		return appErrors.Wrap(err, "failed to begin transaction")
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	// Check if category exists
	reader := uow.Categories()
	existingCategory, err := reader.FindByID(ctx, userID, categoryID)
	if err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to find category")
	}
	
	if existingCategory == nil {
		uow.Rollback()
		return appErrors.NewNotFound("category not found")
	}
	
	// Check for child categories
	children, err := reader.FindChildCategories(ctx, userID, categoryID)
	if err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to check for child categories")
	}
	
	if len(children) > 0 {
		uow.Rollback()
		return appErrors.BadRequest("cannot delete category with children")
	}
	
	// Delete through writer
	writer := uow.Categories()
	if err := writer.Delete(ctx, userID, categoryID); err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to delete category")
	}
	
	// Register event - commented out until UnitOfWork supports it
	// uow.RegisterDomainEvent(category.NewCategoryDeletedEvent(categoryID, userID))
	
	// Commit
	if err := uow.Commit(); err != nil {
		return appErrors.Wrap(err, "failed to commit transaction")
	}
	
	// Publish event - commented out for now
	// s.eventBus.Publish(ctx, category.NewCategoryDeletedEvent(categoryID, userID))
	
	return nil
}

// ListCategories lists categories for a user - READ OPERATION.
func (s *CategoryServiceClean) ListCategories(ctx context.Context, userID string) ([]*dto.CategoryDTO, error) {
	// Use reader directly
	categories, err := s.categoryReader.FindByUser(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to list categories")
	}
	
	// Convert to DTOs
	dtos := make([]*dto.CategoryDTO, len(categories))
	for i, cat := range categories {
		dtos[i] = dto.CategoryFromDomain(cat)
	}
	
	return dtos, nil
}

// GetCategoryTree gets the category hierarchy tree - READ OPERATION.
func (s *CategoryServiceClean) GetCategoryTree(ctx context.Context, userID string) ([]*dto.CategoryTreeNode, error) {
	// Get all categories
	categories, err := s.categoryReader.FindCategoryTree(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get category tree")
	}
	
	// Build tree structure
	return s.buildCategoryTree(categories), nil
}

// buildCategoryTree builds a tree structure from flat categories.
func (s *CategoryServiceClean) buildCategoryTree(categories []category.Category) []*dto.CategoryTreeNode {
	// Create map for quick lookup
	catMap := make(map[string]*dto.CategoryTreeNode)
	var roots []*dto.CategoryTreeNode
	
	// First pass: create nodes
	for _, cat := range categories {
		node := &dto.CategoryTreeNode{
			CategoryDTO: dto.CategoryFromDomain(cat),
			Children:    []*dto.CategoryTreeNode{},
		}
		catMap[string(cat.ID)] = node
		
		if cat.ParentID == nil || *cat.ParentID == "" {
			roots = append(roots, node)
		}
	}
	
	// Second pass: build relationships
	for _, cat := range categories {
		if cat.ParentID != nil && *cat.ParentID != "" {
			parentIDStr := string(*cat.ParentID)
			if parent, exists := catMap[parentIDStr]; exists {
				catIDStr := string(cat.ID)
				if child, exists := catMap[catIDStr]; exists {
					parent.Children = append(parent.Children, child)
				}
			}
		}
	}
	
	return roots
}

// AssignNodeToCategory assigns a node to a category - WRITE OPERATION.
func (s *CategoryServiceClean) AssignNodeToCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	// Create unit of work
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return appErrors.Wrap(err, "failed to create unit of work")
	}
	
	if err := uow.Begin(ctx); err != nil {
		return appErrors.Wrap(err, "failed to begin transaction")
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	// Verify category exists
	reader := uow.Categories()
	cat, err := reader.FindByID(ctx, userID, categoryID)
	if err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to find category")
	}
	
	if cat == nil {
		uow.Rollback()
		return appErrors.NewNotFound("category not found")
	}
	
	// Create assignment through writer
	writer := uow.Categories()
	mapping := node.NodeCategory{
		UserID:     userID,
		NodeID:     nodeID,
		CategoryID: categoryID,
	}
	
	if err := writer.AssignNodeToCategory(ctx, mapping); err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to assign node to category")
	}
	
	// Commit
	if err := uow.Commit(); err != nil {
		return appErrors.Wrap(err, "failed to commit transaction")
	}
	
	return nil
}

// RemoveNodeFromCategory removes a node from a category - WRITE OPERATION.
func (s *CategoryServiceClean) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	// Create unit of work
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return appErrors.Wrap(err, "failed to create unit of work")
	}
	
	if err := uow.Begin(ctx); err != nil {
		return appErrors.Wrap(err, "failed to begin transaction")
	}
	
	defer func() {
		if r := recover(); r != nil {
			uow.Rollback()
			panic(r)
		}
	}()
	
	// Remove through writer
	writer := uow.Categories()
	if err := writer.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID); err != nil {
		uow.Rollback()
		return appErrors.Wrap(err, "failed to remove node from category")
	}
	
	// Commit
	if err := uow.Commit(); err != nil {
		return appErrors.Wrap(err, "failed to commit transaction")
	}
	
	return nil
}