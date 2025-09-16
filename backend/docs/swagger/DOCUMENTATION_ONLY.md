# ⚠️ DOCUMENTATION GENERATION ONLY

## IMPORTANT: This Directory is NOT Part of the Application

This directory (`backend/docs/swagger/`) contains code used **EXCLUSIVELY** for generating OpenAPI/Swagger documentation.

### Key Points:
- ❌ **NOT** part of the running application
- ❌ **NOT** imported by any runtime code
- ❌ **NOT** tested or executed during normal operation
- ✅ **ONLY** scanned by `swag` tool during documentation generation

### Purpose:
The files here define models and API structures that the `swag` tool uses to generate the OpenAPI specification (`openapi.yaml`). This separation allows us to control exactly what appears in the API documentation without affecting the actual application code.

### Files:
- `main.go` - API metadata and general documentation
- `models.go` - Core request/response models for documentation
- `additional_models.go` - Additional type definitions for API docs

### Why Separate?
1. **Clean separation** - Documentation concerns don't pollute domain models
2. **Flexibility** - Can simplify complex internal models for API consumers
3. **Control** - Choose exactly what to expose in public API documentation

### Maintenance:
When updating domain models, remember to:
1. Check if the change affects the public API
2. Update corresponding models in this directory if needed
3. Regenerate documentation: `make swagger`
4. Verify changes in generated `docs/api/swagger.yaml`

### Preventing Confusion:
All files in this directory use the build tag to exclude from normal builds:
```go
//go:build swagger
// +build swagger
```

This ensures they're only used during documentation generation, not runtime.

### Commands:
```bash
# Generate documentation
make swagger

# View documentation
make swagger-serve
# Then visit: http://localhost:8080/swagger/index.html
```

### Risk Mitigation:
To prevent documentation drift:
1. CI/CD pipeline validates documentation is up-to-date
2. Pre-commit hooks can auto-generate on model changes
3. Regular audits ensure documentation matches implementation

---

**Remember**: If you're looking for the actual application code, check:
- Domain models: `backend/domain/core/`
- API handlers: `backend/interfaces/http/rest/handlers/`
- Application logic: `backend/application/`