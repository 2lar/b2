package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/persistence"
	"brain2-backend/internal/repository"
	"go.uber.org/zap"
)

// CategoryRepositoryAdapter implements repository.CategoryRepository using the Store interface.
// This adapter provides database-agnostic category operations through the persistence layer.
type CategoryRepositoryAdapter struct {
	store  persistence.Store
	logger *zap.Logger
}

// NewCategoryRepositoryAdapter creates a new CategoryRepositoryAdapter.
func NewCategoryRepositoryAdapter(store persistence.Store, logger *zap.Logger) repository.CategoryRepository {
	return &CategoryRepositoryAdapter{
		store:  store,
		logger: logger,
	}
}

// CreateCategory creates a new category.
func (r *CategoryRepositoryAdapter) CreateCategory(ctx context.Context, category domain.Category) error {
	r.logger.Debug("creating category",
		zap.String("category_id", string(category.ID)),
		zap.String("name", category.Name))

	// Create the category record
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
		"Timestamp":   category.CreatedAt.Format(time.RFC3339),
	}

	categoryRecord := persistence.Record{
		Key:       categoryKey,
		Data:      categoryData,
		Version:   1, // Default version for new categories
		CreatedAt: category.CreatedAt,
		UpdatedAt: category.UpdatedAt,
	}

	err := r.store.Put(ctx, categoryRecord)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	r.logger.Debug("successfully created category", zap.String("category_id", string(category.ID)))
	return nil
}

// FindCategoryByID retrieves a category by its ID.
func (r *CategoryRepositoryAdapter) FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID),
		SortKey:      "METADATA#v0",
	}

	record, err := r.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}

	if record == nil {
		return nil, nil // Category not found
	}

	// Convert record to domain.Category
	category, err := r.recordToCategory(record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record to category: %w", err)
	}

	return category, nil
}

// FindCategoryByName retrieves a category by its name.
func (r *CategoryRepositoryAdapter) FindCategoryByName(ctx context.Context, userID, name string) (*domain.Category, error) {
	// Since we don't have a GSI for name lookup, we'll scan all categories and filter
	// In a production system, you might want to add a GSI for name-based queries
	categories, err := r.findAllUserCategories(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, category := range categories {
		if category.Name == name {
			return category, nil
		}
	}

	return nil, nil // Category not found
}

// FindCategories finds categories based on query criteria.
func (r *CategoryRepositoryAdapter) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]domain.Category, error) {
	r.logger.Debug("finding categories",
		zap.String("user_id", query.UserID),
		zap.String("search_text", query.SearchText))

	// Get all user categories
	categories, err := r.findAllUserCategories(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	// Filter by search text if provided
	var result []domain.Category
	for _, category := range categories {
		// Convert to value type
		if query.SearchText == "" || 
		   strings.Contains(strings.ToLower(category.Name), strings.ToLower(query.SearchText)) ||
		   strings.Contains(strings.ToLower(category.Description), strings.ToLower(query.SearchText)) {
			result = append(result, *category)
		}
	}

	// Apply limit and offset
	if query.Offset > 0 && query.Offset < len(result) {
		result = result[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result, nil
}

// UpdateCategory updates an existing category.
func (r *CategoryRepositoryAdapter) UpdateCategory(ctx context.Context, category domain.Category) error {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", category.UserID, string(category.ID)),
		SortKey:      "METADATA#v0",
	}

	updates := map[string]interface{}{
		"Name":        category.Name,
		"Description": category.Description,
		"Color":       category.Color,
		"UpdatedAt":   category.UpdatedAt.Format(time.RFC3339),
	}

	err := r.store.Update(ctx, key, updates, nil)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	r.logger.Debug("successfully updated category", zap.String("category_id", string(category.ID)))
	return nil
}

// DeleteCategory removes a category.
func (r *CategoryRepositoryAdapter) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	key := persistence.Key{
		PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID),
		SortKey:      "METADATA#v0",
	}

	err := r.store.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	r.logger.Debug("successfully deleted category", zap.String("category_id", categoryID))
	return nil
}

// GetCategoriesPage retrieves a paginated list of categories.
func (r *CategoryRepositoryAdapter) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	// Convert pagination to store query
	storeQuery := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", query.UserID),
		SortKeyPrefix: stringPtr("METADATA#"),
		Limit:         int32Ptr(int32(pagination.GetEffectiveLimit())),
	}

	// Add pagination cursor if provided
	if pagination.Cursor != "" {
		// Parse cursor to last evaluated key
		lastCategoryID := pagination.Cursor
		storeQuery.LastEvaluated = map[string]interface{}{
			"PK": fmt.Sprintf("USER#%s#CATEGORY#%s", query.UserID, lastCategoryID),
			"SK": "METADATA#v0",
		}
	}

	result, err := r.store.Scan(ctx, storeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories page: %w", err)
	}

	// Convert records to categories
	categories := make([]domain.Category, 0, len(result.Records))
	for _, record := range result.Records {
		category, err := r.recordToCategory(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to category", zap.Error(err))
			continue
		}
		categories = append(categories, *category)
	}

	// Determine next cursor
	var nextCursor string
	if result.LastEvaluated != nil {
		if pk, ok := result.LastEvaluated["PK"].(string); ok {
			// Extract category ID from PK
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

// CountCategories counts the total number of categories for a user.
func (r *CategoryRepositoryAdapter) CountCategories(ctx context.Context, userID string) (int, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count categories: %w", err)
	}

	return int(result.Count), nil
}

// FindCategoriesWithOptions finds categories with additional query options.
func (r *CategoryRepositoryAdapter) FindCategoriesWithOptions(ctx context.Context, query repository.CategoryQuery, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Apply query options to create QueryOptions
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	// Use the regular FindCategories method for now
	// TODO: Apply queryOpts for filtering/sorting
	return r.FindCategories(ctx, query)
}

// FindCategoriesPageWithOptions finds categories with pagination and additional options.
func (r *CategoryRepositoryAdapter) FindCategoriesPageWithOptions(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.CategoryPage, error) {
	// Apply query options to create QueryOptions
	queryOpts := &repository.QueryOptions{}
	for _, opt := range opts {
		opt(queryOpts)
	}

	// Use the regular GetCategoriesPage method for now
	// TODO: Apply queryOpts for filtering/sorting
	return r.GetCategoriesPage(ctx, query, pagination)
}

// Helper methods

func (r *CategoryRepositoryAdapter) findCategoriesByIDs(ctx context.Context, userID string, categoryIDs []string) ([]*domain.Category, error) {
	keys := make([]persistence.Key, len(categoryIDs))
	for i, categoryID := range categoryIDs {
		keys[i] = persistence.Key{
			PartitionKey: fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID),
			SortKey:      "METADATA#v0",
		}
	}

	records, err := r.store.BatchGet(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get categories: %w", err)
	}

	categories := make([]*domain.Category, 0, len(records))
	for _, record := range records {
		category, err := r.recordToCategory(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to category", zap.Error(err))
			continue
		}
		categories = append(categories, category)
	}

	return categories, nil
}

func (r *CategoryRepositoryAdapter) findAllUserCategories(ctx context.Context, userID string) ([]*domain.Category, error) {
	query := persistence.Query{
		PartitionKey:  fmt.Sprintf("USER#%s#CATEGORY#", userID),
		SortKeyPrefix: stringPtr("METADATA#"),
	}

	result, err := r.store.Scan(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user categories: %w", err)
	}

	categories := make([]*domain.Category, 0, len(result.Records))
	for _, record := range result.Records {
		category, err := r.recordToCategory(&record)
		if err != nil {
			r.logger.Warn("failed to convert record to category", zap.Error(err))
			continue
		}
		categories = append(categories, category)
	}

	return categories, nil
}

func (r *CategoryRepositoryAdapter) recordToCategory(record *persistence.Record) (*domain.Category, error) {
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

	// Parse timestamp
	var createdAt time.Time
	if ts, ok := record.Data["Timestamp"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, ts)
	} else {
		createdAt = record.CreatedAt
	}

	// Create category using domain constructor
	return &domain.Category{
		ID:          domain.CategoryID(categoryID),
		UserID:      userID,
		Name:        name,
		Description: description,
		Color:       color,
		Level:       0,
		NoteCount:   0,
		AIGenerated: false,
		CreatedAt:   createdAt,
		UpdatedAt:   record.UpdatedAt,
	}, nil
}

// Additional methods required by repository.CategoryRepository interface

// Save implements the adapter-compatible Save method
func (r *CategoryRepositoryAdapter) Save(ctx context.Context, category *domain.Category) error {
	return r.CreateCategory(ctx, *category)
}

// FindByID implements the adapter-compatible FindByID method
func (r *CategoryRepositoryAdapter) FindByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	return r.FindCategoryByID(ctx, userID, categoryID)
}

// Delete implements the adapter-compatible Delete method
func (r *CategoryRepositoryAdapter) Delete(ctx context.Context, userID, categoryID string) error {
	return r.DeleteCategory(ctx, userID, categoryID)
}

// FindCategoriesByLevel finds categories by hierarchy level
func (r *CategoryRepositoryAdapter) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]domain.Category, error) {
	// TODO: Implement proper level-based filtering
	categories, err := r.findAllUserCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	var result []domain.Category
	for _, cat := range categories {
		if cat.Level == level {
			result = append(result, *cat)
		}
	}
	return result, nil
}

// Category hierarchy operations (placeholders for now)
func (r *CategoryRepositoryAdapter) CreateCategoryHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	// TODO: Implement category hierarchy creation
	return nil
}

func (r *CategoryRepositoryAdapter) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	// TODO: Implement category hierarchy deletion
	return nil
}

func (r *CategoryRepositoryAdapter) FindChildCategories(ctx context.Context, userID, parentID string) ([]domain.Category, error) {
	// TODO: Implement child category finding
	return []domain.Category{}, nil
}

func (r *CategoryRepositoryAdapter) FindParentCategory(ctx context.Context, userID, childID string) (*domain.Category, error) {
	// TODO: Implement parent category finding
	return nil, nil
}

func (r *CategoryRepositoryAdapter) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	// TODO: Implement category tree retrieval
	categories, err := r.findAllUserCategories(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	var result []domain.Category
	for _, cat := range categories {
		result = append(result, *cat)
	}
	return result, nil
}

// Node-category mapping operations (placeholders for now)
func (r *CategoryRepositoryAdapter) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	// TODO: Implement node-category assignment
	return nil
}

func (r *CategoryRepositoryAdapter) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	// TODO: Implement node-category removal
	return nil
}

func (r *CategoryRepositoryAdapter) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*domain.Node, error) {
	// TODO: Implement finding nodes by category
	return []*domain.Node{}, nil
}

func (r *CategoryRepositoryAdapter) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	// TODO: Implement finding categories for node
	return []domain.Category{}, nil
}

func (r *CategoryRepositoryAdapter) BatchAssignCategories(ctx context.Context, mappings []domain.NodeCategory) error {
	// TODO: Implement batch category assignment
	return nil
}

func (r *CategoryRepositoryAdapter) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	// TODO: Implement category note count updates
	return nil
}

