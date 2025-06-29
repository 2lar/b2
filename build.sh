#!/bin/bash
# Master Build Script for Brain2 Application
# This script builds all components of the application in the correct order

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[BUILD]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Verify required tools
print_status "Checking required tools..."
missing_tools=()

if ! command_exists go; then
    missing_tools+=("go")
fi

if ! command_exists npm; then
    missing_tools+=("npm")
fi

if ! command_exists node; then
    missing_tools+=("node")
fi

if [ ${#missing_tools[@]} -ne 0 ]; then
    print_error "Missing required tools: ${missing_tools[*]}"
    print_error "Please install the missing tools and try again."
    exit 1
fi

print_success "All required tools are available"

# Build components
BUILD_START_TIME=$(date +%s)

print_status "=================================================="
print_status "Building Brain2 Application Components"
print_status "=================================================="

# 1. Build Backend (Go Lambda)
print_status "Step 1/4: Building Backend Go Lambda..."
cd backend

if [ ! -f "build.sh" ]; then
    print_error "Backend build script not found!"
    exit 1
fi

chmod +x build.sh
./build.sh

if [ ! -f "build/function.zip" ]; then
    print_error "Backend build failed - function.zip not created"
    exit 1
fi

print_success "Backend built successfully"
cd ..

# 2. Build Lambda Authorizer
print_status "Step 2/4: Building Lambda Authorizer..."
cd infra/lambda/authorizer

# Clean previous build
if [ -f "clean.sh" ]; then
    chmod +x clean.sh
    ./clean.sh
fi

# Install dependencies
print_status "Installing Lambda authorizer dependencies..."
npm install

# Check if JavaScript file already exists or compile TypeScript
print_status "Ensuring JavaScript build exists..."
if [ ! -f "index.js" ] && [ -f "index.ts" ]; then
    print_status "Compiling TypeScript to JavaScript..."
    # Simple compilation without strict type checking to avoid browser-specific types
    npx tsc index.ts --target es2020 --module commonjs --esModuleInterop --allowSyntheticDefaultImports --skipLibCheck
fi

if [ ! -f "index.js" ]; then
    print_error "Lambda authorizer build failed - index.js not created"
    exit 1
fi

print_success "Lambda Authorizer built successfully"
cd ../../..

# 3. Build Frontend
print_status "Step 3/4: Building Frontend..."
cd frontend

if [ ! -f "package.json" ]; then
    print_error "Frontend package.json not found!"
    exit 1
fi

# Use the existing frontend build script
npm run build

if [ ! -d "dist" ]; then
    print_error "Frontend build failed - dist directory not created"
    exit 1
fi

print_success "Frontend built successfully"
cd ..

# 4. Prepare Infrastructure
print_status "Step 4/4: Preparing Infrastructure..."
cd infra

if [ ! -f "package.json" ]; then
    print_error "Infrastructure package.json not found!"
    exit 1
fi

# Install CDK dependencies (but don't build TypeScript due to conflicts)
print_status "Installing CDK dependencies..."
npm install

print_success "Infrastructure prepared successfully"
cd ..

# Summary
BUILD_END_TIME=$(date +%s)
BUILD_DURATION=$((BUILD_END_TIME - BUILD_START_TIME))

print_status "=================================================="
print_success "Build Complete! 🎉"
print_status "=================================================="
print_status "Build Summary:"
print_status "  ✅ Backend (Go Lambda): backend/build/function.zip"
print_status "  ✅ Lambda Authorizer: infra/lambda/authorizer/index.js"
print_status "  ✅ Frontend: frontend/dist/"
print_status "  ✅ Infrastructure: infra/ (ready for deployment)"
print_status ""
print_status "Build completed in ${BUILD_DURATION} seconds"
print_status ""
print_status "Next steps:"
print_status "  1. Deploy infrastructure: cd infra && npx cdk deploy"
print_status "  2. Or run individual components for development"
print_status "=================================================="