# Adding New Error Handling Guide

This guide explains how to add error handling for new features using the unified error system.

## Quick Start: Adding Errors for a New Feature

### 1. Define Error Codes (if needed)

Add new error codes to `internal/errors/codes.go`:

```go
const (
    // Add your feature-specific error codes
    CodeYourFeatureNotFound     ErrorCode = "YOUR_FEATURE_NOT_FOUND"
    CodeYourFeatureInvalid      ErrorCode = "YOUR_FEATURE_INVALID"
    CodeYourFeatureTimeout      ErrorCode = "YOUR_FEATURE_TIMEOUT"
)
```

### 2. Create Domain Errors

In your domain package, define domain-specific errors using the unified system:

```go
// internal/domain/yourfeature/errors.go
package yourfeature

import "brain2-backend/internal/errors"

var (
    ErrFeatureNotFound = errors.NotFound(
        errors.CodeYourFeatureNotFound.String(),
        "your feature not found",
    ).WithResource("yourfeature").Build()
    
    ErrFeatureInvalid = errors.Validation(
        errors.CodeYourFeatureInvalid.String(),
        "your feature validation failed",
    ).WithResource("yourfeature").Build()
)
```

### 3. Handle Errors in Application Layer

In your service layer, wrap errors with context:

```go
// internal/application/services/yourfeature_service.go
func (s *YourFeatureService) CreateFeature(ctx context.Context, cmd *CreateFeatureCommand) error {
    // Validate input
    if cmd.Name == "" {
        return errors.ServiceValidationError("name", "name is required", cmd.Name)
    }
    
    // Domain operation
    feature, err := yourfeature.NewFeature(cmd.Name)
    if err != nil {
        return errors.ApplicationError(ctx, "CreateFeature", err)
    }
    
    // Repository operation
    if err := s.repo.Save(ctx, feature); err != nil {
        return errors.ApplicationError(ctx, "SaveFeature", err)
    }
    
    return nil
}
```

### 4. Handle Errors in Repository Layer

In your repository, use repository-specific error helpers:

```go
// internal/infrastructure/persistence/dynamodb/yourfeature_repository.go
func (r *YourFeatureRepository) FindByID(ctx context.Context, id string) (*YourFeature, error) {
    result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{...})
    if err != nil {
        return nil, errors.RepositoryError("FindByID", err, "yourfeature")
    }
    
    if result.Item == nil {
        return nil, errors.NotFound(
            errors.CodeYourFeatureNotFound.String(),
            "feature not found",
        ).WithResource("yourfeature").Build()
    }
    
    return feature, nil
}
```

### 5. Handle Errors in HTTP Layer

Errors are automatically handled by the middleware, but you can customize responses:

```go
// internal/interfaces/http/v1/handlers/yourfeature_handler.go
func (h *YourFeatureHandler) Create(w http.ResponseWriter, r *http.Request) {
    // Parse request...
    
    result, err := h.service.CreateFeature(r.Context(), cmd)
    if err != nil {
        // The unified error system automatically determines the correct HTTP status
        handleServiceError(w, err)
        return
    }
    
    // Success response...
}
```

## Common Patterns

### Pattern 1: Validation Errors
```go
errors.Validation(code, message).
    WithDetails("specific validation failure").
    WithRecoveryMetadata(map[string]interface{}{
        "field": fieldName,
        "value": invalidValue,
    }).Build()
```

### Pattern 2: Not Found Errors
```go
errors.NotFound(code, message).
    WithResource(resourceType).
    WithUserID(userID).
    Build()
```

### Pattern 3: Retryable Errors
```go
errors.Timeout(code, message).
    WithRetryAfter(5 * time.Second).
    WithRetryInfo(retryCount, maxRetries).
    Build()
```

### Pattern 4: Business Rule Violations
```go
errors.NewError(errors.ErrorTypeDomain, code, message).
    WithSeverity(errors.SeverityMedium).
    WithResource(resourceType).
    WithDetails(ruleDescription).
    Build()
```

### Pattern 5: Wrapping External Errors
```go
if err != nil {
    return errors.Wrap(err, "YourOperation", "Failed to perform operation")
}
```

## Error Types and When to Use Them

| Error Type | Use Case | HTTP Status |
|------------|----------|-------------|
| `Validation` | Input validation failures | 400 |
| `NotFound` | Resource doesn't exist | 404 |
| `Conflict` | Resource conflicts, optimistic locking | 409 |
| `Unauthorized` | Authentication failures | 401 |
| `Forbidden` | Authorization failures | 403 |
| `Timeout` | Operation timeouts | 503 |
| `Internal` | Unexpected errors | 500 |
| `RateLimit` | Rate limiting | 429 |

## Best Practices

1. **Always provide context**: Use `WithResource()`, `WithOperation()`, `WithUserID()`
2. **Be specific with codes**: Create feature-specific error codes
3. **Add recovery hints**: Use `WithRecoveryStrategy()` for actionable errors
4. **Mark retryable errors**: Use `WithRetryable(true)` and `WithRetryAfter()`
5. **Set appropriate severity**: Critical for system failures, Low for user errors
6. **Preserve error chains**: Use `WithCause()` to maintain error context
7. **Test error paths**: Write tests for error scenarios

## Testing Error Handling

```go
func TestYourFeature_ErrorHandling(t *testing.T) {
    // Test validation error
    err := service.CreateFeature(ctx, &CreateFeatureCommand{Name: ""})
    assert.True(t, errors.IsValidation(err))
    
    // Test not found error
    _, err = service.GetFeature(ctx, "non-existent")
    assert.True(t, errors.IsNotFound(err))
    
    // Check error details
    var unifiedErr *errors.UnifiedError
    if errors.As(err, &unifiedErr) {
        assert.Equal(t, "yourfeature", unifiedErr.Resource)
        assert.Equal(t, errors.CodeYourFeatureNotFound.String(), unifiedErr.Code)
    }
}
```

## Migration from Old Error Patterns

If migrating from old error patterns:

```go
// Old pattern
return fmt.Errorf("feature not found: %s", id)

// New pattern
return errors.NotFound(
    errors.CodeYourFeatureNotFound.String(),
    fmt.Sprintf("feature not found: %s", id),
).WithResource("yourfeature").Build()

// Old pattern
return errors.New("validation failed")

// New pattern
return errors.Validation(
    errors.CodeValidationFailed.String(),
    "validation failed",
).Build()
```

## Monitoring and Debugging

The unified error system provides rich context for monitoring:

- **Severity levels** trigger appropriate alerts
- **Error codes** enable metric aggregation
- **Stack traces** aid in debugging
- **Correlation IDs** enable request tracing
- **User context** helps with user-specific issues

## Questions?

Refer to:
- `internal/errors/README.md` for system overview
- `internal/errors/example_usage.go` for more examples
- `internal/errors/codes.go` for available error codes
- `internal/errors/unified_errors.go` for core implementation