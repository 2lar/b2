package queries

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/infrastructure/cqrs"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// NodeReaderBridge adapts CQRS NodeReader to work with the NodeQueryService.
// This bridge provides error handling, caching, and performance optimizations.
type NodeReaderBridge struct {
	reader repository.NodeReader
	cache  Cache
}

// NewNodeReaderBridge creates a new NodeReaderBridge with proper error handling.
func NewNodeReaderBridge(nodeRepo repository.NodeRepository, cache Cache) repository.NodeReader {
	return &NodeReaderBridge{
		reader: cqrs.NewNodeReaderAdapter(nodeRepo),
		cache:  cache,
	}
}

// FindByID retrieves a node by ID with caching and error handling.
func (b *NodeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("node_bridge:find_by_id:%s", id.String())
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if node, ok := cached.(*domain.Node); ok {
				return node, nil
			}
		}
	}

	// Delegate to CQRS reader with error handling
	node, err := b.reader.FindByID(ctx, id)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by ID")
	}

	// Cache successful result
	if node != nil && b.cache != nil {
		b.cache.Set(ctx, cacheKey, node, 5*time.Minute)
	}

	return node, nil
}

// Exists checks if a node exists with caching.
func (b *NodeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	cacheKey := fmt.Sprintf("node_bridge:exists:%s", id.String())
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if exists, ok := cached.(bool); ok {
				return exists, nil
			}
		}
	}

	exists, err := b.reader.Exists(ctx, id)
	if err != nil {
		return false, appErrors.Wrap(err, "node reader bridge failed to check existence")
	}

	// Cache result
	if b.cache != nil {
		b.cache.Set(ctx, cacheKey, exists, 2*time.Minute)
	}

	return exists, nil
}

// FindByUser retrieves nodes for a user with error handling and optional caching.
func (b *NodeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by user")
	}

	return nodes, nil
}

// CountByUser returns the count of nodes for a user with caching.
func (b *NodeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	cacheKey := fmt.Sprintf("node_bridge:count_by_user:%s", userID.String())
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if count, ok := cached.(int); ok {
				return count, nil
			}
		}
	}

	count, err := b.reader.CountByUser(ctx, userID)
	if err != nil {
		return 0, appErrors.Wrap(err, "node reader bridge failed to count by user")
	}

	// Cache count for a shorter time since it changes frequently
	if b.cache != nil {
		b.cache.Set(ctx, cacheKey, count, 1*time.Minute)
	}

	return count, nil
}

// FindByKeywords implements keyword-based search with error handling.
func (b *NodeReaderBridge) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindByKeywords(ctx, userID, keywords, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by keywords")
	}

	return nodes, nil
}

// GetNodesPage implements paginated node retrieval with error handling.
func (b *NodeReaderBridge) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	page, err := b.reader.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to get nodes page")
	}

	return page, nil
}

// CountNodes implements node counting with caching.
func (b *NodeReaderBridge) CountNodes(ctx context.Context, userID string) (int, error) {
	cacheKey := fmt.Sprintf("node_bridge:count_nodes:%s", userID)
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if count, ok := cached.(int); ok {
				return count, nil
			}
		}
	}

	count, err := b.reader.CountNodes(ctx, userID)
	if err != nil {
		return 0, appErrors.Wrap(err, "node reader bridge failed to count nodes")
	}

	// Cache count for a shorter time
	if b.cache != nil {
		b.cache.Set(ctx, cacheKey, count, 1*time.Minute)
	}

	return count, nil
}

// Delegate all other methods to the underlying reader with error wrapping
func (b *NodeReaderBridge) FindByTags(ctx context.Context, userID domain.UserID, tags []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindByTags(ctx, userID, tags, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by tags")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) FindByContent(ctx context.Context, userID domain.UserID, searchTerm string, fuzzy bool, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindByContent(ctx, userID, searchTerm, fuzzy, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by content")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) FindRecentlyCreated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindRecentlyCreated(ctx, userID, days, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find recently created")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) FindRecentlyUpdated(ctx context.Context, userID domain.UserID, days int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindRecentlyUpdated(ctx, userID, days, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find recently updated")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindBySpecification(ctx, spec, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find by specification")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	count, err := b.reader.CountBySpecification(ctx, spec)
	if err != nil {
		return 0, appErrors.Wrap(err, "node reader bridge failed to count by specification")
	}
	return count, nil
}

func (b *NodeReaderBridge) FindPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	page, err := b.reader.FindPage(ctx, query, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find page")
	}
	return page, nil
}

func (b *NodeReaderBridge) FindConnected(ctx context.Context, nodeID domain.NodeID, depth int, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindConnected(ctx, nodeID, depth, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find connected")
	}
	return nodes, nil
}

func (b *NodeReaderBridge) FindSimilar(ctx context.Context, nodeID domain.NodeID, threshold float64, opts ...repository.QueryOption) ([]*domain.Node, error) {
	nodes, err := b.reader.FindSimilar(ctx, nodeID, threshold, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "node reader bridge failed to find similar")
	}
	return nodes, nil
}

// EdgeReaderBridge adapts CQRS EdgeReader to work with the EdgeQueryService.
type EdgeReaderBridge struct {
	reader repository.EdgeReader
	cache  Cache
}

// NewEdgeReaderBridge creates a new EdgeReaderBridge with proper error handling.
func NewEdgeReaderBridge(edgeRepo repository.EdgeRepository, cache Cache) repository.EdgeReader {
	return &EdgeReaderBridge{
		reader: cqrs.NewEdgeReaderAdapter(edgeRepo),
		cache:  cache,
	}
}

// FindByID retrieves an edge by ID with error handling.
func (b *EdgeReaderBridge) FindByID(ctx context.Context, id domain.NodeID) (*domain.Edge, error) {
	edge, err := b.reader.FindByID(ctx, id)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by ID")
	}
	return edge, nil
}

// Exists checks if an edge exists.
func (b *EdgeReaderBridge) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	exists, err := b.reader.Exists(ctx, id)
	if err != nil {
		return false, appErrors.Wrap(err, "edge reader bridge failed to check existence")
	}
	return exists, nil
}

// FindByUser retrieves edges for a user with caching.
func (b *EdgeReaderBridge) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	cacheKey := fmt.Sprintf("edge_bridge:find_by_user:%s", userID.String())
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if edges, ok := cached.([]*domain.Edge); ok {
				return edges, nil
			}
		}
	}

	edges, err := b.reader.FindByUser(ctx, userID, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by user")
	}

	// Cache edges for a short time
	if b.cache != nil {
		b.cache.Set(ctx, cacheKey, edges, 2*time.Minute)
	}

	return edges, nil
}

// CountByUser returns the count of edges for a user with caching.
func (b *EdgeReaderBridge) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	cacheKey := fmt.Sprintf("edge_bridge:count_by_user:%s", userID.String())
	if b.cache != nil {
		if cached, found := b.cache.Get(ctx, cacheKey); found {
			if count, ok := cached.(int); ok {
				return count, nil
			}
		}
	}

	count, err := b.reader.CountByUser(ctx, userID)
	if err != nil {
		return 0, appErrors.Wrap(err, "edge reader bridge failed to count by user")
	}

	// Cache count
	if b.cache != nil {
		b.cache.Set(ctx, cacheKey, count, 1*time.Minute)
	}

	return count, nil
}

// FindEdges implements edge finding with error handling.
func (b *EdgeReaderBridge) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	edges, err := b.reader.FindEdges(ctx, query)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find edges")
	}
	return edges, nil
}

// FindPage implements paginated edge retrieval.
func (b *EdgeReaderBridge) FindPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	page, err := b.reader.FindPage(ctx, query, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find page")
	}
	return page, nil
}

// Delegate all other methods with error wrapping
func (b *EdgeReaderBridge) FindBySourceNode(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindBySourceNode(ctx, sourceID, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by source node")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindByTargetNode(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindByTargetNode(ctx, targetID, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by target node")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindByNode(ctx context.Context, nodeID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindByNode(ctx, nodeID, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by node")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindBetweenNodes(ctx context.Context, node1ID, node2ID domain.NodeID) ([]*domain.Edge, error) {
	edges, err := b.reader.FindBetweenNodes(ctx, node1ID, node2ID)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find between nodes")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindStrongConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindStrongConnections(ctx, userID, threshold, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find strong connections")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindWeakConnections(ctx context.Context, userID domain.UserID, threshold float64, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindWeakConnections(ctx, userID, threshold, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find weak connections")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) FindBySpecification(ctx context.Context, spec repository.Specification, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	edges, err := b.reader.FindBySpecification(ctx, spec, opts...)
	if err != nil {
		return nil, appErrors.Wrap(err, "edge reader bridge failed to find by specification")
	}
	return edges, nil
}

func (b *EdgeReaderBridge) CountBySpecification(ctx context.Context, spec repository.Specification) (int, error) {
	count, err := b.reader.CountBySpecification(ctx, spec)
	if err != nil {
		return 0, appErrors.Wrap(err, "edge reader bridge failed to count by specification")
	}
	return count, nil
}

func (b *EdgeReaderBridge) CountBySourceID(ctx context.Context, sourceID domain.NodeID) (int, error) {
	count, err := b.reader.CountBySourceID(ctx, sourceID)
	if err != nil {
		return 0, appErrors.Wrap(err, "edge reader bridge failed to count by source ID")
	}
	return count, nil
}

// CacheManager provides utility methods for managing cache invalidation across bridges.
type CacheManager struct {
	cache Cache
}

// NewCacheManager creates a new cache manager.
func NewCacheManager(cache Cache) *CacheManager {
	return &CacheManager{cache: cache}
}

// InvalidateNode invalidates all cached data related to a specific node.
func (cm *CacheManager) InvalidateNode(ctx context.Context, userID, nodeID string) {
	if cm.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("node_bridge:find_by_id:%s", nodeID),
		fmt.Sprintf("node_bridge:exists:%s", nodeID),
		fmt.Sprintf("node_bridge:count_by_user:%s", userID),
		fmt.Sprintf("node_bridge:count_nodes:%s", userID),
		fmt.Sprintf("edge_bridge:find_by_user:%s", userID),
		fmt.Sprintf("edge_bridge:count_by_user:%s", userID),
	}

	for _, pattern := range patterns {
		cm.cache.Delete(ctx, pattern)
	}
}

// InvalidateUser invalidates all cached data for a user.
func (cm *CacheManager) InvalidateUser(ctx context.Context, userID string) {
	if cm.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("node_bridge:count_by_user:%s", userID),
		fmt.Sprintf("node_bridge:count_nodes:%s", userID),
		fmt.Sprintf("edge_bridge:find_by_user:%s", userID),
		fmt.Sprintf("edge_bridge:count_by_user:%s", userID),
	}

	for _, pattern := range patterns {
		cm.cache.Delete(ctx, pattern)
	}
}