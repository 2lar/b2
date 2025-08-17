# Implementation Gaps for Production Readiness

## Overview
This document outlines the remaining implementation gaps that need to be addressed to achieve production excellence. All items listed here are practical, necessary improvements with no waste code.

**Note:** Category AI features are intentionally excluded as they are not yet designed.

---

## 🔴 Critical Gaps (Blocking Production)

### 1. Complete Repository Implementations

#### 1.1 EdgeRepository Implementation
**Location:** `internal/infrastructure/dynamodb/edge_repository.go`
**Status:** Missing
**Impact:** Core functionality incomplete

```go
// Required implementation
type EdgeRepository struct {
    client    *dynamodb.Client
    tableName string
    indexName string
}

// Must implement all methods from repository.EdgeRepository interface
```

**Tasks:**
- [ ] Implement CreateEdges method
- [ ] Implement CreateEdge method  
- [ ] Implement FindEdges method
- [ ] Implement GetEdgesPage method
- [ ] Implement FindEdgesWithOptions method
- [ ] Add proper error handling
- [ ] Add logging and metrics

#### 1.2 Fix CQRS Repository Adapters
**Location:** `internal/infrastructure/cqrs/`
**Status:** Placeholder implementations
**Impact:** CQRS pattern incomplete

**Tasks:**
- [ ] Complete NodeReaderAdapter implementation
- [ ] Complete NodeWriterAdapter implementation
- [ ] Complete EdgeReaderAdapter implementation
- [ ] Complete EdgeWriterAdapter implementation
- [ ] Remove placeholder return statements
- [ ] Add proper type conversions where needed

#### 1.3 Implement UnitOfWork
**Location:** `internal/infrastructure/dynamodb/unit_of_work.go`
**Status:** Referenced but not implemented
**Impact:** Transaction management unavailable

**Tasks:**
- [ ] Create DynamoDBUnitOfWork struct
- [ ] Implement Begin, Commit, Rollback methods
- [ ] Integrate with repository factories
- [ ] Add event publishing support
- [ ] Handle transaction rollback properly

---

## 🟡 High Priority Gaps (Needed for Robustness)

### 2. Fix Decorator Configurations

#### 2.1 Repository Decorators
**Location:** `internal/di/providers.go` (lines ~70-90)
**Status:** Commented out due to config type mismatches
**Impact:** No retry, caching, or circuit breaker functionality

**Tasks:**
- [ ] Fix RetryConfig type compatibility
- [ ] Fix CircuitBreakerConfig type compatibility  
- [ ] Fix CachingConfig type compatibility
- [ ] Uncomment and test decorator chain
- [ ] Verify decorator ordering is correct

```go
// Currently commented out - needs fixing:
// if cfg.Features.EnableRetries {
//     decorated = decorators.NewRetryNodeRepository(decorated, cfg.Infrastructure.RetryConfig)
// }
```

### 3. Complete Service Layer

#### 3.1 Category Service with AI Feature Flag
**Location:** `internal/service/category/enhanced_service.go`
**Status:** AI categorization not properly flagged
**Impact:** AI features always attempted even when disabled

**Implementation Required:**
```go
// Check feature flag before AI operations
func (s *enhancedService) CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error) {
    // Check if AI is enabled via configuration
    if !s.config.Features.EnableAIProcessing {
        // Use keyword-based categorization only
        return s.categorizeByKeywords(ctx, node, existingCategories)
    }
    
    // Existing AI logic here...
}
```

**Tasks:**
- [ ] Add config field to enhancedService struct
- [ ] Update NewEnhancedService to accept config
- [ ] Wrap all AI calls with feature flag check
- [ ] Ensure keyword fallback always works
- [ ] Update handler to check feature flag
- [ ] Add tests for both AI and non-AI paths

#### 3.2 NodeService Completion
**Location:** `internal/application/services/node_service.go`
**Status:** Partially implemented
**Impact:** Missing update and delete operations

**Tasks:**
- [ ] Implement UpdateNode method
- [ ] Implement DeleteNode method
- [ ] Implement BulkCreateNodes method
- [ ] Add proper event publishing
- [ ] Add idempotency support for all operations

#### 3.3 Query Service Adapters
**Location:** `internal/di/providers.go`
**Status:** Bridge implementations needed
**Impact:** Query services not fully functional

**Tasks:**
- [ ] Complete NodeReaderBridge implementation
- [ ] Complete EdgeReaderBridge implementation
- [ ] Add proper error handling in bridges
- [ ] Implement query caching

---

## 🟢 Medium Priority Gaps (Enhanced Functionality)

### 4. Missing Infrastructure Components

#### 4.1 Health Check Handler
**Location:** `internal/interfaces/http/handlers/health_handler.go`
**Status:** Missing
**Impact:** No health monitoring

```go
// Required implementation
type HealthHandler struct {
    nodeRepo     repository.NodeRepository
    edgeRepo     repository.EdgeRepository
    categoryRepo repository.CategoryRepository
    logger       *zap.Logger
}

// Implement:
// - Liveness probe endpoint
// - Readiness probe endpoint  
// - Dependency health checks
```

**Tasks:**
- [ ] Create HealthHandler struct
- [ ] Implement /health/live endpoint
- [ ] Implement /health/ready endpoint
- [ ] Add database connectivity check
- [ ] Add cache connectivity check (if enabled)
- [ ] Add metrics for health status

#### 4.2 Idempotency Store
**Location:** `internal/infrastructure/dynamodb/idempotency_store.go`
**Status:** Interface exists, implementation missing
**Impact:** No idempotent operation support

**Tasks:**
- [ ] Implement DynamoDB-based idempotency store
- [ ] Add TTL for idempotency keys
- [ ] Handle concurrent requests properly
- [ ] Add metrics for idempotency hits/misses

### 5. Complete Application Layer Adapters

#### 5.1 Repository Adapters
**Location:** `internal/application/adapters/`
**Status:** Some adapters incomplete
**Impact:** Service layer can't fully utilize repositories

**Tasks:**
- [ ] Complete EdgeRepositoryAdapter
- [ ] Complete GraphRepositoryAdapter  
- [ ] Fix UnitOfWorkAdapter to use real implementation
- [ ] Add proper error wrapping in all adapters

---

## 🔵 Low Priority Gaps (Nice to Have)

### 6. Documentation Gaps

#### 6.1 Architecture Decision Records
**Location:** `docs/adr/`
**Status:** Only one ADR exists
**Impact:** Architectural decisions not documented

**Tasks:**
- [ ] ADR-002: CQRS Implementation Strategy
- [ ] ADR-003: Repository Pattern with Specifications
- [ ] ADR-004: Event-Driven Architecture
- [ ] ADR-005: Error Handling Strategy
- [ ] ADR-006: Configuration Management Approach

#### 6.2 Example Tests
**Location:** `*_example_test.go` files
**Status:** Limited examples
**Impact:** Learning value reduced

**Tasks:**
- [ ] Add repository pattern examples
- [ ] Add service orchestration examples
- [ ] Add error handling examples
- [ ] Add configuration examples

---

## 📋 Implementation Order

### Week 1: Critical Infrastructure
1. **Day 1-2:** EdgeRepository implementation
2. **Day 2-3:** UnitOfWork implementation  
3. **Day 3-4:** Fix CQRS adapters
4. **Day 4-5:** Fix decorator configurations

### Week 2: Service Layer & Health
1. **Day 1-2:** Complete NodeService
2. **Day 2-3:** Health check implementation
3. **Day 3-4:** Idempotency store
4. **Day 4-5:** Complete application adapters

### Week 3: Documentation & Testing
1. **Day 1-2:** Write ADRs
2. **Day 2-3:** Add example tests
3. **Day 3-5:** Integration testing

---

## ✅ Definition of Done

Each implementation must include:
- [ ] Unit tests with >80% coverage
- [ ] Integration tests for critical paths
- [ ] Proper error handling with typed errors
- [ ] Structured logging with appropriate levels
- [ ] Metrics instrumentation where applicable
- [ ] Documentation explaining the implementation
- [ ] Example usage in tests or comments

---

## 🚫 Out of Scope

The following items are intentionally excluded:
- Category AI service implementation (not designed yet)
- External monitoring/observability tools (use existing AWS tools)
- CI/CD pipeline changes (separate effort)
- Database schema changes (current schema is sufficient)
- Authentication service changes (current implementation is adequate)

---

## 📊 Success Metrics

Implementation is complete when:
- All critical gaps are resolved
- All high priority gaps are resolved
- Unit test coverage >80% overall
- Integration tests pass consistently
- Health checks return accurate status
- No placeholder/TODO comments in production code
- All decorators are functional and tested

---

## 🔧 Testing Strategy

### Unit Tests Required
- Each repository implementation
- Each service method
- Each handler endpoint
- All decorators
- Configuration validation

### Integration Tests Required  
- Full node creation flow
- Node update with optimistic locking
- Query with pagination
- Transaction rollback scenarios
- Health check accuracy

### Performance Tests Recommended
- Repository operation latency
- Decorator overhead measurement
- Cache hit/miss rates
- Circuit breaker behavior

---

## 📝 Implementation Examples

### AI Feature Flag Implementation

Here's the specific implementation needed for proper AI feature flag handling:

#### 1. Update Enhanced Category Service
```go
// internal/service/category/enhanced_service.go

type enhancedService struct {
    BasicService
    repo   repository.Repository
    llmSvc *llm.Service
    config *config.Config // Add configuration
}

// Update constructor
func NewEnhancedService(repo repository.Repository, llmSvc *llm.Service, cfg *config.Config) EnhancedService {
    return &enhancedService{
        BasicService: BasicService{
            categoryRepo: nil,
            aiService:    nil,
        },
        repo:   repo,
        llmSvc: llmSvc,
        config: cfg,
    }
}

// Update CategorizeNode to check feature flag
func (s *enhancedService) CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error) {
    existingCategories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
        UserID: node.UserID().String(),
    })
    if err != nil {
        return nil, appErrors.Wrap(err, "failed to fetch existing categories")
    }

    var finalCategories []domain.Category
    var mappings []domain.NodeCategory

    // Check feature flag for AI processing
    if s.config.Features.EnableAIProcessing && s.llmSvc != nil && s.llmSvc.IsAvailable() {
        // Try AI categorization
        suggestions, err := s.llmSvc.SuggestCategories(ctx, node.Content().String(), existingCategories)
        if err != nil {
            log.Printf("AI categorization failed for node %s: %v", node.ID().String(), err)
        } else {
            // Process AI suggestions...
            // (existing AI processing code)
        }
    }

    // Always fallback to keyword-based categorization if no AI results
    if len(finalCategories) == 0 {
        keywordCategories, err := s.categorizeByKeywords(ctx, node, existingCategories)
        if err != nil {
            return nil, appErrors.Wrap(err, "keyword categorization failed")
        }
        finalCategories = keywordCategories
        // Add mappings...
    }

    return finalCategories, nil
}
```

#### 2. Update Category Handler
```go
// internal/handlers/category.go

func (h *CategoryHandler) CategorizeNode(w http.ResponseWriter, r *http.Request) {
    userID, ok := getUserID(r)
    if !ok {
        api.Error(w, http.StatusUnauthorized, "Authentication required")
        return
    }
    
    nodeID := chi.URLParam(r, "nodeId")
    if nodeID == "" {
        api.Error(w, http.StatusBadRequest, "Node ID is required")
        return
    }

    // Check if AI processing is enabled
    if !h.config.Features.EnableAIProcessing {
        api.Success(w, http.StatusOK, map[string]interface{}{
            "message":    "Auto-categorization disabled (AI processing not enabled)",
            "categories": []interface{}{},
            "nodeId":     nodeID,
        })
        return
    }

    // Get node and perform categorization
    // ... rest of implementation
}
```

#### 3. Update DI Provider
```go
// internal/di/providers.go

func provideCategoryService(
    repo repository.Repository,
    llmSvc *llm.Service,
    cfg *config.Config,
) categoryService.EnhancedService {
    return categoryService.NewEnhancedService(repo, llmSvc, cfg)
}
```

---

## 📝 Notes

- Prioritize completing existing patterns over introducing new ones
- Maintain consistency with established code style
- Use existing error types and patterns
- Follow the established layer boundaries
- No business logic in adapters or handlers
- Keep decorators focused on single concerns
- Ensure backward compatibility with existing APIs