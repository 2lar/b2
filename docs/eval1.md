# Brain2 Backend Architecture Evaluation & Refactoring Plan

**Date:** 2025-08-23  
**Status:** Comprehensive Assessment  
**Purpose:** Technical evaluation for learning-focused refactoring

## Executive Summary

The Brain2 backend is a sophisticated, event-driven knowledge management system built with enterprise-grade patterns including Clean Architecture, Domain-Driven Design, and CQRS. While architecturally sound, the current implementation reveals opportunities for simplification, optimization, and enhanced learning value.

**Key Findings:**
- ✅ **Strengths**: Well-structured domain model, clear layer separation, comprehensive DI setup
- ⚠️ **Areas for Improvement**: Over-engineering for use case complexity, incomplete CQRS migration, deployment reliability
- 🎯 **Learning Opportunity**: Excellent foundation for understanding enterprise patterns with practical refinements needed

---

## Table of Contents

1. [Application Purpose & Core Requirements](#application-purpose--core-requirements)
2. [Current Architecture Analysis](#current-architecture-analysis)
3. [Pattern Implementation Assessment](#pattern-implementation-assessment)
4. [Identified Issues & Anti-Patterns](#identified-issues--anti-patterns)
5. [Refactoring Roadmap](#refactoring-roadmap)
6. [Learning Opportunities](#learning-opportunities)
7. [Implementation Guidelines](#implementation-guidelines)
8. [Testing Strategy](#testing-strategy)
9. [Performance Optimization Plan](#performance-optimization-plan)
10. [Documentation & Resources](#documentation--resources)

---

## Application Purpose & Core Requirements

### What Brain2 Does

Brain2 is a **graph-based personal knowledge management system** that automatically connects memories, thoughts, and ideas. Core functionality includes:

**Primary Features:**
- **Memory Management**: Create, update, delete personal notes/thoughts
- **Automatic Connections**: AI-powered keyword extraction and relationship discovery
- **Knowledge Graph**: Interactive visualization of connected memories
- **Real-time Updates**: Live graph updates via WebSockets
- **User Isolation**: Complete data separation between users
- **Category System**: Optional organization of memories

**Technical Requirements:**
- **Scalability**: Handle 10K+ memories per user
- **Performance**: Sub-2s response times, minimal cold starts
- **Security**: JWT authentication, data isolation
- **Availability**: 99.9% uptime on AWS serverless stack
- **Cost**: Stay within AWS free tier limits

### Current Business Model

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   User Creates  │───▶│  AI Extracts     │───▶│  Graph Updates  │
│   Memory        │    │  Keywords        │    │  Automatically  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Persistence   │◀───│  Connection      │◀───│  Real-time      │
│   (DynamoDB)    │    │  Discovery       │    │  Notifications  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

---

## Current Architecture Analysis

### Technology Stack Assessment

| Component | Current Choice | Assessment | Learning Value |
|-----------|---------------|------------|----------------|
| **Language** | Go | ✅ Excellent for serverless, strong typing | High - industry standard |
| **Framework** | Chi Router | ✅ Lightweight, fast | Medium - simple HTTP routing |
| **Architecture** | Clean Architecture + DDD | ⚠️ Over-engineered for scope | Very High - enterprise patterns |
| **Data** | DynamoDB | ✅ Perfect for graph data | High - NoSQL mastery |
| **Deployment** | AWS Lambda + CDK | ⚠️ CDK complex for Go | High - modern cloud patterns |
| **DI** | Google Wire | ✅ Compile-time injection | Very High - advanced Go technique |
| **Testing** | Limited coverage | ❌ Major gap | Critical - quality assurance |

### Layer Structure Analysis

```
┌─────────────────────────────────────────────────────────────────┐
│                        INTERFACE LAYER                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  HTTP Handlers  │  │   WebSocket     │  │    GraphQL      │  │
│  │  (Memory CRUD)  │  │   (Real-time)   │  │   (Future)      │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│                      APPLICATION LAYER                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │  Node Service   │  │  Query Service  │  │  Event Bus      │  │
│  │  (Commands)     │  │  (Queries)      │  │  (Events)       │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│                        DOMAIN LAYER                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │     Node        │  │      Edge       │  │   Category      │  │
│  │  (Aggregate)    │  │   (Entity)      │  │  (Aggregate)    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│                    INFRASTRUCTURE LAYER                        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │   DynamoDB      │  │   EventBridge   │  │     Cache       │  │
│  │ (Persistence)   │  │   (Events)      │  │   (Redis?)      │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Code Organization Quality

**Strengths:**
- ✅ Clear package boundaries with `/internal` structure
- ✅ Logical separation of concerns
- ✅ Consistent naming conventions
- ✅ Well-documented architectural decisions (ADRs)

**Weaknesses:**
- ❌ Inconsistent error handling patterns
- ❌ Mixed legacy and modern code styles
- ❌ Duplicate type definitions
- ❌ Incomplete interface implementations

---

## Pattern Implementation Assessment

### 1. Clean Architecture Implementation

**Current State: B+ (Good with Issues)**

```go
// ✅ GOOD: Clear dependency direction
// interfaces/http/handlers/memory.go
type MemoryHandler struct {
    nodeService     services.NodeService      // Depends on abstraction
    queryService    services.NodeQueryService // Not implementation
    eventBridge     events.EventBridge       // Interface, not concrete
}

// ❌ ISSUE: Mixed concerns in handlers
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    // HTTP parsing (correct layer)
    var req api.CreateNodeRequest
    
    // Business logic leakage (wrong layer)
    if len(req.Content) < 10 {
        return errors.New("content too short") // Should be domain validation
    }
    
    // Application service call (correct)
    result, err := h.nodeService.CreateNode(ctx, cmd)
}
```

**Improvements Needed:**
1. Move validation to domain layer
2. Remove business logic from handlers
3. Standardize error handling across layers
4. Complete interface abstractions

### 2. Domain-Driven Design Assessment

**Current State: A- (Very Good)**

```go
// ✅ EXCELLENT: Rich domain model with behavior
type Node struct {
    // Private fields for encapsulation
    id        shared.NodeID
    content   shared.Content
    title     shared.Title
    keywords  shared.Keywords
    
    // Business methods, not just data
    func (n *Node) UpdateContent(content shared.Content) error
    func (n *Node) AddConnection(targetID shared.NodeID) error
    func (n *Node) ExtractKeywords() shared.Keywords
}

// ✅ GOOD: Value objects with validation
type Content struct {
    value string
}

func NewContent(value string) (Content, error) {
    if len(strings.TrimSpace(value)) == 0 {
        return Content{}, ErrEmptyContent
    }
    if len(value) > MaxContentLength {
        return Content{}, ErrContentTooLong
    }
    return Content{value: value}, nil
}
```

**Issues Found:**
- **Duplicate Fields**: Both private and public fields in Node struct
- **Incomplete Value Objects**: Some primitives still used directly
- **Missing Invariants**: Some business rules not enforced

### 3. CQRS Implementation Analysis

**Current State: C (Incomplete Migration)**

```go
// ✅ GOOD: Separate command and query services
type NodeService struct {          // Write side
    nodeRepo    repository.NodeRepository
    eventBus    events.EventBus
}

type NodeQueryService struct {     // Read side
    nodeRepo    repository.NodeRepository  // ❌ Same repo used
    cache       cache.Cache
}

// ❌ ISSUE: Bridge patterns indicating incomplete migration
// services/types.go contains duplicate command structures
type CreateNodeCommand struct {    // Duplicate 1
    UserID  string
    Content string
    // Missing Title field ❌
}

// commands/node_commands.go
type CreateNodeCommand struct {    // Duplicate 2
    UserID  string
    Content string
    Title   string  // ✅ Has title field
}
```

**Migration Status:**
- **Write Side**: 70% complete, good command structure
- **Read Side**: 40% complete, needs dedicated read models
- **Event Handling**: 30% complete, basic event bus setup
- **Consistency**: Major issues with duplicate types

### 4. Repository Pattern Evaluation

**Current State: B (Good Foundation, Needs Refinement)**

```go
// ✅ EXCELLENT: Interface abstraction
type NodeRepository interface {
    FindByID(ctx context.Context, id shared.NodeID) (*node.Node, error)
    Save(ctx context.Context, node *node.Node) error
    FindByUserID(ctx context.Context, userID shared.UserID) ([]*node.Node, error)
    Delete(ctx context.Context, id shared.NodeID) error
}

// ✅ GOOD: DynamoDB implementation
type DynamoNodeRepository struct {
    client    *dynamodb.Client
    tableName string
    logger    *zap.Logger
}

// ❌ ISSUE: Methods that violate interface segregation
func (r *DynamoNodeRepository) FindWithConnections(...) // Too specific
func (r *DynamoNodeRepository) BatchSaveWithEdges(...) // Mixed concerns
```

**Strengths:**
- Clean interface abstractions
- Proper dependency injection
- Good error handling
- Performance optimizations (batch operations)

**Issues:**
- Interface segregation violations
- Mixed concerns in some methods
- Inconsistent error types
- Missing specification pattern

### 5. Dependency Injection Analysis

**Current State: A (Excellent Implementation)**

```go
// ✅ EXCELLENT: Google Wire usage
//go:generate go run -mod=mod github.com/google/wire/cmd/wire

// wire.go - Provider definitions
func ProvideNodeService(
    repo repository.NodeRepository,
    eventBus events.EventBus,
    analyzer services.ConnectionAnalyzer,
) *services.NodeService {
    return services.NewNodeService(repo, eventBus, analyzer)
}

// wire_gen.go - Generated code
func InitializeContainer() (*Container, error) {
    // Compile-time dependency resolution
    nodeRepo := provideNodeRepository(...)
    eventBus := provideEventBus(...)
    nodeService := provideNodeService(nodeRepo, eventBus, ...)
    return &Container{...}
}
```

**Strengths:**
- Compile-time dependency resolution
- No reflection overhead
- Clear dependency graph
- Easy to test with mocks

**Minor Issues:**
- Some circular dependency hints
- Missing interface bindings for testing

---

## Identified Issues & Anti-Patterns

### Critical Issues (P0 - Must Fix)

#### 1. Title Field Deployment Bug
**Impact**: Core feature broken, blocks user workflow

```go
// Problem: Duplicate command definitions causing confusion
// File 1: internal/application/services/types.go
type CreateNodeCommand struct {
    UserID  string  `json:"user_id"`
    Content string  `json:"content"`
    Tags    []string `json:"tags"`
    // Missing Title field ❌
}

// File 2: internal/application/commands/node_commands.go  
type CreateNodeCommand struct {
    UserID  string  `json:"user_id"`
    Content string  `json:"content"`
    Title   string  `json:"title"`  // ✅ Has title field
    Tags    []string `json:"tags"`
}
```

**Root Cause**: Build system using cached/wrong version during deployment

#### 2. Build & Deployment Pipeline Issues
**Impact**: Development velocity severely impacted

```bash
# Problems identified:
# 1. Go build cache not being cleared
# 2. CDK asset hashing not detecting changes
# 3. Lambda not updating despite "successful" deployment

# Current workaround needed:
go clean -cache
go clean -modcache  
go build -a  # Force rebuild all packages
# Direct AWS CLI update required
```

#### 3. Testing Coverage Gaps
**Impact**: No confidence in refactoring safety

```bash
# Test files found (very limited):
find . -name "*_test.go"
# ./internal/repository/pagination_test.go
# ./internal/di/container_test.go
# ./internal/di/wire_test.go
# ./internal/middleware/middleware_test.go
# ./infrastructure/dynamodb/tests/integration_test.go
# ./internal/config/config_test.go
# ./infrastructure/dynamodb/idempotency_test.go

# Missing: Domain logic tests, service tests, integration tests
```

### High Priority Issues (P1 - Should Fix)

#### 1. Architecture Complexity vs. Use Case
**Analysis**: Enterprise patterns for simple CRUD operations

```
Complexity Levels:
┌─────────────────────┬─────────────────┬──────────────────┐
│   Current Pattern   │   Complexity    │   Use Case Fit   │
├─────────────────────┼─────────────────┼──────────────────┤
│ DDD + Aggregates    │      High       │     Medium       │
│ CQRS + Event Sourcing│     Very High   │      Low         │
│ Clean Architecture  │      Medium     │      High        │
│ Repository Pattern  │      Medium     │      High        │
│ Unit of Work        │      High       │      Low         │
│ Domain Events       │      High       │      Medium      │
└─────────────────────┴─────────────────┴──────────────────┘
```

#### 2. Performance Bottlenecks

```go
// Issue: Cold start times averaging 15+ seconds
func init() {
    // Heavy DI container initialization
    container, err = di.InitializeContainer()  // 8-12 seconds
    
    // Complex validation setup
    if err := container.Validate()  // 2-3 seconds
    
    // Database connections
    // AWS service clients
    // Event bus setup
}

// Impact on user experience:
// - First request timeout (15s+ cold start)
// - Subsequent requests fast (<100ms)
// - Users abandon on first slow load
```

### Medium Priority Issues (P2 - Nice to Fix)

#### 1. Code Duplication & Inconsistency

```go
// Multiple ways to handle the same concept
// Pattern 1: Old style
func (h *Handler) CreateMemory(w http.ResponseWriter, r *http.Request) {
    decoder := json.NewDecoder(r.Body)
    var req CreateRequest
    decoder.Decode(&req)  // No error handling
}

// Pattern 2: New style with proper error handling
func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
    var req api.CreateNodeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        api.WriteError(w, api.NewBadRequest("invalid JSON", err))
        return
    }
}

// Pattern 3: Using validation middleware
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
    req, err := validators.ParseAndValidate[api.CreateCategoryRequest](r.Body)
    if err != nil {
        response.WriteValidationError(w, err)
        return
    }
}
```

#### 2. Error Handling Inconsistency

```go
// Different error styles throughout codebase
// Style 1: Domain errors
return shared.NewDomainError("invalid_content", "content too short", err)

// Style 2: Application errors  
return appErrors.Wrap(err, "failed to create node")

// Style 3: HTTP errors
return api.NewBadRequest("validation failed", err)

// Style 4: Standard Go errors
return fmt.Errorf("database error: %w", err)
```

---

## Refactoring Roadmap

### Phase 1: Critical Fixes & Foundation (Week 1-2)
*Priority: P0 issues, establish development confidence*

#### 1.1 Fix Deployment Pipeline
```bash
# Goals:
# - Reliable deployments
# - Faster iteration cycle  
# - Confidence in changes reaching production

# Tasks:
1. Create reliable build scripts with forced rebuilds
2. Fix CDK asset detection issues
3. Add deployment verification checks
4. Document deployment process
```

**Implementation Steps:**
```bash
# backend/build-reliable.sh
#!/bin/bash
set -e
echo "🧹 Clearing all caches..."
go clean -cache -modcache
rm -rf build/

echo "🔧 Force rebuilding with verification..."
go mod download
go generate ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -ldflags="-s -w" \
    -o build/main/bootstrap ./cmd/main

# Verify binary contains expected changes
if strings build/main/bootstrap | grep -q "EXPECTED_DEBUG_STRING"; then
    echo "✅ Build contains latest changes"
else
    echo "❌ Build missing expected changes"
    exit 1
fi
```

#### 1.2 Resolve Duplicate Types Issue
```go
// Remove duplicate CreateNodeCommand from services/types.go
// Keep only the version in commands/node_commands.go

// Before: Two conflicting definitions
// After: Single source of truth

// commands/node_commands.go (KEEP)
type CreateNodeCommand struct {
    UserID         string    `json:"user_id" validate:"required,uuid"`
    Content        string    `json:"content" validate:"required,min=1,max=10000"`
    Title          string    `json:"title" validate:"max=200"`  // ✅ Title field
    Tags           []string  `json:"tags" validate:"max=10,dive,min=1,max=50"`
    IdempotencyKey string    `json:"idempotency_key"`
}
```

#### 1.3 Add Critical Tests
```go
// tests/domain/node_test.go
func TestNode_CreateWithTitle(t *testing.T) {
    userID := shared.NewUserID()
    content := shared.NewContent("Test content")
    title := shared.NewTitle("Test title")
    tags := shared.NewTags([]string{"test"})
    
    node, err := node.NewNode(userID, content, title, tags)
    
    assert.NoError(t, err)
    assert.Equal(t, "Test title", node.Title().String())
    assert.Equal(t, "Test content", node.Content().String())
}

// tests/integration/create_node_test.go
func TestCreateNode_EndToEnd(t *testing.T) {
    // Test full request flow including title persistence
    req := api.CreateNodeRequest{
        Content: "Integration test content",
        Title:   "Integration test title",
        Tags:    []string{"integration", "test"},
    }
    
    resp := makeRequest(t, "POST", "/api/v1/nodes", req)
    
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var created api.NodeResponse
    json.Unmarshal(resp.Body, &created)
    assert.Equal(t, "Integration test title", created.Title)
}
```

### Phase 2: Architecture Simplification (Week 3-4)
*Priority: Reduce complexity while maintaining learning value*

#### 2.1 Streamline Domain Model
```go
// Current: Overly complex with duplicate fields
type Node struct {
    // Private fields (DDD style)
    id        shared.NodeID    
    content   shared.Content   
    title     shared.Title     
    
    // Public fields (compatibility)
    ID        shared.NodeID    `json:"id"`        // Duplicate!
    Content   shared.Content   `json:"content"`   // Duplicate!
    Title     shared.Title     `json:"title"`     // Duplicate!
}

// Simplified: Single source of truth
type Node struct {
    id        shared.NodeID    // Keep private for encapsulation
    content   shared.Content   // Rich value objects
    title     shared.Title     // Optional with validation
    tags      shared.Tags      // Normalized tags
    keywords  shared.Keywords  // Auto-extracted
    userID    shared.UserID    // Owner
    createdAt time.Time       // Metadata
    updatedAt time.Time
    version   int             // Optimistic locking
    
    // Remove public duplicates entirely
    // Use accessor methods instead
}

// Clean accessors
func (n *Node) ID() shared.NodeID     { return n.id }
func (n *Node) Content() shared.Content { return n.content }  
func (n *Node) Title() shared.Title   { return n.title }

// Business methods
func (n *Node) UpdateTitle(title shared.Title) error {
    if err := title.Validate(); err != nil {
        return shared.NewDomainError("invalid_title", err.Error(), err)
    }
    
    n.title = title
    n.updatedAt = time.Now()
    n.version++
    
    // Domain event for CQRS
    n.RaiseEvent(shared.NodeTitleUpdated{
        NodeID: n.id,
        NewTitle: title.String(),
        UpdatedAt: n.updatedAt,
    })
    
    return nil
}
```

#### 2.2 Complete CQRS Migration
```go
// commands/node_commands.go - Write side
type CreateNodeCommand struct {
    UserID  shared.UserID    `validate:"required"`
    Content shared.Content   `validate:"required"`
    Title   shared.Title     `validate:"optional"`
    Tags    shared.Tags      `validate:"optional"`
}

type UpdateNodeCommand struct {
    NodeID  shared.NodeID    `validate:"required"`
    Content shared.Content   `validate:"optional"`
    Title   shared.Title     `validate:"optional"`
    Version int              `validate:"required"` // Optimistic locking
}

// queries/node_queries.go - Read side
type GetNodeQuery struct {
    NodeID shared.NodeID `validate:"required"`
    UserID shared.UserID `validate:"required"`
}

type ListNodesQuery struct {
    UserID     shared.UserID `validate:"required"`
    Search     string        `validate:"optional"`
    Tags       []string      `validate:"optional"`
    Pagination Pagination    `validate:"required"`
}

// Read models optimized for queries
type NodeReadModel struct {
    ID            string    `json:"id"`
    UserID        string    `json:"user_id"`
    Title         string    `json:"title"`
    ContentSnippet string   `json:"content_snippet"` // First 200 chars
    TagList       []string  `json:"tags"`
    ConnectionCount int     `json:"connection_count"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

#### 2.3 Optimize Cold Start Performance
```go
// Lazy initialization pattern
type Container struct {
    config     *config.Config
    
    // Lazy-loaded services
    nodeService     *services.NodeService
    nodeServiceOnce sync.Once
    
    queryService     *queries.NodeQueryService  
    queryServiceOnce sync.Once
}

func (c *Container) GetNodeService() *services.NodeService {
    c.nodeServiceOnce.Do(func() {
        // Initialize only when first needed
        c.nodeService = c.buildNodeService()
    })
    return c.nodeService
}

// Target: <5s cold start (vs current 15s+)
func main() {
    // Fast initialization - defer heavy work
    container := di.NewLazyContainer()
    router := setupBasicRouter()  // <500ms
    
    lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
        // Services initialized only on first use
        return router.ServeHTTP(ctx, req)
    })
}
```

### Phase 3: Advanced Patterns for Learning (Week 5-6)
*Priority: Educational value, understanding modern patterns*

#### 3.1 Event Sourcing Implementation
```go
// Event store for audit trail and replay capability
type Event struct {
    ID          string                 `json:"id"`
    AggregateID string                 `json:"aggregate_id"`
    Type        string                 `json:"type"`
    Data        map[string]interface{} `json:"data"`
    Version     int                    `json:"version"`
    Timestamp   time.Time              `json:"timestamp"`
}

type EventStore interface {
    SaveEvents(ctx context.Context, aggregateID string, events []Event, expectedVersion int) error
    GetEvents(ctx context.Context, aggregateID string) ([]Event, error)
    GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]Event, error)
}

// Node aggregate that can be rebuilt from events
func (n *Node) LoadFromHistory(events []Event) error {
    for _, event := range events {
        switch event.Type {
        case "NodeCreated":
            n.applyNodeCreatedEvent(event.Data)
        case "ContentUpdated":
            n.applyContentUpdatedEvent(event.Data)
        case "TitleUpdated":
            n.applyTitleUpdatedEvent(event.Data)
        }
    }
    return nil
}
```

#### 3.2 Saga Pattern for Complex Workflows
```go
// Orchestrate complex multi-step processes
type CreateNodeSaga struct {
    sagaID      string
    nodeID      shared.NodeID
    userID      shared.UserID
    currentStep SagaStep
    completed   bool
    compensations []CompensationAction
}

func (s *CreateNodeSaga) Execute(ctx context.Context) error {
    // Step 1: Create node
    if err := s.createNode(ctx); err != nil {
        return s.compensate(ctx, err)
    }
    
    // Step 2: Extract keywords  
    if err := s.extractKeywords(ctx); err != nil {
        return s.compensate(ctx, err)
    }
    
    // Step 3: Find connections
    if err := s.discoverConnections(ctx); err != nil {
        return s.compensate(ctx, err)
    }
    
    // Step 4: Update graph
    if err := s.updateGraph(ctx); err != nil {
        return s.compensate(ctx, err)
    }
    
    s.completed = true
    return nil
}

func (s *CreateNodeSaga) compensate(ctx context.Context, err error) error {
    // Execute compensation actions in reverse order
    for i := len(s.compensations) - 1; i >= 0; i-- {
        if compErr := s.compensations[i].Execute(ctx); compErr != nil {
            log.Printf("Compensation failed: %v", compErr)
        }
    }
    return err
}
```

#### 3.3 GraphQL Interface
```go
// Modern API alternative to REST
type Resolver struct {
    nodeService  *services.NodeService
    queryService *queries.NodeQueryService
}

func (r *Resolver) Node(ctx context.Context, args struct{ ID string }) (*NodeResolver, error) {
    node, err := r.queryService.GetNode(ctx, queries.GetNodeQuery{
        NodeID: shared.NodeID(args.ID),
        UserID: getUserFromContext(ctx),
    })
    if err != nil {
        return nil, err
    }
    
    return &NodeResolver{node: node}, nil
}

func (r *NodeResolver) Connections(ctx context.Context) ([]*ConnectionResolver, error) {
    // Lazy loading of connections
    connections, err := r.queryService.GetConnections(ctx, r.node.ID())
    if err != nil {
        return nil, err
    }
    
    resolvers := make([]*ConnectionResolver, len(connections))
    for i, conn := range connections {
        resolvers[i] = &ConnectionResolver{connection: conn}
    }
    return resolvers, nil
}

// GraphQL schema
"""
type Node {
    id: ID!
    title: String
    content: String!
    tags: [String!]!
    connections: [Connection!]!
    createdAt: DateTime!
    updatedAt: DateTime!
}

type Mutation {
    createNode(input: CreateNodeInput!): Node!
    updateNode(id: ID!, input: UpdateNodeInput!): Node!
    deleteNode(id: ID!): Boolean!
}
"""
```

### Phase 4: Performance & Scale Optimization (Week 7-8)
*Priority: Production readiness, monitoring, optimization*

#### 4.1 Read Model Optimization
```go
// Separate read database optimized for queries
type NodeReadModel struct {
    // Denormalized for fast queries
    ID              string    `dynamodb:"pk"`
    UserID          string    `dynamodb:"sk"`
    Title           string    `dynamodb:"title"`
    ContentSnippet  string    `dynamodb:"content_snippet"`
    FullContent     string    `dynamodb:"full_content"`
    Tags            []string  `dynamodb:"tags"`
    Keywords        []string  `dynamodb:"keywords"`
    ConnectionIDs   []string  `dynamodb:"connection_ids"`
    ConnectionCount int       `dynamodb:"connection_count"`
    CreatedAt       time.Time `dynamodb:"created_at"`
    UpdatedAt       time.Time `dynamodb:"updated_at"`
    
    // Indexed fields for fast searching
    SearchText      string    `dynamodb:"search_text,index=search-index"`
    TagsIndex       string    `dynamodb:"tags_index,index=tags-index"`
}

// Async read model updates via event handlers
func (h *NodeReadModelHandler) HandleNodeCreated(event shared.NodeCreatedEvent) error {
    readModel := &NodeReadModel{
        ID:             event.NodeID.String(),
        UserID:         event.UserID.String(),
        Title:          event.Title.String(),
        ContentSnippet: truncateContent(event.Content.String(), 200),
        FullContent:    event.Content.String(),
        Tags:           event.Tags.ToSlice(),
        Keywords:       event.Keywords.ToSlice(),
        CreatedAt:      event.Timestamp,
        UpdatedAt:      event.Timestamp,
        
        // Searchable text combining all fields
        SearchText:     strings.ToLower(event.Title.String() + " " + event.Content.String()),
        TagsIndex:      strings.Join(event.Tags.ToSlice(), "|"),
    }
    
    return h.readRepo.Save(context.Background(), readModel)
}
```

#### 4.2 Multi-Layer Caching Strategy
```go
// L1: In-memory cache (fastest)
type MemoryCache struct {
    cache *sync.Map
    ttl   time.Duration
}

// L2: Redis cache (shared across instances)
type RedisCache struct {
    client redis.UniversalClient
    ttl    time.Duration
}

// L3: Database (source of truth)
type CachedNodeRepository struct {
    repo      repository.NodeRepository
    l1Cache   *MemoryCache
    l2Cache   *RedisCache
    metrics   metrics.Collector
}

func (r *CachedNodeRepository) FindByID(ctx context.Context, id shared.NodeID) (*node.Node, error) {
    cacheKey := fmt.Sprintf("node:%s", id.String())
    
    // L1 cache check
    if cached, found := r.l1Cache.Get(cacheKey); found {
        r.metrics.IncrementCounter("cache.l1.hit")
        return cached.(*node.Node), nil
    }
    
    // L2 cache check
    if cached, err := r.l2Cache.Get(ctx, cacheKey); err == nil {
        r.metrics.IncrementCounter("cache.l2.hit")
        node := deserializeNode(cached)
        r.l1Cache.Set(cacheKey, node) // Promote to L1
        return node, nil
    }
    
    // L3: Database
    r.metrics.IncrementCounter("cache.miss")
    node, err := r.repo.FindByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Populate caches
    r.l1Cache.Set(cacheKey, node)
    r.l2Cache.Set(ctx, cacheKey, serializeNode(node))
    
    return node, nil
}
```

#### 4.3 Comprehensive Monitoring
```go
// Structured logging with correlation IDs
type StructuredLogger struct {
    logger *zap.Logger
}

func (l *StructuredLogger) LogNodeOperation(ctx context.Context, operation string, nodeID shared.NodeID, duration time.Duration, err error) {
    correlationID := getCorrelationID(ctx)
    userID := getUserIDFromContext(ctx)
    
    fields := []zap.Field{
        zap.String("correlation_id", correlationID),
        zap.String("user_id", userID.String()),
        zap.String("node_id", nodeID.String()),
        zap.String("operation", operation),
        zap.Duration("duration", duration),
    }
    
    if err != nil {
        fields = append(fields, zap.Error(err))
        l.logger.Error("Node operation failed", fields...)
    } else {
        l.logger.Info("Node operation completed", fields...)
    }
}

// Custom metrics for business operations
type MetricsCollector struct {
    cloudWatch *cloudwatch.Client
}

func (m *MetricsCollector) RecordNodeOperation(operation string, duration time.Duration, success bool) {
    // Custom CloudWatch metrics
    metric := &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("Brain2/Backend"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String("NodeOperationDuration"),
                Value:      aws.Float64(duration.Seconds()),
                Unit:       types.StandardUnitSeconds,
                Dimensions: []types.Dimension{
                    {
                        Name:  aws.String("Operation"),
                        Value: aws.String(operation),
                    },
                    {
                        Name:  aws.String("Success"),
                        Value: aws.String(fmt.Sprintf("%t", success)),
                    },
                },
            },
        },
    }
    
    m.cloudWatch.PutMetricData(context.Background(), metric)
}
```

---

## Learning Opportunities

### 1. Enterprise Patterns Mastery

**Current Implementation Learning Value: ★★★★★**

The codebase excellently demonstrates real-world enterprise patterns:

#### Domain-Driven Design Deep Dive
```go
// Rich Aggregate Root with business logic
type Node struct {
    // Encapsulation: private fields
    id      shared.NodeID
    content shared.Content
    
    // Invariant enforcement
    func (n *Node) UpdateContent(content shared.Content) error {
        // Business rule: content must be validated
        if err := content.Validate(); err != nil {
            return shared.NewDomainError("invalid_content", err.Error(), err)
        }
        
        // Update with side effects
        oldKeywords := n.keywords
        n.content = content
        n.keywords = content.ExtractKeywords()
        n.updatedAt = time.Now()
        n.version++
        
        // Domain event for eventual consistency
        n.RaiseEvent(shared.NodeContentUpdated{
            NodeID:      n.id,
            OldKeywords: oldKeywords,
            NewKeywords: n.keywords,
            UpdatedAt:   n.updatedAt,
        })
        
        return nil
    }
}

// Value Object with business rules
type Content struct {
    value string
}

func NewContent(value string) (Content, error) {
    trimmed := strings.TrimSpace(value)
    
    // Business rules enforced at creation
    if len(trimmed) == 0 {
        return Content{}, errors.New("content cannot be empty")
    }
    if len(trimmed) > 10000 {
        return Content{}, errors.New("content too long")
    }
    if containsProfanity(trimmed) {
        return Content{}, errors.New("content contains inappropriate language")
    }
    
    return Content{value: trimmed}, nil
}

// Business behavior in the value object
func (c Content) ExtractKeywords() shared.Keywords {
    // NLP processing, hashtag extraction, etc.
    words := strings.Fields(strings.ToLower(c.value))
    keywords := make([]string, 0)
    
    for _, word := range words {
        if isSignificantWord(word) {
            keywords = append(keywords, word)
        }
    }
    
    return shared.NewKeywords(keywords)
}
```

**Learning Outcomes:**
- Understanding when to use value objects vs entities
- Implementing business rules within domain models
- Managing aggregate boundaries and consistency
- Domain event patterns for decoupling

#### CQRS Pattern Implementation
```go
// Command side - optimized for writes
type CreateNodeCommandHandler struct {
    nodeRepo repository.NodeRepository
    eventBus events.EventBus
    unitOfWork repository.UnitOfWork
}

func (h *CreateNodeCommandHandler) Handle(ctx context.Context, cmd commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // Start transaction
    uow, err := h.unitOfWork.Begin(ctx)
    if err != nil {
        return nil, err
    }
    defer uow.Rollback() // Auto-rollback if not committed
    
    // Create aggregate
    node, err := node.NewNode(cmd.UserID, cmd.Content, cmd.Title, cmd.Tags)
    if err != nil {
        return nil, err
    }
    
    // Persist
    if err := uow.NodeRepository().Save(ctx, node); err != nil {
        return nil, err
    }
    
    // Publish events
    for _, event := range node.GetUncommittedEvents() {
        h.eventBus.Publish(ctx, event)
    }
    
    // Commit transaction
    if err := uow.Commit(); err != nil {
        return nil, err
    }
    
    return &dto.CreateNodeResult{NodeID: node.ID()}, nil
}

// Query side - optimized for reads
type NodeQueryHandler struct {
    readModel repository.NodeReadModelRepository
    cache     cache.Cache
}

func (h *NodeQueryHandler) Handle(ctx context.Context, query queries.GetNodeQuery) (*dto.NodeView, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("node_view:%s", query.NodeID)
    if cached, found := h.cache.Get(cacheKey); found {
        return cached.(*dto.NodeView), nil
    }
    
    // Load from optimized read model
    readModel, err := h.readModel.FindByID(ctx, query.NodeID)
    if err != nil {
        return nil, err
    }
    
    // Convert to view DTO
    view := &dto.NodeView{
        ID:              readModel.ID,
        Title:           readModel.Title,
        ContentSnippet:  readModel.ContentSnippet,
        Tags:           readModel.Tags,
        ConnectionCount: readModel.ConnectionCount,
        CreatedAt:      readModel.CreatedAt,
        UpdatedAt:      readModel.UpdatedAt,
    }
    
    // Cache for future queries
    h.cache.Set(cacheKey, view, 5*time.Minute)
    
    return view, nil
}
```

**Learning Outcomes:**
- Separation of read and write concerns
- Optimizing data models for their specific use cases  
- Event-driven architecture patterns
- Performance implications of CQRS

### 2. Go Advanced Techniques

#### Dependency Injection with Google Wire
```go
// wire.go - Dependency graph definition
//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

package di

func InitializeContainer() (*Container, error) {
    // Wire analyzes these dependencies at compile time
    panic(wire.Build(
        // Providers (functions that create dependencies)
        ProvideConfig,
        ProvideLogger,
        ProvideDynamoDBClient,
        ProvideEventBus,
        
        // Repository layer
        dynamodb.NewNodeRepository,
        dynamodb.NewEdgeRepository,
        
        // Application services  
        services.NewNodeService,
        queries.NewNodeQueryService,
        
        // HTTP handlers
        handlers.NewMemoryHandler,
        
        // Container assembly
        NewContainer,
    ))
}

// Generated code (wire_gen.go) - never edit manually
func InitializeContainer() (*Container, error) {
    config, err := ProvideConfig()
    if err != nil {
        return nil, err
    }
    
    logger, err := ProvideLogger(config)
    if err != nil {
        return nil, err
    }
    
    dynamoClient, err := ProvideDynamoDBClient(config)
    if err != nil {
        return nil, err
    }
    
    nodeRepo := dynamodb.NewNodeRepository(dynamoClient, config.TableName, logger)
    eventBus := ProvideEventBus(config, logger)
    nodeService := services.NewNodeService(nodeRepo, eventBus)
    
    container := NewContainer(config, nodeService, /* ... */)
    return container, nil
}
```

**Learning Outcomes:**
- Compile-time dependency injection (vs runtime reflection)
- Dependency graph analysis and circular dependency detection
- Provider pattern and interface binding
- Testing with dependency injection

#### Interface Design Patterns
```go
// Segregated interfaces (Interface Segregation Principle)
type NodeReader interface {
    FindByID(ctx context.Context, id shared.NodeID) (*node.Node, error)
    FindByUserID(ctx context.Context, userID shared.UserID, spec Specification) ([]*node.Node, error)
    Count(ctx context.Context, userID shared.UserID, spec Specification) (int, error)
}

type NodeWriter interface {
    Save(ctx context.Context, node *node.Node) error
    Delete(ctx context.Context, id shared.NodeID) error
    SaveBatch(ctx context.Context, nodes []*node.Node) error
}

// Composition for full repository
type NodeRepository interface {
    NodeReader
    NodeWriter
}

// Specification pattern for flexible queries
type Specification interface {
    IsSatisfiedBy(node *node.Node) bool
    ToSQLWhere() string  // For SQL databases
    ToDynamoFilter() expression.ConditionBuilder  // For DynamoDB
}

type KeywordSpecification struct {
    keywords []string
}

func (s *KeywordSpecification) IsSatisfiedBy(node *node.Node) bool {
    nodeKeywords := node.Keywords().ToSlice()
    for _, keyword := range s.keywords {
        if contains(nodeKeywords, keyword) {
            return true
        }
    }
    return false
}

// Composable specifications
func NewKeywordSpec(keywords ...string) Specification {
    return &KeywordSpecification{keywords: keywords}
}

func (s *KeywordSpecification) And(other Specification) Specification {
    return &AndSpecification{left: s, right: other}
}

// Usage
spec := NewKeywordSpec("golang", "programming").
    And(NewDateRangeSpec(lastWeek, today)).
    And(NewUserSpec(userID))

nodes, err := repo.FindByUserID(ctx, userID, spec)
```

**Learning Outcomes:**
- Interface design principles and patterns
- Composition over inheritance in Go
- Specification pattern for query flexibility  
- Generic programming techniques in Go

### 3. AWS Serverless Architecture

#### Lambda Optimization Patterns
```go
// Connection pooling and reuse
type LambdaContainer struct {
    // Reused across invocations
    db     *sql.DB
    cache  *redis.Client
    config *Config
    
    // Lazy initialization
    nodeService     *services.NodeService
    nodeServiceOnce sync.Once
}

var globalContainer *LambdaContainer

func init() {
    // Initialize expensive resources once
    globalContainer = &LambdaContainer{
        config: loadConfig(),
    }
    
    // Database connection pool
    globalContainer.db = createDBPool()
    
    // Cache connection  
    globalContainer.cache = createCacheClient()
}

func HandleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    // Services initialized only when needed
    nodeService := globalContainer.GetNodeService()
    
    // Process request
    return processRequest(ctx, request, nodeService)
}

// Performance monitoring
func (c *LambdaContainer) GetNodeService() *services.NodeService {
    c.nodeServiceOnce.Do(func() {
        start := time.Now()
        c.nodeService = services.NewNodeService(
            c.getNodeRepository(),
            c.getEventBus(),
        )
        
        initDuration := time.Since(start)
        if initDuration > 100*time.Millisecond {
            log.Printf("WARNING: NodeService initialization took %v", initDuration)
        }
    })
    
    return c.nodeService
}
```

**Learning Outcomes:**
- Lambda cold start optimization techniques
- Connection pooling and resource reuse
- Monitoring and alerting for serverless applications
- Cost optimization strategies

#### DynamoDB Single-Table Design
```go
// Single table with composite keys for related data
type DynamoDBItem struct {
    PK   string                 `dynamodb:"PK"`   // Partition key  
    SK   string                 `dynamodb:"SK"`   // Sort key
    Type string                 `dynamodb:"Type"` // Item type
    Data map[string]interface{} `dynamodb:"-"`    // Varies by type
    
    // Global Secondary Index keys
    GSI1PK string `dynamodb:"GSI1PK"`
    GSI1SK string `dynamodb:"GSI1SK"`
    GSI2PK string `dynamodb:"GSI2PK"`  
    GSI2SK string `dynamodb:"GSI2SK"`
    
    // Item-specific fields
    Title       string    `dynamodb:"Title,omitempty"`
    Content     string    `dynamodb:"Content,omitempty"`
    Tags        []string  `dynamodb:"Tags,omitempty"`
    Keywords    []string  `dynamodb:"Keywords,omitempty"`
    CreatedAt   time.Time `dynamodb:"CreatedAt"`
    UpdatedAt   time.Time `dynamodb:"UpdatedAt"`
}

// Key patterns for different access patterns
// User's nodes: PK=USER#123, SK=NODE#456
// Node connections: PK=NODE#456, SK=EDGE#789  
// User's tags: PK=USER#123, SK=TAG#golang
// Search by keyword: GSI1PK=KEYWORD#golang, GSI1SK=USER#123#NODE#456

func (r *DynamoRepository) FindNodesByKeyword(ctx context.Context, userID shared.UserID, keyword string) ([]*node.Node, error) {
    // Use GSI for keyword search
    queryInput := &dynamodb.QueryInput{
        TableName:              aws.String(r.tableName),
        IndexName:              aws.String("GSI1"),
        KeyConditionExpression: aws.String("GSI1PK = :pk AND begins_with(GSI1SK, :sk_prefix)"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk":        StringAttr(fmt.Sprintf("KEYWORD#%s", keyword)),
            ":sk_prefix": StringAttr(fmt.Sprintf("USER#%s#NODE#", userID.String())),
        },
    }
    
    result, err := r.client.Query(ctx, queryInput)
    if err != nil {
        return nil, err
    }
    
    nodes := make([]*node.Node, 0, len(result.Items))
    for _, item := range result.Items {
        if node := r.itemToNode(item); node != nil {
            nodes = append(nodes, node)
        }
    }
    
    return nodes, nil
}
```

**Learning Outcomes:**
- NoSQL design patterns and access pattern optimization
- DynamoDB GSI usage and query optimization
- Single-table design benefits and trade-offs
- Cost optimization for DynamoDB operations

---

## Implementation Guidelines

### 1. Development Workflow

#### Git Workflow for Refactoring
```bash
# Feature branch strategy
git checkout -b feature/phase1-critical-fixes
git checkout -b feature/phase2-architecture-simplification  
git checkout -b feature/phase3-advanced-patterns
git checkout -b feature/phase4-performance-optimization

# Each phase should be independently deployable
# Small, incremental changes with tests
git commit -m "feat: resolve duplicate CreateNodeCommand types

- Remove duplicate from services/types.go
- Keep commands/node_commands.go as single source of truth
- Update all imports to use consistent command structure
- Add tests for command validation

Fixes: #123"
```

#### Testing Strategy per Phase
```go
// Phase 1: Integration tests for critical paths
func TestCreateNodeWithTitle_Integration(t *testing.T) {
    // Setup real DynamoDB local instance
    container := setupTestContainer(t)
    defer container.Cleanup()
    
    // Test full flow
    cmd := &commands.CreateNodeCommand{
        UserID:  shared.NewUserID(), 
        Content: shared.NewContent("Test content"),
        Title:   shared.NewTitle("Test title"),
        Tags:    shared.NewTags([]string{"test"}),
    }
    
    result, err := container.NodeService().CreateNode(context.Background(), cmd)
    require.NoError(t, err)
    
    // Verify persistence
    savedNode, err := container.NodeRepository().FindByID(context.Background(), result.NodeID)
    require.NoError(t, err)
    assert.Equal(t, "Test title", savedNode.Title().String())
}

// Phase 2: Unit tests for domain logic
func TestNode_UpdateTitle(t *testing.T) {
    // Arrange
    node := createTestNode(t)
    newTitle := shared.NewTitle("Updated title")
    
    // Act
    err := node.UpdateTitle(newTitle)
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, "Updated title", node.Title().String())
    assert.True(t, node.Version() > 0) // Version incremented
    
    // Verify domain events
    events := node.GetUncommittedEvents()
    assert.Len(t, events, 1)
    assert.IsType(t, shared.NodeTitleUpdated{}, events[0])
}

// Phase 3: Event-driven tests
func TestNodeCreatedEventHandler(t *testing.T) {
    // Setup event bus with test handler
    eventBus := events.NewTestEventBus()
    handler := &NodeCreatedHandler{
        readModelRepo: &MockReadModelRepository{},
    }
    eventBus.Subscribe(shared.NodeCreated{}, handler)
    
    // Publish event
    event := shared.NodeCreated{
        NodeID:  shared.NewNodeID(),
        UserID:  shared.NewUserID(),
        Content: shared.NewContent("Test"),
        Title:   shared.NewTitle("Test title"),
    }
    eventBus.Publish(context.Background(), event)
    
    // Verify handler was called
    eventBus.WaitForCompletion(1 * time.Second)
    assert.True(t, handler.WasCalled())
}

// Phase 4: Performance tests
func BenchmarkNodeCreation(b *testing.B) {
    container := setupBenchmarkContainer(b)
    defer container.Cleanup()
    
    cmd := &commands.CreateNodeCommand{
        UserID:  shared.NewUserID(),
        Content: shared.NewContent("Benchmark content"),
        Title:   shared.NewTitle("Benchmark title"),
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := container.NodeService().CreateNode(context.Background(), cmd)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### 2. Code Quality Standards

#### Error Handling Patterns
```go
// Consistent error handling throughout the application
package errors

import (
    "fmt"
    "net/http"
)

// Domain errors (business rule violations)
type DomainError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
    Cause   error  `json:"-"`
}

func (e DomainError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewDomainError(code, message string, cause error) *DomainError {
    return &DomainError{
        Code:    code,
        Message: message,
        Cause:   cause,
    }
}

// Application errors (infrastructure issues)
type ApplicationError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    HTTPStatus int    `json:"-"`
    Cause      error  `json:"-"`
}

func (e ApplicationError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Predefined application errors
var (
    ErrNodeNotFound = &ApplicationError{
        Code:       "node_not_found",
        Message:    "The requested node does not exist",
        HTTPStatus: http.StatusNotFound,
    }
    
    ErrUnauthorized = &ApplicationError{
        Code:       "unauthorized",
        Message:    "Authentication required",
        HTTPStatus: http.StatusUnauthorized,
    }
    
    ErrInternalServer = &ApplicationError{
        Code:       "internal_server_error",
        Message:    "An unexpected error occurred",
        HTTPStatus: http.StatusInternalServerError,
    }
)

// HTTP error handling
func WriteError(w http.ResponseWriter, err error) {
    switch e := err.(type) {
    case *DomainError:
        writeJSONError(w, http.StatusBadRequest, e.Code, e.Message)
    case *ApplicationError:
        writeJSONError(w, e.HTTPStatus, e.Code, e.Message)
    default:
        writeJSONError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
    }
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    
    response := map[string]interface{}{
        "error": map[string]interface{}{
            "code":    code,
            "message": message,
        },
    }
    
    json.NewEncoder(w).Encode(response)
}
```

#### Logging Standards
```go
// Structured logging with correlation IDs
type Logger struct {
    zap *zap.Logger
}

func (l *Logger) LogWithContext(ctx context.Context, level zapcore.Level, message string, fields ...zap.Field) {
    // Extract context information
    correlationID := getCorrelationID(ctx)
    userID := getUserID(ctx)
    requestID := getRequestID(ctx)
    
    // Standard context fields
    contextFields := []zap.Field{
        zap.String("correlation_id", correlationID),
        zap.String("user_id", userID),
        zap.String("request_id", requestID),
        zap.String("service", "brain2-backend"),
        zap.String("version", getBuildVersion()),
    }
    
    // Combine with provided fields
    allFields := append(contextFields, fields...)
    
    l.zap.Log(level, message, allFields...)
}

// Business event logging
func (l *Logger) LogNodeOperation(ctx context.Context, operation string, nodeID shared.NodeID, duration time.Duration, err error) {
    fields := []zap.Field{
        zap.String("operation", operation),
        zap.String("node_id", nodeID.String()),
        zap.Duration("duration", duration),
    }
    
    if err != nil {
        fields = append(fields, zap.Error(err))
        l.LogWithContext(ctx, zapcore.ErrorLevel, "Node operation failed", fields...)
    } else {
        l.LogWithContext(ctx, zapcore.InfoLevel, "Node operation completed", fields...)
    }
}

// Usage in handlers
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    ctx := r.Context()
    
    // Parse request
    var req api.CreateNodeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.logger.LogWithContext(ctx, zapcore.WarnLevel, "Invalid request JSON", zap.Error(err))
        errors.WriteError(w, errors.NewDomainError("invalid_json", "Request body is not valid JSON", err))
        return
    }
    
    // Execute business operation
    cmd := &commands.CreateNodeCommand{
        UserID:  getUserIDFromContext(ctx),
        Content: shared.NewContent(req.Content),
        Title:   shared.NewTitle(req.Title),
        Tags:    shared.NewTags(req.Tags),
    }
    
    result, err := h.nodeService.CreateNode(ctx, cmd)
    duration := time.Since(start)
    
    if err != nil {
        h.logger.LogNodeOperation(ctx, "create_node", shared.NodeID(""), duration, err)
        errors.WriteError(w, err)
        return
    }
    
    h.logger.LogNodeOperation(ctx, "create_node", result.NodeID, duration, nil)
    
    // Return success
    response := &api.CreateNodeResponse{
        ID:        result.NodeID.String(),
        Title:     result.Title,
        Content:   result.Content,
        CreatedAt: result.CreatedAt,
    }
    
    writeJSON(w, http.StatusCreated, response)
}
```

### 3. Deployment & Operations

#### Infrastructure as Code Best Practices
```typescript
// infra/lib/constructs/lambda-function.ts
export class OptimizedLambdaFunction extends Construct {
  public readonly function: Function;
  
  constructor(scope: Construct, id: string, props: OptimizedLambdaProps) {
    super(scope, id);
    
    // Lambda optimization
    this.function = new Function(this, 'Function', {
      runtime: Runtime.PROVIDED_AL2,
      handler: 'bootstrap',
      code: Code.fromAsset(props.buildPath),
      
      // Performance optimization
      memorySize: 1024,  // More memory = faster CPU
      timeout: Duration.seconds(30),
      reservedConcurrencyLimit: 100,
      
      // Environment optimization
      environment: {
        GOMEMLIMIT: '900MB',  // Leave room for Lambda overhead
        GODEBUG: 'gctrace=1,madvdontneed=1',
        AWS_LAMBDA_GO_MAX_PROCS: '2',
      },
      
      // Monitoring
      logRetention: RetentionDays.ONE_WEEK,
      tracing: Tracing.ACTIVE,
      
      // Networking
      vpc: props.vpc,
      vpcSubnets: { subnetType: SubnetType.PRIVATE_WITH_EGRESS },
      securityGroups: [props.securityGroup],
    });
    
    // CloudWatch alarms
    this.createAlarms();
    
    // Performance monitoring
    this.createDashboard();
  }
  
  private createAlarms(): void {
    // Cold start duration alarm
    new Alarm(this, 'ColdStartAlarm', {
      metric: this.function.metricDuration({
        dimensionsMap: { 'ColdStart': 'true' }
      }),
      threshold: 5000, // 5 seconds
      evaluationPeriods: 2,
      treatMissingData: TreatMissingData.NOT_BREACHING,
    });
    
    // Error rate alarm
    new Alarm(this, 'ErrorRateAlarm', {
      metric: this.function.metricErrors(),
      threshold: 10,
      evaluationPeriods: 3,
    });
    
    // Throttle alarm
    new Alarm(this, 'ThrottleAlarm', {
      metric: this.function.metricThrottles(),
      threshold: 1,
      evaluationPeriods: 1,
    });
  }
}
```

#### Deployment Pipeline
```yaml
# .github/workflows/deploy.yml
name: Deploy Backend

on:
  push:
    branches: [main]
    paths: ['backend/**', 'infra/**']

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
          
      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          
      - name: Run tests
        run: |
          cd backend
          go test -v -race -cover ./...
          
      - name: Run linting
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          working-directory: backend
          
  build:
    needs: test
    runs-on: ubuntu-latest
    outputs:
      binary-hash: ${{ steps.hash.outputs.hash }}
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
          
      - name: Build binaries
        run: |
          cd backend
          ./build.sh
          
      - name: Calculate binary hash
        id: hash
        run: |
          cd backend/build/main
          echo "hash=$(sha256sum bootstrap | cut -d' ' -f1)" >> $GITHUB_OUTPUT
          
      - name: Upload build artifacts
        uses: actions/upload-artifact@v3
        with:
          name: lambda-binaries
          path: backend/build/
          
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '18'
          cache: 'npm'
          cache-dependency-path: infra/package-lock.json
          
      - name: Download build artifacts
        uses: actions/download-artifact@v3
        with:
          name: lambda-binaries
          path: backend/build/
          
      - name: Deploy infrastructure
        run: |
          cd infra
          npm ci
          npx cdk deploy --all --require-approval never
          
      - name: Verify deployment
        run: |
          # Wait for deployment to propagate
          sleep 30
          
          # Test health endpoint
          HEALTH_URL=$(aws cloudformation describe-stacks \
            --stack-name Brain2Stack \
            --query 'Stacks[0].Outputs[?OutputKey==`HttpApiUrl`].OutputValue' \
            --output text)/health
            
          RESPONSE=$(curl -s -w "%{http_code}" "$HEALTH_URL")
          if [[ "${RESPONSE: -3}" != "200" ]]; then
            echo "Health check failed: $RESPONSE"
            exit 1
          fi
          
          # Verify binary hash matches
          DEPLOYED_HASH=$(curl -s "$HEALTH_URL" | jq -r '.build_hash')
          if [[ "$DEPLOYED_HASH" != "${{ needs.build.outputs.binary-hash }}" ]]; then
            echo "Binary hash mismatch. Expected: ${{ needs.build.outputs.binary-hash }}, Got: $DEPLOYED_HASH"
            exit 1
          fi
          
          echo "✅ Deployment verified successfully"
```

---

## Testing Strategy

### 1. Test Pyramid Implementation

```
                    ┌──────────────────┐
                    │   E2E Tests      │  ← 5% of tests
                    │ (Full user flow) │
                    └──────────────────┘
                  ┌────────────────────────┐
                  │  Integration Tests     │  ← 20% of tests
                  │  (Service + DB)        │
                  └────────────────────────┘
              ┌──────────────────────────────────┐
              │         Unit Tests               │  ← 75% of tests
              │  (Domain logic + Pure functions) │
              └──────────────────────────────────┘
```

#### Unit Tests - Domain Logic
```go
// tests/domain/node_test.go
func TestNode_NewNode_ValidatesContent(t *testing.T) {
    testCases := []struct {
        name        string
        content     string
        expectError bool
        errorCode   string
    }{
        {
            name:        "valid content",
            content:     "This is valid content",
            expectError: false,
        },
        {
            name:        "empty content",
            content:     "",
            expectError: true,
            errorCode:   "empty_content",
        },
        {
            name:        "content too long",
            content:     strings.Repeat("x", 10001),
            expectError: true,
            errorCode:   "content_too_long",
        },
        {
            name:        "content with profanity",
            content:     "This contains badword",
            expectError: true,
            errorCode:   "inappropriate_content",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            userID := shared.NewUserID()
            content, err := shared.NewContent(tc.content)
            
            if tc.expectError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tc.errorCode)
                return
            }
            
            require.NoError(t, err)
            
            node, err := node.NewNode(userID, content, shared.Title{}, shared.Tags{})
            require.NoError(t, err)
            assert.Equal(t, tc.content, node.Content().String())
        })
    }
}

func TestNode_UpdateTitle_RaisesEvent(t *testing.T) {
    // Arrange
    node := createTestNode(t)
    newTitle, _ := shared.NewTitle("Updated Title")
    
    // Act
    err := node.UpdateTitle(newTitle)
    
    // Assert
    require.NoError(t, err)
    
    events := node.GetUncommittedEvents()
    require.Len(t, events, 1)
    
    event, ok := events[0].(shared.NodeTitleUpdated)
    require.True(t, ok, "Expected NodeTitleUpdated event")
    assert.Equal(t, node.ID(), event.NodeID)
    assert.Equal(t, "Updated Title", event.NewTitle)
}
```

#### Integration Tests - Repository Layer
```go
// tests/integration/node_repository_test.go
func TestNodeRepository_SaveAndRetrieve(t *testing.T) {
    // Setup real DynamoDB local
    container := testcontainers.SetupDynamoDB(t)
    defer container.Cleanup()
    
    repo := dynamodb.NewNodeRepository(
        container.Client(),
        container.TableName(),
        "test-index",
        zaptest.NewLogger(t),
    )
    
    // Create test node
    userID := shared.NewUserID()
    content, _ := shared.NewContent("Integration test content")
    title, _ := shared.NewTitle("Integration test title")
    tags := shared.NewTags([]string{"integration", "test"})
    
    originalNode, err := node.NewNode(userID, content, title, tags)
    require.NoError(t, err)
    
    // Save
    ctx := context.WithValue(context.Background(), "userID", userID)
    err = repo.Save(ctx, originalNode)
    require.NoError(t, err)
    
    // Retrieve
    retrievedNode, err := repo.FindByID(ctx, originalNode.ID())
    require.NoError(t, err)
    
    // Assert
    assert.Equal(t, originalNode.ID(), retrievedNode.ID())
    assert.Equal(t, originalNode.Title().String(), retrievedNode.Title().String())
    assert.Equal(t, originalNode.Content().String(), retrievedNode.Content().String())
    assert.Equal(t, originalNode.Tags().ToSlice(), retrievedNode.Tags().ToSlice())
}

func TestNodeRepository_FindByUserID_WithSpecification(t *testing.T) {
    // Setup
    container := testcontainers.SetupDynamoDB(t)
    defer container.Cleanup()
    repo := createNodeRepository(t, container)
    
    // Create test data
    userID := shared.NewUserID()
    ctx := context.WithValue(context.Background(), "userID", userID)
    
    nodes := []*node.Node{
        createNodeWithTags(t, userID, "Node 1", []string{"golang", "backend"}),
        createNodeWithTags(t, userID, "Node 2", []string{"golang", "frontend"}),
        createNodeWithTags(t, userID, "Node 3", []string{"python", "backend"}),
    }
    
    for _, n := range nodes {
        require.NoError(t, repo.Save(ctx, n))
    }
    
    // Test specification query
    spec := repository.NewTagSpecification("golang")
    results, err := repo.FindByUserID(ctx, userID, spec)
    
    require.NoError(t, err)
    assert.Len(t, results, 2) // Should find 2 golang nodes
    
    for _, result := range results {
        assert.Contains(t, result.Tags().ToSlice(), "golang")
    }
}
```

#### E2E Tests - Full User Journey
```go
// tests/e2e/create_node_journey_test.go
func TestCreateNodeJourney_WithTitle(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }
    
    // Setup test environment
    testEnv := e2e.SetupTestEnvironment(t)
    defer testEnv.Cleanup()
    
    // Authenticate test user
    authToken := testEnv.AuthenticateTestUser()
    
    // Create node request
    requestBody := map[string]interface{}{
        "content": "This is an E2E test node with a title",
        "title":   "E2E Test Title",
        "tags":    []string{"e2e", "test"},
    }
    
    requestJSON, _ := json.Marshal(requestBody)
    
    // Make API request
    req, _ := http.NewRequest("POST", testEnv.APIBaseURL+"/api/v1/nodes", bytes.NewBuffer(requestJSON))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+authToken)
    
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()
    
    // Verify response
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var createResponse map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&createResponse)
    require.NoError(t, err)
    
    nodeID, ok := createResponse["id"].(string)
    require.True(t, ok, "Response should contain node ID")
    require.NotEmpty(t, nodeID)
    
    assert.Equal(t, "E2E Test Title", createResponse["title"])
    assert.Equal(t, "This is an E2E test node with a title", createResponse["content"])
    
    // Verify persistence by retrieving the node
    getReq, _ := http.NewRequest("GET", testEnv.APIBaseURL+"/api/v1/nodes/"+nodeID, nil)
    getReq.Header.Set("Authorization", "Bearer "+authToken)
    
    getResp, err := client.Do(getReq)
    require.NoError(t, err)
    defer getResp.Body.Close()
    
    assert.Equal(t, http.StatusOK, getResp.StatusCode)
    
    var getResponse map[string]interface{}
    err = json.NewDecoder(getResp.Body).Decode(&getResponse)
    require.NoError(t, err)
    
    // Verify persistence
    assert.Equal(t, nodeID, getResponse["id"])
    assert.Equal(t, "E2E Test Title", getResponse["title"])
    assert.Equal(t, "This is an E2E test node with a title", getResponse["content"])
    
    // Verify in graph endpoint
    graphResp, err := client.Get(testEnv.APIBaseURL + "/api/v1/graph?user_id=" + testEnv.UserID)
    require.NoError(t, err)
    defer graphResp.Body.Close()
    
    var graphData map[string]interface{}
    err = json.NewDecoder(graphResp.Body).Decode(&graphData)
    require.NoError(t, err)
    
    nodes, ok := graphData["nodes"].([]interface{})
    require.True(t, ok)
    
    // Find our node in the graph
    found := false
    for _, n := range nodes {
        node := n.(map[string]interface{})
        if node["id"] == nodeID {
            found = true
            assert.Equal(t, "E2E Test Title", node["title"])
            break
        }
    }
    assert.True(t, found, "Node should appear in graph data")
}
```

### 2. Testing Infrastructure

#### Test Containers for Integration Tests
```go
// tests/testcontainers/dynamodb.go
package testcontainers

import (
    "context"
    "testing"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

type DynamoDBContainer struct {
    container testcontainers.Container
    client    *dynamodb.Client
    tableName string
}

func SetupDynamoDB(t *testing.T) *DynamoDBContainer {
    ctx := context.Background()
    
    // Start DynamoDB Local container
    req := testcontainers.ContainerRequest{
        Image:        "amazon/dynamodb-local:latest",
        ExposedPorts: []string{"8000/tcp"},
        Cmd:          []string{"-jar", "DynamoDBLocal.jar", "-inMemory", "-sharedDb"},
        WaitingFor:   wait.ForListeningPort("8000/tcp"),
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        t.Fatalf("Failed to start DynamoDB container: %v", err)
    }
    
    // Get container endpoint
    host, err := container.Host(ctx)
    if err != nil {
        t.Fatalf("Failed to get container host: %v", err)
    }
    
    port, err := container.MappedPort(ctx, "8000")
    if err != nil {
        t.Fatalf("Failed to get container port: %v", err)
    }
    
    // Create DynamoDB client
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-west-2"),
        config.WithEndpointResolver(aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
            return aws.Endpoint{
                URL:               fmt.Sprintf("http://%s:%s", host, port.Port()),
                SigningRegion:     region,
                HostnameImmutable: true,
            }, nil
        })),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
    )
    if err != nil {
        t.Fatalf("Failed to create AWS config: %v", err)
    }
    
    client := dynamodb.NewFromConfig(cfg)
    
    // Create test table
    tableName := "test-brain2-" + randomString(8)
    err = createTestTable(ctx, client, tableName)
    if err != nil {
        t.Fatalf("Failed to create test table: %v", err)
    }
    
    return &DynamoDBContainer{
        container: container,
        client:    client,
        tableName: tableName,
    }
}

func (c *DynamoDBContainer) Client() *dynamodb.Client {
    return c.client
}

func (c *DynamoDBContainer) TableName() string {
    return c.tableName
}

func (c *DynamoDBContainer) Cleanup() {
    if c.container != nil {
        c.container.Terminate(context.Background())
    }
}
```

---

## Performance Optimization Plan

### 1. Current Performance Baseline

**Measured Performance (Lambda):**
- Cold Start: 15-20 seconds (Target: <5 seconds)
- Warm Request: 50-200ms (Target: <100ms)  
- Memory Usage: 512MB (Target: optimize for cost)
- Database Operations: 100-300ms (Target: <100ms)

### 2. Optimization Strategies

#### Cold Start Reduction
```go
// Strategy 1: Lazy initialization
type LazyContainer struct {
    config *config.Config
    
    // Lazy services with sync.Once
    nodeService     *services.NodeService
    nodeServiceOnce sync.Once
    
    // Connection pools initialized once
    dbPool      *sql.DB
    cacheClient *redis.Client
}

func (c *LazyContainer) GetNodeService() *services.NodeService {
    c.nodeServiceOnce.Do(func() {
        // Only initialize when first needed
        c.nodeService = services.NewNodeService(
            c.getNodeRepository(),
            c.getEventBus(),
        )
    })
    return c.nodeService
}

// Strategy 2: Provisioned concurrency for critical functions
// In CDK:
// provisionedConcurrencyConfig: {
//     provisionedConcurrencyUtilization: 0.8,
//     minCapacity: 1,
//     maxCapacity: 5,
// }
```

#### Database Query Optimization
```go
// Strategy 1: Batch operations
func (r *DynamoRepository) SaveBatch(ctx context.Context, nodes []*node.Node) error {
    const maxBatchSize = 25 // DynamoDB limit
    
    for i := 0; i < len(nodes); i += maxBatchSize {
        end := i + maxBatchSize
        if end > len(nodes) {
            end = len(nodes)
        }
        
        batch := nodes[i:end]
        if err := r.saveBatchChunk(ctx, batch); err != nil {
            return err
        }
    }
    
    return nil
}

// Strategy 2: Read optimization with projections
func (r *DynamoRepository) FindNodeSummaries(ctx context.Context, userID shared.UserID) ([]*NodeSummary, error) {
    input := &dynamodb.QueryInput{
        TableName:              aws.String(r.tableName),
        KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
        ProjectionExpression:   aws.String("SK, Title, Content, CreatedAt, UpdatedAt"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":pk":        StringAttr(fmt.Sprintf("USER#%s", userID.String())),
            ":sk_prefix": StringAttr("NODE#"),
        },
        Limit: aws.Int32(50), // Pagination
    }
    
    result, err := r.client.Query(ctx, input)
    if err != nil {
        return nil, err
    }
    
    summaries := make([]*NodeSummary, len(result.Items))
    for i, item := range result.Items {
        summaries[i] = r.itemToNodeSummary(item)
    }
    
    return summaries, nil
}

// Strategy 3: Connection pooling and reuse
var dbPool *sql.DB
var dbPoolOnce sync.Once

func GetDBPool() *sql.DB {
    dbPoolOnce.Do(func() {
        config := &sql.Config{
            MaxOpenConns:    10,
            MaxIdleConns:    5,
            ConnMaxLifetime: 5 * time.Minute,
            ConnMaxIdleTime: 30 * time.Second,
        }
        
        var err error
        dbPool, err = sql.OpenDB(connector, config)
        if err != nil {
            panic(fmt.Sprintf("Failed to create DB pool: %v", err))
        }
    })
    
    return dbPool
}
```

#### Caching Strategy Implementation
```go
// Multi-tier caching with TTL and invalidation
type CacheManager struct {
    l1 *sync.Map        // In-memory (fastest)
    l2 *redis.Client    // Distributed (shared)
    l3 repository.Repository // Database (source of truth)
    
    metrics *metrics.Collector
}

func (c *CacheManager) GetNode(ctx context.Context, nodeID shared.NodeID) (*node.Node, error) {
    cacheKey := fmt.Sprintf("node:%s", nodeID.String())
    
    // L1: Memory cache
    if value, found := c.l1.Load(cacheKey); found {
        c.metrics.RecordCacheHit("l1", "node")
        return value.(*node.Node), nil
    }
    
    // L2: Redis cache  
    if cached, err := c.l2.Get(ctx, cacheKey).Result(); err == nil {
        c.metrics.RecordCacheHit("l2", "node")
        
        var node node.Node
        if err := json.Unmarshal([]byte(cached), &node); err == nil {
            c.l1.Store(cacheKey, &node) // Promote to L1
            return &node, nil
        }
    }
    
    // L3: Database
    c.metrics.RecordCacheMiss("node")
    node, err := c.l3.FindNodeByID(ctx, nodeID)
    if err != nil {
        return nil, err
    }
    
    // Populate caches
    c.l1.Store(cacheKey, node)
    
    if nodeJSON, err := json.Marshal(node); err == nil {
        c.l2.Set(ctx, cacheKey, nodeJSON, 5*time.Minute)
    }
    
    return node, nil
}

func (c *CacheManager) InvalidateNode(ctx context.Context, nodeID shared.NodeID) error {
    cacheKey := fmt.Sprintf("node:%s", nodeID.String())
    
    // Remove from all cache levels
    c.l1.Delete(cacheKey)
    c.l2.Del(ctx, cacheKey)
    
    // Also invalidate related caches
    userCachePattern := fmt.Sprintf("user:*:nodes")
    c.l2.Del(ctx, userCachePattern)
    
    return nil
}
```

### 3. Performance Monitoring

```go
// Custom metrics for performance tracking
type PerformanceMonitor struct {
    cloudWatch *cloudwatch.Client
    logger     *zap.Logger
}

func (m *PerformanceMonitor) TrackOperation(ctx context.Context, operation string, fn func() error) error {
    start := time.Now()
    
    // Track operation
    err := fn()
    duration := time.Since(start)
    
    // Record metrics
    m.recordDuration(operation, duration)
    m.recordSuccess(operation, err == nil)
    
    // Log if slow
    if duration > 1*time.Second {
        m.logger.Warn("Slow operation detected",
            zap.String("operation", operation),
            zap.Duration("duration", duration),
            zap.Error(err),
        )
    }
    
    return err
}

func (m *PerformanceMonitor) recordDuration(operation string, duration time.Duration) {
    metric := &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("Brain2/Performance"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String("OperationDuration"),
                Value:      aws.Float64(duration.Seconds()),
                Unit:       types.StandardUnitSeconds,
                Dimensions: []types.Dimension{
                    {
                        Name:  aws.String("Operation"),
                        Value: aws.String(operation),
                    },
                },
                Timestamp: aws.Time(time.Now()),
            },
        },
    }
    
    m.cloudWatch.PutMetricData(context.Background(), metric)
}

// Usage in handlers
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    err := h.monitor.TrackOperation(r.Context(), "create_node", func() error {
        // Business logic here
        return h.nodeService.CreateNode(r.Context(), cmd)
    })
    
    if err != nil {
        errors.WriteError(w, err)
        return
    }
    
    // Success response
}
```

---

## Documentation & Resources

### 1. Architecture Decision Records (ADRs)

#### ADR Template
```markdown
# ADR-XXX: [Decision Title]

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
What is the issue that we're seeing that is motivating this decision or change?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult to do because of this change?

### Positive Consequences
- [Benefit 1]
- [Benefit 2]

### Negative Consequences  
- [Cost 1]
- [Cost 2]

### Neutral Consequences
- [Consideration 1]
- [Consideration 2]

## References
- [Link to relevant documentation]
- [Related ADRs]
```

### 2. Learning Resources

#### Go Advanced Patterns
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Google Wire Documentation](https://github.com/google/wire)
- [Clean Architecture in Go](https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1)
- [Go Concurrency Patterns](https://blog.golang.org/pipelines)

#### Domain-Driven Design
- [Domain-Driven Design by Eric Evans](https://www.domainlanguage.com/ddd/)
- [Implementing Domain-Driven Design by Vaughn Vernon](https://vaughnvernon.com/iddd/)
- [DDD Aggregate Pattern](https://martinfowler.com/bliki/DDD_Aggregate.html)
- [Event Sourcing Pattern](https://martinfowler.com/eaaDev/EventSourcing.html)

#### CQRS and Event-Driven Architecture
- [CQRS Journey by Microsoft](https://docs.microsoft.com/en-us/previous-versions/msp-n-p/jj554200(v=pandp.10))
- [Event Storming Workshop](https://www.eventstorming.com/)
- [Saga Pattern Implementation](https://microservices.io/patterns/data/saga.html)
- [Event-Driven Architecture Patterns](https://serverlessland.com/event-driven-architecture)

#### AWS Serverless Best Practices
- [AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [Serverless Application Lens](https://docs.aws.amazon.com/wellarchitected/latest/serverless-applications-lens/welcome.html)
- [CDK Best Practices](https://docs.aws.amazon.com/cdk/v2/guide/best-practices.html)

### 3. Development Tools

#### Recommended VSCode Extensions
```json
{
  "recommendations": [
    "golang.go",
    "ms-vscode.vscode-json",
    "amazonwebservices.aws-toolkit-vscode",
    "github.copilot",
    "ms-azuretools.vscode-docker",
    "redhat.vscode-yaml",
    "bradlc.vscode-tailwindcss"
  ]
}
```

#### Makefile for Common Tasks
```makefile
# Makefile for Brain2 Backend Development

.PHONY: help build test lint deploy clean

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all Lambda functions
	cd backend && ./build.sh

test: ## Run all tests
	cd backend && go test -v -race -cover ./...

test-integration: ## Run integration tests
	cd backend && go test -v -tags=integration ./tests/integration/...

test-e2e: ## Run end-to-end tests
	cd backend && go test -v -tags=e2e ./tests/e2e/...

lint: ## Run linting
	cd backend && golangci-lint run ./...

generate: ## Generate code (Wire, mocks, etc.)
	cd backend && go generate ./...

deploy: build ## Deploy to AWS
	cd infra && npx cdk deploy --all

deploy-dev: build ## Deploy to development environment
	cd infra && npx cdk deploy --all --profile dev

clean: ## Clean build artifacts
	cd backend && rm -rf build/
	cd infra && rm -rf cdk.out/

format: ## Format code
	cd backend && go fmt ./...
	cd backend && goimports -w .

deps: ## Update dependencies
	cd backend && go mod tidy
	cd infra && npm update

docker-build: ## Build Docker image for local testing
	docker build -t brain2-backend ./backend

docker-run: docker-build ## Run backend in Docker
	docker run -p 8080:8080 brain2-backend

metrics: ## View CloudWatch metrics
	aws cloudwatch get-metric-statistics \
		--namespace "Brain2/Backend" \
		--metric-name "OperationDuration" \
		--start-time $(shell date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
		--end-time $(shell date -u +%Y-%m-%dT%H:%M:%S) \
		--period 300 \
		--statistics Average,Maximum

logs: ## View Lambda logs
	aws logs filter-log-events \
		--log-group-name "/aws/lambda/brain2-dev-compute-BackendLambda" \
		--start-time $(shell date -d '1 hour ago' +%s)000

.DEFAULT_GOAL := help
```

---

## Conclusion

This evaluation reveals a **sophisticated but over-engineered backend architecture** that provides excellent learning opportunities while requiring pragmatic refinement. The implementation demonstrates advanced enterprise patterns (DDD, CQRS, Clean Architecture) but applies them to use cases that don't fully justify their complexity.

### Key Takeaways

1. **Strong Foundation**: The architecture demonstrates proper layering, dependency injection, and domain modeling
2. **Learning Value**: Excellent examples of enterprise patterns with real-world complexity
3. **Practical Issues**: Deployment reliability, testing gaps, and performance concerns need immediate attention
4. **Refactoring Opportunity**: Phased approach to simplify while maintaining educational value

### Recommended Path Forward

**Phase 1 (Critical)**: Fix deployment issues and add comprehensive testing  
**Phase 2 (Architecture)**: Simplify domain model and complete CQRS migration  
**Phase 3 (Learning)**: Implement advanced patterns like Event Sourcing and Saga  
**Phase 4 (Production)**: Optimize performance and add comprehensive monitoring

This roadmap balances practical development needs with learning objectives, ensuring the codebase remains valuable for understanding enterprise software patterns while being maintainable and performant.

---

**Total Assessment: B+ (Good with Clear Improvement Path)**
- Architecture Design: A-
- Implementation Quality: B
- Testing Coverage: C-
- Performance: C+
- Learning Value: A+