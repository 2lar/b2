package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend2/application/commands"
	"backend2/application/ports"
	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
	"backend2/domain/core/valueobjects"
	"backend2/infrastructure/persistence/dynamodb"
)

// Logger interface for flexible logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// CreateNodeOrchestrator orchestrates the complex node creation process
// It breaks down the monolithic CreateNodeCommand into smaller, focused operations
type CreateNodeOrchestrator struct {
	uow             ports.UnitOfWork
	nodeRepo        ports.NodeRepository
	graphRepo       ports.GraphRepository
	edgeRepo        ports.EdgeRepository
	eventPublisher  ports.EventPublisher
	distributedLock *dynamodb.DistributedLock
	logger          Logger
}

// NewCreateNodeOrchestrator creates a new orchestrator instance
func NewCreateNodeOrchestrator(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	eventPublisher ports.EventPublisher,
	distributedLock *dynamodb.DistributedLock,
	logger Logger,
) *CreateNodeOrchestrator {
	return &CreateNodeOrchestrator{
		uow:             uow,
		nodeRepo:        nodeRepo,
		graphRepo:       graphRepo,
		edgeRepo:        edgeRepo,
		eventPublisher:  eventPublisher,
		distributedLock: distributedLock,
		logger:          logger,
	}
}

// Handle orchestrates the node creation process
func (o *CreateNodeOrchestrator) Handle(ctx context.Context, cmd commands.CreateNodeCommand) (*entities.Node, error) {
	// Validate command
	if err := o.validateCommand(cmd); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	// Start unit of work transaction
	if err := o.uow.Begin(ctx); err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer o.uow.Rollback() // Will be no-op if commit succeeds

	// Step 1: Ensure graph exists or create it
	graph, err := o.ensureGraphExists(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure graph exists: %w", err)
	}

	// Step 2: Create the node
	node, err := o.createNode(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Step 3: Set the GraphID on the node BEFORE saving
	// This is critical - the repository requires nodes to have a GraphID before saving
	node.SetGraphID(string(graph.ID()))

	// Step 4: Save the node now that it has a GraphID
	if err := o.saveNodeWithUoW(ctx, node); err != nil {
		return nil, fmt.Errorf("failed to save node: %w", err)
	}

	// Step 5: Add node to graph first (before creating edges)
	if err := graph.AddNode(node); err != nil {
		return nil, fmt.Errorf("failed to add node to graph: %w", err)
	}

	// Step 6: Edge creation now happens asynchronously via EventBridge
	// The NodeCreated event will trigger the connect-node Lambda
	// which will discover and create edges based on similarity
	// This prevents race conditions when multiple nodes are created rapidly

	// Step 7: Save the graph with all nodes and edges
	if err := o.saveGraphWithUoW(ctx, graph); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}

	// Commit transaction
	if err := o.uow.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Publish domain events after successful commit
	events := node.GetUncommittedEvents()
	events = append(events, graph.GetUncommittedEvents()...)

	if len(events) > 0 {
		if err := o.eventPublisher.PublishBatch(ctx, events); err != nil {
			// Log error but don't fail - events can be retried
			o.logger.Error("Failed to publish domain events",
				"error", err,
				"eventCount", len(events),
				"nodeID", node.ID().String(),
			)
		} else {
			// Mark events as committed after successful publishing
			node.MarkEventsAsCommitted()
			graph.MarkEventsAsCommitted()
		}
	}

	// Get node count safely for logging
	nodes, nodeErr := graph.Nodes()
	nodeCount := 0
	if nodeErr != nil {
		nodeCount = -1 // Indicate large graph
	} else {
		nodeCount = len(nodes)
	}

	// Update graph metadata after transaction commits
	// This ensures we count the actual committed data
	if err := o.graphRepo.UpdateGraphMetadata(ctx, string(graph.ID())); err != nil {
		o.logger.Error("Failed to update graph metadata after commit",
			"error", err,
			"graphID", string(graph.ID()),
			"nodeCount", nodeCount,
			"edgeCount", len(graph.GetEdges()),
		)
		// Don't fail the operation if metadata update fails
	}

	o.logger.Info("Node created successfully",
		"nodeID", node.ID().String(),
		"graphID", string(graph.ID()),
		"userID", cmd.UserID,
		"totalGraphNodes", nodeCount,
	)

	return node, nil
}

// validateCommand validates the create node command
func (o *CreateNodeOrchestrator) validateCommand(cmd commands.CreateNodeCommand) error {
	if cmd.UserID == "" {
		return errors.New("user ID is required")
	}

	if cmd.Title == "" {
		return errors.New("title is required")
	}

	if len(cmd.Title) > 255 {
		return errors.New("title exceeds maximum length of 255 characters")
	}

	if len(cmd.Content) > 50000 {
		return errors.New("content exceeds maximum length of 50000 characters")
	}

	// Validate position bounds
	if cmd.X < -10000 || cmd.X > 10000 ||
		cmd.Y < -10000 || cmd.Y > 10000 ||
		cmd.Z < -10000 || cmd.Z > 10000 {
		return errors.New("position coordinates out of bounds")
	}

	return nil
}

// ensureGraphExists gets or creates a graph for the user with distributed locking to prevent race conditions
func (o *CreateNodeOrchestrator) ensureGraphExists(ctx context.Context, userID string) (*aggregates.Graph, error) {
	// Try to get the user's default graph first (no lock needed for read)
	graph, err := o.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err == nil {
		// Graph exists, return it
		return graph, nil
	}

	// Graph doesn't exist, need to create it with distributed locking
	// to prevent race conditions when multiple requests try to create the same graph
	lockResource := fmt.Sprintf("default_graph_creation_%s", userID)
	lockDuration := 30 * time.Second // Lock for up to 30 seconds
	lockTimeout := 5 * time.Second   // Wait up to 5 seconds to acquire lock

	lock, err := o.distributedLock.TryAcquireLock(ctx, lockResource, userID, lockDuration, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for graph creation: %w", err)
	}
	defer func() {
		if releaseErr := lock.Release(ctx); releaseErr != nil {
			o.logger.Error("Failed to release distributed lock",
				"resource", lockResource,
				"error", releaseErr,
			)
		}
	}()

	// Double-check if graph was created by another process while we were waiting for the lock
	graph, err = o.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err == nil {
		o.logger.Debug("Default graph found after acquiring lock (created by another process)", "userID", userID)
		return graph, nil
	}

	// Create a new default graph for the user
	o.logger.Debug("Creating new default graph for user with distributed lock", "userID", userID)

	graph, err = aggregates.NewGraph(userID, "Default Graph")
	if err != nil {
		return nil, fmt.Errorf("failed to create new graph: %w", err)
	}

	// Don't save the graph here - it will be saved once in the main transaction
	// after the node is added to it. This avoids duplicate operations on the same item.
	o.logger.Info("Created default graph (will be saved in transaction)", "graphID", graph.ID().String(), "userID", userID)

	return graph, nil
}

// createNode creates a new node with the provided details
func (o *CreateNodeOrchestrator) createNode(ctx context.Context, cmd commands.CreateNodeCommand) (*entities.Node, error) {
	// Create value objects
	content, err := valueobjects.NewNodeContent(cmd.Title, cmd.Content, valueobjects.FormatMarkdown)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}

	// Note: URL handling would need to be done differently
	// as NodeContent doesn't have SetURL method

	position, err := valueobjects.NewPosition3D(cmd.X, cmd.Y, cmd.Z)
	if err != nil {
		return nil, fmt.Errorf("failed to create position: %w", err)
	}

	// Create the node entity
	node, err := entities.NewNode(
		cmd.UserID,
		content,
		position,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Add tags if provided
	for _, tag := range cmd.Tags {
		if err := node.AddTag(tag); err != nil {
			o.logger.Debug("Failed to add tag", "tag", tag, "error", err)
		}
	}

	// Note: Metadata handling would depend on if cmd.Metadata field exists
	// and if node has SetMetadata method

	o.logger.Debug("Node created", "nodeID", node.ID().String(), "title", cmd.Title)

	return node, nil
}

// Helper methods for UoW operations

func (o *CreateNodeOrchestrator) saveNodeWithUoW(ctx context.Context, node *entities.Node) error {
	// Check if repository supports UoW
	if repoWithUoW, ok := o.nodeRepo.(interface {
		SaveWithUoW(context.Context, *entities.Node, interface{}) error
	}); ok {
		return repoWithUoW.SaveWithUoW(ctx, node, o.uow)
	}
	// If UoW is required but not supported, fail fast
	// This prevents partial updates outside transaction boundaries
	return fmt.Errorf("repository does not support unit of work transactions")
}

func (o *CreateNodeOrchestrator) saveGraphWithUoW(ctx context.Context, graph *aggregates.Graph) error {
	// Check if repository supports UoW
	if repoWithUoW, ok := o.graphRepo.(interface {
		SaveWithUoW(context.Context, *aggregates.Graph, interface{}) error
	}); ok {
		return repoWithUoW.SaveWithUoW(ctx, graph, o.uow)
	}
	// If UoW is required but not supported, fail fast
	return fmt.Errorf("repository does not support unit of work transactions")
}

func (o *CreateNodeOrchestrator) saveEdgeWithUoW(ctx context.Context, graphID string, edge *aggregates.Edge) error {
	// Check if repository supports UoW
	if repoWithUoW, ok := o.edgeRepo.(interface {
		SaveWithUoW(context.Context, string, *aggregates.Edge, interface{}) error
	}); ok {
		return repoWithUoW.SaveWithUoW(ctx, graphID, edge, o.uow)
	}
	// If UoW is required but not supported, fail fast
	return fmt.Errorf("repository does not support unit of work transactions")
}
