# Backend Improvement Plan - Issue #830

## Targeted Backend Improvement Plan

Based on the comprehensive code review, here's the focused implementation plan for critical improvements:

### 1️⃣ **DEPENDENCY INJECTION REFACTORING**

**Remove Backward Compatibility Layer:**
- Delete the compatibility wrapper in `container.go` 
- Remove the old `Container` type entirely
- Update all references to use `ApplicationContainer` directly
- Simplify the initialization flow

**Fix Circular Dependencies:**
- Ensure containers are initialized in strict order: Infrastructure → Repository → Service → Handler
- Remove any bidirectional dependencies between containers
- Use interfaces instead of concrete types for dependencies

**Interface-Based DI:**
- Create `internal/di/contracts.go` with interface definitions
- Update containers to depend on interfaces, not implementations
- This will improve testability and decoupling

### 2️⃣ **SERVICE LAYER IMPROVEMENTS** 

**Extract Business Logic to Domain:**
- Move complex business rules from `NodeService` to domain entities/services
- Keep application services thin - only orchestration
- Domain services like `ConnectionAnalyzer` are already good examples

**SAGA Pattern Assessment:**
- Current transactional operations are relatively simple
- **SAGA not immediately needed** - your UnitOfWork pattern is sufficient
- Would only recommend SAGA if you add multi-service distributed transactions

### 3️⃣ **ERROR HANDLING REFINEMENT**

**Complete UnifiedError Migration:**
- Replace all old error types with `UnifiedError`
- Remove legacy error handling code
- Ensure consistent error responses across all endpoints

**Add Correlation IDs:**
- Add `CorrelationID` field to `UnifiedError`
- Pass correlation ID through context
- Include in all error logs for tracing

**Error Recovery Strategies:**
- Implement retry logic for transient errors (already have `Retryable` field)
- Add exponential backoff for retries
- Define clear recovery strategies per error type

### 4️⃣ **CONFIGURATION MANAGEMENT**

**Configuration Validation:**
- Add validation in `config.LoadConfig()` to fail fast on invalid configs
- Implement required vs optional field checks
- Validate AWS credentials and table names at startup

**Environment-Specific Configs:**
- Create config files: `config.dev.yaml`, `config.prod.yaml`
- Implement proper defaults with override hierarchy
- Add configuration schema documentation

**Feature Flags:**
- Add `FeatureFlags` section to config
- Implement toggle mechanism in services
- Allow runtime feature enabling/disabling

### 5️⃣ **PERFORMANCE OPTIMIZATIONS**

**Already Implemented:**
- ✅ Generic repository with batch operations
- ✅ Connection pooling (AWS SDK handles this)
- ✅ Adaptive concurrency pool (`adaptive_pool.go`)

**Still Needed:**

**Request Batching for DynamoDB:**
- Enhance batch operations to queue and combine requests
- Implement write buffer for batch writes
- Add configurable batch size and flush intervals

**Query Optimization:**
- Review and optimize GSI usage
- Implement query result caching for frequently accessed data
- Add pagination limits to prevent large scans

**Async Processing:**
- Implement job queue for heavy operations
- Move non-critical operations to background workers
- Add async event publishing

## 📝 **IMPLEMENTATION STEPS**

**Step 1: DI Refactoring (Priority 1)**
```
1. Create internal/di/contracts.go with interfaces
2. Remove backward compatibility from container.go
3. Update all code to use ApplicationContainer
4. Fix any circular dependencies
```

**Step 2: Error Handling (Priority 2)**
```
1. Add correlation ID support to UnifiedError
2. Update all error creation to use UnifiedError
3. Implement retry strategies in service layer
4. Add error recovery metadata
```

**Step 3: Configuration Management (Priority 3)**
```
1. Add validation to config.LoadConfig()
2. Create environment-specific config files
3. Implement feature flags system
4. Add config hot-reload capability (optional)
```

**Step 4: Service Layer Improvements (Priority 4)**
```
1. Review services for business logic that belongs in domain
2. Move complex logic to domain services
3. Keep application services focused on orchestration
4. Ensure proper separation of concerns
```

**Step 5: Performance Optimizations (Priority 5)**
```
1. Implement request batching for DynamoDB writes
2. Add query result caching where appropriate
3. Implement async job processing for heavy operations
4. Optimize query patterns and indexes
```

## 🚀 **IMMEDIATE ACTIONS**

1. **Start with DI refactoring** - it's foundational and will make other changes easier
2. **Then tackle error handling** - critical for debugging and monitoring
3. **Configuration management** can be done in parallel
4. **Service layer improvements** are ongoing refactoring
5. **Performance optimizations** can be measured and implemented incrementally

## 📊 **PROGRESS TRACKING**

### Completed
- [x] DI Refactoring
  - [x] Create interface contracts (contracts.go)
  - [x] Remove backward compatibility (simplified container.go)
  - [x] Update to ApplicationContainer (using interfaces)
  - [x] Fix circular dependencies (interfaces instead of concrete types)

- [x] Error Handling
  - [x] Add correlation IDs (unified_errors.go with context helpers)
  - [x] Complete UnifiedError migration (added EnrichWithContext)
  - [x] Implement retry strategies (created retry.go with exponential backoff)
  
- [x] Configuration Management
  - [x] Add validation (using validator tags, already existed)
  - [x] Environment-specific configs (base.yaml, development.yaml, production.yaml exist)
  - [ ] Feature flags (partially implemented, needs usage in services)

- [x] Service Layer
  - [x] Extract business logic to domain (node_domain_service.go)
  - [x] Thin application services (services already follow orchestration pattern)
  
- [x] Performance
  - [x] Request batching (batch_writer.go with efficient DynamoDB batching)
  - [x] Query optimization (query_optimizer.go with caching and parallel queries)
  - [x] Async processing (processor.go with worker pools and priority queues)

This focused plan addresses core concerns without over-engineering. The changes will significantly improve maintainability, debuggability, and performance while keeping the codebase clean and simple.