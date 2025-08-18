package commands

import (
	"context"
	"fmt"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
	"go.uber.org/zap"
)

// CategoryCommandHandler handles category-related command operations using CQRS pattern.
// It directly uses the Store interface for persistence operations.
type CategoryCommandHandler struct {
	store            persistence.Store
	logger           *zap.Logger
	eventBus         shared.EventBus
	idempotencyStore repository.IdempotencyStore
}

// NewCategoryCommandHandler creates a new CategoryCommandHandler.
func NewCategoryCommandHandler(
	store persistence.Store,
	logger *zap.Logger,
	eventBus shared.EventBus,
	idempotencyStore repository.IdempotencyStore,
) *CategoryCommandHandler {
	return &CategoryCommandHandler{
		store:            store,
		logger:           logger,
		eventBus:         eventBus,
		idempotencyStore: idempotencyStore,
	}
}

// HandleCreateCategory handles the CreateCategoryCommand.
func (h *CategoryCommandHandler) HandleCreateCategory(ctx context.Context, cmd *CreateCategoryCommand) (*dto.CreateCategoryResult, error) {
	h.logger.Debug("handling create category command",
		zap.String("user_id", cmd.UserID),
		zap.String("title", cmd.Title))

	// 1. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := h.checkIdempotency(ctx, *cmd.IdempotencyKey, "CREATE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.CreateCategoryResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 3. Create domain entity using factory method
	category, err := category.NewCategory(userID, cmd.Title, cmd.Description)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create category")
	}

	// 4. Set color if provided
	if cmd.Color != nil && *cmd.Color != "" {
		if err := category.SetColor(*cmd.Color); err != nil {
			return nil, appErrors.NewValidation("invalid color: " + err.Error())
		}
	}

	// 5. Prepare operations for atomic transaction
	operations := []persistence.Operation{}

	// 6. Add category creation operation
	categoryKey := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", category.UserID, string(category.ID)),
		SortKey:      "METADATA#v0",
	}

	categoryData := map[string]interface{}{
		"CategoryID":  string(category.ID),
		"UserID":      category.UserID,
		"Name":        category.Name,
		"Description": category.Description,
		"Color":       category.Color,
		"IsLatest":    true,
		"Timestamp":   category.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	operations = append(operations, persistence.Operation{
		Type: persistence.OperationTypePut,
		Key:  categoryKey,
		Data: categoryData,
	})

	// 7. Execute transaction
	err = h.store.Transaction(ctx, operations)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create category")
	}

	// 8. Publish domain events
	for _, event := range category.GetUncommittedEvents() {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Warn("failed to publish domain event", zap.Error(err))
			// Don't fail the operation for event publishing failures
		}
	}
	category.MarkEventsAsCommitted()

	// 9. Convert to response DTO
	result := &dto.CreateCategoryResult{
		Category: dto.ToCategoryView(category),
		Message:  "Category created successfully",
	}

	// 10. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		h.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "CREATE_CATEGORY", cmd.UserID, result)
	}

	h.logger.Debug("category created successfully",
		zap.String("category_id", string(category.ID)),
		zap.String("user_id", cmd.UserID))

	return result, nil
}

// HandleUpdateCategory handles the UpdateCategoryCommand.
func (h *CategoryCommandHandler) HandleUpdateCategory(ctx context.Context, cmd *UpdateCategoryCommand) (*dto.UpdateCategoryResult, error) {
	h.logger.Debug("handling update category command",
		zap.String("user_id", cmd.UserID),
		zap.String("category_id", cmd.CategoryID))

	if !cmd.HasChanges() {
		return nil, appErrors.NewValidation("no changes specified in update command")
	}

	// 1. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := h.checkIdempotency(ctx, *cmd.IdempotencyKey, "UPDATE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.UpdateCategoryResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := shared.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 3. Retrieve existing category
	categoryKey := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID.String(), string(categoryID)),
		SortKey:      "METADATA#v0",
	}

	record, err := h.store.Get(ctx, categoryKey)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve category")
	}
	if record == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 4. Reconstruct category from record
	category, err := h.recordToCategory(record)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to reconstruct category")
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

	// 6. Prepare update operations
	updates := map[string]interface{}{
		"Name":        category.Name,
		"Description": category.Description,
		"Color":       category.Color,
		"UpdatedAt":   category.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// 7. Execute update with optimistic locking
	conditionExpr := "Version = :prevVersion"
	err = h.store.Update(ctx, categoryKey, updates, &conditionExpr)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to update category")
	}

	// 8. Publish domain events
	for _, event := range category.GetUncommittedEvents() {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Warn("failed to publish domain event", zap.Error(err))
			// Don't fail the operation for event publishing failures
		}
	}
	category.MarkEventsAsCommitted()

	// 9. Convert to response DTO
	result := &dto.UpdateCategoryResult{
		Category: dto.ToCategoryView(category),
		Message:  "Category updated successfully",
	}

	// 10. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		h.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "UPDATE_CATEGORY", cmd.UserID, result)
	}

	h.logger.Debug("category updated successfully",
		zap.String("category_id", cmd.CategoryID),
		zap.String("user_id", cmd.UserID))

	return result, nil
}

// HandleDeleteCategory handles the DeleteCategoryCommand.
func (h *CategoryCommandHandler) HandleDeleteCategory(ctx context.Context, cmd *DeleteCategoryCommand) (*dto.DeleteCategoryResult, error) {
	h.logger.Debug("handling delete category command",
		zap.String("user_id", cmd.UserID),
		zap.String("category_id", cmd.CategoryID))

	// 1. Handle idempotency if key is provided
	if cmd.IdempotencyKey != nil {
		if result, exists, err := h.checkIdempotency(ctx, *cmd.IdempotencyKey, "DELETE_CATEGORY", cmd.UserID); err != nil {
			return nil, err
		} else if exists {
			return result.(*dto.DeleteCategoryResult), nil
		}
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := shared.ParseCategoryID(cmd.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 3. Verify category exists and get its data
	categoryKey := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID.String(), cmd.CategoryID),
		SortKey:      "METADATA#v0",
	}

	record, err := h.store.Get(ctx, categoryKey)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve category")
	}
	if record == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 4. Reconstruct category for event publishing
	category, err := h.recordToCategory(record)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to reconstruct category")
	}

	// 5. Find and delete all related records (category + any node-category relationships)
	operations := []persistence.Operation{}

	// Delete the main category record
	operations = append(operations, persistence.Operation{
		Type: persistence.OperationTypeDelete,
		Key:  categoryKey,
	})

	// TODO: In a complete implementation, we would also need to find and delete
	// all node-category relationship records. For now, we'll assume they're
	// handled by the application service or via cascading deletes.

	// 6. Execute transaction
	err = h.store.Transaction(ctx, operations)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to delete category")
	}

	// 7. Create and publish deletion event
	deletionEvent := shared.NewCategoryDeletedEvent(categoryID, userID, category.Name, 0, 0)
	if err := h.eventBus.Publish(ctx, deletionEvent); err != nil {
		h.logger.Warn("failed to publish deletion event", zap.Error(err))
		// Don't fail the operation for event publishing failures
	}

	// 8. Convert to response DTO
	result := &dto.DeleteCategoryResult{
		Success: true,
		Message: "Category deleted successfully",
	}

	// 9. Store idempotency result if key was provided
	if cmd.IdempotencyKey != nil {
		h.storeIdempotencyResult(ctx, *cmd.IdempotencyKey, "DELETE_CATEGORY", cmd.UserID, result)
	}

	h.logger.Debug("category deleted successfully",
		zap.String("category_id", cmd.CategoryID),
		zap.String("user_id", cmd.UserID))

	return result, nil
}

// Helper methods

func (h *CategoryCommandHandler) recordToCategory(record *persistence.Record) (*category.Category, error) {
	// Extract required fields
	categoryID, ok := record.Data["CategoryID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing CategoryID in record")
	}

	userID, ok := record.Data["UserID"].(string)
	if !ok {
		return nil, fmt.Errorf("missing UserID in record")
	}

	name, ok := record.Data["Name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing Name in record")
	}

	// Extract optional fields with defaults
	description := ""
	if d, ok := record.Data["Description"].(string); ok {
		description = d
	}

	color := "#000000"
	if c, ok := record.Data["Color"].(string); ok {
		color = c
	}

	// Create domain category from record data
	category := &category.Category{
		ID:          shared.CategoryID(categoryID),
		UserID:      userID,
		Name:        name,
		Title:       name, // Use name as title for compatibility
		Description: description,
		Color:       &color,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	
	return category, nil
}

// Idempotency helper methods
func (h *CategoryCommandHandler) checkIdempotency(ctx context.Context, key, operation, userID string) (interface{}, bool, error) {
	if h.idempotencyStore == nil {
		return nil, false, nil
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	result, exists, err := h.idempotencyStore.Get(ctx, idempotencyKey)
	if err != nil {
		return nil, false, appErrors.Wrap(err, "failed to check idempotency")
	}

	return result, exists, nil
}

func (h *CategoryCommandHandler) storeIdempotencyResult(ctx context.Context, key, operation, userID string, result interface{}) {
	if h.idempotencyStore == nil {
		return
	}

	idempotencyKey := repository.IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      key,
	}

	if err := h.idempotencyStore.Store(ctx, idempotencyKey, result); err != nil {
		h.logger.Warn("failed to store idempotency result", zap.Error(err))
	}
}