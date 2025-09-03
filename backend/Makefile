# Brain2 Backend Makefile
# Comprehensive build and test automation

.PHONY: all build test clean help

# Variables
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOGET := $(GO) get
GOMOD := $(GO) mod
WIRE := wire

# Directories
SRC_DIR := ./...
TEST_DIR := ./tests/...
INTERNAL_DIR := ./internal/...
CMD_DIR := ./cmd/...

# Build variables
LAMBDA_FUNCS := main
BUILD_DIR := bin
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Docker
DOCKER_COMPOSE := docker-compose
TEST_COMPOSE_FILE := tests/docker-compose.yml

# Default target
all: clean build test

# Help target
help:
	@echo "Available targets:"
	@echo "  all              - Clean, build, and test everything"
	@echo "  build            - Build all Lambda functions"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-contracts   - Run contract tests"
	@echo "  test-bdd         - Run BDD tests"
	@echo "  test-bench       - Run performance benchmarks"
	@echo "  test-coverage    - Generate test coverage report"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Download dependencies"
	@echo "  wire             - Generate dependency injection code"
	@echo "  mocks            - Generate mocks"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linters"
	@echo "  vet              - Run go vet"
	@echo "  docker-up        - Start test dependencies"
	@echo "  docker-down      - Stop test dependencies"

# Build targets
build: wire
	@echo "Building Lambda functions..."
	@for func in $(LAMBDA_FUNCS); do \
		echo "Building $$func..."; \
		GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$$func ./cmd/$$func || exit 1; \
	done
	@echo "Build complete!"

build-local:
	@echo "Building for local development..."
	@$(GOBUILD) -o $(BUILD_DIR)/server ./cmd/server

# Test targets
test: test-unit test-integration test-contracts test-bdd
	@echo "All tests completed!"

test-unit:
	@echo "Running unit tests..."
	@$(GOTEST) $(GOFLAGS) -tags=unit -short ./...

test-integration: docker-up
	@echo "Running integration tests..."
	@$(GOTEST) $(GOFLAGS) -tags=integration ./...
	@$(MAKE) docker-down

test-contracts:
	@echo "Running contract tests..."
	@$(GOTEST) $(GOFLAGS) -tags=contracts ./...

test-bdd:
	@echo "Running BDD tests..."
	@$(GOTEST) $(GOFLAGS) -tags=bdd ./...

test-bench:
	@echo "Running performance benchmarks..."
	@$(GOTEST) -tags=bench -bench=. -benchmem -benchtime=10s ./...

test-coverage:
	@echo "Generating test coverage..."
	@$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(SRC_DIR)
	@$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@$(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print "Total Coverage: " $$3}'

test-race:
	@echo "Running tests with race detector..."
	@$(GOTEST) -race $(SRC_DIR)

test-all: test test-race test-bench test-coverage
	@echo "All test suites completed!"

# Specific test targets
test-domain:
	@echo "Testing domain layer..."
	@$(GOTEST) $(GOFLAGS) ./internal/core/domain/...

test-application:
	@echo "Testing application layer..."
	@$(GOTEST) $(GOFLAGS) ./internal/core/application/...

test-infrastructure:
	@echo "Testing infrastructure layer..."
	@$(GOTEST) $(GOFLAGS) ./internal/infrastructure/...

test-sagas:
	@echo "Testing sagas..."
	@$(GOTEST) $(GOFLAGS) ./internal/core/application/sagas/...
	@$(GOTEST) $(GOFLAGS) ./tests/features/*saga*

# Mock generation
mocks:
	@echo "Generating mocks..."
	@go install github.com/vektra/mockery/v2@latest
	@mockery --all --dir=internal/core/application/ports --output=tests/mocks --outpkg=mocks
	@echo "Mocks generated!"

# Dependency management
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

deps-upgrade:
	@echo "Upgrading dependencies..."
	@$(GOGET) -u ./...
	@$(GOMOD) tidy

# Code generation
wire:
	@echo "Generating Wire dependencies..."
	@cd internal/di && $(WIRE)

generate:
	@echo "Running go generate..."
	@$(GO) generate $(SRC_DIR)

swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g ./cmd/server/main.go -o ./pkg/api

# Code quality
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .

lint:
	@echo "Running linters..."
	@golangci-lint run $(SRC_DIR)

vet:
	@echo "Running go vet..."
	@$(GO) vet $(SRC_DIR)

security:
	@echo "Running security scan..."
	@gosec -fmt json -out security-report.json $(SRC_DIR)

quality: fmt vet lint security
	@echo "Code quality checks completed!"

# Docker targets
docker-up:
	@echo "Starting test dependencies..."
	@if [ -f $(TEST_COMPOSE_FILE) ]; then \
		$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) up -d; \
		echo "Waiting for services to be ready..."; \
		sleep 5; \
	else \
		echo "No docker-compose file found, skipping..."; \
	fi

docker-down:
	@echo "Stopping test dependencies..."
	@if [ -f $(TEST_COMPOSE_FILE) ]; then \
		$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) down; \
	fi

docker-logs:
	@$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) logs -f

# Clean targets
clean:
	@echo "Cleaning build artifacts..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@rm -f security-report.json
	@echo "Clean complete!"

clean-cache:
	@echo "Cleaning Go cache..."
	@$(GOCLEAN) -cache

clean-all: clean clean-cache
	@echo "Deep clean complete!"

# Development helpers
dev: deps wire fmt build-local
	@echo "Development environment ready!"

watch:
	@echo "Watching for changes..."
	@air -c .air.toml

run: build-local
	@echo "Running local server..."
	@./$(BUILD_DIR)/server

# CI/CD targets
ci: deps wire test-unit test-coverage lint vet
	@echo "CI checks passed!"

cd: ci build
	@echo "Ready for deployment!"

# Installation targets
install-tools:
	@echo "Installing development tools..."
	@go install github.com/google/wire/cmd/wire@latest
	@go install github.com/vektra/mockery/v2@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/cosmtrek/air@latest
	@echo "Tools installed!"

# Reporting
report: test-coverage
	@echo "=== Test Coverage Report ==="
	@$(GO) tool cover -func=$(COVERAGE_FILE) | tail -5
	@echo ""
	@echo "=== Code Statistics ==="
	@echo "Lines of code:"
	@find . -name "*.go" -not -path "./vendor/*" -not -path "./tests/*" | xargs wc -l | tail -1
	@echo ""
	@echo "Number of packages:"
	@go list ./... | wc -l
	@echo ""
	@echo "Number of tests:"
	@grep -r "func Test" --include="*.go" . | wc -l

# Phony targets
.PHONY: build build-local test test-unit test-integration test-contracts test-bdd test-bench \
        test-coverage test-race test-all test-domain test-application test-infrastructure \
        test-sagas mocks deps deps-upgrade wire generate swagger fmt lint vet security \
        quality docker-up docker-down docker-logs clean clean-cache clean-all dev watch \
        run ci cd install-tools report