#!/bin/bash
# Backend2 Test Runner
# Comprehensive testing with various modes and options

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
TEST_TYPE="unit"
VERBOSE=false
COVERAGE=false
RACE=false
BENCH=false
PROFILE=false
TIMEOUT="10m"
PACKAGE="./..."
FAIL_FAST=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --unit)
            TEST_TYPE="unit"
            shift
            ;;
        --integration)
            TEST_TYPE="integration"
            shift
            ;;
        --e2e)
            TEST_TYPE="e2e"
            shift
            ;;
        --all)
            TEST_TYPE="all"
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --coverage|-c)
            COVERAGE=true
            shift
            ;;
        --race|-r)
            RACE=true
            shift
            ;;
        --bench|-b)
            BENCH=true
            shift
            ;;
        --profile)
            PROFILE=true
            shift
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --package|-p)
            PACKAGE="$2"
            shift 2
            ;;
        --fail-fast)
            FAIL_FAST=true
            shift
            ;;
        --help|-h)
            echo "Backend2 Test Runner"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Test Types:"
            echo "  --unit            Run unit tests (default)"
            echo "  --integration     Run integration tests"
            echo "  --e2e             Run end-to-end tests"
            echo "  --all             Run all test types"
            echo ""
            echo "Options:"
            echo "  --verbose, -v     Show detailed test output"
            echo "  --coverage, -c    Generate coverage report"
            echo "  --race, -r        Enable race detection"
            echo "  --bench, -b       Run benchmarks"
            echo "  --profile         Generate CPU/memory profiles"
            echo "  --timeout <dur>   Test timeout (default: 10m)"
            echo "  --package <pkg>   Test specific package (default: ./...)"
            echo "  --fail-fast       Stop on first test failure"
            echo "  --help, -h        Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                           # Run unit tests"
            echo "  $0 --all --coverage         # All tests with coverage"
            echo "  $0 --integration --race     # Integration tests with race detection"
            echo "  $0 --bench --profile        # Run benchmarks with profiling"
            echo "  $0 -p ./domain/... -v       # Test domain layer with verbose output"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            echo "Use --help to see available options"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}====================================${NC}"
echo -e "${BLUE}    Backend2 Test Runner${NC}"
echo -e "${BLUE}    Type: $TEST_TYPE${NC}"
echo -e "${BLUE}====================================${NC}"
echo ""

# Create directories for test artifacts
COVERAGE_DIR="coverage"
PROFILE_DIR="profiles"
mkdir -p "$COVERAGE_DIR"
mkdir -p "$PROFILE_DIR"

# Build base test command
TEST_CMD="go test"

# Add verbose flag
if [ "$VERBOSE" = true ]; then
    TEST_CMD="$TEST_CMD -v"
fi

# Add race detection
if [ "$RACE" = true ]; then
    TEST_CMD="$TEST_CMD -race"
    echo -e "${YELLOW}🏃 Race detection enabled${NC}"
fi

# Add timeout
TEST_CMD="$TEST_CMD -timeout $TIMEOUT"

# Add fail-fast
if [ "$FAIL_FAST" = true ]; then
    TEST_CMD="$TEST_CMD -failfast"
    echo -e "${YELLOW}⚡ Fail-fast mode enabled${NC}"
fi

# Function to run tests
run_tests() {
    local test_name=$1
    local test_tags=$2
    local test_pkgs=$3
    
    echo -e "${CYAN}🧪 Running $test_name tests...${NC}"
    
    local cmd="$TEST_CMD"
    
    # Add test tags if specified
    if [ -n "$test_tags" ]; then
        cmd="$cmd -tags=$test_tags"
    fi
    
    # Add coverage if requested
    if [ "$COVERAGE" = true ]; then
        cmd="$cmd -coverprofile=$COVERAGE_DIR/${test_name}_coverage.out -covermode=atomic"
    fi
    
    # Add CPU profiling if requested
    if [ "$PROFILE" = true ]; then
        cmd="$cmd -cpuprofile=$PROFILE_DIR/${test_name}_cpu.prof"
        cmd="$cmd -memprofile=$PROFILE_DIR/${test_name}_mem.prof"
    fi
    
    # Add package specification
    cmd="$cmd $test_pkgs"
    
    # Run the tests
    echo "Command: $cmd"
    echo ""
    
    if $cmd; then
        echo -e "${GREEN}✅ $test_name tests passed${NC}"
        return 0
    else
        echo -e "${RED}❌ $test_name tests failed${NC}"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    echo -e "${MAGENTA}⚡ Running benchmarks...${NC}"
    
    local cmd="go test -bench=. -benchmem -benchtime=10s"
    
    if [ "$VERBOSE" = true ]; then
        cmd="$cmd -v"
    fi
    
    if [ "$PROFILE" = true ]; then
        cmd="$cmd -cpuprofile=$PROFILE_DIR/bench_cpu.prof"
        cmd="$cmd -memprofile=$PROFILE_DIR/bench_mem.prof"
    fi
    
    cmd="$cmd $PACKAGE"
    
    echo "Command: $cmd"
    echo ""
    
    if $cmd | tee "$PROFILE_DIR/benchmark_results.txt"; then
        echo -e "${GREEN}✅ Benchmarks completed${NC}"
        echo -e "${CYAN}Results saved to: $PROFILE_DIR/benchmark_results.txt${NC}"
    else
        echo -e "${RED}❌ Benchmarks failed${NC}"
        return 1
    fi
}

# Main test execution
test_failed=false

case $TEST_TYPE in
    unit)
        # Unit tests - test domain and application layers
        if [ "$PACKAGE" = "./..." ]; then
            run_tests "unit" "" "./domain/... ./application/..." || test_failed=true
        else
            run_tests "unit" "" "$PACKAGE" || test_failed=true
        fi
        ;;
        
    integration)
        # Integration tests - test infrastructure and interfaces
        if [ "$PACKAGE" = "./..." ]; then
            run_tests "integration" "integration" "./infrastructure/... ./interfaces/..." || test_failed=true
        else
            run_tests "integration" "integration" "$PACKAGE" || test_failed=true
        fi
        ;;
        
    e2e)
        # End-to-end tests
        echo -e "${YELLOW}🔄 Starting test dependencies...${NC}"
        if [ -f "docker-compose.test.yml" ]; then
            docker-compose -f docker-compose.test.yml up -d
            sleep 5  # Wait for services to start
        fi
        
        run_tests "e2e" "e2e" "./tests/e2e/..." || test_failed=true
        
        if [ -f "docker-compose.test.yml" ]; then
            echo -e "${YELLOW}🔄 Stopping test dependencies...${NC}"
            docker-compose -f docker-compose.test.yml down
        fi
        ;;
        
    all)
        # Run all test types
        echo -e "${CYAN}📋 Running all test suites...${NC}"
        echo ""
        
        # Unit tests
        run_tests "unit" "" "./domain/... ./application/..." || test_failed=true
        echo ""
        
        # Integration tests
        run_tests "integration" "integration" "./infrastructure/... ./interfaces/..." || test_failed=true
        echo ""
        
        # E2E tests
        if [ -d "tests/e2e" ]; then
            run_tests "e2e" "e2e" "./tests/e2e/..." || test_failed=true
        fi
        ;;
        
    *)
        echo -e "${RED}❌ Unknown test type: $TEST_TYPE${NC}"
        exit 1
        ;;
esac

# Run benchmarks if requested
if [ "$BENCH" = true ]; then
    echo ""
    run_benchmarks || test_failed=true
fi

# Generate coverage report if requested
if [ "$COVERAGE" = true ] && [ "$test_failed" = false ]; then
    echo ""
    echo -e "${YELLOW}📊 Generating coverage report...${NC}"
    
    # Merge coverage files if multiple exist
    if ls "$COVERAGE_DIR"/*_coverage.out 1> /dev/null 2>&1; then
        echo "mode: atomic" > "$COVERAGE_DIR/coverage.out"
        tail -q -n +2 "$COVERAGE_DIR"/*_coverage.out >> "$COVERAGE_DIR/coverage.out"
    elif [ -f "$COVERAGE_DIR/unit_coverage.out" ]; then
        cp "$COVERAGE_DIR/unit_coverage.out" "$COVERAGE_DIR/coverage.out"
    fi
    
    if [ -f "$COVERAGE_DIR/coverage.out" ]; then
        # Generate HTML report
        go tool cover -html="$COVERAGE_DIR/coverage.out" -o "$COVERAGE_DIR/coverage.html"
        echo -e "${GREEN}✅ Coverage report: $COVERAGE_DIR/coverage.html${NC}"
        
        # Show coverage summary
        echo ""
        echo -e "${CYAN}Coverage Summary:${NC}"
        go tool cover -func="$COVERAGE_DIR/coverage.out" | tail -10
        echo ""
        total_coverage=$(go tool cover -func="$COVERAGE_DIR/coverage.out" | grep total | awk '{print $3}')
        echo -e "${GREEN}Total Coverage: $total_coverage${NC}"
        
        # Check coverage threshold
        threshold=70.0
        current=$(echo $total_coverage | sed 's/%//')
        if (( $(echo "$current < $threshold" | bc -l) )); then
            echo -e "${YELLOW}⚠️  Coverage is below threshold ($threshold%)${NC}"
        fi
    fi
fi

# Generate profile analysis if requested
if [ "$PROFILE" = true ] && [ "$test_failed" = false ]; then
    echo ""
    echo -e "${YELLOW}📈 Analyzing profiles...${NC}"
    
    if [ -f "$PROFILE_DIR/unit_cpu.prof" ]; then
        echo "CPU profile: $PROFILE_DIR/unit_cpu.prof"
        echo "View with: go tool pprof $PROFILE_DIR/unit_cpu.prof"
    fi
    
    if [ -f "$PROFILE_DIR/unit_mem.prof" ]; then
        echo "Memory profile: $PROFILE_DIR/unit_mem.prof"
        echo "View with: go tool pprof $PROFILE_DIR/unit_mem.prof"
    fi
    
    if [ -f "$PROFILE_DIR/bench_cpu.prof" ]; then
        echo "Benchmark CPU profile: $PROFILE_DIR/bench_cpu.prof"
        echo "View with: go tool pprof $PROFILE_DIR/bench_cpu.prof"
    fi
fi

# Final summary
echo ""
echo -e "${BLUE}====================================${NC}"
if [ "$test_failed" = true ]; then
    echo -e "${RED}    ❌ Tests Failed${NC}"
    echo -e "${BLUE}====================================${NC}"
    exit 1
else
    echo -e "${GREEN}    ✅ All Tests Passed${NC}"
    echo -e "${BLUE}====================================${NC}"
    
    # Show helpful next steps
    echo ""
    echo -e "${CYAN}Next steps:${NC}"
    
    if [ "$COVERAGE" = false ]; then
        echo "  • Run with --coverage to generate coverage report"
    fi
    
    if [ "$RACE" = false ] && [ "$TEST_TYPE" != "e2e" ]; then
        echo "  • Run with --race to detect race conditions"
    fi
    
    if [ "$BENCH" = false ]; then
        echo "  • Run with --bench to execute benchmarks"
    fi
    
    if [ "$TEST_TYPE" != "all" ]; then
        echo "  • Run with --all to execute all test suites"
    fi
fi