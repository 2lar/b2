#!/bin/bash
# backend/build.sh

set -e

echo "Building Go Lambda function..."

# 1. Clean previous builds
rm -rf build/
rm -f bootstrap

# 2. Build the Go binary for Amazon Linux 2
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap main.go node_operations.go graph_operations.go

# 3. Grant execute permissions. This is critical.
chmod +x bootstrap

# 4. Create a clean build directory
mkdir -p build

# 5. Move the executable into the build directory
mv bootstrap build/

# 6. Create the zip archive from within the build directory to ensure correct structure
cd build
zip function.zip bootstrap
cd ..

echo "✅ Lambda function built successfully! The package is in build/function.zip"