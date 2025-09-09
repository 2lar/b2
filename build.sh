#!/bin/bash

set -e  # Exit immediately if any command returns non-zero status

# Terminal output formatting
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
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

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Load environment variables from root .env file
load_environment() {
    if [[ -f ".env" ]]; then
        print_status "Loading environment configuration from root .env file..."
        source ./scripts/load-env.sh all
        print_success "Environment variables loaded"
    elif [[ -f ".env.example" ]]; then
        print_warning "No .env file found, but .env.example exists"
        print_warning "Copy .env.example to .env and configure your values:"
        print_warning "  cp .env.example .env"
        print_warning "Continuing with default values..."
    else
        print_warning "No environment configuration found"
        print_warning "Using default build settings"
    fi
}

# Load environment first
load_environment

# Validate prerequisites
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
    print_error ""
    print_error "Installation guidance:"
    print_error "  - Go: https://golang.org/doc/install"
    print_error "  - Node.js & npm: https://nodejs.org/"
    print_error "  - Or use package managers: brew, apt, yum, chocolatey"
    exit 1
fi

print_success "All required tools are available"

BUILD_START_TIME=$(date +%s)

print_status "=================================================="
print_status "Building Brain2 Application Components"
print_status "=================================================="

print_status "Step 1/4: Building Backend Go Lambda..."

cd backend

if [ ! -f "build.sh" ]; then
    print_error "Backend build script not found!"
    print_error "Expected: backend/build.sh"
    print_error "Current directory: $(pwd)"
    exit 1
fi

chmod +x build.sh

# Pass environment variables to backend build
# Backend will use environment variables for its configuration
print_status "Building backend with environment: ${PROJECT_ENV:-default}"
./build.sh

# Check if any of the Lambda build directories exist
if [ ! -d "build/lambda" ] && [ ! -d "build/api" ]; then
    print_error "Backend build failed - no build directories created"
    print_error "Expected artifacts in: backend/build/"
    print_error "Check backend build logs for compilation errors"
    exit 1
fi

print_success "Backend built successfully"

cd ..

print_status "Step 2/4: Building Lambda Authorizer..."

cd infra/lambda/authorizer

if [ -f "clean.sh" ]; then
    print_status "Cleaning previous authorizer build..."
    chmod +x clean.sh
    ./clean.sh
fi

print_status "Installing Lambda authorizer dependencies..."
npm install

print_status "Ensuring JavaScript build exists..."
if [ ! -f "index.js" ] && [ -f "index.ts" ]; then
    print_status "Compiling TypeScript to JavaScript..."
    npx tsc index.ts \
        --target es2020 \
        --module commonjs \
        --esModuleInterop \
        --allowSyntheticDefaultImports \
        --skipLibCheck
fi

if [ ! -f "index.js" ]; then
    print_error "Lambda authorizer build failed - index.js not created"
    print_error "Check TypeScript compilation errors above"
    print_error "Verify index.ts exists and is valid TypeScript"
    exit 1
fi

print_success "Lambda Authorizer built successfully"

cd ../../..

print_status "Step 3/4: Building Frontend..."

cd frontend

if [ ! -f "package.json" ]; then
    print_error "Frontend package.json not found!"
    print_error "Expected: frontend/package.json"
    print_error "Current directory: $(pwd)"
    exit 1
fi

# Create a temporary .env file for frontend with VITE_ variables
print_status "Setting up frontend environment variables..."
if [[ -n "${VITE_SUPABASE_URL:-}" ]]; then
    # Create temporary .env file for Vite build
    cat > .env.local << EOF
# Auto-generated from root .env during build
VITE_SUPABASE_URL=${VITE_SUPABASE_URL}
VITE_SUPABASE_ANON_KEY=${VITE_SUPABASE_ANON_KEY}
VITE_API_BASE_URL=${VITE_API_BASE_URL}
VITE_WEBSOCKET_URL=${VITE_WEBSOCKET_URL:-}
VITE_DEBUG=${VITE_DEBUG:-false}
VITE_MODE=${VITE_MODE:-production}
EOF
    print_success "Frontend environment configured"
else
    print_warning "No VITE_ variables found in root .env, using existing frontend/.env if present"
fi

npm run build

# Clean up temporary env file
if [[ -f ".env.local" ]]; then
    rm .env.local
fi

if [ ! -d "dist" ]; then
    print_error "Frontend build failed - dist directory not created"
    print_error "Check frontend build logs for compilation errors"
    print_error "Verify build script exists in package.json"
    exit 1
fi

print_success "Frontend built successfully"

cd ..

print_status "Step 4/4: Preparing Infrastructure..."

cd infra

if [ ! -f "package.json" ]; then
    print_error "Infrastructure package.json not found!"
    print_error "Expected: infra/package.json"
    print_error "Current directory: $(pwd)"
    exit 1
fi

print_status "Installing CDK dependencies..."
npm install

print_success "Infrastructure prepared successfully"

BUILD_END_TIME=$(date +%s)
BUILD_DURATION=$((BUILD_END_TIME - BUILD_START_TIME))

print_status "=================================================="
print_success "Build Complete! 🎉"
print_status "=================================================="

print_status "Build Summary:"
print_status "  ✅ Backend (Go Lambda): backend/build/"
print_status "  ✅ Lambda Authorizer: infra/lambda/authorizer/index.js"
print_status "  ✅ Frontend: frontend/dist/"
print_status "  ✅ Infrastructure: infra/cdk.out"
print_status ""

print_status "Build completed in ${BUILD_DURATION} seconds"
print_status ""

print_status "Next steps:"
print_status "  1. Deploy infrastructure: cd infra && npx cdk deploy"
print_status "  2. Or run individual components for development"
print_status "    - Backend: cd backend && ./run-local.sh"
print_status "    - Frontend: cd frontend && npm run dev"
print_status "  3. Monitor deployment: Check AWS console for resource status"
print_status "=================================================="
