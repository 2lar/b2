# Standardized Error Handling System

## Overview

This package provides a unified, comprehensive error handling system for the Brain2 backend. It consolidates multiple error handling approaches into a single, consistent system that provides rich error context, proper logging, and standardized HTTP responses.

## Key Features

### 1. Unified Error Structure (`UnifiedError`)
- **Consistent error representation** across all layers (domain, repository, application, HTTP)
- **Rich context** including operation, resource, user ID, request ID, and correlation ID
- **Error classification** by type, code, and severity
- **Recovery information** including retry strategies and compensation functions
- **Stack traces** for debugging

### 2. Error Types and Codes
- **Comprehensive error types**: Validation, NotFound, Conflict, Unauthorized, Internal, Timeout, etc.
- **Specific error codes**: Over 40 predefined error codes for common scenarios
- **HTTP status mapping**: Automatic HTTP status code determination
- **Severity levels**: Critical, High, Medium, Low for proper alerting

### 3. Layer-Specific Adapters
- **Domain adapter** (`domain_adapter.go`): Converts domain errors to unified errors
- **Repository adapter** (`repository_adapter.go`): Handles database and infrastructure errors
- **Application adapter** (`application_adapter.go`): Manages service layer errors with context enrichment

### 4. HTTP Middleware
- **Error enrichment**: Adds correlation IDs, request IDs, and user context
- **Panic recovery**: Gracefully handles panics with proper logging
- **Standardized responses**: Consistent error response format across all endpoints

### 5. Structured Logging
- **Context-aware logging**: Automatically includes correlation ID, request ID, user ID
- **Log level mapping**: Based on error severity
- **Performance metrics**: Request duration, status codes, error rates

## Usage Examples

### Creating Errors

```go
// Validation error
err := errors.Validation(errors.CodeNodeContentEmpty, "Content cannot be empty").
    WithResource("node").
    WithDetails("Node content is required").
    Build()

// Not found error
err := errors.NotFound(errors.CodeNodeNotFound, "Node not found").
    WithResource("node").
    WithRecoveryMetadata(map[string]interface{}{
        "node_id": nodeID,
    }).
    Build()

// Retryable error
err := errors.Timeout(errors.CodeTimeout, "Operation timed out").
    WithOperation("CreateNode").
    WithRetryable(true).
    WithRetryAfter(5 * time.Second).
    Build()
```

### Repository Layer

```go
func (r *NodeRepository) GetNode(ctx context.Context, nodeID string) (*Node, error) {
    result, err := r.db.GetItem(ctx, input)
    if err != nil {
        return nil, errors.RepositoryError("GetNode", "node", err)
    }
    // ...
}
```

### Application Layer

```go
func (s *NodeService) CreateNode(ctx context.Context, cmd CreateNodeCommand) (*NodeDTO, error) {
    node, err := s.repo.Create(ctx, node)
    if err != nil {
        return nil, errors.ApplicationError(ctx, "CreateNode", err)
    }
    // ...
}
```

### HTTP Handlers

```go
func CreateNodeHandler(w http.ResponseWriter, r *http.Request) {
    result, err := service.CreateNode(r.Context(), cmd)
    if err != nil {
        errors.WriteHTTPError(w, err, logger)
        return
    }
    // ...
}
```

### Middleware Setup

```go
// Setup middleware chain
router := chi.NewRouter()

// Add correlation ID
router.Use(errors.CorrelationIDMiddleware(logger))

// Add error enrichment
router.Use(errors.ErrorEnrichmentMiddleware(logger.Logger))

// Add request logging
router.Use(errors.RequestLoggingMiddleware(logger))

// Add error logging
router.Use(errors.ErrorLoggingMiddleware(logger))
```

## Error Checking

```go
// Check error type
if errors.IsValidation(err) {
    // Handle validation error
}

if errors.IsNotFound(err) {
    // Handle not found error
}

// Check if retryable
if errors.IsRetryable(err) {
    // Implement retry logic
}

// Get severity
severity := errors.GetSeverity(err)
switch severity {
case errors.SeverityCritical:
    // Page on-call engineer
case errors.SeverityHigh:
    // Alert team
}
```

## Migration Guide

### From Old Error Systems

The system provides backward compatibility through adapters:

```go
// Convert from domain errors
unifiedErr := errors.FromDomainError(domainErr)

// Convert from legacy errors
unifiedErr := errors.FromLegacyError(legacyErr)

// Convert back to domain error (if needed)
domainErr := errors.ToDomainError(unifiedErr)
```

### Gradual Migration

1. Start using unified errors in new code
2. Update critical paths first (error-prone operations)
3. Migrate layer by layer (HTTP → Application → Repository → Domain)
4. Update tests to use new error checking functions
5. Remove old error packages once migration is complete

## Benefits

1. **Consistency**: Single error handling approach across the codebase
2. **Observability**: Rich context for debugging and monitoring
3. **Maintainability**: Centralized error definitions and handling logic
4. **Reliability**: Proper error classification and recovery strategies
5. **Performance**: Efficient error handling with minimal overhead
6. **Security**: No sensitive information leakage in error messages

## Best Practices

1. **Always add context**: Use WithOperation, WithResource, WithUserID
2. **Set appropriate severity**: Critical for system failures, Low for user errors
3. **Mark retryable errors**: Set WithRetryable(true) for transient failures
4. **Include recovery strategies**: Add WithRecoveryStrategy for complex errors
5. **Use specific error codes**: Choose from predefined codes or create new ones
6. **Log at appropriate levels**: Based on severity, not all errors need ERROR level
7. **Test error paths**: Include error scenarios in unit and integration tests

## Testing

The package includes comprehensive tests covering:
- Error creation and building
- Type checking and classification
- HTTP status code mapping
- Bulk operation errors
- Legacy error conversion
- Stack trace capture

Run tests:
```bash
go test ./internal/errors -v
```

## Performance Considerations

- Stack traces are captured only for internal errors (configurable)
- Logging is sampled in production to prevent flooding
- Error objects are lightweight with lazy initialization
- Context enrichment is done only when needed

## Future Enhancements

- [ ] Integration with distributed tracing (OpenTelemetry)
- [ ] Error metrics and dashboards
- [ ] Automatic error reporting to monitoring services
- [ ] Error recovery automation
- [ ] Machine learning for error pattern detection