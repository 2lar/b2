# Backend

Brain2's backend is written in Go and organised around Domain-Driven Design (DDD), Clean Architecture, and CQRS. It powers the graph knowledge engine, exposing REST APIs (with scaffolding for WebSocket, GraphQL, and gRPC adapters), coordinates asynchronous workloads via AWS EventBridge and Lambda, and persists state in DynamoDB. This README describes how the backend is structured, how to work with it safely, and which tooling is available for local development.

## Quick Start

1. **Prerequisites**
   - Go 1.23+
   - AWS CLI configured with credentials for local development
   - (Optional) Docker + DynamoDB Local if you want to avoid touching AWS resources

2. **Install dependencies**
   ```bash
   cd backend
   make deps               # go mod download + tidy + verify
   ```

3. **Run the API locally**
   ```bash
   make run                # builds and starts cmd/api with wiring
   # or use hot reload
   ./dev.sh                # uses Air once installed
   ```

4. **Execute tests**
   ```bash
   make test               # full suite using ./test.sh under the hood
   ./test.sh --all --coverage
   ```

5. **Build binaries**
   ```bash
   ./build.sh              # builds api, worker, and Lambda binaries into ./build
   ./build.sh --component api --quick --skip-tests
   ```

For a full build of the entire monorepo see `../build.sh` at the repository root.

## Architecture at a Glance

```
┌────────────────────────────────────────────┐
│                Interfaces                  │
│  REST (chi), WebSocket hub, future gRPC    │
├────────────────────────────────────────────┤
│               Application                  │
│  Commands, Queries, Mediator, Sagas, DI    │
├────────────────────────────────────────────┤
│                  Domain                    │
│  Graph aggregate, Nodes, Events, Specs     │
├────────────────────────────────────────────┤
│             Infrastructure                 │
│  DynamoDB, EventBridge, Observability, DI  │
└────────────────────────────────────────────┘
```

Dependencies always point inwards: interfaces depend on application, application depends on domain, and infrastructure provides adapters that are injected via interfaces defined in the application layer.

## Directory Guide

```
backend/
├── application/                # CQRS application layer
│   ├── commands/               # Command DTOs, validators, handlers (e.g. create_node)
│   ├── queries/                # Query DTOs and handlers (graph fetch, search projections)
│   ├── mediator/               # Mediator pattern coordinating command/query buses
│   ├── ports/                  # Interfaces for repositories, event bus, operation store
│   ├── projections/            # Read models such as GraphStatsProjection
│   ├── events/                 # Event handler registry and listeners (websocket, ops)
│   ├── loaders/                # DataLoader-style batching utilities
│   ├── sagas/                  # Long-running workflows (create node orchestrator, migrations)
│   └── services/               # Application services (GraphLoader, GraphLazyService, EdgeService)
│
├── domain/                     # Pure domain model
│   ├── core/                   # Aggregates, entities, value objects, validators
│   ├── events/                 # Domain events (NodeCreated, GraphUpdated, etc.)
│   ├── services/               # Domain services operating on aggregates
│   ├── specifications/         # Business rules expressed as specs
│   └── versioning/             # Schema migration helpers for aggregates
│
├── infrastructure/             # Adapters for external systems
│   ├── acl/                    # Anti-corruption layer for third-party APIs
│   ├── config/                 # Env loading, dynamic feature flags, watchers
│   ├── di/                     # Google Wire providers + generated container
│   ├── messaging/              # EventBridge publisher and dispatch pipeline
│   ├── observability/          # Logging (zap), metrics, tracing helpers
│   └── persistence/            # DynamoDB repositories, caches, in-memory implementations
│
├── interfaces/                 # Delivery mechanisms
│   ├── http/                   # REST router (chi), handlers, middleware, v1 endpoints
│   ├── websocket/              # Hub/server for real-time updates
│   ├── graphql/                # Placeholder for future GraphQL adapter
│   ├── grpc/                   # Placeholder for future gRPC services
│   └── cli/                    # Reserved for command-line utilities
│
├── pkg/                        # Cross-cutting helpers shared across layers
│   ├── auth/                   # JWT helpers and API Gateway integration
│   ├── common/                 # Utility types
│   ├── errors/                 # Centralised error handling and mapping
│   ├── observability/          # Wrapper helpers for metrics/tracing integration
│   ├── utils/                  # Generic helpers
│   └── validation/             # Request/domain validation helpers
│
├── cmd/                        # Deployable entrypoints
│   ├── api/                    # Long-running REST API for servers/containers
│   ├── lambda/                 # API Gateway Lambda (chi adapter)
│   ├── worker/                 # Background event processor & cleanup loops
│   ├── connect-node/           # Lambda to auto-connect nodes after creation
│   ├── cleanup-handler/        # Lambda for async resource cleanup
│   ├── ws-connect/             # API Gateway WebSocket connect handler
│   ├── ws-disconnect/          # API Gateway WebSocket disconnect handler
│   ├── ws-send-message/        # API Gateway WebSocket broadcast handler
│   └── migrate/                # CLI entrypoint for future schema migrations
│
├── tests/                      # Higher-level test suites
│   ├── unit/                   # Table-driven unit tests around aggregates & handlers
│   ├── integration/            # Adapters backed by DynamoDB/EventBridge fakes
│   ├── e2e/                    # End-to-end scenarios (API + async flows)
│   ├── fixtures/               # Re-usable JSON/YAML fixtures
│   └── mocks/                  # Generated mocks consumed by tests
│
├── build/                      # Build artefacts emitted by build.sh (ignored in git)
├── config/features.json        # Default feature-flag payload consumed by dynamic config
├── Makefile                    # Canonical build/test targets (fmt, lint, ci, etc.)
├── build.sh                    # Comprehensive build orchestration with caching options
├── test.sh                     # Test runner supporting unit/integration/e2e/coverage
└── dev.sh                      # Hot-reload friendly local runner for api/worker/migrate
```

## Domain Layer Highlights

- **Graph aggregate (`domain/core/aggregates`)** models nodes, edges, metadata, and supports lazy-loading behaviour for large graphs (`graph_lazy.go`).
- **Entities & value objects** cover `Node`, `Edge`, `GraphID`, `NodeID`, tags, content, timestamps, and guard invariants via validators and specifications.
- **Domain services** encapsulate operations that span aggregates (e.g. graph projections, edge policies) without leaking infrastructure concerns.
- **Domain events (`domain/events`)** such as `NodeCreated` or `EdgeDeleted` capture business outcomes and feed into application-level handlers.
- **Specifications** express complex business constraints (e.g. edge eligibility) to keep aggregates readable and composable.
- **Versioning helpers** support schema evolution so stored aggregates can be migrated forward safely.

## Application Layer Highlights

- **Commands (`application/commands`)** encapsulate state-changing operations. Handlers orchestrate repositories and services, perform validation, and emit domain events. Examples include `create_node`, `update_node`, `bulk_delete_nodes`, and `create_edge`.
- **Queries (`application/queries`)** provide read models, often delegating to projections (`application/projections`) for denormalised views like graph stats.
- **Mediator (`application/mediator`)** is the single entry point for commands/queries used by delivery adapters. It supports pipeline behaviours for logging, validation, and metrics.
- **Ports (`application/ports`)** declare the interfaces implemented by infrastructure adapters (node/edge/graph repositories, event bus, operation store). This keeps the application layer infrastructure-agnostic.
- **Services (`application/services`)** such as `GraphLoader`, `GraphLazyService`, and `EdgeService` provide reusable orchestration logic across handlers.
- **Sagas (`application/sagas`)** manage multi-step workflows (e.g. orchestrating node creation with async edge discovery or graph migrations) and hook into the operation store for status tracking.
- **Events (`application/events`)** contain the handler registry, listener implementations (e.g. WebSocket broadcaster), and integration points for async pipelines.
- **Loaders (`application/loaders`)** implement batched read-through caching patterns, useful for reducing DynamoDB round-trips when resolving projections.

## Infrastructure Layer Highlights

- **Configuration (`infrastructure/config`)** loads environment variables into `Config`, validates required settings, and exposes a dynamic config manager capable of hot-reloading `features.json` changes. Feature toggles include saga orchestrator, async deletion, auto-connect, and WebSocket support.
- **Dependency Injection (`infrastructure/di`)** uses Google Wire to assemble the container: repositories, mediator, event bus, logger, error handler, projections, and operation listeners. Run `make wire` (or rely on `build.sh`/`dev.sh`) when you change providers.
- **Persistence (`infrastructure/persistence`)** offers concrete adapters:
  - `dynamodb` repositories for nodes, edges, and graphs leveraging multiple GSIs.
  - `cache` and `memory` packages for in-memory/testing implementations.
  - `schema` helpers for table/index definitions if provisioning infrastructure.
  - `search` placeholder for future secondary indexes or search services.
- **Messaging (`infrastructure/messaging`)** wraps AWS EventBridge for publishing domain events and includes an in-process dispatcher for local development and worker processing.
- **Observability (`infrastructure/observability`)** configures zap loggers, metrics emission (CloudWatch), and optional OpenTelemetry tracing when enabled via environment flags.
- **ACL (`infrastructure/acl`)** provides anti-corruption adapters for third-party APIs (currently starting with an external API adapter stub).

## Interface Adapters

- **REST API (`interfaces/http/rest`)** uses chi with layered middleware (request ID, logging, auth). The v1 surface exposes:
  - `POST /api/v1/nodes/` (create), `GET/PUT/DELETE /api/v1/nodes/{nodeID}`, `GET /api/v1/nodes/`, `POST /api/v1/nodes/bulk-delete`
  - `GET /api/v1/graphs/{graphID}`, `/graphs/{graphID}/stats`, and filtered listings
  - `POST /api/v1/edges/` and `DELETE /api/v1/edges/{edgeID}`
  - `GET /api/v1/search` for graph-wide search
  - `GET /api/v1/graph-data` for visualisation payloads
  - `GET /api/v1/operations/{operationID}` for saga/async status tracking
  - Category routes are scaffolded for future taxonomy management
- **WebSocket adapter (`interfaces/websocket`)** implements a hub/server pair that keeps track of connections, broadcasts operation updates, and integrates with the application event listeners.
- **GraphQL & gRPC directories** are currently placeholders; adding resolvers/services here should reuse the mediator.
- **CLI (`interfaces/cli`)** is reserved for future command-line tooling.

## Entry Points (`cmd/`)

| Entry | Purpose | Notes |
|-------|---------|-------|
| `cmd/api` | Long-running REST API (used locally or in containers) | Wires all components, exposes chi router, enables local EventBridge dispatcher |
| `cmd/lambda` | API Gateway HTTP Lambda | Uses `aws-lambda-go-api-proxy` to wrap chi, pre-warms DynamoDB connections, handles authorizer context |
| `cmd/worker` | Background worker | Processes domain events via the dispatcher, runs periodic cleanup loops (extensible to saga processing) |
| `cmd/connect-node` | Async edge discovery Lambda | Invoked via EventBridge/SQS to create graph edges around a node |
| `cmd/cleanup-handler` | Resource cleanup Lambda | Stub for async removal of orphaned resources |
| `cmd/ws-*` | WebSocket connect/disconnect/message Lambdas | Manage API Gateway WebSocket lifecycle and DynamoDB connection tracking |
| `cmd/migrate` | Migration CLI | Currently a scaffold; extend when schema migrations are introduced |

Build artefacts are emitted to `./build/<component>/` (binary plus metadata). Lambda targets use the `bootstrap` naming convention.

## Dependency Injection Workflow

1. Update provider functions in `infrastructure/di/*.go` when introducing new dependencies.
2. Regenerate the wire graph:
   ```bash
   cd backend
   make wire        # or go run github.com/google/wire/cmd/wire ./...
   ```
3. The generated container (`wire_gen.go`) exposes `InitializeContainer` used by all entrypoints.

`dev.sh` and `build.sh` run Wire automatically to avoid stale containers.

## Configuration & Environment Variables

Configuration is provided via environment variables with sensible defaults. Key settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_ADDRESS` | `:8080` | HTTP bind address for `cmd/api` |
| `ENVIRONMENT` | `development` | Switches logger mode and validations |
| `AWS_REGION` | `us-west-2` | AWS region for clients |
| `DYNAMODB_TABLE` / `TABLE_NAME` | `brain2` | Primary DynamoDB table |
| `INDEX_NAME` | `KeywordIndex` | GSI1 for user-level queries |
| `GSI2_INDEX_NAME` | `EdgeIndex` | GSI2 for NodeID lookups |
| `GSI3_INDEX_NAME` | `TargetNodeIndex` | GSI3 for edge target lookups |
| `GSI4_INDEX_NAME` | `TagIndex` | GSI4 for tag-based queries |
| `EVENT_BUS_NAME` | `brain2-events` | EventBridge bus for domain events |
| `IS_LAMBDA` | `false` | Signals Lambda runtime for entrypoints |
| `COLD_START_TIMEOUT` | `3000` | Milliseconds allowed during Lambda cold start |
| `WEBSOCKET_ENDPOINT` | _empty_ | API Gateway endpoint for WebSocket callbacks |
| `CONNECTIONS_TABLE` | `brain2-connections` | DynamoDB table for WebSocket connections |
| `JWT_SECRET` | _empty_ | Required in production for signature verification |
| `JWT_ISSUER` | `brain2-backend2` | Token issuer validation |
| `LOG_LEVEL` | `info` | `debug` recommended during local dev |
| `ENABLE_METRICS` | `false` | Toggle CloudWatch metrics emission |
| `ENABLE_TRACING` | `false` | Toggle OpenTelemetry exporters |
| `ENABLE_CORS` | `true` | Enables CORS middleware |
| `ENABLE_LAZY_LOADING` | `true` | Enables lazy graph hydration |
| `EDGE_SYNC_LIMIT` | `20` | Sync edge creation threshold |
| `EDGE_SIMILARITY_THRESHOLD` | `0.3` | Minimum similarity score for auto edges |
| `EDGE_MAX_PER_NODE` | `100` | Safeguard on per-node edge counts |
| `EDGE_ASYNC_ENABLED` | `true` | Allows async edge creation |
| `FEATURE_*` | see defaults | Feature flags (saga orchestrator, async deletion, auto connect, websocket) |

For local iteration you can export variables inline or create a dir-local `.env` that you source via `scripts/load-env.sh` (from repository root).

## Feature Flags & Dynamic Configuration

- `config/features.json` ships default values for runtime toggles and operational limits.
- `DynamicConfigManager` watches the file (or alternative stores) and hot-reloads values, emitting callbacks so long-running processes can react without restarts.
- Feature flags govern whether sagas are enabled, async deletions run, auto-connect edges are created, and whether WebSocket broadcasting is active.
- Limits such as `syncEdgeLimit` or `maxEdgesPerNode` are updated live, enabling gradual tuning in production.

## Persistence & Data Access

- **DynamoDB** is the source of truth. Repositories are defined in `application/ports` and implemented in `infrastructure/persistence/dynamodb`. They use partition/sort keys plus GSIs to support CQRS read patterns.
- **In-memory and cache layers** exist for testing (`memory`) or future caching strategies (`cache`). Swap them via DI in tests or local experiments.
- **Schema utilities** in `infrastructure/persistence/schema` capture the expected table/index definitions to keep infra and code in sync.

## Messaging & Async Workflows

- All domain events are published via the `EventBridgePublisher`. When running locally (`cmd/api`/`cmd/worker`), a local dispatcher short-circuits EventBridge so handlers run in-process.
- The worker service wires the same handlers to process events continuously, leaving room for future saga orchestration or queue polling.
- `cmd/connect-node` demonstrates how Lambdas can use the DI container to re-use repositories/services for background tasks (auto-edge creation).
- Operation tracking is surfaced via the `OperationEventListener` and `application/ports/operation_store`, allowing REST clients to poll `/operations/{id}`.

## Development Workflow

- Format and lint before submitting changes:
  ```bash
  make fmt       # gofmt + goimports
  make lint      # golangci-lint with sensible defaults
  make vet       # go vet ./...
  ```
- Generate mocks when interfaces change:
  ```bash
  make mocks     # wraps go generate/mockgen
  ```
- Preferred loop for API work:
  ```bash
  ./dev.sh --service api         # hot reload via Air
  ./dev.sh --service worker      # background workers with reload
  ./dev.sh --service migrate up  # run CLI migrations when implemented
  ```
- To run a focused command or query handler during development:
  ```bash
  go test ./application/commands/handlers -run TestCreateNodeHandler
  go test ./domain/core/aggregates -run TestGraphLazyLoad
  ```

## Testing Strategy

- **Unit tests** live alongside code (e.g. `create_node_handler_test.go`) and can be run with `./test.sh --unit` or `go test` directly.
- **Integration tests** sit under `tests/integration` and focus on adapter correctness; run with `./test.sh --integration`.
- **End-to-end tests** in `tests/e2e` exercise REST endpoints and async flows; `./test.sh --e2e` will spin up required dependencies (Docker compose hooks are respected if `docker-compose.test.yml` exists).
- **Coverage reports** are generated to `coverage/coverage.html` when you pass `--coverage`. The script merges multiple coverage profiles automatically.
- Enable race detection with `./test.sh --race` or pass `--race` to `build.sh` for race-safe binaries.

## Observability

- Logging uses zap (`ProvideLogger`) and includes structured request context (request ID, user info). Log level is governed by `LOG_LEVEL` and environment.
- Metrics can be emitted to CloudWatch through `infrastructure/observability/metrics` when enabled.
- Tracing hooks are ready for OpenTelemetry exporters; toggle via `ENABLE_TRACING`.
- Error handling is centralised in `pkg/errors`, mapping domain/application errors to HTTP responses and capturing context for telemetry.

## Contributing & Conventions

- Keep the domain layer free of infrastructure dependencies; communicate across layers through interfaces defined in `application/ports`.
- Prefer command/query handlers over direct repository calls in delivery layers. Always go through the mediator (`container.Mediator`).
- Validate inputs early using `validation` helpers and return typed errors so the error handler can produce user-friendly responses.
- Document significant architectural changes in this README or `docs/` and provide matching tests.
- Follow the repo-wide style: `gofmt`, descriptive names, guard clauses, and thin handlers that delegate to domain logic.
- When adding new binaries under `cmd/`, wire them through `infrastructure/di` and add corresponding targets to `build.sh`/`Makefile` if needed.

## Troubleshooting

- **Wire errors**: run `make wire` to regenerate. Ensure the `wire` CLI is installed (`go install github.com/google/wire/cmd/wire@latest`).
- **Missing dependencies**: `make deps` will tidy and verify modules. If AWS SDK installation is slow, consider enabling module proxy caching.
- **Hot reload not working**: install `air` (`go install github.com/cosmtrek/air@latest`) or run `./dev.sh --no-reload` to fall back to plain `go run`.
- **DynamoDB access failures**: double-check AWS credentials or point the config at your local DynamoDB endpoint by exporting `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and overriding SDK endpoints if needed.

---

This backend is intentionally modular: new delivery mechanisms (GraphQL, gRPC), storage adapters, or background processors can be added by extending the DI container and reusing the domain/application core. Reach out in docs/ or open an issue when proposing large changes so we preserve the Clean Architecture boundaries.
