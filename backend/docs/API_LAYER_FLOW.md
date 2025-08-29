# API Layer Flow Documentation

This document explains how HTTP requests flow through the Brain2 backend architecture, detailing the responsibilities of each layer and how data transforms as it moves through the system.

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Complete Request Flow](#complete-request-flow)
- [Layer-by-Layer Breakdown](#layer-by-layer-breakdown)
- [Data Transformation Examples](#data-transformation-examples)
- [Error Flow](#error-flow)
- [Event Flow](#event-flow)
- [Performance Considerations](#performance-considerations)

---

## Architecture Overview

Brain2 follows **Clean Architecture** principles with clear layer boundaries and dependency inversion:

```
┌─────────────────────────────────────────────────────────────┐
│                     CLIENT REQUEST                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 INTERFACE LAYER                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ HTTP Handler│  │  Middleware │  │  Request/Response   │ │
│  │             │  │  Pipeline   │  │    Transformation  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                APPLICATION LAYER                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Use Case    │  │  Commands   │  │   Query Services    │ │
│  │ Services    │  │  & Queries  │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  DOMAIN LAYER                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Entities    │  │ Value       │  │  Domain Services    │ │
│  │ (Aggregates)│  │ Objects     │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│               INFRASTRUCTURE LAYER                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Repositories│  │  Event Bus  │  │   External APIs     │ │
│  │ (DynamoDB)  │  │(EventBridge)│  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              EXTERNAL SYSTEMS                               │
│           DynamoDB  │  EventBridge  │  Supabase             │
└─────────────────────────────────────────────────────────────┘
```

---

## Complete Request Flow

### Example: Creating a New Memory Node

Here's how a `POST /api/v1/memories` request flows through the system:

```
1. HTTP Request
   ↓
2. Lambda Runtime (cmd/main/main.go)
   ↓
3. Chi Router Routing
   ↓
4. Middleware Pipeline
   ↓
5. HTTP Handler (Interface Layer)
   ↓
6. Application Service (Application Layer)
   ↓
7. Domain Entity (Domain Layer)
   ↓
8. Repository (Infrastructure Layer)
   ↓
9. DynamoDB Storage
   ↓
10. Event Publishing
    ↓
11. Response Generation
```

---

## Layer-by-Layer Breakdown

### 1. Lambda Runtime Entry Point

**File**: `cmd/main/main.go`

**Responsibilities**:
- AWS Lambda runtime integration
- Cold start optimization
- Request/response format conversion
- Global error handling and recovery

```go
func main() {
    lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
        // Convert API Gateway request to standard HTTP request
        return chiLambda.ProxyWithContextV2(ctx, req)
    })
}
```

**Data Format**: 
- **Input**: `events.APIGatewayV2HTTPRequest` (AWS-specific format)
- **Output**: `events.APIGatewayV2HTTPResponse` (AWS-specific format)

### 2. HTTP Router

**Framework**: Chi Router  
**Configuration**: `internal/di/wire_providers.go` → `provideRouter()`

**Responsibilities**:
- Route requests to appropriate handlers
- URL parameter extraction
- Method matching
- Route-level middleware application

```go
func (h *MemoryHandler) Routes() chi.Router {
    r := chi.NewRouter()
    
    r.Post("/", h.CreateMemory)           // POST /api/v1/memories
    r.Get("/{id}", h.GetMemory)           // GET /api/v1/memories/{id}
    r.Put("/{id}", h.UpdateMemory)        // PUT /api/v1/memories/{id}
    r.Delete("/{id}", h.DeleteMemory)     // DELETE /api/v1/memories/{id}
    
    return r
}
```

### 3. Middleware Pipeline

**Location**: `internal/interfaces/http/middleware/`

**Middleware Chain** (executed in order):
```go
r.Use(middleware.RequestID)          // 1. Add request tracing ID
r.Use(middleware.Logger)             // 2. Request/response logging
r.Use(middleware.Recoverer)          // 3. Panic recovery
r.Use(middleware.Timeout(30*time.Second)) // 4. Request timeout
r.Use(authMiddleware.Authenticate)   // 5. JWT authentication
r.Use(middleware.CORS)               // 6. CORS headers
```

**Key Middleware Functions**:

#### Authentication Middleware
```go
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract JWT token from Authorization header
        token := extractBearerToken(r.Header.Get("Authorization"))
        
        // Validate JWT with Supabase
        claims, err := m.jwtValidator.ValidateToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // Add user context to request
        ctx := context.WithValue(r.Context(), "user_id", claims.Subject)
        ctx = context.WithValue(ctx, "user_claims", claims)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Data Transformations**:
- Adds `user_id` and `user_claims` to request context
- Adds `request_id` for tracing
- Sets CORS headers on response

### 4. HTTP Handler (Interface Layer)

**Location**: `internal/interfaces/http/v1/handlers/`

**Responsibilities**:
- HTTP-specific request parsing
- Input validation and sanitization
- DTO conversion
- HTTP status code mapping
- Response formatting

```go
// internal/interfaces/http/v1/handlers/memory.go
func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
    // 1. Extract user context (set by auth middleware)
    userID, ok := r.Context().Value("user_id").(string)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // 2. Parse and validate request body
    var req dto.CreateMemoryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // 3. Validate DTO
    if err := h.validator.Struct(&req); err != nil {
        handleValidationError(w, err)
        return
    }
    
    // 4. Generate idempotency key for duplicate prevention
    idempotencyKey := generateIdempotencyKey(r)
    
    // 5. Create application command
    cmd := &commands.CreateNodeCommand{
        UserID:         userID,
        Content:        req.Content,
        Title:          req.Title,
        Tags:           req.Tags,
        IdempotencyKey: idempotencyKey,
    }
    
    // 6. Call application layer
    result, err := h.nodeService.CreateNode(r.Context(), cmd)
    if err != nil {
        handleServiceError(w, err)  // Convert domain/app errors to HTTP status
        return
    }
    
    // 7. Convert to response DTO and return
    response := dto.CreateMemoryResponse{
        NodeID:    result.NodeID,
        Success:   true,
        Message:   "Memory created successfully",
        CreatedAt: result.CreatedAt,
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(response)
}
```

**Data Transformations**:
- **Input**: HTTP request body → `dto.CreateMemoryRequest`
- **Output**: `dto.CreateMemoryResponse` → JSON response

### 5. Application Service (Application Layer)

**Location**: `internal/application/services/`

**Responsibilities**:
- Use case orchestration
- Transaction management (Unit of Work)
- DTO ↔ Domain object conversion
- Domain event publishing
- Cross-cutting concerns (logging, tracing)

```go
// internal/application/services/node_service.go
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // 1. Start distributed tracing span
    ctx, span := s.tracer.Start(ctx, "NodeService.CreateNode")
    defer span.End()
    
    // 2. Convert DTO to domain value objects
    userID, err := shared.NewUserID(cmd.UserID)
    if err != nil {
        return nil, errors.ValidationError("userId", "invalid user ID format", cmd.UserID)
    }
    
    content, err := shared.NewContent(cmd.Content)
    if err != nil {
        return nil, errors.ValidationError("content", "invalid content", cmd.Content)
    }
    
    title, err := shared.NewTitle(cmd.Title)
    if err != nil {
        return nil, errors.ValidationError("title", "invalid title", cmd.Title)
    }
    
    tags := shared.NewTags(cmd.Tags...)
    
    // 3. Start unit of work for transaction management
    uow, err := s.uowFactory.NewUnitOfWork(ctx)
    if err != nil {
        return nil, errors.InternalError("failed to start transaction", err)
    }
    defer uow.Rollback() // Auto-rollback if not committed
    
    // 4. Create domain entity (business logic executed here)
    node, err := node.NewNode(userID, content, title, tags)
    if err != nil {
        return nil, errors.ApplicationError(ctx, "CreateNode", err)
    }
    
    // 5. Persist through repository
    if err := s.nodeRepo.Save(ctx, node); err != nil {
        return nil, errors.ApplicationError(ctx, "SaveNode", err)
    }
    
    // 6. Auto-discover connections to other nodes
    connections, err := s.connectionAnalyzer.FindConnections(ctx, node, userID)
    if err != nil {
        // Log error but don't fail - connections are not critical
        span.RecordError(err)
    } else {
        // Create edges for discovered connections
        for _, connection := range connections {
            edge, err := edge.NewEdge(node.ID(), connection.TargetNodeID, connection.Weight, connection.Type)
            if err == nil {
                s.edgeRepo.Save(ctx, edge) // Best effort
            }
        }
    }
    
    // 7. Publish domain events
    for _, event := range node.GetUncommittedEvents() {
        if err := s.eventBus.Publish(ctx, event); err != nil {
            return nil, errors.ApplicationError(ctx, "PublishEvent", err)
        }
    }
    
    // 8. Commit transaction
    if err := uow.Commit(); err != nil {
        return nil, errors.InternalError("failed to commit transaction", err)
    }
    
    // 9. Mark events as committed
    node.MarkEventsAsCommitted()
    
    // 10. Convert domain object back to DTO
    return &dto.CreateNodeResult{
        NodeID:    node.ID().String(),
        Title:     node.Title().String(),
        Content:   node.Content().String(),
        Tags:      node.Tags().ToSlice(),
        Keywords:  node.Keywords().ToSlice(),
        CreatedAt: node.CreatedAt(),
        Version:   node.Version(),
    }, nil
}
```

**Data Transformations**:
- **Input**: `commands.CreateNodeCommand` (application DTO)
- **Internal**: Domain value objects (`shared.UserID`, `shared.Content`, etc.)
- **Output**: `dto.CreateNodeResult` (response DTO)

### 6. Domain Entity (Domain Layer)

**Location**: `internal/domain/node/`

**Responsibilities**:
- Business rule enforcement
- Domain event generation
- Entity state management
- Invariant validation

```go
// internal/domain/node/node.go
func NewNode(userID shared.UserID, content shared.Content, title shared.Title, tags shared.Tags) (*Node, error) {
    // 1. Validate business rules
    if err := content.Validate(); err != nil {
        return nil, shared.NewDomainError("invalid_content", "node content validation failed", err)
    }
    
    if err := title.Validate(); err != nil {
        return nil, shared.NewDomainError("invalid_title", "node title validation failed", err)
    }
    
    // 2. Generate unique ID and timestamps
    now := time.Now()
    nodeID := shared.NewNodeID()
    
    // 3. Extract keywords from content (business logic)
    keywords := content.ExtractKeywords()
    
    // 4. Create entity with all invariants satisfied
    node := &Node{
        BaseAggregateRoot: shared.NewBaseAggregateRoot(nodeID.String()),
        id:        nodeID,
        userID:    userID,
        content:   content,
        title:     title,
        keywords:  keywords,
        tags:      tags,
        createdAt: now,
        updatedAt: now,
        version:   shared.NewVersion(), // Optimistic locking
        archived:  false,
        metadata:  make(map[string]interface{}),
        events:    []shared.DomainEvent{},
    }
    
    // 5. Generate domain event for node creation
    event := shared.NewNodeCreatedEvent(
        nodeID, userID, content, keywords, tags, node.version,
    )
    node.addEvent(event)
    
    // 6. Validate all invariants before returning
    if err := node.ValidateInvariants(); err != nil {
        return nil, err
    }
    
    return node, nil
}
```

**Data Transformations**:
- **Input**: Value objects (`shared.UserID`, `shared.Content`, etc.)
- **Internal**: Rich domain entity with encapsulated behavior
- **Side Effects**: Domain events generated

### 7. Repository (Infrastructure Layer)

**Location**: `internal/infrastructure/persistence/dynamodb/`

**Responsibilities**:
- Domain object ↔ storage format conversion
- Database operations (CRUD)
- Query optimization
- Connection management

```go
// internal/infrastructure/persistence/dynamodb/node_repository_refactored.go
func (r *NodeRepositoryV2) Save(ctx context.Context, node *node.Node) error {
    // 1. Convert domain entity to DynamoDB item
    item, err := r.entityConfig.ToItem(node)
    if err != nil {
        return errors.RepositoryError("ToItem", err, "node")
    }
    
    // 2. Add conditional expression for optimistic locking
    condition := expression.AttributeNotExists(expression.Name("PK")).
        Or(expression.Equal(expression.Name("Version"), expression.Value(node.Version()-1)))
    
    // 3. Build DynamoDB expression
    expr, err := expression.NewBuilder().
        WithCondition(condition).
        Build()
    if err != nil {
        return errors.RepositoryError("BuildExpression", err, "node")
    }
    
    // 4. Execute DynamoDB PutItem with optimistic locking
    _, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName:                aws.String(r.tableName),
        Item:                     item,
        ConditionExpression:      expr.Condition(),
        ExpressionAttributeNames: expr.Names(),
        ExpressionAttributeValues: expr.Values(),
    })
    
    if err != nil {
        // Handle conditional check failure (optimistic lock conflict)
        var conditionalErr *types.ConditionalCheckFailedException
        if errors.As(err, &conditionalErr) {
            return errors.ConflictError("version_conflict", "node was modified by another request", nil)
        }
        
        return errors.RepositoryError("PutItem", err, "node")
    }
    
    return nil
}
```

**Data Transformations**:
- **Input**: Domain entity (`*node.Node`)
- **Storage**: DynamoDB item format (`map[string]types.AttributeValue`)
- **Output**: Success/error status

### 8. Entity Configuration (Data Mapping)

**Location**: `internal/infrastructure/persistence/dynamodb/entity_configs.go`

**Responsibilities**:
- Domain ↔ DynamoDB format conversion
- Type safety and validation
- Schema evolution handling

```go
func (c *NodeEntityConfig) ToItem(n *node.Node) (map[string]types.AttributeValue, error) {
    // Convert rich domain object to flat DynamoDB item
    return map[string]types.AttributeValue{
        "PK":        &types.AttributeValueMemberS{Value: n.UserID().String()},
        "SK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", n.ID().String())},
        "GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", n.UserID().String())},
        "GSI1SK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("NODE#%s", n.CreatedAt().Format(time.RFC3339))},
        "Type":      &types.AttributeValueMemberS{Value: "NODE"},
        "NodeID":    &types.AttributeValueMemberS{Value: n.ID().String()},
        "UserID":    &types.AttributeValueMemberS{Value: n.UserID().String()},
        "Content":   &types.AttributeValueMemberS{Value: n.Content().String()},
        "Title":     &types.AttributeValueMemberS{Value: n.Title().String()},
        "Keywords":  &types.AttributeValueMemberSS{Value: n.Keywords().ToSlice()},
        "Tags":      &types.AttributeValueMemberSS{Value: n.Tags().ToSlice()},
        "CreatedAt": &types.AttributeValueMemberS{Value: n.CreatedAt().Format(time.RFC3339)},
        "UpdatedAt": &types.AttributeValueMemberS{Value: n.UpdatedAt().Format(time.RFC3339)},
        "Version":   &types.AttributeValueMemberN{Value: strconv.Itoa(n.Version())},
        "Archived":  &types.AttributeValueMemberBOOL{Value: n.IsArchived()},
    }, nil
}

func (c *NodeEntityConfig) ParseItem(item map[string]types.AttributeValue) (*node.Node, error) {
    // Convert flat DynamoDB item back to rich domain object
    // Handle missing fields for backward compatibility
    // Validate required fields
    // Reconstruct value objects
    
    nodeID, err := shared.ParseNodeID(item["NodeID"].(*types.AttributeValueMemberS).Value)
    if err != nil {
        return nil, err
    }
    
    // ... more parsing logic ...
    
    return node.ReconstructNode(nodeID, userID, content, title, keywords, tags,
        createdAt, updatedAt, version, archived), nil
}
```

---

## Data Transformation Examples

### Example 1: HTTP Request → Domain Entity

```json
// 1. HTTP Request Body
{
  "content": "Remember to review the project proposal",
  "title": "Project Review",
  "tags": ["work", "important"]
}
```

```go
// 2. DTO (Interface Layer)
type CreateMemoryRequest struct {
    Content string   `json:"content" validate:"required,min=1,max=10000"`
    Title   string   `json:"title" validate:"max=200"`
    Tags    []string `json:"tags" validate:"max=10,dive,min=1,max=50"`
}
```

```go
// 3. Application Command
type CreateNodeCommand struct {
    UserID         string   
    Content        string   
    Title          string   
    Tags           []string 
    IdempotencyKey string   
}
```

```go
// 4. Domain Value Objects
userID := shared.UserID{value: "user-123"}
content := shared.Content{
    value: "Remember to review the project proposal",
    wordCount: 7,
    keywords: []string{"review", "project", "proposal"}
}
title := shared.Title{value: "Project Review"}
tags := shared.Tags{values: []string{"work", "important"}}
```

```go
// 5. Domain Entity
node := &Node{
    id: NodeID{value: "node-456"},
    userID: userID,
    content: content,
    title: title,
    keywords: Keywords{values: ["review", "project", "proposal"]},
    tags: tags,
    createdAt: time.Now(),
    updatedAt: time.Now(),
    version: Version{value: 0},
    archived: false,
    events: [NodeCreatedEvent{...}]
}
```

### Example 2: Domain Entity → DynamoDB Item

```go
// Domain Entity
node := &Node{
    id: NodeID{value: "node-456"},
    userID: UserID{value: "user-123"},
    content: Content{value: "Remember to review..."},
    // ... other fields
}
```

```go
// DynamoDB Item
item := map[string]types.AttributeValue{
    "PK":        &types.AttributeValueMemberS{Value: "user-123"},
    "SK":        &types.AttributeValueMemberS{Value: "NODE#node-456"},
    "GSI1PK":    &types.AttributeValueMemberS{Value: "USER#user-123"},
    "GSI1SK":    &types.AttributeValueMemberS{Value: "NODE#2024-01-01T12:00:00Z"},
    "Type":      &types.AttributeValueMemberS{Value: "NODE"},
    "NodeID":    &types.AttributeValueMemberS{Value: "node-456"},
    "UserID":    &types.AttributeValueMemberS{Value: "user-123"},
    "Content":   &types.AttributeValueMemberS{Value: "Remember to review the project proposal"},
    "Title":     &types.AttributeValueMemberS{Value: "Project Review"},
    "Keywords":  &types.AttributeValueMemberSS{Value: ["review", "project", "proposal"]},
    "Tags":      &types.AttributeValueMemberSS{Value: ["work", "important"]},
    "CreatedAt": &types.AttributeValueMemberS{Value: "2024-01-01T12:00:00Z"},
    "UpdatedAt": &types.AttributeValueMemberS{Value: "2024-01-01T12:00:00Z"},
    "Version":   &types.AttributeValueMemberN{Value: "0"},
    "Archived":  &types.AttributeValueMemberBOOL{Value: false},
}
```

---

## Error Flow

Errors flow upward through the layers, with each layer adding appropriate context:

### Domain Layer Errors

```go
// Domain business rule violation
if len(n.tags.ToSlice()) >= 10 {
    return shared.NewBusinessRuleError("max_tags_exceeded", "Node", "cannot exceed 10 tags")
}
```

### Application Layer Error Handling

```go
// Application layer catches and adds context
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    node, err := node.NewNode(userID, content, title, tags)
    if err != nil {
        return nil, errors.ApplicationError(ctx, "CreateNode", err)
    }
    // ...
}
```

### Repository Layer Error Handling

```go
// Repository layer converts infrastructure errors
func (r *NodeRepository) Save(ctx context.Context, node *node.Node) error {
    _, err = r.client.PutItem(ctx, input)
    if err != nil {
        // Convert DynamoDB error to domain error
        var conditionalErr *types.ConditionalCheckFailedException
        if errors.As(err, &conditionalErr) {
            return errors.ConflictError("version_conflict", "node was modified by another request", nil)
        }
        return errors.RepositoryError("PutItem", err, "node")
    }
    return nil
}
```

### HTTP Layer Error Handling

```go
// HTTP layer converts to appropriate status codes
func handleServiceError(w http.ResponseWriter, err error) {
    var unifiedErr *errors.UnifiedError
    if errors.As(err, &unifiedErr) {
        switch unifiedErr.Type {
        case errors.ErrorTypeValidation:
            http.Error(w, unifiedErr.Error(), http.StatusBadRequest)
        case errors.ErrorTypeNotFound:
            http.Error(w, unifiedErr.Error(), http.StatusNotFound)
        case errors.ErrorTypeConflict:
            http.Error(w, unifiedErr.Error(), http.StatusConflict)
        case errors.ErrorTypeUnauthorized:
            http.Error(w, unifiedErr.Error(), http.StatusUnauthorized)
        default:
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    // Fallback for unstructured errors
    http.Error(w, "Internal server error", http.StatusInternalServerError)
}
```

---

## Event Flow

Domain events are generated during business operations and flow through the event system:

### Domain Event Generation

```go
// Domain entity generates events
func (n *Node) UpdateContent(newContent shared.Content) error {
    // Business logic...
    
    // Generate domain event
    event := shared.NewNodeContentUpdatedEvent(
        n.id, n.userID, oldContent, newContent, 
        oldKeywords, n.keywords, n.version,
    )
    n.addEvent(event)
    
    return nil
}
```

### Event Publishing (Application Layer)

```go
// Application service publishes events
func (s *NodeService) UpdateNode(ctx context.Context, cmd *commands.UpdateNodeCommand) error {
    // Update node...
    
    // Publish all uncommitted events
    for _, event := range node.GetUncommittedEvents() {
        if err := s.eventBus.Publish(ctx, event); err != nil {
            return errors.ApplicationError(ctx, "PublishEvent", err)
        }
    }
    
    node.MarkEventsAsCommitted()
    return nil
}
```

### Event Bus (Infrastructure)

```go
// EventBridge adapter publishes to AWS
func (e *EventBridgePublisher) Publish(ctx context.Context, event shared.DomainEvent) error {
    eventEntry := types.PutEventsRequestEntry{
        Source:      aws.String("brain2.backend"),
        DetailType:  aws.String(event.EventType()),
        Detail:      aws.String(string(eventJSON)),
        EventBusName: aws.String(e.eventBusName),
    }
    
    _, err := e.client.PutEvents(ctx, &eventbridge.PutEventsInput{
        Entries: []types.PutEventsRequestEntry{eventEntry},
    })
    
    return err
}
```

---

## Performance Considerations

### Layer-Specific Optimizations

#### HTTP Layer
- **Connection Pooling**: Reuse HTTP connections across Lambda invocations
- **Response Compression**: Gzip responses for large payloads
- **Request Batching**: Batch multiple operations when possible

#### Application Layer
- **Transaction Scoping**: Keep transactions as short as possible
- **Parallel Processing**: Use goroutines for independent operations
- **Caching**: Cache frequently accessed data

#### Domain Layer
- **Lazy Loading**: Load related entities only when needed
- **Event Batching**: Batch domain events for publishing
- **Invariant Caching**: Cache expensive validation results

#### Infrastructure Layer
- **Connection Reuse**: Maintain DynamoDB connection pools
- **Batch Operations**: Use DynamoDB BatchWriteItem when possible
- **Query Optimization**: Use appropriate indexes and projections

### Request Flow Optimization

```go
// Example: Optimized node creation with parallel operations
func (s *NodeService) CreateNode(ctx context.Context, cmd *commands.CreateNodeCommand) (*dto.CreateNodeResult, error) {
    // 1. Create node (sequential - required)
    node, err := node.NewNode(userID, content, title, tags)
    if err != nil {
        return nil, err
    }
    
    // 2. Save node (sequential - required)
    if err := s.nodeRepo.Save(ctx, node); err != nil {
        return nil, err
    }
    
    // 3. Parallel operations (non-critical path)
    var wg sync.WaitGroup
    
    // Connection discovery in background
    wg.Add(1)
    go func() {
        defer wg.Done()
        connections, _ := s.connectionAnalyzer.FindConnections(ctx, node, userID)
        for _, conn := range connections {
            edge, _ := edge.NewEdge(node.ID(), conn.TargetNodeID, conn.Weight, conn.Type)
            s.edgeRepo.Save(ctx, edge) // Best effort
        }
    }()
    
    // Event publishing in background
    wg.Add(1)
    go func() {
        defer wg.Done()
        for _, event := range node.GetUncommittedEvents() {
            s.eventBus.Publish(ctx, event) // Best effort
        }
    }()
    
    // Don't wait for background operations to complete
    // Return response immediately
    return &dto.CreateNodeResult{
        NodeID:    node.ID().String(),
        CreatedAt: node.CreatedAt(),
    }, nil
    
    // Background operations continue after response is sent
}
```

This architecture ensures clean separation of concerns while maintaining high performance and scalability for the Brain2 knowledge graph system.