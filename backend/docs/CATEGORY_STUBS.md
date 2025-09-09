# Category Feature Stubs

## Overview
The category management feature is currently implemented as stubs in backend to allow the frontend to function without the deprecated backend. These stubs return successful responses with empty data, enabling future implementation without breaking the current system.

## Stubbed Endpoints

All category endpoints return successful responses with a `stub: true` field to indicate they're placeholder implementations.

### Category Management
- `GET /api/v2/categories` - Returns empty category list
- `POST /api/v2/categories/rebuild` - Returns success without rebuilding
- `GET /api/v2/categories/suggest` - Returns empty suggestions

### Node Categories
- `GET /api/v2/nodes/{nodeId}/categories` - Returns empty categories for node
- `POST /api/v2/nodes/{nodeId}/categories` - Returns success without categorizing

## Response Format

All stub responses include:
```json
{
  "stub": true,
  // ... other response data
}
```

## Implementation Location
- Handler: `/interfaces/http/rest/handlers/category_handler.go`
- Routes: `/interfaces/http/rest/router.go`

## Future Implementation

To implement the full category feature:

1. **Create Domain Models**
   - Add category entities in `/domain/core/entities/`
   - Define category value objects in `/domain/core/valueobjects/`

2. **Implement Commands**
   - Create category commands in `/application/commands/`
   - Add command handlers in `/application/commands/handlers/`

3. **Implement Queries**
   - Create category queries in `/application/queries/`
   - Add query handlers in `/application/queries/handlers/`

4. **Add Repository**
   - Implement category repository in `/infrastructure/persistence/dynamodb/`
   - Add category ports in `/application/ports/`

5. **Update Handler**
   - Replace stub implementations in `category_handler.go`
   - Connect to command/query buses

6. **Database Schema**
   - Design DynamoDB schema for categories
   - Consider GSI for category lookups

## Testing
The stubs allow the system to function while the full implementation is pending. Frontend will receive empty data but won't encounter errors.

## Migration from Backend v1
The old backend (`/backend`) had full category implementation. This stub ensures compatibility while the feature is reimplemented using the DDD/CQRS architecture.