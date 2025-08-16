# Brain2 Backend - Command Master List

This document provides a comprehensive list of all commands for developing, building, testing, and deploying the Brain2 backend application.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Environment Setup](#environment-setup)
- [Development Commands](#development-commands)
- [Build Commands](#build-commands)
- [Testing Commands](#testing-commands)
- [Dependency Injection (Wire)](#dependency-injection-wire)
- [Local Development](#local-development)
- [AWS/Lambda Commands](#awslambda-commands)
- [Docker Commands](#docker-commands)
- [Database Commands](#database-commands)
- [Code Quality](#code-quality)
- [Debugging Commands](#debugging-commands)
- [Environment Variables Reference](#environment-variables-reference)

---

## Prerequisites

### Install Required Tools
```bash
# Install Go (1.22 or higher)
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install Wire for dependency injection
go install github.com/google/wire/cmd/wire@latest

# Install AWS CLI
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Install SAM CLI (for local Lambda testing)
pip install aws-sam-cli

# Install golangci-lint (for code quality)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2

# Install go-swagger (for API documentation)
go install github.com/go-swagger/go-swagger/cmd/swagger@latest
```

---

## Environment Setup

### Initial Project Setup
```bash
# Clone the repository
git clone <repository-url>
cd b2/backend

# Install Go dependencies
go mod download
go mod tidy

# Verify installation
go mod verify

# Create local environment file
cp .env.example .env  # Edit with your values
```

### Configure AWS Credentials
```bash
# Configure AWS CLI
aws configure
# Enter: AWS Access Key ID, Secret Access Key, Region, Output format

# Or use environment variables
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1

# For local development with LocalStack
export AWS_ENDPOINT=http://localhost:4566
```

---

## Development Commands

### Dependency Management
```bash
# Download all dependencies
go mod download

# Add a new dependency
go get github.com/package/name

# Update dependencies
go get -u ./...

# Clean up unused dependencies
go mod tidy

# Vendor dependencies (optional)
go mod vendor

# Verify dependencies
go mod verify

# View dependency graph
go mod graph
```

### Code Generation
```bash
# Generate mocks for testing
go generate ./...

# Generate OpenAPI types
go run cmd/codegen/main.go

# Generate API documentation
swagger generate spec -o ./swagger.json
```

---

## Build Commands

### Quick Build (for testing compilation)
```bash
# Build without tests
./test_build.sh

# Build single Lambda function
go build -o bin/main cmd/main/main.go

# Build with race detector (development only)
go build -race -o bin/main cmd/main/main.go
```

### Full Build (with tests and Wire)
```bash
# Complete build process
./build.sh

# This script:
# 1. Cleans previous artifacts
# 2. Installs dependencies
# 3. Runs tests
# 4. Validates Wire configuration
# 5. Generates dependency injection code
# 6. Builds all Lambda functions
```

### Manual Build Steps
```bash
# Clean build artifacts
rm -rf build/

# Build for Lambda (Linux/AMD64)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o build/bootstrap cmd/main/main.go

# Make executable
chmod +x build/bootstrap

# Build with debugging symbols
GOOS=linux GOARCH=amd64 go build \
  -gcflags="all=-N -l" \
  -o build/bootstrap cmd/main/main.go
```

---

## Testing Commands

### Run Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with detailed coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./internal/domain
go test ./internal/service/...

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestNodeCreation ./...

# Run tests with timeout
go test -timeout 30s ./...

# Run integration tests (if tagged)
go test -tags=integration ./...

# Run benchmarks
go test -bench=. ./...
go test -bench=. -benchmem ./...

# Run tests in parallel
go test -parallel 4 ./...
```

### Test Coverage Analysis
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Check coverage percentage
go test -cover ./... | grep -E '^(ok|FAIL)' | awk '{print $2, $5}'
```

---

## Dependency Injection (Wire)

### Wire Commands
```bash
# Navigate to DI directory
cd internal/di

# Check Wire configuration
wire check

# Generate dependency injection code
wire

# Or use go generate
go generate

# Diff generated code
wire diff

# Update Wire
go get -u github.com/google/wire/cmd/wire

# Clean Wire cache
rm -rf .wire_cache
```

### Wire Troubleshooting
```bash
# Validate providers
cd internal/di && wire check

# Show Wire version
wire version

# Debug Wire generation
WIRE_DEBUG=1 wire

# Force regeneration
rm wire_gen.go && wire
```

---

## Local Development

### Run Locally
```bash
# Run the main application
go run cmd/main/main.go

# Run with environment variables
ENV=development go run cmd/main/main.go

# Run with custom config
go run cmd/main/main.go -config=./config/dev.yaml

# Run with hot reload (requires air)
go install github.com/cosmtrek/air@latest
air

# Run with dlv debugger
dlv debug cmd/main/main.go
```

### SAM Local (Lambda Emulation)
```bash
# Start API Gateway locally
sam local start-api

# Start with specific template
sam local start-api -t template.yaml

# Start with environment variables
sam local start-api --env-vars env.json

# Start on different port
sam local start-api --port 3001

# Invoke specific function
sam local invoke "FunctionName" -e event.json

# Generate sample event
sam local generate-event apigateway aws-proxy > event.json
```

### Local DynamoDB
```bash
# Run DynamoDB Local with Docker
docker run -d -p 8000:8000 \
  --name dynamodb-local \
  amazon/dynamodb-local

# Create local table
aws dynamodb create-table \
  --table-name brain2-dev \
  --attribute-definitions \
    AttributeName=PK,AttributeType=S \
    AttributeName=SK,AttributeType=S \
  --key-schema \
    AttributeName=PK,KeyType=HASH \
    AttributeName=SK,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --endpoint-url http://localhost:8000

# List tables
aws dynamodb list-tables --endpoint-url http://localhost:8000

# Scan table
aws dynamodb scan --table-name brain2-dev \
  --endpoint-url http://localhost:8000
```

---

## AWS/Lambda Commands

### Deployment
```bash
# Package Lambda function
zip -j function.zip build/bootstrap

# Deploy with AWS CLI
aws lambda update-function-code \
  --function-name brain2-backend \
  --zip-file fileb://function.zip

# Deploy with SAM
sam build
sam deploy --guided

# Deploy to specific environment
sam deploy --config-env production

# Deploy with parameters
sam deploy --parameter-overrides \
  TableName=brain2-prod \
  Environment=production
```

### Lambda Management
```bash
# List functions
aws lambda list-functions

# Get function configuration
aws lambda get-function-configuration \
  --function-name brain2-backend

# Update environment variables
aws lambda update-function-configuration \
  --function-name brain2-backend \
  --environment Variables={KEY1=value1,KEY2=value2}

# Invoke function
aws lambda invoke \
  --function-name brain2-backend \
  --payload '{"test": "data"}' \
  response.json

# View logs
aws logs tail /aws/lambda/brain2-backend --follow

# Get function metrics
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Duration \
  --dimensions Name=FunctionName,Value=brain2-backend \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-02T00:00:00Z \
  --period 3600 \
  --statistics Average
```

---

## Docker Commands

### Build Docker Image
```bash
# Build application image
docker build -t brain2-backend:latest .

# Build with build args
docker build \
  --build-arg GO_VERSION=1.22 \
  -t brain2-backend:latest .

# Multi-stage build for smaller image
docker build -f Dockerfile.multistage -t brain2-backend:latest .
```

### Run with Docker
```bash
# Run container
docker run -p 8080:8080 \
  -e TABLE_NAME=brain2-dev \
  -e AWS_REGION=us-east-1 \
  brain2-backend:latest

# Run with AWS credentials
docker run -p 8080:8080 \
  -v ~/.aws:/root/.aws:ro \
  brain2-backend:latest

# Run with docker-compose
docker-compose up

# Run in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

---

## Database Commands

### DynamoDB Operations
```bash
# Create table
aws dynamodb create-table \
  --table-name brain2-prod \
  --attribute-definitions file://table-schema.json \
  --key-schema file://key-schema.json \
  --billing-mode PAY_PER_REQUEST

# Describe table
aws dynamodb describe-table --table-name brain2-prod

# Create Global Secondary Index
aws dynamodb update-table \
  --table-name brain2-prod \
  --global-secondary-index-updates file://gsi-update.json

# Backup table
aws dynamodb create-backup \
  --table-name brain2-prod \
  --backup-name brain2-backup-$(date +%Y%m%d)

# List backups
aws dynamodb list-backups --table-name brain2-prod

# Enable point-in-time recovery
aws dynamodb update-continuous-backups \
  --table-name brain2-prod \
  --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true
```

### Data Migration
```bash
# Export table data
aws dynamodb export-table-to-point-in-time \
  --table-arn arn:aws:dynamodb:region:account:table/brain2-prod \
  --s3-bucket export-bucket \
  --s3-prefix exports/

# Import data
aws dynamodb import-table \
  --s3-bucket-source S3Bucket=import-bucket,S3KeyPrefix=imports/ \
  --input-format CSV \
  --table-creation-parameters file://table-params.json
```

---

## Code Quality

### Linting
```bash
# Run golangci-lint
golangci-lint run

# Run with specific linters
golangci-lint run --enable=gofmt,govet,errcheck

# Fix issues automatically
golangci-lint run --fix

# Run on specific directory
golangci-lint run ./internal/...

# Custom config
golangci-lint run -c .golangci.yml
```

### Formatting
```bash
# Format all Go files
go fmt ./...

# Format with gofmt
gofmt -w -s .

# Format imports
goimports -w .

# Check formatting without changes
gofmt -l .
```

### Static Analysis
```bash
# Run go vet
go vet ./...

# Run staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Run gosec (security)
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...

# Check for inefficient assignments
ineffassign ./...

# Check for unused code
unused ./...
```

---

## Debugging Commands

### Profiling
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Block profiling
go test -blockprofile=block.prof -bench=.

# Trace execution
go test -trace=trace.out
go tool trace trace.out
```

### Debugging
```bash
# Debug with Delve
dlv debug cmd/main/main.go

# Attach to running process
dlv attach <pid>

# Debug test
dlv test ./internal/service

# Remote debugging
dlv debug --headless --listen=:2345 --api-version=2

# Set breakpoint in Delve
(dlv) break main.main
(dlv) continue
(dlv) print variableName
(dlv) stack
```

### Logs and Monitoring
```bash
# View CloudWatch logs
aws logs tail /aws/lambda/brain2-backend --follow

# Filter logs
aws logs filter-log-events \
  --log-group-name /aws/lambda/brain2-backend \
  --filter-pattern "ERROR"

# Get log insights
aws logs start-query \
  --log-group-name /aws/lambda/brain2-backend \
  --start-time $(date -d '1 hour ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, @message | filter @message like /ERROR/'
```

---

## Environment Variables Reference

### Core Configuration
```bash
# Environment
ENVIRONMENT=development|staging|production
ENV=development|staging|production

# AWS Configuration
AWS_REGION=us-east-1
AWS_PROFILE=default
AWS_ENDPOINT=http://localhost:4566  # LocalStack
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret
AWS_SESSION_TOKEN=token  # For temporary credentials

# Database
TABLE_NAME=brain2-dev
INDEX_NAME=KeywordIndex
DB_MAX_RETRIES=3
DB_RETRY_BASE_DELAY=100ms
DB_CONNECTION_POOL=10
DB_TIMEOUT=10s
DB_READ_CAPACITY=5
DB_WRITE_CAPACITY=5
DB_ENABLE_BACKUPS=false
DB_ENABLE_STREAMS=false
```

### Server Configuration
```bash
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=60s
SERVER_SHUTDOWN_TIMEOUT=10s
SERVER_MAX_REQUEST_SIZE=10485760  # 10MB
SERVER_REQUEST_TIMEOUT=30s
SERVER_ENABLE_HTTPS=false
SERVER_CERT_FILE=/path/to/cert.pem
SERVER_KEY_FILE=/path/to/key.pem
```

### Domain Configuration
```bash
DOMAIN_SIMILARITY_THRESHOLD=0.3
DOMAIN_MAX_CONNECTIONS=10
DOMAIN_MAX_CONTENT_LENGTH=10000
DOMAIN_MIN_KEYWORD_LENGTH=3
DOMAIN_RECENCY_WEIGHT=0.2
DOMAIN_DIVERSITY_THRESHOLD=0.5
DOMAIN_MAX_TAGS=10
DOMAIN_MAX_NODES_PER_USER=10000
```

### Feature Flags
```bash
# Core Features
ENABLE_CACHING=true
ENABLE_AUTO_CONNECT=true
ENABLE_AI_PROCESSING=false
ENABLE_METRICS=true
ENABLE_TRACING=false
ENABLE_EVENT_BUS=false

# Infrastructure Features
ENABLE_RETRIES=true
ENABLE_CIRCUIT_BREAKER=true
ENABLE_RATE_LIMITING=false
ENABLE_COMPRESSION=true

# Debug Features
ENABLE_DEBUG_ENDPOINTS=false
ENABLE_PROFILING=false
ENABLE_LOGGING=true
VERBOSE_LOGGING=false

# Experimental Features
ENABLE_GRAPHQL=false
ENABLE_WEBSOCKETS=false
ENABLE_BATCH_API=false
```

### Infrastructure Settings
```bash
# Retry Configuration
RETRY_MAX_RETRIES=3
RETRY_INITIAL_DELAY=100ms
RETRY_MAX_DELAY=5s
RETRY_BACKOFF_FACTOR=2.0
RETRY_JITTER_FACTOR=0.1
RETRY_ON_TIMEOUT=true
RETRY_ON_5XX=true

# Circuit Breaker
CB_FAILURE_THRESHOLD=0.5
CB_SUCCESS_THRESHOLD=0.8
CB_MINIMUM_REQUESTS=10
CB_WINDOW_SIZE=10s
CB_OPEN_DURATION=30s
CB_HALF_OPEN_REQUESTS=3

# Other
IDEMPOTENCY_TTL=24h
HEALTH_CHECK_INTERVAL=30s
GRACEFUL_SHUTDOWN_DELAY=5s
```

### Caching Configuration
```bash
CACHE_PROVIDER=memory|redis|memcached
CACHE_MAX_ITEMS=1000
CACHE_TTL=5m
CACHE_QUERY_TTL=1m

# Redis Settings
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10
```

### Logging Configuration
```bash
LOG_LEVEL=debug|info|warn|error|fatal
LOG_FORMAT=json|console
LOG_OUTPUT=stdout|stderr|file
LOG_FILE_PATH=/var/log/brain2.log
LOG_MAX_SIZE=100  # MB
LOG_MAX_AGE=30    # Days
LOG_MAX_BACKUPS=10
LOG_COMPRESS=true
```

### Security Configuration
```bash
JWT_SECRET=your-secret-key-min-32-chars
JWT_EXPIRY=24h
API_KEY_HEADER=X-API-Key
ENABLE_AUTH=true
ALLOWED_ORIGINS=*
TRUSTED_PROXIES=
SECURE_HEADERS=true
ENABLE_CSRF=false
CSRF_TOKEN_LENGTH=32
```

### Rate Limiting
```bash
RATE_LIMIT_ENABLED=true
RATE_LIMIT_RPM=100  # Requests per minute
RATE_LIMIT_BURST=10
RATE_LIMIT_CLEANUP=1m
RATE_LIMIT_BY_IP=true
RATE_LIMIT_BY_USER=false
RATE_LIMIT_BY_API_KEY=false
```

### CORS Configuration
```bash
CORS_ENABLED=true
CORS_ALLOWED_ORIGINS=*
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=*
CORS_EXPOSED_HEADERS=
CORS_ALLOW_CREDENTIALS=false
CORS_MAX_AGE=86400
```

### Monitoring
```bash
# Metrics
METRICS_PROVIDER=prometheus|datadog|cloudwatch|statsd
METRICS_INTERVAL=10s
METRICS_NAMESPACE=brain2
PROMETHEUS_PORT=9090
PROMETHEUS_PATH=/metrics
DATADOG_API_KEY=
DATADOG_HOST=localhost
DATADOG_PORT=8125

# Tracing
TRACING_ENABLED=false
TRACING_PROVIDER=jaeger|zipkin|xray|datadog
TRACING_SERVICE_NAME=brain2
TRACING_SAMPLE_RATE=0.1
TRACING_ENDPOINT=
TRACING_AGENT_HOST=localhost
TRACING_AGENT_PORT=6831

# Events
EVENTS_PROVIDER=eventbridge|kafka|rabbitmq|sns
EVENT_BUS_NAME=default
EVENT_TOPIC_PREFIX=brain2
EVENT_RETRY_ATTEMPTS=3
EVENT_BATCH_SIZE=10
```

---

## Quick Development Workflow

```bash
# 1. Setup environment
export ENVIRONMENT=development
export TABLE_NAME=brain2-dev
export AWS_REGION=us-east-1

# 2. Install dependencies
go mod tidy

# 3. Generate Wire code
cd internal/di && wire && cd ../..

# 4. Run tests
go test ./...

# 5. Build locally
go build -o bin/main cmd/main/main.go

# 6. Run locally
./bin/main

# Or use the build script for Lambda
./build.sh
```

---

## Production Deployment Workflow

```bash
# 1. Run tests
go test -race ./...

# 2. Build for production
ENVIRONMENT=production ./build.sh

# 3. Package Lambda
cd build && zip -r ../function.zip . && cd ..

# 4. Deploy
aws lambda update-function-code \
  --function-name brain2-backend-prod \
  --zip-file fileb://function.zip

# 5. Update configuration
aws lambda update-function-configuration \
  --function-name brain2-backend-prod \
  --environment Variables={ENVIRONMENT=production}

# 6. Monitor logs
aws logs tail /aws/lambda/brain2-backend-prod --follow
```

---

## Troubleshooting

### Common Issues and Solutions

```bash
# Wire generation fails
rm internal/di/wire_gen.go
cd internal/di && wire

# Module dependencies issues
go clean -modcache
go mod download

# Build fails with permission denied
chmod +x build.sh
chmod +x test_build.sh

# Lambda timeout issues
aws lambda update-function-configuration \
  --function-name brain2-backend \
  --timeout 30

# DynamoDB throttling
aws dynamodb update-table \
  --table-name brain2-prod \
  --provisioned-throughput ReadCapacityUnits=10,WriteCapacityUnits=10

# Out of memory in Lambda
aws lambda update-function-configuration \
  --function-name brain2-backend \
  --memory-size 512
```

---

## Useful Aliases

Add these to your `.bashrc` or `.zshrc`:

```bash
# Brain2 Backend Aliases
alias b2-build="cd ~/b2/backend && ./build.sh"
alias b2-test="cd ~/b2/backend && go test ./..."
alias b2-run="cd ~/b2/backend && go run cmd/main/main.go"
alias b2-wire="cd ~/b2/backend/internal/di && wire && cd -"
alias b2-logs="aws logs tail /aws/lambda/brain2-backend --follow"
alias b2-deploy="cd ~/b2/backend && ./build.sh && sam deploy"
alias b2-local="cd ~/b2/backend && sam local start-api"
alias b2-fmt="cd ~/b2/backend && go fmt ./... && goimports -w ."
alias b2-lint="cd ~/b2/backend && golangci-lint run"
alias b2-clean="cd ~/b2/backend && rm -rf build/ && go clean -modcache"
```

---

## Additional Resources

- [Go Documentation](https://golang.org/doc/)
- [AWS Lambda Go](https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html)
- [Wire Documentation](https://github.com/google/wire)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [SAM CLI Documentation](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-command-reference.html)