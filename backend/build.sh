#!/bin/bash
# This script builds the Go Lambda function, preparing it for deployment.

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Building Go Lambda function..."

# 1. Clean previous build artifacts to ensure a fresh build.
rm -rf build/
rm -f bootstrap

# 2. Build the Go binary.
#    GOOS=linux: Compiles for the Linux operating system, which AWS Lambda uses.
#    GOARCH=amd64: Compiles for the x86-64 architecture.
#    CGO_ENABLED=0: Disables Cgo to create a static binary, which is ideal for Lambda.
#    -o bootstrap: Names the output executable 'bootstrap', which is the default name
#                  AWS Lambda looks for with a "provided" runtime.
#    ./cmd/main/: Specifies the path to our main package.
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bootstrap ./cmd/main/

# 3. Grant execute permissions to the binary. This is critical for Lambda to run it.
chmod +x bootstrap

# 4. Create the deployment directory structure.
mkdir -p build

# 5. Move the executable into the build directory.
mv bootstrap build/

# 6. Create the zip archive for deployment.
#    We 'cd' into the build directory first to ensure the 'bootstrap' file
#    is at the root of the zip archive, which is required by AWS Lambda.
cd build
zip function.zip bootstrap
cd ..

echo "✅ Lambda function built successfully! The package is in build/function.zip"
