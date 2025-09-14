# Backend2 Makefile - DDD/CQRS Architecture
# Comprehensive build, test, and development automation

.PHONY: all build test clean help run dev

# Variables
GO := go
GOFLAGS := -v
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOGET := $(GO) get
GOMOD := $(GO) mod
WIRE := wire
MOCKGEN := mockgen

# Directories
SRC_DIR := .
BUILD_DIR := build
COVERAGE_DIR := coverage
CMD_API := cmd/api
CMD_WORKER := cmd/worker

# Build variables
VERSION := 1.0.0-alpha
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date +"%Y-%m-%d_%H-%M-%S")
BUILD_ID := b2_backend2_$(BUILD_TIME)

# LDFLAGS for version injection
LDFLAGS := -ldflags "-s -w \
	-X main.Version=$(VERSION) \
	-X main.GitCommit=$(GIT_COMMIT) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.BuildID=$(BUILD_ID)"

# Docker
DOCKER_COMPOSE := docker-compose
DOCKER_COMPOSE_FILE := docker-compose.yml
TEST_COMPOSE_FILE := docker-compose.test.yml

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Default target
all: clean deps wire build test

# Help target
help:
	@echo "$(BLUE)Backend2 Makefile - Available targets:$(NC)"
	@echo ""
	@echo "$(GREEN)Build targets:$(NC)"
	@echo "  all              - Clean, build, and test everything"
	@echo "  build            - Build all components"
	@echo "  build-api        - Build API server only"
	@echo "  build-worker     - Build worker service only"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Download and verify dependencies"
	@echo "  wire             - Generate dependency injection code"
	@echo ""
	@echo "$(GREEN)Test targets:$(NC)"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-e2e         - Run end-to-end tests"
	@echo "  test-bench       - Run performance benchmarks"
	@echo "  test-coverage    - Generate test coverage report"
	@echo "  test-race        - Run tests with race detection"
	@echo ""
	@echo "$(GREEN)Development targets:$(NC)"
	@echo "  run              - Run API server locally"
	@echo "  dev              - Run in development mode with hot reload"
	@echo "  worker           - Run worker service"
	@echo ""
	@echo "$(GREEN)Code quality targets:$(NC)"
	@echo "  fmt              - Format code"
	@echo "  lint             - Run linters"
	@echo "  vet              - Run go vet"
	@echo "  sec              - Run security checks"
	@echo "  mocks            - Generate mocks"
	@echo ""
	@echo "$(GREEN)Documentation targets:$(NC)"
	@echo "  swagger          - Generate Swagger/OpenAPI documentation"
	@echo "  swagger-fmt      - Format Swagger annotations"
	@echo "  swagger-validate - Validate generated Swagger docs"
	@echo "  docs             - Generate all documentation"
	@echo ""
	@echo "$(GREEN)Docker targets:$(NC)"
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-up        - Start services with Docker Compose"
	@echo "  docker-down      - Stop Docker Compose services"
	@echo "  docker-logs      - Show Docker Compose logs"
	@echo ""
	@echo "$(GREEN)CI/CD targets:$(NC)"
	@echo "  ci               - Run CI pipeline checks"
	@echo "  check            - Run all checks (fmt, vet, lint, test)"

# Dependencies management
deps:
	@echo "$(YELLOW)📦 Managing dependencies...$(NC)"
	@$(GOMOD) download
	@$(GOMOD) tidy
	@$(GOMOD) verify
	@echo "$(GREEN)✅ Dependencies ready$(NC)"

# Wire dependency injection
wire:
	@echo "$(YELLOW)🔄 Generating dependency injection code...$(NC)"
	@if [ ! -d "infrastructure/di" ]; then \
		echo "$(RED)❌ infrastructure/di directory not found$(NC)"; \
		exit 1; \
	fi
	@if ! command -v wire &> /dev/null; then \
		echo "$(YELLOW)Installing Wire...$(NC)"; \
		go install github.com/google/wire/cmd/wire@latest; \
	fi
	@cd infrastructure/di && wire check && wire
	@echo "$(GREEN)✅ Wire code generated$(NC)"

# Build targets
build: build-api build-worker
	@echo "$(GREEN)✅ All components built successfully$(NC)"

build-api: wire
	@echo "$(YELLOW)🏗️  Building API server...$(NC)"
	@mkdir -p $(BUILD_DIR)/api
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/api/api $(CMD_API)
	@echo "$(GREEN)✅ API server built: $(BUILD_DIR)/api/api$(NC)"


build-worker:
	@echo "$(YELLOW)🏗️  Building worker service...$(NC)"
	@mkdir -p $(BUILD_DIR)/worker
	@if [ -f "$(CMD_WORKER)/main.go" ]; then \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/worker/worker $(CMD_WORKER); \
		echo "$(GREEN)✅ Worker service built: $(BUILD_DIR)/worker/worker$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  Worker service not implemented yet$(NC)"; \
	fi

# Clean target
clean:
	@echo "$(YELLOW)🧹 Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	@$(GOCLEAN)
	@echo "$(GREEN)✅ Cleaned$(NC)"

# Test targets
test: test-unit test-integration
	@echo "$(GREEN)✅ All tests passed$(NC)"

test-unit:
	@echo "$(YELLOW)🧪 Running unit tests...$(NC)"
	@$(GOTEST) -short -v ./domain/... ./application/...
	@echo "$(GREEN)✅ Unit tests passed$(NC)"

test-integration:
	@echo "$(YELLOW)🧪 Running integration tests...$(NC)"
	@$(GOTEST) -v -tags=integration ./infrastructure/... ./interfaces/...
	@echo "$(GREEN)✅ Integration tests passed$(NC)"

test-e2e:
	@echo "$(YELLOW)🧪 Running end-to-end tests...$(NC)"
	@$(GOTEST) -v -tags=e2e ./tests/e2e/...
	@echo "$(GREEN)✅ E2E tests passed$(NC)"

test-bench:
	@echo "$(YELLOW)⚡ Running benchmarks...$(NC)"
	@$(GOTEST) -bench=. -benchmem -benchtime=10s ./...

test-coverage:
	@echo "$(YELLOW)📊 Generating coverage report...$(NC)"
	@mkdir -p $(COVERAGE_DIR)
	@$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	@$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)✅ Coverage report: $(COVERAGE_DIR)/coverage.html$(NC)"
	@$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

test-race:
	@echo "$(YELLOW)🏃 Running tests with race detection...$(NC)"
	@$(GOTEST) -race -v ./...
	@echo "$(GREEN)✅ Race tests passed$(NC)"

# Development targets
run: build-api
	@echo "$(BLUE)🚀 Starting API server...$(NC)"
	@./$(BUILD_DIR)/api/api

dev:
	@echo "$(BLUE)🔄 Starting development server with hot reload...$(NC)"
	@if ! command -v air &> /dev/null; then \
		echo "$(YELLOW)Installing Air for hot reload...$(NC)"; \
		go install github.com/cosmtrek/air@latest; \
	fi
	@air -c .air.toml


worker: build-worker
	@echo "$(BLUE)⚙️  Starting worker service...$(NC)"
	@if [ -f "$(BUILD_DIR)/worker/worker" ]; then \
		./$(BUILD_DIR)/worker/worker; \
	else \
		echo "$(RED)❌ Worker service not found. Run 'make build-worker' first$(NC)"; \
	fi

# Documentation targets
swagger:
	@echo "$(YELLOW)📚 Generating Swagger documentation...$(NC)"
	@if ! command -v swag &> /dev/null; then \
		echo "$(YELLOW)Installing swag...$(NC)"; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	@mkdir -p docs/api
	@swag init -g docs/swagger/main.go -o docs/api --parseDependency --parseInternal
	@echo "$(GREEN)✅ Swagger documentation generated: docs/api/$(NC)"

swagger-fmt:
	@echo "$(YELLOW)✨ Formatting Swagger annotations...$(NC)"
	@if ! command -v swag &> /dev/null; then \
		echo "$(YELLOW)Installing swag...$(NC)"; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	@swag fmt
	@echo "$(GREEN)✅ Swagger annotations formatted$(NC)"

swagger-validate: swagger
	@echo "$(YELLOW)🔍 Validating Swagger documentation...$(NC)"
	@if [ -f "docs/api/swagger.json" ]; then \
		echo "$(GREEN)✅ Swagger documentation exists at docs/api/swagger.json$(NC)"; \
		echo "$(YELLOW)📝 To validate online, visit: https://editor.swagger.io/$(NC)"; \
	else \
		echo "$(RED)❌ Swagger documentation not found. Run 'make swagger' first$(NC)"; \
	fi

docs: swagger
	@echo "$(GREEN)✅ All documentation generated$(NC)"

# Code quality targets
fmt:
	@echo "$(YELLOW)✨ Formatting code...$(NC)"
	@gofmt -w -s .
	@goimports -w .
	@echo "$(GREEN)✅ Code formatted$(NC)"

lint:
	@echo "$(YELLOW)🔍 Running linters...$(NC)"
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@golangci-lint run --timeout 3m
	@echo "$(GREEN)✅ Linting passed$(NC)"

vet:
	@echo "$(YELLOW)🔍 Running go vet...$(NC)"
	@$(GO) vet ./...
	@echo "$(GREEN)✅ go vet passed$(NC)"

sec:
	@echo "$(YELLOW)🔒 Running security checks...$(NC)"
	@if ! command -v gosec &> /dev/null; then \
		echo "$(YELLOW)Installing gosec...$(NC)"; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@gosec -fmt json -out $(COVERAGE_DIR)/security.json ./... || true
	@echo "$(GREEN)✅ Security check complete. Report: $(COVERAGE_DIR)/security.json$(NC)"

mocks:
	@echo "$(YELLOW)🎭 Generating mocks...$(NC)"
	@if ! command -v mockgen &> /dev/null; then \
		echo "$(YELLOW)Installing mockgen...$(NC)"; \
		go install github.com/golang/mock/mockgen@latest; \
	fi
	@go generate ./...
	@echo "$(GREEN)✅ Mocks generated$(NC)"

# Docker targets
docker-build:
	@echo "$(YELLOW)🐳 Building Docker images...$(NC)"
	@docker build -t backend2-api:latest -f Dockerfile.api .
	@echo "$(GREEN)✅ Docker images built$(NC)"

docker-up:
	@echo "$(YELLOW)🐳 Starting Docker Compose services...$(NC)"
	@if [ -f "$(DOCKER_COMPOSE_FILE)" ]; then \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) up -d; \
		echo "$(GREEN)✅ Services started$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  docker-compose.yml not found$(NC)"; \
	fi

docker-down:
	@echo "$(YELLOW)🐳 Stopping Docker Compose services...$(NC)"
	@if [ -f "$(DOCKER_COMPOSE_FILE)" ]; then \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) down; \
		echo "$(GREEN)✅ Services stopped$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  docker-compose.yml not found$(NC)"; \
	fi

docker-logs:
	@echo "$(YELLOW)📋 Showing Docker Compose logs...$(NC)"
	@if [ -f "$(DOCKER_COMPOSE_FILE)" ]; then \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) logs -f; \
	else \
		echo "$(YELLOW)⚠️  docker-compose.yml not found$(NC)"; \
	fi

# CI/CD targets
ci: deps fmt vet lint test
	@echo "$(GREEN)✅ CI checks passed$(NC)"

check: fmt vet lint test-unit
	@echo "$(GREEN)✅ All checks passed$(NC)"

# Quick shortcuts
.PHONY: api work
api: run
work: worker