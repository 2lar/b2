// Package interfaces defines contracts for dependency injection with compile-time safety.
// These interfaces ensure loose coupling and enable easy testing through mocking.
package interfaces

import (
	"context"
	"net/http"
	"time"

	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
)

// ============================================================================
// INFRASTRUCTURE INTERFACES
// ============================================================================

// Logger defines the logging contract.
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	With(fields ...interface{}) Logger
	Sync() error
}

// EventBus defines the event publishing contract.
type EventBus interface {
	Publish(ctx context.Context, event shared.DomainEvent) error
	PublishBatch(ctx context.Context, events []shared.DomainEvent) error
}

// Cache defines the caching contract.
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// ============================================================================
// TRANSACTION INTERFACES
// ============================================================================

// UnitOfWork represents a transactional boundary for aggregate operations.
// It provides access to repositories and manages domain events within a transaction.
type UnitOfWork interface {
	// Transaction control
	Begin(ctx context.Context) error
	Commit() error
	Rollback() error
	
	// Repository access
	Nodes() repository.NodeRepository
	Edges() repository.EdgeRepository
	Categories() repository.CategoryRepository
	
	// Event publishing
	PublishEvent(event shared.DomainEvent)
}

// UnitOfWorkFunc is the function signature for unit of work operations.
// This replaces the TransactionExecutor pattern with UnitOfWork for better DDD alignment.
type UnitOfWorkFunc func(ctx context.Context, uow UnitOfWork) error

// Transaction represents a database transaction.
type Transaction interface {
	// Repository access
	Nodes() repository.NodeRepository
	Edges() repository.EdgeRepository
	Categories() repository.CategoryRepository
	Keywords() repository.KeywordRepository
	Graph() repository.GraphRepository
	
	// Event management
	RecordEvent(event shared.DomainEvent)
	RecordEvents(events ...shared.DomainEvent)
}

// ============================================================================
// REPOSITORY INTERFACES
// ============================================================================

// AllRepositories bundles all repository interfaces.
type AllRepositories interface {
	Nodes() repository.NodeRepository
	Edges() repository.EdgeRepository
	Categories() repository.CategoryRepository
	Keywords() repository.KeywordRepository
	Graph() repository.GraphRepository
}

// ============================================================================
// SERVICE INTERFACES
// ============================================================================

// NodeService defines the contract for node business logic.
type NodeService interface {
	CreateNode(ctx context.Context, userID, content, title string, tags []string) (*node.Node, error)
	UpdateNode(ctx context.Context, userID, nodeID, content, title string, tags []string) (*node.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error
	GetNode(ctx context.Context, userID, nodeID string) (*node.Node, error)
	ListNodes(ctx context.Context, userID string, limit int, cursor string) ([]*node.Node, string, error)
	BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted, failed []string, err error)
}

// CategoryService defines the contract for category business logic.
type CategoryService interface {
	CreateCategory(ctx context.Context, userID, name, description string, parentID *string) (*category.Category, error)
	UpdateCategory(ctx context.Context, userID, categoryID, name, description string) (*category.Category, error)
	DeleteCategory(ctx context.Context, userID, categoryID string) error
	GetCategory(ctx context.Context, userID, categoryID string) (*category.Category, error)
	ListCategories(ctx context.Context, userID string) ([]*category.Category, error)
	AssignNodeToCategory(ctx context.Context, userID, nodeID, categoryID string) error
	RemoveNodeFromCategory(ctx context.Context, userID, nodeID, categoryID string) error
}

// EdgeService defines the contract for edge business logic.
type EdgeService interface {
	CreateEdge(ctx context.Context, userID, sourceNodeID, targetNodeID string, weight float64) (*edge.Edge, error)
	DeleteEdge(ctx context.Context, userID, edgeID string) error
	GetEdgesBetweenNodes(ctx context.Context, userID, sourceNodeID, targetNodeID string) ([]*edge.Edge, error)
	GetNodeEdges(ctx context.Context, userID, nodeID string) ([]*edge.Edge, error)
}

// GraphService defines the contract for graph operations.
type GraphService interface {
	GetGraphData(ctx context.Context, userID string, nodeIDs []string) (*shared.Graph, error)
	GetSubgraph(ctx context.Context, userID string, centerNodeID string, depth int) (*shared.Graph, error)
	GetConnectedComponents(ctx context.Context, userID string) ([]shared.Graph, error)
}

// AllServices bundles all service interfaces.
type AllServices interface {
	NodeService() NodeService
	CategoryService() CategoryService
	EdgeService() EdgeService
	GraphService() GraphService
}

// ============================================================================
// HANDLER INTERFACES
// ============================================================================

// Handler defines the HTTP handler contract.
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// MemoryHandler defines the contract for memory-related HTTP endpoints.
type MemoryHandler interface {
	CreateNode(w http.ResponseWriter, r *http.Request)
	GetNode(w http.ResponseWriter, r *http.Request)
	UpdateNode(w http.ResponseWriter, r *http.Request)
	DeleteNode(w http.ResponseWriter, r *http.Request)
	ListNodes(w http.ResponseWriter, r *http.Request)
	BulkDeleteNodes(w http.ResponseWriter, r *http.Request)
	GetGraphData(w http.ResponseWriter, r *http.Request)
}

// CategoryHandler defines the contract for category-related HTTP endpoints.
type CategoryHandler interface {
	CreateCategory(w http.ResponseWriter, r *http.Request)
	GetCategory(w http.ResponseWriter, r *http.Request)
	UpdateCategory(w http.ResponseWriter, r *http.Request)
	DeleteCategory(w http.ResponseWriter, r *http.Request)
	ListCategories(w http.ResponseWriter, r *http.Request)
	AssignNodeToCategory(w http.ResponseWriter, r *http.Request)
	RemoveNodeFromCategory(w http.ResponseWriter, r *http.Request)
	GetNodeCategories(w http.ResponseWriter, r *http.Request)
	GetNodesInCategory(w http.ResponseWriter, r *http.Request)
	CategorizeNode(w http.ResponseWriter, r *http.Request)
}

// HealthHandler defines the contract for health check endpoints.
type HealthHandler interface {
	Check(w http.ResponseWriter, r *http.Request)
	Ready(w http.ResponseWriter, r *http.Request)
}

// AllHandlers bundles all handler interfaces.
type AllHandlers interface {
	MemoryHandler() MemoryHandler
	CategoryHandler() CategoryHandler
	HealthHandler() HealthHandler
}

// ============================================================================
// AWS CLIENT INTERFACES
// ============================================================================

// DynamoDBClient defines the contract for DynamoDB operations.
type DynamoDBClient interface {
	// Define only the methods we actually use
	PutItem(ctx context.Context, params interface{}) (interface{}, error)
	GetItem(ctx context.Context, params interface{}) (interface{}, error)
	DeleteItem(ctx context.Context, params interface{}) (interface{}, error)
	Query(ctx context.Context, params interface{}) (interface{}, error)
	TransactWriteItems(ctx context.Context, params interface{}) (interface{}, error)
}

// EventBridgeClient defines the contract for EventBridge operations.
type EventBridgeClient interface {
	PutEvents(ctx context.Context, params interface{}) (interface{}, error)
}