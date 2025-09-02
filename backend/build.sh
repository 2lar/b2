#!/bin/bash
# This script builds all Go Lambda functions, preparing them for deployment.
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
#   ./build.sh --component main
#   ./build.sh --component cleanup-handler
#   ./build.sh --component connect-node
#   ./build.sh --component ws-connect
#   ./build.sh --component ws-disconnect
#   ./build.sh --component ws-send-message
#
# Quick component build:
#   ./build.sh --component main --quick
#
# AVAILABLE COMPONENTS:
# ====================
# • main              - Main backend API Lambda
# • cleanup-handler   - Async cleanup Lambda for node deletion
# • connect-node      - Node connection discovery Lambda
# • ws-connect        - WebSocket connect handler
# • ws-disconnect     - WebSocket disconnect handler
# • ws-send-message   - WebSocket message sender

# Exit immediately if a command exits with a non-zero status.
set -e

# Parse command line arguments
SKIP_TESTS=false
QUICK_BUILD=false
SPECIFIC_COMPONENT=""
TEST_LEVEL="unit"  # Default to unit tests for speed
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
            TEST_LEVEL="$2"  # unit, integration, all, ci
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo ""
            echo "Usage: $0 [--skip-tests] [--quick] [--component <name>] [--test-level <level>]"
            echo ""
            echo "Options:"
            echo "  --skip-tests     Skip running tests"
            echo "  --quick          Skip cache clearing for faster incremental builds"
            echo "  --component      Build only specified component (main, cleanup-handler, etc.)"
            echo "  --test-level     Test level: unit, integration, all, ci (default: unit)"
            echo ""
            echo "Examples:"
            echo "  $0                              # Full build with unit tests"
            echo "  $0 --test-level all            # Full build with all tests"
            echo "  $0 --quick --skip-tests        # Fast incremental build"
            echo "  $0 --component main --quick     # Quick build of main component only"
            exit 1
            ;;
    esac
done

# Cleaning phase - conditional based on build type
if [ "$QUICK_BUILD" = false ]; then
    echo "🧹 Cleaning previous build artifacts and Go caches..."
    rm -rf build/
    go clean -cache
    go clean -modcache
    
    echo "🛠️ Installing dependencies..."
    go get github.com/getkin/kin-openapi/openapi3
    go mod tidy
    go mod vendor  # Ensure vendor directory has all dependencies
else
    echo "⚡ Quick build mode - preserving caches and doing incremental build"
    # Only clean build directory, keep Go caches
    rm -rf build/
    
    echo "🛠️ Updating dependencies (quick)..."
    go mod tidy
    go mod vendor  # Update vendor directory even in quick mode
fi

if [ "$SKIP_TESTS" = false ]; then
    echo "🧪 Running tests (level: $TEST_LEVEL)..."
    
    # Check if Makefile exists and use it for testing
    if [ -f "Makefile" ]; then
        case $TEST_LEVEL in
            unit)
                make test-unit
                ;;
            integration)
                make test-integration
                ;;
            all)
                make test-all
                ;;
            ci)
                make ci
                ;;
            *)
                echo "⚠️  Unknown test level: $TEST_LEVEL, running unit tests"
                make test-unit
                ;;
        esac
        
        if [ $? -ne 0 ]; then
            echo "❌ Tests failed"
            exit 1
        fi
    else
        # Fallback to simple go test if Makefile doesn't exist
        echo "⚠️  Makefile not found, using simple go test"
        go test ./...
        if [ $? -ne 0 ]; then
            echo "❌ Tests failed"
            exit 1
        fi
    fi
else
    echo "⏭️  Skipping tests (--skip-tests flag provided)"
fi

# Validate and generate dependency injection code with Wire
echo "🔍 Validating dependency injection code with Wire..."
(
    cd internal/di
    wire check
)
if [ $? -ne 0 ]; then
    echo "❌ Wire validation failed. Please check your dependency injection configuration."
    exit 1
fi

echo "🔄 Generating dependency injection code with Wire..."
(
    cd internal/di
    go generate
)
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

# Determine which components to build
if [ -n "$SPECIFIC_COMPONENT" ]; then
    # Validate specified component exists
    if [ ! -d "cmd/$SPECIFIC_COMPONENT" ]; then
        echo "❌ Component '$SPECIFIC_COMPONENT' does not exist in cmd/ directory"
        echo "Available components:"
        ls -1 cmd/ | sed 's/^/  • /'
        exit 1
    fi
    apps="$SPECIFIC_COMPONENT"
    echo "🎯 Building specific component: $SPECIFIC_COMPONENT"
else
    # Discover all applications in the cmd directory
    apps=$(ls -d cmd/*/ 2>/dev/null | xargs -n 1 basename)
    
    if [ -z "$apps" ]; then
        echo "⚠️  No Lambda functions found in cmd/ directory"
        exit 0
    fi
    
    echo "🏗️ Building all components: $(echo $apps | tr '\n' ' ')"
fi

# Loop through each application and build it
build_count=0
for app in $apps
do
    echo "--- Building $app ---"

    # Define the source and output paths
    SRC_PATH="./cmd/$app"
    OUTPUT_PATH="./build/$app"

    # Validate source directory exists
    if [ ! -d "$SRC_PATH" ]; then
        echo "❌ Source directory $SRC_PATH does not exist"
        exit 1
    fi

    # Create the output directory
    mkdir -p "$OUTPUT_PATH"

    # Build the Go binary for AWS Lambda
    # GOOS=linux GOARCH=amd64: Compiles for the Lambda runtime environment.
    # CGO_ENABLED=0: Creates a static binary without C dependencies.
    # -a: Force rebuild of all packages (conditional based on quick build)
    # -ldflags="-s -w": Strip debug info for smaller binary, add build timestamp
    # -o $OUTPUT_PATH/bootstrap: Names the output 'bootstrap', the default for "provided" runtimes.
    BUILD_TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
    BUILD_ID="brain2_build_${BUILD_TIMESTAMP}_$$"
    
    # Set build flags based on quick build mode
    BUILD_FLAGS=""
    if [ "$QUICK_BUILD" = false ]; then
        BUILD_FLAGS="-a"  # Force rebuild all packages
        echo "🔨 Compiling $app for Lambda runtime (Full rebuild, Build ID: $BUILD_ID)..."
    else
        echo "🔨 Compiling $app for Lambda runtime (Incremental, Build ID: $BUILD_ID)..."
    fi
    
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $BUILD_FLAGS -ldflags="-s -w -X main.BuildID=$BUILD_ID" -o "$OUTPUT_PATH/bootstrap" "$SRC_PATH"
    
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build $app"
        exit 1
    fi

    # Grant execute permissions to the binary. This is critical for Lambda.
    chmod +x "$OUTPUT_PATH/bootstrap"
    
    # Verify the binary was created and is executable
    if [ ! -x "$OUTPUT_PATH/bootstrap" ]; then
        echo "❌ Built binary for $app is not executable"
        exit 1
    fi

    # Verify the binary contains our build ID (for deployment verification)
    if strings "$OUTPUT_PATH/bootstrap" | grep -q "$BUILD_ID"; then
        echo "✅ Build verification passed - binary contains build ID: $BUILD_ID"
    else
        echo "⚠️  Build verification warning - build ID not found in binary"
    fi

    # Create timestamp file for CDK to detect changes
    echo "$BUILD_TIMESTAMP" > "$OUTPUT_PATH/build_timestamp.txt"
    echo "$BUILD_ID" > "$OUTPUT_PATH/build_id.txt"

    # Get binary size for reporting  
    binary_size=$(stat -c%s "$OUTPUT_PATH/bootstrap" 2>/dev/null | numfmt --to=iec 2>/dev/null || echo "unknown")
    echo "✅ Successfully built $app (size: $binary_size, timestamp: $BUILD_TIMESTAMP)"
    
    build_count=$((build_count + 1))
done

echo ""
echo "📊 Build Summary:"
if [ -n "$SPECIFIC_COMPONENT" ]; then
    echo "   • Built component: $SPECIFIC_COMPONENT"
else
    echo "   • Built $build_count Lambda function(s)"
fi
echo "   • All binaries are ready for deployment"
echo "   • Build timestamp files created for CDK change detection"
if [ "$QUICK_BUILD" = false ]; then
    echo "   • Full rebuild completed with cache clearing"
    echo "   • Run './build.sh --quick --skip-tests' for faster incremental rebuilds"
else
    echo "   • Quick incremental build completed"
    echo "   • Run './build.sh' for full rebuild with cache clearing"
fi

# Show build verification info
echo ""
echo "🔍 Build Verification:"
for app in $apps; do
    if [ -f "./build/$app/build_id.txt" ]; then
        build_id=$(cat "./build/$app/build_id.txt")
        echo "   • $app: $build_id"
    fi
done

echo ""
if [ -n "$SPECIFIC_COMPONENT" ]; then
    echo "🎉 Component '$SPECIFIC_COMPONENT' built successfully!"
else
    echo "🎉 All Lambda functions built successfully!"
fi
echo "💡 Tip: Use './build.sh --component <name>' to build individual components"
echo "💡 Tip: CDK will now reliably detect changes due to timestamp files"
