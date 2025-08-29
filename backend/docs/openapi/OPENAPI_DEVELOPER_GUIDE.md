# OpenAPI Developer Guide

This guide provides detailed instructions for developers on how to add, modify, and maintain OpenAPI documentation using swaggo/swag annotations.

## Quick Start

1. Add annotations to your handler function
2. Define request/response types in `pkg/api/types.go`
3. Run `./generate-openapi.sh` to test
4. Build normally - documentation updates automatically

## Annotation Patterns

### Basic Handler Annotation Template

```go
// HandlerName handles HTTP_METHOD /api/endpoint
// @Summary Brief description (required)
// @Description Detailed description of what this endpoint does
// @Tags Tag Name
// @Accept json              // For endpoints that accept JSON
// @Produce json             // For endpoints that return JSON  
// @Security Bearer          // For authenticated endpoints
// @Param paramName path string true "Parameter description"
// @Param request body TypeName true "Request body description"
// @Success 200 {object} ResponseType "Success description"
// @Failure 400 {object} api.ErrorResponse "Error description"
// @Router /endpoint [method]
func (h *Handler) HandlerName(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Annotation Reference

### Core Annotations

| Annotation | Required | Description | Example |
|------------|----------|-------------|---------|
| `@Summary` | ✅ | Brief endpoint description | `@Summary Create a new user` |
| `@Description` | ❌ | Detailed description | `@Description Creates a new user account with validation` |
| `@Tags` | ✅ | Logical grouping | `@Tags User Management` |
| `@Router` | ✅ | HTTP route and method | `@Router /users [post]` |

### HTTP Content Annotations

| Annotation | Usage | Description |
|------------|-------|-------------|
| `@Accept` | Request endpoints | Content types accepted |
| `@Produce` | Response endpoints | Content types returned |

**Common values:**
- `json` → `application/json`
- `xml` → `application/xml`  
- `plain` → `text/plain`
- `html` → `text/html`
- `multipart/form-data` → File uploads

### Parameter Annotations

```go
// Path parameters
@Param paramName path paramType required "Description" example("value")

// Query parameters  
@Param paramName query paramType required "Description" default("default") example("value")

// Request body
@Param request body TypeName required "Description"

// Header parameters
@Param paramName header paramType required "Description"
```

**Parameter Types:**
- `string`, `integer`, `number`, `boolean`
- `array` (use with `collectionFormat`)
- Custom types (reference to Go structs)

### Response Annotations

```go
// Success responses
@Success statusCode {responseType} ResponseTypeName "Description"

// Error responses
@Failure statusCode {responseType} ResponseTypeName "Description"
```

**Response Types:**
- `{object}` - JSON object response
- `{array}` - JSON array response  
- `{string}` - Plain text response
- `{file}` - File download

### Security Annotations

```go
// For endpoints requiring authentication
@Security Bearer

// For public endpoints, omit @Security
```

## Complete Examples

### 1. Simple GET Endpoint

```go
// GetUser handles GET /api/users/{userId}
// @Summary Get user by ID
// @Description Retrieves a specific user by their unique identifier
// @Tags User Management
// @Produce json
// @Security Bearer
// @Param userId path string true "User ID"
// @Success 200 {object} api.UserResponse "User found successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid user ID format"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "User not found"
// @Router /users/{userId} [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### 2. POST Endpoint with Request Body

```go
// CreateUser handles POST /api/users
// @Summary Create a new user
// @Description Creates a new user account with email validation
// @Tags User Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body api.CreateUserRequest true "User creation request"
// @Success 201 {object} api.UserResponse "User created successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 409 {object} api.ErrorResponse "User already exists"
// @Router /users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### 3. GET with Query Parameters

```go
// ListUsers handles GET /api/users
// @Summary List users with filtering
// @Description Retrieves a paginated list of users with optional filtering
// @Tags User Management
// @Produce json
// @Security Bearer
// @Param limit query int false "Maximum number of users" default(50) example(20)
// @Param offset query int false "Number of users to skip" default(0) example(0)
// @Param status query string false "Filter by user status" Enums(active, inactive, pending)
// @Param search query string false "Search users by name or email"
// @Success 200 {array} api.UserResponse "Users retrieved successfully"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /users [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### 4. DELETE Endpoint

```go
// DeleteUser handles DELETE /api/users/{userId}
// @Summary Delete a user
// @Description Permanently deletes a user account and all associated data
// @Tags User Management
// @Security Bearer
// @Param userId path string true "User ID"
// @Success 204 "User deleted successfully"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 404 {object} api.ErrorResponse "User not found"
// @Failure 403 {object} api.ErrorResponse "Insufficient permissions"
// @Router /users/{userId} [delete]
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### 5. File Upload Endpoint

```go
// UploadAvatar handles POST /api/users/{userId}/avatar
// @Summary Upload user avatar
// @Description Uploads and sets a new avatar image for the user
// @Tags User Management
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param userId path string true "User ID"
// @Param avatar formData file true "Avatar image file"
// @Success 200 {object} api.UserResponse "Avatar uploaded successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid file format"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Failure 413 {object} api.ErrorResponse "File too large"
// @Router /users/{userId}/avatar [post]
func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## Defining API Types

### 1. Request Types

```go
// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
    Name     string  `json:"name" validate:"required" example:"John Doe"`
    Email    string  `json:"email" validate:"required,email" example:"john@example.com"`
    Password string  `json:"password" validate:"required,min=8" example:"secretpassword"`
    Role     *string `json:"role,omitempty" example:"admin"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
    Name  string  `json:"name,omitempty" example:"Jane Doe"`
    Email string  `json:"email,omitempty" example:"jane@example.com"`
    Role  *string `json:"role,omitempty" example:"user"`
}
```

### 2. Response Types

```go
// UserResponse represents a user in API responses
type UserResponse struct {
    ID        string    `json:"id" example:"usr_123"`
    Name      string    `json:"name" example:"John Doe"`
    Email     string    `json:"email" example:"john@example.com"`
    Role      string    `json:"role" example:"admin"`
    Status    string    `json:"status" example:"active"`
    CreatedAt time.Time `json:"createdAt" example:"2024-01-15T10:30:00Z"`
    UpdatedAt time.Time `json:"updatedAt" example:"2024-01-16T10:30:00Z"`
}

// UserListResponse represents a paginated list of users
type UserListResponse struct {
    Users      []UserResponse `json:"users"`
    TotalCount int           `json:"totalCount" example:"150"`
    Page       int           `json:"page" example:"1"`
    PerPage    int           `json:"perPage" example:"20"`
}
```

### 3. Error Response (Standardized)

```go
// ErrorResponse represents a standardized error response
type ErrorResponse struct {
    Error     string                 `json:"error" example:"Invalid request data"`
    Details   string                `json:"details,omitempty" example:"Email is required"`
    RequestID string                `json:"requestId,omitempty" example:"req_123"`
    Timestamp time.Time             `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}
```

## Type Annotations

### Field Tags

```go
type ExampleType struct {
    // Required field with validation
    Name string `json:"name" validate:"required" example:"John Doe"`
    
    // Optional field with default
    Age *int `json:"age,omitempty" default:"25" example:"30"`
    
    // Enum field
    Status string `json:"status" enums:"active,inactive,pending" example:"active"`
    
    // Array field
    Tags []string `json:"tags" example:"admin,user"`
    
    // Nested object
    Address *Address `json:"address,omitempty"`
    
    // Map/object field
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

### Validation Tags

swaggo/swag recognizes common validation tags:

```go
type ValidationExample struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=128"`
    Age      int    `json:"age" validate:"min=18,max=120"`
    Website  string `json:"website,omitempty" validate:"url"`
}
```

## Advanced Features

### 1. Custom Response Headers

```go
// @Success 200 {object} api.UserResponse "User retrieved" Headers(X-Rate-Limit-Remaining=500)
// @Success 201 {object} api.UserResponse "User created" Headers(Location=/api/users/123)
```

### 2. Multiple Content Types

```go
// @Accept json,xml
// @Produce json,xml
```

### 3. Array Parameters

```go
// @Param tags query []string false "Filter by tags" collectionFormat(multi)
// Example: ?tags=admin&tags=user
```

### 4. Deprecated Endpoints

```go
// @Summary Old endpoint (deprecated)
// @Deprecated
// @Router /old-endpoint [get]
```

## Main API Information

The main API information is defined in `cmd/main/main.go`:

```go
// @title Brain2 Knowledge Graph API
// @version 1.0.0
// @description A RESTful API for managing personal knowledge graphs.
//
// @contact.name Brain2 API Support
// @contact.url https://github.com/your-org/brain2-backend
// @contact.email support@brain2.example.com
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host api.brain2.example.com
// @BasePath /api
//
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
```

## Best Practices

### 1. Consistent Naming

```go
// ✅ Good - Clear, consistent naming
// @Summary Get user profile
// @Router /users/{userId} [get]

// ❌ Bad - Inconsistent, unclear
// @Summary Gets the user
// @Router /user/{id} [get]
```

### 2. Comprehensive Descriptions

```go
// ✅ Good - Detailed, helpful description
// @Description Retrieves a user's profile information including personal details, 
// @Description preferences, and account status. Requires authentication and 
// @Description appropriate permissions to access other users' profiles.

// ❌ Bad - Too brief
// @Description Gets user info
```

### 3. Proper Error Documentation

```go
// ✅ Good - All possible errors documented
// @Failure 400 {object} api.ErrorResponse "Invalid request format"
// @Failure 401 {object} api.ErrorResponse "Authentication required" 
// @Failure 403 {object} api.ErrorResponse "Insufficient permissions"
// @Failure 404 {object} api.ErrorResponse "User not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"

// ❌ Bad - Missing error cases
// @Failure 400 {object} api.ErrorResponse "Bad request"
```

### 4. Example Values

```go
// ✅ Good - Realistic examples
type User struct {
    ID    string `json:"id" example:"usr_7f8a9b2c"`
    Name  string `json:"name" example:"Alice Johnson"`
    Email string `json:"email" example:"alice.johnson@company.com"`
}

// ❌ Bad - Generic examples
type User struct {
    ID    string `json:"id" example:"123"`
    Name  string `json:"name" example:"string"`
    Email string `json:"email" example:"email"`
}
```

## Common Mistakes

### 1. Missing Router Annotation

```go
// ❌ Error - Missing @Router
// @Summary Get user
func GetUser(w http.ResponseWriter, r *http.Request) {}

// ✅ Fixed
// @Summary Get user
// @Router /users/{userId} [get]
func GetUser(w http.ResponseWriter, r *http.Request) {}
```

### 2. Incorrect Type References

```go
// ❌ Error - Type doesn't exist
// @Success 200 {object} NonExistentType

// ✅ Fixed - Use existing type
// @Success 200 {object} api.UserResponse
```

### 3. Missing Security for Protected Endpoints

```go
// ❌ Error - Missing @Security for protected endpoint
// @Summary Delete user (admin only)
func DeleteUser(w http.ResponseWriter, r *http.Request) {}

// ✅ Fixed
// @Summary Delete user (admin only)
// @Security Bearer
func DeleteUser(w http.ResponseWriter, r *http.Request) {}
```

### 4. Inconsistent Tag Names

```go
// ❌ Error - Inconsistent tags
// @Tags User Management
// @Tags Users
// @Tags user

// ✅ Fixed - Consistent tags
// @Tags User Management
```

## Testing Your Documentation

1. **Generate locally:**
   ```bash
   ./generate-openapi.sh --validate
   ```

2. **Check for errors in output:**
   - Look for parsing errors
   - Verify all types are found
   - Check for missing references

3. **Validate in Swagger UI:**
   - Start your application
   - Visit `/api/swagger-ui`
   - Test endpoints directly in the UI

4. **Review generated files:**
   ```bash
   # Check YAML structure
   cat pkg/api/swagger.yaml
   
   # Validate JSON
   python -m json.tool pkg/api/swagger.json
   ```

---

*Keep this guide updated as new annotation patterns are added to the codebase.*