package cqrs

import (
	"context"
	"fmt"
	"log"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// NodeReaderAdapter adapts existing NodeRepository to implement NodeReader interface.
// This provides CQRS read operations with optimized query capabilities.
type NodeReaderAdapter struct {
	nodeRepo repository.NodeRepository
}

// NewNodeReaderAdapter creates a new NodeReaderAdapter.
func NewNodeReaderAdapter(nodeRepo repository.NodeRepository) repository.NodeReader {
	return &NodeReaderAdapter{nodeRepo: nodeRepo}
}

// Single entity queries
func (r *NodeReaderAdapter) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Extract user ID from domain context or use a workaround
	// For now, we need to find a way to get userID - this is a limitation of the interface
	return nil, fmt.Errorf("FindByID not fully implemented - requires userID context")
}

func (r *NodeReaderAdapter) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	// Similar issue - need userID context
	return false, fmt.Errorf("Exists not fully implemented - requires userID context")
}

// User-scoped queries
func (r *NodeReaderAdapter) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	return r.nodeRepo.FindNodes(ctx, query)
}

func (r *NodeReaderAdapter) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	return r.nodeRepo.CountNodes(ctx, userID.String())
}

// Content-based queries
func (r *NodeReaderAdapter) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID:   userID.String(),
		Keywords: keywords,
	}
	return r.nodeRepo.FindNodes(ctx, query)
}

func (r *NodeReaderAdapter) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require extending NodeQuery to support tags filtering
	return nil, fmt.Errorf("FindByTags not implemented - requires tags query support")
}

func (r *NodeReaderAdapter) FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require extending NodeQuery to support content search
	return nil, fmt.Errorf("FindByContent not implemented - requires content search support")
}

// Time-based queries
func (r *NodeReaderAdapter) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require extending NodeQuery to support date range filtering
	return nil, fmt.Errorf("FindRecentlyCreated not implemented - requires date range support")
}

func (r *NodeReaderAdapter) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require extending NodeQuery to support date range filtering
	return nil, fmt.Errorf("FindRecentlyUpdated not implemented - requires date range support")
}

// Specification-based queries
func (r *NodeReaderAdapter) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return nil, fmt.Errorf("FindBySpecification not implemented - requires specification support")
}

func (r *NodeReaderAdapter) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, fmt.Errorf("CountBySpecification not implemented - requires specification support")
}

// Paginated queries
func (r *NodeReaderAdapter) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.nodeRepo.GetNodesPage(ctx, query, pagination)
}

// Relationship queries
func (r *NodeReaderAdapter) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require a more complex implementation using the graph repository
	return nil, fmt.Errorf("FindConnected not implemented - requires graph traversal")
}

func (r *NodeReaderAdapter) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would require similarity search capabilities
	return nil, fmt.Errorf("FindSimilar not implemented - requires similarity search")
}

// Query service compatibility methods
func (r *NodeReaderAdapter) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return r.nodeRepo.GetNodesPage(ctx, query, pagination)
}

func (r *NodeReaderAdapter) CountNodes(ctx context.Context, userID string) (int, error) {
	return r.nodeRepo.CountNodes(ctx, userID)
}

// NodeWriterAdapter adapts existing NodeRepository to implement NodeWriter interface.
type NodeWriterAdapter struct {
	nodeRepo repository.NodeRepository
}

// NewNodeWriterAdapter creates a new NodeWriterAdapter.
func NewNodeWriterAdapter(nodeRepo repository.NodeRepository) repository.NodeWriter {
	return &NodeWriterAdapter{nodeRepo: nodeRepo}
}

// Create operations
func (w *NodeWriterAdapter) Save(ctx context.Context, node *domain.Node) error {
	return w.nodeRepo.CreateNodeAndKeywords(ctx, node)
}

func (w *NodeWriterAdapter) SaveBatch(ctx context.Context, nodes []*domain.Node) error {
	// Batch operations would require transactional support
	for _, node := range nodes {
		if err := w.Save(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Update operations
func (w *NodeWriterAdapter) Update(ctx context.Context, node *domain.Node) error {
	// This would require an UpdateNode method on the base repository
	return fmt.Errorf("Update not implemented - requires UpdateNode method")
}

func (w *NodeWriterAdapter) UpdateBatch(ctx context.Context, nodes []*domain.Node) error {
	for _, node := range nodes {
		if err := w.Update(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Delete operations
func (w *NodeWriterAdapter) Delete(ctx context.Context, id domain.NodeID) error {
	// Need to extract userID somehow - this is a limitation
	return fmt.Errorf("Delete not fully implemented - requires userID context")
}

func (w *NodeWriterAdapter) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	for _, id := range ids {
		if err := w.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// Soft delete operations (archiving)
func (w *NodeWriterAdapter) Archive(ctx context.Context, id domain.NodeID) error {
	return fmt.Errorf("Archive not implemented - requires archiving support")
}

func (w *NodeWriterAdapter) Unarchive(ctx context.Context, id domain.NodeID) error {
	return fmt.Errorf("Unarchive not implemented - requires archiving support")
}

// Version management for optimistic locking
func (w *NodeWriterAdapter) UpdateVersion(ctx context.Context, id domain.NodeID, expectedVersion domain.Version) error {
	return fmt.Errorf("UpdateVersion not implemented - requires version management")
}

// EdgeReaderAdapter adapts existing EdgeRepository to implement EdgeReader interface.
type EdgeReaderAdapter struct {
	edgeRepo repository.EdgeRepository
}

// NewEdgeReaderAdapter creates a new EdgeReaderAdapter.
func NewEdgeReaderAdapter(edgeRepo repository.EdgeRepository) repository.EdgeReader {
	return &EdgeReaderAdapter{edgeRepo: edgeRepo}
}

// Single entity queries
func (r *EdgeReaderAdapter) FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error) {
	// Edge lookup by ID is complex since edges are identified by source+target
	return nil, fmt.Errorf("FindByID not implemented for edges - edges identified by source+target")
}

func (r *EdgeReaderAdapter) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	return false, fmt.Errorf("Exists not implemented for edges")
}

// User-scoped queries
func (r *EdgeReaderAdapter) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{
		UserID: userID.String(),
	}
	return r.edgeRepo.FindEdges(ctx, query)
}

func (r *EdgeReaderAdapter) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	edges, err := r.FindByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(edges), nil
}

// Node relationship queries
func (r *EdgeReaderAdapter) FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Need to extract userID from context
	return nil, fmt.Errorf("FindBySourceNode not fully implemented - requires userID context")
}

func (r *EdgeReaderAdapter) FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindByTargetNode not fully implemented - requires userID context")
}

func (r *EdgeReaderAdapter) FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindByNode not fully implemented - requires userID context")
}

func (r *EdgeReaderAdapter) FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindBetweenNodes not implemented")
}

// Weight-based queries
func (r *EdgeReaderAdapter) FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindStrongConnections not implemented - requires weight filtering")
}

func (r *EdgeReaderAdapter) FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindWeakConnections not implemented - requires weight filtering")
}

// Specification-based queries
func (r *EdgeReaderAdapter) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("FindBySpecification not implemented for edges")
}

func (r *EdgeReaderAdapter) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, fmt.Errorf("CountBySpecification not implemented for edges")
}

// Paginated queries
func (r *EdgeReaderAdapter) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return r.edgeRepo.GetEdgesPage(ctx, query, pagination)
}

// Query service compatibility methods
func (r *EdgeReaderAdapter) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	return r.edgeRepo.FindEdges(ctx, query)
}

func (r *EdgeReaderAdapter) CountBySourceID(ctx context.Context, sourceID domain.NodeID) (int, error) {
	return 0, fmt.Errorf("CountBySourceID not implemented")
}

// EdgeWriterAdapter adapts existing EdgeRepository to implement EdgeWriter interface.
type EdgeWriterAdapter struct {
	edgeRepo repository.EdgeRepository
}

// NewEdgeWriterAdapter creates a new EdgeWriterAdapter.
func NewEdgeWriterAdapter(edgeRepo repository.EdgeRepository) repository.EdgeWriter {
	return &EdgeWriterAdapter{edgeRepo: edgeRepo}
}

// Create operations
func (w *EdgeWriterAdapter) Save(ctx context.Context, edge *domain.Edge) error {
	return w.edgeRepo.CreateEdge(ctx, edge)
}

func (w *EdgeWriterAdapter) SaveBatch(ctx context.Context, edges []*domain.Edge) error {
	for _, edge := range edges {
		if err := w.Save(ctx, edge); err != nil {
			return err
		}
	}
	return nil
}

// Update operations (edges are typically immutable, but weight can change)
func (w *EdgeWriterAdapter) UpdateWeight(ctx context.Context, id domain.NodeID, newWeight float64, expectedVersion domain.Version) error {
	return fmt.Errorf("UpdateWeight not implemented - requires weight update support")
}

// Delete operations
func (w *EdgeWriterAdapter) Delete(ctx context.Context, id domain.NodeID) error {
	return fmt.Errorf("Delete not implemented for edges - requires edge identification")
}

func (w *EdgeWriterAdapter) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	return fmt.Errorf("DeleteBatch not implemented for edges")
}

func (w *EdgeWriterAdapter) DeleteByNode(ctx context.Context, nodeID domain.NodeID) error {
	return fmt.Errorf("DeleteByNode not implemented - requires userID context")
}

// Bulk operations for performance
func (w *EdgeWriterAdapter) SaveManyToOne(ctx context.Context, sourceID domain.NodeID, targetIDs []domain.NodeID, weights []float64) error {
	// This would map to CreateEdges method
	return fmt.Errorf("SaveManyToOne not implemented - requires bulk edge creation")
}

func (w *EdgeWriterAdapter) SaveOneToMany(ctx context.Context, sourceIDs []domain.NodeID, targetID domain.NodeID, weights []float64) error {
	return fmt.Errorf("SaveOneToMany not implemented - requires bulk edge creation")
}

// CategoryReaderAdapter adapts existing CategoryRepository to implement CategoryReader interface.
type CategoryReaderAdapter struct {
	categoryRepo repository.CategoryRepository
}

// NewCategoryReaderAdapter creates a new CategoryReaderAdapter.
func NewCategoryReaderAdapter(categoryRepo repository.CategoryRepository) repository.CategoryReader {
	return &CategoryReaderAdapter{categoryRepo: categoryRepo}
}

// Single entity queries
func (r *CategoryReaderAdapter) FindByID(ctx context.Context, userID string, categoryID string) (*domain.Category, error) {
	return r.categoryRepo.FindCategoryByID(ctx, userID, categoryID)
}

func (r *CategoryReaderAdapter) Exists(ctx context.Context, userID string, categoryID string) (bool, error) {
	category, err := r.FindByID(ctx, userID, categoryID)
	if err != nil {
		return false, err
	}
	return category != nil, nil
}

// User-scoped queries
func (r *CategoryReaderAdapter) FindByUser(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	query := repository.CategoryQuery{
		UserID: userID,
	}
	return r.categoryRepo.FindCategories(ctx, query)
}

func (r *CategoryReaderAdapter) CountByUser(ctx context.Context, userID string) (int, error) {
	categories, err := r.FindByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(categories), nil
}

// Hierarchy queries
func (r *CategoryReaderAdapter) FindRootCategories(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	return r.categoryRepo.FindCategoriesByLevel(ctx, userID, 0)
}

func (r *CategoryReaderAdapter) FindChildCategories(ctx context.Context, userID string, parentID string) ([]domain.Category, error) {
	return r.categoryRepo.FindChildCategories(ctx, userID, parentID)
}

func (r *CategoryReaderAdapter) FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]domain.Category, error) {
	// This would require building the path from child to root
	return nil, fmt.Errorf("FindCategoryPath not implemented - requires path building")
}

func (r *CategoryReaderAdapter) FindCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	return r.categoryRepo.GetCategoryTree(ctx, userID)
}

// Level-based queries
func (r *CategoryReaderAdapter) FindByLevel(ctx context.Context, userID string, level int, opts ...repository.QueryOption) ([]domain.Category, error) {
	return r.categoryRepo.FindCategoriesByLevel(ctx, userID, level)
}

// Activity queries
func (r *CategoryReaderAdapter) FindMostActive(ctx context.Context, userID string, limit int) ([]domain.Category, error) {
	return nil, fmt.Errorf("FindMostActive not implemented - requires activity tracking")
}

func (r *CategoryReaderAdapter) FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...repository.QueryOption) ([]domain.Category, error) {
	return nil, fmt.Errorf("FindRecentlyUsed not implemented - requires usage tracking")
}

// Specification-based queries
func (r *CategoryReaderAdapter) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]domain.Category, error) {
	return nil, fmt.Errorf("FindBySpecification not implemented for categories")
}

func (r *CategoryReaderAdapter) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, fmt.Errorf("CountBySpecification not implemented for categories")
}

// Query service compatibility methods
func (r *CategoryReaderAdapter) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	// This would require implementing CategoryPage in the base repository
	return nil, fmt.Errorf("GetCategoriesPage not implemented - requires CategoryPage support")
}

func (r *CategoryReaderAdapter) CountCategories(ctx context.Context, userID string) (int, error) {
	return r.CountByUser(ctx, userID)
}

// CategoryWriterAdapter adapts existing CategoryRepository to implement CategoryWriter interface.
type CategoryWriterAdapter struct {
	categoryRepo repository.CategoryRepository
}

// NewCategoryWriterAdapter creates a new CategoryWriterAdapter.
func NewCategoryWriterAdapter(categoryRepo repository.CategoryRepository) repository.CategoryWriter {
	return &CategoryWriterAdapter{categoryRepo: categoryRepo}
}

// Create operations
func (w *CategoryWriterAdapter) Save(ctx context.Context, category *domain.Category) error {
	return w.categoryRepo.CreateCategory(ctx, *category)
}

func (w *CategoryWriterAdapter) SaveBatch(ctx context.Context, categories []*domain.Category) error {
	for _, category := range categories {
		if err := w.Save(ctx, category); err != nil {
			return err
		}
	}
	return nil
}

// Update operations
func (w *CategoryWriterAdapter) Update(ctx context.Context, category *domain.Category) error {
	return w.categoryRepo.UpdateCategory(ctx, *category)
}

func (w *CategoryWriterAdapter) UpdateBatch(ctx context.Context, categories []*domain.Category) error {
	for _, category := range categories {
		if err := w.Update(ctx, category); err != nil {
			return err
		}
	}
	return nil
}

// Delete operations
func (w *CategoryWriterAdapter) Delete(ctx context.Context, userID string, categoryID string) error {
	return w.categoryRepo.DeleteCategory(ctx, userID, categoryID)
}

func (w *CategoryWriterAdapter) DeleteBatch(ctx context.Context, userID string, categoryIDs []string) error {
	for _, categoryID := range categoryIDs {
		if err := w.Delete(ctx, userID, categoryID); err != nil {
			return err
		}
	}
	return nil
}

func (w *CategoryWriterAdapter) DeleteHierarchy(ctx context.Context, userID string, categoryID string) error {
	// This would require recursive deletion of children
	return fmt.Errorf("DeleteHierarchy not implemented - requires recursive deletion")
}

// Hierarchy operations
func (w *CategoryWriterAdapter) CreateHierarchy(ctx context.Context, hierarchy domain.CategoryHierarchy) error {
	return w.categoryRepo.CreateCategoryHierarchy(ctx, hierarchy)
}

func (w *CategoryWriterAdapter) DeleteHierarchyRelation(ctx context.Context, userID string, parentID string, childID string) error {
	return w.categoryRepo.DeleteCategoryHierarchy(ctx, userID, parentID, childID)
}

// Node-Category mapping operations
func (w *CategoryWriterAdapter) AssignNodeToCategory(ctx context.Context, mapping domain.NodeCategory) error {
	return w.categoryRepo.AssignNodeToCategory(ctx, mapping)
}

func (w *CategoryWriterAdapter) RemoveNodeFromCategory(ctx context.Context, userID string, nodeID string, categoryID string) error {
	return w.categoryRepo.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

func (w *CategoryWriterAdapter) BatchAssignNodes(ctx context.Context, mappings []domain.NodeCategory) error {
	return w.categoryRepo.BatchAssignCategories(ctx, mappings)
}

// Maintenance operations
func (w *CategoryWriterAdapter) UpdateNoteCounts(ctx context.Context, userID string) error {
	// This would require recalculating all category note counts
	return fmt.Errorf("UpdateNoteCounts not implemented - requires count recalculation")
}

func (w *CategoryWriterAdapter) RecalculateHierarchy(ctx context.Context, userID string) error {
	return fmt.Errorf("RecalculateHierarchy not implemented - requires hierarchy recalculation")
}

// Combined CQRS Repository Implementations

// CQRSNodeRepositoryAdapter combines read and write adapters for complete CQRS support.
type CQRSNodeRepositoryAdapter struct {
	repository.NodeReader
	repository.NodeWriter
}

// NewCQRSNodeRepositoryAdapter creates a combined CQRS node repository.
func NewCQRSNodeRepositoryAdapter(nodeRepo repository.NodeRepository) repository.CQRSNodeRepository {
	return &CQRSNodeRepositoryAdapter{
		NodeReader: NewNodeReaderAdapter(nodeRepo),
		NodeWriter: NewNodeWriterAdapter(nodeRepo),
	}
}

// CQRSEdgeRepositoryAdapter combines read and write adapters for complete CQRS support.
type CQRSEdgeRepositoryAdapter struct {
	repository.EdgeReader
	repository.EdgeWriter
}

// NewCQRSEdgeRepositoryAdapter creates a combined CQRS edge repository.
func NewCQRSEdgeRepositoryAdapter(edgeRepo repository.EdgeRepository) repository.CQRSEdgeRepository {
	return &CQRSEdgeRepositoryAdapter{
		EdgeReader: NewEdgeReaderAdapter(edgeRepo),
		EdgeWriter: NewEdgeWriterAdapter(edgeRepo),
	}
}

// CQRSCategoryRepositoryAdapter combines read and write adapters for complete CQRS support.
type CQRSCategoryRepositoryAdapter struct {
	repository.CategoryReader
	repository.CategoryWriter
}

// NewCQRSCategoryRepositoryAdapter creates a combined CQRS category repository.
func NewCQRSCategoryRepositoryAdapter(categoryRepo repository.CategoryRepository) repository.CQRSCategoryRepository {
	return &CQRSCategoryRepositoryAdapter{
		CategoryReader: NewCategoryReaderAdapter(categoryRepo),
		CategoryWriter: NewCategoryWriterAdapter(categoryRepo),
	}
}

// Helper functions for logging unimplemented methods
func logUnimplementedMethod(methodName, reason string) {
	log.Printf("WARN: CQRS method %s not fully implemented: %s", methodName, reason)
}