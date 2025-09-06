#!/bin/bash
# Backend2 Code Quality Checker
# Comprehensive linting, formatting, and static analysis

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
FIX=false
STRICT=false
SECURITY=false
COMPLEXITY=false
VERBOSE=false
FORMAT_ONLY=false
INSTALL_TOOLS=false

# Track issues found
ISSUES_FOUND=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --fix|-f)
            FIX=true
            shift
            ;;
        --strict)
            STRICT=true
            shift
            ;;
        --security|-s)
            SECURITY=true
            shift
            ;;
        --complexity|-c)
            COMPLEXITY=true
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --format-only)
            FORMAT_ONLY=true
            shift
            ;;
        --install)
            INSTALL_TOOLS=true
            shift
            ;;
        --help|-h)
            echo "Backend2 Code Quality Checker"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --fix, -f         Auto-fix issues where possible"
            echo "  --strict          Enable strict checking mode"
            echo "  --security, -s    Run security analysis"
            echo "  --complexity, -c  Check code complexity"
            echo "  --verbose, -v     Show detailed output"
            echo "  --format-only     Only check/fix formatting"
            echo "  --install         Install required tools"
            echo "  --help, -h        Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                    # Basic linting"
            echo "  $0 --fix             # Auto-fix issues"
            echo "  $0 --strict --security  # Comprehensive check"
            echo "  $0 --format-only --fix  # Format code only"
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
echo -e "${BLUE}    Backend2 Code Quality Check${NC}"
echo -e "${BLUE}====================================${NC}"
echo ""

# Function to install a tool if not present
install_tool() {
    local tool_name=$1
    local install_cmd=$2
    
    if ! command -v "$tool_name" &> /dev/null; then
        if [ "$INSTALL_TOOLS" = true ]; then
            echo -e "${YELLOW}Installing $tool_name...${NC}"
            eval "$install_cmd"
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}✅ $tool_name installed${NC}"
            else
                echo -e "${RED}❌ Failed to install $tool_name${NC}"
                return 1
            fi
        else
            echo -e "${YELLOW}⚠️  $tool_name is not installed. Run with --install to install tools${NC}"
            return 1
        fi
    fi
    return 0
}

# 1. Format Check
if [ "$FORMAT_ONLY" = true ] || [ "$FORMAT_ONLY" = false ]; then
    echo -e "${CYAN}📝 Checking code formatting...${NC}"
    
    # gofmt check
    unformatted=$(gofmt -l . 2>/dev/null | grep -v "vendor\|build\|wire_gen.go\|tmp" || true)
    if [ -n "$unformatted" ]; then
        ISSUES_FOUND=true
        if [ "$FIX" = true ]; then
            echo -e "${YELLOW}Fixing formatting issues...${NC}"
            gofmt -w .
            echo -e "${GREEN}✅ Formatting fixed${NC}"
        else
            echo -e "${RED}✗ Files need formatting:${NC}"
            echo "$unformatted"
            echo -e "${YELLOW}Run with --fix to auto-format${NC}"
        fi
    else
        echo -e "${GREEN}✅ Code formatting is correct${NC}"
    fi
    
    # goimports check
    if install_tool "goimports" "go install golang.org/x/tools/cmd/goimports@latest"; then
        echo -e "${CYAN}📦 Checking imports...${NC}"
        unimported=$(goimports -l . 2>/dev/null | grep -v "vendor\|build\|wire_gen.go\|tmp" || true)
        if [ -n "$unimported" ]; then
            ISSUES_FOUND=true
            if [ "$FIX" = true ]; then
                echo -e "${YELLOW}Fixing import issues...${NC}"
                goimports -w .
                echo -e "${GREEN}✅ Imports fixed${NC}"
            else
                echo -e "${RED}✗ Files have import issues:${NC}"
                echo "$unimported"
                echo -e "${YELLOW}Run with --fix to auto-fix imports${NC}"
            fi
        else
            echo -e "${GREEN}✅ Imports are correct${NC}"
        fi
    fi
fi

if [ "$FORMAT_ONLY" = true ]; then
    if [ "$ISSUES_FOUND" = false ]; then
        echo ""
        echo -e "${GREEN}✨ All formatting checks passed!${NC}"
    fi
    exit 0
fi

# 2. Go Vet
echo ""
echo -e "${CYAN}🔍 Running go vet...${NC}"
if go vet ./... 2>&1 | grep -v "vendor"; then
    echo -e "${GREEN}✅ go vet passed${NC}"
else
    ISSUES_FOUND=true
    echo -e "${RED}✗ go vet found issues${NC}"
fi

# 3. Staticcheck
if install_tool "staticcheck" "go install honnef.co/go/tools/cmd/staticcheck@latest"; then
    echo ""
    echo -e "${CYAN}🔍 Running staticcheck...${NC}"
    if [ "$STRICT" = true ]; then
        if staticcheck -checks all ./...; then
            echo -e "${GREEN}✅ staticcheck passed (strict mode)${NC}"
        else
            ISSUES_FOUND=true
            echo -e "${RED}✗ staticcheck found issues${NC}"
        fi
    else
        if staticcheck ./...; then
            echo -e "${GREEN}✅ staticcheck passed${NC}"
        else
            ISSUES_FOUND=true
            echo -e "${RED}✗ staticcheck found issues${NC}"
        fi
    fi
fi

# 4. golangci-lint
if install_tool "golangci-lint" "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; then
    echo ""
    echo -e "${CYAN}🔍 Running golangci-lint...${NC}"
    
    # Create .golangci.yml if it doesn't exist
    if [ ! -f ".golangci.yml" ]; then
        echo -e "${YELLOW}Creating .golangci.yml configuration...${NC}"
        cat > .golangci.yml <<'EOF'
linters:
  enable:
    - gofmt
    - goimports
    - golint
    - govet
    - errcheck
    - ineffassign
    - unconvert
    - misspell
    - prealloc
    - nakedret
    - gocritic
    - gochecknoinits
    - gochecknoglobals
    - dupl
    - goconst

linters-settings:
  dupl:
    threshold: 100
  goconst:
    min-len: 2
    min-occurrences: 2
  misspell:
    locale: US

issues:
  exclude-use-default: false
  exclude:
    - "should have comment or be unexported"
    - "don't use an underscore in package name"
  exclude-rules:
    - path: _test\.go
      linters:
        - dupl
    - path: wire_gen\.go
      linters:
        - all

run:
  skip-dirs:
    - vendor
    - build
    - tmp
EOF
    fi
    
    lint_cmd="golangci-lint run --timeout 3m"
    if [ "$FIX" = true ]; then
        lint_cmd="$lint_cmd --fix"
    fi
    if [ "$VERBOSE" = true ]; then
        lint_cmd="$lint_cmd -v"
    fi
    
    if $lint_cmd; then
        echo -e "${GREEN}✅ golangci-lint passed${NC}"
    else
        ISSUES_FOUND=true
        echo -e "${RED}✗ golangci-lint found issues${NC}"
        if [ "$FIX" = false ]; then
            echo -e "${YELLOW}Run with --fix to auto-fix some issues${NC}"
        fi
    fi
fi

# 5. Security Check
if [ "$SECURITY" = true ]; then
    if install_tool "gosec" "go install github.com/securego/gosec/v2/cmd/gosec@latest"; then
        echo ""
        echo -e "${MAGENTA}🔒 Running security analysis...${NC}"
        
        # Create output directory
        mkdir -p reports
        
        if [ "$VERBOSE" = true ]; then
            gosec -fmt json -out reports/security.json -stdout -verbose ./...
        else
            if gosec -fmt json -out reports/security.json ./... 2>/dev/null; then
                echo -e "${GREEN}✅ No security issues found${NC}"
            else
                ISSUES_FOUND=true
                echo -e "${YELLOW}⚠️  Security issues found. Check reports/security.json${NC}"
                
                # Show summary
                if [ -f "reports/security.json" ]; then
                    issue_count=$(grep -o '"severity"' reports/security.json | wc -l)
                    high_count=$(grep -o '"severity":"HIGH"' reports/security.json | wc -l)
                    medium_count=$(grep -o '"severity":"MEDIUM"' reports/security.json | wc -l)
                    low_count=$(grep -o '"severity":"LOW"' reports/security.json | wc -l)
                    
                    echo "  High:   $high_count"
                    echo "  Medium: $medium_count"
                    echo "  Low:    $low_count"
                fi
            fi
        fi
    fi
fi

# 6. Complexity Check
if [ "$COMPLEXITY" = true ]; then
    if install_tool "gocyclo" "go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"; then
        echo ""
        echo -e "${MAGENTA}📊 Checking code complexity...${NC}"
        
        complexity_threshold=10
        if [ "$STRICT" = true ]; then
            complexity_threshold=7
        fi
        
        complex_functions=$(gocyclo -over $complexity_threshold . 2>/dev/null | grep -v "vendor\|build\|wire_gen.go" || true)
        if [ -n "$complex_functions" ]; then
            ISSUES_FOUND=true
            echo -e "${YELLOW}⚠️  Functions with high complexity (threshold: $complexity_threshold):${NC}"
            echo "$complex_functions"
            echo -e "${YELLOW}Consider refactoring these functions${NC}"
        else
            echo -e "${GREEN}✅ All functions are within complexity threshold ($complexity_threshold)${NC}"
        fi
    fi
    
    # Check for code duplication
    if install_tool "dupl" "go install github.com/mibk/dupl@latest"; then
        echo ""
        echo -e "${MAGENTA}🔍 Checking for code duplication...${NC}"
        
        dupl_threshold=50
        if [ "$STRICT" = true ]; then
            dupl_threshold=30
        fi
        
        duplicates=$(dupl -t $dupl_threshold . 2>/dev/null | grep -v "vendor\|build\|wire_gen.go\|_test.go" || true)
        if [ -n "$duplicates" ]; then
            ISSUES_FOUND=true
            echo -e "${YELLOW}⚠️  Duplicate code found (threshold: $dupl_threshold tokens):${NC}"
            echo "$duplicates" | head -20
            echo -e "${YELLOW}Consider refactoring duplicate code${NC}"
        else
            echo -e "${GREEN}✅ No significant code duplication found${NC}"
        fi
    fi
fi

# 7. Check for TODOs and FIXMEs
echo ""
echo -e "${CYAN}📌 Checking for TODOs and FIXMEs...${NC}"
todos=$(grep -r "TODO\|FIXME\|XXX\|HACK" --include="*.go" . 2>/dev/null | grep -v "vendor\|build" | wc -l || echo "0")
if [ "$todos" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Found $todos TODO/FIXME comments${NC}"
    if [ "$VERBOSE" = true ]; then
        echo "Locations:"
        grep -rn "TODO\|FIXME\|XXX\|HACK" --include="*.go" . 2>/dev/null | grep -v "vendor\|build" | head -10
    fi
else
    echo -e "${GREEN}✅ No TODO/FIXME comments found${NC}"
fi

# 8. Check go.mod
echo ""
echo -e "${CYAN}📦 Checking dependencies...${NC}"
if go mod verify; then
    echo -e "${GREEN}✅ Dependencies verified${NC}"
else
    ISSUES_FOUND=true
    echo -e "${RED}✗ Dependency verification failed${NC}"
fi

# Check for outdated dependencies
if install_tool "go-mod-outdated" "go install github.com/psampaz/go-mod-outdated@latest"; then
    outdated=$(go list -u -m -json all | go-mod-outdated -direct 2>/dev/null || true)
    if [ -n "$outdated" ]; then
        echo -e "${YELLOW}⚠️  Outdated dependencies found:${NC}"
        echo "$outdated"
    else
        echo -e "${GREEN}✅ All dependencies are up to date${NC}"
    fi
fi

# Final Summary
echo ""
echo -e "${BLUE}====================================${NC}"
if [ "$ISSUES_FOUND" = true ]; then
    echo -e "${RED}    ⚠️  Issues Found${NC}"
    echo -e "${BLUE}====================================${NC}"
    echo ""
    echo -e "${YELLOW}Recommendations:${NC}"
    echo "  • Run with --fix to auto-fix formatting issues"
    echo "  • Address the issues reported above"
    echo "  • Run with --strict for more thorough checking"
    exit 1
else
    echo -e "${GREEN}    ✅ All Checks Passed${NC}"
    echo -e "${BLUE}====================================${NC}"
    echo ""
    echo -e "${GREEN}✨ Code quality looks great!${NC}"
    
    if [ "$STRICT" = false ]; then
        echo ""
        echo -e "${CYAN}For more thorough checking:${NC}"
        echo "  • Run with --strict for stricter rules"
        echo "  • Run with --security for security analysis"
        echo "  • Run with --complexity for complexity checks"
    fi
fi