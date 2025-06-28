#!/bin/bash
# This script builds all Go Lambda functions for the event-driven architecture.

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Building all Go Lambda functions..."

# 1. Clean previous build artifacts to ensure a fresh build.
rm -rf build/
rm -f bootstrap

# Define all Lambda functions to build
declare -a LAMBDAS=("main" "connect-node" "ws-connect" "ws-disconnect" "ws-send-message")

# 2. Build each Lambda function
for lambda in "${LAMBDAS[@]}"; do
    echo "Building $lambda Lambda..."
    
    # Create build directory for this Lambda
    if [ "$lambda" = "main" ]; then
        BUILD_DIR="build"
    else
        BUILD_DIR="build/$lambda"
    fi
    mkdir -p "$BUILD_DIR"
    
    # Build the Go binary
    # GOOS=linux: Compiles for the Linux operating system, which AWS Lambda uses.
    # GOARCH=amd64: Compiles for the x86-64 architecture.
    # CGO_ENABLED=0: Disables Cgo to create a static binary, which is ideal for Lambda.
    # -o bootstrap: Names the output executable 'bootstrap', which is the default name
    #               AWS Lambda looks for with a "provided" runtime.
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$BUILD_DIR/bootstrap" "./cmd/$lambda/"
    
    # Grant execute permissions to the binary. This is critical for Lambda to run it.
    chmod +x "$BUILD_DIR/bootstrap"
    
    # Create the zip archive for deployment.
    # We 'cd' into the build directory first to ensure the 'bootstrap' file
    # is at the root of the zip archive, which is required by AWS Lambda.
    cd "$BUILD_DIR"
    zip function.zip bootstrap
    cd - > /dev/null  # Go back to original directory, suppress output
    
    echo "✅ $lambda Lambda built successfully in $BUILD_DIR/"
done

echo ""
echo "🎉 All Lambda functions built successfully!"
echo "📁 Build structure:"
echo "   build/                     (main API Lambda)"
echo "   build/connect-node/        (connection processing Lambda)"
echo "   build/ws-connect/          (WebSocket connect Lambda)"
echo "   build/ws-disconnect/       (WebSocket disconnect Lambda)"  
echo "   build/ws-send-message/     (WebSocket message Lambda)"
