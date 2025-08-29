# Local Development Guide

This guide covers running the Brain2 backend locally for development and testing, including both Lambda emulation and standard HTTP server modes.

## Table of Contents
- [Development Environment Setup](#development-environment-setup)
- [Running as HTTP Server (Recommended for Development)](#running-as-http-server-recommended-for-development)
- [Running with Lambda Emulation](#running-with-lambda-emulation)
- [Database Setup Options](#database-setup-options)
- [Environment Configuration](#environment-configuration)
- [Testing and Debugging](#testing-and-debugging)
- [Common Development Workflows](#common-development-workflows)

---

## Development Environment Setup

### Prerequisites

```bash
# Install Go 1.22+
curl -OL https://golang.org/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install Wire for dependency injection
go install github.com/google/wire/cmd/wire@latest

# Install AWS CLI (for DynamoDB Local)
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Install development tools
go install github.com/cosmtrek/air@latest      # Hot reload
go install github.com/go-delve/delve/cmd/dlv@latest  # Debugger
```

### Project Setup

```bash
# Clone and setup
git clone <repository-url>
cd b2/backend

# Install dependencies
go mod download
go mod tidy

# Verify Wire setup
cd internal/di
wire check
cd ../..

# Verify build
go build ./cmd/main/main.go
```

---

## Running as HTTP Server (Recommended for Development)

For faster development cycles, run the backend as a standard HTTP server instead of Lambda emulation.

### Step 1: Create HTTP Server Entry Point

```go
// cmd/dev-server/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "brain2-backend/internal/di"
)

func main() {
    // Initialize dependency injection container
    container, err := di.InitializeContainer()
    if err != nil {
        log.Fatalf("Failed to initialize container: %v", err)
    }

    // Get the Chi router from container
    router := container.GetRouter()

    // Create HTTP server
    server := &http.Server{
        Addr:         ":8080",
        Handler:      router,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // Start server in goroutine
    go func() {
        log.Printf("Starting development server on http://localhost:8080")
        log.Printf("API endpoints available at http://localhost:8080/api/v1/")
        
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed to start: %v", err)
        }
    }()

    // Wait for interrupt signal to gracefully shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down server...")

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    // Cleanup container resources
    if err := container.Shutdown(context.Background()); err != nil {
        log.Printf("Error during container shutdown: %v", err)
    }

    log.Println("Server exited")
}
```

### Step 2: Create Development Environment

```bash
# Create development environment file
cp .env.example .env.dev
```

```bash
# .env.dev - Development configuration
ENV=development
LOG_LEVEL=debug

# Local DynamoDB (see Database Setup section)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=local
AWS_SECRET_ACCESS_KEY=local
AWS_ENDPOINT_URL=http://localhost:8000

# Database
TABLE_NAME=brain2-dev
INDEX_NAME=GSI1

# EventBridge (optional - use local or mock)
EVENT_BUS_NAME=brain2-dev-events
EVENT_BUS_ARN=arn:aws:events:us-east-1:000000000000:event-bus/brain2-dev-events

# Disable AWS authentication for local development
AWS_SDK_LOAD_CONFIG=false

# Enable debug features
WIRE_DEBUG=true
ENABLE_REQUEST_LOGGING=true
ENABLE_PERFORMANCE_LOGGING=true
```

### Step 3: Run Development Server

```bash
# Option 1: Direct run
ENV=development go run cmd/dev-server/main.go

# Option 2: With hot reload (recommended)
ENV=development air -c .air.toml

# Option 3: Build and run
go build -o bin/dev-server cmd/dev-server/main.go
ENV=development ./bin/dev-server
```

### Step 4: Create Air Configuration for Hot Reload

```toml
# .air.toml - Hot reload configuration
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main cmd/dev-server/main.go"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "build", "docs"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html", "yaml", "yml"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

### Step 5: Test the Development Server

```bash
# Test health endpoint
curl http://localhost:8080/health

# Test API endpoints (requires authentication)
curl -H "Authorization: Bearer your-jwt-token" \
     http://localhost:8080/api/v1/memories

# Test with sample data
curl -X POST http://localhost:8080/api/v1/memories \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-jwt-token" \
  -d '{"content": "Test memory", "title": "Test", "tags": ["test"]}'
```

---

## Running with Lambda Emulation

For testing Lambda-specific behavior, use AWS SAM or serverless-offline.

### Option 1: AWS SAM Local

#### Step 1: Create SAM Template

```yaml
# template.yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Globals:
  Function:
    Timeout: 30
    MemorySize: 512
    Runtime: provided.al2
    Environment:
      Variables:
        ENV: development
        TABLE_NAME: brain2-dev
        INDEX_NAME: GSI1
        AWS_REGION: us-east-1

Resources:
  MainFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: build/main/
      Handler: bootstrap
      Events:
        ApiEvent:
          Type: HttpApi
          Properties:
            Path: /{proxy+}
            Method: ANY

  WebSocketConnectFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: build/ws-connect/
      Handler: bootstrap

  WebSocketDisconnectFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: build/ws-disconnect/
      Handler: bootstrap

Outputs:
  ApiEndpoint:
    Description: "API Gateway endpoint URL"
    Value: !Sub "https://${ServerlessHttpApi}.execute-api.${AWS::Region}.amazonaws.com/"
```

#### Step 2: Build and Run with SAM

```bash
# Build Lambda functions
./build.sh

# Start SAM local
sam local start-api --port 3000

# Test endpoints
curl http://localhost:3000/health
curl http://localhost:3000/api/v1/memories
```

### Option 2: Direct Lambda Testing

```go
// cmd/test-lambda/main.go - Test Lambda function directly
package main

import (
    "context"
    "encoding/json"
    "log"

    "brain2-backend/internal/di"
    
    "github.com/aws/aws-lambda-go/events"
)

func main() {
    // Initialize container
    container, err := di.InitializeContainer()
    if err != nil {
        log.Fatalf("Failed to initialize container: %v", err)
    }

    // Create test API Gateway request
    req := events.APIGatewayV2HTTPRequest{
        RequestContext: events.APIGatewayV2HTTPRequestContext{
            HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
                Method: "GET",
                Path:   "/health",
            },
        },
        Headers: map[string]string{
            "Content-Type": "application/json",
        },
    }

    // Get Lambda handler function (this is what would be called by Lambda runtime)
    // You'll need to extract the handler logic from cmd/main/main.go
    response := handleRequest(context.Background(), req, container)

    // Print response
    responseJSON, _ := json.MarshalIndent(response, "", "  ")
    log.Printf("Response: %s", responseJSON)
}

func handleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest, container *di.Container) events.APIGatewayV2HTTPResponse {
    // Implementation would mirror the logic in cmd/main/main.go
    // but return the response instead of using the Lambda runtime
    return events.APIGatewayV2HTTPResponse{
        StatusCode: 200,
        Body:       `{"status": "healthy"}`,
        Headers: map[string]string{
            "Content-Type": "application/json",
        },
    }
}
```

---

## Database Setup Options

### Option 1: DynamoDB Local (Recommended)

```bash
# Download DynamoDB Local
wget https://s3.us-west-2.amazonaws.com/dynamodb-local/dynamodb_local_latest.tar.gz
tar -xvzf dynamodb_local_latest.tar.gz
mkdir dynamodb-local
mv DynamoDBLocal.jar dynamodb-local/
mv DynamoDBLocal_lib dynamodb-local/

# Start DynamoDB Local
cd dynamodb-local
java -Djava.library.path=./DynamoDBLocal_lib -jar DynamoDBLocal.jar -sharedDb -port 8000

# Or run in background
nohup java -Djava.library.path=./DynamoDBLocal_lib -jar DynamoDBLocal.jar -sharedDb -port 8000 > dynamodb.log 2>&1 &
```

### Option 2: Docker DynamoDB Local

```bash
# Run DynamoDB Local in Docker
docker run -p 8000:8000 amazon/dynamodb-local

# Or with data persistence
docker run -p 8000:8000 -v /tmp/dynamodb-data:/home/dynamodblocal/data \
  amazon/dynamodb-local -jar DynamoDBLocal.jar -dbPath /home/dynamodblocal/data -sharedDb
```

### Option 3: LocalStack (Full AWS Emulation)

```bash
# Install LocalStack
pip install localstack

# Start LocalStack with DynamoDB and EventBridge
SERVICES=dynamodb,events localstack start

# DynamoDB will be available on port 4566
```

### Database Setup Script

```bash
#!/bin/bash
# scripts/setup-local-db.sh - Create local development database

# Wait for DynamoDB Local to be ready
echo "Waiting for DynamoDB Local..."
until curl -s http://localhost:8000 > /dev/null; do
  sleep 1
done

# Configure AWS CLI for local DynamoDB
aws configure set aws_access_key_id local
aws configure set aws_secret_access_key local
aws configure set region us-east-1

# Create table
aws dynamodb create-table \
  --endpoint-url http://localhost:8000 \
  --table-name brain2-dev \
  --attribute-definitions \
    AttributeName=PK,AttributeType=S \
    AttributeName=SK,AttributeType=S \
    AttributeName=GSI1PK,AttributeType=S \
    AttributeName=GSI1SK,AttributeType=S \
  --key-schema \
    AttributeName=PK,KeyType=HASH \
    AttributeName=SK,KeyType=RANGE \
  --global-secondary-indexes \
    'IndexName=GSI1,KeySchema=[{AttributeName=GSI1PK,KeyType=HASH},{AttributeName=GSI1SK,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}' \
  --provisioned-throughput \
    ReadCapacityUnits=5,WriteCapacityUnits=5

echo "Table created successfully!"

# Verify table creation
aws dynamodb describe-table \
  --endpoint-url http://localhost:8000 \
  --table-name brain2-dev \
  --query 'Table.TableStatus'
```

---

## Environment Configuration

### Development Environment Variables

Create different environment files for different scenarios:

```bash
# .env.dev - Local development with DynamoDB Local
ENV=development
LOG_LEVEL=debug
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=local
AWS_SECRET_ACCESS_KEY=local
AWS_ENDPOINT_URL=http://localhost:8000
TABLE_NAME=brain2-dev
INDEX_NAME=GSI1
EVENT_BUS_NAME=brain2-dev-events

# .env.test - Testing environment
ENV=test
LOG_LEVEL=error
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_ENDPOINT_URL=http://localhost:8000
TABLE_NAME=brain2-test
INDEX_NAME=GSI1
EVENT_BUS_NAME=brain2-test-events

# .env.integration - Integration testing with real AWS
ENV=integration
LOG_LEVEL=info
AWS_REGION=us-east-1
# AWS credentials from ~/.aws/credentials
TABLE_NAME=brain2-integration
INDEX_NAME=GSI1
EVENT_BUS_NAME=brain2-integration-events
```

### Environment Loading

```go
// internal/config/config.go - Add development-specific config

func LoadConfig() *Config {
    cfg := &Config{}
    
    // Load from environment file if present
    if envFile := os.Getenv("ENV_FILE"); envFile != "" {
        if err := loadEnvFile(envFile); err != nil {
            log.Printf("Warning: Could not load env file %s: %v", envFile, err)
        }
    } else {
        // Try to load .env.dev in development
        if os.Getenv("ENV") == "development" {
            if err := loadEnvFile(".env.dev"); err != nil {
                log.Printf("Info: No .env.dev file found")
            }
        }
    }
    
    // Load configuration from environment variables
    cfg.Environment = getEnvWithDefault("ENV", "development")
    cfg.LogLevel = getEnvWithDefault("LOG_LEVEL", "info")
    cfg.Database.TableName = getEnvWithDefault("TABLE_NAME", "brain2-dev")
    cfg.Database.IndexName = getEnvWithDefault("INDEX_NAME", "GSI1")
    
    // Development-specific settings
    if cfg.Environment == "development" {
        cfg.Debug = true
        cfg.EnableRequestLogging = getEnvBool("ENABLE_REQUEST_LOGGING", true)
        cfg.EnablePerformanceLogging = getEnvBool("ENABLE_PERFORMANCE_LOGGING", true)
    }
    
    return cfg
}

func loadEnvFile(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if len(line) == 0 || strings.HasPrefix(line, "#") {
            continue
        }
        
        parts := strings.SplitN(line, "=", 2)
        if len(parts) == 2 {
            key := strings.TrimSpace(parts[0])
            value := strings.TrimSpace(parts[1])
            os.Setenv(key, value)
        }
    }
    
    return scanner.Err()
}
```

---

## Testing and Debugging

### Running Tests Locally

```bash
# Run all tests
go test ./...

# Run tests with local DynamoDB
ENV=test go test ./...

# Run integration tests
ENV=integration go test -tags=integration ./...

# Run specific test package
go test ./internal/domain/node/...

# Run with coverage
go test -cover ./...
```

### Debugging with Delve

```bash
# Debug development server
dlv debug cmd/dev-server/main.go

# Debug with arguments
dlv debug cmd/dev-server/main.go -- --config=dev

# Debug tests
dlv test ./internal/domain/node

# Remote debugging (useful for Docker)
dlv debug cmd/dev-server/main.go --headless --listen=:2345 --api-version=2
```

### Debug Configuration for VS Code

```json
// .vscode/launch.json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Dev Server",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/dev-server/main.go",
            "env": {
                "ENV": "development",
                "LOG_LEVEL": "debug"
            },
            "args": []
        },
        {
            "name": "Debug Tests",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}/internal/domain/node",
            "env": {
                "ENV": "test"
            }
        },
        {
            "name": "Debug Lambda Function",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/main/main.go",
            "env": {
                "ENV": "development",
                "_LAMBDA_SERVER_PORT": "8080"
            }
        }
    ]
}
```

### Performance Profiling

```go
// Enable profiling in development server
import _ "net/http/pprof"

func main() {
    // In development, start pprof server
    if os.Getenv("ENV") == "development" {
        go func() {
            log.Println("Starting pprof server on :6060")
            log.Println(http.ListenAndServe("localhost:6060", nil))
        }()
    }
    
    // ... rest of main function
}
```

```bash
# Profile CPU usage
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Profile memory usage
go tool pprof http://localhost:6060/debug/pprof/heap

# Profile goroutines
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

---

## Common Development Workflows

### Workflow 1: Adding New Feature

```bash
# 1. Start development environment
docker run -d -p 8000:8000 amazon/dynamodb-local  # Start DB
ENV_FILE=.env.dev air  # Start server with hot reload

# 2. Create feature branch
git checkout -b feature/new-feature

# 3. Add domain logic, tests, handlers
# ... make changes ...

# 4. Test changes
curl http://localhost:8080/api/v1/your-new-endpoint

# 5. Run tests
go test ./...

# 6. Build for Lambda
./build.sh

# 7. Test with SAM local
sam local start-api
```

### Workflow 2: Database Schema Changes

```bash
# 1. Update domain entities
# ... modify Go structs ...

# 2. Update repository implementations
# ... modify DynamoDB mapping ...

# 3. Test migration with existing data
ENV=test go test ./internal/infrastructure/persistence/dynamodb/...

# 4. Run integration tests
./scripts/setup-local-db.sh  # Create fresh database
ENV=integration go test -tags=integration ./...
```

### Workflow 3: Performance Testing

```bash
# 1. Start development server with profiling
ENV=development go run cmd/dev-server/main.go

# 2. Generate load
for i in {1..100}; do
  curl -s http://localhost:8080/health > /dev/null &
done
wait

# 3. Collect profiles
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=10

# 4. Analyze results
# (pprof) top10
# (pprof) web
```

### Workflow 4: Integration Testing

```bash
# 1. Setup integration environment
export ENV=integration
export TABLE_NAME=brain2-integration-$USER
export EVENT_BUS_NAME=brain2-integration-$USER

# 2. Create AWS resources (one-time)
aws dynamodb create-table --cli-input-json file://table-definition.json

# 3. Run integration tests
go test -tags=integration ./...

# 4. Cleanup resources
aws dynamodb delete-table --table-name $TABLE_NAME
```

### Development Makefile

```makefile
# Makefile - Common development tasks

.PHONY: dev test build clean setup-db

# Start development server
dev:
	ENV_FILE=.env.dev air

# Run tests
test:
	ENV=test go test ./...

# Run integration tests
test-integration:
	ENV=integration go test -tags=integration ./...

# Build all Lambda functions
build:
	./build.sh

# Clean build artifacts
clean:
	rm -rf build/ tmp/ bin/

# Setup local database
setup-db:
	./scripts/setup-local-db.sh

# Start local services
services:
	docker-compose up -d dynamodb

# Stop local services  
services-stop:
	docker-compose down

# Generate Wire dependencies
wire:
	cd internal/di && wire

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Full development setup
setup: clean wire build setup-db
	@echo "Development environment ready!"
```

This local development guide provides comprehensive setup for efficient development, testing, and debugging of the Brain2 backend outside of the Lambda environment.