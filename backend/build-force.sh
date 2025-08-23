#!/bin/bash
# Force rebuild script with cache clearing for proper Lambda deployment

set -e

echo "🧹 FORCE CLEAN: Removing all caches and build artifacts..."
rm -rf build/
go clean -cache
go clean -modcache

echo "🛠️ Installing dependencies..."
go get github.com/getkin/kin-openapi/openapi3
go mod download
go mod tidy

echo "🧪 Running tests..."
go test ./...

echo "🔍 Validating dependency injection code with Wire..."
(
    cd internal/di
    wire check
)
if [ $? -ne 0 ]; then
    echo "❌ Wire validation failed."
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

echo "🏗️ Building Lambda functions with FORCE rebuild..."

apps=$(ls -d cmd/*/ 2>/dev/null | xargs -n 1 basename)

if [ -z "$apps" ]; then
    echo "⚠️  No Lambda functions found in cmd/ directory"
    exit 0
fi

build_count=0
for app in $apps
do
    echo "--- Force building $app ---"
    
    SRC_PATH="./cmd/$app"
    OUTPUT_PATH="./build/$app"
    
    if [ ! -d "$SRC_PATH" ]; then
        echo "❌ Source directory $SRC_PATH does not exist"
        exit 1
    fi
    
    mkdir -p "$OUTPUT_PATH"
    
    # Force rebuild with -a flag and verbose output
    echo "🔨 Force compiling $app for Lambda runtime..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
        -a \
        -v \
        -ldflags="-s -w" \
        -o "$OUTPUT_PATH/bootstrap" \
        "$SRC_PATH" 2>&1 | tail -5
    
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build $app"
        exit 1
    fi
    
    chmod +x "$OUTPUT_PATH/bootstrap"
    
    if [ ! -x "$OUTPUT_PATH/bootstrap" ]; then
        echo "❌ Built binary for $app is not executable"
        exit 1
    fi
    
    # Verify the binary contains our debug strings
    if [ "$app" = "main" ]; then
        echo "🔍 Verifying main Lambda has debug logging..."
        if strings "$OUTPUT_PATH/bootstrap" | grep -q "DEBUG HANDLER"; then
            echo "✅ Debug logging found in binary"
        else
            echo "⚠️  WARNING: Debug logging NOT found in binary"
        fi
    fi
    
    binary_size=$(stat -c%s "$OUTPUT_PATH/bootstrap" 2>/dev/null | numfmt --to=iec 2>/dev/null || echo "unknown")
    echo "✅ Successfully built $app (size: $binary_size)"
    
    build_count=$((build_count + 1))
done

echo ""
echo "📊 Build Summary:"
echo "   • Force rebuilt $build_count Lambda function(s)"
echo "   • All caches cleared"
echo "   • All binaries rebuilt from scratch"

echo "🎉 Force build complete!"