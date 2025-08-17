# Brain2 Application Optimization Plan

## 1. Bundle Size Optimization - Code Splitting Strategy

### Current Issue
- Single large JavaScript bundle: 962.13 kB (299.84 kB gzipped)
- Warning about chunks larger than 500 kB
- Using deprecated CJS build of Vite's Node API

### Solution: Implement Code Splitting

#### Update `frontend/vite.config.ts`:

```typescript
import { defineConfig, loadEnv } from 'vite'
import { resolve } from 'path'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  
  return {
    plugins: [react()],
    root: 'src',
    build: {
      outDir: '../dist',
      // Add rollup options for code splitting
      rollupOptions: {
        output: {
          // Manual chunks for vendor libraries
          manualChunks: {
            // React ecosystem
            'react-vendor': ['react', 'react-dom', 'react-router-dom'],
            // State management and data fetching
            'state-vendor': ['zustand', '@tanstack/react-query'],
            // Visualization libraries (the heaviest)
            'graph-vendor': ['cytoscape', 'cytoscape-cola'],
            // Utilities
            'utils-vendor': ['lodash-es'],
            // Authentication
            'auth-vendor': ['@supabase/supabase-js']
          },
          // Dynamic imports for features
          chunkFileNames: (chunkInfo) => {
            const facadeModuleId = chunkInfo.facadeModuleId ? chunkInfo.facadeModuleId.split('/').pop() : 'chunk'
            return `assets/[name]-${facadeModuleId}-[hash].js`
          }
        }
      },
      // Increase chunk size warning limit since we're manually chunking
      chunkSizeWarningLimit: 600,
      // Enable source maps for production debugging
      sourcemap: true,
      // Minification options
      minify: 'terser',
      terserOptions: {
        compress: {
          drop_console: true,
          drop_debugger: true
        }
      }
    },
    envDir: '../',
    resolve: {
      alias: {
        '@app': resolve(__dirname, './src/app'),
        '@common': resolve(__dirname, './src/common'),
        '@features': resolve(__dirname, './src/features'),
        '@services': resolve(__dirname, './src/services'),
        '@types': resolve(__dirname, './src/types')
      }
    },
    // Optimize dependencies
    optimizeDeps: {
      include: ['cytoscape', 'cytoscape-cola', '@tanstack/react-query'],
      exclude: ['@tanstack/react-query-devtools']
    }
  }
})
```

#### Implement Lazy Loading in Routes:

```typescript
// frontend/src/app/App.tsx
import React, { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';

// Lazy load heavy components
const Dashboard = lazy(() => import('@features/dashboard/components/Dashboard'));
const GraphVisualization = lazy(() => import('@features/memories/components/GraphVisualization'));

function LoadingFallback() {
  return <div className="loading-spinner">Loading...</div>;
}

export function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<LoadingFallback />}>
        <Routes>
          <Route path="/dashboard" element={<Dashboard />} />
          {/* Other routes */}
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
```

## 2. Graph Node Connection Synchronization Issue

### Current Issue
Nodes appear in the graph before edges are fully formed, causing disconnected nodes to appear temporarily.

### Root Cause Analysis
The current flow creates nodes and edges asynchronously through EventBridge, causing a race condition where the graph updates before connections are established.

### Solution: Implement Synchronous Edge Creation with Optimistic Updates

#### Backend Changes:

1. **Modify `CreateNodeWithEdges` to ensure atomic operations:**

```go
// backend/internal/service/memory/service.go
func (s *service) CreateNodeWithEdges(ctx context.Context, userID, content string) (*domain.Node, []domain.Edge, error) {
    // ... existing node creation logic ...
    
    // Create edges synchronously before returning
    edges := []domain.Edge{}
    for _, relatedID := range relatedNodeIDs {
        edge := domain.Edge{
            SourceID: node.ID,
            TargetID: relatedID,
            Weight:   calculateEdgeWeight(node.Keywords, relatedNodes[relatedID].Keywords),
        }
        edges = append(edges, edge)
    }
    
    // Use transaction to ensure atomicity
    if err := s.repo.CreateNodeWithEdgesAtomic(ctx, node, edges); err != nil {
        return nil, nil, err
    }
    
    // Return both node and edges for immediate UI update
    return &node, edges, nil
}
```

2. **Add WebSocket notification with complete data:**

```go
// backend/internal/handlers/memory.go
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    // ... existing validation ...
    
    node, edges, err := h.memoryService.CreateNodeWithEdges(r.Context(), userID, req.Content)
    if err != nil {
        handleServiceError(w, err)
        return
    }
    
    // Send complete graph update via WebSocket
    graphUpdate := GraphUpdate{
        Type: "nodeCreated",
        Node: node,
        Edges: edges,
        Timestamp: time.Now(),
    }
    
    h.wsService.BroadcastToUser(userID, graphUpdate)
    
    api.Success(w, http.StatusCreated, response)
}
```

#### Frontend Changes:

1. **Implement optimistic updates with rollback:**

```typescript
// frontend/src/features/memories/hooks/useCreateMemory.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';

export function useCreateMemory() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: async (content: string) => {
      return await nodesApi.createNode({ content });
    },
    // Optimistic update
    onMutate: async (content) => {
      await queryClient.cancelQueries({ queryKey: ['graph'] });
      
      const previousGraph = queryClient.getQueryData(['graph']);
      
      // Create temporary node with pending status
      const tempNode = {
        id: `temp-${Date.now()}`,
        content,
        label: content.substring(0, 50),
        isPending: true
      };
      
      queryClient.setQueryData(['graph'], (old) => ({
        ...old,
        nodes: [...old.nodes, tempNode]
      }));
      
      return { previousGraph };
    },
    // Replace with real data when response arrives
    onSuccess: (data, variables, context) => {
      queryClient.setQueryData(['graph'], (old) => {
        // Remove temporary node and add real node with edges
        const filtered = old.nodes.filter(n => !n.isPending);
        return {
          nodes: [...filtered, data.node],
          edges: [...old.edges, ...data.edges]
        };
      });
    },
    // Rollback on error
    onError: (err, variables, context) => {
      queryClient.setQueryData(['graph'], context.previousGraph);
    }
  });
}
```

2. **Update WebSocket handler to process complete updates:**

```typescript
// frontend/src/services/webSocketClient.ts
handleGraphUpdate(update: GraphUpdate) {
  if (update.type === 'nodeCreated') {
    // Update graph with both node and edges atomically
    this.graphStore.addNodeWithEdges(update.node, update.edges);
  }
}
```

## 3. Comprehensive Code Audit & Architecture Improvements

### A. Dead Code Elimination

#### Unused Files to Remove:
- `backend/internal/repository/config.go` (duplicate content)
- Remove test files marked with `//go:build ignore`
- Clean up placeholder handler functions in `backend/internal/di/http.go`

#### Unused Dependencies:
```bash
# Run in frontend/
npm prune --production

# Check for unused packages
npx depcheck
```

### B. Architecture Improvements

#### 1. **Implement Proper Dependency Injection:**

```go
// backend/internal/di/container.go
type Container struct {
    Config           *config.Config
    Repository       repository.Repository
    MemoryService    memory.Service
    CategoryService  category.Service
    WebSocketService websocket.Service
    EventBus         events.EventBus
}

func NewContainer(cfg *config.Config) (*Container, error) {
    // Initialize with proper error handling and lifecycle management
    repo := dynamodb.NewRepository(cfg.DynamoDB)
    
    return &Container{
        Config:          cfg,
        Repository:      repo,
        MemoryService:   memory.NewService(repo),
        CategoryService: category.NewService(repo),
        // ... other services
    }, nil
}
```

#### 2. **Implement Repository Pattern with Connection Pooling:**

```go
// backend/internal/repository/pool.go
type ConnectionPool struct {
    client    *dynamodb.Client
    semaphore chan struct{}
    metrics   *Metrics
}

func NewConnectionPool(size int) *ConnectionPool {
    return &ConnectionPool{
        semaphore: make(chan struct{}, size),
    }
}

func (p *ConnectionPool) Execute(ctx context.Context, fn func(*dynamodb.Client) error) error {
    select {
    case p.semaphore <- struct{}{}:
        defer func() { <-p.semaphore }()
        return fn(p.client)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

#### 3. **Add Circuit Breaker for External Services:**

```go
// backend/pkg/resilience/circuit_breaker.go
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    state        atomic.Value // "closed", "open", "half-open"
    failures     atomic.Int32
    lastFailTime atomic.Value
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state.Load() == "open" {
        if time.Since(cb.lastFailTime.Load().(time.Time)) > cb.resetTimeout {
            cb.state.Store("half-open")
        } else {
            return ErrCircuitBreakerOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.recordFailure()
    } else {
        cb.recordSuccess()
    }
    
    return err
}
```

### C. Performance Optimizations

#### 1. **Implement Caching Layer:**

```typescript
// frontend/src/common/cache/QueryCache.ts
export class QueryCache {
  private cache = new Map<string, CacheEntry>();
  private maxAge = 5 * 60 * 1000; // 5 minutes
  
  get(key: string): any | null {
    const entry = this.cache.get(key);
    if (!entry) return null;
    
    if (Date.now() - entry.timestamp > this.maxAge) {
      this.cache.delete(key);
      return null;
    }
    
    return entry.data;
  }
  
  set(key: string, data: any): void {
    this.cache.set(key, {
      data,
      timestamp: Date.now()
    });
  }
}
```

#### 2. **Add Request Batching:**

```typescript
// frontend/src/services/batchedApi.ts
class BatchedApiClient {
  private queue: Map<string, Promise<any>> = new Map();
  private batchTimeout: number = 50; // ms
  
  async batchRequest<T>(key: string, fn: () => Promise<T>): Promise<T> {
    if (this.queue.has(key)) {
      return this.queue.get(key) as Promise<T>;
    }
    
    const promise = new Promise<T>((resolve, reject) => {
      setTimeout(async () => {
        try {
          const result = await fn();
          resolve(result);
        } catch (error) {
          reject(error);
        } finally {
          this.queue.delete(key);
        }
      }, this.batchTimeout);
    });
    
    this.queue.set(key, promise);
    return promise;
  }
}
```

### D. Scalability Improvements

#### 1. **Implement Rate Limiting:**

```go
// backend/pkg/middleware/rate_limiter.go
func RateLimiter(requests int, duration time.Duration) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Every(duration/time.Duration(requests)), requests)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### 2. **Add Monitoring and Observability:**

```go
// backend/pkg/monitoring/metrics.go
type Metrics struct {
    RequestCount    *prometheus.CounterVec
    RequestDuration *prometheus.HistogramVec
    ErrorCount      *prometheus.CounterVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        RequestCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "api_requests_total",
                Help: "Total number of API requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        // ... other metrics
    }
}
```

### E. Testing Strategy

#### 1. **Add Integration Tests:**

```go
// backend/internal/service/memory/service_test.go
func TestCreateNodeWithEdges_Integration(t *testing.T) {
    // Setup test database
    repo := setupTestRepo(t)
    service := NewService(repo)
    
    // Test node creation with edges
    node, edges, err := service.CreateNodeWithEdges(ctx, "user1", "test content")
    
    assert.NoError(t, err)
    assert.NotNil(t, node)
    assert.NotEmpty(t, edges)
    
    // Verify edges are created
    graph, err := service.GetGraphData(ctx, "user1")
    assert.NoError(t, err)
    assert.Contains(t, graph.Nodes, node)
}
```

#### 2. **Add E2E Tests:**

```typescript
// frontend/tests/e2e/graph.spec.ts
describe('Graph Visualization', () => {
  it('should display newly created nodes with connections', async () => {
    await page.goto('/dashboard');
    
    // Create a new memory
    await page.fill('[data-testid="memory-input"]', 'Test memory content');
    await page.click('[data-testid="create-button"]');
    
    // Wait for graph update
    await page.waitForSelector('[data-testid="graph-node"]');
    
    // Verify node has connections
    const edges = await page.$$('[data-testid="graph-edge"]');
    expect(edges.length).toBeGreaterThan(0);
  });
});
```

## Implementation Priority

### Phase 1 (Week 1):
1. ✅ Implement code splitting in Vite config
2. ✅ Fix node connection synchronization issue
3. ✅ Remove dead code and unused dependencies

### Phase 2 (Week 2):
1. ⏳ Add dependency injection container
2. ⏳ Implement caching layer
3. ⏳ Add circuit breaker for resilience

### Phase 3 (Week 3):
1. ⏳ Add comprehensive monitoring
2. ⏳ Implement rate limiting
3. ⏳ Add integration and E2E tests

### Phase 4 (Week 4):
1. ⏳ Performance profiling and optimization
2. ⏳ Documentation updates
3. ⏳ Load testing and benchmarking

## Expected Outcomes

### Performance Improvements:
- **Bundle size**: Reduce initial load from 962KB to ~200KB (main chunk)
- **Time to Interactive**: Improve by 40-50% with code splitting
- **API Response Time**: Reduce by 30% with caching and batching

### Reliability Improvements:
- **Graph consistency**: 100% reliable node-edge synchronization
- **Error recovery**: Automatic retry with circuit breaker
- **Data integrity**: Atomic operations with proper transactions

### Maintainability Improvements:
- **Code coverage**: Increase to 80%+ with tests
- **Type safety**: 100% type coverage
- **Documentation**: Complete API and architecture docs

## Monitoring Dashboard

Set up monitoring for:
- Bundle size tracking
- Performance metrics (Core Web Vitals)
- API latency and error rates
- WebSocket connection stability
- Memory usage patterns

## Additional Recommendations

1. **Consider GraphQL**: For more efficient data fetching with complex graph queries
2. **Add Redis Cache**: For session management and temporary data
3. **Implement CDN**: Use CloudFront more effectively for static assets
4. **Add Search**: Implement Elasticsearch for full-text search capabilities
5. **Mobile App**: Consider React Native for mobile experience