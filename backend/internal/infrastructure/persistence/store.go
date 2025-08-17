// Package persistence provides database-agnostic persistence interfaces and implementations.
// This abstraction layer allows easy migration between different database technologies.
package persistence

import (
	"context"
	"time"
)

// Key represents a unique identifier for a record in the store.
type Key struct {
	PartitionKey string            // Primary partition key (e.g., USER#123#NODE#456)
	SortKey      string            // Sort key for range queries (e.g., METADATA#v0)
	Attributes   map[string]string // Additional key attributes for composite keys
}

// Record represents a data record in the store.
type Record struct {
	Key        Key                    // Unique identifier
	Data       map[string]interface{} // Record data
	Version    int64                  // Optimistic locking version
	CreatedAt  time.Time              // Creation timestamp
	UpdatedAt  time.Time              // Last update timestamp
	ExpiresAt  *time.Time             // Optional expiration time (TTL)
}

// Query represents a query operation against the store.
type Query struct {
	PartitionKey   string                 // Required partition key
	SortKeyPrefix  *string                // Optional sort key prefix for range queries
	FilterExpr     *string                // Optional filter expression
	IndexName      *string                // Optional index name for GSI queries
	Attributes     map[string]interface{} // Query parameters
	Limit          *int32                 // Optional result limit
	LastEvaluated  map[string]interface{} // For pagination
	ScanForward    *bool                  // Sort direction (true=ascending, false=descending)
}

// Operation represents a single operation in a transaction.
type Operation struct {
	Type   OperationType              // Operation type (Put, Delete, Update)
	Key    Key                        // Target key
	Data   map[string]interface{}     // Data for Put/Update operations
	ConditionExpr *string            // Optional condition expression
	Attributes map[string]interface{} // Operation parameters
}

// OperationType defines the type of operation in a transaction.
type OperationType string

const (
	OperationTypePut    OperationType = "PUT"
	OperationTypeDelete OperationType = "DELETE"
	OperationTypeUpdate OperationType = "UPDATE"
)

// QueryResult represents the result of a query operation.
type QueryResult struct {
	Records       []Record               // Retrieved records
	LastEvaluated map[string]interface{} // For pagination
	Count         int32                  // Number of records returned
	ScannedCount  int32                  // Number of records scanned
}

// Store abstracts the underlying database technology.
// This interface allows the repository layer to work with different databases
// (DynamoDB, PostgreSQL, MongoDB, etc.) without changing business logic.
type Store interface {
	// Basic CRUD operations
	Get(ctx context.Context, key Key) (*Record, error)
	Put(ctx context.Context, record Record) error
	Delete(ctx context.Context, key Key) error
	Update(ctx context.Context, key Key, updates map[string]interface{}, conditionExpr *string) error

	// Query operations
	Query(ctx context.Context, query Query) (*QueryResult, error)
	Scan(ctx context.Context, query Query) (*QueryResult, error)

	// Batch operations for performance
	BatchGet(ctx context.Context, keys []Key) ([]Record, error)
	BatchPut(ctx context.Context, records []Record) error
	BatchDelete(ctx context.Context, keys []Key) error

	// Transaction operations for consistency
	Transaction(ctx context.Context, operations []Operation) error

	// Health and diagnostics
	HealthCheck(ctx context.Context) error
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// StoreConfig holds configuration for store implementations.
type StoreConfig struct {
	TableName       string                 // Primary table name
	IndexNames      map[string]string      // Named indexes (GSI1, GSI2, etc.)
	TimeoutMs       int32                  // Operation timeout in milliseconds
	RetryAttempts   int                    // Number of retry attempts
	ConsistentRead  bool                   // Use consistent reads (for eventually consistent stores)
	Attributes      map[string]interface{} // Store-specific configuration (supports any type)
}

// StoreMetrics provides observability into store operations.
type StoreMetrics struct {
	OperationCount    map[string]int64  // Count by operation type
	LatencyMs         map[string]int64  // Latency by operation type
	ErrorCount        map[string]int64  // Errors by type
	LastOperation     time.Time         // Timestamp of last operation
	ConnectionStatus  string            // Current connection status
}

// StoreFactory creates store instances based on configuration.
type StoreFactory interface {
	CreateStore(config StoreConfig) (Store, error)
	GetSupportedTypes() []string
}