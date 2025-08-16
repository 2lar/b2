// Package di provides reader adapters for CQRS query services
package di

import (
	"context"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// NodeReaderAdapter adapts NodeRepository to NodeReader interface
type NodeReaderAdapter struct {
	repo repository.NodeRepository
}

// NewNodeReaderAdapter creates a new adapter
func NewNodeReaderAdapter(repo repository.NodeRepository) repository.NodeReader {
	return &NodeReaderAdapter{repo: repo}
}

// FindByID implements NodeReader
func (a *NodeReaderAdapter) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Use empty userID as a workaround - might need refinement
	return a.repo.FindNodeByID(ctx, "", id.String())
}

// Exists implements NodeReader
func (a *NodeReaderAdapter) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	node, err := a.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	return node != nil, nil
}

// FindByUser implements NodeReader
func (a *NodeReaderAdapter) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	query := repository.NodeQuery{
		UserID: userID.String(),
	}
	
	// Apply options - simplified for now
	// We'll use default pagination
	pagination := repository.Pagination{
		Limit:  100,
		Offset: 0,
	}
	
	page, err := a.repo.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, err
	}
	
	return page.Items, nil
}

// CountByUser implements NodeReader
func (a *NodeReaderAdapter) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	return a.repo.CountNodes(ctx, userID.String())
}

// FindByKeywords implements NodeReader
func (a *NodeReaderAdapter) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need to be implemented using the KeywordRepository
	// For now, return an empty result
	return []*domain.Node{}, nil
}

// FindByTags implements NodeReader
func (a *NodeReaderAdapter) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need specific implementation
	return []*domain.Node{}, nil
}

// FindByContent implements NodeReader
func (a *NodeReaderAdapter) FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need specific implementation
	return []*domain.Node{}, nil
}

// FindRecentlyCreated implements NodeReader
func (a *NodeReaderAdapter) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need specific implementation
	return []*domain.Node{}, nil
}

// FindRecentlyUpdated implements NodeReader
func (a *NodeReaderAdapter) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need specific implementation
	return []*domain.Node{}, nil
}

// FindBySpecification implements NodeReader
func (a *NodeReaderAdapter) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need specification pattern implementation
	return []*domain.Node{}, nil
}

// CountBySpecification implements NodeReader
func (a *NodeReaderAdapter) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	// This would need specification pattern implementation
	return 0, nil
}

// FindPage implements NodeReader
func (a *NodeReaderAdapter) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return a.repo.GetNodesPage(ctx, query, pagination)
}

// FindConnected implements NodeReader
func (a *NodeReaderAdapter) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need graph traversal implementation
	return []*domain.Node{}, nil
}

// FindSimilar implements NodeReader
func (a *NodeReaderAdapter) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// This would need similarity calculation implementation
	return []*domain.Node{}, nil
}

// GetNodesPage implements NodeReader
func (a *NodeReaderAdapter) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	return a.repo.GetNodesPage(ctx, query, pagination)
}

// CountNodes implements NodeReader
func (a *NodeReaderAdapter) CountNodes(ctx context.Context, userID string) (int, error) {
	return a.repo.CountNodes(ctx, userID)
}

// EdgeReaderAdapter adapts EdgeRepository to EdgeReader interface
type EdgeReaderAdapter struct {
	repo repository.EdgeRepository
}

// NewEdgeReaderAdapter creates a new adapter
func NewEdgeReaderAdapter(repo repository.EdgeRepository) repository.EdgeReader {
	return &EdgeReaderAdapter{repo: repo}
}

// FindByID implements EdgeReader
func (a *EdgeReaderAdapter) FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error) {
	// EdgeRepository doesn't have FindByID, so we need to work around this
	// This is a limitation of the current repository interface
	return nil, nil
}

// Exists implements EdgeReader
func (a *EdgeReaderAdapter) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	// EdgeRepository doesn't have this method
	return false, nil
}

// FindByUser implements EdgeReader
func (a *EdgeReaderAdapter) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{
		UserID: userID.String(),
	}
	
	edges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	return edges, nil
}

// CountByUser implements EdgeReader
func (a *EdgeReaderAdapter) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	// This would need to be implemented
	return 0, nil
}

// FindBySourceNode implements EdgeReader
func (a *EdgeReaderAdapter) FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{
		SourceID: sourceID.String(),
	}
	
	edges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	return edges, nil
}

// FindByTargetNode implements EdgeReader
func (a *EdgeReaderAdapter) FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{
		TargetID: targetID.String(),
	}
	
	edges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	return edges, nil
}

// FindByNode implements EdgeReader
func (a *EdgeReaderAdapter) FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Find edges where the node is either source or target
	query := repository.EdgeQuery{
		SourceID: nodeID.String(),
	}
	
	edges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// Also find edges where node is target
	query.SourceID = ""
	query.TargetID = nodeID.String()
	
	targetEdges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	edges = append(edges, targetEdges...)
	return edges, nil
}

// FindBetweenNodes implements EdgeReader
func (a *EdgeReaderAdapter) FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error) {
	query := repository.EdgeQuery{
		SourceID: node1ID.String(),
		TargetID: node2ID.String(),
	}
	
	edges, err := a.repo.FindEdges(ctx, query)
	if err != nil {
		return nil, err
	}
	
	return edges, nil
}

// FindStrongConnections implements EdgeReader
func (a *EdgeReaderAdapter) FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// This would need specific implementation
	return []*domain.Edge{}, nil
}

// FindWeakConnections implements EdgeReader
func (a *EdgeReaderAdapter) FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// This would need specific implementation
	return []*domain.Edge{}, nil
}

// FindPage implements EdgeReader
func (a *EdgeReaderAdapter) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	return a.repo.GetEdgesPage(ctx, query, pagination)
}

// FindEdges implements EdgeReader
func (a *EdgeReaderAdapter) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	return a.repo.FindEdges(ctx, query)
}

// FindBySpecification implements EdgeReader
func (a *EdgeReaderAdapter) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// This would need specification pattern implementation
	return []*domain.Edge{}, nil
}

// CountBySpecification implements EdgeReader
func (a *EdgeReaderAdapter) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	// This would need specification pattern implementation
	return 0, nil
}

// CountBySourceID implements EdgeReader
func (a *EdgeReaderAdapter) CountBySourceID(ctx context.Context, sourceID domain.NodeID) (int, error) {
	// This would need to be implemented
	return 0, nil
}