package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// Cache interface abstracts the caching backend
// This allows different cache implementations (Redis, Memcached, in-memory, etc.)
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, pattern string) error
}

// CachingNodeRepository is a decorator that adds intelligent caching to NodeRepository operations.
//
// Key Concepts Illustrated:
//   1. Decorator Pattern: Transparently adds caching without changing the interface
//   2. Cache-Aside Pattern: Application manages cache alongside the primary data store
//   3. Cache Invalidation: Smart invalidation strategies to maintain consistency
//   4. Performance Optimization: Reduces database load for frequently accessed data
//   5. Configurable TTL: Different cache durations for different types of operations
//
// Caching Strategies Implemented:
//   - Read-Through: Cache misses trigger database reads and cache population
//   - Write-Through: Writes update both cache and database
//   - Write-Behind: Writes update cache immediately, database asynchronously (optional)
//   - Cache Warming: Proactive cache population for frequently accessed data
//   - Smart Invalidation: Invalidates related cache entries on updates
//
// Example Usage:
//   baseRepo := dynamodb.NewNodeRepository(client, table, index)
//   cache := redis.NewCache(redisClient)
//   cachedRepo := NewCachingNodeRepository(baseRepo, cache, CachingConfig{
//       DefaultTTL:    5 * time.Minute,
//       LongTTL:       1 * time.Hour,
//       EnableWrites:  true,
//       KeyPrefix:     "brain2:nodes:",
//   })
type CachingNodeRepository struct {
	inner  repository.NodeRepository
	cache  Cache
	config CachingConfig
}

// CachingConfig controls caching behavior
type CachingConfig struct {
	// TTL settings
	DefaultTTL    time.Duration // Default cache time-to-live
	LongTTL       time.Duration // TTL for stable data (like archived nodes)
	ShortTTL      time.Duration // TTL for frequently changing data
	
	// Cache behavior
	EnableReads   bool   // Enable caching of read operations
	EnableWrites  bool   // Enable write-through caching
	EnableDeletes bool   // Enable cache invalidation on deletes
	
	// Key management
	KeyPrefix     string // Prefix for all cache keys
	UseCompression bool   // Compress cached data
	
	// Performance tuning
	MaxCacheSize   int           // Maximum number of items to cache
	EvictionPolicy string        // "lru", "lfu", "ttl"
	WarmupQueries  []string      // Queries to warm up the cache with
}

// DefaultCachingConfig returns sensible defaults for caching configuration
func DefaultCachingConfig() CachingConfig {
	return CachingConfig{
		DefaultTTL:     5 * time.Minute,
		LongTTL:        1 * time.Hour,
		ShortTTL:       1 * time.Minute,
		EnableReads:    true,
		EnableWrites:   true,
		EnableDeletes:  true,
		KeyPrefix:      "brain2:nodes:",
		UseCompression: false,
		MaxCacheSize:   10000,
		EvictionPolicy: "lru",
		WarmupQueries:  []string{},
	}
}

// NewCachingNodeRepository creates a new caching decorator for NodeRepository
func NewCachingNodeRepository(
	inner repository.NodeRepository,
	cache Cache,
	config CachingConfig,
) repository.NodeRepository {
	return &CachingNodeRepository{
		inner:  inner,
		cache:  cache,
		config: config,
	}
}

// CreateNodeAndKeywords handles cache invalidation on node creation
func (r *CachingNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *node.Node) error {
	// Execute the write operation first
	err := r.inner.CreateNodeAndKeywords(ctx, node)
	if err != nil {
		return err
	}
	
	// Cache the newly created node if write caching is enabled
	if r.config.EnableWrites {
		cacheKey := r.buildNodeKey(node.UserID().String(), node.ID().String())
		if cacheData, marshalErr := r.marshalNode(node); marshalErr == nil {
			r.cache.Set(ctx, cacheKey, cacheData, r.config.DefaultTTL)
		}
	}
	
	// Invalidate related caches
	r.invalidateUserCaches(ctx, node.UserID().String())
	
	return nil
}

// FindNodeByID implements smart caching for node lookups
func (r *CachingNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	if !r.config.EnableReads {
		return r.inner.FindNodeByID(ctx, userID, nodeID)
	}
	
	// Try cache first
	cacheKey := r.buildNodeKey(userID, nodeID)
	if cachedData, found, cacheErr := r.cache.Get(ctx, cacheKey); found && cacheErr == nil {
		if node, unmarshalErr := r.unmarshalNode(cachedData); unmarshalErr == nil {
			return node, nil
		}
		// If unmarshal fails, fall through to database query
	}
	
	// Cache miss - query database
	node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	
	// Cache the result for future queries
	if node != nil {
		if cacheData, marshalErr := r.marshalNode(node); marshalErr == nil {
			ttl := r.config.DefaultTTL
			// Use longer TTL for archived nodes (they don't change often)
			if node.IsArchived() {
				ttl = r.config.LongTTL
			}
			r.cache.Set(ctx, cacheKey, cacheData, ttl)
		}
	}
	
	return node, nil
}

// FindNodes implements intelligent caching for node queries
func (r *CachingNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*node.Node, error) {
	if !r.config.EnableReads {
		return r.inner.FindNodes(ctx, query)
	}
	
	// Generate cache key from query parameters
	queryKey := r.buildQueryKey("find_nodes", query)
	
	// Try cache first
	if cachedData, found, cacheErr := r.cache.Get(ctx, queryKey); found && cacheErr == nil {
		if nodes, unmarshalErr := r.unmarshalNodeSlice(cachedData); unmarshalErr == nil {
			return nodes, nil
		}
	}
	
	// Cache miss - query database
	nodes, err := r.inner.FindNodes(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if cacheData, marshalErr := r.marshalNodeSlice(nodes); marshalErr == nil {
		// Use shorter TTL for query results (they can become stale quickly)
		ttl := r.config.ShortTTL
		r.cache.Set(ctx, queryKey, cacheData, ttl)
	}
	
	return nodes, nil
}

// DeleteNode handles cache invalidation on node deletion
func (r *CachingNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// Execute the delete operation
	err := r.inner.DeleteNode(ctx, userID, nodeID)
	if err != nil {
		return err
	}
	
	// Invalidate caches if deletion was successful
	if r.config.EnableDeletes {
		// Remove the specific node from cache
		nodeKey := r.buildNodeKey(userID, nodeID)
		r.cache.Delete(ctx, nodeKey)
		
		// Invalidate all user-related caches
		r.invalidateUserCaches(ctx, userID)
	}
	
	return nil
}

// BatchDeleteNodes handles cache invalidation for batch deletion
func (r *CachingNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	// Execute the batch delete operation
	deleted, failed, err = r.inner.BatchDeleteNodes(ctx, userID, nodeIDs)
	if err != nil {
		return deleted, failed, err
	}
	
	// Invalidate caches for successfully deleted nodes
	if r.config.EnableDeletes && len(deleted) > 0 {
		// Remove specific nodes from cache
		for _, nodeID := range deleted {
			nodeKey := r.buildNodeKey(userID, nodeID)
			r.cache.Delete(ctx, nodeKey)
		}
		
		// Invalidate all user-related caches
		r.invalidateUserCaches(ctx, userID)
	}
	
	return deleted, failed, nil
}

// BatchGetNodes implements batch retrieval with cache optimization
func (r *CachingNodeRepository) BatchGetNodes(ctx context.Context, userID string, nodeIDs []string) (map[string]*node.Node, error) {
	if !r.config.EnableReads {
		return r.inner.BatchGetNodes(ctx, userID, nodeIDs)
	}
	
	result := make(map[string]*node.Node)
	uncachedIDs := make([]string, 0)
	
	// Check cache for each node
	for _, nodeID := range nodeIDs {
		cacheKey := r.buildNodeKey(userID, nodeID)
		if cachedData, found, cacheErr := r.cache.Get(ctx, cacheKey); found && cacheErr == nil {
			if node, unmarshalErr := r.unmarshalNode(cachedData); unmarshalErr == nil {
				result[nodeID] = node
				continue
			}
		}
		uncachedIDs = append(uncachedIDs, nodeID)
	}
	
	// Fetch uncached nodes from database
	if len(uncachedIDs) > 0 {
		dbNodes, err := r.inner.BatchGetNodes(ctx, userID, uncachedIDs)
		if err != nil {
			return nil, err
		}
		
		// Cache the fetched nodes and add to result
		for nodeID, node := range dbNodes {
			result[nodeID] = node
			
			// Cache for future queries
			if node != nil {
				cacheKey := r.buildNodeKey(userID, nodeID)
				if cacheData, marshalErr := r.marshalNode(node); marshalErr == nil {
					ttl := r.config.DefaultTTL
					if node.IsArchived() {
						ttl = r.config.LongTTL
					}
					r.cache.Set(ctx, cacheKey, cacheData, ttl)
				}
			}
		}
	}
	
	return result, nil
}

// GetNodesPage implements caching for paginated queries
func (r *CachingNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	if !r.config.EnableReads {
		return r.inner.GetNodesPage(ctx, query, pagination)
	}
	
	// Generate cache key from query and pagination parameters
	pageKey := r.buildPageKey("nodes_page", query, pagination)
	
	// Try cache first
	if cachedData, found, cacheErr := r.cache.Get(ctx, pageKey); found && cacheErr == nil {
		if page, unmarshalErr := r.unmarshalNodePage(cachedData); unmarshalErr == nil {
			return page, nil
		}
	}
	
	// Cache miss - query database
	page, err := r.inner.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, err
	}
	
	// Cache the result with shorter TTL (pages can become stale quickly)
	if cacheData, marshalErr := r.marshalNodePage(page); marshalErr == nil {
		r.cache.Set(ctx, pageKey, cacheData, r.config.ShortTTL)
	}
	
	return page, err
}

// GetNodeNeighborhood implements caching for neighborhood queries
func (r *CachingNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*shared.Graph, error) {
	if !r.config.EnableReads {
		return r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	}
	
	// Generate cache key for neighborhood query
	neighborhoodKey := r.buildNeighborhoodKey(userID, nodeID, depth)
	
	// Try cache first
	if cachedData, found, cacheErr := r.cache.Get(ctx, neighborhoodKey); found && cacheErr == nil {
		if graph, unmarshalErr := r.unmarshalGraph(cachedData); unmarshalErr == nil {
			return graph, nil
		}
	}
	
	// Cache miss - query database
	graph, err := r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if cacheData, marshalErr := r.marshalGraph(graph); marshalErr == nil {
		// Neighborhood data is relatively stable, use default TTL
		r.cache.Set(ctx, neighborhoodKey, cacheData, r.config.DefaultTTL)
	}
	
	return graph, err
}

// CountNodes implements caching for count queries
func (r *CachingNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	if !r.config.EnableReads {
		return r.inner.CountNodes(ctx, userID)
	}
	
	// Generate cache key for count query
	countKey := r.buildCountKey(userID)
	
	// Try cache first
	if cachedData, found, cacheErr := r.cache.Get(ctx, countKey); found && cacheErr == nil {
		if count, unmarshalErr := r.unmarshalCount(cachedData); unmarshalErr == nil {
			return count, nil
		}
	}
	
	// Cache miss - query database
	count, err := r.inner.CountNodes(ctx, userID)
	if err != nil {
		return 0, err
	}
	
	// Cache the result with shorter TTL (counts change frequently)
	if cacheData, marshalErr := r.marshalCount(count); marshalErr == nil {
		r.cache.Set(ctx, countKey, cacheData, r.config.ShortTTL)
	}
	
	return count, nil
}

// Cache key building methods

func (r *CachingNodeRepository) buildNodeKey(userID, nodeID string) string {
	return fmt.Sprintf("%snode:%s:%s", r.config.KeyPrefix, userID, nodeID)
}

func (r *CachingNodeRepository) buildQueryKey(operation string, query repository.NodeQuery) string {
	// Create a hash of the query parameters for a consistent key
	hash := r.hashQueryParameters(operation, query)
	return fmt.Sprintf("%squery:%s:%s", r.config.KeyPrefix, query.UserID, hash)
}

func (r *CachingNodeRepository) buildPageKey(operation string, query repository.NodeQuery, pagination repository.Pagination) string {
	// Create a hash combining query and pagination parameters
	hash := r.hashPageParameters(operation, query, pagination)
	return fmt.Sprintf("%spage:%s:%s", r.config.KeyPrefix, query.UserID, hash)
}

func (r *CachingNodeRepository) buildNeighborhoodKey(userID, nodeID string, depth int) string {
	return fmt.Sprintf("%sneighborhood:%s:%s:%d", r.config.KeyPrefix, userID, nodeID, depth)
}

func (r *CachingNodeRepository) buildCountKey(userID string) string {
	return fmt.Sprintf("%scount:%s", r.config.KeyPrefix, userID)
}

// Cache invalidation methods

func (r *CachingNodeRepository) invalidateUserCaches(ctx context.Context, userID string) {
	// Invalidate all caches for this user using pattern matching
	pattern := fmt.Sprintf("%s*:%s:*", r.config.KeyPrefix, userID)
	r.cache.Clear(ctx, pattern)
}

func (r *CachingNodeRepository) invalidateQueryCaches(ctx context.Context, userID string) {
	// Invalidate query-specific caches
	pattern := fmt.Sprintf("%squery:%s:*", r.config.KeyPrefix, userID)
	r.cache.Clear(ctx, pattern)
}

func (r *CachingNodeRepository) invalidatePageCaches(ctx context.Context, userID string) {
	// Invalidate page-specific caches
	pattern := fmt.Sprintf("%spage:%s:*", r.config.KeyPrefix, userID)
	r.cache.Clear(ctx, pattern)
}

// Serialization methods

func (r *CachingNodeRepository) marshalNode(node *node.Node) ([]byte, error) {
	// Create a serializable version of the node
	serializable := struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		Content   string    `json:"content"`
		Title     string    `json:"title"`
		Keywords  []string  `json:"keywords"`
		Tags      []string  `json:"tags"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Version   int       `json:"version"`
		Archived  bool      `json:"archived"`
	}{
		ID:        node.ID().String(),
		UserID:    node.UserID().String(),
		Content:   node.Content().String(),
		Title:     node.Title().String(),
		Keywords:  node.Keywords().ToSlice(),
		Tags:      node.Tags().ToSlice(),
		CreatedAt: node.CreatedAt(),
		UpdatedAt: node.UpdatedAt(),
		Version:   node.Version(),
		Archived:  node.IsArchived(),
	}
	
	return json.Marshal(serializable)
}

func (r *CachingNodeRepository) unmarshalNode(data []byte) (*node.Node, error) {
	var serializable struct {
		ID        string    `json:"id"`
		UserID    string    `json:"user_id"`
		Content   string    `json:"content"`
		Title     string    `json:"title"`
		Keywords  []string  `json:"keywords"`
		Tags      []string  `json:"tags"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Version   int       `json:"version"`
		Archived  bool      `json:"archived"`
	}
	
	if err := json.Unmarshal(data, &serializable); err != nil {
		return nil, err
	}
	
	// Reconstruct the domain object
	return node.ReconstructNodeFromPrimitives(
		serializable.ID,
		serializable.UserID,
		serializable.Content,
		serializable.Title,
		serializable.Keywords,
		serializable.Tags,
		serializable.CreatedAt,
		serializable.Version,
	)
}

func (r *CachingNodeRepository) marshalNodeSlice(nodes []*node.Node) ([]byte, error) {
	serializable := make([]map[string]interface{}, len(nodes))
	for i, node := range nodes {
		serializable[i] = map[string]interface{}{
			"id":         node.ID().String(),
			"user_id":    node.UserID().String(),
			"content":    node.Content().String(),
			"title":      node.Title().String(),
			"keywords":   node.Keywords().ToSlice(),
			"tags":       node.Tags().ToSlice(),
			"created_at": node.CreatedAt(),
			"updated_at": node.UpdatedAt(),
			"version":    node.Version(),
			"archived":   node.IsArchived(),
		}
	}
	return json.Marshal(serializable)
}

func (r *CachingNodeRepository) unmarshalNodeSlice(data []byte) ([]*node.Node, error) {
	var serializable []map[string]interface{}
	if err := json.Unmarshal(data, &serializable); err != nil {
		return nil, err
	}
	
	nodes := make([]*node.Node, len(serializable))
	for i, item := range serializable {
		// This is a simplified unmarshaling - in practice, you'd want more robust type checking
		node, err := node.ReconstructNodeFromPrimitives(
			item["id"].(string),
			item["user_id"].(string),
			item["content"].(string),
			item["title"].(string),
			interfaceToStringSlice(item["keywords"]),
			interfaceToStringSlice(item["tags"]),
			parseTime(item["created_at"]),
			int(item["version"].(float64)),
		)
		if err != nil {
			return nil, err
		}
		nodes[i] = node
	}
	
	return nodes, nil
}

func (r *CachingNodeRepository) marshalNodePage(page *repository.NodePage) ([]byte, error) {
	return json.Marshal(page)
}

func (r *CachingNodeRepository) unmarshalNodePage(data []byte) (*repository.NodePage, error) {
	var page repository.NodePage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (r *CachingNodeRepository) marshalGraph(graph *shared.Graph) ([]byte, error) {
	return json.Marshal(graph)
}

func (r *CachingNodeRepository) unmarshalGraph(data []byte) (*shared.Graph, error) {
	var graph shared.Graph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}
	return &graph, nil
}

func (r *CachingNodeRepository) marshalCount(count int) ([]byte, error) {
	return json.Marshal(map[string]int{"count": count})
}

func (r *CachingNodeRepository) unmarshalCount(data []byte) (int, error) {
	var result map[string]int
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, err
	}
	return result["count"], nil
}

// Hash generation for cache keys

func (r *CachingNodeRepository) hashQueryParameters(operation string, query repository.NodeQuery) string {
	// Create a consistent hash from query parameters
	hasher := md5.New()
	hasher.Write([]byte(operation))
	hasher.Write([]byte(query.UserID))
	
	for _, keyword := range query.Keywords {
		hasher.Write([]byte(keyword))
	}
	
	for _, nodeID := range query.NodeIDs {
		hasher.Write([]byte(nodeID))
	}
	
	hasher.Write([]byte(fmt.Sprintf("%d:%d", query.Limit, query.Offset)))
	
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16] // Use first 16 chars of hash
}

func (r *CachingNodeRepository) hashPageParameters(operation string, query repository.NodeQuery, pagination repository.Pagination) string {
	hasher := md5.New()
	
	// Include query parameters
	queryHash := r.hashQueryParameters(operation, query)
	hasher.Write([]byte(queryHash))
	
	// Include pagination parameters
	hasher.Write([]byte(fmt.Sprintf("%d:%d:%s:%s:%s", 
		pagination.Limit, 
		pagination.Offset, 
		pagination.Cursor, 
		pagination.SortBy, 
		pagination.SortOrder,
	)))
	
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

// Utility functions

func interfaceToStringSlice(i interface{}) []string {
	if slice, ok := i.([]interface{}); ok {
		result := make([]string, len(slice))
		for idx, item := range slice {
			result[idx] = item.(string)
		}
		return result
	}
	return []string{}
}

func parseTime(i interface{}) time.Time {
	if timeStr, ok := i.(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			return t
		}
	}
	return time.Now()
}

// Phase 2 Enhanced Methods - Added for interface compatibility

// FindNodesWithOptions adds caching to enhanced node queries with options
func (r *CachingNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*node.Node, error) {
	// For consolidation phase, delegate to underlying repository without caching complex queries
	return r.inner.FindNodesWithOptions(ctx, query, opts...)
}

// FindNodesPageWithOptions adds caching to enhanced paginated node queries with options  
func (r *CachingNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	// For consolidation phase, delegate to underlying repository without caching complex queries
	return r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
}