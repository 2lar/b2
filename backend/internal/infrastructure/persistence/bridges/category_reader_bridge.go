package bridges

import (
	"context"
	"fmt"
	"strings"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/repository"
	"go.uber.org/zap"
)

// CategoryReaderBridge implements repository.CategoryReader using the Store interface.
// This bridge provides CQRS read operations optimized for query scenarios.
type CategoryReaderBridge struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewCategoryReaderBridge creates a new CategoryReaderBridge.
func NewCategoryReaderBridge(store persistence.Store, logger *zap.Logger) repository.CategoryReader {
	return &CategoryReaderBridge{
		store:  store,
		logger: logger,
	}
}

// FindByID retrieves a single category by ID.
func (b *CategoryReaderBridge) FindByID(ctx context.Context, userID string, categoryID string) (*domain.Category, error) {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID),
		SortKey:      "METADATA#v0",
	}

	record, err := b.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	if record == nil {
		return nil, nil // Category not found
	}

	return b.recordToCategory(record)
}

// Exists checks if a category exists.
func (b *CategoryReaderBridge) Exists(ctx context.Context, userID string, categoryID string) (bool, error) {
	category, err := b.FindByID(ctx, userID, categoryID)
	if err != nil {
		return false, err
	}
	return category != nil, nil
}

// FindByUser retrieves all categories for a user.
func (b *CategoryReaderBridge) FindByUser(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Apply query options
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	// Apply query options to store query
	if queryOpts.Limit > 0 {
		query.Limit = int32Ptr(int32(queryOpts.Limit))
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user categories: %w", err)
	}

	var categories []domain.Category
	for _, record := range result.Records {
		category, err := b.recordToCategory(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to category", zap.Error(err))
			continue
		}
		categories = append(categories, *category)
	}

	return categories, nil
}

// CountByUser counts categories for a user.
func (b *CategoryReaderBridge) CountByUser(ctx context.Context, userID string) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := b.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count categories: %w", err)
	}

	return int(result.Count), nil
}

// FindRootCategories finds categories with no parent.
func (b *CategoryReaderBridge) FindRootCategories(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Get all categories and filter for root categories
	allCategories, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var rootCategories []domain.Category
	for _, category := range allCategories {
		if category.ParentID == nil {
			rootCategories = append(rootCategories, category)
		}
	}

	return rootCategories, nil
}

// FindByLevel finds categories at a specific hierarchy level.
func (b *CategoryReaderBridge) FindByLevel(ctx context.Context, userID string, level int, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Get all categories and filter by level
	allCategories, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var levelCategories []domain.Category
	for _, category := range allCategories {
		if category.Level == level {
			levelCategories = append(levelCategories, category)
		}
	}

	return levelCategories, nil
}

// FindByName finds categories by name pattern.
func (b *CategoryReaderBridge) FindByName(ctx context.Context, userID string, namePattern string, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Get all categories and filter by name
	allCategories, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}

	var matchingCategories []domain.Category
	for _, category := range allCategories {
		if strings.Contains(strings.ToLower(category.Name), strings.ToLower(namePattern)) {
			matchingCategories = append(matchingCategories, category)
		}
	}

	return matchingCategories, nil
}

// CountBySpecification counts categories matching a specification.
func (b *CategoryReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	// This would require implementing the specification pattern for categories
	// For now, return 0
	return 0, nil
}

// CountCategories counts all categories for a user (compatibility method).
func (b *CategoryReaderBridge) CountCategories(ctx context.Context, userID string) (int, error) {
	return b.CountByUser(ctx, userID)
}

// FindChildCategories finds child categories of a parent category.
func (b *CategoryReaderBridge) FindChildCategories(ctx context.Context, userID string, parentID string) ([]domain.Category, error) {
	// Get all categories and filter by parent ID
	allCategories, err := b.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var childCategories []domain.Category
	for _, category := range allCategories {
		if category.ParentID != nil && string(*category.ParentID) == parentID {
			childCategories = append(childCategories, category)
		}
	}

	return childCategories, nil
}

// FindCategoryPath finds the path from root to a specific category.
func (b *CategoryReaderBridge) FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]domain.Category, error) {
	// TODO: Implement proper path traversal
	// For now, just return the category itself
	category, err := b.FindByID(ctx, userID, categoryID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return []domain.Category{}, nil
	}
	return []domain.Category{*category}, nil
}

// FindCategoryTree finds the complete category tree for a user.
func (b *CategoryReaderBridge) FindCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	// Return all categories - TODO: organize as tree structure
	return b.FindByUser(ctx, userID)
}

// FindMostActive finds the most active categories based on usage.
func (b *CategoryReaderBridge) FindMostActive(ctx context.Context, userID string, limit int) ([]domain.Category, error) {
	// TODO: Implement based on actual usage metrics
	// For now, return first N categories
	allCategories, err := b.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	if len(allCategories) > limit {
		return allCategories[:limit], nil
	}
	return allCategories, nil
}

// FindRecentlyUsed finds recently used categories.
func (b *CategoryReaderBridge) FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...repository.QueryOption) ([]domain.Category, error) {
	// TODO: Implement based on actual usage tracking
	// For now, return all categories
	return b.FindByUser(ctx, userID, opts...)
}

// FindBySpecification finds categories matching a specification.
func (b *CategoryReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]domain.Category, error) {
	// TODO: Implement specification pattern for categories
	// For now, return empty result
	return []domain.Category{}, nil
}

// GetCategoriesPage retrieves categories with pagination.
func (b *CategoryReaderBridge) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#CATEGORY#%s", query.UserID, pagination.Cursor),
			"SK": "METADATA#v0",
		}
	}

	result, err := b.store.Scan(ctx, storeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories page: %w", err)
	}

	// Convert records to categories
	categories := make([]domain.Category, 0, len(result.Records))
	for _, record := range result.Records {
		category, err := b.recordToCategory(&record)
		if err != nil {
			b.logger.Warn("failed to convert record to category", zap.Error(err))
			continue
		}
		categories = append(categories, *category)
	}

	// Filter by search text if provided
	if query.SearchText != "" {
		var filteredCategories []domain.Category
		for _, category := range categories {
			if strings.Contains(strings.ToLower(category.Name), strings.ToLower(query.SearchText)) ||
			   strings.Contains(strings.ToLower(category.Description), strings.ToLower(query.SearchText)) {
				filteredCategories = append(filteredCategories, category)
			}
		}
		categories = filteredCategories
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
			parts := strings.Split(pk, "#")
			if len(parts) >= 4 {
				nextCursor = parts[3] // CATEGORY ID
			}
		}
	}

	return &repository.CategoryPage{
		Items:      categories,
		HasMore:    nextCursor != "",
		NextCursor: nextCursor,
		TotalCount: int(result.Count),
		PageInfo: repository.PageInfo{
			PageSize:    pagination.GetEffectiveLimit(),
			ItemsInPage: len(categories),
		},
	}, nil
}

// Helper methods

func (b *CategoryReaderBridge) recordToCategory(record *persistence.Record) (*domain.Category, error) {
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

	var color *string
	if c, ok := record.Data["Color"].(string); ok {
		color = &c
	}

	// Create category
	category := &domain.Category{
		ID:          domain.CategoryID(categoryID),
		UserID:      userID,
		Name:        name,
		Description: description,
		Color:       color,
		Level:       0,
		NoteCount:   0,
		AIGenerated: false,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}

	return category, nil
}

// Utility functions
func stringPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}