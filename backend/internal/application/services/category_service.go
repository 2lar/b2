// Package services contains application services that orchestrate use cases.
// This CategoryService demonstrates the Application Service pattern with AI integration fallback.
//
// Key Concepts Illustrated:
//   - Application Service Pattern: Orchestrates category-related business operations
//   - AI Service Integration: Optional AI service with graceful fallback
//   - Fallback Mechanism: Uses domain logic when AI service is unavailable
//   - Command/Query Responsibility Segregation (CQRS): Separates reads from writes
//   - Transaction Management: Uses Unit of Work pattern for consistency
//   - Domain Event Publishing: Communicates changes to other parts of the system
package services

import (
	"context"

	"brain2-backend/internal/application/adapters"
	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	categoryService "brain2-backend/internal/service/category"
	appErrors "brain2-backend/pkg/errors"
)

// CategoryService implements the Application Service pattern for category operations
// with optional AI service integration and graceful fallback to domain-based logic.
type CategoryService struct {
	// Core dependencies for category operations (using adapters)
	categoryAdapter  adapters.CategoryRepositoryAdapter // For category persistence
	nodeAdapter      adapters.NodeRepositoryAdapter     // For node operations
	uow              adapters.UnitOfWorkAdapter         // For transaction management
	eventBus         domain.EventBus                    // For domain event publishing
	idempotencyStore repository.IdempotencyStore        // For idempotent operations
	
	// Optional AI service with fallback
	aiService categoryService.AIService // Optional AI service for categorization
}

// NewCategoryService creates a new CategoryService with all required dependencies.
// The AI service is optional and the service will work without it.
func NewCategoryService(
	categoryAdapter adapters.CategoryRepositoryAdapter,
	nodeAdapter adapters.NodeRepositoryAdapter,
	uow adapters.UnitOfWorkAdapter,
	eventBus domain.EventBus,
	idempotencyStore repository.IdempotencyStore,
	aiService categoryService.AIService, // Can be nil
) *CategoryService {
	return &CategoryService{
		categoryAdapter:  categoryAdapter,
		nodeAdapter:      nodeAdapter,
		uow:              uow,
		eventBus:         eventBus,
		idempotencyStore: idempotencyStore,
		aiService:        aiService,
	}
}

// CreateCategory implements the use case for creating a new category.
func (s *CategoryService) CreateCategory(ctx context.Context, cmd *commands.CreateCategoryCommand) (*dto.CreateCategoryResult, error) {
	// 1. Start unit of work for transaction boundary
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback() // Rollback if not committed

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "CREATE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.CreateCategoryResult), nil
		}
	}

	// 3. Convert application command to domain objects
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 4. Create domain entity using factory method
	category, err := domain.NewCategory(userID, cmd.Title, cmd.Description)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create category")
	}

	// 5. Set color if provided
	if cmd.Color != "" {
		if err := category.SetColor(cmd.Color); err != nil {
			return nil, appErrors.NewValidation("invalid color: " + err.Error())
		}
	}

	// 6. Save the category
	if err := s.uow.Categories().Save(ctx, category); err != nil {
		return nil, appErrors.Wrap(err, "failed to save category")
	}

	// 7. Publish domain events
	for _, event := range category.GetUncommittedEvents() {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, appErrors.Wrap(err, "failed to publish domain event")
		}
	}
	category.MarkEventsAsCommitted()

	// 8. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 9. Convert to response DTO
	result := &dto.CreateCategoryResult{
		Category: dto.ToCategoryView(category),
		Message:  "Category created successfully",
	}

	// 10. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		s.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "CREATE_CATEGORY", cmd.UserID, result)
	}

	return result, nil
}

// UpdateCategory implements the use case for updating an existing category.
func (s *CategoryService) UpdateCategory(ctx context.Context, cmd *commands.UpdateCategoryCommand) (*dto.UpdateCategoryResult, error) {
	if !cmd.HasChanges() {
		return nil, appErrors.NewValidation("no changes specified in update command")
	}

	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "UPDATE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.UpdateCategoryResult), nil
		}
	}

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := domain.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 4. Retrieve existing category
	category, err := s.uow.Categories().FindByID(ctx, userID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 5. Apply updates using domain methods
	if cmd.UpdateTitle && cmd.Title != nil {
		if err := category.UpdateTitle(*cmd.Title); err != nil {
			return nil, appErrors.Wrap(err, "failed to update category title")
		}
	}

	if cmd.UpdateDescription && cmd.Description != nil {
		if err := category.UpdateDescription(*cmd.Description); err != nil {
			return nil, appErrors.Wrap(err, "failed to update category description")
		}
	}

	if cmd.UpdateColor && cmd.Color != nil {
		if err := category.SetColor(*cmd.Color); err != nil {
			return nil, appErrors.Wrap(err, "failed to update category color")
		}
	}

	// 6. Save updated category
	if err := s.uow.Categories().Save(ctx, category); err != nil {
		return nil, appErrors.Wrap(err, "failed to save updated category")
	}

	// 7. Publish domain events
	for _, event := range category.GetUncommittedEvents() {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, appErrors.Wrap(err, "failed to publish domain event")
		}
	}
	category.MarkEventsAsCommitted()

	// 8. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 9. Convert to response DTO
	result := &dto.UpdateCategoryResult{
		Category: dto.ToCategoryView(category),
		Message:  "Category updated successfully",
	}

	// 10. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		s.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "UPDATE_CATEGORY", cmd.UserID, result)
	}

	return result, nil
}

// DeleteCategory implements the use case for deleting a category.
func (s *CategoryService) DeleteCategory(ctx context.Context, cmd *commands.DeleteCategoryCommand) (*dto.DeleteCategoryResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "DELETE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.DeleteCategoryResult), nil
		}
	}

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := domain.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 4. Verify category exists and user owns it
	category, err := s.uow.Categories().FindByID(ctx, userID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 5. Remove all node-category relationships first
	if err := s.uow.NodeCategories().RemoveAllFromCategory(ctx, string(categoryID)); err != nil {
		return nil, appErrors.Wrap(err, "failed to remove node-category relationships")
	}

	// 6. Delete the category
	if err := s.uow.Categories().Delete(ctx, userID, categoryID); err != nil {
		return nil, appErrors.Wrap(err, "failed to delete category")
	}

	// 7. Create and publish deletion event
	deletionEvent := domain.NewCategoryDeletedEvent(categoryID, userID, category.Title, category.Level, category.NoteCount)

	if err := s.eventBus.Publish(ctx, deletionEvent); err != nil {
		return nil, appErrors.Wrap(err, "failed to publish deletion event")
	}

	// 8. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 9. Convert to response DTO
	result := &dto.DeleteCategoryResult{
		Success: true,
		Message: "Category deleted successfully",
	}

	// 10. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		s.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "DELETE_CATEGORY", cmd.UserID, result)
	}

	return result, nil
}

// AssignNodeToCategory implements the use case for assigning a node to a category.
func (s *CategoryService) AssignNodeToCategory(ctx context.Context, cmd *commands.AssignNodeToCategoryCommand) (*dto.AssignNodeToCategoryResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "ASSIGN_NODE", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.AssignNodeToCategoryResult), nil
		}
	}

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := domain.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	nodeID, err := domain.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Verify both category and node exist and belong to user
	category, err := s.uow.Categories().FindByID(ctx, userID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	node, err := s.uow.Nodes().FindByID(ctx, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}
	if !node.UserID().Equals(userID) {
		return nil, appErrors.NewUnauthorized("node belongs to different user")
	}

	// 5. Create node-category relationship
	nodeCategory, err := domain.NewNodeCategory(userID.String(), nodeID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create node-category relationship")
	}

	if err := s.uow.NodeCategories().Assign(ctx, nodeCategory); err != nil {
		return nil, appErrors.Wrap(err, "failed to save node-category relationship")
	}

	// 6. Publish domain events
	for _, event := range nodeCategory.GetUncommittedEvents() {
		if err := s.eventBus.Publish(ctx, event); err != nil {
			return nil, appErrors.Wrap(err, "failed to publish domain event")
		}
	}
	nodeCategory.MarkEventsAsCommitted()

	// 7. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 8. Convert to response DTO
	result := &dto.AssignNodeToCategoryResult{
		Success:    true,
		CategoryID: cmd.CategoryID,
		NodeID:     cmd.NodeID,
		Message:    "Node assigned to category successfully",
	}

	// 9. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		s.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "ASSIGN_NODE", cmd.UserID, result)
	}

	return result, nil
}

// RemoveNodeFromCategory implements the use case for removing a node from a category.
func (s *CategoryService) RemoveNodeFromCategory(ctx context.Context, cmd *commands.RemoveNodeFromCategoryCommand) (*dto.RemoveNodeFromCategoryResult, error) {
	// 1. Start unit of work
	if err := s.uow.Begin(ctx); err != nil {
		return nil, appErrors.Wrap(err, "failed to begin transaction")
	}
	defer s.uow.Rollback()

	// 2. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := s.checkIdempotency(ctx, *cmd.IdempotencyKey, "REMOVE_NODE", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.RemoveNodeFromCategoryResult), nil
		}
	}

	// 3. Parse domain identifiers
	userID, err := domain.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := domain.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	nodeID, err := domain.ParseNodeID(cmd.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 4. Remove the node-category relationship
	if err := s.uow.NodeCategories().Remove(ctx, userID.String(), nodeID.String(), string(categoryID)); err != nil {
		return nil, appErrors.Wrap(err, "failed to remove node from category")
	}

	// 5. Create and publish removal event
	removalEvent := domain.NewNodeRemovedFromCategoryEvent(nodeID, categoryID, userID)

	if err := s.eventBus.Publish(ctx, removalEvent); err != nil {
		return nil, appErrors.Wrap(err, "failed to publish removal event")
	}

	// 6. Commit transaction
	if err := s.uow.Commit(); err != nil {
		return nil, appErrors.Wrap(err, "failed to commit transaction")
	}

	// 7. Convert to response DTO
	result := &dto.RemoveNodeFromCategoryResult{
		Success:    true,
		CategoryID: cmd.CategoryID,
		NodeID:     cmd.NodeID,
		Message:    "Node removed from category successfully",
	}

	// 8. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		s.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "REMOVE_NODE", cmd.UserID, result)
	}

	return result, nil
}

// Helper methods for idempotency handling (same pattern as NodeService)

func (s *CategoryService) checkIdempotency(ctx context.Context, key, operation, userID string) (interface{}, bool, error) {
	if s.idempotencyStore == nil {
		return nil, false, nil
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	result, exists, err := s.idempotencyStore.Get(ctx, idempotencyKey)
	if err != nil {
		return nil, false, appErrors.Wrap(err, "failed to check idempotency")
	}

	return result, exists, nil
}

func (s *CategoryService) storeIdempotencyResult(ctx context.Context, key, operation, userID string, result interface{}) {
	if s.idempotencyStore == nil {
		return
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	s.idempotencyStore.Store(ctx, idempotencyKey, result)
}

// IsAIServiceAvailable checks if the AI service is available for use.
// This method demonstrates the fallback pattern - always check availability before using AI.
func (s *CategoryService) IsAIServiceAvailable() bool {
	return s.aiService != nil
}

// GetAIServiceStatus returns information about the AI service status.
func (s *CategoryService) GetAIServiceStatus() string {
	if s.aiService == nil {
		return "AI service not configured - using domain-based fallback"
	}
	return "AI service available"
}