# Common Developer Tasks Guide

This guide provides step-by-step instructions for the most common development tasks in the Brain2 backend. Each section includes complete examples and explains how changes flow through all architectural layers.

## Table of Contents
- [Adding a New API Endpoint](#adding-a-new-api-endpoint)
- [Creating a New Domain Entity](#creating-a-new-domain-entity)
- [Adding a New Lambda Function](#adding-a-new-lambda-function)
- [Working with Wire Dependency Injection](#working-with-wire-dependency-injection)
- [Database Schema Changes](#database-schema-changes)
- [Testing New Features](#testing-new-features)
- [Understanding Layer Flow](#understanding-layer-flow)

---

## Adding a New API Endpoint

This example shows how to add a new endpoint `/api/v1/memories/{id}/tags` for managing memory tags.

### Step 1: Define Domain Logic (Domain Layer)

First, add the business logic to your domain entity:

```go
// internal/domain/node/node.go - Add to existing Node entity
func (n *Node) AddTag(tag shared.Tag) error {
    if n.archived {
        return shared.ErrCannotUpdateArchivedNode
    }
    
    // Business rule: max 10 tags per node
    if len(n.tags.ToSlice()) >= 10 {
        return shared.NewBusinessRuleError("max_tags_exceeded", "Node", "cannot exceed 10 tags")
    }
    
    newTags := n.tags.Add(tag)
    n.tags = newTags
    n.updatedAt = time.Now()
    n.version = n.version.Next()
    
    // Generate domain event
    event := shared.NewTagAddedEvent(n.id, n.userID, tag, n.version)
    n.addEvent(event)
    
    return nil
}

func (n *Node) RemoveTag(tag shared.Tag) error {
    if n.archived {
        return shared.ErrCannotUpdateArchivedNode
    }
    
    newTags := n.tags.Remove(tag)
    if newTags.Equals(n.tags) {
        return nil // Tag didn't exist, no change
    }
    
    n.tags = newTags
    n.updatedAt = time.Now()
    n.version = n.version.Next()
    
    // Generate domain event
    event := shared.NewTagRemovedEvent(n.id, n.userID, tag, n.version)
    n.addEvent(event)
    
    return nil
}
```

### Step 2: Create Application Commands (Application Layer)

```go
// internal/application/commands/tag_commands.go
package commands

type AddTagCommand struct {
    NodeID        string `json:"nodeId" validate:"required"`
    UserID        string `json:"userId" validate:"required"`
    Tag           string `json:"tag" validate:"required,min=1,max=50"`
    IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type RemoveTagCommand struct {
    NodeID        string `json:"nodeId" validate:"required"`
    UserID        string `json:"userId" validate:"required"`
    Tag           string `json:"tag" validate:"required"`
    IdempotencyKey string `json:"idempotencyKey,omitempty"`
}
```

### Step 3: Implement Application Service (Application Layer)

```go
// internal/application/services/node_service.go - Add to existing NodeService

func (s *NodeService) AddTag(ctx context.Context, cmd *commands.AddTagCommand) error {
    // Start tracing span
    ctx, span := s.tracer.Start(ctx, "NodeService.AddTag")
    defer span.End()
    
    // Create value objects
    nodeID, err := shared.ParseNodeID(cmd.NodeID)
    if err != nil {
        return errors.ValidationError("nodeId", "invalid node ID format", cmd.NodeID)
    }
    
    userID, err := shared.NewUserID(cmd.UserID)
    if err != nil {
        return errors.ValidationError("userId", "invalid user ID format", cmd.UserID)
    }
    
    tag, err := shared.NewTag(cmd.Tag)
    if err != nil {
        return errors.ValidationError("tag", "invalid tag format", cmd.Tag)
    }
    
    // Start unit of work
    uow, err := s.uowFactory.NewUnitOfWork(ctx)
    if err != nil {
        return errors.InternalError("failed to start transaction", err)
    }
    defer uow.Rollback() // Rollback if not committed
    
    // Get the node
    node, err := s.nodeRepo.FindByID(ctx, userID, nodeID)
    if err != nil {
        return errors.ApplicationError(ctx, "GetNode", err)
    }
    
    // Apply business logic
    if err := node.AddTag(tag); err != nil {
        return errors.ApplicationError(ctx, "AddTag", err)
    }
    
    // Save changes
    if err := s.nodeRepo.Save(ctx, node); err != nil {
        return errors.ApplicationError(ctx, "SaveNode", err)
    }
    
    // Publish events
    for _, event := range node.GetUncommittedEvents() {
        if err := s.eventBus.Publish(ctx, event); err != nil {
            return errors.ApplicationError(ctx, "PublishEvent", err)
        }
    }
    
    // Commit transaction
    if err := uow.Commit(); err != nil {
        return errors.InternalError("failed to commit transaction", err)
    }
    
    node.MarkEventsAsCommitted()
    return nil
}

func (s *NodeService) RemoveTag(ctx context.Context, cmd *commands.RemoveTagCommand) error {
    // Similar implementation...
}
```

### Step 4: Create DTOs (Interface Layer)

```go
// internal/interfaces/http/v1/dto/tag_dto.go
package dto

type AddTagRequest struct {
    Tag string `json:"tag" validate:"required,min=1,max=50"`
}

type RemoveTagRequest struct {
    Tag string `json:"tag" validate:"required"`
}

type TagResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message,omitempty"`
}
```

### Step 5: Implement HTTP Handler (Interface Layer)

```go
// internal/interfaces/http/v1/handlers/memory.go - Add to existing MemoryHandler

func (h *MemoryHandler) AddTag(w http.ResponseWriter, r *http.Request) {
    // Extract path parameters
    nodeID := chi.URLParam(r, "id")
    if nodeID == "" {
        http.Error(w, "node ID is required", http.StatusBadRequest)
        return
    }
    
    // Get user ID from context (set by auth middleware)
    userID, ok := r.Context().Value("user_id").(string)
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Parse request body
    var req dto.AddTagRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
    
    // Validate request
    if err := h.validator.Struct(&req); err != nil {
        http.Error(w, fmt.Sprintf("validation failed: %v", err), http.StatusBadRequest)
        return
    }
    
    // Generate idempotency key
    idempotencyKey := generateIdempotencyKey(r)
    
    // Create command
    cmd := &commands.AddTagCommand{
        NodeID:         nodeID,
        UserID:         userID,
        Tag:            req.Tag,
        IdempotencyKey: idempotencyKey,
    }
    
    // Execute command
    if err := h.nodeService.AddTag(r.Context(), cmd); err != nil {
        handleServiceError(w, err)
        return
    }
    
    // Success response
    response := dto.TagResponse{
        Success: true,
        Message: "Tag added successfully",
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(response)
}

func (h *MemoryHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
    // Similar implementation...
}
```

### Step 6: Add Routes (Interface Layer)

```go
// internal/interfaces/http/v1/handlers/memory.go - Add to route configuration

func (h *MemoryHandler) Routes() chi.Router {
    r := chi.NewRouter()
    
    // Existing routes...
    r.Get("/{id}", h.GetMemory)
    r.Put("/{id}", h.UpdateMemory)
    r.Delete("/{id}", h.DeleteMemory)
    
    // New tag routes
    r.Post("/{id}/tags", h.AddTag)
    r.Delete("/{id}/tags", h.RemoveTag)
    
    return r
}
```

### Step 7: Update Wire Providers (DI Layer)

```go
// internal/di/wire_providers.go - No changes needed!
// The existing NodeService provider already covers the new methods
```

### Step 8: Add Tests

```go
// internal/domain/node/node_test.go
func TestNode_AddTag(t *testing.T) {
    node := createTestNode(t)
    tag, _ := shared.NewTag("important")
    
    err := node.AddTag(tag)
    
    assert.NoError(t, err)
    assert.True(t, node.Tags().Contains("important"))
    assert.Len(t, node.GetUncommittedEvents(), 1)
}

// internal/application/services/node_service_test.go
func TestNodeService_AddTag(t *testing.T) {
    // Mock setup...
    service := setupTestNodeService(t, mockRepo, mockEventBus)
    
    cmd := &commands.AddTagCommand{
        NodeID: "node-123",
        UserID: "user-456", 
        Tag:    "important",
    }
    
    err := service.AddTag(context.Background(), cmd)
    
    assert.NoError(t, err)
    // Verify mocks were called correctly...
}

// internal/interfaces/http/v1/handlers/memory_test.go
func TestMemoryHandler_AddTag(t *testing.T) {
    // HTTP test setup...
    req := httptest.NewRequest("POST", "/memories/123/tags", strings.NewReader(`{"tag": "important"}`))
    w := httptest.NewRecorder()
    
    handler.AddTag(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### Step 9: Build and Test

```bash
# Generate Wire code (if you added new providers)
cd internal/di && wire

# Run tests
go test ./...

# Build application
./build.sh

# Test manually with curl
curl -X POST http://localhost:8080/api/v1/memories/123/tags \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{"tag": "important"}'
```

---

## Creating a New Domain Entity

This example shows how to create a new `Bookmark` entity.

### Step 1: Create Domain Entity

```go
// internal/domain/bookmark/bookmark.go
package bookmark

import (
    "time"
    "brain2-backend/internal/domain/shared"
)

type Bookmark struct {
    shared.BaseAggregateRoot
    
    id          shared.BookmarkID
    userID      shared.UserID
    nodeID      shared.NodeID
    title       shared.Title
    url         shared.URL
    createdAt   time.Time
    updatedAt   time.Time
    version     shared.Version
}

func NewBookmark(userID shared.UserID, nodeID shared.NodeID, title shared.Title, url shared.URL) (*Bookmark, error) {
    // Validation
    if err := url.Validate(); err != nil {
        return nil, shared.NewDomainError("invalid_url", "bookmark URL validation failed", err)
    }
    
    now := time.Now()
    bookmarkID := shared.NewBookmarkID()
    
    bookmark := &Bookmark{
        BaseAggregateRoot: shared.NewBaseAggregateRoot(bookmarkID.String()),
        id:        bookmarkID,
        userID:    userID,
        nodeID:    nodeID,
        title:     title,
        url:       url,
        createdAt: now,
        updatedAt: now,
        version:   shared.NewVersion(),
    }
    
    // Generate domain event
    event := shared.NewBookmarkCreatedEvent(bookmarkID, userID, nodeID, url, bookmark.version)
    bookmark.AddEvent(event)
    
    return bookmark, nil
}

// Getters
func (b *Bookmark) ID() shared.BookmarkID { return b.id }
func (b *Bookmark) UserID() shared.UserID { return b.userID }
func (b *Bookmark) NodeID() shared.NodeID { return b.nodeID }
func (b *Bookmark) Title() shared.Title { return b.title }
func (b *Bookmark) URL() shared.URL { return b.url }
func (b *Bookmark) CreatedAt() time.Time { return b.createdAt }
func (b *Bookmark) UpdatedAt() time.Time { return b.updatedAt }
func (b *Bookmark) Version() int { return b.version.Int() }

// Business methods
func (b *Bookmark) UpdateTitle(newTitle shared.Title) error {
    if b.title.Equals(newTitle) {
        return nil // No change
    }
    
    oldTitle := b.title
    b.title = newTitle
    b.updatedAt = time.Now()
    b.version = b.version.Next()
    
    event := shared.NewBookmarkTitleUpdatedEvent(b.id, b.userID, oldTitle, newTitle, b.version)
    b.AddEvent(event)
    
    return nil
}
```

### Step 2: Add Value Objects

```go
// internal/domain/shared/value_objects.go - Add new value objects

// BookmarkID represents a unique bookmark identifier
type BookmarkID struct {
    value string
}

func NewBookmarkID() BookmarkID {
    return BookmarkID{value: uuid.New().String()}
}

func ParseBookmarkID(value string) (BookmarkID, error) {
    if value == "" {
        return BookmarkID{}, NewValidationError("id", "bookmark ID cannot be empty", value)
    }
    return BookmarkID{value: value}, nil
}

func (id BookmarkID) String() string { return id.value }
func (id BookmarkID) IsEmpty() bool  { return id.value == "" }
func (id BookmarkID) Equals(other BookmarkID) bool { return id.value == other.value }

// URL represents a valid URL
type URL struct {
    value string
}

func NewURL(value string) (URL, error) {
    if value == "" {
        return URL{}, NewValidationError("url", "URL cannot be empty", value)
    }
    
    // Validate URL format
    if !isValidURL(value) {
        return URL{}, NewValidationError("url", "invalid URL format", value)
    }
    
    return URL{value: value}, nil
}

func (u URL) String() string { return u.value }
func (u URL) Validate() error {
    if !isValidURL(u.value) {
        return NewValidationError("url", "invalid URL format", u.value)
    }
    return nil
}

func isValidURL(str string) bool {
    // URL validation logic
    return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}
```

### Step 3: Create Repository Interface

```go
// internal/repository/interfaces.go - Add to existing interfaces

type BookmarkRepository interface {
    Save(ctx context.Context, bookmark *bookmark.Bookmark) error
    FindByID(ctx context.Context, userID shared.UserID, id shared.BookmarkID) (*bookmark.Bookmark, error)
    FindByUserID(ctx context.Context, userID shared.UserID, opts ...QueryOption) ([]*bookmark.Bookmark, error)
    FindByNodeID(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) ([]*bookmark.Bookmark, error)
    Delete(ctx context.Context, userID shared.UserID, id shared.BookmarkID) error
}
```

### Step 4: Implement Repository

```go
// internal/infrastructure/persistence/dynamodb/bookmark_repository.go
package dynamodb

import (
    "context"
    "brain2-backend/internal/domain/bookmark"
    "brain2-backend/internal/domain/shared"
    "brain2-backend/internal/repository"
)

type BookmarkRepositoryV2 struct {
    *GenericRepository[*bookmark.Bookmark]
}

func NewBookmarkRepositoryV2(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *BookmarkRepositoryV2 {
    return &BookmarkRepositoryV2{
        GenericRepository: CreateBookmarkRepository(client, tableName, indexName, logger),
    }
}

// Domain-specific queries
func (r *BookmarkRepositoryV2) FindByNodeID(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) ([]*bookmark.Bookmark, error) {
    return r.Query(ctx, userID.String(),
        WithSKPrefix("BOOKMARK#"),
        WithFilter(
            expression.Equal(expression.Name("NodeID"), expression.Value(nodeID.String())),
        ),
    )
}

// Factory function for GenericRepository
func CreateBookmarkRepository(client *dynamodb.Client, tableName, indexName string, logger *zap.Logger) *GenericRepository[*bookmark.Bookmark] {
    config := &BookmarkEntityConfig{}
    return NewGenericRepository[*bookmark.Bookmark](client, tableName, indexName, config, logger)
}

// Entity configuration for GenericRepository
type BookmarkEntityConfig struct{}

func (c *BookmarkEntityConfig) ParseItem(item map[string]types.AttributeValue) (*bookmark.Bookmark, error) {
    // Parse DynamoDB item to Bookmark entity
    // Implementation details...
}

func (c *BookmarkEntityConfig) ToItem(b *bookmark.Bookmark) (map[string]types.AttributeValue, error) {
    // Convert Bookmark entity to DynamoDB item
    // Implementation details...
}

func (c *BookmarkEntityConfig) BuildKey(userID, entityID string) map[string]types.AttributeValue {
    return map[string]types.AttributeValue{
        "PK": &types.AttributeValueMemberS{Value: userID},
        "SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("BOOKMARK#%s", entityID)},
    }
}

func (c *BookmarkEntityConfig) GetEntityType() string { return "BOOKMARK" }
func (c *BookmarkEntityConfig) GetID(b *bookmark.Bookmark) string { return b.ID().String() }
func (c *BookmarkEntityConfig) GetUserID(b *bookmark.Bookmark) string { return b.UserID().String() }
func (c *BookmarkEntityConfig) GetVersion(b *bookmark.Bookmark) int { return b.Version() }
```

### Step 5: Create Application Service

```go
// internal/application/services/bookmark_service.go
package services

type BookmarkService struct {
    bookmarkRepo     repository.BookmarkRepository
    nodeRepo         repository.NodeRepository
    uowFactory       repository.UnitOfWorkFactory
    eventBus         shared.EventBus
    tracer           trace.Tracer
}

func NewBookmarkService(
    bookmarkRepo repository.BookmarkRepository,
    nodeRepo repository.NodeRepository,
    uowFactory repository.UnitOfWorkFactory,
    eventBus shared.EventBus,
) *BookmarkService {
    return &BookmarkService{
        bookmarkRepo: bookmarkRepo,
        nodeRepo:     nodeRepo,
        uowFactory:   uowFactory,
        eventBus:     eventBus,
        tracer:       otel.Tracer("brain2-backend.application.bookmark_service"),
    }
}

func (s *BookmarkService) CreateBookmark(ctx context.Context, cmd *commands.CreateBookmarkCommand) (*dto.CreateBookmarkResult, error) {
    // Implementation similar to NodeService patterns...
}
```

### Step 6: Add Wire Providers

```go
// internal/di/wire_providers.go - Add new providers

// Repository provider
func provideBookmarkRepository(client *dynamodb.Client, config *config.Config, logger *zap.Logger, repositoryFactory *RepositoryFactory, cache Cache, collector MetricsCollector) repository.BookmarkRepository {
    return dynamodb.NewBookmarkRepositoryV2(client, config.Database.TableName, config.Database.IndexName, logger)
}

// Service provider
func provideBookmarkService(
    bookmarkRepo repository.BookmarkRepository,
    nodeRepo repository.NodeRepository,
    uowFactory repository.UnitOfWorkFactory,
    eventBus shared.EventBus,
) *services.BookmarkService {
    return services.NewBookmarkService(bookmarkRepo, nodeRepo, uowFactory, eventBus)
}
```

### Step 7: Update Wire Sets

```go
// internal/di/wire_sets.go - Add to existing sets

var SuperSet = wire.NewSet(
    // Config and infrastructure
    ConfigSet,
    InfrastructureSet,
    
    // Repositories
    RepositorySet,
    provideBookmarkRepository, // Add new repository
    
    // Services
    ServiceSet,
    provideBookmarkService,    // Add new service
    
    // Handlers and router
    HandlerSet,
    provideContainer,
)
```

### Step 8: Generate Wire Code

```bash
# Navigate to DI directory
cd internal/di

# Generate Wire code
wire

# Verify no errors
wire check
```

---

## Adding a New Lambda Function

This example shows how to create a new Lambda function for processing bookmarks asynchronously.

### Step 1: Create Lambda Handler

```go
// cmd/bookmark-processor/main.go
package main

import (
    "context"
    "encoding/json"
    "log"

    "brain2-backend/internal/di"
    "brain2-backend/internal/domain/shared"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

var container *di.Container

func init() {
    var err error
    container, err = di.InitializeContainer()
    if err != nil {
        log.Fatalf("Failed to initialize container: %v", err)
    }
    log.Println("Bookmark processor initialized successfully")
}

type BookmarkEvent struct {
    BookmarkID string `json:"bookmarkId"`
    UserID     string `json:"userId"`
    URL        string `json:"url"`
    Action     string `json:"action"`
}

func handler(ctx context.Context, event events.SQSEvent) error {
    bookmarkService := container.GetBookmarkService()
    
    for _, record := range event.Records {
        var bookmarkEvent BookmarkEvent
        if err := json.Unmarshal([]byte(record.Body), &bookmarkEvent); err != nil {
            log.Printf("Error unmarshaling event: %v", err)
            continue
        }
        
        switch bookmarkEvent.Action {
        case "extract_metadata":
            if err := extractBookmarkMetadata(ctx, bookmarkService, &bookmarkEvent); err != nil {
                log.Printf("Error processing bookmark: %v", err)
            }
        case "validate_url":
            if err := validateBookmarkURL(ctx, bookmarkService, &bookmarkEvent); err != nil {
                log.Printf("Error validating URL: %v", err)
            }
        }
    }
    
    return nil
}

func extractBookmarkMetadata(ctx context.Context, service *services.BookmarkService, event *BookmarkEvent) error {
    // Extract metadata from URL (title, description, etc.)
    // Update bookmark with extracted metadata
    return nil
}

func validateBookmarkURL(ctx context.Context, service *services.BookmarkService, event *BookmarkEvent) error {
    // Validate that URL is still accessible
    // Mark bookmark as invalid if not accessible
    return nil
}

func main() {
    lambda.Start(handler)
}
```

### Step 2: Update Build Script

The existing `build.sh` will automatically detect the new Lambda function in `cmd/bookmark-processor/` and build it. No changes needed!

### Step 3: Update CDK Infrastructure

```typescript
// infra/lib/brain2-stack.ts - Add new Lambda function

const bookmarkProcessorFunction = new lambda.Function(this, 'BookmarkProcessor', {
    runtime: lambda.Runtime.PROVIDED_AL2,
    code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/bookmark-processor')),
    handler: 'bootstrap',
    timeout: Duration.minutes(5),
    memorySize: 512,
    environment: {
        TABLE_NAME: table.tableName,
        INDEX_NAME: gsiName,
        EVENT_BUS_NAME: eventBridge.eventBusName,
    },
});

// Grant permissions
table.grantReadWriteData(bookmarkProcessorFunction);
eventBridge.grantPutEventsTo(bookmarkProcessorFunction);

// Add SQS trigger
const bookmarkQueue = new sqs.Queue(this, 'BookmarkQueue', {
    deadLetterQueue: {
        queue: dlq,
        maxReceiveCount: 3,
    },
});

bookmarkProcessorFunction.addEventSource(new SqsEventSource(bookmarkQueue, {
    batchSize: 10,
}));
```

### Step 4: Build and Deploy

```bash
# Build new Lambda function
./build.sh --component bookmark-processor

# Deploy infrastructure
cd ../infra
npx cdk deploy
```

---

## Working with Wire Dependency Injection

### Common Wire Patterns

#### Adding a New Service Provider

```go
// internal/di/wire_providers.go

func provideMyNewService(
    dependency1 Dependency1,
    dependency2 Dependency2,
    config *config.Config,
) *services.MyNewService {
    return services.NewMyNewService(dependency1, dependency2, config.MyNewServiceConfig)
}
```

#### Adding Dependencies to Existing Service

```go
// If you need to add a new dependency to an existing service:

// 1. Update the service constructor
func NewNodeService(
    nodeRepo repository.NodeRepository,
    edgeRepo repository.EdgeRepository,
    uowFactory repository.UnitOfWorkFactory,
    eventBus shared.EventBus,
    connectionAnalyzer *domainServices.ConnectionAnalyzer,
    idempotencyStore repository.IdempotencyStore,
    // Add new dependency here
    newDependency NewDependencyInterface,
) *NodeService {
    return &NodeService{
        nodeRepo:           nodeRepo,
        edgeRepo:           edgeRepo,
        uowFactory:         uowFactory,
        eventBus:           eventBus,
        connectionAnalyzer: connectionAnalyzer,
        idempotencyStore:   idempotencyStore,
        newDependency:      newDependency, // Add here
        tracer:             otel.Tracer("brain2-backend.application.node_service"),
    }
}

// 2. Update the Wire provider (if it exists)
func provideNodeService(
    nodeRepo repository.NodeRepository,
    edgeRepo repository.EdgeRepository,
    uowFactory repository.UnitOfWorkFactory,
    eventBus shared.EventBus,
    connectionAnalyzer *domainServices.ConnectionAnalyzer,
    idempotencyStore repository.IdempotencyStore,
    newDependency NewDependencyInterface, // Add here
) *services.NodeService {
    return services.NewNodeService(nodeRepo, edgeRepo, uowFactory, eventBus, connectionAnalyzer, idempotencyStore, newDependency)
}

// 3. Regenerate Wire code
// cd internal/di && wire
```

#### Creating Provider Groups

```go
// internal/di/wire_sets.go

var RepositorySet = wire.NewSet(
    provideNodeRepository,
    provideEdgeRepository,
    provideCategoryRepository,
    provideKeywordRepository,
    provideBookmarkRepository, // Add new repositories here
)

var ServiceSet = wire.NewSet(
    provideNodeService,
    provideCategoryAppService,
    provideCleanupService,
    provideBookmarkService,   // Add new services here
)
```

#### Interface Bindings

```go
// If you need to bind an implementation to an interface:

// internal/di/wire_providers.go
func provideMyInterface(impl *ConcreteImplementation) MyInterface {
    return impl
}

// Or use wire.Bind in sets:
var InterfaceBindings = wire.NewSet(
    wire.Bind(new(MyInterface), new(*ConcreteImplementation)),
)
```

### Wire Generation Process

```bash
# Navigate to DI directory
cd internal/di

# Check Wire configuration (optional but recommended)
wire check

# Generate dependency injection code
wire

# The generated wire_gen.go file will contain:
# - InitializeContainer() function with all dependencies wired
# - All provider functions called in correct order
# - Compile-time error if dependencies are missing or circular

# Verify the build still works
cd ../..
go build ./cmd/main/main.go
```

---

## Database Schema Changes

### Adding New Fields to Existing Entity

#### Step 1: Update Domain Entity

```go
// internal/domain/node/node.go
type Node struct {
    shared.BaseAggregateRoot
    
    // Existing fields...
    id        shared.NodeID
    content   shared.Content
    title     shared.Title
    
    // New field
    priority  shared.Priority  // Add new field
    
    // Rest of fields...
    createdAt time.Time
    updatedAt time.Time
}

// Add getter
func (n *Node) Priority() shared.Priority {
    return n.priority
}

// Add business method
func (n *Node) SetPriority(newPriority shared.Priority) error {
    if n.archived {
        return shared.ErrCannotUpdateArchivedNode
    }
    
    oldPriority := n.priority
    n.priority = newPriority
    n.updatedAt = time.Now()
    n.version = n.version.Next()
    
    event := shared.NewNodePriorityUpdatedEvent(n.id, n.userID, oldPriority, newPriority, n.version)
    n.addEvent(event)
    
    return nil
}
```

#### Step 2: Update Value Object

```go
// internal/domain/shared/value_objects.go
type Priority int

const (
    PriorityLow    Priority = 1
    PriorityMedium Priority = 2
    PriorityHigh   Priority = 3
)

func NewPriority(value int) (Priority, error) {
    if value < 1 || value > 3 {
        return Priority(0), NewValidationError("priority", "priority must be between 1 and 3", value)
    }
    return Priority(value), nil
}

func (p Priority) Int() int { return int(p) }
func (p Priority) String() string {
    switch p {
    case PriorityLow:
        return "low"
    case PriorityMedium:
        return "medium"
    case PriorityHigh:
        return "high"
    default:
        return "unknown"
    }
}
```

#### Step 3: Update Repository Implementation

```go
// internal/infrastructure/persistence/dynamodb/node_repository_refactored.go

// Update the entity configuration
type NodeEntityConfig struct{}

func (c *NodeEntityConfig) ParseItem(item map[string]types.AttributeValue) (*node.Node, error) {
    // Existing parsing logic...
    
    // Add new field parsing with backward compatibility
    var priority shared.Priority
    if priorityAttr, exists := item["Priority"]; exists {
        if priorityValue, err := strconv.Atoi(*priorityAttr.(*types.AttributeValueMemberN).Value); err == nil {
            priority, _ = shared.NewPriority(priorityValue)
        }
    }
    // If Priority field doesn't exist, use default
    if priority == 0 {
        priority = shared.PriorityMedium // Default value for backward compatibility
    }
    
    return node.ReconstructNodeWithPriority(
        nodeID, userID, content, title, keywords, tags,
        createdAt, updatedAt, version, archived, priority,
    ), nil
}

func (c *NodeEntityConfig) ToItem(n *node.Node) (map[string]types.AttributeValue, error) {
    item := map[string]types.AttributeValue{
        "PK":        &types.AttributeValueMemberS{Value: n.UserID().String()},
        "SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", n.ID().String())},
        "Content":   &types.AttributeValueMemberS{Value: n.Content().String()},
        "Title":     &types.AttributeValueMemberS{Value: n.Title().String()},
        "Keywords":  &types.AttributeValueMemberSS{Value: n.Keywords().ToSlice()},
        "Tags":      &types.AttributeValueMemberSS{Value: n.Tags().ToSlice()},
        "CreatedAt": &types.AttributeValueMemberS{Value: n.CreatedAt().Format(time.RFC3339)},
        "UpdatedAt": &types.AttributeValueMemberS{Value: n.UpdatedAt().Format(time.RFC3339)},
        "Version":   &types.AttributeValueMemberN{Value: strconv.Itoa(n.Version())},
        "Archived":  &types.AttributeValueMemberBOOL{Value: n.IsArchived()},
        "Priority":  &types.AttributeValueMemberN{Value: strconv.Itoa(n.Priority().Int())}, // Add new field
    }
    
    return item, nil
}
```

#### Step 4: Update Factory Methods

```go
// internal/domain/node/node.go

// Update NewNode factory
func NewNode(userID shared.UserID, content shared.Content, title shared.Title, tags shared.Tags) (*Node, error) {
    // Existing validation...
    
    node := &Node{
        BaseAggregateRoot: shared.NewBaseAggregateRoot(nodeID.String()),
        id:        nodeID,
        userID:    userID,
        content:   content,
        title:     title,
        keywords:  keywords,
        tags:      tags,
        priority:  shared.PriorityMedium, // Default value
        createdAt: now,
        updatedAt: now,
        version:   shared.NewVersion(),
        archived:  false,
        metadata:  make(map[string]interface{}),
        events:    []shared.DomainEvent{},
    }
    
    return node, nil
}

// Add new reconstruction method with priority
func ReconstructNodeWithPriority(id shared.NodeID, userID shared.UserID, content shared.Content, title shared.Title, keywords shared.Keywords, tags shared.Tags,
    createdAt, updatedAt time.Time, version shared.Version, archived bool, priority shared.Priority) *Node {
    return &Node{
        BaseAggregateRoot: shared.NewBaseAggregateRoot(id.String()),
        id:        id,
        userID:    userID,
        content:   content,
        title:     title,
        keywords:  keywords,
        tags:      tags,
        priority:  priority, // Include new field
        createdAt: createdAt,
        updatedAt: updatedAt,
        version:   version,
        archived:  archived,
        metadata:  make(map[string]interface{}),
        events:    []shared.DomainEvent{},
    }
}
```

### Migration Strategy

Since DynamoDB is schema-less, the migration is handled by:

1. **Backward Compatibility**: Repository parsing handles missing fields with defaults
2. **Forward Compatibility**: New fields are added to items when saving
3. **Gradual Migration**: Existing items get new fields when next updated

### Testing Schema Changes

```go
// test/migration_test.go
func TestNodePriorityMigration(t *testing.T) {
    // Test that old items without Priority field can be loaded
    oldItem := map[string]types.AttributeValue{
        "PK": &types.AttributeValueMemberS{Value: "user-123"},
        "SK": &types.AttributeValueMemberS{Value: "NODE#node-456"},
        // ... other fields but NO Priority field
    }
    
    config := &NodeEntityConfig{}
    node, err := config.ParseItem(oldItem)
    
    assert.NoError(t, err)
    assert.Equal(t, shared.PriorityMedium, node.Priority()) // Should use default
}

func TestNodePriorityPersistence(t *testing.T) {
    // Test that new Priority field is saved
    node := createTestNodeWithPriority(t, shared.PriorityHigh)
    
    config := &NodeEntityConfig{}
    item, err := config.ToItem(node)
    
    assert.NoError(t, err)
    assert.Contains(t, item, "Priority")
    assert.Equal(t, "3", item["Priority"].(*types.AttributeValueMemberN).Value)
}
```

---

## Testing New Features

### Unit Testing Domain Logic

```go
// internal/domain/bookmark/bookmark_test.go
package bookmark

import (
    "testing"
    "brain2-backend/internal/domain/shared"
    "github.com/stretchr/testify/assert"
)

func TestNewBookmark(t *testing.T) {
    userID, _ := shared.NewUserID("user-123")
    nodeID, _ := shared.ParseNodeID("node-456")
    title, _ := shared.NewTitle("Test Bookmark")
    url, _ := shared.NewURL("https://example.com")
    
    bookmark, err := NewBookmark(userID, nodeID, title, url)
    
    assert.NoError(t, err)
    assert.Equal(t, userID, bookmark.UserID())
    assert.Equal(t, nodeID, bookmark.NodeID())
    assert.Equal(t, title, bookmark.Title())
    assert.Equal(t, url, bookmark.URL())
    assert.Len(t, bookmark.GetUncommittedEvents(), 1)
}

func TestBookmark_UpdateTitle(t *testing.T) {
    bookmark := createTestBookmark(t)
    newTitle, _ := shared.NewTitle("Updated Title")
    
    err := bookmark.UpdateTitle(newTitle)
    
    assert.NoError(t, err)
    assert.Equal(t, newTitle, bookmark.Title())
    assert.Len(t, bookmark.GetUncommittedEvents(), 2) // Creation + Update events
}

func createTestBookmark(t *testing.T) *Bookmark {
    userID, _ := shared.NewUserID("user-123")
    nodeID, _ := shared.ParseNodeID("node-456")
    title, _ := shared.NewTitle("Test Bookmark")
    url, _ := shared.NewURL("https://example.com")
    
    bookmark, err := NewBookmark(userID, nodeID, title, url)
    assert.NoError(t, err)
    bookmark.MarkEventsAsCommitted() // Clear initial events for cleaner tests
    
    return bookmark
}
```

### Integration Testing with DynamoDB

```go
// internal/infrastructure/persistence/dynamodb/bookmark_repository_test.go
package dynamodb

import (
    "context"
    "testing"
    "brain2-backend/internal/domain/bookmark"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/suite"
)

type BookmarkRepositoryTestSuite struct {
    suite.Suite
    repo      *BookmarkRepositoryV2
    ctx       context.Context
    cleanup   func()
}

func (suite *BookmarkRepositoryTestSuite) SetupSuite() {
    // Setup test DynamoDB (local or testcontainers)
    suite.ctx = context.Background()
    suite.repo, suite.cleanup = setupTestBookmarkRepository(suite.T())
}

func (suite *BookmarkRepositoryTestSuite) TearDownSuite() {
    suite.cleanup()
}

func (suite *BookmarkRepositoryTestSuite) TestSaveAndFindBookmark() {
    bookmark := createTestBookmark(suite.T())
    
    // Save bookmark
    err := suite.repo.Save(suite.ctx, bookmark)
    suite.NoError(err)
    
    // Find bookmark
    found, err := suite.repo.FindByID(suite.ctx, bookmark.UserID(), bookmark.ID())
    suite.NoError(err)
    suite.Equal(bookmark.ID(), found.ID())
    suite.Equal(bookmark.Title(), found.Title())
    suite.Equal(bookmark.URL(), found.URL())
}

func (suite *BookmarkRepositoryTestSuite) TestFindByNodeID() {
    bookmark := createTestBookmark(suite.T())
    err := suite.repo.Save(suite.ctx, bookmark)
    suite.NoError(err)
    
    bookmarks, err := suite.repo.FindByNodeID(suite.ctx, bookmark.UserID(), bookmark.NodeID())
    suite.NoError(err)
    suite.Len(bookmarks, 1)
    suite.Equal(bookmark.ID(), bookmarks[0].ID())
}

func TestBookmarkRepositoryTestSuite(t *testing.T) {
    suite.Run(t, new(BookmarkRepositoryTestSuite))
}

func setupTestBookmarkRepository(t *testing.T) (*BookmarkRepositoryV2, func()) {
    // Setup test DynamoDB client (using localstack or testcontainers)
    // Return repository and cleanup function
}
```

### Service Layer Testing with Mocks

```go
// internal/application/services/bookmark_service_test.go
package services

import (
    "context"
    "testing"
    "brain2-backend/internal/application/commands"
    "brain2-backend/internal/domain/bookmark"
    "brain2-backend/internal/repository/mocks"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestBookmarkService_CreateBookmark(t *testing.T) {
    // Setup mocks
    mockBookmarkRepo := &mocks.MockBookmarkRepository{}
    mockNodeRepo := &mocks.MockNodeRepository{}
    mockUowFactory := &mocks.MockUnitOfWorkFactory{}
    mockUow := &mocks.MockUnitOfWork{}
    mockEventBus := &mocks.MockEventBus{}
    
    // Setup mock expectations
    mockUowFactory.On("NewUnitOfWork", mock.Anything).Return(mockUow, nil)
    mockNodeRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(createTestNode(t), nil)
    mockBookmarkRepo.On("Save", mock.Anything, mock.AnythingOfType("*bookmark.Bookmark")).Return(nil)
    mockEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)
    mockUow.On("Commit").Return(nil)
    mockUow.On("Rollback").Return(nil)
    
    // Create service
    service := NewBookmarkService(mockBookmarkRepo, mockNodeRepo, mockUowFactory, mockEventBus)
    
    // Execute command
    cmd := &commands.CreateBookmarkCommand{
        UserID: "user-123",
        NodeID: "node-456", 
        Title:  "Test Bookmark",
        URL:    "https://example.com",
    }
    
    result, err := service.CreateBookmark(context.Background(), cmd)
    
    // Verify results
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.NotEmpty(t, result.BookmarkID)
    
    // Verify all mocks were called
    mockBookmarkRepo.AssertExpectations(t)
    mockNodeRepo.AssertExpectations(t)
    mockUowFactory.AssertExpectations(t)
    mockEventBus.AssertExpectations(t)
}
```

### HTTP Handler Testing

```go
// internal/interfaces/http/v1/handlers/bookmark_handler_test.go
package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "brain2-backend/internal/interfaces/http/v1/dto"
    "brain2-backend/internal/application/services/mocks"
    "github.com/go-chi/chi/v5"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestBookmarkHandler_CreateBookmark(t *testing.T) {
    // Setup mock service
    mockService := &mocks.MockBookmarkService{}
    mockService.On("CreateBookmark", mock.Anything, mock.AnythingOfType("*commands.CreateBookmarkCommand")).
        Return(&dto.CreateBookmarkResult{BookmarkID: "bookmark-789"}, nil)
    
    // Create handler
    handler := NewBookmarkHandler(mockService, createTestValidator())
    
    // Create test request
    reqBody := dto.CreateBookmarkRequest{
        NodeID: "node-456",
        Title:  "Test Bookmark",
        URL:    "https://example.com",
    }
    bodyBytes, _ := json.Marshal(reqBody)
    
    req := httptest.NewRequest("POST", "/bookmarks", bytes.NewBuffer(bodyBytes))
    req.Header.Set("Content-Type", "application/json")
    
    // Add user context (normally done by auth middleware)
    ctx := context.WithValue(req.Context(), "user_id", "user-123")
    req = req.WithContext(ctx)
    
    // Execute request
    w := httptest.NewRecorder()
    handler.CreateBookmark(w, req)
    
    // Verify response
    assert.Equal(t, http.StatusCreated, w.Code)
    
    var response dto.CreateBookmarkResult
    err := json.NewDecoder(w.Body).Decode(&response)
    assert.NoError(t, err)
    assert.Equal(t, "bookmark-789", response.BookmarkID)
    
    // Verify mock was called
    mockService.AssertExpectations(t)
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test package
go test ./internal/domain/bookmark/...

# Run integration tests (if tagged)
go test -tags=integration ./...

# Run tests with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Understanding Layer Flow

### Request Flow Diagram

```
Client Request
      │
      ▼
┌─────────────────┐
│   HTTP Handler  │ ← Interface Layer (adapters)
│   (Interface)   │   • Parse HTTP request
│                 │   • Validate input
│                 │   • Extract user context
│                 │   • Convert to DTOs
└─────────────────┘
      │
      ▼
┌─────────────────┐
│ Application     │ ← Application Layer (use cases)
│ Service         │   • Orchestrate business operations
│                 │   • Start transactions
│                 │   • Convert DTOs to domain objects
│                 │   • Publish events
└─────────────────┘
      │
      ▼
┌─────────────────┐
│ Domain Entity   │ ← Domain Layer (business logic)
│ (Node, etc.)    │   • Enforce business rules
│                 │   • Generate domain events
│                 │   • Validate invariants
│                 │   • Encapsulate behavior
└─────────────────┘
      │
      ▼
┌─────────────────┐
│ Repository      │ ← Infrastructure Layer (external concerns)
│ (DynamoDB)      │   • Persist domain objects
│                 │   • Handle database operations
│                 │   • Convert to/from storage format
│                 │   • Manage transactions
└─────────────────┘
      │
      ▼
┌─────────────────┐
│ External        │
│ Systems         │
│ (DynamoDB,      │
│  EventBridge)   │
└─────────────────┘
```

### Data Flow Through Layers

#### 1. HTTP Request → Application Command

```go
// Interface Layer (Handler)
func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
    // Parse HTTP request
    var req dto.CreateMemoryRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Extract context
    userID := r.Context().Value("user_id").(string)
    
    // Convert to application command
    cmd := &commands.CreateNodeCommand{
        UserID:  userID,
        Content: req.Content,
        Title:   req.Title,
        Tags:    req.Tags,
    }
    
    // Call application layer
    result, err := h.nodeService.CreateNode(r.Context(), cmd)
}
```

#### 2. Application Command → Domain Operation

```go
// Application Layer (Service)
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // Convert to domain value objects
    userID, _ := shared.NewUserID(cmd.UserID)
    content, _ := shared.NewContent(cmd.Content)
    title, _ := shared.NewTitle(cmd.Title)
    tags := shared.NewTags(cmd.Tags...)
    
    // Create domain entity (business logic)
    node, err := node.NewNode(userID, content, title, tags)
    if err != nil {
        return nil, err
    }
    
    // Persist through repository
    if err := s.nodeRepo.Save(ctx, node); err != nil {
        return nil, err
    }
    
    // Convert back to DTO
    return &dto.CreateNodeResult{
        NodeID: node.ID().String(),
    }, nil
}
```

#### 3. Domain Entity → Repository Storage

```go
// Infrastructure Layer (Repository)
func (r *NodeRepositoryV2) Save(ctx context.Context, node *node.Node) error {
    // Convert domain entity to storage format
    item, err := r.entityConfig.ToItem(node)
    if err != nil {
        return err
    }
    
    // Persist to DynamoDB
    _, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: aws.String(r.tableName),
        Item:      item,
    })
    
    return err
}
```

### Layer Responsibilities

#### Interface Layer (HTTP Handlers)
- **What**: HTTP request/response handling
- **Responsibilities**:
  - Parse HTTP requests
  - Validate input format
  - Extract authentication/authorization context
  - Convert between HTTP and application DTOs
  - Handle HTTP-specific concerns (status codes, headers)
- **What NOT to do**:
  - Business logic
  - Direct database access
  - Complex validation (delegate to application layer)

#### Application Layer (Services)
- **What**: Use case orchestration
- **Responsibilities**:
  - Coordinate between domain objects
  - Manage transactions and unit of work
  - Convert between DTOs and domain objects
  - Publish domain events
  - Handle application-specific validation
- **What NOT to do**:
  - HTTP concerns
  - Database queries (use repositories)
  - Business rule enforcement (delegate to domain)

#### Domain Layer (Entities, Value Objects)
- **What**: Business logic and rules
- **Responsibilities**:
  - Enforce business invariants
  - Encapsulate business behavior
  - Generate domain events
  - Maintain entity consistency
  - Define domain concepts
- **What NOT to do**:
  - Persistence concerns
  - Infrastructure dependencies
  - Application flow control

#### Infrastructure Layer (Repositories, External Services)
- **What**: External system integration
- **Responsibilities**:
  - Data persistence and retrieval
  - External service communication
  - Technology-specific implementations
  - Data format conversion
  - Connection management
- **What NOT to do**:
  - Business logic
  - Domain rule enforcement
  - Application orchestration

### Error Flow

Errors flow upward through the layers, with each layer adding context:

```go
// Domain Layer - Business rule violation
return shared.NewBusinessRuleError("max_tags_exceeded", "Node", "cannot exceed 10 tags")

// Application Layer - Add application context
return errors.ApplicationError(ctx, "AddTag", err)

// Interface Layer - Convert to HTTP status
handleServiceError(w, err) // Returns 409 Conflict for business rule violations
```

This guide provides the foundation for understanding how to work within the Brain2 backend architecture. Each layer has clear responsibilities, and following these patterns ensures consistency and maintainability.