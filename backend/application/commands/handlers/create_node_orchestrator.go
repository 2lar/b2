package handlers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/application/commands"
	"backend/application/ports"
	"backend/application/services"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	domainservices "backend/domain/services"
	"backend/infrastructure/config"
	"backend/infrastructure/persistence/dynamodb"
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
	uow              ports.UnitOfWork
	nodeRepo         ports.NodeRepository
	graphRepo        ports.GraphRepository
	edgeRepo         ports.EdgeRepository
	edgeService      *services.EdgeService
	graphLazyService *services.GraphLazyService
	eventPublisher   ports.EventPublisher
	distributedLock  *dynamodb.DistributedLock
	config           *config.EdgeCreationConfig
	appConfig        *config.Config
	logger           Logger
}

// NewCreateNodeOrchestrator creates a new orchestrator instance
func NewCreateNodeOrchestrator(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	edgeService *services.EdgeService,
	graphLazyService *services.GraphLazyService,
	eventPublisher ports.EventPublisher,
	distributedLock *dynamodb.DistributedLock,
	edgeConfig *config.EdgeCreationConfig,
	appConfig *config.Config,
	logger Logger,
) *CreateNodeOrchestrator {
	return &CreateNodeOrchestrator{
		uow:              uow,
		nodeRepo:         nodeRepo,
		graphRepo:        graphRepo,
		edgeRepo:         edgeRepo,
		edgeService:      edgeService,
		graphLazyService: graphLazyService,
		eventPublisher:   eventPublisher,
		distributedLock:  distributedLock,
		config:           edgeConfig,
		appConfig:        appConfig,
		logger:           logger,
	}
}

// Handle orchestrates the node creation process
// Following CQRS pattern, this method returns void (only error) and publishes events for state changes
func (o *CreateNodeOrchestrator) Handle(ctx context.Context, cmd commands.CreateNodeCommand) error {
	// Validate command
	if err := o.validateCommand(cmd); err != nil {
		return fmt.Errorf("invalid command: %w", err)
	}

	// Start unit of work transaction
	if err := o.uow.Begin(ctx); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer o.uow.Rollback() // Will be no-op if commit succeeds

	// Step 1: Determine graph loading strategy and get/create graph
	var graphID string
	var graph *aggregates.Graph
	var lazyGraph *aggregates.GraphLazy
	isLazyMode := o.appConfig.EnableLazyLoading && o.graphLazyService != nil
	
	if isLazyMode {
		// Use lazy-loaded graph for better performance
		o.logger.Info("Using lazy-loaded graph for node creation",
			"userID", cmd.UserID,
			"lazyLoading", true,
		)
		
		// For lazy loading, we don't need to load the entire graph
		// Just ensure we have a graph ID from the repository
		defaultGraph, err := o.graphRepo.GetUserDefaultGraph(ctx, cmd.UserID)
		if err != nil {
			// If no default graph, create one
			defaultGraph, err = o.graphRepo.GetOrCreateDefaultGraph(ctx, cmd.UserID)
			if err != nil {
				return fmt.Errorf("failed to get or create default graph: %w", err)
			}
		}
		graphID = string(defaultGraph.ID())
		graph = defaultGraph // Keep reference for later use
		
		// Register the graph with lazy service for future operations
		lazyGraph, err = o.graphLazyService.GetOrCreateForUser(ctx, cmd.UserID, graphID)
		if err != nil {
			o.logger.Error("Failed to register graph with lazy service, falling back to regular graph",
				"error", err,
			)
			// Fall back to regular graph
			isLazyMode = false
			graph, err = o.ensureGraphExists(ctx, cmd.UserID)
			if err != nil {
				return fmt.Errorf("failed to ensure graph exists: %w", err)
			}
			graphID = string(graph.ID())
		}
	} else {
		// Use regular graph loading (current behavior)
		o.logger.Debug("Using standard graph loading",
			"userID", cmd.UserID,
			"lazyLoading", false,
		)
		var err error
		graph, err = o.ensureGraphExists(ctx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("failed to ensure graph exists: %w", err)
		}
		graphID = string(graph.ID())
	}

	// Step 2: Create the node
	node, err := o.createNode(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	// Step 3: Set the GraphID on the node BEFORE saving
	// This is critical - the repository requires nodes to have a GraphID before saving
	node.SetGraphID(graphID)

	// Step 4: Save the node now that it has a GraphID
	if err := o.saveNodeWithUoW(ctx, node); err != nil {
		return fmt.Errorf("failed to save node: %w", err)
	}

	// Step 5: Add node to graph and handle edge discovery
	var syncEdges []aggregates.EdgeCandidate
	var asyncCandidates []aggregates.EdgeCandidate

	if isLazyMode && lazyGraph != nil {
		// For lazy loading, we still need to discover and create sync edges
		// This ensures immediate connectivity for new nodes
		// Add node ID to lazy graph for tracking
		if err := lazyGraph.AddNodeID(node.ID()); err != nil {
			o.logger.Info("Failed to add node to lazy graph",
				"error", err,
				"nodeID", node.ID().String(),
			)
		}

		// IMPORTANT: Even in lazy mode, we need to discover edges for immediate connectivity
		// We need to load existing nodes to discover edges
		existingNodes, err := o.nodeRepo.GetByGraphID(ctx, graphID)
		if err != nil {
			o.logger.Error("Failed to load nodes for edge discovery in lazy mode",
				"error", err,
				"graphID", graphID,
			)
		} else if len(existingNodes) > 0 {
			// Create a temporary graph for edge discovery
			tempGraph, err := aggregates.NewGraph(cmd.UserID, "temp")
			if err == nil {
				// Add existing nodes to temp graph
				for _, existingNode := range existingNodes {
					tempGraph.AddNode(existingNode)
				}
				// Add the new node
				tempGraph.AddNode(node)

				// Discover edges using the temp graph
				syncEdges, asyncCandidates, err = o.edgeService.DiscoverEdges(
					ctx, node, tempGraph, o.config.SyncEdgeLimit,
				)
				if err != nil {
					o.logger.Error("Failed to discover edges in lazy mode",
						"error", err,
						"nodeID", node.ID().String(),
					)
				} else {
					// Create sync edges directly in the repository
					for _, edgeCandidate := range syncEdges {
						edge := &aggregates.Edge{
							ID:       fmt.Sprintf("EDGE#%s#%s", edgeCandidate.SourceID.String(), edgeCandidate.TargetID.String()),
							SourceID: edgeCandidate.SourceID,
							TargetID: edgeCandidate.TargetID,
							Type:     edgeCandidate.Type,
							Weight:   edgeCandidate.Similarity,
							CreatedAt: time.Now(),
						}

						// Save edge directly using edge repository
						if edgeRepoWithUoW, ok := o.edgeRepo.(interface {
							SaveWithUoW(context.Context, string, *aggregates.Edge, interface{}) error
						}); ok {
							if err := edgeRepoWithUoW.SaveWithUoW(ctx, graphID, edge, o.uow); err != nil {
								o.logger.Error("Failed to save sync edge in lazy mode",
									"error", err,
									"edgeID", edge.ID,
								)
							}
						}
					}

					o.logger.Info("Synchronous edges created in lazy mode",
						"nodeID", node.ID().String(),
						"syncEdgeCount", len(syncEdges),
						"asyncPending", len(asyncCandidates),
					)
				}
			}
		}

	} else {
		// Regular graph handling (current behavior)
		// Add node to graph first (before creating edges)
		if err := graph.AddNode(node); err != nil {
			return fmt.Errorf("failed to add node to graph: %w", err)
		}
		
		// Use the domain's edge discovery method
		syncEdges, asyncCandidates, err = o.edgeService.DiscoverEdges(
			ctx, node, graph, o.config.SyncEdgeLimit,
		)
		if err != nil {
			o.logger.Error("Failed to discover edges",
				"error", err,
				"nodeID", node.ID().String(),
			)
			// Don't fail node creation if edge discovery fails
		} else {
			// Create high-priority edges synchronously using domain method
			if len(syncEdges) > 0 {
				// Use the Graph's DiscoverAndConnectEdges method for sync edges
				// This leverages the domain logic for creating edges
				for _, edgeCandidate := range syncEdges {
					edge, err := graph.ConnectNodes(
						edgeCandidate.SourceID,
						edgeCandidate.TargetID,
						edgeCandidate.Type,
					)
					if err != nil {
						o.logger.Error("Failed to create sync edge",
							"error", err,
							"source", edgeCandidate.SourceID.String(),
							"target", edgeCandidate.TargetID.String(),
						)
						continue
					}
					// Set the weight based on similarity
					edge.Weight = edgeCandidate.Similarity
				}
			}

			o.logger.Info("Synchronous edges created",
				"nodeID", node.ID().String(),
				"syncEdgeCount", len(syncEdges),
				"asyncPending", len(asyncCandidates),
			)

			// Async edges will be handled by the connect-node Lambda via events
		}

		// Step 6: Save the graph with all nodes and edges
		if err := o.saveGraphWithUoW(ctx, graph); err != nil {
			return fmt.Errorf("failed to update graph: %w", err)
		}
	}

	// Commit transaction
	if err := o.uow.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Publish domain events after successful commit
	domainEvents := node.GetUncommittedEvents()
	
	// Only append graph events if we have a regular graph (not lazy mode)
	if !isLazyMode && graph != nil {
		domainEvents = append(domainEvents, graph.GetUncommittedEvents()...)
	}

	// If there are async edge candidates, create enhanced event
	if len(asyncCandidates) > 0 && o.config.AsyncEnabled {
		// Convert service candidates to event candidates
		eventCandidates := make([]events.EdgeCandidate, len(asyncCandidates))
		for i, candidate := range asyncCandidates {
			eventCandidates[i] = events.EdgeCandidate{
				SourceID:   candidate.SourceID.String(),
				TargetID:   candidate.TargetID.String(),
				Type:       string(candidate.Type),
				Similarity: candidate.Similarity,
			}
		}

		// Use domain service for keyword extraction
		nodeContent := node.Content()
		textAnalyzer := domainservices.NewDefaultTextAnalyzer()
		keywords := textAnalyzer.ExtractKeywords(nodeContent.Title() + " " + nodeContent.Body())
		
		// Create enhanced event with pending edges
		enhancedEvent := events.NewNodeCreatedWithPendingEdges(
			node.ID(),
			graphID,
			cmd.UserID,
			nodeContent.Title(),
			keywords,
			node.GetTags(),
			len(syncEdges),
			eventCandidates,
		)
		domainEvents = append(domainEvents, enhancedEvent)
	}

	if len(domainEvents) > 0 {
		if err := o.eventPublisher.PublishBatch(ctx, domainEvents); err != nil {
			// Log error but don't fail - events can be retried
			o.logger.Error("Failed to publish domain events",
				"error", err,
				"eventCount", len(domainEvents),
				"nodeID", node.ID().String(),
			)
		} else {
			// Mark events as committed after successful publishing
			node.MarkEventsAsCommitted()
			if !isLazyMode && graph != nil {
				graph.MarkEventsAsCommitted()
			}
		}
	}

	// Get node count safely for logging
	nodeCount := 0
	edgeCount := 0
	
	if isLazyMode && lazyGraph != nil {
		// For lazy mode, use the lazy graph's count methods
		nodeCount = lazyGraph.NodeCount()
		edgeCount = lazyGraph.EdgeCount()
	} else if graph != nil {
		// For regular mode, count from the loaded graph
		nodes, nodeErr := graph.Nodes()
		if nodeErr != nil {
			nodeCount = -1 // Indicate large graph
		} else {
			nodeCount = len(nodes)
		}
		edgeCount = len(graph.GetEdges())
	}

	// Update graph metadata after transaction commits
	// This ensures we count the actual committed data
	if err := o.graphRepo.UpdateGraphMetadata(ctx, graphID); err != nil {
		o.logger.Error("Failed to update graph metadata after commit",
			"error", err,
			"graphID", graphID,
			"nodeCount", nodeCount,
			"edgeCount", edgeCount,
		)
		// Don't fail the operation if metadata update fails
	}

	o.logger.Info("Node created successfully",
		"nodeID", node.ID().String(),
		"graphID", graphID,
		"userID", cmd.UserID,
		"totalGraphNodes", nodeCount,
		"lazyMode", isLazyMode,
	)

	return nil
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
