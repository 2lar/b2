package transaction

import (
	"context"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/repository"
)

// transactionalCategoryWrapper wraps category repository operations with transaction context
type transactionalCategoryWrapper struct {
	base repository.CategoryRepository
	tx   repository.Transaction
}

// NewTransactionalCategoryWrapper creates a new transactional category repository wrapper
func NewTransactionalCategoryWrapper(base repository.CategoryRepository, tx repository.Transaction) repository.CategoryRepository {
	return &transactionalCategoryWrapper{
		base: base,
		tx:   tx,
	}
}

func (w *transactionalCategoryWrapper) CreateCategory(ctx context.Context, category category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateCategory(ctx, category)
}

func (w *transactionalCategoryWrapper) FindCategoryByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoryByID(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) UpdateCategory(ctx context.Context, category category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.UpdateCategory(ctx, category)
}

func (w *transactionalCategoryWrapper) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteCategory(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) FindCategories(ctx context.Context, query repository.CategoryQuery) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategories(ctx, query)
}

func (w *transactionalCategoryWrapper) AssignNodeToCategory(ctx context.Context, mapping node.NodeCategory) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.AssignNodeToCategory(ctx, mapping)
}

func (w *transactionalCategoryWrapper) RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

func (w *transactionalCategoryWrapper) FindCategoriesByLevel(ctx context.Context, userID string, level int) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoriesByLevel(ctx, userID, level)
}

func (w *transactionalCategoryWrapper) Save(ctx context.Context, category *category.Category) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.Save(ctx, category)
}

func (w *transactionalCategoryWrapper) FindByID(ctx context.Context, userID, categoryID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindByID(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) Delete(ctx context.Context, userID, categoryID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.Delete(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) CreateCategoryHierarchy(ctx context.Context, hierarchy category.CategoryHierarchy) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.CreateCategoryHierarchy(ctx, hierarchy)
}

func (w *transactionalCategoryWrapper) DeleteCategoryHierarchy(ctx context.Context, userID, parentID, childID string) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.DeleteCategoryHierarchy(ctx, userID, parentID, childID)
}

func (w *transactionalCategoryWrapper) FindChildCategories(ctx context.Context, userID, parentID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindChildCategories(ctx, userID, parentID)
}

func (w *transactionalCategoryWrapper) FindParentCategory(ctx context.Context, userID, childID string) (*category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindParentCategory(ctx, userID, childID)
}

func (w *transactionalCategoryWrapper) GetCategoryTree(ctx context.Context, userID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.GetCategoryTree(ctx, userID)
}

func (w *transactionalCategoryWrapper) FindNodesByCategory(ctx context.Context, userID, categoryID string) ([]*node.Node, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindNodesByCategory(ctx, userID, categoryID)
}

func (w *transactionalCategoryWrapper) FindCategoriesForNode(ctx context.Context, userID, nodeID string) ([]category.Category, error) {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.FindCategoriesForNode(ctx, userID, nodeID)
}

func (w *transactionalCategoryWrapper) BatchAssignCategories(ctx context.Context, mappings []node.NodeCategory) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.BatchAssignCategories(ctx, mappings)
}

func (w *transactionalCategoryWrapper) UpdateCategoryNoteCounts(ctx context.Context, userID string, categoryCounts map[string]int) error {
	ctx = context.WithValue(ctx, txContextKey, w.tx)
	return w.base.UpdateCategoryNoteCounts(ctx, userID, categoryCounts)
}