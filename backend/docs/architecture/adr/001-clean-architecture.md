# ADR-001: Adopt Clean Architecture Pattern

## Status
Accepted

## Context
The Brain2 backend started with a mixed architecture pattern that combined elements of:
- Traditional MVC in handlers
- Domain-Driven Design in the domain layer
- Repository pattern for data access
- Partial CQRS implementation

This led to:
- Inconsistent code organization
- Mixed responsibilities across layers
- Difficulty in maintaining and extending the codebase
- Challenges in testing individual components

## Decision
We will adopt Clean Architecture (Hexagonal Architecture) with the following structure:

```
internal/
├── domain/           # Core business logic (entities, value objects, domain services)
├── application/      # Use cases, application services, CQRS handlers
├── interfaces/       # External interfaces (HTTP, gRPC, CLI)
│   └── http/        # HTTP-specific implementations
├── infrastructure/   # External implementations (databases, external services)
│   ├── persistence/ # Repository implementations
│   └── services/    # External service integrations
```

### Key Principles:
1. **Dependency Rule**: Dependencies point inward (infrastructure → application → domain)
2. **Interface Segregation**: Small, focused interfaces for each capability
3. **Dependency Injection**: All dependencies are injected, not created
4. **Port and Adapters**: Clear boundaries between layers

## Consequences

### Positive:
- Clear separation of concerns
- Improved testability (each layer can be tested in isolation)
- Technology independence (easy to swap implementations)
- Better code organization and discoverability
- Easier onboarding for new developers

### Negative:
- Initial complexity for simple operations
- More boilerplate code
- Need for mapping between layer DTOs
- Learning curve for developers unfamiliar with the pattern

### Neutral:
- Requires discipline to maintain architectural boundaries
- Need for comprehensive documentation
- More files and packages to manage

## Implementation Notes

### Migration Strategy:
1. Create new clean architecture structure alongside existing code
2. Gradually migrate handlers to the interfaces layer
3. Move business logic to application services
4. Ensure backward compatibility during migration
5. Remove legacy code once migration is complete

### Example Handler Migration:

**Before (Legacy):**
```go
// internal/handlers/memory.go
func (h *MemoryHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
    // Mixed HTTP and business logic
    userID, _ := getUserID(r)
    var req api.CreateNodeRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Direct service call with business logic
    node, edges, err := h.memoryService.CreateNode(ctx, userID, req.Content, tags)
    
    // Direct response writing
    api.Success(w, http.StatusCreated, response)
}
```

**After (Clean Architecture):**
```go
// internal/interfaces/http/handlers/memory_handler.go
func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
    // Only HTTP concerns
    userID := r.Context().Value("userID").(string)
    var req api.CreateNodeRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Delegate to application service
    node, _, err := h.memoryService.CreateNode(r.Context(), userID, req.Content, req.Tags)
    if err != nil {
        h.handleServiceError(w, err)
        return
    }
    
    // Clean response handling
    response.WriteJSON(w, http.StatusCreated, toDTO(node))
}
```

## References
- [Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture by Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design by Eric Evans](https://domainlanguage.com/ddd/)