#!/bin/bash
# =============================================================================
# Brain2 Master Build Script - Multi-Language Application Orchestration
# =============================================================================
#
# 📚 EDUCATIONAL OVERVIEW:
# This build script demonstrates advanced DevOps practices for orchestrating
# complex, multi-language applications. It showcases dependency management,
# error handling, progress reporting, and deployment preparation for modern
# serverless applications.
#
# 🏗️ KEY DEVOPS CONCEPTS:
#
# 1. BUILD ORCHESTRATION:
#    - Coordinated builds across multiple technology stacks
#    - Dependency-aware build ordering (backend → frontend → infrastructure)
#    - Failure handling with immediate exit on errors
#    - Validation of build artifacts and prerequisites
#
# 2. CROSS-PLATFORM COMPATIBILITY:
#    - POSIX-compliant shell scripting for Linux/macOS/WSL
#    - Tool detection and validation before execution
#    - Environment-agnostic directory navigation
#    - Portable command patterns and best practices
#
# 3. DEVELOPER EXPERIENCE:
#    - Color-coded output for visual clarity and debugging
#    - Progress indicators and build phase reporting
#    - Comprehensive error messages with actionable guidance
#    - Build timing and performance metrics
#
# 4. ERROR HANDLING STRATEGY:
#    - Fail-fast approach with immediate error exit
#    - Detailed error reporting with context
#    - Prerequisite validation before build attempts
#    - Artifact verification after each build step
#
# 5. BUILD ARTIFACT MANAGEMENT:
#    - Verification of expected build outputs
#    - Clean separation of build artifacts by component
#    - Deployment-ready artifact organization
#    - Build reproducibility and consistency
#
# 🔄 BUILD WORKFLOW:
# 1. Tool Prerequisites → 2. Backend (Go) → 3. Auth (TypeScript) → 4. Frontend (JavaScript) → 5. Infrastructure (CDK)
#
# 🎯 LEARNING OBJECTIVES:
# - Multi-language build orchestration
# - Shell scripting best practices and patterns
# - DevOps automation and toolchain management
# - Error handling in build pipelines
# - Cross-platform development workflows
# - Build artifact validation and management

set -e  # Exit immediately if any command returns non-zero status

# =============================================================================
# Terminal Output Formatting - Enhanced Developer Experience
# =============================================================================
#
# ANSI COLOR CODES:
# These ANSI escape sequences provide colored terminal output for better
# visual distinction between different types of messages. Colors improve
# readability and help developers quickly identify issues during builds.
#
# COLOR PSYCHOLOGY IN DEVOPS:
# - Blue: Informational messages, progress updates
# - Green: Success states, completed operations
# - Yellow: Warnings, non-critical issues
# - Red: Errors, failures requiring attention
# - NC (No Color): Reset to default terminal color

# ANSI color escape sequences for terminal formatting
RED='\033[0;31m'        # Error messages and critical failures
GREEN='\033[0;32m'      # Success messages and completed operations
YELLOW='\033[1;33m'     # Warning messages and advisory notices
BLUE='\033[0;34m'       # Informational messages and progress updates
NC='\033[0m'            # No Color - reset to terminal default

# =============================================================================
# Logging Functions - Structured Output with Visual Hierarchy
# =============================================================================
#
# These functions provide consistent, color-coded output throughout the build
# process. They demonstrate best practices for build script communication
# and error reporting.

# Build Progress Indicator
# 
# USAGE: print_status "Building frontend components..."
# PURPOSE: Inform developers about current build phase
# VISUAL: Blue [BUILD] prefix for easy scanning of build logs
print_status() {
    echo -e "${BLUE}[BUILD]${NC} $1"
}

# Success Notification
#
# USAGE: print_success "Frontend built successfully"
# PURPOSE: Confirm successful completion of build steps
# VISUAL: Green [SUCCESS] prefix for positive reinforcement
print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Warning Alert
#
# USAGE: print_warning "Skipping optional optimization step"
# PURPOSE: Highlight non-critical issues that may need attention
# VISUAL: Yellow [WARNING] prefix for cautionary information
print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Error Notification
#
# USAGE: print_error "Required tool 'go' not found"
# PURPOSE: Clearly indicate failures that prevent build completion
# VISUAL: Red [ERROR] prefix for immediate attention
print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# =============================================================================
# System Capability Detection - Cross-Platform Tool Validation
# =============================================================================
#
# This function implements the standard Unix pattern for checking if a
# command/tool exists in the system PATH. It's essential for failing fast
# when required dependencies are missing.

# Command Existence Checker
#
# IMPLEMENTATION DETAILS:
# - Uses 'command -v' for POSIX compliance (works on all Unix-like systems)
# - Redirects both stdout and stderr to /dev/null for silent operation
# - Returns 0 (success) if command exists, non-zero if missing
# - More reliable than 'which' command which isn't always available
#
# USAGE EXAMPLE:
# if command_exists "node"; then
#     echo "Node.js is available"
# else
#     echo "Node.js is not installed"
# fi
#
# WHY NOT 'which' COMMAND:
# - 'which' is not POSIX standard and may not exist on all systems
# - 'command -v' is built into the shell and always available
# - More portable across different Unix-like operating systems
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# =============================================================================
# Prerequisites Validation - Fail-Fast Tool Detection
# =============================================================================
#
# FAIL-FAST PHILOSOPHY:
# Rather than discovering missing tools halfway through the build process,
# we validate all prerequisites upfront. This saves developer time and
# provides clear, actionable error messages.
#
# REQUIRED TOOLCHAIN:
# - Go: Backend Lambda function compilation
# - Node.js: JavaScript runtime for frontend and infrastructure
# - npm: Package manager for JavaScript dependencies

print_status "Checking required tools..."

# Initialize array to collect missing tools for batch reporting
# BASH ARRAY SYNTAX: () creates empty array, += appends elements
missing_tools=()

# Go Language Toolchain Validation
# REQUIRED FOR: Backend Lambda function compilation and Go module management
if ! command_exists go; then
    missing_tools+=("go")
fi

# Node Package Manager Validation
# REQUIRED FOR: Installing JavaScript dependencies, running build scripts
if ! command_exists npm; then
    missing_tools+=("npm")
fi

# Node.js Runtime Validation
# REQUIRED FOR: JavaScript execution, TypeScript compilation, CDK operations
if ! command_exists node; then
    missing_tools+=("node")
fi

# Comprehensive Missing Tools Report
# BASH ARRAY LENGTH: ${#array[@]} gets the number of elements
# BATCH ERROR REPORTING: Collect all missing tools before failing
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

# =============================================================================
# Build Performance Monitoring - Timing and Metrics
# =============================================================================
#
# BUILD TIMING STRATEGY:
# Capture start time for performance monitoring and developer feedback.
# Unix timestamp provides precise timing for build optimization analysis.
#
# PERFORMANCE BENEFITS:
# - Identify slow build steps for optimization
# - Monitor build performance regression over time
# - Provide developer feedback on build duration

# Capture build start time in Unix timestamp format
# UNIX TIMESTAMP: Seconds since January 1, 1970 (epoch time)
# USAGE: Enables accurate duration calculation regardless of build time
BUILD_START_TIME=$(date +%s)

# Visual Build Phase Separator
# AESTHETICS: Clear visual separation makes build logs easier to parse
# SCANNING: Developers can quickly locate build phases in output
print_status "=================================================="
print_status "Building Brain2 Application Components"
print_status "=================================================="

# =============================================================================
# STEP 1: Backend Go Lambda Function Build
# =============================================================================
#
# BUILD PRIORITY: Backend first ensures API availability for frontend integration
# TECHNOLOGY: Go compiled to AWS Lambda compatible binary
# OUTPUT: Deployment-ready function.zip for serverless infrastructure
#
# GO LAMBDA COMPILATION PROCESS:
# 1. Cross-compile Go code for Linux/AMD64 (AWS Lambda runtime)
# 2. Create bootstrap executable (AWS Lambda Go runtime requirement)
# 3. Package into ZIP with proper permissions and structure
# 4. Validate artifact size and structure for deployment

print_status "Step 1/4: Building Backend Go Lambda..."

# Navigate to backend directory for context-specific build
# DIRECTORY CONTEXT: Each component has isolated build dependencies
cd backend

# Build Script Validation
# DEFENSIVE PROGRAMMING: Verify build script exists before execution
# FAIL-FAST: Better to fail here than during script execution
if [ ! -f "build.sh" ]; then
    print_error "Backend build script not found!"
    print_error "Expected: backend/build.sh"
    print_error "Current directory: $(pwd)"
    exit 1
fi

# Executable Permission Management
# UNIX PERMISSIONS: Ensure build script has execute permissions
# PORTABILITY: Git doesn't always preserve execute permissions across platforms
chmod +x build.sh

# Execute Backend Build Process
# DELEGATION: Use component-specific build script for specialized logic
# CONTEXT: Backend build handles Go modules, compilation, and packaging
./build.sh

# Build Artifact Validation
# VERIFICATION: Ensure expected output exists before continuing
# AWS LAMBDA REQUIREMENT: function.zip must exist for deployment
if [ ! -f "build/function.zip" ]; then
    print_error "Backend build failed - function.zip not created"
    print_error "Expected artifact: backend/build/function.zip"
    print_error "Check backend build logs for compilation errors"
    exit 1
fi

print_success "Backend built successfully"

# Return to project root for next build step
# NAVIGATION: Maintain consistent working directory for subsequent operations
cd ..

# =============================================================================
# STEP 2: Lambda Authorizer TypeScript Build
# =============================================================================
#
# BUILD PURPOSE: JWT authentication handler for API Gateway
# TECHNOLOGY: TypeScript compiled to Node.js JavaScript for AWS Lambda
# OUTPUT: index.js ready for AWS Lambda deployment
#
# AUTHORIZER FUNCTION ROLE:
# 1. Validate JWT tokens from Supabase authentication
# 2. Extract user identity from token claims
# 3. Return AWS IAM policy for API Gateway access control
# 4. Enable secure, stateless authentication for serverless APIs

print_status "Step 2/4: Building Lambda Authorizer..."

# Navigate to authorizer-specific directory
# NESTED STRUCTURE: infra/lambda/authorizer contains isolated auth logic
cd infra/lambda/authorizer

# Optional Build Cleanup
# CLEAN BUILD STRATEGY: Remove previous artifacts if cleanup script exists
# ISOLATION: Prevent stale artifacts from affecting new builds
if [ -f "clean.sh" ]; then
    print_status "Cleaning previous authorizer build..."
    chmod +x clean.sh
    ./clean.sh
fi

# Dependency Installation
# NODE MODULES: Install TypeScript compiler and JWT validation libraries
# PACKAGE MANAGEMENT: npm handles transitive dependencies automatically
print_status "Installing Lambda authorizer dependencies..."
npm install

# TypeScript to JavaScript Compilation
# CONDITIONAL COMPILATION: Only compile if source exists and target doesn't
# FLEXIBLE BUILD: Supports both pre-compiled and source-only scenarios
print_status "Ensuring JavaScript build exists..."
if [ ! -f "index.js" ] && [ -f "index.ts" ]; then
    print_status "Compiling TypeScript to JavaScript..."
    
    # TypeScript Compilation Configuration
    # TARGET ES2020: Modern JavaScript features supported by AWS Lambda Node.js runtime
    # COMMONJS MODULES: AWS Lambda requires CommonJS module format, not ES modules
    # INTEROP FLAGS: Enable compatibility between CommonJS and ES module imports
    # SKIP LIB CHECK: Avoid type checking external library files for faster compilation
    npx tsc index.ts \
        --target es2020 \
        --module commonjs \
        --esModuleInterop \
        --allowSyntheticDefaultImports \
        --skipLibCheck
fi

# Compilation Artifact Validation
# REQUIRED OUTPUT: index.js is the entry point for AWS Lambda execution
# DEPLOYMENT DEPENDENCY: CDK deployment expects compiled JavaScript
if [ ! -f "index.js" ]; then
    print_error "Lambda authorizer build failed - index.js not created"
    print_error "Check TypeScript compilation errors above"
    print_error "Verify index.ts exists and is valid TypeScript"
    exit 1
fi

print_success "Lambda Authorizer built successfully"

# Return to project root for consistency
# NAVIGATION: Maintain predictable working directory for subsequent steps
cd ../../..

# =============================================================================
# STEP 3: Frontend JavaScript Application Build
# =============================================================================
#
# BUILD PURPOSE: Single Page Application (SPA) with graph visualization
# TECHNOLOGY: TypeScript/JavaScript bundled for browser deployment
# OUTPUT: Static assets in dist/ directory for CDN distribution
#
# FRONTEND BUILD PROCESS:
# 1. TypeScript compilation to JavaScript
# 2. Module bundling and dependency resolution
# 3. Asset optimization (minification, compression)
# 4. Generate deployment-ready static files

print_status "Step 3/4: Building Frontend..."

# Navigate to frontend application directory
# SEPARATION: Frontend has independent build process and dependencies
cd frontend

# Project Structure Validation
# NPM PROJECT: package.json defines build scripts and dependencies
# VALIDATION: Ensure we're in a valid Node.js project directory
if [ ! -f "package.json" ]; then
    print_error "Frontend package.json not found!"
    print_error "Expected: frontend/package.json"
    print_error "Current directory: $(pwd)"
    exit 1
fi

# Execute Frontend Build Process
# NPM SCRIPT: Delegates to build system defined in package.json
# BUILD SYSTEM: Typically Vite, Webpack, or similar bundler
# AUTOMATION: Build script handles all compilation and optimization
npm run build

# Build Output Validation
# DIST DIRECTORY: Standard output location for built frontend assets
# DEPLOYMENT READY: Static files ready for CDN/S3 deployment
if [ ! -d "dist" ]; then
    print_error "Frontend build failed - dist directory not created"
    print_error "Check frontend build logs for compilation errors"
    print_error "Verify build script exists in package.json"
    exit 1
fi

print_success "Frontend built successfully"

# Return to project root
cd ..

# =============================================================================
# STEP 4: Infrastructure Dependencies Preparation
# =============================================================================
#
# BUILD PURPOSE: AWS CDK infrastructure definition preparation
# TECHNOLOGY: TypeScript CDK constructs and AWS resource definitions
# OUTPUT: Prepared CDK project ready for deployment
#
# CDK PREPARATION PROCESS:
# 1. Install AWS CDK dependencies and construct libraries
# 2. Prepare TypeScript infrastructure definitions
# 3. Ready for synthesis and deployment commands
# 4. Note: No TypeScript compilation here to avoid conflicts

print_status "Step 4/4: Preparing Infrastructure..."

# Navigate to infrastructure definition directory
# CDK PROJECT: Contains AWS infrastructure as code definitions
cd infra

# Infrastructure Project Validation
# CDK REQUIREMENT: package.json with CDK dependencies and scripts
if [ ! -f "package.json" ]; then
    print_error "Infrastructure package.json not found!"
    print_error "Expected: infra/package.json"
    print_error "Current directory: $(pwd)"
    exit 1
fi

# CDK Dependencies Installation
# AWS CDK: Install construct libraries and AWS SDK dependencies
# PREPARATION ONLY: Don't compile TypeScript to avoid type conflicts
# DEPLOYMENT READY: CDK can now synthesize CloudFormation templates
print_status "Installing CDK dependencies..."
npm install

print_success "Infrastructure prepared successfully"

# Return to project root for summary
cd ..

# =============================================================================
# Build Completion Summary and Performance Metrics
# =============================================================================
#
# PERFORMANCE TRACKING:
# Calculate total build duration for optimization insights
# DEVELOPER FEEDBACK: Provide clear success confirmation and next steps
# DEPLOYMENT GUIDANCE: Direct developers to appropriate deployment commands

# Calculate Build Performance Metrics
# UNIX TIMESTAMP ARITHMETIC: End time minus start time = duration in seconds
BUILD_END_TIME=$(date +%s)
BUILD_DURATION=$((BUILD_END_TIME - BUILD_START_TIME))

# Visual Success Banner
# CELEBRATION: Clear visual indication of successful build completion
# FORMATTING: Consistent with other status messages throughout script
print_status "=================================================="
print_success "Build Complete! 🎉"
print_status "=================================================="

# Comprehensive Build Artifact Summary
# VERIFICATION: Confirm all expected outputs are available
# DEPLOYMENT MAPPING: Show where each component's artifacts are located
print_status "Build Summary:"
print_status "  ✅ Backend (Go Lambda): backend/build/function.zip"
print_status "  ✅ Lambda Authorizer: infra/lambda/authorizer/index.js"
print_status "  ✅ Frontend: frontend/dist/"
print_status "  ✅ Infrastructure: infra/ (ready for deployment)"
print_status ""

# Performance Metrics Reporting
# BUILD TIME: Help developers understand build performance
# OPTIMIZATION: Enable identification of slow build steps
print_status "Build completed in ${BUILD_DURATION} seconds"
print_status ""

# Next Steps Guidance
# DEPLOYMENT: Guide developers to appropriate next actions
# DEVELOPMENT: Provide alternatives for development vs. production workflows
print_status "Next steps:"
print_status "  1. Deploy infrastructure: cd infra && npx cdk deploy"
print_status "  2. Or run individual components for development"
print_status "    - Backend: cd backend && go run ."
print_status "    - Frontend: cd frontend && npm run dev"
print_status "  3. Monitor deployment: Check AWS console for resource status"
print_status "=================================================="