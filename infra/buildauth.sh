#!/bin/bash

set -e

# Terminal output formatting
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[BUILD]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_status "Building Lambda Authorizer..."

cd lambda/authorizer

# Clean previous build
if [ -f "index.js" ]; then
    print_status "Cleaning previous build..."
    rm -f index.js
    rm -f index.js.map
fi

# Install dependencies
print_status "Installing dependencies..."
npm install

# Compile TypeScript to JavaScript
print_status "Compiling TypeScript..."
npx tsc index.ts \
    --target es2020 \
    --module commonjs \
    --esModuleInterop \
    --allowSyntheticDefaultImports \
    --skipLibCheck \
    --sourceMap

# Verify build
if [ ! -f "index.js" ]; then
    print_error "Build failed - index.js not created"
    exit 1
fi

print_success "Lambda Authorizer built successfully"
print_status "Output: infra/lambda/authorizer/index.js"