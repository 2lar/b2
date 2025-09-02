#!/bin/bash
# Build script for infra Lambda functions (specifically the authorizer)

set -e

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[BUILD-LAMBDA]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_info "Building authorizer Lambda function..."

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LAMBDA_DIR="${SCRIPT_DIR}/lambda/authorizer"

# Check if authorizer directory exists
if [ ! -d "$LAMBDA_DIR" ]; then
    log_error "Authorizer Lambda directory not found at: $LAMBDA_DIR"
    exit 1
fi

# Navigate to authorizer directory
cd "$LAMBDA_DIR"

log_info "Installing authorizer dependencies..."
if [ -f "package.json" ]; then
    npm install
    if [ $? -ne 0 ]; then
        log_error "Failed to install authorizer dependencies"
        exit 1
    fi
else
    log_error "package.json not found in authorizer directory"
    exit 1
fi

log_info "Compiling TypeScript to JavaScript..."

# Clean any existing JavaScript files
if [ -f "index.js" ]; then
    rm -f index.js
    log_info "Cleaned existing index.js"
fi

if [ -f "index.d.ts" ]; then
    rm -f index.d.ts
    log_info "Cleaned existing index.d.ts"
fi

# Check if TypeScript source exists
if [ ! -f "index.ts" ]; then
    log_error "index.ts not found in authorizer directory"
    exit 1
fi

# Compile TypeScript
npx tsc index.ts \
    --target es2020 \
    --module commonjs \
    --esModuleInterop \
    --allowSyntheticDefaultImports \
    --skipLibCheck \
    --strict false

if [ $? -ne 0 ]; then
    log_error "TypeScript compilation failed"
    exit 1
fi

# Verify JavaScript file was created
if [ ! -f "index.js" ]; then
    log_error "index.js was not created - compilation may have failed silently"
    exit 1
fi

# Check file size (should not be empty)
if [ ! -s "index.js" ]; then
    log_error "index.js is empty - compilation failed"
    exit 1
fi

log_success "Authorizer Lambda built successfully"
log_info "JavaScript file created: $(ls -lh index.js | awk '{print $5, $9}')"

# Optional: Show a snippet of the compiled code for verification
log_info "Compiled JavaScript preview:"
echo "----------------------------------------"
head -5 index.js
echo "..."
echo "----------------------------------------"

log_success "Lambda build completed successfully!"