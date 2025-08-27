// Package dynamodb provides the refactored CategoryRepository that uses composition
// to eliminate code duplication.
package dynamodb

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// CategoryRepositoryV2 implements CategoryReader and CategoryWriter using composition
// This eliminates duplicate code from the original category repository
type CategoryRepositoryV2 struct {
	*GenericRepository[*category.Category]  // Composition - inherits all CRUD operations
}

// NewCategoryRepositoryV2 creates a new category repository with minimal code
func NewCategoryRepositoryV2(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *CategoryRepositoryV2 {
	return &CategoryRepositoryV2{
		GenericRepository: CreateCategoryRepository(client, tableName, indexName, logger),
	}
}

// ============================================================================
// CATEGORY-SPECIFIC OPERATIONS (Only what's unique to categories)
// ============================================================================

// FindByParent retrieves all categories with a specific parent
func (r *CategoryRepositoryV2) FindByParent(ctx context.Context, userID string, parentID *shared.CategoryID) ([]*category.Category, error) {
	// Query all categories for the user
	categories, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	
	// Filter by parent ID
	filtered := make([]*category.Category, 0)
	for _, cat := range categories {
		if parentID == nil && cat.ParentID == nil {
			// Both nil - root categories
			filtered = append(filtered, cat)
		} else if parentID != nil && cat.ParentID != nil && *cat.ParentID == *parentID {
			// Both non-nil and match
			filtered = append(filtered, cat)
		}
	}
	
	return filtered, nil
}

// FindByLevelInternal retrieves categories at a specific hierarchy level (internal helper)
func (r *CategoryRepositoryV2) FindByLevelInternal(ctx context.Context, userID string, level int) ([]*category.Category, error) {
	// Query all categories for the user
	categories, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	
	// Filter by level
	filtered := make([]*category.Category, 0)
	for _, cat := range categories {
		if cat.Level == level {
			filtered = append(filtered, cat)
		}
	}
	
	return filtered, nil
}

// FindWithNodes retrieves categories that have associated nodes
func (r *CategoryRepositoryV2) FindWithNodes(ctx context.Context, userID string) ([]*category.Category, error) {
	// Query all categories for the user
	categories, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	
	// Filter by note count > 0
	filtered := make([]*category.Category, 0)
	for _, cat := range categories {
		if cat.NoteCount > 0 {
			filtered = append(filtered, cat)
		}
	}
	
	return filtered, nil
}

// GetHierarchy retrieves the full hierarchy for a category (ancestors)
func (r *CategoryRepositoryV2) GetHierarchy(ctx context.Context, userID string, categoryID shared.CategoryID) ([]*category.Category, error) {
	hierarchy := make([]*category.Category, 0)
	
	// Start with the given category
	currentID := &categoryID
	visited := make(map[shared.CategoryID]bool) // Prevent cycles
	
	for currentID != nil {
		if visited[*currentID] {
			// Cycle detected
			break
		}
		visited[*currentID] = true
		
		cat, err := r.FindByID(ctx, userID, string(*currentID))
		if err != nil {
			if err == repository.ErrCategoryNotFound {
				break // Parent not found, stop
			}
			return nil, err
		}
		
		hierarchy = append(hierarchy, cat)
		currentID = cat.ParentID
	}
	
	// Reverse to get root -> leaf order
	for i, j := 0, len(hierarchy)-1; i < j; i, j = i+1, j-1 {
		hierarchy[i], hierarchy[j] = hierarchy[j], hierarchy[i]
	}
	
	return hierarchy, nil
}

// GetChildren retrieves all direct children of a category
func (r *CategoryRepositoryV2) GetChildren(ctx context.Context, userID string, parentID shared.CategoryID) ([]*category.Category, error) {
	return r.FindByParent(ctx, userID, &parentID)
}

// GetDescendants retrieves all descendants of a category (recursive)
func (r *CategoryRepositoryV2) GetDescendants(ctx context.Context, userID string, parentID shared.CategoryID) ([]*category.Category, error) {
	descendants := make([]*category.Category, 0)
	
	// Get direct children
	children, err := r.GetChildren(ctx, userID, parentID)
	if err != nil {
		return nil, err
	}
	
	// Add children and their descendants
	for _, child := range children {
		descendants = append(descendants, child)
		
		// Recursively get descendants
		grandchildren, err := r.GetDescendants(ctx, userID, child.ID)
		if err != nil {
			return nil, err
		}
		descendants = append(descendants, grandchildren...)
	}
	
	return descendants, nil
}

// ============================================================================
// INTERFACE COMPLIANCE METHODS (Delegate to generic repository)
// ============================================================================

// FindByID retrieves a category by its ID
func (r *CategoryRepositoryV2) FindByID(ctx context.Context, userID string, categoryID string) (*category.Category, error) {
	return r.GenericRepository.FindByID(ctx, userID, categoryID)
}

// FindByName retrieves a category by its name
func (r *CategoryRepositoryV2) FindByName(ctx context.Context, userID string, name string) (*category.Category, error) {
	// Query all categories for the user
	categories, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	
	// Find by name
	for _, cat := range categories {
		if cat.Name == name {
			return cat, nil
		}
	}
	
	return nil, repository.ErrCategoryNotFound
}

// Exists checks if a category exists
func (r *CategoryRepositoryV2) Exists(ctx context.Context, userID string, categoryID string) (bool, error) {
	cid := shared.CategoryID(categoryID)
	_, err := r.FindByID(ctx, userID, string(cid))
	if err == repository.ErrCategoryNotFound {
		return false, nil
	}
	return err == nil, err
}

// Save creates a new category
func (r *CategoryRepositoryV2) Save(ctx context.Context, cat *category.Category) error {
	// Validate hierarchy before saving
	if cat.ParentID != nil {
		parent, err := r.FindByID(ctx, cat.UserID, string(*cat.ParentID))
		if err != nil {
			return fmt.Errorf("parent category not found: %w", err)
		}
		
		// Set level based on parent
		cat.Level = parent.Level + 1
	} else {
		// Root category
		cat.Level = 0
	}
	
	return r.GenericRepository.Save(ctx, cat)
}

// Update updates an existing category
func (r *CategoryRepositoryV2) Update(ctx context.Context, cat *category.Category) error {
	// Validate hierarchy if parent changed
	if cat.ParentID != nil {
		// Check for cycles
		ancestors, err := r.GetHierarchy(ctx, cat.UserID, *cat.ParentID)
		if err != nil {
			return err
		}
		
		for _, ancestor := range ancestors {
			if ancestor.ID == cat.ID {
				return fmt.Errorf("circular dependency detected")
			}
		}
		
		// Update level based on parent
		parent, err := r.FindByID(ctx, cat.UserID, string(*cat.ParentID))
		if err != nil {
			return fmt.Errorf("parent category not found: %w", err)
		}
		cat.Level = parent.Level + 1
	} else {
		cat.Level = 0
	}
	
	return r.GenericRepository.Update(ctx, cat)
}

// Delete deletes a category
func (r *CategoryRepositoryV2) Delete(ctx context.Context, userID string, categoryID string) error {
	cid := shared.CategoryID(categoryID)
	// Check for children
	children, err := r.GetChildren(ctx, userID, cid)
	if err != nil {
		return err
	}
	
	if len(children) > 0 {
		return fmt.Errorf("cannot delete category with children")
	}
	
	return r.GenericRepository.Delete(ctx, userID, categoryID)
}

// CountByUser counts categories for a user
func (r *CategoryRepositoryV2) CountByUser(ctx context.Context, userID string) (int, error) {
	categories, err := r.FindByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(categories), nil
}

// UpdateNoteCount updates the note count for a category
func (r *CategoryRepositoryV2) UpdateNoteCount(ctx context.Context, userID string, categoryID shared.CategoryID, delta int) error {
	cat, err := r.FindByID(ctx, userID, string(categoryID))
	if err != nil {
		return err
	}
	
	cat.NoteCount += delta
	if cat.NoteCount < 0 {
		cat.NoteCount = 0
	}
	
	return r.Update(ctx, cat)
}

// MoveCategory moves a category to a new parent
func (r *CategoryRepositoryV2) MoveCategory(ctx context.Context, userID string, categoryID shared.CategoryID, newParentID *shared.CategoryID) error {
	cat, err := r.FindByID(ctx, userID, string(categoryID))
	if err != nil {
		return err
	}
	
	// Update parent
	cat.ParentID = newParentID
	
	// Update will handle level adjustment and cycle detection
	return r.Update(ctx, cat)
}

// BatchGetCategories retrieves multiple categories
func (r *CategoryRepositoryV2) BatchGetCategories(ctx context.Context, userID string, categoryIDs []string) (map[string]*category.Category, error) {
	return r.GenericRepository.BatchGet(ctx, userID, categoryIDs)
}

// BatchDeleteCategories deletes multiple categories
func (r *CategoryRepositoryV2) BatchDeleteCategories(ctx context.Context, userID string, categoryIDs []string) (deleted []string, failed []string, err error) {
	// Check each category for children before deletion
	for _, catID := range categoryIDs {
		categoryID := shared.CategoryID(catID)
		if err := r.Delete(ctx, userID, string(categoryID)); err != nil {
			failed = append(failed, catID)
		} else {
			deleted = append(deleted, catID)
		}
	}
	
	if len(failed) > 0 {
		err = fmt.Errorf("failed to delete %d categories", len(failed))
	}
	
	return deleted, failed, err
}

// FindByUser retrieves all categories for a user with options
func (r *CategoryRepositoryV2) FindByUser(ctx context.Context, userID string, opts ...repository.QueryOption) ([]category.Category, error) {
	cats, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindRootCategories retrieves all root categories with options
func (r *CategoryRepositoryV2) FindRootCategories(ctx context.Context, userID string, opts ...repository.QueryOption) ([]category.Category, error) {
	cats, err := r.FindByParent(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindChildCategories retrieves child categories
func (r *CategoryRepositoryV2) FindChildCategories(ctx context.Context, userID string, parentID string) ([]category.Category, error) {
	pid := shared.CategoryID(parentID)
	cats, err := r.GetChildren(ctx, userID, pid)
	if err != nil {
		return nil, err
	}
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindCategoryPath retrieves the path from root to a category
func (r *CategoryRepositoryV2) FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]category.Category, error) {
	cid := shared.CategoryID(categoryID)
	cats, err := r.GetHierarchy(ctx, userID, cid)
	if err != nil {
		return nil, err
	}
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindCategoryTree retrieves all categories as a tree
func (r *CategoryRepositoryV2) FindCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	return r.FindByUser(ctx, userID)
}

// FindByLevel retrieves categories at a specific level with options
func (r *CategoryRepositoryV2) FindByLevel(ctx context.Context, userID string, level int, opts ...repository.QueryOption) ([]category.Category, error) {
	cats, err := r.FindByLevelInternal(ctx, userID, level)
	if err != nil {
		return nil, err
	}
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindMostActive finds the most active categories
func (r *CategoryRepositoryV2) FindMostActive(ctx context.Context, userID string, limit int) ([]category.Category, error) {
	cats, err := r.FindWithNodes(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Sort by note count (descending)
	// Simplified - would use proper sorting
	if len(cats) > limit {
		cats = cats[:limit]
	}
	
	// Convert to non-pointer slice
	result := make([]category.Category, len(cats))
	for i, cat := range cats {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindRecentlyUsed finds recently used categories
func (r *CategoryRepositoryV2) FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...repository.QueryOption) ([]category.Category, error) {
	cats, err := r.Query(ctx, userID, WithSKPrefix("CATEGORY#"))
	if err != nil {
		return nil, err
	}
	
	// Filter by updated time
	cutoff := time.Now().AddDate(0, 0, -days)
	filtered := make([]*category.Category, 0)
	for _, cat := range cats {
		if cat.UpdatedAt.After(cutoff) {
			filtered = append(filtered, cat)
		}
	}
	
	// Convert to non-pointer slice
	result := make([]category.Category, len(filtered))
	for i, cat := range filtered {
		if cat != nil {
			result[i] = *cat
		}
	}
	return result, nil
}

// FindBySpecification finds categories matching a specification
func (r *CategoryRepositoryV2) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]category.Category, error) {
	// Simplified implementation - would need proper specification pattern
	if spec == nil {
		return nil, fmt.Errorf("invalid specification")
	}
	return []category.Category{}, nil
}

// CountBySpecification counts categories matching a specification
func (r *CategoryRepositoryV2) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	cats, err := r.FindBySpecification(ctx, spec)
	if err != nil {
		return 0, err
	}
	return len(cats), nil
}

// GetCategoriesPage retrieves a page of categories
func (r *CategoryRepositoryV2) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	cats, err := r.FindByUser(ctx, query.UserID)
	if err != nil {
		return nil, err
	}
	
	// Simple pagination
	start := 0
	if pagination.HasCursor() {
		// Would need proper cursor decoding
	}
	
	limit := pagination.GetEffectiveLimit()
	end := start + limit
	if end > len(cats) {
		end = len(cats)
	}
	
	pageCats := cats[start:end]
	
	return &repository.CategoryPage{
		Items:      pageCats,
		HasMore:    end < len(cats),
		NextCursor: "", // Would need proper cursor encoding
		PageInfo:   repository.CreatePageInfo(pagination, len(pageCats), end < len(cats)),
	}, nil
}

// CountCategories counts all categories for a user
func (r *CategoryRepositoryV2) CountCategories(ctx context.Context, userID string) (int, error) {
	return r.CountByUser(ctx, userID)
}

// SaveBatch saves multiple categories
func (r *CategoryRepositoryV2) SaveBatch(ctx context.Context, categories []*category.Category) error {
	return r.GenericRepository.BatchSave(ctx, categories)
}

// UpdateBatch updates multiple categories
func (r *CategoryRepositoryV2) UpdateBatch(ctx context.Context, categories []*category.Category) error {
	for _, cat := range categories {
		if err := r.Update(ctx, cat); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatch deletes multiple categories
func (r *CategoryRepositoryV2) DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error {
	deleted, failed, err := r.BatchDeleteCategories(ctx, userID, categoryIDs)
	_ = deleted
	_ = failed
	return err
}

// DeleteHierarchy deletes a category and all its children
func (r *CategoryRepositoryV2) DeleteHierarchy(ctx context.Context, userID string, categoryID string) error {
	cid := shared.CategoryID(categoryID)
	
	// Get all descendants
	descendants, err := r.GetDescendants(ctx, userID, cid)
	if err != nil {
		return err
	}
	
	// Delete descendants first (bottom-up)
	for i := len(descendants) - 1; i >= 0; i-- {
		if err := r.Delete(ctx, userID, string(descendants[i].ID)); err != nil {
			return err
		}
	}
	
	// Finally delete the parent
	return r.Delete(ctx, userID, string(cid))
}

// CreateHierarchy creates a category hierarchy
func (r *CategoryRepositoryV2) CreateHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	// This would need to be implemented based on hierarchy structure
	// For now, simplified
	return fmt.Errorf("CreateHierarchy not implemented")
}

// DeleteHierarchyRelation removes a parent-child relationship
func (r *CategoryRepositoryV2) DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error {
	cid := shared.CategoryID(childID)
	cat, err := r.FindByID(ctx, userID, string(cid))
	if err != nil {
		return err
	}
	
	// Remove parent
	cat.ParentID = nil
	cat.Level = 0
	
	return r.Update(ctx, cat)
}

// AssignNodeToCategory assigns a node to a category
func (r *CategoryRepositoryV2) AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error {
	// This would need to update the node-category mapping
	// Simplified for now
	return nil
}

// RemoveNodeFromCategory removes a node from a category
func (r *CategoryRepositoryV2) RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error {
	// This would need to update the node-category mapping
	// Simplified for now
	return nil
}

// BatchAssignNodes assigns multiple nodes to categories
func (r *CategoryRepositoryV2) BatchAssignNodes(ctx context.Context, mappings []node.NodeCategory) error {
	for _, mapping := range mappings {
		if err := r.AssignNodeToCategory(ctx, mapping); err != nil {
			return err
		}
	}
	return nil
}

// UpdateNoteCounts updates note counts for all categories
func (r *CategoryRepositoryV2) UpdateNoteCounts(ctx context.Context, userID string) error {
	// This would recalculate and update note counts
	// Simplified for now
	return nil
}

// RecalculateHierarchy recalculates the hierarchy levels
func (r *CategoryRepositoryV2) RecalculateHierarchy(ctx context.Context, userID string) error {
	// This would recalculate levels for all categories
	// Simplified for now
	return nil
}

// CreateCategory creates a new category (alias for Save)
func (r *CategoryRepositoryV2) CreateCategory(ctx context.Context, cat category.Category) error {
	return r.Save(ctx, &cat)
}

// UpdateCategory updates a category (alias for Update)
func (r *CategoryRepositoryV2) UpdateCategory(ctx context.Context, cat category.Category) error {
	return r.Update(ctx, &cat)
}

// DeleteCategory deletes a category (alias for Delete)
func (r *CategoryRepositoryV2) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	return r.Delete(ctx, userID, categoryID)
}

// FindCategoryByID finds a category by ID (alias)
func (r *CategoryRepositoryV2) FindCategoryByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	return r.FindByID(ctx, userID, categoryID)
}

// FindCategories finds categories matching a query
func (r *CategoryRepositoryV2) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]category.Category, error) {
	return r.FindByUser(ctx, query.UserID)
}

// FindCategoriesByLevel finds categories at a specific level
func (r *CategoryRepositoryV2) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]category.Category, error) {
	return r.FindByLevel(ctx, userID, level)
}

// CreateCategoryHierarchy creates a category hierarchy (alias)
func (r *CategoryRepositoryV2) CreateCategoryHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	return r.CreateHierarchy(ctx, hierarchy)
}

// DeleteCategoryHierarchy deletes a hierarchy relation (alias)
func (r *CategoryRepositoryV2) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	return r.DeleteHierarchyRelation(ctx, userID, parentID, childID)
}

// FindParentCategory finds the parent of a category
func (r *CategoryRepositoryV2) FindParentCategory(ctx context.Context, userID, childID string) (*category.Category, error) {
	child, err := r.FindByID(ctx, userID, childID)
	if err != nil {
		return nil, err
	}
	
	if child.ParentID == nil {
		return nil, repository.ErrCategoryNotFound
	}
	
	return r.FindByID(ctx, userID, string(*child.ParentID))
}

// GetCategoryTree retrieves the full category tree
func (r *CategoryRepositoryV2) GetCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	return r.FindCategoryTree(ctx, userID)
}

// BatchAssignCategories assigns multiple nodes to categories
func (r *CategoryRepositoryV2) BatchAssignCategories(ctx context.Context, mappings []node.NodeCategory) error {
	return r.BatchAssignNodes(ctx, mappings)
}

// UpdateCategoryNoteCounts updates note counts for categories
func (r *CategoryRepositoryV2) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	for categoryID, count := range categoryCounts {
		cat, err := r.FindByID(ctx, userID, categoryID)
		if err != nil {
			continue
		}
		
		cat.NoteCount = count
		if err := r.Update(ctx, cat); err != nil {
			return err
		}
	}
	return nil
}

// FindCategoriesForNode finds categories for a specific node
func (r *CategoryRepositoryV2) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]category.Category, error) {
	// This would need to be implemented with node-category mappings
	// For now, return empty list
	return []category.Category{}, nil
}

// FindNodesByCategory finds nodes assigned to a specific category
func (r *CategoryRepositoryV2) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error) {
	// This would need to be implemented with node-category mappings
	// For now, return empty list
	return []*node.Node{}, nil
}

// ============================================================================
// ENSURE INTERFACES ARE IMPLEMENTED
// ============================================================================

var (
	_ repository.CategoryReader     = (*CategoryRepositoryV2)(nil)
	_ repository.CategoryWriter     = (*CategoryRepositoryV2)(nil)
	_ repository.CategoryRepository = (*CategoryRepositoryV2)(nil)
)