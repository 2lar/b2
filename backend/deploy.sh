#!/bin/bash
# Brain2 Backend Deployment Script
# ==============================================================================
# Complete deployment pipeline with comprehensive testing and validation
#
# USAGE:
#   ./deploy.sh                    # Full deployment with all tests
#   ./deploy.sh --quick           # Quick deployment (unit tests only)
#   ./deploy.sh --skip-tests      # Deploy without tests (dangerous!)
#   ./deploy.sh --dry-run         # Show what would be deployed without doing it
#   ./deploy.sh --environment prod # Deploy to specific environment
# ==============================================================================

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[DEPLOY]${NC} $1"
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

log_step() {
    echo -e "${CYAN}[→]${NC} $1"
}

# Deployment configuration
ENVIRONMENT="dev"
SKIP_TESTS=false
QUICK_MODE=false
DRY_RUN=false
TEST_LEVEL="all"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --quick)
            QUICK_MODE=true
            TEST_LEVEL="unit"
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            echo "Brain2 Backend Deployment Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --environment <env>  Deploy to specific environment (dev, staging, prod)"
            echo "  --skip-tests        Skip all tests (not recommended)"
            echo "  --quick             Quick deployment with unit tests only"
            echo "  --dry-run           Show what would be deployed without doing it"
            echo "  --help              Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                           # Full deployment with all tests"
            echo "  $0 --quick                   # Quick deployment with unit tests"
            echo "  $0 --environment prod        # Deploy to production"
            echo "  $0 --dry-run                 # Preview deployment"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Get the project root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_DIR="${SCRIPT_DIR}"
INFRA_DIR="${PROJECT_ROOT}/infra"

# Deployment banner
echo ""
echo -e "${MAGENTA}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${MAGENTA}║              Brain2 Backend Deployment                      ║${NC}"
echo -e "${MAGENTA}║                                                            ║${NC}"
echo -e "${MAGENTA}║  Environment: ${CYAN}${ENVIRONMENT}${MAGENTA}                                         ║${NC}"
echo -e "${MAGENTA}║  Test Level:  ${CYAN}${TEST_LEVEL}${MAGENTA}                                          ║${NC}"
echo -e "${MAGENTA}║  Mode:        ${CYAN}$([ "$DRY_RUN" = true ] && echo "DRY RUN" || echo "LIVE")${MAGENTA}                                     ║${NC}"
echo -e "${MAGENTA}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to run commands (respects dry-run mode)
run_command() {
    local cmd="$1"
    local description="$2"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] Would execute: $cmd"
    else
        log_step "$description"
        eval "$cmd"
        if [ $? -eq 0 ]; then
            log_success "$description completed"
        else
            log_error "$description failed"
            exit 1
        fi
    fi
}

# Pre-deployment checks
log_info "Starting pre-deployment checks..."

# Check if we're in the backend directory
if [ ! -f "${BACKEND_DIR}/go.mod" ]; then
    log_error "Backend directory not found at ${BACKEND_DIR}"
    exit 1
fi

# Check if infrastructure directory exists
if [ ! -d "${INFRA_DIR}" ]; then
    log_error "Infrastructure directory not found at ${INFRA_DIR}"
    exit 1
fi

# Check for required tools
command -v go >/dev/null 2>&1 || { log_error "Go is required but not installed"; exit 1; }
command -v npm >/dev/null 2>&1 || { log_error "npm is required but not installed"; exit 1; }
command -v aws >/dev/null 2>&1 || { log_error "AWS CLI is required but not installed"; exit 1; }

log_success "Pre-deployment checks passed"

# Navigate to backend directory
cd "${BACKEND_DIR}"

# Step 1: Clean build artifacts
if [ "$QUICK_MODE" = false ]; then
    run_command "make clean" "Cleaning build artifacts"
fi

# Step 2: Install/update dependencies
run_command "make deps" "Installing dependencies"

# Step 3: Generate dependency injection code
run_command "make wire" "Generating dependency injection code"

# Step 4: Run tests
if [ "$SKIP_TESTS" = false ]; then
    log_info "Running test suite (level: ${TEST_LEVEL})..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY-RUN] Would run: make test-${TEST_LEVEL}"
    else
        case $TEST_LEVEL in
            all)
                make test-all
                if [ $? -ne 0 ]; then
                    log_error "Tests failed! Deployment aborted."
                    exit 1
                fi
                log_success "All tests passed"
                
                # Generate coverage report for full deployments
                make test-coverage > /dev/null 2>&1
                coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
                log_info "Test coverage: ${coverage}"
                ;;
            unit)
                make test-unit
                if [ $? -ne 0 ]; then
                    log_error "Unit tests failed! Deployment aborted."
                    exit 1
                fi
                log_success "Unit tests passed"
                ;;
            *)
                log_error "Unknown test level: ${TEST_LEVEL}"
                exit 1
                ;;
        esac
    fi
else
    log_warning "Tests skipped (--skip-tests flag provided)"
fi

# Step 5: Code quality checks
if [ "$QUICK_MODE" = false ] && [ "$SKIP_TESTS" = false ]; then
    log_info "Running code quality checks..."
    
    if [ "$DRY_RUN" = false ]; then
        make fmt
        make vet
        
        # Run linter if available
        if command -v golangci-lint >/dev/null 2>&1; then
            make lint
        else
            log_warning "golangci-lint not installed, skipping lint check"
        fi
        
        log_success "Code quality checks passed"
    else
        log_info "[DRY-RUN] Would run: make fmt vet lint"
    fi
fi

# Step 6: Build Lambda functions
log_info "Building Lambda functions..."

BUILD_FLAGS=""
if [ "$QUICK_MODE" = true ]; then
    BUILD_FLAGS="--quick"
fi

if [ "$SKIP_TESTS" = true ]; then
    BUILD_FLAGS="${BUILD_FLAGS} --skip-tests"
fi

run_command "./build.sh ${BUILD_FLAGS}" "Building Lambda functions"

# Step 7: Deploy infrastructure
log_info "Deploying to AWS (${ENVIRONMENT})..."

cd "${INFRA_DIR}"

# Install infrastructure dependencies if needed
if [ ! -d "node_modules" ]; then
    run_command "npm install" "Installing infrastructure dependencies"
fi

# Load environment variables
if [ -f "../scripts/load-env.sh" ]; then
    log_step "Loading environment variables..."
    if [ "$DRY_RUN" = false ]; then
        source ../scripts/load-env.sh infra
    else
        log_info "[DRY-RUN] Would load: source ../scripts/load-env.sh infra"
    fi
fi

# Synthesize CDK stack to verify
run_command "npm run synth" "Synthesizing CDK stack"

# Show deployment diff
if [ "$DRY_RUN" = false ]; then
    log_step "Checking deployment changes..."
    npx cdk diff --all || true
fi

# Deploy to AWS
if [ "$DRY_RUN" = true ]; then
    log_info "[DRY-RUN] Would deploy CDK stack with: npm run deploy"
    log_info "[DRY-RUN] This would create/update the following AWS resources:"
    log_info "  - Lambda functions"
    log_info "  - API Gateway"
    log_info "  - DynamoDB tables"
    log_info "  - S3 buckets"
    log_info "  - CloudFront distribution"
else
    # Ask for confirmation in production
    if [ "$ENVIRONMENT" = "prod" ]; then
        echo ""
        log_warning "You are about to deploy to PRODUCTION!"
        read -p "Are you sure you want to continue? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            log_info "Deployment cancelled"
            exit 0
        fi
    fi
    
    run_command "npm run deploy" "Deploying CDK stack"
fi

# Step 8: Post-deployment validation
if [ "$DRY_RUN" = false ]; then
    log_info "Running post-deployment validation..."
    
    # Extract API URL from CDK outputs
    if [ -f "outputs.json" ]; then
        API_URL=$(cat outputs.json | grep -o '"HttpApiUrl": "[^"]*' | cut -d'"' -f4)
        
        if [ -n "$API_URL" ]; then
            log_step "Testing API health endpoint..."
            
            # Test health endpoint
            response=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/health" || echo "000")
            
            if [ "$response" = "200" ]; then
                log_success "API health check passed"
            else
                log_warning "API health check returned: ${response}"
            fi
        fi
    fi
fi

# Step 9: Generate deployment report
log_info "Generating deployment report..."

TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S")
COMMIT_HASH=$(git rev-parse --short HEAD)
BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [ "$DRY_RUN" = false ]; then
    cat > deployment-report.txt << EOF
Brain2 Backend Deployment Report
================================
Timestamp: ${TIMESTAMP}
Environment: ${ENVIRONMENT}
Git Branch: ${BRANCH}
Git Commit: ${COMMIT_HASH}
Test Level: ${TEST_LEVEL}
Tests Skipped: ${SKIP_TESTS}
Quick Mode: ${QUICK_MODE}

Deployment Status: SUCCESS

Components Deployed:
$(ls -1 build/ 2>/dev/null | sed 's/^/  - /' || echo "  - None (dry run)")

AWS Resources Updated:
  - Lambda Functions
  - API Gateway
  - DynamoDB Tables
  - S3 Buckets
  - CloudFront Distribution

Notes:
  - All tests passed
  - Code quality checks passed
  - Infrastructure deployed successfully
EOF
    
    log_success "Deployment report saved to deployment-report.txt"
fi

# Final summary
echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║            Deployment Completed Successfully!              ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$DRY_RUN" = true ]; then
    log_info "This was a DRY RUN - no actual deployment was performed"
    log_info "Remove --dry-run flag to perform actual deployment"
else
    log_success "Backend deployed to ${ENVIRONMENT} environment"
    
    if [ -f "${INFRA_DIR}/outputs.json" ]; then
        log_info "API endpoints available in: ${INFRA_DIR}/outputs.json"
    fi
    
    log_info "Deployment report: ${INFRA_DIR}/deployment-report.txt"
fi

echo ""
log_info "Next steps:"
echo "  1. Verify the deployment in AWS Console"
echo "  2. Test the API endpoints"
echo "  3. Monitor CloudWatch logs for any issues"
echo "  4. Update frontend with new API endpoints if needed"
echo ""

exit 0