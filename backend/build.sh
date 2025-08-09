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

# Run Wire to generate dependency injection code
echo "🔄 Generating dependency injection code with Wire..."
(cd internal/di && go generate)

echo "🏗️ Building Lambda functions..."

# Discover all applications in the cmd directory
apps=$(ls -d cmd/*/ | xargs -n 1 basename)

# Loop through each application and build it
for app in $apps
do
    echo "--- Building $app ---"

    # Define the source and output paths
    SRC_PATH="./cmd/$app"
    OUTPUT_PATH="./build/$app"

    # Create the output directory
    mkdir -p "$OUTPUT_PATH"

    # Build the Go binary for AWS Lambda
    # GOOS=linux GOARCH=amd64: Compiles for the Lambda runtime environment.
    # CGO_ENABLED=0: Creates a static binary without C dependencies.
    # -o $OUTPUT_PATH/bootstrap: Names the output 'bootstrap', the default for "provided" runtimes.
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$OUTPUT_PATH/bootstrap" "$SRC_PATH"

    # Grant execute permissions to the binary. This is critical for Lambda.
    chmod +x "$OUTPUT_PATH/bootstrap"

    echo "✅ Successfully built $app"
done

echo "🎉 All Lambda functions built successfully!"
