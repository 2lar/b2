// Package ports defines the interfaces (ports) that the application core uses
// to interact with external systems. These ports enable the hexagonal architecture
// by decoupling the core business logic from infrastructure concerns.
package ports

import (
	"context"
	"time"
	
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/specifications"
)

// NodeRepository is the port for node persistence operations.
// This interface is implemented by adapters in the infrastructure layer.
type NodeRepository interface {
	// Save persists a node aggregate with its events
	Save(ctx context.Context, aggregate *node.Aggregate) error
	
	// FindByID retrieves a node by its ID
	FindByID(ctx context.Context, id string) (*node.Aggregate, error)
	
	// GetByID is an alias for FindByID for convenience
	GetByID(ctx context.Context, id string) (*node.Aggregate, error)
	
	// FindBySpecification retrieves nodes matching a specification
	FindBySpecification(ctx context.Context, spec specifications.Specification[*node.Aggregate]) ([]*node.Aggregate, error)
	
	// Delete removes a node
	Delete(ctx context.Context, id string) error
	
	// Exists checks if a node exists
	Exists(ctx context.Context, id string) (bool, error)
	
	// Count returns the total number of nodes matching a specification
	Count(ctx context.Context, spec specifications.Specification[*node.Aggregate]) (int64, error)
}

// EdgeRepository is the port for edge persistence operations
type EdgeRepository interface {
	// CreateEdge creates a new edge between nodes
	CreateEdge(ctx context.Context, edge *Edge) error
	
	// DeleteEdge removes an edge
	DeleteEdge(ctx context.Context, sourceID, targetID string) error
	
	// FindEdgesByNode finds all edges connected to a node
	FindEdgesByNode(ctx context.Context, nodeID string) ([]Edge, error)
	
	// GetEdge retrieves a specific edge
	GetEdge(ctx context.Context, sourceID, targetID string) (*Edge, error)
	
	// UpdateEdge updates an edge
	UpdateEdge(ctx context.Context, edge *Edge) error
	
	// UpdateStrength updates the strength of an edge
	UpdateStrength(ctx context.Context, sourceID, targetID string, strength float64) error
}

// Edge represents a connection between nodes
type Edge struct {
	ID       string
	SourceID string
	TargetID string
	Type     string
	Weight   float64
	Strength float64
	UserID   string
	Metadata map[string]interface{}
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CategoryRepository is the port for category persistence operations
type CategoryRepository interface {
	// Save persists a category
	Save(ctx context.Context, category Category) error
	
	// FindByID retrieves a category by ID
	FindByID(ctx context.Context, id string) (*Category, error)
	
	// FindAll retrieves all categories for a user
	FindAll(ctx context.Context, userID string) ([]Category, error)
	
	// Delete removes a category
	Delete(ctx context.Context, id string) error
	
	// AssignNode assigns a node to a category
	AssignNode(ctx context.Context, nodeID, categoryID string) error
	
	// UnassignNode removes a node from a category
	UnassignNode(ctx context.Context, nodeID, categoryID string) error
}

// Category represents a category entity
type Category struct {
	ID          string
	UserID      string
	Name        string
	Description string
	ParentID    string
}

// EventStore is the port for event persistence
type EventStore interface {
	// SaveEvents persists events for an aggregate
	SaveEvents(ctx context.Context, aggregateID string, events []events.DomainEvent, expectedVersion int64) error
	
	// LoadEvents retrieves all events for an aggregate
	LoadEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error)
	
	// LoadEventsAfterVersion retrieves events after a specific version
	LoadEventsAfterVersion(ctx context.Context, aggregateID string, version int64) ([]events.DomainEvent, error)
	
	// GetSnapshot retrieves the latest snapshot for an aggregate
	GetSnapshot(ctx context.Context, aggregateID string) (*events.AggregateSnapshot, error)
	
	// SaveSnapshot persists a snapshot
	SaveSnapshot(ctx context.Context, snapshot *events.AggregateSnapshot) error
}

// Snapshot represents a point-in-time state snapshot
type Snapshot struct {
	AggregateID string
	Version     int64
	Data        []byte
	CreatedAt   int64
}

// UnitOfWork manages transactional boundaries
type UnitOfWork interface {
	// Begin starts a new unit of work
	Begin(ctx context.Context) error
	
	// Commit commits all changes
	Commit() error
	
	// Rollback rolls back all changes
	Rollback() error
	
	// NodeRepository returns the node repository for this unit of work
	NodeRepository() NodeRepository
	
	// EdgeRepository returns the edge repository for this unit of work
	EdgeRepository() EdgeRepository
	
	// EventStore returns the event store for this unit of work
	EventStore() EventStore
}

// UnitOfWorkFactory creates new units of work
type UnitOfWorkFactory interface {
	// Create creates a new unit of work
	Create(ctx context.Context) (UnitOfWork, error)
}

// ReadModelProjection is the port for updating read models from events
type ReadModelProjection interface {
	// Handle processes an event to update read models
	Handle(ctx context.Context, event events.DomainEvent) error
	
	// Reset clears and rebuilds the projection
	Reset(ctx context.Context) error
	
	// GetCheckpoint returns the last processed event position
	GetCheckpoint(ctx context.Context) (int64, error)
	
	// SaveCheckpoint saves the processing checkpoint
	SaveCheckpoint(ctx context.Context, position int64) error
}

// QueryRepository is the port for read model queries (CQRS read side)
type QueryRepository interface {
	// FindNodesByUser retrieves all nodes for a user (denormalized view)
	FindNodesByUser(ctx context.Context, userID string, options QueryOptions) (*NodeQueryResult, error)
	
	// SearchNodes performs full-text search on nodes
	SearchNodes(ctx context.Context, query string, options QueryOptions) (*NodeQueryResult, error)
	
	// GetNodeGraph retrieves the graph structure around a node
	GetNodeGraph(ctx context.Context, nodeID string, depth int) (*GraphView, error)
	
	// GetStatistics retrieves usage statistics
	GetStatistics(ctx context.Context, userID string) (*Statistics, error)
}

// QueryOptions contains options for queries
type QueryOptions struct {
	Offset  int
	Limit   int
	SortBy  string
	Order   string
	Filters map[string]interface{}
}

// NodeQueryResult contains query results with pagination
type NodeQueryResult struct {
	Nodes      []NodeView
	TotalCount int64
	HasMore    bool
}

// NodeView is a denormalized read model for nodes
type NodeView struct {
	ID              string
	UserID          string
	Content         string
	Title           string
	Tags            []string
	Categories      []string
	ConnectionCount int
	CreatedAt       int64
	UpdatedAt       int64
}

// GraphView represents a graph structure
type GraphView struct {
	Nodes []NodeView
	Edges []EdgeView
}

// EdgeView is a read model for edges
type EdgeView struct {
	SourceID string
	TargetID string
	Strength float64
}

// Statistics contains usage statistics
type Statistics struct {
	TotalNodes       int64
	TotalConnections int64
	TotalCategories  int64
	RecentActivity   []Activity
}

// Activity represents a recent activity
type Activity struct {
	Type      string
	NodeID    string
	Timestamp int64
}