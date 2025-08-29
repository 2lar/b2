# Brain2 Backend Documentation

Welcome to the Brain2 backend documentation. This directory contains comprehensive documentation for the automated OpenAPI specification generation system and other backend components.

## 📚 Documentation Overview

### OpenAPI Documentation System

| Document | Purpose | Audience |
|----------|---------|----------|
| **[OpenAPI Generation Overview](OPENAPI_GENERATION.md)** | System architecture and benefits | All developers |
| **[Developer Guide](OPENAPI_DEVELOPER_GUIDE.md)** | Annotation patterns and examples | Backend developers |
| **[Build Integration](OPENAPI_BUILD_INTEGRATION.md)** | CI/CD and build system integration | DevOps engineers |
| **[Troubleshooting Guide](OPENAPI_TROUBLESHOOTING.md)** | Error diagnosis and resolution | All developers |

### Architecture Documentation

| Document | Purpose |
|----------|---------|
| **[Architecture Decisions](ARCHITECTURE_DECISIONS.md)** | ADRs for key architectural choices |
| **[Dependency Injection Patterns](DEPENDENCY_INJECTION_PATTERNS.md)** | DI container and Wire patterns |
| **[API Layer Flow](API_LAYER_FLOW.md)** | Request/response flow documentation |

## 🚀 Quick Start

### For New Developers

1. **Read the Overview**: Start with [OPENAPI_GENERATION.md](OPENAPI_GENERATION.md) to understand the system
2. **Learn Annotations**: Review [OPENAPI_DEVELOPER_GUIDE.md](OPENAPI_DEVELOPER_GUIDE.md) for practical examples
3. **Test Locally**: Run `./generate-openapi.sh` to generate documentation
4. **View Results**: Access Swagger UI at `/api/swagger-ui` when running the application

### For DevOps Engineers

1. **Build Integration**: Review [OPENAPI_BUILD_INTEGRATION.md](OPENAPI_BUILD_INTEGRATION.md)
2. **CI/CD Setup**: Implement the provided pipeline examples
3. **Monitoring**: Set up alerts for generation failures
4. **Troubleshooting**: Bookmark [OPENAPI_TROUBLESHOOTING.md](OPENAPI_TROUBLESHOOTING.md)

### For API Consumers

1. **Interactive Documentation**: Visit `/api/swagger-ui` for full API documentation
2. **OpenAPI Spec**: Download specification from `/api/swagger.yaml` or `/api/swagger.json`
3. **Authentication**: All endpoints require JWT Bearer tokens (except health checks)

## 🔧 System Architecture

The Brain2 backend implements automated OpenAPI generation using the following components:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Go Code       │    │   swaggo/swag    │    │   Generated     │
│   Annotations   │───▶│   Generator      │───▶│   OpenAPI       │
│                 │    │                  │    │   Specification │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Key Features

- ✅ **Automated Generation**: OpenAPI spec generated from code annotations
- ✅ **Build Integration**: Automatic generation during build process
- ✅ **Type Safety**: All request/response models properly typed
- ✅ **Interactive UI**: Swagger UI for testing and exploration
- ✅ **Multiple Formats**: Go, JSON, and YAML output formats
- ✅ **Validation Support**: Optional specification validation
- ✅ **Always Current**: Documentation stays in sync with code

## 📊 Generated Documentation Stats

Current API documentation includes:

| Metric | Count | Coverage |
|--------|-------|----------|
| **API Endpoints** | 13 | 100% |
| **HTTP Methods** | 4 | GET, POST, PUT, DELETE |
| **Request Models** | 6 | With validation and examples |
| **Response Models** | 9 | Comprehensive error handling |
| **Authentication** | JWT | Bearer token security |
| **Documentation Tags** | 4 | Logical endpoint grouping |

## 🛠️ Development Workflow

### Adding New Endpoints

1. **Create Handler**: Implement HTTP handler function
2. **Add Annotations**: Document with swaggo comments
3. **Define Types**: Add request/response types to `pkg/api/types.go`
4. **Test Generation**: Run `./generate-openapi.sh --validate`
5. **Verify Documentation**: Check Swagger UI output
6. **Build and Deploy**: Normal build process includes generation

### Example Handler Documentation

```go
// CreateUser handles POST /api/users
// @Summary Create a new user account
// @Description Creates a new user with email validation and role assignment
// @Tags User Management
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body api.CreateUserRequest true "User creation data"
// @Success 201 {object} api.UserResponse "User created successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Authentication required"
// @Router /users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

## 📋 Common Tasks

### Generate Documentation

```bash
# Basic generation
./generate-openapi.sh

# With validation
./generate-openapi.sh --validate

# Custom output location
./generate-openapi.sh --output ./docs/api
```

### Build with Documentation

```bash
# Full build (includes OpenAPI generation)
./build.sh

# Quick incremental build
./build.sh --quick

# Build specific component
./build.sh --component main
```

### Access Documentation

```bash
# Start application locally
go run ./cmd/main

# Access Swagger UI
open http://localhost:8080/api/swagger-ui

# Get OpenAPI spec
curl http://localhost:8080/api/swagger.yaml
```

## 🐛 Troubleshooting

### Quick Diagnostics

```bash
# Test OpenAPI generation
./generate-openapi.sh 2>&1 | tee openapi.log

# Check dependencies
which swag && swag --version
go mod verify

# Validate existing spec
python3 -c "import yaml; yaml.safe_load(open('pkg/api/swagger.yaml'))"
```

### Common Issues

| Issue | Quick Fix |
|-------|-----------|
| `swag command not found` | Run `go install github.com/swaggo/swag/cmd/swag@latest` |
| `inconsistent vendoring` | Run `go mod tidy && go mod vendor` |
| `cannot find type definition` | Add package import to handler file |
| `parsing failed` | Check annotation syntax in handler |

For detailed troubleshooting, see [OPENAPI_TROUBLESHOOTING.md](OPENAPI_TROUBLESHOOTING.md).

## 📈 Monitoring and Metrics

### Build System Integration

The OpenAPI generation is monitored as part of the build process:

- **Success Rate**: Should maintain >99% generation success
- **Generation Time**: Baseline ~3 seconds, alert if >10 seconds
- **Specification Size**: Track growth and complexity over time
- **Documentation Coverage**: Ensure all endpoints are documented

### Quality Metrics

- **Validation Pass Rate**: 100% (build fails on validation errors)
- **Type Coverage**: All request/response models typed
- **Example Completeness**: All models include example values
- **Security Coverage**: Authentication documented for all protected endpoints

## 🔮 Future Enhancements

### Planned Improvements

- **Code Generation**: Generate client SDKs from OpenAPI specifications
- **Contract Testing**: Validate API responses against generated schemas
- **Multi-Version Support**: Handle API versioning in documentation
- **Performance Monitoring**: Track documentation generation performance
- **Advanced Validation**: Implement custom OpenAPI linting rules

### Community Contributions

The OpenAPI generation system is designed to be extensible. Contributions welcome for:

- Additional annotation patterns
- New validation rules
- Performance optimizations
- Documentation improvements
- Tooling enhancements

## 📞 Support

### Internal Resources

- **Documentation**: This directory contains comprehensive guides
- **Code Examples**: See handler files for annotation patterns
- **Build Integration**: Check `build.sh` and `generate-openapi.sh`
- **Generated Output**: Review `pkg/api/` directory

### External Resources

- **swaggo/swag**: https://github.com/swaggo/swag
- **OpenAPI Specification**: https://spec.openapis.org/oas/v3.0.3
- **Swagger UI**: https://swagger.io/tools/swagger-ui/

### Getting Help

1. **Check Documentation**: Start with the appropriate guide above
2. **Review Troubleshooting**: See [OPENAPI_TROUBLESHOOTING.md](OPENAPI_TROUBLESHOOTING.md)
3. **Search Issues**: Look for similar problems in project history
4. **Create Issue**: Include debug logs and minimal reproduction case

---

## 📝 Documentation Maintenance

This documentation is maintained alongside code changes. When modifying the OpenAPI system:

1. **Update Relevant Docs**: Modify documentation for changed functionality
2. **Test Examples**: Ensure all code examples work correctly
3. **Update Metrics**: Refresh statistics and performance numbers
4. **Review Accuracy**: Verify all information remains current

*Last updated: Generated automatically with OpenAPI implementation*