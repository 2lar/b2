// Package dynamodb provides DynamoDB implementations of repository interfaces.
// This file implements CategoryReader and CategoryWriter interfaces using direct CQRS patterns.
package dynamodb

import (
	"context"
	"fmt"
	"time"
	
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	sharedContext "brain2-backend/internal/context"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/zap"
)

// CategoryRepositoryCQRS implements both CategoryReader and CategoryWriter interfaces directly.
type CategoryRepositoryCQRS struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	logger    *zap.Logger
}

// NewCategoryRepositoryCQRS creates a new category repository with direct CQRS support.
func NewCategoryRepositoryCQRS(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *CategoryRepositoryCQRS {
	return &CategoryRepositoryCQRS{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger,
	}
}

// Ensure interfaces are implemented
var (
	_ repository.CategoryReader     = (*CategoryRepositoryCQRS)(nil)
	_ repository.CategoryWriter     = (*CategoryRepositoryCQRS)(nil)
	_ repository.CategoryRepository = (*CategoryRepositoryCQRS)(nil)
)

// ============================================================================
// CATEGORY READER INTERFACE - Read Operations
// ============================================================================

// FindByID retrieves a category by its ID.
func (r *CategoryRepositoryCQRS) FindByID(ctx context.Context, userID string, categoryID string) (*category.Category, error) {
	// Build the composite key for DynamoDB
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", categoryID)},
	}
	
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	
	if result.Item == nil {
		return nil, repository.ErrCategoryNotFound
	}
	
	// Parse the category from DynamoDB item
	category, err := r.parseCategoryFromItem(result.Item)
	if err != nil {
		return nil, err
	}
	
	return category, nil
}

// Exists checks if a category exists.
func (r *CategoryRepositoryCQRS) Exists(ctx context.Context, userID string, categoryID string) (bool, error) {
	category, err := r.FindByID(ctx, userID, categoryID)
	if err == repository.ErrCategoryNotFound {
		return false, nil
	}
	return category != nil, err
}

// FindByUser retrieves all categories for a user.
func (r *CategoryRepositoryCQRS) FindByUser(ctx context.Context, userID string, opts ...repository.QueryOption) ([]category.Category, error) {
	// Apply query options
	options := repository.ApplyQueryOptions(opts...)
	
	// Build key condition expression
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID)))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("CATEGORY#"))
	
	// Build the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Limit:                     aws.Int32(int32(options.Limit)),
	}
	
	if options.SortOrder == repository.SortOrderAsc {
		input.ScanIndexForward = aws.Bool(true)
	} else {
		input.ScanIndexForward = aws.Bool(false)
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	
	categories := make([]category.Category, 0, len(result.Items))
	for _, item := range result.Items {
		category, err := r.parseCategoryFromItem(item)
		if err != nil {
			r.logger.Warn("Failed to parse category", zap.Error(err))
			continue
		}
		categories = append(categories, *category)
	}
	
	return categories, nil
}

// CountByUser counts categories for a user.
func (r *CategoryRepositoryCQRS) CountByUser(ctx context.Context, userID string) (int, error) {
	keyEx := expression.Key("PK").Equal(expression.Value(fmt.Sprintf("USER#%s", userID)))
	keyEx = keyEx.And(expression.Key("SK").BeginsWith("CATEGORY#"))
	
	expr, err := expression.NewBuilder().
		WithKeyCondition(keyEx).
		Build()
	if err != nil {
		return 0, fmt.Errorf("failed to build expression: %w", err)
	}
	
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		Select:                    types.SelectCount,
	}
	
	result, err := r.client.Query(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to count categories: %w", err)
	}
	
	return int(result.Count), nil
}

// FindRootCategories finds root categories (no parent).
func (r *CategoryRepositoryCQRS) FindRootCategories(ctx context.Context, userID string, opts ...repository.QueryOption) ([]category.Category, error) {
	categories, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter for root categories (no parent)
	roots := make([]category.Category, 0)
	for _, cat := range categories {
		if cat.ParentID == nil {
			roots = append(roots, cat)
		}
	}
	
	return roots, nil
}

// FindChildCategories finds child categories of a parent.
func (r *CategoryRepositoryCQRS) FindChildCategories(ctx context.Context, userID string, parentID string) ([]category.Category, error) {
	categories, err := r.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Filter for children of the specified parent
	children := make([]category.Category, 0)
	for _, cat := range categories {
		if cat.ParentID != nil && string(*cat.ParentID) == parentID {
			children = append(children, cat)
		}
	}
	
	return children, nil
}

// FindCategoryPath finds the path from root to a category.
func (r *CategoryRepositoryCQRS) FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]category.Category, error) {
	path := make([]category.Category, 0)
	
	// Start with the target category
	current, err := r.FindByID(ctx, userID, categoryID)
	if err != nil {
		return nil, err
	}
	
	path = append([]category.Category{*current}, path...)
	
	// Walk up the tree to the root
	for current.ParentID != nil {
		parent, err := r.FindByID(ctx, userID, string(*current.ParentID))
		if err != nil {
			break // Stop if we can't find parent
		}
		path = append([]category.Category{*parent}, path...)
		current = parent
	}
	
	return path, nil
}

// FindCategoryTree finds the entire category tree.
func (r *CategoryRepositoryCQRS) FindCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	return r.FindByUser(ctx, userID)
}

// FindByLevel finds categories at a specific level.
func (r *CategoryRepositoryCQRS) FindByLevel(ctx context.Context, userID string, level int, opts ...repository.QueryOption) ([]category.Category, error) {
	categories, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Filter by level
	filtered := make([]category.Category, 0)
	for _, cat := range categories {
		if cat.Level == level {
			filtered = append(filtered, cat)
		}
	}
	
	return filtered, nil
}

// FindMostActive finds most active categories.
func (r *CategoryRepositoryCQRS) FindMostActive(ctx context.Context, userID string, limit int) ([]category.Category, error) {
	categories, err := r.FindByUser(ctx, userID, repository.WithLimit(limit))
	if err != nil {
		return nil, err
	}
	
	// Sort by activity (note count) - would need actual implementation
	// For now, just return the first N categories
	return categories, nil
}

// FindRecentlyUsed finds recently used categories.
func (r *CategoryRepositoryCQRS) FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...repository.QueryOption) ([]category.Category, error) {
	categories, err := r.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]category.Category, 0)
	for _, cat := range categories {
		if cat.UpdatedAt.After(cutoff) {
			filtered = append(filtered, cat)
		}
	}
	
	return filtered, nil
}

// FindBySpecification finds categories matching a specification.
func (r *CategoryRepositoryCQRS) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]category.Category, error) {
	// This would be implemented based on the specification pattern
	// For now, return empty result
	return []category.Category{}, nil
}

// CountBySpecification counts categories matching a specification.
func (r *CategoryRepositoryCQRS) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

// GetCategoriesPage retrieves a page of categories.
func (r *CategoryRepositoryCQRS) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	opts := []repository.QueryOption{
		repository.WithLimit(pagination.Limit),
	}
	
	if pagination.Cursor != "" {
		opts = append(opts, repository.WithCursor(pagination.Cursor))
	}
	
	categories, err := r.FindByUser(ctx, query.UserID, opts...)
	if err != nil {
		return nil, err
	}
	
	// Generate next cursor if we have a full page
	nextCursor := ""
	if len(categories) == pagination.Limit && len(categories) > 0 {
		lastCat := categories[len(categories)-1]
		nextCursor = string(lastCat.ID)
	}
	
	return &repository.CategoryPage{
		Items:      categories,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// CountCategories counts all categories for a user.
func (r *CategoryRepositoryCQRS) CountCategories(ctx context.Context, userID string) (int, error) {
	return r.CountByUser(ctx, userID)
}

// ============================================================================
// CATEGORY WRITER INTERFACE - Write Operations
// ============================================================================

// Save creates a new category.
func (r *CategoryRepositoryCQRS) Save(ctx context.Context, category *category.Category) error {
	// Build the item with composite keys
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", category.UserID)},
		"SK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", category.ID)},
		"EntityType": &types.AttributeValueMemberS{Value: "CATEGORY"},
		"CategoryID": &types.AttributeValueMemberS{Value: string(category.ID)},
		"UserID":     &types.AttributeValueMemberS{Value: category.UserID},
		"Name":       &types.AttributeValueMemberS{Value: category.Name},
		"Level":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", category.Level)},
		"NoteCount":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", category.NoteCount)},
		"CreatedAt":  &types.AttributeValueMemberS{Value: category.CreatedAt.Format(time.RFC3339)},
		"UpdatedAt":  &types.AttributeValueMemberS{Value: category.UpdatedAt.Format(time.RFC3339)},
	}
	
	// Add ParentID if it exists
	if category.ParentID != nil {
		item["ParentID"] = &types.AttributeValueMemberS{Value: string(*category.ParentID)}
	} else {
		item["ParentID"] = &types.AttributeValueMemberS{Value: ""}
	}
	
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	}
	
	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to save category: %w", err)
	}
	
	return nil
}

// SaveBatch saves multiple categories in a batch.
func (r *CategoryRepositoryCQRS) SaveBatch(ctx context.Context, categories []*category.Category) error {
	// Process in batches of 25 (DynamoDB limit)
	const batchSize = 25
	
	for i := 0; i < len(categories); i += batchSize {
		end := i + batchSize
		if end > len(categories) {
			end = len(categories)
		}
		
		batch := categories[i:end]
		for _, category := range batch {
			if err := r.Save(ctx, category); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// Update updates an existing category.
func (r *CategoryRepositoryCQRS) Update(ctx context.Context, category *category.Category) error {
	// Build update expression
	update := expression.Set(expression.Name("Name"), expression.Value(category.Name)).
		Set(expression.Name("Level"), expression.Value(category.Level)).
		Set(expression.Name("NoteCount"), expression.Value(category.NoteCount)).
		Set(expression.Name("UpdatedAt"), expression.Value(category.UpdatedAt.Format(time.RFC3339)))
	
	// Set ParentID - handle nil case
	if category.ParentID != nil {
		update = update.Set(expression.Name("ParentID"), expression.Value(string(*category.ParentID)))
	} else {
		update = update.Set(expression.Name("ParentID"), expression.Value(""))
	}
	
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}
	
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", category.UserID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", category.ID)},
	}
	
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.tableName),
		Key:                       key,
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	
	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	
	return nil
}

// UpdateBatch updates multiple categories in a batch.
func (r *CategoryRepositoryCQRS) UpdateBatch(ctx context.Context, categories []*category.Category) error {
	for _, category := range categories {
		if err := r.Update(ctx, category); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes a category.
func (r *CategoryRepositoryCQRS) Delete(ctx context.Context, userID string, categoryID string) error {
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", categoryID)},
	}
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	
	return nil
}

// DeleteBatch deletes multiple categories in a batch.
func (r *CategoryRepositoryCQRS) DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error {
	for _, categoryID := range categoryIDs {
		if err := r.Delete(ctx, userID, categoryID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteHierarchy deletes a category and all its children.
func (r *CategoryRepositoryCQRS) DeleteHierarchy(ctx context.Context, userID string, categoryID string) error {
	// Find all children
	children, err := r.FindChildCategories(ctx, userID, categoryID)
	if err != nil {
		return err
	}
	
	// Recursively delete children
	for _, child := range children {
		if err := r.DeleteHierarchy(ctx, userID, string(child.ID)); err != nil {
			return err
		}
	}
	
	// Delete the category itself
	return r.Delete(ctx, userID, categoryID)
}

// CreateHierarchy creates a category hierarchy.
func (r *CategoryRepositoryCQRS) CreateHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	// Save hierarchy relationship in DynamoDB
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", hierarchy.UserID)},
		"SK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("HIERARCHY#%s#%s", hierarchy.ParentID, hierarchy.ChildID)},
		"EntityType": &types.AttributeValueMemberS{Value: "HIERARCHY"},
		"ParentID":   &types.AttributeValueMemberS{Value: hierarchy.ParentID},
		"ChildID":    &types.AttributeValueMemberS{Value: hierarchy.ChildID},
		"CreatedAt":  &types.AttributeValueMemberS{Value: hierarchy.CreatedAt.Format(time.RFC3339)},
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}
	
	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create hierarchy: %w", err)
	}
	
	return nil
}

// DeleteHierarchyRelation deletes a hierarchy relation.
func (r *CategoryRepositoryCQRS) DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error {
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("HIERARCHY#%s#%s", parentID, childID)},
	}
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete hierarchy relation: %w", err)
	}
	
	return nil
}

// AssignNodeToCategory assigns a node to a category.
func (r *CategoryRepositoryCQRS) AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error {
	// Save node-category mapping in DynamoDB
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", mapping.UserID)},
		"SK":         &types.AttributeValueMemberS{Value: fmt.Sprintf("NODECAT#%s#%s", mapping.NodeID, mapping.CategoryID)},
		"EntityType": &types.AttributeValueMemberS{Value: "NODECAT"},
		"NodeID":     &types.AttributeValueMemberS{Value: mapping.NodeID},
		"CategoryID": &types.AttributeValueMemberS{Value: mapping.CategoryID},
		"CreatedAt":  &types.AttributeValueMemberS{Value: mapping.CreatedAt.Format(time.RFC3339)},
	}
	
	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}
	
	_, err := r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to assign node to category: %w", err)
	}
	
	return nil
}

// RemoveNodeFromCategory removes a node from a category.
func (r *CategoryRepositoryCQRS) RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error {
	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("NODECAT#%s#%s", nodeID, categoryID)},
	}
	
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       key,
	}
	
	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to remove node from category: %w", err)
	}
	
	return nil
}

// BatchAssignNodes assigns multiple nodes to categories.
func (r *CategoryRepositoryCQRS) BatchAssignNodes(ctx context.Context, mappings []node.NodeCategory) error {
	for _, mapping := range mappings {
		if err := r.AssignNodeToCategory(ctx, mapping); err != nil {
			return err
		}
	}
	return nil
}

// BatchAssignCategories assigns multiple categories (compatibility with interface).
func (r *CategoryRepositoryCQRS) BatchAssignCategories(ctx context.Context, mappings []node.NodeCategory) error {
	return r.BatchAssignNodes(ctx, mappings)
}

// UpdateCategoryNoteCounts updates note counts for categories.
func (r *CategoryRepositoryCQRS) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	for categoryID, count := range categoryCounts {
		// Update note count for each category
		update := expression.Set(expression.Name("NoteCount"), expression.Value(count)).
			Set(expression.Name("UpdatedAt"), expression.Value(time.Now().Format(time.RFC3339)))
		
		expr, err := expression.NewBuilder().
			WithUpdate(update).
			Build()
		if err != nil {
			return fmt.Errorf("failed to build expression: %w", err)
		}
		
		key := map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", userID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("CATEGORY#%s", categoryID)},
		}
		
		input := &dynamodb.UpdateItemInput{
			TableName:                 aws.String(r.tableName),
			Key:                       key,
			UpdateExpression:          expr.Update(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		}
		
		if _, err := r.client.UpdateItem(ctx, input); err != nil {
			return fmt.Errorf("failed to update category note count: %w", err)
		}
	}
	
	return nil
}

// UpdateNoteCounts updates note counts for all categories.
func (r *CategoryRepositoryCQRS) UpdateNoteCounts(ctx context.Context, userID string) error {
	// This would recalculate and update note counts for all categories
	// Implementation depends on how nodes are linked to categories
	return nil
}

// RecalculateHierarchy recalculates the category hierarchy.
func (r *CategoryRepositoryCQRS) RecalculateHierarchy(ctx context.Context, userID string) error {
	// This would recalculate levels and parent-child relationships
	// Implementation depends on business requirements
	return nil
}

// ============================================================================
// CATEGORY REPOSITORY INTERFACE - Additional Methods for Compatibility
// ============================================================================

// CreateCategory creates a new category (compatibility method).
func (r *CategoryRepositoryCQRS) CreateCategory(ctx context.Context, category category.Category) error {
	return r.Save(ctx, &category)
}

// UpdateCategory updates a category (compatibility method).
func (r *CategoryRepositoryCQRS) UpdateCategory(ctx context.Context, category category.Category) error {
	return r.Update(ctx, &category)
}

// DeleteCategory deletes a category (compatibility method).
func (r *CategoryRepositoryCQRS) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	return r.Delete(ctx, userID, categoryID)
}

// FindCategoryByID finds a category by ID (compatibility method).
func (r *CategoryRepositoryCQRS) FindCategoryByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	return r.FindByID(ctx, userID, categoryID)
}

// FindCategories finds categories based on a query (compatibility method).
func (r *CategoryRepositoryCQRS) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]category.Category, error) {
	ctx = sharedContext.WithUserID(ctx, query.UserID)
	return r.FindByUser(ctx, query.UserID)
}

// FindCategoriesByLevel finds categories by level (compatibility method).
func (r *CategoryRepositoryCQRS) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]category.Category, error) {
	return r.FindByLevel(ctx, userID, level)
}

// CreateCategoryHierarchy creates a hierarchy (compatibility method).
func (r *CategoryRepositoryCQRS) CreateCategoryHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	return r.CreateHierarchy(ctx, hierarchy)
}

// DeleteCategoryHierarchy deletes a hierarchy (compatibility method).
func (r *CategoryRepositoryCQRS) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	return r.DeleteHierarchyRelation(ctx, userID, parentID, childID)
}

// FindParentCategory finds the parent of a category (compatibility method).
func (r *CategoryRepositoryCQRS) FindParentCategory(ctx context.Context, userID, childID string) (*category.Category, error) {
	child, err := r.FindByID(ctx, userID, childID)
	if err != nil {
		return nil, err
	}
	
	if child.ParentID == nil {
		return nil, nil // No parent
	}
	
	return r.FindByID(ctx, userID, string(*child.ParentID))
}

// GetCategoryTree gets the category tree (compatibility method).
func (r *CategoryRepositoryCQRS) GetCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	return r.FindCategoryTree(ctx, userID)
}

// FindNodesByCategory finds nodes in a category (compatibility method).
func (r *CategoryRepositoryCQRS) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error) {
	// This would query node-category mappings
	// For now, return empty result
	return []*node.Node{}, nil
}

// FindCategoriesForNode finds categories for a node (compatibility method).
func (r *CategoryRepositoryCQRS) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]category.Category, error) {
	// This would query node-category mappings
	// For now, return empty result
	return []category.Category{}, nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// parseCategoryFromItem parses a DynamoDB item into a Category domain object.
func (r *CategoryRepositoryCQRS) parseCategoryFromItem(item map[string]types.AttributeValue) (*category.Category, error) {
	category := &category.Category{}
	
	if v, ok := item["CategoryID"].(*types.AttributeValueMemberS); ok {
		category.ID = shared.CategoryID(v.Value)
	}
	if v, ok := item["UserID"].(*types.AttributeValueMemberS); ok {
		category.UserID = v.Value
	}
	if v, ok := item["Name"].(*types.AttributeValueMemberS); ok {
		category.Name = v.Value
	}
	if v, ok := item["Level"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%d", &category.Level)
	}
	if v, ok := item["ParentID"].(*types.AttributeValueMemberS); ok && v.Value != "" {
		parentID := shared.CategoryID(v.Value)
		category.ParentID = &parentID
	}
	if v, ok := item["NoteCount"].(*types.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%d", &category.NoteCount)
	}
	
	// Parse timestamps
	if v, ok := item["CreatedAt"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			category.CreatedAt = t
		}
	}
	if v, ok := item["UpdatedAt"].(*types.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			category.UpdatedAt = t
		}
	}
	
	return category, nil
}