# Achieving 10/10 Code Organization Excellence
## Comprehensive Implementation Roadmap

### Executive Summary

**Current State**: 8.5/10 - Strong foundation with excellent architecture  
**Target State**: 10/10 - Industry-leading code organization and engineering practices  
**Gap**: 1.5 points requiring systematic improvements across testing, architecture, documentation, and tooling

This document provides a detailed, step-by-step roadmap to transform the B2 codebase into an exemplary model of modern software engineering practices.

---

## Current State Analysis

### Strengths (8.5/10)
✅ **Excellent Architecture**: Clean architecture with proper domain separation  
✅ **Modern Tech Stack**: Go + TypeScript + AWS serverless  
✅ **Feature Organization**: Well-structured feature-based frontend architecture  
✅ **Type Safety**: Strong TypeScript integration with generated API types  
✅ **Infrastructure as Code**: Proper CDK implementation  

### Critical Gaps (1.5 points to address)
❌ **Testing Infrastructure**: Minimal test coverage and organization  
❌ **Advanced Patterns**: Missing enterprise-grade architectural patterns  
❌ **Documentation**: Incomplete API docs and developer guides  
❌ **Code Quality Tools**: Missing linting, formatting, and analysis setup  

---

## Gap Analysis & Point Allocation

| Category | Current Score | Target Score | Points to Gain | Priority |
|----------|--------------|--------------|----------------|----------|
| **Testing Infrastructure** | 2/10 | 10/10 | **2.0 points** | Critical |
| **Architectural Patterns** | 7/10 | 10/10 | **0.8 points** | High |
| **Documentation & DX** | 6/10 | 10/10 | **0.5 points** | Medium |
| **Code Quality Tools** | 6/10 | 10/10 | **0.2 points** | Low |
| **Total Improvement Needed** | | | **1.5 points** | |

---

# Phase 1: Testing Infrastructure Excellence
## Target: +2.0 points (Critical Priority)

### Overview
Establish comprehensive testing infrastructure across all layers with proper organization, coverage reporting, and CI integration.

### 1.1 Frontend Testing Setup

#### Dependencies to Install
```bash
cd /home/wsl/b2/frontend
npm install -D @testing-library/react @testing-library/jest-dom @testing-library/user-event
npm install -D vitest @vitest/ui @vitest/coverage-v8
npm install -D jsdom happy-dom
npm install -D playwright @playwright/test
```

#### File Structure to Create
```
frontend/
├── tests/
│   ├── __fixtures__/          # Test data and mocks
│   ├── __mocks__/             # Module mocks
│   ├── e2e/                   # End-to-end tests
│   │   ├── auth.spec.ts
│   │   ├── dashboard.spec.ts
│   │   └── graph.spec.ts
│   ├── integration/           # Integration tests
│   │   ├── api-client.test.ts
│   │   └── websocket.test.ts
│   └── setup/                 # Test configuration
│       ├── setup-tests.ts
│       └── test-utils.tsx
├── src/
│   ├── components/
│   │   └── __tests__/         # Component unit tests
│   ├── features/
│   │   ├── auth/
│   │   │   └── __tests__/
│   │   ├── memories/
│   │   │   └── __tests__/
│   │   └── categories/
│   │       └── __tests__/
│   └── services/
│       └── __tests__/         # Service unit tests
├── vitest.config.ts
├── playwright.config.ts
└── coverage/                  # Coverage reports
```

#### Configuration Files

**vitest.config.ts**
```typescript
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./tests/setup/setup-tests.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      exclude: [
        'node_modules/',
        'tests/',
        '**/*.d.ts',
        'src/types/generated/',
        'dist/',
      ],
      thresholds: {
        global: {
          branches: 80,
          functions: 80,
          lines: 80,
          statements: 80,
        },
      },
    },
  },
  resolve: {
    alias: {
      '@app': resolve(__dirname, './src/app'),
      '@common': resolve(__dirname, './src/common'),
      '@features': resolve(__dirname, './src/features'),
      '@services': resolve(__dirname, './src/services'),
      '@types': resolve(__dirname, './src/types'),
    },
  },
});
```

**playwright.config.ts**
```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],
  webServer: {
    command: 'npm run preview',
    port: 4173,
  },
});
```

**tests/setup/setup-tests.ts**
```typescript
import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock environment variables
vi.mock('../src/vite-env.d.ts', () => ({
  VITE_SUPABASE_URL: 'http://localhost:54321',
  VITE_SUPABASE_ANON_KEY: 'test-key',
  VITE_API_BASE_URL: 'http://localhost:3000',
  VITE_WEBSOCKET_URL: 'ws://localhost:3001',
}));

// Mock WebSocket
global.WebSocket = vi.fn();

// Mock ResizeObserver
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));
```

**tests/setup/test-utils.tsx**
```typescript
import React, { ReactElement } from 'react';
import { render, RenderOptions } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';

const AllTheProviders = ({ children }: { children: React.ReactNode }) => {
  return (
    <BrowserRouter>
      {children}
    </BrowserRouter>
  );
};

const customRender = (
  ui: ReactElement,
  options?: Omit<RenderOptions, 'wrapper'>,
) => render(ui, { wrapper: AllTheProviders, ...options });

export * from '@testing-library/react';
export { customRender as render };
```

#### Package.json Scripts Update
```json
{
  "scripts": {
    "test": "vitest",
    "test:ui": "vitest --ui",
    "test:run": "vitest run",
    "test:coverage": "vitest run --coverage",
    "test:e2e": "playwright test",
    "test:e2e:ui": "playwright test --ui"
  }
}
```

### 1.2 Backend Testing Setup

#### Dependencies to Install
```bash
cd /home/wsl/b2/backend
go get github.com/stretchr/testify/suite
go get github.com/stretchr/testify/mock
go get github.com/DATA-DOG/go-sqlmock
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/dynamodb
```

#### File Structure to Create
```
backend/
├── tests/
│   ├── fixtures/              # Test data
│   ├── integration/           # Integration tests
│   │   ├── api_test.go
│   │   └── dynamodb_test.go
│   ├── mocks/                 # Generated mocks
│   │   ├── repository_mock.go
│   │   └── service_mock.go
│   └── testutils/            # Test utilities
│       ├── test_server.go
│       └── test_container.go
├── internal/
│   ├── domain/
│   │   └── *_test.go         # Domain unit tests
│   ├── repository/
│   │   └── *_test.go         # Repository tests
│   ├── service/
│   │   └── *_test.go         # Service tests
│   └── handlers/
│       └── *_test.go         # Handler tests
└── coverage.out
```

#### Test Configuration Files

**Makefile**
```makefile
.PHONY: test test-unit test-integration test-coverage test-race

# Unit tests
test-unit:
	go test ./internal/... -v -short

# Integration tests
test-integration:
	go test ./tests/integration/... -v -tags=integration

# All tests
test:
	go test ./... -v

# Test with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

# Race condition testing
test-race:
	go test ./... -race -v

# Generate mocks
mocks:
	mockgen -source=internal/repository/repository.go -destination=tests/mocks/repository_mock.go
	mockgen -source=internal/service/memory/service.go -destination=tests/mocks/memory_service_mock.go

# Clean test artifacts
clean-test:
	rm -f coverage.out coverage.html
	go clean -testcache
```

**tests/testutils/test_server.go**
```go
package testutils

import (
	"context"
	"net/http/httptest"
	"testing"

	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/config"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/suite"
)

type APITestSuite struct {
	suite.Suite
	server     *httptest.Server
	repo       *repository.MockRepository
	memService *memory.Service
	cfg        *config.Config
}

func (s *APITestSuite) SetupSuite() {
	s.cfg = &config.Config{
		Region:    "us-east-1",
		TableName: "test-table",
	}
	
	// Setup mocks and services
	s.repo = &repository.MockRepository{}
	s.memService = memory.NewService(s.repo)
	
	// Setup test server
	router := chi.NewRouter()
	// Add routes here
	s.server = httptest.NewServer(router)
}

func (s *APITestSuite) TearDownSuite() {
	s.server.Close()
}

func (s *APITestSuite) SetupTest() {
	// Reset mocks before each test
	s.repo.Reset()
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
```

### 1.3 Infrastructure Testing Setup

#### Dependencies to Install
```bash
cd /home/wsl/b2/infra
npm install -D @aws-cdk/assertions aws-cdk-lib@latest
npm install -D jest @types/jest ts-jest
```

#### File Structure to Create
```
infra/
├── test/
│   ├── unit/
│   │   ├── b2-stack.test.ts
│   │   └── constructs.test.ts
│   ├── integration/
│   │   └── deployment.test.ts
│   └── __snapshots__/
├── jest.config.js
└── coverage/
```

#### Configuration Files

**jest.config.js**
```javascript
module.exports = {
  testEnvironment: 'node',
  roots: ['<rootDir>/test'],
  testMatch: ['**/*.test.ts'],
  transform: {
    '^.+\\.tsx?$': 'ts-jest'
  },
  coverageDirectory: 'coverage',
  collectCoverageFrom: [
    'lib/**/*.ts',
    '!lib/**/*.d.ts',
  ],
  coverageThreshold: {
    global: {
      branches: 80,
      functions: 80,
      lines: 80,
      statements: 80,
    },
  },
};
```

**test/unit/b2-stack.test.ts**
```typescript
import { App } from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import { B2Stack } from '../../lib/b2-stack';

describe('B2Stack', () => {
  let template: Template;

  beforeAll(() => {
    const app = new App();
    const stack = new B2Stack(app, 'TestStack', {
      env: { account: '123456789012', region: 'us-east-1' },
    });
    template = Template.fromStack(stack);
  });

  test('creates DynamoDB table', () => {
    template.hasResourceProperties('AWS::DynamoDB::Table', {
      BillingMode: 'PAY_PER_REQUEST',
      AttributeDefinitions: [
        { AttributeName: 'PK', AttributeType: 'S' },
        { AttributeName: 'SK', AttributeType: 'S' },
      ],
    });
  });

  test('creates Lambda functions', () => {
    template.resourceCountIs('AWS::Lambda::Function', 6);
  });

  test('creates API Gateway', () => {
    template.hasResourceProperties('AWS::ApiGatewayV2::Api', {
      ProtocolType: 'HTTP',
    });
  });

  test('snapshot test', () => {
    expect(template.toJSON()).toMatchSnapshot();
  });
});
```

### 1.4 Test Implementation Examples

#### Frontend Component Test Example
**src/features/auth/components/__tests__/AuthSection.test.tsx**
```typescript
import { render, screen, fireEvent, waitFor } from '../../../../tests/setup/test-utils';
import { AuthSection } from '../AuthSection';
import { vi } from 'vitest';

vi.mock('../../../services/authClient', () => ({
  auth: {
    signIn: vi.fn(),
    signUp: vi.fn(),
    signOut: vi.fn(),
    onAuthStateChange: vi.fn(),
  },
}));

describe('AuthSection', () => {
  it('renders login form by default', () => {
    render(<AuthSection />);
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('handles sign in submission', async () => {
    const mockSignIn = vi.fn().mockResolvedValue({ user: { id: '1' } });
    vi.mocked(auth.signIn).mockImplementation(mockSignIn);

    render(<AuthSection />);
    
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: 'test@example.com' },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: 'password' },
    });
    
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith('test@example.com', 'password');
    });
  });
});
```

#### Backend Service Test Example
**internal/service/memory/service_test.go**
```go
package memory

import (
	"context"
	"testing"

	"brain2-backend/internal/domain"
	"brain2-backend/tests/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MemoryServiceTestSuite struct {
	suite.Suite
	service *Service
	repo    *mocks.MockRepository
}

func (s *MemoryServiceTestSuite) SetupTest() {
	s.repo = &mocks.MockRepository{}
	s.service = NewService(s.repo)
}

func (s *MemoryServiceTestSuite) TestCreateMemory() {
	// Arrange
	ctx := context.Background()
	memory := domain.Node{
		ID:      "test-id",
		UserID:  "user-1",
		Content: "Test memory content",
	}

	s.repo.On("CreateNodeAndKeywords", ctx, memory).Return(nil)
	s.repo.On("CreateEdges", ctx, "user-1", "test-id", mock.AnythingOfType("[]string")).Return(nil)

	// Act
	err := s.service.CreateMemory(ctx, memory)

	// Assert
	s.NoError(err)
	s.repo.AssertExpectations(s.T())
}

func TestMemoryServiceSuite(t *testing.T) {
	suite.Run(t, new(MemoryServiceTestSuite))
}
```

---

# Phase 2: Advanced Architectural Patterns
## Target: +0.8 points (High Priority)

### Overview
Implement enterprise-grade architectural patterns including dependency injection, error boundaries, circuit breakers, and middleware systems.

### 2.1 Frontend Architecture Enhancements

#### Error Boundary System
**src/common/components/ErrorBoundary.tsx**
```typescript
import React, { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo);
    this.props.onError?.(error, errorInfo);
  }

  public render() {
    if (this.state.hasError) {
      return this.props.fallback || (
        <div className="error-boundary">
          <h2>Something went wrong.</h2>
          <details style={{ whiteSpace: 'pre-wrap' }}>
            {this.state.error && this.state.error.toString()}
          </details>
        </div>
      );
    }

    return this.props.children;
  }
}
```

#### React Query Integration
```bash
npm install @tanstack/react-query @tanstack/react-query-devtools
```

**src/app/providers/QueryProvider.tsx**
```typescript
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { ReactNode } from 'react';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes
      retry: (failureCount, error) => {
        if (error?.status === 404) return false;
        return failureCount < 3;
      },
    },
  },
});

interface Props {
  children: ReactNode;
}

export function QueryProvider({ children }: Props) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}
```

#### Custom Hooks for Business Logic
**src/features/memories/hooks/useMemories.ts**
```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { nodesApi } from '../api/nodes';
import type { Node } from '@services';

export function useMemories(userId: string) {
  return useQuery({
    queryKey: ['memories', userId],
    queryFn: () => nodesApi.listNodes(userId),
    enabled: !!userId,
  });
}

export function useCreateMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: nodesApi.createNode,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['memories'] });
      queryClient.invalidateQueries({ queryKey: ['graph'] });
    },
  });
}

export function useDeleteMemory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ userId, nodeId }: { userId: string; nodeId: string }) =>
      nodesApi.deleteNode(userId, nodeId),
    onMutate: async ({ nodeId }) => {
      await queryClient.cancelQueries({ queryKey: ['memories'] });
      
      const previousMemories = queryClient.getQueryData(['memories']);
      
      queryClient.setQueryData(['memories'], (old: Node[] = []) =>
        old.filter(memory => memory.id !== nodeId)
      );

      return { previousMemories };
    },
    onError: (err, variables, context) => {
      if (context?.previousMemories) {
        queryClient.setQueryData(['memories'], context.previousMemories);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['memories'] });
    },
  });
}
```

### 2.2 Backend Architecture Enhancements

#### Dependency Injection Container
**internal/container/container.go**
```go
package container

import (
	"context"

	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/memory"
	"brain2-backend/internal/service/category"
	"brain2-backend/internal/service/llm"
	"brain2-backend/pkg/config"
	"brain2-backend/infrastructure/dynamodb"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type Container struct {
	cfg    *config.Config
	ddb    *dynamodb.Client
	repo   repository.Repository
	
	memoryService   *memory.Service
	categoryService *category.Service
	llmService      *llm.Service
}

func NewContainer(cfg *config.Config, ddbClient *dynamodb.Client) *Container {
	c := &Container{
		cfg: cfg,
		ddb: ddbClient,
	}
	
	c.initializeServices()
	return c
}

func (c *Container) initializeServices() {
	// Repository layer
	c.repo = dynamodb.NewRepository(c.ddb, c.cfg)
	
	// Service layer
	c.llmService = llm.NewService()
	c.memoryService = memory.NewService(c.repo, c.llmService)
	c.categoryService = category.NewService(c.repo, c.llmService)
}

// Getters
func (c *Container) MemoryService() *memory.Service   { return c.memoryService }
func (c *Container) CategoryService() *category.Service { return c.categoryService }
func (c *Container) LLMService() *llm.Service         { return c.llmService }
func (c *Container) Repository() repository.Repository { return c.repo }
```

#### Middleware System
**internal/middleware/middleware.go**
```go
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestID adds a unique request ID to the context
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// StructuredLogger provides structured logging for requests
func StructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&structuredLogger{logger})
}

type structuredLogger struct {
	logger *slog.Logger
}

func (l *structuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &structuredLogEntry{
		logger: l.logger,
		request: r,
	}
}

type structuredLogEntry struct {
	logger  *slog.Logger
	request *http.Request
}

func (l *structuredLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	l.logger.Info("request completed",
		slog.String("method", l.request.Method),
		slog.String("path", l.request.URL.Path),
		slog.Int("status", status),
		slog.Int("bytes", bytes),
		slog.Duration("elapsed", elapsed),
		slog.String("request_id", l.request.Context().Value("request_id").(string)),
	)
}

func (l *structuredLogEntry) Panic(v interface{}, stack []byte) {
	l.logger.Error("request panic",
		slog.Any("panic", v),
		slog.String("stack", string(stack)),
		slog.String("request_id", l.request.Context().Value("request_id").(string)),
	)
}
```

#### Circuit Breaker Pattern
**internal/circuit/breaker.go**
```go
package circuit

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests")
)

type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

type Settings struct {
	Name                string
	MaxRequests         uint32
	Interval            time.Duration
	Timeout             time.Duration
	ReadyToTrip         func(counts Counts) bool
	OnStateChange       func(name string, from State, to State)
}

type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	onStateChange func(name string, from State, to State)

	mutex      sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

func NewCircuitBreaker(st Settings) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:          st.Name,
		maxRequests:   st.MaxRequests,
		interval:      st.Interval,
		timeout:       st.Timeout,
		readyToTrip:   st.ReadyToTrip,
		onStateChange: st.OnStateChange,
	}

	cb.toNewGeneration(time.Now())
	return cb
}

func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, err == nil)
	return result, err
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	if state == StateHalfOpen {
		cb.setState(StateClosed, now)
	}
}

func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	if cb.readyToTrip(cb.counts) {
		cb.setState(StateOpen, now)
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}
```

### 2.3 Cross-Cutting Concerns

#### Structured Logging
**pkg/logger/logger.go**
```go
package logger

import (
	"context"
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func New(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: true,
	}
	
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	
	return &Logger{logger}
}

func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return l.Logger.With(slog.String("request_id", requestID))
	}
	return l.Logger
}

func (l *Logger) WithUserID(userID string) *slog.Logger {
	return l.Logger.With(slog.String("user_id", userID))
}

func (l *Logger) WithService(service string) *slog.Logger {
	return l.Logger.With(slog.String("service", service))
}
```

#### Metrics Collection
**pkg/metrics/metrics.go**
```go
package metrics

import (
	"context"
	"sync"
	"time"
)

type Metrics struct {
	counters map[string]int64
	gauges   map[string]float64
	timers   map[string][]time.Duration
	mutex    sync.RWMutex
}

func New() *Metrics {
	return &Metrics{
		counters: make(map[string]int64),
		gauges:   make(map[string]float64),
		timers:   make(map[string][]time.Duration),
	}
}

func (m *Metrics) Counter(name string) *Counter {
	return &Counter{metrics: m, name: name}
}

func (m *Metrics) Gauge(name string) *Gauge {
	return &Gauge{metrics: m, name: name}
}

func (m *Metrics) Timer(name string) *Timer {
	return &Timer{metrics: m, name: name}
}

type Counter struct {
	metrics *Metrics
	name    string
}

func (c *Counter) Inc() {
	c.Add(1)
}

func (c *Counter) Add(delta int64) {
	c.metrics.mutex.Lock()
	defer c.metrics.mutex.Unlock()
	c.metrics.counters[c.name] += delta
}

type Gauge struct {
	metrics *Metrics
	name    string
}

func (g *Gauge) Set(value float64) {
	g.metrics.mutex.Lock()
	defer g.metrics.mutex.Unlock()
	g.metrics.gauges[g.name] = value
}

type Timer struct {
	metrics *Metrics
	name    string
}

func (t *Timer) Time(fn func()) {
	start := time.Now()
	fn()
	t.Record(time.Since(start))
}

func (t *Timer) Record(duration time.Duration) {
	t.metrics.mutex.Lock()
	defer t.metrics.mutex.Unlock()
	t.metrics.timers[t.name] = append(t.metrics.timers[t.name], duration)
}
```

---

# Phase 3: Documentation Excellence & Developer Experience
## Target: +0.5 points (Medium Priority)

### Overview
Create comprehensive documentation, API specifications, and developer tools to ensure excellent developer experience and knowledge transfer.

### 3.1 API Documentation

#### OpenAPI Specification Enhancement
**openapi.yaml** (Enhanced version)
```yaml
openapi: 3.0.3
info:
  title: Brain2 - Second Brain API
  description: |
    A graph-based personal knowledge management system API that automatically 
    connects memories, thoughts, and ideas based on their content.
    
    ## Authentication
    All endpoints require JWT authentication via Supabase.
    Include the token in the Authorization header: `Bearer <jwt_token>`
    
    ## Rate Limiting  
    API requests are limited to 1000 requests per hour per user.
    
    ## Error Handling
    All errors follow the standard HTTP status codes with detailed error messages
    in the response body.
  version: 2.0.0
  contact:
    name: API Support
    email: support@brain2.com
  license:
    name: MIT
    url: https://opensource.org/licenses/MIT

servers:
  - url: https://api.brain2.com/v1
    description: Production server
  - url: https://staging-api.brain2.com/v1
    description: Staging server
  - url: http://localhost:3000/v1
    description: Development server

security:
  - JWTAuth: []

components:
  securitySchemes:
    JWTAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: Supabase JWT token

  schemas:
    Error:
      type: object
      required:
        - error
        - message
      properties:
        error:
          type: string
          description: Error code
          example: "VALIDATION_ERROR"
        message:
          type: string
          description: Human-readable error message
          example: "Content cannot be empty"
        details:
          type: object
          description: Additional error details
          
    Node:
      type: object
      required:
        - id
        - user_id
        - content
        - created_at
      properties:
        id:
          type: string
          format: uuid
          description: Unique identifier for the memory node
          example: "550e8400-e29b-41d4-a716-446655440000"
        user_id:
          type: string
          format: uuid
          description: ID of the user who owns this memory
          example: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
        content:
          type: string
          description: The actual memory content
          minLength: 1
          maxLength: 10000
          example: "Meeting notes: Discussed project timeline and deliverables"
        keywords:
          type: array
          items:
            type: string
          description: Extracted keywords for connection purposes
          example: ["meeting", "project", "timeline", "deliverables"]
        created_at:
          type: string
          format: date-time
          description: When the memory was created
          example: "2024-01-15T10:30:00Z"
        updated_at:
          type: string
          format: date-time
          description: When the memory was last updated
          example: "2024-01-15T10:30:00Z"

paths:
  /memories:
    get:
      summary: List memories
      description: |
        Retrieve a paginated list of memories for the authenticated user.
        Results can be filtered and sorted.
      tags:
        - Memories
      parameters:
        - name: limit
          in: query
          description: Number of memories to return (max 100)
          schema:
            type: integer
            minimum: 1
            maximum: 100
            default: 20
        - name: offset
          in: query
          description: Number of memories to skip
          schema:
            type: integer
            minimum: 0
            default: 0
        - name: search
          in: query
          description: Search query to filter memories
          schema:
            type: string
            maxLength: 1000
        - name: sort
          in: query
          description: Sort order for results
          schema:
            type: string
            enum: [created_asc, created_desc, updated_asc, updated_desc]
            default: created_desc
      responses:
        '200':
          description: List of memories
          content:
            application/json:
              schema:
                type: object
                properties:
                  memories:
                    type: array
                    items:
                      $ref: '#/components/schemas/Node'
                  total:
                    type: integer
                    description: Total number of memories
                  limit:
                    type: integer
                  offset:
                    type: integer
              examples:
                success:
                  summary: Successful response
                  value:
                    memories:
                      - id: "550e8400-e29b-41d4-a716-446655440000"
                        user_id: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
                        content: "Meeting notes from today"
                        keywords: ["meeting", "notes"]
                        created_at: "2024-01-15T10:30:00Z"
                        updated_at: "2024-01-15T10:30:00Z"
                    total: 1
                    limit: 20
                    offset: 0
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
    post:
      summary: Create memory
      description: Create a new memory node with automatic keyword extraction
      tags:
        - Memories
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - content
              properties:
                content:
                  type: string
                  minLength: 1
                  maxLength: 10000
                  description: Memory content
            examples:
              simple:
                summary: Simple memory
                value:
                  content: "Remember to buy groceries tomorrow"
              detailed:
                summary: Detailed memory
                value:
                  content: "Project meeting conclusions: We decided to use microservices architecture with Docker containers. Next steps include setting up CI/CD pipeline and defining API contracts."
      responses:
        '201':
          description: Memory created successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Node'
        '400':
          description: Validation error
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'

tags:
  - name: Memories
    description: Memory management operations
  - name: Categories
    description: Category and organization operations
  - name: Graph
    description: Graph visualization and connection operations
```

#### Swagger UI Setup
**docs/index.html**
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Brain2 API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
    <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: './openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
    };
    </script>
</body>
</html>
```

### 3.2 Architectural Decision Records (ADRs)

#### ADR Template
**docs/adr/template.md**
```markdown
# ADR-XXXX: [Title]

## Status
[Proposed | Accepted | Deprecated | Superseded]

## Context
What is the issue that we're seeing that is motivating this decision or change?

## Decision
What is the change that we're proposing or have agreed to implement?

## Consequences
What becomes easier or more difficult to do and any risks introduced by this change?

### Positive
- Benefit 1
- Benefit 2

### Negative  
- Risk 1
- Risk 2

### Neutral
- Change 1
- Change 2

## Alternatives Considered
What other options were considered and why were they rejected?

## Related Decisions
Links to related ADRs or decisions that influenced this one.

## References
Links to external resources, documentation, or research that informed this decision.
```

#### Example ADR
**docs/adr/0001-use-single-table-dynamodb-design.md**
```markdown
# ADR-0001: Use Single-Table DynamoDB Design

## Status
Accepted

## Context
We need to design the database structure for Brain2, which stores memories (nodes), their relationships (edges), categories, and user data. The system needs to:

1. Support complex queries for graph traversal
2. Scale to millions of memories per user
3. Maintain low latency for real-time updates
4. Keep costs predictable and low

We considered three approaches:
1. Relational database (PostgreSQL)
2. Multiple DynamoDB tables (normalized approach)
3. Single DynamoDB table (denormalized approach)

## Decision
We will use a single-table DynamoDB design with the following structure:

- **Primary Key (PK)**: Composite key for data partitioning
- **Sort Key (SK)**: For data sorting and range queries
- **GSI1**: For alternate access patterns
- **Overloaded attributes**: Different entity types in same table

### Key Patterns:
- User: `USER#<user_id>` | `PROFILE`
- Memory: `USER#<user_id>` | `MEMORY#<memory_id>`
- Edge: `USER#<user_id>` | `EDGE#<source_id>#<target_id>`
- Category: `USER#<user_id>` | `CATEGORY#<category_id>`

## Consequences

### Positive
- **Cost Efficiency**: Single table reduces DynamoDB costs significantly
- **Performance**: Single-digit millisecond latency with proper key design
- **Atomic Operations**: Transactions can span related entities
- **Scalability**: Automatic scaling with minimal operational overhead
- **Query Efficiency**: Most queries require single table access

### Negative
- **Complexity**: Requires careful access pattern design
- **Learning Curve**: Team needs to understand NoSQL modeling
- **Query Limitations**: Some complex queries require application-level joins
- **Schema Evolution**: Changes require careful migration planning

### Neutral
- **Data Modeling**: Requires upfront investment in access pattern analysis
- **Tooling**: Limited traditional SQL tooling support

## Alternatives Considered

### PostgreSQL with Graph Extensions
- **Pros**: SQL familiarity, complex queries, ACID transactions
- **Cons**: Higher operational overhead, scaling complexity, cost at scale
- **Rejected**: Operational complexity and scaling costs

### Multiple DynamoDB Tables
- **Pros**: Cleaner separation, easier to understand
- **Cons**: Cross-table queries, higher costs, transaction limitations
- **Rejected**: Cost and performance implications

## Related Decisions
- ADR-0002: Event-driven architecture for real-time updates
- ADR-0003: Serverless-first approach

## References
- [AWS DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [The DynamoDB Book](https://dynamodbbook.com/)
- [Single Table Design Patterns](https://www.alexdebrie.com/posts/dynamodb-single-table/)
```

### 3.3 Developer Documentation

#### Development Setup Guide
**docs/DEVELOPMENT.md**
```markdown
# Development Guide

## Prerequisites

### Required Software
- **Node.js 20+** - JavaScript runtime
- **Go 1.21+** - Backend language
- **AWS CLI** - AWS command line interface
- **AWS CDK CLI** - Infrastructure as code tool
- **Docker** - For local testing (optional)

### Recommended Tools
- **Visual Studio Code** with extensions:
  - Go extension
  - TypeScript extension
  - AWS Toolkit
  - REST Client
- **Postman** or **Insomnia** - API testing
- **DynamoDB Local** - Local testing

## Project Structure

```
brain2/
├── frontend/           # React TypeScript application
│   ├── src/
│   │   ├── app/        # Application setup and routing
│   │   ├── common/     # Shared components and utilities
│   │   ├── features/   # Feature-based modules
│   │   ├── services/   # API clients and external services
│   │   └── types/      # TypeScript type definitions
│   ├── tests/          # Test files and setup
│   └── dist/           # Build output
├── backend/            # Go Lambda functions
│   ├── cmd/            # Application entry points
│   ├── internal/       # Private application code
│   │   ├── domain/     # Business entities
│   │   ├── repository/ # Data access layer
│   │   └── service/    # Business logic
│   ├── pkg/            # Public packages
│   └── infrastructure/ # External service implementations
├── infra/              # AWS CDK infrastructure code
│   ├── lib/            # CDK stack definitions
│   ├── lambda/         # Node.js Lambda functions
│   └── test/           # Infrastructure tests
└── docs/               # Documentation
    ├── api/            # API documentation
    ├── adr/            # Architectural decision records
    └── guides/         # Development guides
```

## Quick Start

### 1. Clone and Install Dependencies
```bash
git clone <repository-url>
cd brain2

# Install frontend dependencies
cd frontend && npm install && cd ..

# Install infrastructure dependencies  
cd infra && npm install && cd ..

# Install Go dependencies
cd backend && go mod tidy && cd ..
```

### 2. Environment Setup
```bash
# Copy environment templates
cp frontend/.env.example frontend/.env
cp infra/.env.example infra/.env

# Edit the .env files with your credentials
```

### 3. Build and Deploy
```bash
# Build everything
./build.sh

# Deploy infrastructure
cd infra
npx cdk deploy --all
```

## Development Workflow

### Frontend Development
```bash
cd frontend

# Start development server
npm run dev

# Run tests
npm run test
npm run test:coverage

# Run linting
npm run lint
npm run type-check

# Build for production
npm run build
```

### Backend Development
```bash
cd backend

# Run tests
go test ./...
make test-coverage

# Build Lambda functions
make build

# Run local API server (if available)
go run cmd/main/main.go
```

### Infrastructure Development
```bash
cd infra

# Run tests
npm test

# Synthesize CloudFormation
npx cdk synth

# Deploy changes
npx cdk deploy

# Destroy resources (caution!)
npx cdk destroy
```

## Testing Strategy

### Unit Tests
- **Frontend**: Vitest + React Testing Library
- **Backend**: Go standard testing + testify
- **Infrastructure**: Jest + CDK assertions

### Integration Tests
- **API Testing**: Supertest or similar
- **Database Testing**: DynamoDB Local
- **Infrastructure Testing**: CDK integration tests

### End-to-End Tests
- **Browser Testing**: Playwright
- **API Workflow Testing**: Custom test suites

## Code Quality

### Linting and Formatting
```bash
# Frontend
npm run lint        # ESLint
npm run format      # Prettier

# Backend  
make lint          # golangci-lint
make format        # gofmt

# Infrastructure
npm run lint       # ESLint for TypeScript
```

### Pre-commit Hooks
We use Husky for pre-commit hooks that run:
- Linting
- Type checking
- Unit tests
- Formatting

## Debugging

### Frontend Debugging
- Use browser developer tools
- React Developer Tools extension
- Vite dev server for hot reloading

### Backend Debugging
- Use VS Code Go debugger
- Add logging with structured logger
- Use AWS CloudWatch for deployed functions

### Infrastructure Debugging
- CDK diff to see changes
- CloudFormation events in AWS Console
- AWS CLI for resource inspection

## Common Issues

### Build Issues
```bash
# Clear all caches
rm -rf frontend/node_modules frontend/dist
rm -rf infra/node_modules infra/cdk.out
rm -rf backend/bin

# Reinstall dependencies
cd frontend && npm install
cd ../infra && npm install
cd ../backend && go mod tidy
```

### Deployment Issues
```bash
# Check AWS credentials
aws sts get-caller-identity

# Verify CDK bootstrap
npx cdk bootstrap

# Check CloudFormation stack status
aws cloudformation describe-stacks --stack-name <stack-name>
```

## Performance Tips

### Frontend
- Use React.memo for expensive components
- Implement virtual scrolling for large lists
- Optimize bundle size with code splitting
- Use service workers for caching

### Backend
- Monitor Lambda cold starts
- Optimize DynamoDB queries
- Use connection pooling where applicable
- Implement proper error handling

### Infrastructure
- Monitor AWS costs regularly
- Use appropriate Lambda memory settings
- Implement CloudWatch alarms
- Regular security reviews

## Contributing

1. Create feature branch from `main`
2. Make changes with tests
3. Run full test suite
4. Create pull request
5. Code review and merge

## Support

- **Documentation**: Check `/docs` directory
- **API Reference**: `/docs/api/openapi.yaml`
- **Issues**: GitHub issue tracker
- **Architecture**: `/docs/adr/` directory
```

### 3.4 Code Examples and Tutorials

#### Tutorial: Adding a New Feature
**docs/tutorials/adding-new-feature.md**
```markdown
# Tutorial: Adding a New Feature

This tutorial walks through adding a new feature to Brain2, using "Memory Tags" as an example.

## Overview
We'll add the ability to tag memories with custom labels for better organization.

## Step 1: Design the Feature

### Domain Model
```go
// internal/domain/tag.go
type Tag struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    Name        string    `json:"name"`
    Color       string    `json:"color"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}

type MemoryTag struct {
    UserID   string `json:"user_id"`
    MemoryID string `json:"memory_id"`
    TagID    string `json:"tag_id"`
}
```

### API Endpoints
- `GET /tags` - List user's tags
- `POST /tags` - Create new tag
- `PUT /tags/{id}` - Update tag
- `DELETE /tags/{id}` - Delete tag
- `POST /memories/{id}/tags/{tagId}` - Add tag to memory
- `DELETE /memories/{id}/tags/{tagId}` - Remove tag from memory

## Step 2: Backend Implementation

### Repository Layer
```go
// internal/repository/repository.go
type Repository interface {
    // ... existing methods ...
    
    // Tag operations
    CreateTag(ctx context.Context, tag domain.Tag) error
    UpdateTag(ctx context.Context, tag domain.Tag) error
    DeleteTag(ctx context.Context, userID, tagID string) error
    FindTagByID(ctx context.Context, userID, tagID string) (*domain.Tag, error)
    FindTags(ctx context.Context, userID string) ([]domain.Tag, error)
    
    // Memory-Tag relationships
    AddTagToMemory(ctx context.Context, mapping domain.MemoryTag) error
    RemoveTagFromMemory(ctx context.Context, userID, memoryID, tagID string) error
    FindTagsForMemory(ctx context.Context, userID, memoryID string) ([]domain.Tag, error)
    FindMemoriesWithTag(ctx context.Context, userID, tagID string) ([]domain.Node, error)
}
```

### Service Layer
```go
// internal/service/tag/service.go
package tag

import (
    "context"
    "brain2-backend/internal/domain"
    "brain2-backend/internal/repository"
)

type Service struct {
    repo repository.Repository
}

func NewService(repo repository.Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) CreateTag(ctx context.Context, tag domain.Tag) error {
    // Validation
    if err := s.validateTag(tag); err != nil {
        return err
    }
    
    // Business logic
    tag.ID = generateID()
    tag.CreatedAt = time.Now()
    
    return s.repo.CreateTag(ctx, tag)
}

func (s *Service) validateTag(tag domain.Tag) error {
    if tag.Name == "" {
        return errors.New("tag name cannot be empty")
    }
    if len(tag.Name) > 50 {
        return errors.New("tag name too long")
    }
    return nil
}
```

### Handler Layer
```go
// cmd/main/handlers/tag_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    
    "brain2-backend/internal/service/tag"
    "github.com/go-chi/chi/v5"
)

type TagHandler struct {
    tagService *tag.Service
}

func NewTagHandler(tagService *tag.Service) *TagHandler {
    return &TagHandler{tagService: tagService}
}

func (h *TagHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name        string `json:"name"`
        Color       string `json:"color"`
        Description string `json:"description"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    userID := getUserID(r.Context())
    tag := domain.Tag{
        UserID:      userID,
        Name:        req.Name,
        Color:       req.Color,
        Description: req.Description,
    }
    
    if err := h.tagService.CreateTag(r.Context(), tag); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(tag)
}

func (h *TagHandler) RegisterRoutes(r chi.Router) {
    r.Route("/tags", func(r chi.Router) {
        r.Get("/", h.ListTags)
        r.Post("/", h.CreateTag)
        r.Put("/{tagID}", h.UpdateTag)
        r.Delete("/{tagID}", h.DeleteTag)
    })
}
```

## Step 3: Frontend Implementation

### API Client
```typescript
// src/services/apiClient.ts (add to existing)
export const api = {
  // ... existing methods ...
  
  // Tag operations
  async createTag(tag: CreateTagRequest): Promise<Tag> {
    const response = await this.request<Tag>('/tags', {
      method: 'POST',
      body: JSON.stringify(tag),
    });
    return response;
  },
  
  async listTags(): Promise<Tag[]> {
    return this.request<Tag[]>('/tags');
  },
  
  async addTagToMemory(memoryId: string, tagId: string): Promise<void> {
    await this.request(`/memories/${memoryId}/tags/${tagId}`, {
      method: 'POST',
    });
  },
};
```

### React Hooks
```typescript
// src/features/tags/hooks/useTags.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@services/apiClient';

export function useTags() {
  return useQuery({
    queryKey: ['tags'],
    queryFn: api.listTags,
  });
}

export function useCreateTag() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: api.createTag,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}
```

### Components
```typescript
// src/features/tags/components/TagManager.tsx
import React, { useState } from 'react';
import { useTags, useCreateTag } from '../hooks/useTags';

export function TagManager() {
  const { data: tags, isLoading } = useTags();
  const createTag = useCreateTag();
  const [newTag, setNewTag] = useState({ name: '', color: '#3B82F6' });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createTag.mutate(newTag, {
      onSuccess: () => setNewTag({ name: '', color: '#3B82F6' }),
    });
  };

  if (isLoading) return <div>Loading tags...</div>;

  return (
    <div className="tag-manager">
      <h2>Manage Tags</h2>
      
      <form onSubmit={handleSubmit} className="tag-form">
        <input
          type="text"
          placeholder="Tag name"
          value={newTag.name}
          onChange={(e) => setNewTag({ ...newTag, name: e.target.value })}
          required
        />
        <input
          type="color"
          value={newTag.color}
          onChange={(e) => setNewTag({ ...newTag, color: e.target.value })}
        />
        <button type="submit" disabled={createTag.isPending}>
          Create Tag
        </button>
      </form>

      <div className="tags-list">
        {tags?.map((tag) => (
          <div key={tag.id} className="tag-item">
            <span 
              className="tag-color" 
              style={{ backgroundColor: tag.color }}
            />
            <span className="tag-name">{tag.name}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
```

## Step 4: Testing

### Backend Tests
```go
// internal/service/tag/service_test.go
func TestTagService_CreateTag(t *testing.T) {
    repo := &mocks.MockRepository{}
    service := NewService(repo)
    
    tag := domain.Tag{
        UserID: "user-1",
        Name:   "Important",
        Color:  "#FF0000",
    }
    
    repo.On("CreateTag", mock.Anything, mock.MatchedBy(func(t domain.Tag) bool {
        return t.Name == "Important" && t.UserID == "user-1"
    })).Return(nil)
    
    err := service.CreateTag(context.Background(), tag)
    assert.NoError(t, err)
    repo.AssertExpectations(t)
}
```

### Frontend Tests
```typescript
// src/features/tags/components/__tests__/TagManager.test.tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TagManager } from '../TagManager';
import { vi } from 'vitest';

vi.mock('@services/apiClient', () => ({
  api: {
    listTags: vi.fn().mockResolvedValue([
      { id: '1', name: 'Important', color: '#FF0000' }
    ]),
    createTag: vi.fn().mockResolvedValue({ id: '2', name: 'New Tag' }),
  },
}));

describe('TagManager', () => {
  const renderWithQuery = (component: React.ReactElement) => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    return render(
      <QueryClientProvider client={queryClient}>
        {component}
      </QueryClientProvider>
    );
  };

  it('renders tag list', async () => {
    renderWithQuery(<TagManager />);
    
    await waitFor(() => {
      expect(screen.getByText('Important')).toBeInTheDocument();
    });
  });

  it('creates new tag', async () => {
    renderWithQuery(<TagManager />);
    
    fireEvent.change(screen.getByPlaceholderText('Tag name'), {
      target: { value: 'Urgent' },
    });
    
    fireEvent.click(screen.getByText('Create Tag'));
    
    await waitFor(() => {
      expect(api.createTag).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Urgent' })
      );
    });
  });
});
```

## Step 5: Documentation Updates

### OpenAPI Specification
```yaml
# Add to openapi.yaml
/tags:
  get:
    summary: List tags
    responses:
      '200':
        description: List of user tags
        content:
          application/json:
            schema:
              type: array
              items:
                $ref: '#/components/schemas/Tag'
  post:
    summary: Create tag
    requestBody:
      required: true
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/CreateTagRequest'
    responses:
      '201':
        description: Tag created
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Tag'

components:
  schemas:
    Tag:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        color:
          type: string
        description:
          type: string
        created_at:
          type: string
          format: date-time
```

This tutorial demonstrates the complete flow of adding a new feature following Brain2's architectural patterns and best practices.
```

---

# Phase 4: Code Quality Infrastructure
## Target: +0.2 points (Low Priority, High Impact)

### Overview
Establish automated code quality checks, formatting, and static analysis tools across all layers of the application.

### 4.1 Frontend Code Quality

#### ESLint Configuration
**frontend/.eslintrc.js**
```javascript
module.exports = {
  root: true,
  env: {
    browser: true,
    es2022: true,
    node: true,
  },
  extends: [
    'eslint:recommended',
    '@typescript-eslint/recommended',
    '@typescript-eslint/recommended-requiring-type-checking',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
    'plugin:jsx-a11y/recommended',
    'plugin:import/recommended',
    'plugin:import/typescript',
    'prettier',
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    project: './tsconfig.json',
    ecmaFeatures: {
      jsx: true,
    },
  },
  plugins: [
    '@typescript-eslint',
    'react',
    'react-hooks',
    'jsx-a11y',
    'import',
  ],
  rules: {
    // TypeScript specific rules
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/explicit-function-return-type': 'off',
    '@typescript-eslint/explicit-module-boundary-types': 'off',
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/prefer-const': 'error',
    '@typescript-eslint/no-var-requires': 'error',
    
    // React specific rules
    'react/react-in-jsx-scope': 'off',
    'react/prop-types': 'off',
    'react/display-name': 'off',
    'react-hooks/rules-of-hooks': 'error',
    'react-hooks/exhaustive-deps': 'warn',
    
    // Import rules
    'import/order': [
      'error',
      {
        groups: [
          'builtin',
          'external',
          'internal',
          'parent',
          'sibling',
          'index',
        ],
        'newlines-between': 'always',
        alphabetize: { order: 'asc', caseInsensitive: true },
      },
    ],
    'import/no-unresolved': 'error',
    'import/no-cycle': 'error',
    'import/no-unused-modules': 'warn',
    
    // General rules
    'no-console': 'warn',
    'no-debugger': 'error',
    'prefer-const': 'error',
    'no-var': 'error',
  },
  settings: {
    react: {
      version: 'detect',
    },
    'import/resolver': {
      typescript: {
        alwaysTryTypes: true,
        project: './tsconfig.json',
      },
    },
  },
  ignorePatterns: [
    'dist',
    'node_modules',
    'coverage',
    'src/types/generated',
  ],
};
```

#### Prettier Configuration
**frontend/.prettierrc.js**
```javascript
module.exports = {
  semi: true,
  trailingComma: 'es5',
  singleQuote: true,
  printWidth: 80,
  tabWidth: 2,
  useTabs: false,
  bracketSpacing: true,
  bracketSameLine: false,
  arrowParens: 'avoid',
  endOfLine: 'lf',
  overrides: [
    {
      files: '*.json',
      options: {
        printWidth: 200,
      },
    },
  ],
};
```

#### Stylelint Configuration
**frontend/.stylelintrc.js**
```javascript
module.exports = {
  extends: [
    'stylelint-config-standard',
    'stylelint-config-prettier',
  ],
  rules: {
    'selector-class-pattern': [
      '^[a-z]([a-z0-9-]+)?(__([a-z0-9]+-?)+)?(--([a-z0-9]+-?)+){0,2}$',
      {
        message: 'Expected class selector to be BEM format',
      },
    ],
    'declaration-block-trailing-semicolon': 'always',
    'length-zero-no-unit': true,
    'color-hex-case': 'lower',
    'color-hex-length': 'short',
  },
  ignoreFiles: [
    'dist/**/*',
    'node_modules/**/*',
  ],
};
```

### 4.2 Backend Code Quality

#### golangci-lint Configuration
**backend/.golangci.yml**
```yaml
run:
  timeout: 5m
  issues-exit-code: 1
  tests: true
  modules-download-mode: readonly

output:
  format: colored-line-number
  print-issued-lines: true
  print-linter-name: true

linters-settings:
  dupl:
    threshold: 100
  funlen:
    lines: 100
    statements: 50
  gci:
    local-prefixes: brain2-backend
  goconst:
    min-len: 2
    min-occurrences: 2
  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style
    disabled-checks:
      - dupImport
      - ifElseChain
      - octalLiteral
      - whyNoLint
      - wrapperFunc
  gocyclo:
    min-complexity: 15
  goimports:
    local-prefixes: brain2-backend
  golint:
    min-confidence: 0
  gomnd:
    settings:
      mnd:
        checks: argument,case,condition,operation,return,assign
  govet:
    check-shadowing: true
    settings:
      printf:
        funcs:
          - (github.com/golangci/golangci-lint/pkg/logutils.Log).Infof
          - (github.com/golangci/golangci-lint/pkg/logutils.Log).Warnf
          - (github.com/golangci/golangci-lint/pkg/logutils.Log).Errorf
          - (github.com/golangci/golangci-lint/pkg/logutils.Log).Fatalf
  lll:
    line-length: 140
  maligned:
    suggest-new: true
  misspell:
    locale: US
  nolintlint:
    allow-leading-space: true
    allow-unused: false
    require-explanation: false
    require-specific: false

linters:
  disable-all: true
  enable:
    - bodyclose
    - deadcode
    - depguard
    - dogsled
    - dupl
    - errcheck
    - funlen
    - gochecknoinits
    - goconst
    - gocritic
    - gocyclo
    - gofmt
    - goimports
    - golint
    - gomnd
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - interfacer
    - lll
    - misspell
    - nakedret
    - noctx
    - nolintlint
    - rowserrcheck
    - scopelint
    - staticcheck
    - structcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - varcheck
    - whitespace

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gomnd
        - funlen
        - goconst
    - path: cmd/
      linters:
        - gochecknoinits
  exclude:
    - abcdef
  exclude-use-default: false
  fix: false
  max-issues-per-linter: 0
  max-same-issues: 0
  new: false
```

#### Pre-commit Configuration
**backend/.pre-commit-config.yaml**
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-added-large-files
      - id: check-case-conflict
      - id: check-merge-conflict
      
  - repo: local
    hooks:
      - id: go-fmt
        name: go fmt
        entry: gofmt -w -s
        language: system
        types: [go]
        
      - id: go-imports
        name: go imports
        entry: goimports -w
        language: system
        types: [go]
        
      - id: go-lint
        name: go lint
        entry: golangci-lint run --fix
        language: system
        types: [go]
        pass_filenames: false
        
      - id: go-test
        name: go test
        entry: go test ./...
        language: system
        types: [go]
        pass_filenames: false
```

### 4.3 Infrastructure Code Quality

#### CDK Linting
**infra/.eslintrc.js**
```javascript
module.exports = {
  root: true,
  env: {
    node: true,
    es2022: true,
  },
  extends: [
    'eslint:recommended',
    '@typescript-eslint/recommended',
    '@typescript-eslint/recommended-requiring-type-checking',
    'prettier',
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    project: './tsconfig.json',
  },
  plugins: ['@typescript-eslint'],
  rules: {
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/explicit-function-return-type': 'off',
    '@typescript-eslint/explicit-module-boundary-types': 'off',
    '@typescript-eslint/no-explicit-any': 'warn',
    'prefer-const': 'error',
    'no-var': 'error',
  },
  ignorePatterns: [
    'cdk.out',
    'node_modules',
    'coverage',
  ],
};
```

### 4.4 Git Hooks Setup

#### Husky Configuration
**package.json** (root level)
```json
{
  "name": "brain2-monorepo",
  "private": true,
  "scripts": {
    "prepare": "husky install",
    "lint:frontend": "cd frontend && npm run lint",
    "lint:backend": "cd backend && golangci-lint run",
    "lint:infra": "cd infra && npm run lint",
    "test:frontend": "cd frontend && npm run test:run",
    "test:backend": "cd backend && go test ./...",
    "test:infra": "cd infra && npm test",
    "format:frontend": "cd frontend && npm run format",
    "format:backend": "cd backend && gofmt -w -s .",
    "format:infra": "cd infra && npm run format"
  },
  "devDependencies": {
    "husky": "^8.0.3",
    "lint-staged": "^13.2.3"
  },
  "lint-staged": {
    "frontend/**/*.{ts,tsx}": [
      "cd frontend && npm run lint -- --fix",
      "cd frontend && npm run format"
    ],
    "backend/**/*.go": [
      "cd backend && gofmt -w -s",
      "cd backend && goimports -w",
      "cd backend && golangci-lint run --fix"
    ],
    "infra/**/*.ts": [
      "cd infra && npm run lint -- --fix",
      "cd infra && npm run format"
    ],
    "*.{json,yaml,yml,md}": [
      "prettier --write"
    ]
  }
}
```

#### Pre-commit Hook
**.husky/pre-commit**
```bash
#!/usr/bin/env sh
. "$(dirname "$0")/_/husky.sh"

echo "🔍 Running pre-commit checks..."

# Run lint-staged
npx lint-staged

# Run type checking
echo "🔍 Type checking frontend..."
cd frontend && npm run type-check
cd ..

# Run quick tests
echo "🧪 Running quick tests..."
npm run test:frontend -- --run --reporter=basic
npm run test:backend
npm run test:infra

echo "✅ Pre-commit checks passed!"
```

#### Commit Message Hook
**.husky/commit-msg**
```bash
#!/usr/bin/env sh
. "$(dirname "$0")/_/husky.sh"

# Validate commit message format
commit_regex='^(feat|fix|docs|style|refactor|test|chore)(\(.+\))?: .{1,50}'

if ! grep -qE "$commit_regex" "$1"; then
    echo "❌ Invalid commit message format!"
    echo "Format: type(scope): description"
    echo "Types: feat, fix, docs, style, refactor, test, chore"
    echo "Example: feat(auth): add JWT validation"
    exit 1
fi

echo "✅ Commit message format is valid"
```

### 4.5 CI/CD Integration

#### GitHub Actions Workflow
**.github/workflows/ci.yml**
```yaml
name: CI/CD Pipeline

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  frontend-tests:
    name: Frontend Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json
          
      - name: Install dependencies
        run: cd frontend && npm ci
        
      - name: Run linting
        run: cd frontend && npm run lint
        
      - name: Run type checking
        run: cd frontend && npm run type-check
        
      - name: Run unit tests
        run: cd frontend && npm run test:coverage
        
      - name: Run E2E tests
        run: cd frontend && npm run test:e2e
        
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: frontend/coverage/lcov.info
          flags: frontend

  backend-tests:
    name: Backend Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('backend/go.sum') }}
          
      - name: Install dependencies
        run: cd backend && go mod download
        
      - name: Run linting
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          working-directory: backend
          
      - name: Run tests
        run: cd backend && go test -v -race -coverprofile=coverage.out ./...
        
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: backend/coverage.out
          flags: backend

  infrastructure-tests:
    name: Infrastructure Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: infra/package-lock.json
          
      - name: Install dependencies
        run: cd infra && npm ci
        
      - name: Run linting
        run: cd infra && npm run lint
        
      - name: Run tests
        run: cd infra && npm test
        
      - name: CDK Synth
        run: cd infra && npx cdk synth

  security-scan:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          format: 'sarif'
          output: 'trivy-results.sarif'
          
      - name: Upload Trivy scan results
        uses: github/codeql-action/upload-sarif@v2
        with:
          sarif_file: 'trivy-results.sarif'

  deploy:
    name: Deploy
    needs: [frontend-tests, backend-tests, infrastructure-tests]
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1
          
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          
      - name: Build and deploy
        run: |
          ./build.sh
          cd infra
          npm ci
          npx cdk deploy --all --require-approval never
```

---

# Success Metrics & Validation

## Code Quality Metrics

### Testing Coverage Targets
- **Frontend**: >90% line coverage, >85% branch coverage
- **Backend**: >95% line coverage, >90% branch coverage  
- **Infrastructure**: >80% line coverage

### Performance Benchmarks
- **API Response Times**: <200ms for 95th percentile
- **Frontend Bundle Size**: <500KB gzipped
- **Lambda Cold Starts**: <1000ms
- **DynamoDB Queries**: <10ms average

### Code Quality Scores
- **ESLint**: Zero errors, <10 warnings
- **golangci-lint**: Zero critical issues
- **SonarQube Quality Gate**: Passed
- **Security Scan**: Zero high/critical vulnerabilities

## Implementation Timeline

### Phase 1: Testing Infrastructure (3-4 weeks)
**Week 1-2**: Frontend testing setup and basic tests  
**Week 3**: Backend testing expansion  
**Week 4**: Infrastructure testing and CI integration

### Phase 2: Architectural Patterns (2-3 weeks)
**Week 1**: Frontend architecture enhancements  
**Week 2**: Backend patterns implementation  
**Week 3**: Integration and testing

### Phase 3: Documentation (1-2 weeks)
**Week 1**: API documentation and ADRs  
**Week 2**: Developer guides and tutorials

### Phase 4: Code Quality (1 week)
**Week 1**: Linting, formatting, and automation setup

## Risk Mitigation

### Technical Risks
- **Testing Setup Complexity**: Start with basic setup, iterate
- **CI/CD Pipeline Failures**: Implement gradual rollout
- **Performance Regression**: Establish baseline metrics first

### Team Risks  
- **Learning Curve**: Provide training and documentation
- **Time Constraints**: Prioritize high-impact improvements
- **Resistance to Change**: Demonstrate clear benefits

## Success Validation

### Automated Checks
- All CI/CD pipelines pass
- Coverage thresholds met
- Performance benchmarks achieved
- Security scans clean

### Manual Reviews
- Code review process improvements
- Developer experience feedback
- Documentation completeness
- Architecture review

### Business Impact
- Faster feature delivery
- Reduced bug reports
- Improved team confidence
- Better code maintainability

---

# Conclusion

This comprehensive plan provides a roadmap to achieve 10/10 code organization by systematically addressing the four key areas:

1. **Testing Infrastructure** (+2.0 points) - Foundation for quality
2. **Architectural Patterns** (+0.8 points) - Enterprise-grade patterns  
3. **Documentation Excellence** (+0.5 points) - Knowledge sharing
4. **Code Quality Tools** (+0.2 points) - Automated quality assurance

**Total Improvement**: +3.5 points → **Final Rating: 10/10**

The implementation follows a phased approach with clear deliverables, success metrics, and risk mitigation strategies. Each phase builds upon the previous one, ensuring a solid foundation for long-term maintainability and scalability.

Upon completion, Brain2 will serve as an exemplary model of modern software engineering practices, demonstrating industry-leading code organization that exceeds current standards.