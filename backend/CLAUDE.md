# Critical Rules for Brain2 Backend Development

## 🚫 NEVER USE interface{}
**This is the #1 rule - NO EXCEPTIONS**

- ❌ NEVER use `interface{}` or `any` 
- ❌ NEVER use `map[string]interface{}`
- ❌ NEVER use `[]interface{}`

### Instead, ALWAYS:
- ✅ Use specific types (e.g., `string`, `int`, `*Node`)
- ✅ Create new struct types when needed
- ✅ Use generics for type flexibility: `func Process[T any](item T)`
- ✅ Define concrete types for all data structures

### Why This Matters:
- Compile-time type safety catches bugs early
- Better IDE support and autocomplete
- No runtime type assertions (better performance)
- Code is self-documenting

### Examples:
```go
// ❌ BAD - Never do this
func ProcessData(data interface{}) error
func GetResult() (interface{}, error)
EventData() map[string]interface{}

// ✅ GOOD - Always do this
func ProcessNode(node *Node) error
func GetNode() (*Node, error)
EventData() NodeEventPayload

// ✅ GOOD - Use generics when needed
func Process[T any](item T) error
type Cache[T any] struct { items map[string]T }
```

## 📏 Service File Size Limits
- **Maximum 300 lines per service file**
- Split large services by responsibility (command/query/bulk)
- Use separate files for mappers and converters
- One primary responsibility per file

## 🏗️ Clean Architecture Rules
- **Domain layer**: NEVER import application, infrastructure, or interfaces
- **Application layer**: NEVER import infrastructure or interfaces
- **Dependency direction**: Always inward (Infrastructure → Application → Domain)
- **Interfaces**: Define in the layer that uses them, not the layer that implements them

## 🔧 Required Patterns

### Context Usage
```go
// Always use context with timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Resource Cleanup
```go
// Always defer cleanup
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()

// Always unlock mutexes
mu.Lock()
defer mu.Unlock()
```

### Error Handling
```go
// Always handle errors explicitly
if err != nil {
    return fmt.Errorf("failed to process node %s: %w", nodeID, err)
}

// Never ignore errors silently
_ = someFunction() // ❌ BAD

// If you must ignore, document why
_ = file.Close() // Cleanup, error doesn't affect flow
```

## 🎯 Repository Interfaces
- **Domain repositories**: Minimal, only essential operations
- **Application ports**: Extended interfaces for use cases
- **NO duplicate interfaces** across layers
- Repository interfaces belong in domain or application/ports, NOT in repository package

## 🚀 Performance Guidelines
- **No N+1 queries**: Use batch operations
- **Cache frequently accessed data**: Use cache decorators
- **Limit goroutines**: Use worker pools
- **Set timeouts**: Every external call needs a timeout

## 📦 Package Organization
```
internal/
  domain/           # Core business logic, no external dependencies
    node/
    edge/
    category/
  application/      # Use cases and orchestration
    commands/       # Write operations
    queries/        # Read operations
    services/       # Business services (keep small!)
    ports/          # Interfaces for infrastructure
  infrastructure/   # External concerns
    persistence/    # Database implementations
    messaging/      # Event publishing
    cache/         # Caching implementations
  interfaces/       # API layer
    http/          # REST endpoints
    grpc/          # gRPC services
```

## 🔴 Common Mistakes to Avoid
1. Using `interface{}` for "flexibility" - Use generics or specific types
2. Large service files - Split by responsibility
3. Importing outer layers in domain - Keep domain pure
4. Duplicate repository interfaces - Single source of truth
5. Ignoring errors - Always handle or explicitly document
6. Missing timeouts - Every external call needs one
7. Not deferring cleanup - Always use defer for cleanup

## 📝 When Adding New Features
1. Start with domain types (no external dependencies)
2. Define repository interfaces in domain (minimal)
3. Extend interfaces in application/ports if needed
4. Implement in infrastructure
5. Keep services under 300 lines
6. NEVER use interface{} - create specific types

## 🧪 Testing Requirements
- Domain logic: 100% coverage target
- Application services: 80% coverage target
- Focus on behavior, not implementation
- Use interfaces for mocking

---

**Remember**: If you're about to type `interface{}`, STOP and create a proper type instead!