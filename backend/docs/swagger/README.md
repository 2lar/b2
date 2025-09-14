# OpenAPI/Swagger Documentation Setup

## Installation

### 1. Install Swag CLI tool

```bash
# Install swag CLI
go install github.com/swaggo/swag/cmd/swag@latest

# Verify installation
swag --version
```

### 2. Install Swagger dependencies

Add to your `go.mod`:

```bash
go get -u github.com/swaggo/gin-swagger
go get -u github.com/swaggo/files
go get -u github.com/swaggo/swag
```

## Generate Documentation

### Using Make (recommended)

```bash
# Generate Swagger documentation
make swagger

# Generate and serve documentation
make swagger-serve
```

### Manual generation

```bash
# Initialize and generate docs
swag init -g docs/swagger/main.go -o docs/api --parseDependency --parseInternal

# Format the generated files
swag fmt
```

## Integration with Gin

Add this to your router setup:

```go
import (
    "github.com/swaggo/gin-swagger"
    "github.com/swaggo/files"
    _ "backend/docs/api" // Import generated docs
)

func SetupRouter() *gin.Engine {
    router := gin.Default()

    // ... your routes ...

    // Swagger endpoint
    router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

    return router
}
```

## Accessing Documentation

Once the server is running, access the Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

## Documentation Structure

```
docs/
├── swagger/
│   ├── main.go              # Main API documentation and metadata
│   ├── models.go            # Core request/response models
│   ├── additional_models.go # Additional model definitions
│   └── README.md           # This file
├── api/                    # Generated documentation (git-ignored)
│   ├── swagger.json
│   ├── swagger.yaml
│   └── docs.go
interfaces/http/rest/handlers/
├── node_handler_docs.go    # Node endpoints documentation
├── graph_handler_docs.go   # Graph endpoints documentation
├── edge_handler_docs.go    # Edge endpoints documentation
├── search_handler_docs.go  # Search endpoints documentation
└── operation_handler_docs.go # Operation endpoints documentation
```

## Writing Documentation

### Basic endpoint documentation

```go
// GetNode retrieves a node by ID
// @Summary Get node by ID
// @Description Retrieves complete node information
// @Tags nodes
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} docs.NodeResponse
// @Failure 404 {object} docs.ErrorResponse
// @Security BearerAuth
// @Router /nodes/{id} [get]
func (h *NodeHandler) GetNode(c *gin.Context) {
    // Implementation
}
```

### Model documentation

```go
// NodeResponse represents a complete node object
// @Description Complete node information
type NodeResponse struct {
    // Unique node identifier
    // @example "550e8400-e29b-41d4-a716-446655440000"
    ID string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`

    // Node title
    // @example "Understanding CQRS"
    Title string `json:"title" example:"Understanding CQRS"`
}
```

## Best Practices

1. **Keep documentation close to code**: Place handler documentation in separate `*_docs.go` files in the same package

2. **Use examples**: Always provide realistic examples for request/response bodies

3. **Document all status codes**: Include all possible HTTP status codes and their meanings

4. **Group endpoints**: Use tags to group related endpoints together

5. **Version your API**: Include version in the base path (e.g., `/api/v1`)

## Validation

After generating documentation, validate it:

```bash
# Validate swagger.json
swagger validate docs/api/swagger.json

# Or use online validator
# https://editor.swagger.io/
```

## Continuous Integration

Add to your CI pipeline:

```yaml
- name: Generate Swagger docs
  run: |
    go install github.com/swaggo/swag/cmd/swag@latest
    make swagger

- name: Check if docs are up to date
  run: |
    git diff --exit-code docs/api/
```

## Troubleshooting

### Common Issues

1. **Swag command not found**
   - Ensure `$GOPATH/bin` is in your PATH
   - Try: `export PATH=$PATH:$(go env GOPATH)/bin`

2. **Models not recognized**
   - Use `--parseDependency` flag
   - Ensure models are in the same module

3. **Annotations not picked up**
   - Check annotation format
   - Ensure handler files are in the scan path

## Additional Resources

- [Swag Documentation](https://github.com/swaggo/swag)
- [Swagger Specification](https://swagger.io/specification/)
- [Gin-swagger Integration](https://github.com/swaggo/gin-swagger)