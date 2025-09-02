# Brain2 Backend Deployment Guide

## Overview

The Brain2 backend now includes a comprehensive testing infrastructure with multiple test levels and deployment workflows optimized for different scenarios.

## Test Infrastructure

### Test Categories

All tests are organized with build tags for selective execution:

- **Unit Tests** (`-tags=unit`): Fast, isolated tests for business logic
- **Integration Tests** (`-tags=integration`): Tests with external dependencies
- **Contract Tests** (`-tags=contracts`): Repository implementation validation
- **BDD Tests** (`-tags=bdd`): Business scenario tests
- **Performance Tests** (`-tags=bench`): Benchmarks for critical paths

### Test Commands

```bash
# Run specific test categories
make test-unit          # Unit tests only
make test-integration   # Integration tests with Docker
make test-contracts     # Contract tests
make test-bdd          # BDD feature tests
make test-bench        # Performance benchmarks

# Comprehensive testing
make test-all          # All test categories
make test-coverage     # Generate coverage report
make test-race         # Run with race detector

# Domain-specific tests
make test-domain       # Domain layer tests
make test-application  # Application layer tests
make test-sagas       # Saga orchestration tests
```

## Build Process

### build.sh Options

The `build.sh` script now integrates with the Makefile for testing:

```bash
# Full build with all tests
./build.sh

# Quick build with unit tests only
./build.sh --quick

# Build with specific test level
./build.sh --test-level all      # Run all tests
./build.sh --test-level unit     # Unit tests only
./build.sh --test-level ci       # CI test suite

# Skip tests (not recommended)
./build.sh --skip-tests

# Build specific component
./build.sh --component main
./build.sh --component cleanup-handler
```

## Deployment Workflows

### 1. Local Development Workflow

For rapid iteration during development:

```bash
# Quick build and test
./dev.sh

# Or manually:
make test-unit
./build.sh --quick --skip-tests
```

### 2. Pre-Commit Workflow

Before committing changes:

```bash
# Format, lint, and test
make fmt
make lint
make test-unit
```

### 3. Full Deployment Workflow

For production deployments:

```bash
# Complete deployment with all tests
./deploy.sh

# Deployment options
./deploy.sh --quick              # Quick deployment (unit tests only)
./deploy.sh --skip-tests         # Skip tests (dangerous!)
./deploy.sh --dry-run            # Preview deployment
./deploy.sh --environment prod   # Deploy to production
```

### 4. CI/CD Workflow

The GitHub Actions workflow automatically:

1. **On Pull Request**:
   - Runs unit tests
   - Performs security scans
   - Checks code formatting
   - Builds Lambda functions

2. **On Push to Main**:
   - Runs full test suite
   - Generates coverage reports
   - Runs performance benchmarks
   - Deploys to dev environment

3. **Manual Deployment**:
   - Choose environment (dev/staging/prod)
   - Select test level
   - Deploy with validation

## Deployment Script Features

The `deploy.sh` script provides:

- **Pre-deployment validation**: Checks dependencies and configuration
- **Flexible testing**: Choose test levels based on deployment type
- **Code quality checks**: Formatting, linting, security scanning
- **Build optimization**: Quick mode for faster iterations
- **Environment support**: Deploy to dev, staging, or production
- **Dry-run mode**: Preview changes without deploying
- **Post-deployment validation**: Health checks and smoke tests
- **Deployment reports**: Track what was deployed and when

## Environment-Specific Deployments

### Development
```bash
./deploy.sh --environment dev --quick
```

### Staging
```bash
./deploy.sh --environment staging
```

### Production
```bash
./deploy.sh --environment prod
# Requires confirmation
# Runs full test suite
# Generates coverage reports
```

## GitHub Actions Integration

### Manual Deployment

Trigger deployment from GitHub Actions UI:

1. Go to Actions tab
2. Select "Deploy Backend" workflow
3. Click "Run workflow"
4. Choose:
   - Branch to deploy
   - Target environment
   - Test level

### Automated Deployment

Commits to main branch automatically:
- Run full test suite
- Build Lambda functions
- Deploy to dev environment
- Run performance benchmarks
- Generate coverage reports

## Performance Monitoring

### Benchmarks

Run performance tests:

```bash
make test-bench
```

Benchmarks cover:
- Saga execution time
- Node builder performance
- Bulk operations
- Concurrent operations

### Metrics

The deployment process tracks:
- Test execution time
- Code coverage percentage
- Build artifact sizes
- Deployment duration

## Troubleshooting

### Common Issues

1. **Tests failing locally but not in CI**:
   - Check Docker is running for integration tests
   - Ensure all dependencies are installed
   - Run `make deps` to update dependencies

2. **Build fails with Wire errors**:
   - Install Wire: `go install github.com/google/wire/cmd/wire@latest`
   - Regenerate: `make wire`

3. **Deployment fails**:
   - Check AWS credentials
   - Verify environment variables
   - Review deployment logs in CloudWatch

### Debug Commands

```bash
# Verbose test output
go test -v ./...

# Test specific package
go test -v ./internal/core/domain/...

# Run single test
go test -run TestNodeCreation ./tests/features/...

# Check build output
ls -la build/
```

## Best Practices

1. **Always run tests before deployment**:
   - Use `./deploy.sh` which includes tests
   - Never use `--skip-tests` in production

2. **Use appropriate test levels**:
   - Development: Unit tests
   - Staging: All tests
   - Production: All tests + benchmarks

3. **Monitor deployments**:
   - Check CloudWatch logs
   - Verify health endpoints
   - Review deployment reports

4. **Version control**:
   - Tag releases
   - Document changes
   - Keep deployment logs

## Quick Reference

```bash
# Development
./dev.sh                          # Quick dev build
make test-unit                    # Run unit tests
./build.sh --quick                # Fast build

# Testing
make test-all                     # All tests
make test-coverage               # Coverage report
make quality                     # Code quality checks

# Deployment
./deploy.sh                      # Full deployment
./deploy.sh --quick              # Quick deployment
./deploy.sh --environment prod   # Production deployment
./deploy.sh --dry-run           # Preview deployment

# CI/CD
make ci                         # CI pipeline
make cd                         # CD pipeline
```

## Support

For issues or questions:
- Check logs in `deployment-report.txt`
- Review CloudWatch logs
- Open an issue on GitHub