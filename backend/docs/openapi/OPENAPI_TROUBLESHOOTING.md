# OpenAPI Generation Troubleshooting Guide

This guide helps diagnose and resolve common issues with OpenAPI specification generation in the Brain2 backend.

## Quick Diagnostics

### Health Check Commands

Run these commands to quickly identify issues:

```bash
# Test OpenAPI generation
./generate-openapi.sh 2>&1 | tee openapi.log

# Check swag installation
which swag && swag --version

# Verify Go environment
go version && go env GOPATH && go env GOMOD

# Check file permissions
ls -la generate-openapi.sh pkg/api/

# Validate current spec (if exists)
python3 -c "import yaml; print('YAML valid')" < pkg/api/swagger.yaml 2>/dev/null || echo "YAML invalid"
```

### Quick Fixes

```bash
# Fix most common issues
go mod tidy && go mod vendor
chmod +x generate-openapi.sh
rm -rf pkg/api/docs.go pkg/api/swagger.* && ./generate-openapi.sh
```

## Common Error Categories

## 1. Installation and Dependency Issues

### Error: `swag command not found`

**Symptoms:**
```bash
📝 Running swag init to generate OpenAPI spec...
./generate-openapi.sh: line 42: swag: command not found
```

**Causes:**
- swag CLI tool not installed
- GOPATH/bin not in PATH
- Go modules not properly configured

**Solutions:**

```bash
# Manual installation
go install github.com/swaggo/swag/cmd/swag@latest

# Add GOBIN to PATH
export PATH="$(go env GOPATH)/bin:$PATH"
echo 'export PATH="$(go env GOPATH)/bin:$PATH"' >> ~/.bashrc

# Verify installation
swag --version
```

**Prevention:**
- The script auto-installs swag, but PATH issues can prevent execution
- Ensure your environment properly configures Go binary paths

### Error: `inconsistent vendoring`

**Symptoms:**
```bash
go: inconsistent vendoring in /home/user/backend:
    github.com/swaggo/swag@v1.16.6: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt
```

**Solution:**
```bash
# Resync vendor directory
go mod tidy
go mod vendor

# Clean and regenerate
rm -rf vendor/
go mod vendor
```

**Root Cause:** Vendor directory is out of sync with go.mod after adding swag dependencies.

## 2. Parsing and Annotation Errors

### Error: `cannot find type definition`

**Symptoms:**
```bash
ParseComment error in file /path/to/handler.go for comment: 
'@Success 200 {object} api.UserResponse "Success"': 
cannot find type definition: api.UserResponse
```

**Causes:**
- Type doesn't exist in any scanned package
- Package import missing in handler file
- Type name misspelled in annotation

**Solutions:**

```bash
# 1. Verify type exists
grep -r "type UserResponse" pkg/api/

# 2. Check if handler imports the package
grep -r "brain2-backend/pkg/api" internal/interfaces/http/v1/handlers/

# 3. Add missing import
```

**Fix in handler file:**
```go
import (
    // ... other imports
    "brain2-backend/pkg/api"  // Add this import
)
```

### Error: `parsing failed for comment`

**Symptoms:**
```bash
ParseComment error: parsing failed for comment: '@Router /users/{id} [get'
```

**Common Annotation Mistakes:**

| Error | Cause | Fix |
|-------|-------|-----|
| `@Router /path [method` | Missing closing bracket | `@Router /path [method]` |
| `@Param id path int true` | Missing description quotes | `@Param id path int true "User ID"` |
| `@Success 200 object api.Type` | Missing braces | `@Success 200 {object} api.Type` |
| `@Tags User Management, Admin` | Invalid tag syntax | Use separate `@Tags` lines |

**Solution Pattern:**
```go
// ❌ Incorrect
// @Router /users/{id} [get
// @Param id path int true Description
// @Success 200 object api.User

// ✅ Correct
// @Router /users/{id} [get]
// @Param id path int true "User ID"  
// @Success 200 {object} api.User
```

### Error: `duplicate operation id`

**Symptoms:**
```bash
Error: duplicate operation ID 'main.GetUser'
```

**Cause:** Multiple handlers with the same function name or operation ID.

**Solution:**
```go
// Add explicit operation IDs
// @Summary Get user by ID
// @ID getUserById
// @Router /users/{id} [get]

// @Summary Get current user  
// @ID getCurrentUser
// @Router /users/me [get]
```

## 3. Type Resolution Issues

### Error: `circular dependency detected`

**Symptoms:**
```bash
Error: circular dependency detected in type definitions
```

**Causes:**
- Types referencing each other in a loop
- Embedded struct circular references

**Example Problem:**
```go
type User struct {
    Profile *UserProfile `json:"profile"`
}

type UserProfile struct {
    User *User `json:"user"`  // Circular reference
}
```

**Solutions:**
```go
// Option 1: Remove circular reference
type User struct {
    Profile *UserProfile `json:"profile"`
}

type UserProfile struct {
    UserID string `json:"userId"`  // Reference by ID instead
}

// Option 2: Use interface{} for complex relationships
type User struct {
    Profile interface{} `json:"profile,omitempty"`
}
```

### Error: `unsupported type`

**Symptoms:**
```bash
Warning: unsupported type 'context.Context' in field
```

**Unsupported Types:**
- `context.Context`
- `sync.Mutex` 
- Function types
- Channels
- Complex interface types

**Solutions:**
```go
// ❌ Problematic
type Request struct {
    Context context.Context `json:"context"`  // Unsupported
    Handler func() error    `json:"handler"`  // Unsupported
}

// ✅ Fixed
type Request struct {
    // Remove unsupported fields or use json:"-"
    Context context.Context `json:"-"`
    Data    RequestData     `json:"data"`
}
```

## 4. Generation and Build Issues

### Error: `permission denied`

**Symptoms:**
```bash
./generate-openapi.sh: Permission denied
```

**Solution:**
```bash
chmod +x generate-openapi.sh
```

### Error: `failed to create docs.go`

**Symptoms:**
```bash
Error: failed to create docs.go at pkg/api/docs.go: permission denied
```

**Solutions:**
```bash
# Check directory permissions
ls -la pkg/api/

# Fix permissions
chmod 755 pkg/api/
chmod 644 pkg/api/*

# Remove existing files if corrupted
rm -f pkg/api/docs.go pkg/api/swagger.*
./generate-openapi.sh
```

### Error: `go:embed` compilation error

**Symptoms:**
```bash
pkg/api/swagger.go:13:12: pattern swagger.yaml: no matching files found
```

**Cause:** Generated YAML file missing or corrupted.

**Solution:**
```bash
# Regenerate OpenAPI files
rm -f pkg/api/swagger.yaml pkg/api/swagger.json
./generate-openapi.sh

# Verify files exist
ls -la pkg/api/swagger.*
```

## 5. Runtime and Integration Issues

### Error: Swagger UI not accessible

**Symptoms:**
- `/api/swagger-ui` returns 404
- `/api/docs` not found

**Debugging:**
```bash
# Check if routes are registered
grep -r "swagger-ui\|/docs" internal/di/

# Verify handler registration
grep -r "SwaggerUIHandler\|SwaggerHandler" internal/di/
```

**Solution:** Ensure routes are properly registered in router configuration.

### Error: Swagger spec returns empty

**Symptoms:**
- `/api/swagger` returns empty response
- Embedded spec not loading

**Solution:**
```bash
# Verify embedded files
go run -tags debug ./cmd/main &
curl http://localhost:8080/api/swagger
```

**Check embedding:**
```go
// In pkg/api/swagger.go, verify:
//go:embed swagger.yaml
var swaggerYAML []byte
```

## 6. Validation Errors

### Error: OpenAPI validation failed

**Symptoms:**
```bash
Spectral validation failed:
/api/paths//users/{id}/get/responses/200/content is not truthy
```

**Common Validation Issues:**

| Error | Fix |
|-------|-----|
| Missing required fields | Add required annotation fields |
| Invalid schema references | Check type names and paths |
| Duplicate operation IDs | Add explicit `@ID` annotations |
| Invalid parameter types | Use supported OpenAPI types |

**Debug validation:**
```bash
# Install spectral for detailed validation
npm install -g @stoplight/spectral-cli

# Run validation manually
spectral lint pkg/api/swagger.yaml --ruleset spectral:oas
```

## Debugging Workflow

### Step-by-Step Diagnosis

1. **Check Environment**
   ```bash
   go version
   which swag || echo "swag not found"
   echo $GOPATH
   echo $PATH
   ```

2. **Test Basic Generation**
   ```bash
   ./generate-openapi.sh > debug.log 2>&1
   cat debug.log
   ```

3. **Verify Source Files**
   ```bash
   # Check for annotation syntax
   grep -n "@" internal/interfaces/http/v1/handlers/*.go
   
   # Look for incomplete annotations
   grep -B2 -A2 "@Router\|@Summary\|@Success" internal/interfaces/http/v1/handlers/*.go
   ```

4. **Check Type Definitions**
   ```bash
   # Find all API types
   grep -r "type.*struct" pkg/api/
   
   # Check for naming conflicts
   grep -r "type.*Response" pkg/api/ internal/
   ```

5. **Validate Dependencies**
   ```bash
   go mod verify
   go mod tidy
   go list -m all | grep swag
   ```

## Advanced Debugging

### Enable Debug Mode

```bash
# Run swag with verbose output
swag init \
    --generalInfo ./cmd/main/main.go \
    --dir ./ \
    --output ./pkg/api \
    --parseDependency \
    --debug \
    2>&1 | tee swag-debug.log
```

### Inspect Generated Output

```bash
# Check generated Go code
head -50 pkg/api/docs.go

# Validate JSON structure
jq '.' pkg/api/swagger.json > /dev/null && echo "JSON valid" || echo "JSON invalid"

# Check YAML structure
python3 -c "import yaml; yaml.safe_load(open('pkg/api/swagger.yaml'))" && echo "YAML valid" || echo "YAML invalid"
```

### Performance Debugging

```bash
# Time generation steps
time swag init --generalInfo ./cmd/main/main.go --dir ./ --output ./tmp

# Check file sizes
du -h pkg/api/swagger.*
wc -l pkg/api/swagger.yaml

# Memory usage during generation
/usr/bin/time -v swag init --generalInfo ./cmd/main/main.go --dir ./ --output ./tmp 2>&1 | grep -E "Maximum resident|User time|System time"
```

## Prevention Strategies

### Pre-commit Hooks

```bash
#!/bin/bash
# .git/hooks/pre-commit
echo "Validating OpenAPI generation..."
./generate-openapi.sh --validate || exit 1
echo "OpenAPI validation passed ✅"
```

### IDE Integration

**VS Code Settings:**
```json
{
    "go.buildTags": "swaggo",
    "go.lintTool": "golangci-lint",
    "files.associations": {
        "*.go": "go"
    }
}
```

### Development Checklist

Before committing handler changes:

- [ ] Added all required annotations (`@Summary`, `@Router`)
- [ ] Defined request/response types in `pkg/api/types.go`
- [ ] Added proper import statements
- [ ] Tested generation locally: `./generate-openapi.sh`
- [ ] Verified Swagger UI renders correctly
- [ ] Checked for no parsing errors in output

## Recovery Procedures

### Complete Reset

```bash
#!/bin/bash
# complete-openapi-reset.sh
echo "🔄 Performing complete OpenAPI system reset..."

# Remove generated files
rm -f pkg/api/docs.go pkg/api/swagger.json pkg/api/swagger.yaml

# Clean Go modules
go clean -modcache
go mod download

# Reinstall swag
go install github.com/swaggo/swag/cmd/swag@latest

# Regenerate vendor
go mod tidy
go mod vendor

# Test generation
./generate-openapi.sh --validate

echo "✅ OpenAPI system reset complete"
```

### Backup and Restore

```bash
# Backup working specification
cp pkg/api/swagger.yaml pkg/api/swagger.yaml.backup

# Restore from backup
cp pkg/api/swagger.yaml.backup pkg/api/swagger.yaml
```

## Getting Help

### Log Collection

When reporting issues, collect these logs:

```bash
# System info
go version > debug-info.txt
which swag >> debug-info.txt
echo "GOPATH: $GOPATH" >> debug-info.txt
echo "PATH: $PATH" >> debug-info.txt

# Generation logs
./generate-openapi.sh --validate > openapi-debug.log 2>&1

# Build logs  
./build.sh > build-debug.log 2>&1

# File listings
ls -la pkg/api/ >> debug-info.txt
ls -la internal/interfaces/http/v1/handlers/ >> debug-info.txt
```

### Community Resources

- **swaggo/swag Issues**: https://github.com/swaggo/swag/issues
- **OpenAPI Specification**: https://spec.openapis.org/oas/v3.0.3
- **Swagger Documentation**: https://swagger.io/docs/

### Internal Support

For Brain2-specific issues:
1. Check existing documentation in `docs/`
2. Search closed issues in project repository
3. Create detailed issue with debug logs
4. Include minimal reproduction case

---

*This troubleshooting guide is updated based on real-world issues encountered during development.*