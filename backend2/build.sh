#!/bin/bash
# Backend2 Build Script - DDD/CQRS Architecture
# Comprehensive build system for Brain2 backend services
#
# USAGE EXAMPLES:
# ===============
# Full build (cleans all caches, rebuilds everything):
#   ./build.sh
#
# Quick build (incremental, no cache clearing):
#   ./build.sh --quick
#
# Skip tests for faster builds:
#   ./build.sh --skip-tests
#   ./build.sh --quick --skip-tests
#
# Build specific component only:
#   ./build.sh --component api
#   ./build.sh --component migrate
#   ./build.sh --component worker
#
# Build with race detection (for debugging):
#   ./build.sh --race
#
# Build with debug symbols:
#   ./build.sh --debug
#
# Run with linting:
#   ./build.sh --lint
#
# Quick component build:
#   ./build.sh --component api --quick --skip-tests
#
# AVAILABLE COMPONENTS:
# ====================
# Local Services:
# • api               - Main REST API server (DDD/CQRS handlers)
# • migrate           - Database migration tool
# • worker            - Background job processor for async operations
#
# Lambda Functions:
# • cleanup-handler   - Async cleanup Lambda for resource management
# • connect-node      - Node connection discovery Lambda
# • ws-connect        - WebSocket connection handler
# • ws-disconnect     - WebSocket disconnection handler
# • ws-send-message   - WebSocket message broadcaster

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse command line arguments
SKIP_TESTS=false
QUICK_BUILD=false
SPECIFIC_COMPONENT=""
TEST_LEVEL="unit"
ENABLE_RACE=false
DEBUG_BUILD=false
RUN_LINT=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --quick)
            QUICK_BUILD=true
            shift
            ;;
        --component)
            SPECIFIC_COMPONENT="$2"
            shift 2
            ;;
        --test-level)
            TEST_LEVEL="$2"  # unit, integration, e2e, all, coverage
            shift 2
            ;;
        --race)
            ENABLE_RACE=true
            shift
            ;;
        --debug)
            DEBUG_BUILD=true
            shift
            ;;
        --lint)
            RUN_LINT=true
            shift
            ;;
        --help|-h)
            echo "Backend2 Build Script"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --skip-tests     Skip running tests"
            echo "  --quick          Skip cache clearing for faster incremental builds"
            echo "  --component      Build only specified component (see list below)"
            echo "  --test-level     Test level: unit, integration, e2e, all, coverage (default: unit)"
            echo "  --race           Enable race detection in builds"
            echo "  --debug          Build with debug symbols (larger binaries)"
            echo "  --lint           Run linting and formatting checks"
            echo "  --help, -h       Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Full build with unit tests"
            echo "  $0 --test-level all                  # Full build with all tests"
            echo "  $0 --quick --skip-tests              # Fast incremental build"
            echo "  $0 --component api --quick           # Quick build of API only"
            echo "  $0 --lint --test-level coverage     # Build with linting and coverage"
            echo ""
            echo "Available Components:"
            echo "  Local Services:"
            echo "    • api               - REST API server"
            echo "    • migrate           - Database migration tool"
            echo "    • worker            - Background job processor"
            echo "  Lambda Functions:"
            echo "    • cleanup-handler   - Async cleanup handler"
            echo "    • connect-node      - Node connection discovery"
            echo "    • ws-connect        - WebSocket connection"
            echo "    • ws-disconnect     - WebSocket disconnection"
            echo "    • ws-send-message   - WebSocket broadcaster"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            echo "Use --help to see available options"
            exit 1
            ;;
    esac
done

# Project paths
PROJECT_ROOT=$(dirname "$(realpath "$0")")
BUILD_DIR="$PROJECT_ROOT/build"
CMD_DIR="$PROJECT_ROOT/cmd"
COVERAGE_DIR="$PROJECT_ROOT/coverage"

# Build metadata
BUILD_TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
BUILD_ID="b2_backend2_${BUILD_TIMESTAMP}_$$"
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION="2.0.0-alpha"  # Updated to v2 with new architecture

echo -e "${BLUE}====================================${NC}"
echo -e "${BLUE}    Backend2 Build System${NC}"
echo -e "${BLUE}    Version: $VERSION${NC}"
echo -e "${BLUE}    Commit: $GIT_COMMIT${NC}"
echo -e "${BLUE}====================================${NC}"
echo ""

# Cleaning phase - conditional based on build type
if [ "$QUICK_BUILD" = false ]; then
    echo -e "${YELLOW}🧹 Cleaning previous build artifacts and Go caches...${NC}"
    rm -rf "$BUILD_DIR"
    rm -rf "$COVERAGE_DIR"
    go clean -cache
    go clean -modcache
    go clean -testcache
    
    echo -e "${YELLOW}📦 Installing dependencies (full refresh)...${NC}"
    go mod download
    go mod tidy
    go mod verify
else
    echo -e "${BLUE}⚡ Quick build mode - preserving caches${NC}"
    # Only clean build directory, keep Go caches
    rm -rf "$BUILD_DIR"
    
    echo -e "${YELLOW}📦 Updating dependencies (quick)...${NC}"
    go mod tidy
fi

# Create necessary directories
mkdir -p "$BUILD_DIR"
mkdir -p "$COVERAGE_DIR"

# Linting phase (optional)
if [ "$RUN_LINT" = true ]; then
    echo -e "${YELLOW}🔍 Running linting and formatting checks...${NC}"
    
    # Check if golangci-lint is installed
    if ! command -v golangci-lint &> /dev/null; then
        echo -e "${YELLOW}Installing golangci-lint...${NC}"
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    fi
    
    # Run formatters
    echo "  • Checking code formatting..."
    UNFORMATTED=$(gofmt -l . | grep -v "vendor\|build\|wire_gen.go\|.pb.go")
    if [ -z "$UNFORMATTED" ]; then
        echo -e "${GREEN}  ✓ Code formatting is correct${NC}"
    else
        echo -e "${RED}  ✗ Code needs formatting. Run: gofmt -w .${NC}"
        echo "Files needing formatting:"
        echo "$UNFORMATTED"
        exit 1
    fi
    
    # Check for common issues in new architecture
    echo "  • Checking for hardcoded values..."
    HARDCODED=$(grep -r "125deabf-b32e-4313-b893-4a3ddb416cc2" --include="*.go" . 2>/dev/null | grep -v "test" || true)
    if [ -z "$HARDCODED" ]; then
        echo -e "${GREEN}  ✓ No hardcoded UUIDs found${NC}"
    else
        echo -e "${YELLOW}  ⚠️  Hardcoded UUIDs found (may be test data)${NC}"
    fi
    
    # Run go vet
    echo "  • Running go vet..."
    go vet ./...
    echo -e "${GREEN}  ✓ go vet passed${NC}"
    
    # Run golangci-lint if available
    if command -v golangci-lint &> /dev/null; then
        echo "  • Running golangci-lint..."
        golangci-lint run --timeout 3m
        echo -e "${GREEN}  ✓ golangci-lint passed${NC}"
    fi
fi

# Testing phase
if [ "$SKIP_TESTS" = false ]; then
    echo -e "${YELLOW}🧪 Running tests (level: $TEST_LEVEL)...${NC}"
    
    # Check if Makefile exists and use it for testing
    if [ -f "$PROJECT_ROOT/Makefile" ]; then
        case $TEST_LEVEL in
            unit)
                make test-unit
                ;;
            integration)
                make test-integration
                ;;
            e2e)
                make test-e2e
                ;;
            all)
                make test-all
                ;;
            coverage)
                make test-coverage
                ;;
            *)
                echo -e "${YELLOW}⚠️  Unknown test level: $TEST_LEVEL, running unit tests${NC}"
                make test-unit
                ;;
        esac
    else
        # Fallback to direct go test commands
        echo "  • Running go tests directly..."
        case $TEST_LEVEL in
            unit)
                go test -short -v ./...
                ;;
            integration)
                go test -v -tags=integration ./...
                ;;
            e2e)
                go test -v -tags=e2e ./...
                ;;
            all)
                go test -v ./...
                ;;
            coverage)
                go test -v -coverprofile="$COVERAGE_DIR/coverage.out" -covermode=atomic ./...
                go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"
                echo -e "${GREEN}Coverage report: $COVERAGE_DIR/coverage.html${NC}"
                go tool cover -func="$COVERAGE_DIR/coverage.out" | grep total | awk '{print "Total Coverage: " $3}'
                ;;
            *)
                go test -short -v ./...
                ;;
        esac
    fi
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Tests failed${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ All tests passed${NC}"
else
    echo -e "${BLUE}⏭️  Skipping tests (--skip-tests flag provided)${NC}"
fi

# Validate new architecture components
echo -e "${YELLOW}🏛️  Validating DDD/CQRS architecture components...${NC}"

# Check for required configuration files
if [ ! -f "$PROJECT_ROOT/domain/config/domain_config.go" ]; then
    echo -e "${YELLOW}⚠️  Domain configuration not found. Using defaults.${NC}"
else
    echo -e "${GREEN}  ✓ Domain configuration found${NC}"
fi

# Check for repository abstractions
if [ -d "$PROJECT_ROOT/infrastructure/persistence/abstractions" ]; then
    echo -e "${GREEN}  ✓ Repository abstractions found${NC}"
else
    echo -e "${YELLOW}⚠️  Repository abstractions not found${NC}"
fi

# Check for extension points
if [ -f "$PROJECT_ROOT/pkg/extensions/hooks.go" ]; then
    echo -e "${GREEN}  ✓ Extension points configured${NC}"
else
    echo -e "${YELLOW}⚠️  Extension points not configured${NC}"
fi

# Check for schema evolution
if [ -f "$PROJECT_ROOT/infrastructure/persistence/schema/evolution.go" ]; then
    echo -e "${GREEN}  ✓ Schema evolution strategy found${NC}"
else
    echo -e "${YELLOW}⚠️  Schema evolution not configured${NC}"
fi

# Wire dependency injection generation
echo -e "${YELLOW}🔄 Generating dependency injection code with Wire...${NC}"
if [ -d "$PROJECT_ROOT/infrastructure/di" ]; then
    (
        cd "$PROJECT_ROOT/infrastructure/di"
        
        # Check if wire is installed
        if ! command -v wire &> /dev/null; then
            echo "  • Installing Wire..."
            go install github.com/google/wire/cmd/wire@latest
        fi
        
        # Check wire configuration
        wire check
        if [ $? -ne 0 ]; then
            echo -e "${RED}❌ Wire validation failed${NC}"
            exit 1
        fi
        
        # Generate wire code
        wire
        if [ $? -ne 0 ]; then
            echo -e "${RED}❌ Wire code generation failed${NC}"
            exit 1
        fi
    )
    echo -e "${GREEN}✅ Wire code generated successfully${NC}"
fi

# Building phase
echo -e "${YELLOW}🏗️  Building components...${NC}"

# Determine which components to build
if [ -n "$SPECIFIC_COMPONENT" ]; then
    # Validate specified component exists
    if [ ! -d "$CMD_DIR/$SPECIFIC_COMPONENT" ]; then
        echo -e "${RED}❌ Component '$SPECIFIC_COMPONENT' does not exist in cmd/ directory${NC}"
        echo "Available components:"
        ls -1 "$CMD_DIR" 2>/dev/null | grep -v "^$" | sed 's/^/  • /'
        exit 1
    fi
    components="$SPECIFIC_COMPONENT"
    echo -e "${BLUE}🎯 Building specific component: $SPECIFIC_COMPONENT${NC}"
else
    # Discover all components
    components=""
    for dir in "$CMD_DIR"/*/; do
        if [ -d "$dir" ] && [ -f "$dir/main.go" ]; then
            components="$components $(basename "$dir")"
        fi
    done
    
    if [ -z "$components" ]; then
        echo -e "${YELLOW}⚠️  No components found with main.go in cmd/ directory${NC}"
        # Check for empty directories
        for dir in "$CMD_DIR"/*/; do
            if [ -d "$dir" ]; then
                echo "  • $(basename "$dir") (no main.go file)"
            fi
        done
        exit 0
    fi
    
    echo -e "${BLUE}📦 Building components:${NC}"
    for comp in $components; do
        echo "  • $comp"
    done
fi

# Set build flags based on options
BUILD_FLAGS=""
LDFLAGS="-s -w"  # Strip debug info by default

if [ "$QUICK_BUILD" = false ]; then
    BUILD_FLAGS="$BUILD_FLAGS -a"  # Force rebuild all packages
fi

if [ "$ENABLE_RACE" = true ]; then
    BUILD_FLAGS="$BUILD_FLAGS -race"
    echo -e "${YELLOW}⚠️  Race detection enabled (binaries will be larger and slower)${NC}"
fi

if [ "$DEBUG_BUILD" = true ]; then
    LDFLAGS=""  # Don't strip debug info
    BUILD_FLAGS="$BUILD_FLAGS -gcflags='all=-N -l'"  # Disable optimizations
    echo -e "${YELLOW}⚠️  Debug build enabled (binaries will be larger)${NC}"
fi

# Add version and build info to ldflags
LDFLAGS="$LDFLAGS -X main.Version=$VERSION"
LDFLAGS="$LDFLAGS -X main.BuildID=$BUILD_ID"
LDFLAGS="$LDFLAGS -X main.GitCommit=$GIT_COMMIT"
LDFLAGS="$LDFLAGS -X main.BuildTime=$BUILD_TIMESTAMP"

# Build each component
build_count=0
total_size=0

for component in $components; do
    echo ""
    echo -e "${BLUE}Building $component...${NC}"
    
    SRC_PATH="$CMD_DIR/$component"
    OUTPUT_DIR="$BUILD_DIR/$component"
    
    # Validate source has main.go
    if [ ! -f "$SRC_PATH/main.go" ]; then
        echo -e "${YELLOW}  ⚠️  Skipping $component - no main.go file${NC}"
        continue
    fi
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Determine build type and binary name
    # Lambda functions: cleanup-handler, connect-node, ws-*
    # Local services: api, worker, migrate
    
    LAMBDA_COMPONENTS="lambda cleanup-handler connect-node ws-connect ws-disconnect ws-send-message"
    LOCAL_COMPONENTS="api worker migrate"
    
    IS_LAMBDA=false
    for lambda_comp in $LAMBDA_COMPONENTS; do
        if [ "$component" = "$lambda_comp" ]; then
            IS_LAMBDA=true
            break
        fi
    done
    
    # Set binary name based on component type
    if [ "$IS_LAMBDA" = true ]; then
        # Lambda functions use 'bootstrap' naming convention
        BINARY_NAME="bootstrap"
        BINARY_PATH="$OUTPUT_DIR/$BINARY_NAME"
        BUILD_TARGET="Lambda"
    else
        # Local services use component name
        BINARY_NAME="$component"
        BINARY_PATH="$OUTPUT_DIR/$BINARY_NAME"
        BUILD_TARGET="Local"
    fi
    
    # Build the binary
    echo "  • Compiling $component for $BUILD_TARGET (Build ID: $BUILD_ID)..."
    
    if [ "$IS_LAMBDA" = true ]; then
        # Build for AWS Lambda (Linux/AMD64, static binary)
        # Use static linking and Lambda-specific tags
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
            go build $BUILD_FLAGS \
            -tags "lambda.norpc osusergo netgo" \
            -ldflags="$LDFLAGS -extldflags '-static'" \
            -o "$BINARY_PATH" \
            "$SRC_PATH"
    else
        # Build for local/container deployment
        go build $BUILD_FLAGS \
            -ldflags="$LDFLAGS" \
            -o "$BINARY_PATH" \
            "$SRC_PATH"
    fi
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Failed to build $component${NC}"
        exit 1
    fi
    
    # Make binary executable
    chmod +x "$BINARY_PATH"
    
    # Verify the binary
    if [ ! -x "$BINARY_PATH" ]; then
        echo -e "${RED}❌ Built binary for $component is not executable${NC}"
        exit 1
    fi
    
    # Create metadata files
    echo "$BUILD_TIMESTAMP" > "$OUTPUT_DIR/build_timestamp.txt"
    echo "$BUILD_ID" > "$OUTPUT_DIR/build_id.txt"
    echo "$GIT_COMMIT" > "$OUTPUT_DIR/git_commit.txt"
    cat > "$OUTPUT_DIR/build_info.json" <<EOF
{
  "component": "$component",
  "version": "$VERSION",
  "build_id": "$BUILD_ID",
  "build_time": "$BUILD_TIMESTAMP",
  "git_commit": "$GIT_COMMIT",
  "debug": $DEBUG_BUILD,
  "race": $ENABLE_RACE,
  "architecture": "DDD/CQRS",
  "api_version": "v2",
  "features": {
    "domain_config": true,
    "schema_evolution": true,
    "extension_points": true,
    "api_versioning": true
  }
}
EOF
    
    # Get binary size
    if [ -f "$BINARY_PATH" ]; then
        binary_size=$(stat -f%z "$BINARY_PATH" 2>/dev/null || stat -c%s "$BINARY_PATH" 2>/dev/null || echo "0")
        human_size=$(numfmt --to=iec-i --suffix=B "$binary_size" 2>/dev/null || echo "${binary_size}B")
        total_size=$((total_size + binary_size))
        echo -e "${GREEN}  ✅ Built $component successfully (${human_size})${NC}"
    fi
    
    build_count=$((build_count + 1))
done

# Build summary
echo ""
echo -e "${GREEN}====================================${NC}"
echo -e "${GREEN}       Build Complete!${NC}"
echo -e "${GREEN}====================================${NC}"
echo ""
echo -e "${BLUE}📊 Build Summary:${NC}"
echo "  • Components built: $build_count"
if [ $build_count -gt 0 ]; then
    human_total=$(numfmt --to=iec-i --suffix=B "$total_size" 2>/dev/null || echo "${total_size}B")
    echo "  • Total size: $human_total"
fi
echo "  • Build ID: $BUILD_ID"
echo "  • Git commit: $GIT_COMMIT"
echo "  • Output directory: $BUILD_DIR"

if [ "$QUICK_BUILD" = true ]; then
    echo ""
    echo -e "${BLUE}💡 Quick build completed${NC}"
    echo "  Run './build.sh' for full rebuild with cache clearing"
else
    echo ""
    echo -e "${BLUE}💡 Full build completed${NC}"
    echo "  Run './build.sh --quick' for faster incremental builds"
fi

if [ "$SKIP_TESTS" = true ]; then
    echo ""
    echo -e "${YELLOW}⚠️  Tests were skipped${NC}"
    echo "  Run './build.sh' to include tests"
fi

# Additional tips based on components
echo ""
echo -e "${BLUE}🚀 Next steps:${NC}"
if [[ " $components " =~ " api " ]]; then
    echo "  • Run API server: ./build/api/api"
    echo "  • Or use: make run"
fi
if [[ " $components " =~ " migrate " ]]; then
    echo "  • Run migrations: ./build/migrate/migrate up"
fi
if [[ " $components " =~ " worker " ]]; then
    echo "  • Start worker: ./build/worker/worker"
fi

echo ""
echo -e "${GREEN}✨ Build completed successfully!${NC}"