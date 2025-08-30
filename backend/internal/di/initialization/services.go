package initialization

import (
	"context"
	"log"
	"time"

	"brain2-backend/internal/application/queries"
	"brain2-backend/internal/application/services"
	"brain2-backend/internal/config"
	"brain2-backend/internal/domain/shared"

	"go.uber.org/zap"
)

// ServiceConfig holds configuration for service layer initialization
type ServiceConfig struct {
	Config          *config.Config
	EventBus        shared.EventBus
	Logger          *zap.Logger
	EnableCaching   bool
}

// ApplicationServices holds all initialized application services
type ApplicationServices struct {
	NodeAppService      *services.NodeService
	NodeQueryService    *queries.NodeQueryService
	CategoryAppService  *services.CategoryService
	CategoryQueryService *queries.CategoryQueryService
	GraphQueryService   *queries.GraphQueryService
	CleanupService      *services.CleanupService
	QueryCache          queries.Cache
}

// InitializeApplicationServices sets up application layer services
func InitializeApplicationServices(config ServiceConfig, repos *RepositoryServices) *ApplicationServices {
	log.Println("Initializing application services...")
	startTime := time.Now()

	// Initialize cache if enabled
	var queryCache queries.Cache
	if config.EnableCaching {
		queryCache = &SimpleMemoryCacheWrapper{
			cache: NewInMemoryCache(100, 5*time.Minute),
		}
	}

	appServices := &ApplicationServices{
		QueryCache: queryCache,
	}

	// Node services
	appServices.NodeAppService = services.NewNodeService(
		repos.NodeRepository,
		repos.EdgeRepository,
		repos.UnitOfWorkFactory,
		config.EventBus,
		repos.ConnectionAnalyzer,
		repos.IdempotencyStore,
	)

	// Query services with safe type assertions
	if nodeReader := repos.SafeGetNodeReader(); nodeReader != nil {
		appServices.NodeQueryService = queries.NewNodeQueryService(
			nodeReader,
			repos.SafeGetEdgeReader(),
			repos.GraphRepository,
			queryCache,
		)
	}

	if categoryReader := repos.SafeGetCategoryReader(); categoryReader != nil {
		appServices.CategoryQueryService = queries.NewCategoryQueryService(
			categoryReader,
			repos.SafeGetNodeReader(),
			config.Logger,
			queryCache,
		)
	}

	// TODO: GraphQueryService expects persistence.Store but we have repository.GraphRepository
	// appServices.GraphQueryService = queries.NewGraphQueryService(
	//	repos.GraphRepository,
	//	config.Logger,
	//	queryCache,
	// )

	// Category service
	appServices.CategoryAppService = services.NewCategoryService(
		repos.SafeGetCategoryReader(),
		repos.SafeGetCategoryWriter(),
		repos.UnitOfWorkFactory,
		config.EventBus,
		repos.IdempotencyStore,
	)

	// Cleanup service
	appServices.CleanupService = services.NewCleanupService(
		repos.NodeRepository,
		repos.EdgeRepository,
		repos.SafeGetEdgeWriter(),
		repos.IdempotencyStore,
		repos.UnitOfWorkFactory,
	)

	log.Printf("Application services initialized in %v", time.Since(startTime))
	return appServices
}

// SimpleMemoryCacheWrapper wraps the in-memory cache for queries
type SimpleMemoryCacheWrapper struct {
	cache Cache
}

func (w *SimpleMemoryCacheWrapper) Get(ctx context.Context, key string) ([]byte, bool, error) {
	data, found, err := w.cache.Get(ctx, key)
	return data, found, err
}

func (w *SimpleMemoryCacheWrapper) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return w.cache.Set(ctx, key, value, ttl)
}

func (w *SimpleMemoryCacheWrapper) Delete(ctx context.Context, key string) error {
	return nil // TODO: implement if needed
}

func (w *SimpleMemoryCacheWrapper) Clear(ctx context.Context, pattern string) error {
	return nil // TODO: implement if needed
}

// Cache interface for memory cache wrapper
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxSize int, defaultTTL time.Duration) Cache {
	// This would return the actual cache implementation
	// For now, return a placeholder that satisfies the interface
	return &memoryCache{
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
		data:       make(map[string]cacheItem),
	}
}

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

type memoryCache struct {
	maxSize    int
	defaultTTL time.Duration
	data       map[string]cacheItem
}

func (c *memoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	item, found := c.data[key]
	if !found || time.Now().After(item.expiresAt) {
		return nil, false, nil
	}
	return item.data, true, nil
}

func (c *memoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	c.data[key] = cacheItem{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}