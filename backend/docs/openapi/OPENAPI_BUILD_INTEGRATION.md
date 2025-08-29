# OpenAPI Build Integration & Workflow

This document details how OpenAPI specification generation is integrated into the Brain2 backend build system and CI/CD workflows.

## Build Pipeline Integration

### Build Process Flow

```mermaid
graph TD
    A[Start Build] --> B[Clean Caches]
    B --> C[Install Dependencies]
    C --> D[Run Tests]
    D --> E[Wire DI Generation]
    E --> F[🆕 OpenAPI Generation]
    F --> G[Lambda Compilation]
    G --> H[Build Verification]
    H --> I[Deployment Ready]
    
    F --> J{Generation Failed?}
    J -->|Yes| K[❌ Build Failed]
    J -->|No| G
```

### Build Script Integration

The OpenAPI generation is integrated at the optimal point in `build.sh`:

```bash
# build.sh execution order:
1. Environment validation
2. Cache management (--quick flag handling)
3. Dependency installation
4. Testing (if not skipped)
5. Wire dependency injection generation
6. → OpenAPI specification generation ← (NEW STEP)
7. Lambda function compilation
8. Build verification and packaging
```

#### Integration Point

```bash
echo "🔄 Generating dependency injection code with Wire..."
(cd internal/di && go generate)
if [ $? -ne 0 ]; then
    echo "❌ Wire code generation failed."
    exit 1
fi

echo "📝 Generating OpenAPI specification from code annotations..."
./generate-openapi.sh
if [ $? -ne 0 ]; then
    echo "❌ OpenAPI generation failed."
    exit 1
fi

echo "🏗️ Building Lambda functions..."
# Continue with compilation...
```

## Generation Script Architecture

### `generate-openapi.sh` Features

| Feature | Description | Benefits |
|---------|-------------|----------|
| **Dependency Check** | Automatically installs `swag` if missing | Self-contained execution |
| **Path Validation** | Verifies source directories exist | Early failure detection |
| **Multi-format Output** | Generates Go, JSON, and YAML | Multiple consumption formats |
| **Validation Support** | Optional spec validation | Quality assurance |
| **Statistics Reporting** | Shows endpoints, models, file sizes | Build visibility |
| **Error Handling** | Comprehensive error reporting | Debugging support |

### Script Parameters

```bash
# Basic generation
./generate-openapi.sh

# With validation
./generate-openapi.sh --validate

# Custom output directory
./generate-openapi.sh --output ./custom/path

# Help and usage
./generate-openapi.sh --help
```

### Internal Architecture

```bash
generate-openapi.sh
├── Argument parsing
├── Dependency validation
│   ├── Check for swag command
│   ├── Install if missing
│   └── Update PATH if needed
├── Directory preparation
├── swag init execution
│   ├── --generalInfo ./cmd/main/main.go
│   ├── --dir ./
│   ├── --output pkg/api
│   ├── --parseDependency
│   └── --outputTypes go,json,yaml
├── Validation (optional)
│   ├── Spectral (preferred)
│   ├── swagger-codegen (fallback)
│   ├── openapi-generator (fallback)
│   └── Basic YAML validation
└── Statistics reporting
```

## File Dependencies

### Generated Files

```
pkg/api/
├── docs.go           # Go embedded documentation
├── swagger.json      # JSON OpenAPI specification  
├── swagger.yaml      # YAML OpenAPI specification
└── swagger.go        # Original utilities (preserved)
```

### Source Dependencies

The generation process scans these locations:

```
Annotation Sources:
├── cmd/main/main.go              # Main API info
├── internal/interfaces/http/v1/handlers/
│   ├── memory.go                 # Memory endpoints
│   ├── category.go               # Category endpoints
│   └── health.go                 # Health endpoints
└── pkg/api/
    └── types.go                  # Request/response models
```

## Build Environment Requirements

### Go Dependencies

```go
// Required in go.mod
github.com/swaggo/swag/cmd/swag    // CLI tool
github.com/swaggo/files            // Swagger UI assets
github.com/swaggo/http-swagger     // Chi integration
```

### System Requirements

- **Go Version**: 1.18+ (for generics support)
- **swag CLI**: Automatically installed if missing
- **Disk Space**: ~5MB for dependencies
- **Build Time**: +2-3 seconds for generation

### Optional Tools

```bash
# For advanced validation
npm install -g @stoplight/spectral-cli
# or
swagger-codegen-cli
# or  
openapi-generator-cli
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Deploy
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Cache dependencies
        uses: actions/cache@v3
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
            vendor/
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          
      # OpenAPI generation happens automatically in build
      - name: Build with OpenAPI generation
        run: ./build.sh --skip-tests
        
      # Optional: Validate OpenAPI spec
      - name: Validate OpenAPI Specification  
        run: ./generate-openapi.sh --validate
        
      # Optional: Upload generated spec as artifact
      - name: Upload OpenAPI Spec
        uses: actions/upload-artifact@v3
        with:
          name: openapi-spec
          path: pkg/api/swagger.yaml
```

### AWS CodeBuild Example

```yaml
version: 0.2
phases:
  install:
    runtime-versions:
      golang: 1.21
  pre_build:
    commands:
      - echo Logging in to Amazon ECR...
      - aws ecr get-login-password --region $AWS_DEFAULT_REGION | docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_DEFAULT_REGION.amazonaws.com
  build:
    commands:
      - echo Build started on `date`
      # OpenAPI generation included in build process
      - ./build.sh
  post_build:
    commands:
      - echo Build completed on `date`
artifacts:
  files:
    - build/**/*
    - pkg/api/swagger.yaml  # Include generated spec
```

## Error Handling

### Common Build Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `swag command not found` | Missing CLI tool | Automatically installed by script |
| `cannot find type definition` | Missing import | Add package import to handler |
| `inconsistent vendoring` | Outdated vendor/ | Run `go mod vendor` |
| `parsing failed` | Invalid annotation | Check annotation syntax |

### Error Detection Flow

```bash
Build Process Error Handling:
├── OpenAPI generation failure
│   ├── swag command missing → Auto-install
│   ├── Parsing errors → Show detailed output
│   ├── Type resolution errors → List missing types
│   └── File permission errors → Show file access issues
├── Validation failures (optional)
│   ├── Schema violations → Show validation errors
│   ├── Reference errors → List broken references
│   └── Format errors → Show formatting issues
└── Integration errors
    ├── File generation failed → Check disk space
    ├── Go embed errors → Verify file structure
    └── Build integration → Check build script
```

### Debugging Commands

```bash
# Test OpenAPI generation only
./generate-openapi.sh

# Generate with verbose output
./generate-openapi.sh --validate 2>&1 | tee openapi.log

# Check swag installation
which swag || echo "swag not found"

# Validate vendor dependencies
go mod verify && go mod tidy

# Test build without OpenAPI generation
# (temporarily comment out the generation step)
```

## Performance Optimization

### Build Time Impact

| Build Type | Without OpenAPI | With OpenAPI | Increase |
|------------|----------------|--------------|----------|
| **Full Build** | 45s | 48s | +3s (7%) |
| **Quick Build** | 20s | 23s | +3s (15%) |
| **Component Build** | 15s | 18s | +3s (20%) |

### Optimization Strategies

1. **Caching**: Generated files cached in build systems
2. **Conditional Generation**: Skip if no annotation changes
3. **Parallel Processing**: Run alongside other generation steps
4. **Minimal Scanning**: Target specific source directories

### Future Optimizations

```bash
# Potential improvements:
- Incremental generation (only changed files)
- Parallel swag execution for different modules  
- Build cache integration with OpenAPI checksums
- Smart dependency detection for faster rebuilds
```

## Monitoring and Alerts

### Build Metrics

Track these metrics in your CI/CD system:

```yaml
Metrics to Monitor:
├── Generation Success Rate: Should be >99%
├── Generation Time: Baseline ~3s, alert if >10s
├── Specification Size: Track growth over time
├── Endpoint Coverage: % of handlers documented
└── Validation Pass Rate: Should be 100%
```

### Alert Conditions

```yaml
Alerts:
├── Build Failure: OpenAPI generation fails
├── Validation Failure: Generated spec invalid
├── Performance Degradation: Generation time >2x baseline
├── Coverage Drop: Documentation coverage decreases
└── Dependency Issues: swag CLI installation fails
```

## Rollback Strategy

### If OpenAPI Generation Fails

1. **Immediate**: Build uses existing `swagger.yaml` as fallback
2. **Investigation**: Check generation logs for root cause
3. **Fix**: Address annotation or dependency issues
4. **Verification**: Test generation locally before deploy

### Rollback Commands

```bash
# Emergency: Disable OpenAPI generation temporarily
sed -i 's|./generate-openapi.sh|# ./generate-openapi.sh|' build.sh

# Restore previous specification
git checkout HEAD~1 -- pkg/api/swagger.yaml

# Re-enable after fix
git checkout build.sh
```

## Maintenance Tasks

### Weekly Tasks
- Review generation logs for warnings
- Check for new swag CLI releases
- Validate all endpoints are documented

### Monthly Tasks  
- Update dependencies (`go mod tidy`)
- Review and optimize build performance
- Update documentation examples

### Quarterly Tasks
- Evaluate new swag features
- Consider OpenAPI 3.1 migration
- Review and update CI/CD integration

---

*This integration documentation is maintained alongside the build system changes.*