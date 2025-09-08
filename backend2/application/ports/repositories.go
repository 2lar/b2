package ports

import (
	"context"

	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
	"backend2/domain/core/valueobjects"
	"backend2/domain/events"
)

// NodeRepository defines the interface for node persistence
// This is a port in hexagonal architecture - the domain doesn't know about the implementation
type NodeRepository interface {
	// Save persists a node (create or update)
	Save(ctx context.Context, node *entities.Node) error

	// GetByID retrieves a node by its ID
	GetByID(ctx context.Context, id valueobjects.NodeID) (*entities.Node, error)

	// GetByUserID retrieves all nodes for a user
	GetByUserID(ctx context.Context, userID string) ([]*entities.Node, error)

	// GetByGraphID retrieves all nodes for a specific graph
	GetByGraphID(ctx context.Context, graphID string) ([]*entities.Node, error)

	// Delete removes a node
	Delete(ctx context.Context, id valueobjects.NodeID) error

	// Search finds nodes matching the given criteria
	Search(ctx context.Context, criteria SearchCriteria) ([]*entities.Node, error)

	// BulkSave saves multiple nodes in a transaction
	BulkSave(ctx context.Context, nodes []*entities.Node) error

	// DeleteBatch removes multiple nodes in a batch operation
	DeleteBatch(ctx context.Context, nodeIDs []valueobjects.NodeID) error
}

// EdgeRepository defines the interface for edge persistence
type EdgeRepository interface {
	// Save persists an edge (graphID needed since Edge doesn't have it)
	Save(ctx context.Context, graphID string, edge *aggregates.Edge) error

	// GetByGraphID retrieves all edges for a graph
	GetByGraphID(ctx context.Context, graphID string) ([]*aggregates.Edge, error)

	// GetByNodeID retrieves all edges connected to a node
	GetByNodeID(ctx context.Context, nodeID string) ([]*aggregates.Edge, error)

	// Delete removes an edge
	Delete(ctx context.Context, graphID string, sourceID, targetID string) error

	// DeleteByNodeID removes all edges connected to a node
	DeleteByNodeID(ctx context.Context, graphID string, nodeID string) error

	// DeleteByNodeIDs removes all edges connected to multiple nodes
	DeleteByNodeIDs(ctx context.Context, graphID string, nodeIDs []string) error
}

// GraphRepository defines the interface for graph persistence
type GraphRepository interface {
	// Save persists a graph (create or update)
	Save(ctx context.Context, graph *aggregates.Graph) error

	// GetByID retrieves a graph by its ID
	GetByID(ctx context.Context, id aggregates.GraphID) (*aggregates.Graph, error)

	// GetByUserID retrieves all graphs for a user
	GetByUserID(ctx context.Context, userID string) ([]*aggregates.Graph, error)

	// GetUserDefaultGraph retrieves the user's default graph
	GetUserDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error)

	// GetOrCreateDefaultGraph gets or creates a default graph for a user
	GetOrCreateDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error)

	// CreateDefaultGraph creates a default graph for a user (deprecated - use GetOrCreateDefaultGraph)
	CreateDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error)

	// UpdateGraphMetadata updates the node and edge counts for a graph based on actual database state
	UpdateGraphMetadata(ctx context.Context, graphID string) error

	// Delete removes a graph and all its nodes
	Delete(ctx context.Context, id aggregates.GraphID) error
}

// EventStore defines the interface for event persistence
type EventStore interface {
	// SaveEvents persists domain events
	SaveEvents(ctx context.Context, events []events.DomainEvent) error

	// GetEvents retrieves events for an aggregate
	GetEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error)

	// GetEventsByType retrieves events of a specific type
	GetEventsByType(ctx context.Context, eventType string, limit int) ([]events.DomainEvent, error)

	// GetEventsAfter retrieves events after a specific timestamp
	GetEventsAfter(ctx context.Context, aggregateID string, version int) ([]events.DomainEvent, error)

	// DeleteEvents removes all events for an aggregate
	DeleteEvents(ctx context.Context, aggregateID string) error

	// DeleteEventsBatch removes all events for multiple aggregates
	DeleteEventsBatch(ctx context.Context, aggregateIDs []string) error
}

// UnitOfWork defines a transaction boundary for aggregate operations
type UnitOfWork interface {
	// Begin starts a new transaction
	Begin(ctx context.Context) error

	// Commit commits the transaction
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction
	Rollback() error

	// NodeRepository returns the node repository for this transaction
	NodeRepository() NodeRepository

	// EdgeRepository returns the edge repository for this transaction
	EdgeRepository() EdgeRepository

	// GraphRepository returns the graph repository for this transaction
	GraphRepository() GraphRepository
}

// SearchCriteria defines search parameters
type SearchCriteria struct {
	UserID    string
	Query     string
	Tags      []string
	Status    string
	Limit     int
	Offset    int
	OrderBy   string
	OrderDesc bool
}

// EventPublisher defines the interface for publishing domain events
type EventPublisher interface {
	// Publish sends a single event
	Publish(ctx context.Context, event events.DomainEvent) error

	// PublishBatch sends multiple events
	PublishBatch(ctx context.Context, events []events.DomainEvent) error
}

// EventBus defines the interface for publishing domain events
type EventBus interface {
	EventPublisher

	// Subscribe registers a handler for an event type
	Subscribe(eventType string, handler EventHandler) error

	// Unsubscribe removes a handler
	Unsubscribe(eventType string, handler EventHandler) error
}

// EventHandler defines the interface for handling domain events
type EventHandler interface {
	// Handle processes an event
	Handle(ctx context.Context, event events.DomainEvent) error

	// CanHandle checks if this handler can process the event
	CanHandle(eventType string) bool
}

// Cache defines the interface for caching
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) (interface{}, bool)

	// Set stores a value in cache with TTL in seconds
	Set(ctx context.Context, key string, value interface{}, ttl int) error

	// Delete removes a value from cache
	Delete(ctx context.Context, key string) error

	// Clear removes all values from cache
	Clear(ctx context.Context) error
}
