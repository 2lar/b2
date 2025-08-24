// Package dynamodb provides DynamoDB implementations of repository interfaces for the Brain2 application.
//
// This package demonstrates enterprise-grade DynamoDB patterns including:
//   - Single Table Design for optimal performance
//   - CQRS implementation with separate read/write optimizations
//   - Advanced query patterns with GSIs
//   - Optimistic locking for concurrent updates
//   - Batch operations for bulk data handling
//   - Transaction support for consistency
//
// # Architecture Overview
//
// The DynamoDB implementation follows AWS best practices:
//   - **Single Table Design**: All entities in one table with composite keys
//   - **Access Patterns First**: Schema designed around query patterns
//   - **Efficient Queries**: Minimize RCU/WCU consumption
//   - **Scalable Design**: Partition keys distribute load evenly
//
// # Table Schema Design
//
// ## Primary Key Structure
//
//	PK (Partition Key): "USER#{userID}" | "CATEGORY#{categoryID}"
//	SK (Sort Key): "NODE#{nodeID}" | "EDGE#{sourceID}#{targetID}"
//
// ## Global Secondary Index (GSI1)
//
//	GSI1PK: "NODE#{nodeID}" | "USER#{userID}#NODES"
//	GSI1SK: "CREATED#{timestamp}" | "CATEGORY#{categoryID}"
//
// This design enables efficient queries for:
//   - User's nodes: PK = "USER#{userID}", SK begins_with "NODE#"
//   - Node details: PK = "USER#{userID}", SK = "NODE#{nodeID}"
//   - Nodes by category: GSI1PK = "USER#{userID}#NODES", GSI1SK begins_with "CATEGORY#"
//   - Recent nodes: GSI1PK = "USER#{userID}#NODES", GSI1SK > "CREATED#{timestamp}"
//
// # Repository Implementations
//
// ## Unified Repository (unified_repository.go)
//
// Implements all repository interfaces in a single class:
//   - Reduces code duplication
//   - Shares connection pools
//   - Consistent error handling
//   - Unified transaction support
//
// Usage example:
//
//	repo := dynamodb.NewUnifiedRepository(client, config)
//	node, err := repo.FindNodeByID(ctx, userID, nodeID)
//
// ## Specialized Repositories
//
// Individual repositories for specific entities:
//   - **node_repository.go**: Node CRUD operations with advanced queries
//   - **edge_repository.go**: Graph edge management with path queries
//   - **category_repository.go**: Hierarchical category operations
//
// ## CQRS Pattern Implementation
//
// Each repository implements both Reader and Writer interfaces:
//
//	// Read operations - optimized for query performance
//	type NodeReader interface {
//		FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error)
//		FindNodes(ctx context.Context, query NodeQuery) ([]*node.Node, error)
//	}
//
//	// Write operations - optimized for consistency
//	type NodeWriter interface {
//		CreateNode(ctx context.Context, node *node.Node) error
//		UpdateNode(ctx context.Context, node *node.Node) error
//		DeleteNode(ctx context.Context, userID, nodeID string) error
//	}
//
// # Advanced Query Patterns
//
// ## Query Builder (query_builder.go)
//
// Fluent interface for building complex DynamoDB queries:
//
//	query := NewQueryBuilder().
//		ForUser(userID).
//		WithEntity("NODE").
//		FilterByDateRange(startDate, endDate).
//		SortBy("created_at", "DESC").
//		Limit(50).
//		Build()
//
// ## Batch Operations (refactored_batch_operations.go)
//
// Efficient bulk operations with automatic batching:
//
//	batchWriter := repo.NewBatchWriter()
//	for _, node := range nodes {
//		batchWriter.Add(node)
//	}
//	results, err := batchWriter.Execute(ctx)
//
// ## Transaction Support (unit_of_work.go)
//
// ACID transactions across multiple entities:
//
//	uow, err := repo.CreateUnitOfWork(ctx)
//	uow.Nodes().Create(ctx, node)
//	uow.Edges().Create(ctx, edge)
//	err = uow.Commit(ctx) // Atomic operation
//
// # Performance Optimizations
//
// ## Connection Pooling
//
// Optimized AWS SDK client configuration:
//   - Connection reuse across Lambda invocations
//   - Configurable pool sizes
//   - Health check monitoring
//   - Regional optimization
//
//	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
//		o.HTTPClient = &http.Client{
//			Transport: &http.Transport{
//				MaxIdleConns:        100,
//				MaxIdleConnsPerHost: 10,
//				IdleConnTimeout:     90 * time.Second,
//			},
//		}
//	})
//
// ## Query Optimization
//
// Strategies for reducing costs and latency:
//   - **Projection expressions**: Only fetch required attributes
//   - **Filter expressions**: Server-side filtering
//   - **Parallel scans**: For bulk operations
//   - **Consistent reads**: Only when necessary
//
// ## Caching Integration
//
// Repository decorators provide transparent caching:
//
//	cachedRepo := cache.NewCachingRepository(baseRepo, cacheClient)
//	// Automatic cache-aside pattern with TTL
//
// # Error Handling
//
// ## AWS-Specific Errors
//
// Proper handling of DynamoDB-specific conditions:
//   - **ConditionalCheckFailedException**: Optimistic locking conflicts
//   - **ProvisionedThroughputExceededException**: Throttling with backoff
//   - **ResourceNotFoundException**: Missing table/index
//   - **ValidationException**: Malformed requests
//
// Example error handling:
//
//	if err != nil {
//		var ccfe *types.ConditionalCheckFailedException
//		if errors.As(err, &ccfe) {
//			return domain.NewOptimisticLockError("Node was modified by another process")
//		}
//		return errors.Wrap(err, "failed to update node")
//	}
//
// ## Retry Logic
//
// Exponential backoff with jitter for transient failures:
//
//	retryConfig := retry.Config{
//		MaxAttempts: 3,
//		InitialDelay: 100 * time.Millisecond,
//		MaxDelay: 5 * time.Second,
//		Multiplier: 2.0,
//	}
//
// # Data Modeling Best Practices
//
// ## Attribute Design
//
// Consistent naming and typing:
//   - **PascalCase** for attribute names
//   - **ISO 8601** timestamps
//   - **Strongly typed** value objects
//   - **Normalized** enum values
//
// ## Index Strategy
//
// Careful GSI design for access patterns:
//   - **Sparse indexes** for optional attributes
//   - **Overloaded GSIs** for multiple query patterns
//   - **Key compression** for storage efficiency
//   - **Hot partition avoidance**
//
// ## Version Control
//
// Optimistic locking implementation:
//
//	item["Version"] = &types.AttributeValueMemberN{Value: strconv.Itoa(node.Version + 1)}
//	condition := "Version = :currentVersion"
//	conditionValues[":currentVersion"] = &types.AttributeValueMemberN{Value: strconv.Itoa(node.Version)}
//
// # Testing Strategies
//
// ## Local DynamoDB
//
// Development and testing with DynamoDB Local:
//
//	// docker run -p 8000:8000 amazon/dynamodb-local
//	cfg.Endpoint = "http://localhost:8000"
//	cfg.Region = "us-east-1"
//
// ## Integration Tests
//
// Tests that verify end-to-end functionality:
//
//	func TestNodeRepository_CreateAndFind(t *testing.T) {
//		repo := setupTestRepository(t)
//		defer teardownTestRepository(t, repo)
//		
//		node := createTestNode(t)
//		err := repo.CreateNode(ctx, node)
//		assert.NoError(t, err)
//		
//		found, err := repo.FindNodeByID(ctx, node.UserID, node.ID)
//		assert.NoError(t, err)
//		assert.Equal(t, node.Content, found.Content)
//	}
//
// ## Mock Repositories
//
// In-memory implementations for unit testing:
//
//	mockRepo := &MockNodeRepository{
//		nodes: make(map[string]*node.Node),
//	}
//
// # Migration Patterns
//
// ## Schema Evolution
//
// Handling attribute additions and changes:
//   - **Backward compatibility** with missing attributes
//   - **Lazy migration** during read operations
//   - **Batch migration** scripts for breaking changes
//   - **Version markers** for tracking schema versions
//
// ## Data Migration
//
// Safe migration strategies:
//
//	func migrateNodeSchema(ctx context.Context, repo Repository) error {
//		// Read with old schema
//		// Transform data
//		// Write with new schema
//		// Verify migration
//	}
//
// # Monitoring and Observability
//
// ## CloudWatch Metrics
//
// Key metrics for monitoring:
//   - **RequestLatency**: P50, P95, P99 latencies
//   - **ConsumedCapacity**: RCU/WCU utilization
//   - **Throttling**: Rate of throttled requests
//   - **Errors**: Error rates by operation type
//
// ## Custom Metrics
//
// Application-specific measurements:
//
//	metrics.RecordRepositoryOperation(ctx, "CreateNode", duration, err)
//	metrics.RecordCacheHit(ctx, "NodeByID", hit)
//
// ## Distributed Tracing
//
// Trace spans for all repository operations:
//
//	ctx, span := tracer.Start(ctx, "NodeRepository.CreateNode",
//		trace.WithAttributes(
//			attribute.String("dynamodb.table", tableName),
//			attribute.String("node.id", nodeID),
//		),
//	)
//	defer span.End()
//
// # Security Considerations
//
// ## IAM Policies
//
// Principle of least privilege:
//
//	{
//		"Effect": "Allow",
//		"Action": [
//			"dynamodb:GetItem",
//			"dynamodb:PutItem",
//			"dynamodb:UpdateItem",
//			"dynamodb:DeleteItem",
//			"dynamodb:Query"
//		],
//		"Resource": "arn:aws:dynamodb:region:account:table/brain2-*"
//	}
//
// ## Data Encryption
//
// Encryption at rest and in transit:
//   - **AWS KMS** encryption for tables
//   - **TLS** for all connections
//   - **Attribute-level encryption** for sensitive data
//   - **Key rotation** policies
//
// ## Access Logging
//
// Audit trail for all data access:
//   - **CloudTrail** for API calls
//   - **VPC Flow Logs** for network access
//   - **Application logs** for business operations
//
// # Common Patterns
//
// ## Single Table Design
//
//	PK: Entity identifier
//	SK: Sort key for range queries
//	Type: Entity type discriminator
//	Data: Entity-specific attributes
//	GSI1PK/GSI1SK: Additional access patterns
//
// ## Event Sourcing
//
//	type EventStore interface {
//		AppendEvents(ctx context.Context, streamID string, events []DomainEvent) error
//		GetEvents(ctx context.Context, streamID string, fromVersion int) ([]DomainEvent, error)
//	}
//
// ## Saga Pattern
//
//	type SagaState struct {
//		ID string
//		Step int
//		Data map[string]interface{}
//		CompletedSteps []string
//		CompensationData map[string]interface{}
//	}
//
// This package serves as a comprehensive example of how to properly implement
// DynamoDB repositories in a Clean Architecture application, demonstrating
// both basic patterns and advanced techniques for production systems.
package dynamodb