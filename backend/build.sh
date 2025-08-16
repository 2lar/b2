#!/bin/bash
# This script builds all Go Lambda functions, preparing them for deployment.

# Exit immediately if a command exits with a non-zero status.
set -e

echo "🧹 Cleaning previous build artifacts..."
rm -rf build/

echo "🛠️ Installing dependencies..."
go get github.com/getkin/kin-openapi/openapi3
go mod tidy

echo "🧪 Running tests..."
go test ./...

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
    # -o $OUTPUT_PATH/bootstrap: Names the output 'bootstrap', the default for "provided" runtimes.
    echo "🔨 Compiling $app for Lambda runtime..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$OUTPUT_PATH/bootstrap" "$SRC_PATH"
    
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

    # Get binary size for reporting  
    binary_size=$(stat -c%s "$OUTPUT_PATH/bootstrap" 2>/dev/null | numfmt --to=iec 2>/dev/null || echo "unknown")
    echo "✅ Successfully built $app (size: $binary_size)"
    
    build_count=$((build_count + 1))
done

echo ""
echo "📊 Build Summary:"
echo "   • Built $build_count Lambda function(s)"
echo "   • All binaries are ready for deployment"

echo "🎉 All Lambda functions built successfully!"
