Dependency Injection (DI) with Wire

Purpose
- Make construction explicit and testable, without runtime reflection.
- Keep each constructor (“provider”) small and focused; Wire composes them.

Does provider order matter?
- No at runtime. Google Wire analyzes function signatures and generates the real initialization order in wire_gen.go. The order you see in wire.go is for readability only.

How we organize providers (leaf → higher-level)
1) Core
   - ProvideLogger, ProvideErrorHandler
2) AWS config and clients
   - ProvideAWSConfig → ProvideDynamoDBClient, ProvideEventBridgeClient, ProvideCloudWatchClient
3) Local stores
   - ProvideInMemoryCache, ProvideOperationStore
4) Infra utilities
   - ProvideDistributedRateLimiter, ProvideDistributedLock
5) Persistence
   - ProvideNodeRepository, ProvideEdgeRepository, ProvideGraphRepository
   - ProvideEventStore, ProvideUnitOfWork
6) Messaging and metrics
   - ProvideEventBus → ProvideEventPublisher
   - ProvideMetrics
7) Application services
   - ProvideGraphLazyService, ProvideGraphLoader, ProvideEdgeService
8) CQRS
   - ProvideCommandBus, ProvideQueryBus, ProvideMediator
9) Events / projections
   - ProvideEventHandlerRegistry, ProvideOperationEventListener, ProvideGraphStatsProjection
10) HTTP
   - ProvideAuthMiddleware
11) Container assembly
   - wire.Struct(new(Container), "*")

Reading flow
- Start at wire.go to see the grouped set. Then open wire_gen.go to view the exact call sequence Wire generated. Compare with providers.go for constructor details.

Notes
- Changing the order in wire.go will not change runtime init order; it only improves developer comprehension.
- If you introduce a new provider, place it in the lowest section that satisfies its dependencies (e.g., a new repository stays in 5, a new mediator behavior stays in 8).