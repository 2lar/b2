// Package dynamodb provides a stub implementation of NodeCategoryRepository.
// This is a placeholder that will be fully implemented when category features are added.
package dynamodb

import (
	"context"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/repository"
)

// NodeCategoryRepositoryStub is a stub implementation of NodeCategoryRepository.
// It returns empty results and is used as a placeholder until full implementation.
type NodeCategoryRepositoryStub struct{}

// NewNodeCategoryRepositoryStub creates a new stub repository.
func NewNodeCategoryRepositoryStub() repository.NodeCategoryRepository {
	return &NodeCategoryRepositoryStub{}
}

// Assign assigns a node to a category (stub - returns success).
func (r *NodeCategoryRepositoryStub) Assign(ctx context.Context, mapping *node.NodeCategory) error {
	// Stub implementation - always succeeds
	return nil
}

// Remove removes a node from a category (stub - returns success).
func (r *NodeCategoryRepositoryStub) Remove(ctx context.Context, userID, nodeID, categoryID string) error {
	// Stub implementation - always succeeds
	return nil
}

// RemoveAllByNode removes all category assignments for a node (stub - returns success).
func (r *NodeCategoryRepositoryStub) RemoveAllByNode(ctx context.Context, userID, nodeID string) error {
	// Stub implementation - always succeeds
	return nil
}

// RemoveAllByCategory removes all node assignments from a category (stub - returns success).
func (r *NodeCategoryRepositoryStub) RemoveAllByCategory(ctx context.Context, userID, categoryID string) error {
	// Stub implementation - always succeeds
	return nil
}

// RemoveAllFromCategory removes all assignments from a category (stub - returns success).
func (r *NodeCategoryRepositoryStub) RemoveAllFromCategory(ctx context.Context, categoryID string) error {
	// Stub implementation - always succeeds
	return nil
}

// FindByNode finds all category assignments for a node (stub - returns empty).
func (r *NodeCategoryRepositoryStub) FindByNode(ctx context.Context, userID, nodeID string) ([]*node.NodeCategory, error) {
	// Stub implementation - returns empty array
	return []*node.NodeCategory{}, nil
}

// FindByCategory finds all node assignments in a category (stub - returns empty).
func (r *NodeCategoryRepositoryStub) FindByCategory(ctx context.Context, userID, categoryID string) ([]*node.NodeCategory, error) {
	// Stub implementation - returns empty array
	return []*node.NodeCategory{}, nil
}

// FindByUser finds all node-category mappings for a user (stub - returns empty).
func (r *NodeCategoryRepositoryStub) FindByUser(ctx context.Context, userID string) ([]*node.NodeCategory, error) {
	// Stub implementation - returns empty array
	return []*node.NodeCategory{}, nil
}

// Exists checks if a node-category mapping exists (stub - returns false).
func (r *NodeCategoryRepositoryStub) Exists(ctx context.Context, userID, nodeID, categoryID string) (bool, error) {
	// Stub implementation - always returns false
	return false, nil
}

// BatchAssign assigns multiple nodes to categories (stub - returns success).
func (r *NodeCategoryRepositoryStub) BatchAssign(ctx context.Context, mappings []*node.NodeCategory) error {
	// Stub implementation - always succeeds
	return nil
}

// BatchRemove removes multiple node-category mappings (stub - returns success).
func (r *NodeCategoryRepositoryStub) BatchRemove(ctx context.Context, userID string, mappings []struct{ NodeID, CategoryID string }) error {
	// Stub implementation - always succeeds
	return nil
}

// CountByCategory counts nodes in a category (stub - returns 0).
func (r *NodeCategoryRepositoryStub) CountByCategory(ctx context.Context, userID, categoryID string) (int, error) {
	// Stub implementation - always returns 0
	return 0, nil
}

// CountByNode counts categories for a node (stub - returns 0).
func (r *NodeCategoryRepositoryStub) CountByNode(ctx context.Context, userID, nodeID string) (int, error) {
	// Stub implementation - always returns 0
	return 0, nil
}

// GetNodeCountsForCategories gets node counts for multiple categories (stub - returns empty map).
func (r *NodeCategoryRepositoryStub) GetNodeCountsForCategories(ctx context.Context, userID string, categoryIDs []string) (map[string]int, error) {
	// Stub implementation - returns empty counts
	result := make(map[string]int)
	for _, id := range categoryIDs {
		result[id] = 0
	}
	return result, nil
}

// FindNodesByCategory finds nodes by category (stub - returns empty).
func (r *NodeCategoryRepositoryStub) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error) {
	// Stub implementation - returns empty array
	return []*node.Node{}, nil
}

// FindNodesByCategoryPage finds nodes by category with pagination (stub - returns empty page).
func (r *NodeCategoryRepositoryStub) FindNodesByCategoryPage(ctx context.Context, userID, categoryID string, pagination repository.Pagination) (*repository.NodePage, error) {
	// Stub implementation - returns empty page
	return &repository.NodePage{
		Items:       []*node.Node{},
		NextCursor:  "",
		HasMore:     false,
		TotalCount:  0,
		PageInfo: repository.PageInfo{
			CurrentPage: 1,
			PageSize:    pagination.GetEffectiveLimit(),
			ItemsInPage: 0,
		},
	}, nil
}

// CountNodesInCategory counts nodes in a category (stub - returns 0).
func (r *NodeCategoryRepositoryStub) CountNodesInCategory(ctx context.Context, userID, categoryID string) (int, error) {
	// Stub implementation - returns 0
	return 0, nil
}

// FindCategoriesByNode finds categories for a node (stub - returns empty).
func (r *NodeCategoryRepositoryStub) FindCategoriesByNode(ctx context.Context, userID, nodeID string) ([]*category.Category, error) {
	// Stub implementation - returns empty array
	return []*category.Category{}, nil
}