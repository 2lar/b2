package abstractions

import (
	"context"
	"time"
)

// Repository provides a database-agnostic interface for data persistence
// This abstraction allows the application to switch between different databases
// without modifying business logic
type Repository interface {
	// Create stores a new item in the repository
	Create(ctx context.Context, item interface{}) error
	
	// Update modifies an existing item in the repository
	Update(ctx context.Context, item interface{}) error
	
	// Delete removes an item from the repository
	Delete(ctx context.Context, id string) error
	
	// FindByID retrieves an item by its unique identifier
	FindByID(ctx context.Context, id string) (interface{}, error)
	
	// FindAll retrieves all items matching the given criteria
	FindAll(ctx context.Context, criteria QueryCriteria) ([]interface{}, error)
	
	// Count returns the number of items matching the given criteria
	Count(ctx context.Context, criteria QueryCriteria) (int64, error)
	
	// Transaction executes multiple operations atomically
	Transaction(ctx context.Context, fn func(tx Repository) error) error
}

// QueryCriteria represents database-agnostic query parameters
type QueryCriteria struct {
	// Filters to apply to the query
	Filters []Filter
	
	// Sorting options
	Sort []SortOption
	
	// Pagination
	Limit  int
	Offset int
	
	// Include related entities
	Includes []string
}

// Filter represents a query filter condition
type Filter struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
}

// FilterOperator defines the type of comparison
type FilterOperator string

const (
	OpEqual              FilterOperator = "eq"
	OpNotEqual           FilterOperator = "ne"
	OpGreaterThan        FilterOperator = "gt"
	OpGreaterThanOrEqual FilterOperator = "gte"
	OpLessThan           FilterOperator = "lt"
	OpLessThanOrEqual    FilterOperator = "lte"
	OpIn                 FilterOperator = "in"
	OpNotIn              FilterOperator = "nin"
	OpContains           FilterOperator = "contains"
	OpStartsWith         FilterOperator = "starts_with"
	OpEndsWith           FilterOperator = "ends_with"
)

// SortOption defines sorting parameters
type SortOption struct {
	Field string
	Order SortOrder
}

// SortOrder defines the sorting direction
type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

// BatchOperation represents a batch operation request
type BatchOperation struct {
	Type BatchOperationType
	Item interface{}
}

// BatchOperationType defines the type of batch operation
type BatchOperationType string

const (
	BatchCreate BatchOperationType = "create"
	BatchUpdate BatchOperationType = "update"
	BatchDelete BatchOperationType = "delete"
)

// BatchRepository extends Repository with batch operations
type BatchRepository interface {
	Repository
	
	// BatchWrite performs multiple write operations efficiently
	BatchWrite(ctx context.Context, operations []BatchOperation) error
	
	// BatchRead retrieves multiple items by their IDs
	BatchRead(ctx context.Context, ids []string) ([]interface{}, error)
}

// TimestampedEntity represents an entity with timestamps
type TimestampedEntity struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// VersionedEntity represents an entity with optimistic locking
type VersionedEntity struct {
	Version int64
}

// SoftDeletableEntity represents an entity that supports soft deletes
type SoftDeletableEntity struct {
	DeletedAt *time.Time
}

// AuditableEntity combines common entity behaviors
type AuditableEntity struct {
	TimestampedEntity
	VersionedEntity
	SoftDeletableEntity
	CreatedBy string
	UpdatedBy string
}