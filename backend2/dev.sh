#!/bin/bash
# Backend2 Development Server
# Quick development runner with hot reload support

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse command line arguments
SERVICE="api"  # Default service
HOT_RELOAD=true
DEBUG=false
PORT=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --service)
            SERVICE="$2"
            shift 2
            ;;
        --no-reload)
            HOT_RELOAD=false
            shift
            ;;
        --debug)
            DEBUG=true
            shift
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        --help|-h)
            echo "Backend2 Development Server"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --service <name>  Service to run: api, worker, migrate (default: api)"
            echo "  --no-reload       Disable hot reload"
            echo "  --debug           Enable debug logging"
            echo "  --port <port>     Override default port"
            echo "  --help, -h        Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Run API with hot reload"
            echo "  $0 --service worker        # Run worker service"
            echo "  $0 --port 8081 --debug     # Run API on port 8081 with debug"
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
echo -e "${BLUE}    Backend2 Development Server${NC}"
echo -e "${BLUE}    Service: $SERVICE${NC}"
echo -e "${BLUE}====================================${NC}"
echo ""

# Set environment variables for development
export GO_ENV=development
export LOG_LEVEL=${DEBUG:+debug}
export LOG_LEVEL=${LOG_LEVEL:-info}

# Override port if specified
if [ -n "$PORT" ]; then
    export API_PORT=$PORT
    echo -e "${YELLOW}📍 Using custom port: $PORT${NC}"
fi

# Ensure dependencies are up to date
echo -e "${YELLOW}📦 Checking dependencies...${NC}"
go mod tidy

# Generate Wire dependencies
if [ -d "infrastructure/di" ]; then
    echo -e "${YELLOW}🔄 Generating dependency injection code...${NC}"
    cd infrastructure/di
    if ! command -v wire &> /dev/null; then
        echo "Installing Wire..."
        go install github.com/google/wire/cmd/wire@latest
    fi
    wire
    cd ../..
fi

case $SERVICE in
    api)
        if [ "$HOT_RELOAD" = true ]; then
            echo -e "${GREEN}🔄 Starting API server with hot reload...${NC}"
            
            # Check if air is installed
            if ! command -v air &> /dev/null; then
                echo -e "${YELLOW}Installing Air for hot reload...${NC}"
                go install github.com/cosmtrek/air@latest
            fi
            
            # Create .air.toml if it doesn't exist
            if [ ! -f ".air.toml" ]; then
                echo -e "${YELLOW}Creating .air.toml configuration...${NC}"
                cat > .air.toml <<'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/api"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "build"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_error = true

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
EOF
            fi
            
            # Run with air
            air
        else
            echo -e "${GREEN}🚀 Starting API server...${NC}"
            go run ./cmd/api
        fi
        ;;
        
    worker)
        if [ ! -f "cmd/worker/main.go" ]; then
            echo -e "${YELLOW}⚠️  Worker service not implemented yet${NC}"
            echo "Creating placeholder..."
            mkdir -p cmd/worker
            cat > cmd/worker/main.go <<'EOF'
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    log.Println("🚀 Worker service starting...")
    
    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // Worker loop
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            fmt.Println("⚙️  Worker processing...")
            // TODO: Add actual work processing
        case sig := <-sigChan:
            log.Printf("Received signal: %v", sig)
            log.Println("👋 Worker shutting down...")
            os.Exit(0)
        }
    }
}
EOF
        fi
        
        if [ "$HOT_RELOAD" = true ]; then
            echo -e "${GREEN}🔄 Starting worker service with hot reload...${NC}"
            
            if ! command -v air &> /dev/null; then
                echo -e "${YELLOW}Installing Air for hot reload...${NC}"
                go install github.com/cosmtrek/air@latest
            fi
            
            # Create custom air config for worker
            cat > .air.worker.toml <<'EOF'
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/worker"
  cmd = "go build -o ./tmp/worker ./cmd/worker"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "build"]
  include_ext = ["go"]
  kill_delay = "0s"
  log = "worker-build-errors.log"
  stop_on_error = true
EOF
            air -c .air.worker.toml
        else
            echo -e "${GREEN}⚙️  Starting worker service...${NC}"
            go run ./cmd/worker
        fi
        ;;
        
    migrate)
        if [ ! -f "cmd/migrate/main.go" ]; then
            echo -e "${YELLOW}⚠️  Migration tool not implemented yet${NC}"
            echo "Creating placeholder..."
            mkdir -p cmd/migrate
            cat > cmd/migrate/main.go <<'EOF'
package main

import (
    "fmt"
    "log"
    "os"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: migrate [up|down|status]")
        os.Exit(1)
    }
    
    command := os.Args[1]
    
    switch command {
    case "up":
        log.Println("⬆️  Running migrations...")
        // TODO: Implement migration up
        log.Println("✅ Migrations completed")
    case "down":
        log.Println("⬇️  Rolling back migrations...")
        // TODO: Implement migration down
        log.Println("✅ Rollback completed")
    case "status":
        log.Println("📊 Migration status:")
        // TODO: Show migration status
        log.Println("No migrations found")
    default:
        fmt.Printf("Unknown command: %s\n", command)
        fmt.Println("Usage: migrate [up|down|status]")
        os.Exit(1)
    }
}
EOF
        fi
        
        echo -e "${GREEN}🔨 Running migration tool...${NC}"
        shift  # Remove the service argument
        go run ./cmd/migrate "$@"
        ;;
        
    *)
        echo -e "${RED}❌ Unknown service: $SERVICE${NC}"
        echo "Available services: api, worker, migrate"
        exit 1
        ;;
esac