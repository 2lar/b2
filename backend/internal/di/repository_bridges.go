// Package di provides repository bridge implementations.
// These bridges allow existing repositories to be used where CQRS interfaces are expected.
package di

import (
	"context"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	sharedContext "brain2-backend/internal/context"
)

// getUserIDFromContext extracts userID from context using the shared context key
func getUserIDFromContext(ctx context.Context) (string, bool) {
	return sharedContext.GetUserIDFromContext(ctx)
}

// NodeReaderBridge bridges NodeRepository to NodeReader interface
type NodeReaderBridge struct {
	repo   repository.NodeRepository
	userID string // Default userID for operations that need it
}

// NewNodeReaderBridge creates a bridge from NodeRepository to NodeReader
// Note: Since the underlying repository requires userID but the reader interface doesn't,
// this bridge will need to be enhanced with proper user context extraction.
func NewNodeReaderBridge(repo repository.NodeRepository) repository.NodeReader {
	return &NodeReaderBridge{
		repo: repo,
		// TODO: Extract userID from context or request in production
	}
}

// Implement minimal NodeReader interface to satisfy compilation
// These are stub implementations that allow the code to compile

func (b *NodeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Extract userID from context - this is required for DynamoDB queries
	userID, ok := getUserIDFromContext(ctx)
	if !ok {
		return nil, repository.ErrNodeNotFound // Without userID, we cannot find the node
	}
	
	node, err := b.repo.FindNodeByID(ctx, userID, id.String())
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, repository.ErrNodeNotFound
	}
	return node, nil
}

func (b *NodeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	node, err := b.FindByID(ctx, id)
	return node != nil, err
}

func (b *NodeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String()}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	return b.repo.CountNodes(ctx, userID.String())
}

func (b *NodeReaderBridge) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String(), Keywords: keywords}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String(), Tags: tags}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String(), SearchText: searchTerm}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String()}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{UserID: userID.String()}
	return b.repo.FindNodes(ctx, query)
}

func (b *NodeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement specification pattern properly
	// For now, return empty result instead of nil to avoid nil pointer issues
	return []*domain.Node{}, nil
}

func (b *NodeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

func (b *NodeReaderBridge) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return b.repo.GetNodesPage(ctx, query, pagination)
}

func (b *NodeReaderBridge) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// First, get the node to determine its userID
	node, err := b.FindByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, repository.ErrNodeNotFound
	}
	
	// Use the node's userID for the neighborhood query
	graph, err := b.repo.GetNodeNeighborhood(ctx, node.UserID.String(), nodeID.String(), depth)
	if err != nil {
		return nil, err
	}
	return graph.Nodes, nil
}

func (b *NodeReaderBridge) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement similarity search
	// For now, return empty result instead of nil to avoid nil pointer issues
	return []*domain.Node{}, nil
}

func (b *NodeReaderBridge) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return b.repo.GetNodesPage(ctx, query, pagination)
}

func (b *NodeReaderBridge) CountNodes(ctx context.Context, userID string) (int, error) {
	return b.repo.CountNodes(ctx, userID)
}

// EdgeReaderBridge bridges EdgeRepository to EdgeReader interface
type EdgeReaderBridge struct {
	repo repository.EdgeRepository
}

// NewEdgeReaderBridge creates a bridge from EdgeRepository to EdgeReader
func NewEdgeReaderBridge(repo repository.EdgeRepository) repository.EdgeReader {
	return &EdgeReaderBridge{repo: repo}
}

// Implement minimal EdgeReader interface
func (b *EdgeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error) {
	// EdgeRepository doesn't have a direct FindByID, so we need to search
	// This is inefficient and should be improved in production
	query := repository.EdgeQuery{}
	edges, err := b.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	for _, edge := range edges {
		if edge.ID.String() == id.String() {
			return edge, nil
		}
	}
	return nil, repository.ErrEdgeNotFound
}

func (b *EdgeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	edge, err := b.FindByID(ctx, id)
	return edge != nil, err
}

func (b *EdgeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{UserID: userID.String()}
	return b.repo.FindEdges(ctx, query)
}

func (b *EdgeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	edges, err := b.FindByUser(ctx, userID)
	return len(edges), err
}

func (b *EdgeReaderBridge) FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{SourceID: sourceID.String()}
	return b.repo.FindEdges(ctx, query)
}

func (b *EdgeReaderBridge) FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{TargetID: targetID.String()}
	return b.repo.FindEdges(ctx, query)
}

func (b *EdgeReaderBridge) FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	source, _ := b.FindBySourceNode(ctx, nodeID, opts...)
	target, _ := b.FindByTargetNode(ctx, nodeID, opts...)
	
	// Combine and deduplicate
	edgeMap := make(map[string]*domain.Edge)
	for _, e := range source {
		edgeMap[e.ID.String()] = e
	}
	for _, e := range target {
		edgeMap[e.ID.String()] = e
	}
	
	result := make([]*domain.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		result = append(result, e)
	}
	return result, nil
}

func (b *EdgeReaderBridge) FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error) {
	query1 := repository.EdgeQuery{SourceID: node1ID.String(), TargetID: node2ID.String()}
	edges1, _ := b.repo.FindEdges(ctx, query1)
	query2 := repository.EdgeQuery{SourceID: node2ID.String(), TargetID: node1ID.String()}
	edges2, _ := b.repo.FindEdges(ctx, query2)
	return append(edges1, edges2...), nil
}

func (b *EdgeReaderBridge) FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Edge, 0)
	for _, e := range edges {
		if e.Weight() >= threshold {
			result = append(result, e)
		}
	}
	return result, nil
}

func (b *EdgeReaderBridge) FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Edge, 0)
	for _, e := range edges {
		if e.Weight() < threshold {
			result = append(result, e)
		}
	}
	return result, nil
}

func (b *EdgeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return []*domain.Edge{}, nil
}

func (b *EdgeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

func (b *EdgeReaderBridge) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return b.repo.GetEdgesPage(ctx, query, pagination)
}

func (b *EdgeReaderBridge) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	return b.repo.FindEdges(ctx, query)
}

func (b *EdgeReaderBridge) CountBySourceID(ctx context.Context, sourceID domain.NodeID) (int, error) {
	edges, err := b.FindBySourceNode(ctx, sourceID)
	return len(edges), err
}

// CategoryReaderBridge bridges CategoryRepository to CategoryReader interface
type CategoryReaderBridge struct {
	repo repository.CategoryRepository
}

// NewCategoryReaderBridge creates a bridge from CategoryRepository to CategoryReader
func NewCategoryReaderBridge(repo repository.CategoryRepository) repository.CategoryReader {
	return &CategoryReaderBridge{repo: repo}
}

// Implement minimal CategoryReader interface
func (b *CategoryReaderBridge) FindByID(ctx context.Context, userID string, categoryID string) (*domain.Category, error) {
	return b.repo.FindByID(ctx, userID, categoryID)
}

func (b *CategoryReaderBridge) Exists(ctx context.Context, userID string, categoryID string) (bool, error) {
	cat, err := b.FindByID(ctx, userID, categoryID)
	return cat != nil, err
}

func (b *CategoryReaderBridge) FindByUser(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	query := repository.CategoryQuery{UserID: userID}
	return b.repo.FindCategories(ctx, query)
}

func (b *CategoryReaderBridge) CountByUser(ctx context.Context, userID string) (int, error) {
	cats, err := b.FindByUser(ctx, userID)
	return len(cats), err
}

func (b *CategoryReaderBridge) FindRootCategories(ctx context.Context, userID string, opts ...repository.QueryOption) ([]domain.Category, error) {
	return b.repo.FindCategoriesByLevel(ctx, userID, 0)
}

func (b *CategoryReaderBridge) FindChildCategories(ctx context.Context, userID string, parentID string) ([]domain.Category, error) {
	return b.repo.FindChildCategories(ctx, userID, parentID)
}

func (b *CategoryReaderBridge) FindCategoryPath(ctx context.Context, userID string, categoryID string) ([]domain.Category, error) {
	// Stub
	return []domain.Category{}, nil
}

func (b *CategoryReaderBridge) FindCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	return b.repo.GetCategoryTree(ctx, userID)
}

func (b *CategoryReaderBridge) FindByLevel(ctx context.Context, userID string, level int, opts ...repository.QueryOption) ([]domain.Category, error) {
	return b.repo.FindCategoriesByLevel(ctx, userID, level)
}

func (b *CategoryReaderBridge) FindMostActive(ctx context.Context, userID string, limit int) ([]domain.Category, error) {
	// Stub
	return []domain.Category{}, nil
}

func (b *CategoryReaderBridge) FindRecentlyUsed(ctx context.Context, userID string, days int, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Stub
	return []domain.Category{}, nil
}

func (b *CategoryReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]domain.Category, error) {
	// Stub
	return []domain.Category{}, nil
}

func (b *CategoryReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	return 0, nil
}

func (b *CategoryReaderBridge) FindPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	cats, err := b.repo.FindCategories(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// Simple pagination
	start := 0
	end := len(cats)
	if pagination.Limit > 0 && pagination.Limit < end {
		end = pagination.Limit
	}
	
	return &repository.CategoryPage{
		Items: cats[start:end],
		HasMore: end < len(cats),
	}, nil
}

func (b *CategoryReaderBridge) GetCategoriesPage(ctx context.Context, query repository.CategoryQuery, pagination repository.Pagination) (*repository.CategoryPage, error) {
	return b.FindPage(ctx, query, pagination)
}

func (b *CategoryReaderBridge) CountCategories(ctx context.Context, userID string) (int, error) {
	return b.CountByUser(ctx, userID)
}

// Additional methods needed by CategoryReader but not in the interface yet
func (b *CategoryReaderBridge) FindByTitle(ctx context.Context, userID string, title string) (*domain.Category, error) {
	cats, err := b.FindByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range cats {
		if cats[i].Title == title {
			return &cats[i], nil
		}
	}
	return nil, nil
}

func (b *CategoryReaderBridge) FindByParent(ctx context.Context, userID string, parentID string, opts ...repository.QueryOption) ([]*domain.Category, error) {
	cats, err := b.repo.FindChildCategories(ctx, userID, parentID)
	if err != nil {
		return nil, err
	}
	// Convert to pointer slice
	result := make([]*domain.Category, len(cats))
	for i := range cats {
		result[i] = &cats[i]
	}
	return result, nil
}