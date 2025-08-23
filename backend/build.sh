#!/bin/bash
# This script builds all Go Lambda functions, preparing them for deployment.

# Exit immediately if a command exits with a non-zero status.
set -e

# Parse command line arguments
SKIP_TESTS=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--skip-tests]"
            exit 1
            ;;
    esac
done

echo "🧹 Cleaning previous build artifacts and Go caches..."
rm -rf build/
go clean -cache
go clean -modcache

echo "🛠️ Installing dependencies..."
go get github.com/getkin/kin-openapi/openapi3
go mod tidy

if [ "$SKIP_TESTS" = false ]; then
    echo "🧪 Running tests..."
    go test ./...
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

echo "🏗️ Building Lambda functions..."

# Discover all applications in the cmd directory
apps=$(ls -d cmd/*/ 2>/dev/null | xargs -n 1 basename)

if [ -z "$apps" ]; then
    echo "⚠️  No Lambda functions found in cmd/ directory"
    exit 0
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
    # -a: Force rebuild of all packages
    # -ldflags="-s -w": Strip debug info for smaller binary, add build timestamp
    # -o $OUTPUT_PATH/bootstrap: Names the output 'bootstrap', the default for "provided" runtimes.
    BUILD_TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
    BUILD_ID="brain2_build_${BUILD_TIMESTAMP}_$$"
    
    echo "🔨 Compiling $app for Lambda runtime (Build ID: $BUILD_ID)..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -ldflags="-s -w -X main.BuildID=$BUILD_ID" -o "$OUTPUT_PATH/bootstrap" "$SRC_PATH"
    
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
echo "   • Built $build_count Lambda function(s)"
echo "   • All binaries are ready for deployment"
echo "   • Build timestamp files created for CDK change detection"
echo "   • Run './build.sh --skip-tests' for faster rebuilds"

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
echo "🎉 All Lambda functions built successfully!"
echo "💡 Tip: CDK will now reliably detect changes due to timestamp files"
