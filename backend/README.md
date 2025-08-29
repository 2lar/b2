# Brain2 Backend

This repository contains the backend services for the Brain2 application - a graph-based personal knowledge management system that demonstrates advanced serverless architecture patterns on AWS.

## Architecture Overview

Brain2 follows a **Domain-Driven Design (DDD)** approach with **Clean Architecture** principles, implementing **Event-Driven Architecture** patterns for a scalable, maintainable serverless system.

### Key Architectural Patterns

- **Domain-Driven Design (DDD)**: Rich domain models with business logic encapsulated in entities and value objects
- **Clean Architecture**: Clear separation of concerns with dependency inversion
- **Command Query Responsibility Segregation (CQRS)**: Separate read and write models for optimal performance
- **Event-Driven Architecture**: EventBridge for decoupled, scalable communication
- **Generic Repository Pattern**: 90% code reduction through composition-based repositories
- **Lambda-lith Pattern**: Single Lambda handling multiple routes for better cold start performance

### Why These Patterns?

**DDD + Clean Architecture**: Ensures business logic remains independent of frameworks and infrastructure, making the system testable and maintainable as it grows in complexity.

**CQRS**: Optimizes read operations for the knowledge graph (complex queries with joins) separately from write operations (simple CRUD with event generation).

**Event-Driven Architecture**: Enables real-time graph updates, audit trails, and future extensibility without tight coupling between components.

**Generic Repository**: Eliminates 1300+ lines of repetitive CRUD code while maintaining type safety and domain-specific operations.

**Lambda-lith**: Balances serverless benefits with cold start optimization - better than microservices for moderate traffic, better than monoliths for scaling.

**Automated OpenAPI Generation**: Complete API documentation generated automatically from code annotations, ensuring documentation is always current and accurate.

## Project Structure

The project is structured to follow Clean Architecture principles with clear layer boundaries:

### Core Layers

-   **`cmd/`**: Lambda entry points and application bootstrap
    -   `main/main.go`: Primary API Lambda with cold start optimization
    -   `ws-*`: WebSocket Lambdas for real-time graph updates
    -   `cleanup-handler`: Batch cleanup operations Lambda

-   **`internal/domain/`**: Core business logic (Framework-agnostic)
    -   Rich domain models with encapsulated business rules
    -   Domain services for complex operations (connection analysis, graph algorithms)
    -   Domain events for communicating state changes

-   **`internal/application/`**: Use case orchestration (CQRS implementation)
    -   `commands/`: Write operations with business validation
    -   `queries/`: Optimized read operations with caching
    -   `services/`: Application services orchestrating domain operations

-   **`internal/infrastructure/`**: External concerns and implementations
    -   `persistence/dynamodb/`: Repository implementations with generic patterns
    -   `messaging/`: EventBridge integration for event publishing
    -   `concurrency/`: Optimized goroutine pools for Lambda environments

### Supporting Layers

-   **`internal/di/`**: Wire-based dependency injection
    -   Compile-time dependency graph generation
    -   Container pattern for Lambda lifecycle management

-   **`internal/errors/`**: Unified error handling system
    -   Consolidates multiple error handling approaches
    -   Provides structured error responses with recovery strategies

-   **`internal/interfaces/http/`**: HTTP layer (Clean Architecture adapter)
    -   Request/response handling and validation
    -   API versioning and middleware pipeline

-   **`pkg/`**: Reusable utilities and shared types
    -   OpenAPI-generated types for consistency
    -   Cross-cutting concerns and helpers

## Quick Command Reference

For a comprehensive list of all commands, see [📚 Command Master List](../docs/command-masterlist.md)

### Most Common Commands
```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build for Lambda
./build.sh

# Quick build (no tests)
./test_build.sh

# Generate Wire dependencies
cd internal/di && wire

# Run locally
go run cmd/main/main.go

# Format code
go fmt ./...

# Run linter
golangci-lint run
```

## Getting Started

### Prerequisites

-   Go (version 1.22 or higher)
-   AWS CLI configured with appropriate credentials
-   Docker (for local DynamoDB development, if applicable)

### Local Development

1.  **Install Dependencies:**
    ```bash
    go mod tidy
    ```

2.  **Set up Environment Variables:**
    Create a `.env` file in the `backend/` directory (or set them directly in your shell):
    ```
    TABLE_NAME=your-dynamodb-table-name
    INDEX_NAME=your-dynamodb-gsi-name
    ```
    (For local development, you might use a local DynamoDB instance and configure the AWS SDK accordingly.)

3.  **Build the Application:**
    ```bash
    go build -o bin/main cmd/main/main.go
    ```

4.  **Run Locally (e.g., with a local API Gateway emulator or directly):**
    If running as a Lambda function locally, you might use `sam local start-api` or similar tools.

## Dependency Injection with Wire

This project uses [Wire](https://github.com/google/wire) for compile-time dependency injection. Wire generates code that connects components, ensuring a clean and maintainable dependency graph.

### Why Wire?

Wire helps enforce Clean Architecture principles by:
-   **Automating Dependency Graph:** You define how to provide dependencies, and Wire generates the boilerplate code to connect them.
-   **Compile-Time Safety:** Errors in the dependency graph are caught at compile time, not runtime.
-   **Reduced Boilerplate:** Eliminates manual dependency wiring, especially in large applications.
-   **Enforcing Layering:** Encourages explicit dependency declarations, making it harder to violate architectural rules.

## Key Design Decisions

### 1. **Dependency Injection with Wire**

**Decision**: Use Google's Wire for compile-time dependency injection instead of runtime DI frameworks.

**Rationale**: 
- **Cold Start Optimization**: No reflection overhead during Lambda initialization
- **Compile-time Safety**: Dependency graph errors caught at build time, not runtime
- **Zero Runtime Cost**: All wiring happens at compile time, resulting in simple constructors
- **Explicit Dependencies**: Forces clear dependency declarations, preventing architectural violations

**Trade-offs**: Requires code generation step, but the performance and safety benefits outweigh the complexity.

### 2. **Single-Table DynamoDB Design**

**Decision**: Use DynamoDB single-table design with composite keys instead of multiple tables.

**Rationale**:
- **Performance**: Reduces cross-table joins and enables efficient batch operations
- **Cost Optimization**: Fewer read/write capacity units through data locality
- **Scalability**: Leverages DynamoDB's partition key distribution for automatic scaling
- **Consistency**: Related entities (nodes, edges, keywords) stored together for transactional consistency

**Implementation**: `PK = UserID`, `SK = EntityType#EntityID` pattern enables efficient queries and maintains data isolation.

### 3. **Generic Repository Pattern**

**Decision**: Implement composition-based generic repositories instead of inheritance or code duplication.

**Rationale**:
- **Code Reduction**: Eliminated 1,346 lines of duplicated CRUD operations (90% reduction)
- **Type Safety**: Maintains compile-time type safety while sharing implementation
- **Consistency**: Ensures all repositories have identical behavior for common operations
- **Maintainability**: Single point of change for repository patterns and optimizations

**Implementation**: `GenericRepository[T]` handles CRUD, specific repositories add domain-specific queries.

### 4. **Event-Driven Architecture with EventBridge**

**Decision**: Use AWS EventBridge for domain events instead of direct service calls.

**Rationale**:
- **Decoupling**: Services don't need to know about each other, only about events
- **Scalability**: EventBridge handles fan-out and retries automatically
- **Auditability**: All state changes create permanent audit trail
- **Extensibility**: New features can subscribe to existing events without modifying producers
- **Reliability**: Built-in dead letter queues and retry mechanisms

**Trade-offs**: Eventual consistency model requires careful design of dependent operations.

### 5. **Lambda-lith Architecture**

**Decision**: Single Lambda function handling multiple HTTP routes instead of per-function microservices.

**Rationale**:
- **Cold Start Optimization**: Shared connection pools and initialized resources across routes
- **Cost Efficiency**: Lower total memory allocation and execution time for related operations
- **Operational Simplicity**: Single deployment unit, shared monitoring and logging
- **Performance**: Reduced inter-service communication overhead

**When to Split**: Individual functions only for fundamentally different workloads (WebSocket, batch processing).

## Performance Optimizations

### Cold Start Mitigation

```go
// Container-based initialization in init()
var container *di.Container

func init() {
    // Heavy initialization happens once during cold start
    container = di.InitializeContainer()
    // Connection pools, configuration, and services ready for all requests
}
```

**Key Strategies**:
- **Container Pattern**: Single initialization of all services and connections
- **Lazy Loading**: Expensive resources initialized on first use within functions
- **Connection Reuse**: Database and AWS service connections persist across invocations
- **Pre-compilation**: Wire generates all dependency wiring at build time

### DynamoDB Optimization

- **Batch Operations**: Group related reads/writes to minimize round trips
- **Connection Pooling**: Reuse HTTP connections across Lambda invocations
- **Pagination**: Efficient cursor-based pagination for large result sets
- **Projection**: Only fetch required attributes to reduce data transfer

### Goroutine Pool Management

```go
// Adaptive pool sizing based on Lambda environment
pool := concurrency.NewAdaptivePool(
    concurrency.WithLambdaOptimization(), // Smaller pools in Lambda
    concurrency.WithCPUBasedSizing(),     // Scale with available CPU
)
```

**Lambda-specific Considerations**:
- **Memory Constraints**: Smaller goroutine pools to avoid memory pressure
- **CPU Limits**: Pool sizing based on allocated Lambda CPU
- **Timeout Awareness**: Operations respect Lambda execution time limits

### Installation

To install Wire, run:

```bash
go install github.com/google/wire/cmd/wire@latest
```

### Generating Dependencies

After making changes to your dependency providers (functions that create instances of your components) in `internal/di/wire.go`, you need to run Wire to generate the actual dependency injection code.

1.  Navigate to the `di` directory:
    ```bash
    cd backend/internal/di
    ```

2.  Run the `wire` command:
    ```bash
    wire
    ```
    This command will generate or update `wire_gen.go` in the same directory. This file contains the `InitializeAPI` function (and other generated providers) that constructs the entire application's dependency graph.

    **Important:**
    -   `wire.go` contains the `//go:build wireinject` build tag. This tells the Go compiler to *only* compile this file when the `wire` command is run.
    -   `wire_gen.go` contains the `//go:build !wireinject` build tag. This tells the Go compiler to *exclude* `wire.go` and *include* `wire_gen.go` during normal `go build` operations. This prevents redeclaration errors.

### Integration with `main.go`

The `cmd/main/main.go` file is the entry point of the application. It uses the `InitializeAPI` function generated by Wire to get a fully constructed HTTP router (`*chi.Mux`).

```go
package main

import (
	"context"
	"log"

	"brain2-backend/internal/di"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
)

var chiLambda *chiadapter.ChiLambdaV2

func init() {
	// InitializeAPI is generated by Wire and constructs the entire dependency graph.
	router, err := di.InitializeAPI()
	if err != nil {
		log.Fatalf("Failed to initialize API: %v", err)
	}
	// Wrap the chi router with the Lambda adapter
	chiLambda = chiadapter.NewV2(router)
	log.Println("Service initialized successfully with Wire DI")
}

func main() {
	// Start the Lambda handler
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return chiLambda.ProxyWithContextV2(ctx, req)
	})
}
```

By following these steps, you can effectively manage your backend's dependencies using Wire, ensuring a clean and maintainable architecture.

## Error Handling Strategy

The backend uses a **unified error handling system** that consolidates multiple error handling approaches into a single, consistent pattern:

### Error Classification

```go
// Errors are categorized by type for proper handling
ErrorTypeValidation   // Input validation failures
ErrorTypeNotFound     // Resource not found
ErrorTypeConflict     // Business rule violations  
ErrorTypeInternal     // System/infrastructure errors
ErrorTypeDomain       // Domain logic violations
```

### Error Response Structure

```go
type UnifiedError struct {
    Type         ErrorType    // Category for programmatic handling
    Code         string       // Specific error code
    Message      string       // Human-readable message
    Severity     ErrorSeverity // For logging/alerting
    Retryable    bool         // Whether operation can be retried
    Recovery     string       // Suggested recovery approach
}
```

### Why Unified Errors?

- **Consistency**: All layers use the same error structure and handling patterns
- **Debuggability**: Rich context and stack traces for troubleshooting
- **Client Experience**: Structured error responses with actionable recovery suggestions
- **Monitoring**: Proper error classification enables better alerting and metrics

## Testing Strategy

### Unit Testing
- **Domain Models**: Test business logic and invariants in isolation
- **Value Objects**: Validate creation rules and behavior
- **Application Services**: Mock dependencies to test orchestration logic
- **Repositories**: Test query logic and data transformation

### Integration Testing
- **DynamoDB Operations**: Test against DynamoDB Local
- **Event Publishing**: Verify EventBridge message structure
- **Wire DI**: Ensure dependency graph builds correctly

### Performance Testing
- **Cold Start Benchmarks**: Measure initialization time under different conditions
- **Concurrency**: Test goroutine pool behavior under load
- **Memory Usage**: Profile memory allocation in Lambda environment

### Test Organization
```bash
# Unit tests alongside source code
internal/domain/node/node_test.go

# Integration tests in separate directories  
infrastructure/dynamodb/tests/integration_test.go

# End-to-end tests (not implemented yet)
tests/e2e/
```

## API Documentation

The Brain2 backend features **automated OpenAPI specification generation** that ensures API documentation is always accurate and up-to-date.

### 📚 Documentation System

- **Interactive Documentation**: Access Swagger UI at `/api/swagger-ui` or `/api/docs`
- **OpenAPI Specification**: Download from `/api/swagger.yaml` or `/api/swagger.json`
- **Automatic Generation**: Documentation generated from code annotations during build
- **Type Safety**: All request/response models properly typed with examples

### 🚀 Quick Access

```bash
# Generate documentation locally
./generate-openapi.sh

# Generate with validation
./generate-openapi.sh --validate

# Build with automatic generation
./build.sh
```

### 📖 Documentation Guides

| Guide | Purpose |
|-------|---------|
| **[OpenAPI Overview](docs/OPENAPI_GENERATION.md)** | System architecture and features |
| **[Developer Guide](docs/OPENAPI_DEVELOPER_GUIDE.md)** | Annotation patterns and examples |
| **[Build Integration](docs/OPENAPI_BUILD_INTEGRATION.md)** | CI/CD integration and workflows |
| **[Troubleshooting](docs/OPENAPI_TROUBLESHOOTING.md)** | Error diagnosis and solutions |

### 📊 Current API Stats

- **13 Endpoints** fully documented
- **15+ Models** with examples and validation
- **JWT Authentication** properly configured
- **4 Logical Groups** (Memory, Category, Graph, System)

For complete documentation, see [docs/README.md](docs/README.md).
