package sagas

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/services"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	domainservices "backend/domain/services"
	"backend/infrastructure/config"
	"backend/infrastructure/persistence/dynamodb"

	"go.uber.org/zap"
)

// CreateNodeSagaData holds data passed between saga steps
type CreateNodeSagaData struct {
	// Input
	UserID   string
	Title    string
	Content  string
	Tags     []string
	X, Y, Z  float64
	Metadata map[string]interface{}

	// Operation tracking
	OperationID string
	StartTime   time.Time

	// State between steps
	Graph           *aggregates.Graph
	LazyGraph       *aggregates.GraphLazy
	GraphID         string
	Node            *entities.Node
	IsLazyMode      bool
	Lock            *dynamodb.Lock
	SyncEdges       []aggregates.EdgeCandidate
	AsyncCandidates []aggregates.EdgeCandidate
	CreatedEdgeIDs  []string

	// For compensation
	NodeCreated       bool
	GraphCreated      bool
	EdgesCreated      int
	EventsPublished   bool
	MetadataUpdated   bool
}

// CreateNodeSaga orchestrates the complex node creation process using saga pattern
type CreateNodeSaga struct {
	uow              ports.UnitOfWork
	nodeRepo         ports.NodeRepository
	graphRepo        ports.GraphRepository
	edgeRepo         ports.EdgeRepository
	edgeService      *services.EdgeService
	graphLazyService *services.GraphLazyService
	eventPublisher   ports.EventPublisher
	distributedLock  *dynamodb.DistributedLock
	operationStore   ports.OperationStore
	edgeConfig       *config.EdgeCreationConfig
	appConfig        *config.Config
	logger           *zap.Logger
}

// NewCreateNodeSaga creates a new create node saga
func NewCreateNodeSaga(
	uow ports.UnitOfWork,
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	edgeRepo ports.EdgeRepository,
	edgeService *services.EdgeService,
	graphLazyService *services.GraphLazyService,
	eventPublisher ports.EventPublisher,
	distributedLock *dynamodb.DistributedLock,
	operationStore ports.OperationStore,
	edgeConfig *config.EdgeCreationConfig,
	appConfig *config.Config,
	logger *zap.Logger,
) *CreateNodeSaga {
	return &CreateNodeSaga{
		uow:              uow,
		nodeRepo:         nodeRepo,
		graphRepo:        graphRepo,
		edgeRepo:         edgeRepo,
		edgeService:      edgeService,
		graphLazyService: graphLazyService,
		eventPublisher:   eventPublisher,
		distributedLock:  distributedLock,
		operationStore:   operationStore,
		edgeConfig:       edgeConfig,
		appConfig:        appConfig,
		logger:           logger,
	}
}

// BuildSaga constructs the saga with all steps
func (cns *CreateNodeSaga) BuildSaga(operationID string) *Saga {
	return NewSagaBuilder("CreateNode", cns.logger).
		WithMetadata("operation_id", operationID).
		WithStep("ValidateInput", cns.validateInput).
		WithCompensableStep("BeginTransaction", cns.beginTransaction, cns.rollbackTransaction).
		WithCompensableStep("EnsureGraph", cns.ensureGraph, cns.compensateGraph).
		WithCompensableStep("CreateNode", cns.createNode, cns.compensateNode).
		WithCompensableStep("SaveNode", cns.saveNode, cns.compensateSaveNode).
		WithRetryableStep("DiscoverEdges", cns.discoverEdges, 3, 2*time.Second).
		WithCompensableStep("CreateSyncEdges", cns.createSyncEdges, cns.compensateEdges).
		WithCompensableStep("UpdateGraph", cns.updateGraph, cns.compensateGraphUpdate).
		WithStep("CommitTransaction", cns.commitTransaction).
		WithRetryableStep("PublishEvents", cns.publishEvents, 3, 1*time.Second).
		WithStep("UpdateMetadata", cns.updateMetadata).
		Build()
}

// Execute runs the create node saga
func (cns *CreateNodeSaga) Execute(ctx context.Context, data *CreateNodeSagaData) error {
	// Update operation status to running
	if cns.operationStore != nil && data.OperationID != "" {
		cns.operationStore.Update(ctx, data.OperationID, &ports.OperationResult{
			OperationID: data.OperationID,
			Status:      "pending",
			StartedAt:   data.StartTime,
			Metadata: map[string]interface{}{
				"stage": "starting",
			},
		})
	}

	saga := cns.BuildSaga(data.OperationID)
	
	_, err := saga.Execute(ctx, data)
	
	// Update operation status based on result
	if cns.operationStore != nil && data.OperationID != "" {
		now := time.Now()
		if err != nil {
			cns.operationStore.Update(ctx, data.OperationID, &ports.OperationResult{
				OperationID: data.OperationID,
				Status:      ports.OperationStatusFailed,
				StartedAt:   data.StartTime,
				CompletedAt: &now,
				Error:       err.Error(),
			})
		} else {
			cns.operationStore.Update(ctx, data.OperationID, &ports.OperationResult{
				OperationID: data.OperationID,
				Status:      ports.OperationStatusCompleted,
				StartedAt:   data.StartTime,
				CompletedAt: &now,
				Result: map[string]interface{}{
					"node_id":       data.Node.ID().String(),
					"graph_id":      data.GraphID,
					"edges_created": data.EdgesCreated,
				},
			})
		}
	}

	return err
}

// Step 1: Validate input
func (cns *CreateNodeSaga) validateInput(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	if d.UserID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	if d.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if len(d.Title) > 255 {
		return nil, fmt.Errorf("title exceeds maximum length of 255 characters")
	}
	if len(d.Content) > 50000 {
		return nil, fmt.Errorf("content exceeds maximum length of 50000 characters")
	}
	
	// Validate position bounds
	if d.X < -10000 || d.X > 10000 ||
		d.Y < -10000 || d.Y > 10000 ||
		d.Z < -10000 || d.Z > 10000 {
		return nil, fmt.Errorf("position coordinates out of bounds")
	}
	
	cns.logger.Debug("Input validated successfully",
		zap.String("user_id", d.UserID),
		zap.String("title", d.Title),
	)
	
	return d, nil
}

// Step 2: Begin transaction
func (cns *CreateNodeSaga) beginTransaction(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	if err := cns.uow.Begin(ctx); err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	cns.logger.Debug("Transaction started")
	return d, nil
}

func (cns *CreateNodeSaga) rollbackTransaction(ctx context.Context, data interface{}) error {
	cns.uow.Rollback()
	cns.logger.Debug("Transaction rolled back")
	return nil
}

// Step 3: Ensure graph exists
func (cns *CreateNodeSaga) ensureGraph(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	// Determine if we should use lazy loading
	d.IsLazyMode = cns.appConfig.EnableLazyLoading && cns.graphLazyService != nil
	
	if d.IsLazyMode {
		// Use lazy-loaded graph
		cns.logger.Info("Using lazy-loaded graph for node creation",
			zap.String("user_id", d.UserID),
		)
		
		defaultGraph, err := cns.graphRepo.GetUserDefaultGraph(ctx, d.UserID)
		if err != nil {
			// Create new graph if doesn't exist
			defaultGraph, err = cns.createDefaultGraph(ctx, d.UserID)
			if err != nil {
				return nil, fmt.Errorf("failed to create default graph: %w", err)
			}
			d.GraphCreated = true
		}
		
		d.Graph = defaultGraph
		d.GraphID = string(defaultGraph.ID())
		
		// Register with lazy service
		lazyGraph, err := cns.graphLazyService.GetOrCreateForUser(ctx, d.UserID, d.GraphID)
		if err != nil {
			cns.logger.Warn("Failed to register graph with lazy service, falling back",
				zap.Error(err),
			)
			d.IsLazyMode = false
		} else {
			d.LazyGraph = lazyGraph
		}
	} else {
		// Use regular graph loading
		graph, err := cns.ensureGraphWithLock(ctx, d.UserID)
		if err != nil {
			return nil, err
		}
		d.Graph = graph
		d.GraphID = string(graph.ID())
	}
	
	return d, nil
}

func (cns *CreateNodeSaga) compensateGraph(ctx context.Context, data interface{}) error {
	d := data.(*CreateNodeSagaData)
	
	// Only delete graph if we created it in this saga
	if d.GraphCreated && d.Graph != nil {
		cns.logger.Info("Compensating: Deleting created graph",
			zap.String("graph_id", d.GraphID),
		)
		// In practice, you might want to mark for deletion rather than delete immediately
	}
	
	// Release lock if acquired
	if d.Lock != nil {
		d.Lock.Release(ctx)
	}
	
	return nil
}

// Step 4: Create node entity
func (cns *CreateNodeSaga) createNode(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	content, err := valueobjects.NewNodeContent(d.Title, d.Content, valueobjects.FormatMarkdown)
	if err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}
	
	position, err := valueobjects.NewPosition3D(d.X, d.Y, d.Z)
	if err != nil {
		return nil, fmt.Errorf("failed to create position: %w", err)
	}
	
	node, err := entities.NewNode(d.UserID, content, position)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	
	// Add tags
	for _, tag := range d.Tags {
		if err := node.AddTag(tag); err != nil {
			cns.logger.Debug("Failed to add tag", zap.String("tag", tag), zap.Error(err))
		}
	}
	
	// Set graph ID
	node.SetGraphID(d.GraphID)
	
	d.Node = node
	d.NodeCreated = true
	
	cns.logger.Debug("Node entity created",
		zap.String("node_id", node.ID().String()),
		zap.String("title", d.Title),
	)
	
	return d, nil
}

func (cns *CreateNodeSaga) compensateNode(ctx context.Context, data interface{}) error {
	d := data.(*CreateNodeSagaData)
	
	if d.NodeCreated && d.Node != nil {
		cns.logger.Info("Compensating: Node entity marked for cleanup",
			zap.String("node_id", d.Node.ID().String()),
		)
		// Entity will be garbage collected, no DB operation yet
	}
	
	return nil
}

// Step 5: Save node to repository
func (cns *CreateNodeSaga) saveNode(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	// Save node with UoW
	if err := cns.saveNodeWithUoW(ctx, d.Node); err != nil {
		return nil, fmt.Errorf("failed to save node: %w", err)
	}
	
	// Add to graph aggregate
	if d.IsLazyMode && d.LazyGraph != nil {
		if err := d.LazyGraph.AddNodeID(d.Node.ID()); err != nil {
			cns.logger.Warn("Failed to add node to lazy graph",
				zap.Error(err),
				zap.String("node_id", d.Node.ID().String()),
			)
		}
	} else if d.Graph != nil {
		if err := d.Graph.AddNode(d.Node); err != nil {
			return nil, fmt.Errorf("failed to add node to graph: %w", err)
		}
	}
	
	cns.logger.Info("Node saved to repository",
		zap.String("node_id", d.Node.ID().String()),
		zap.String("graph_id", d.GraphID),
	)
	
	return d, nil
}

func (cns *CreateNodeSaga) compensateSaveNode(ctx context.Context, data interface{}) error {
	d := data.(*CreateNodeSagaData)
	
	if d.Node != nil {
		cns.logger.Info("Compensating: Deleting saved node",
			zap.String("node_id", d.Node.ID().String()),
		)
		
		// Delete node from repository (within transaction)
		if err := cns.nodeRepo.Delete(ctx, d.Node.ID()); err != nil {
			cns.logger.Error("Failed to delete node during compensation",
				zap.Error(err),
				zap.String("node_id", d.Node.ID().String()),
			)
			return err
		}
	}
	
	return nil
}

// Step 6: Discover edges (with retry)
func (cns *CreateNodeSaga) discoverEdges(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)

	// Even in lazy mode, we need to discover edges for immediate connectivity
	var graphForDiscovery *aggregates.Graph

	if d.IsLazyMode {
		// In lazy mode, load existing nodes to discover edges
		existingNodes, err := cns.nodeRepo.GetByGraphID(ctx, d.GraphID)
		if err != nil {
			cns.logger.Error("Failed to load nodes for edge discovery in lazy mode",
				zap.Error(err),
				zap.String("graph_id", d.GraphID),
			)
			return d, nil // Continue without edges rather than failing
		}

		if len(existingNodes) == 0 {
			cns.logger.Info("No existing nodes to connect with in lazy mode",
				zap.String("node_id", d.Node.ID().String()),
			)
			return d, nil
		}

		// Create a temporary graph for edge discovery
		tempGraph, err := aggregates.NewGraph(d.UserID, "temp")
		if err != nil {
			cns.logger.Error("Failed to create temp graph for edge discovery",
				zap.Error(err),
			)
			return d, nil
		}

		// Add existing nodes to temp graph
		for _, existingNode := range existingNodes {
			tempGraph.AddNode(existingNode)
		}
		// Add the new node
		tempGraph.AddNode(d.Node)

		graphForDiscovery = tempGraph
		cns.logger.Info("Using temporary graph for edge discovery in lazy mode",
			zap.String("node_id", d.Node.ID().String()),
			zap.Int("existing_nodes", len(existingNodes)),
		)
	} else {
		// Regular mode - use the actual graph
		graphForDiscovery = d.Graph
	}

	// Use edge service to discover edges
	syncEdges, asyncCandidates, err := cns.edgeService.DiscoverEdges(
		ctx, d.Node, graphForDiscovery, cns.edgeConfig.SyncEdgeLimit,
	)
	if err != nil {
		// Don't fail the saga for edge discovery failure
		cns.logger.Error("Edge discovery failed, continuing without edges",
			zap.Error(err),
			zap.String("node_id", d.Node.ID().String()),
		)
		return d, nil
	}
	
	d.SyncEdges = syncEdges
	d.AsyncCandidates = asyncCandidates
	
	cns.logger.Info("Edges discovered",
		zap.String("node_id", d.Node.ID().String()),
		zap.Int("sync_edges", len(syncEdges)),
		zap.Int("async_candidates", len(asyncCandidates)),
	)
	
	return d, nil
}

// Step 7: Create synchronous edges
func (cns *CreateNodeSaga) createSyncEdges(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)

	if len(d.SyncEdges) == 0 {
		return d, nil
	}

	d.CreatedEdgeIDs = make([]string, 0, len(d.SyncEdges))

	// In lazy mode, save edges directly to repository
	if d.IsLazyMode {
		for _, candidate := range d.SyncEdges {
			edge := &aggregates.Edge{
				ID:       fmt.Sprintf("EDGE#%s#%s", candidate.SourceID.String(), candidate.TargetID.String()),
				SourceID: candidate.SourceID,
				TargetID: candidate.TargetID,
				Type:     candidate.Type,
				Weight:   candidate.Similarity,
				CreatedAt: time.Now(),
			}

			// Save edge directly using edge repository with UoW
			if edgeRepoWithUoW, ok := cns.edgeRepo.(interface {
				SaveWithUoW(context.Context, string, *aggregates.Edge, interface{}) error
			}); ok {
				if err := edgeRepoWithUoW.SaveWithUoW(ctx, d.GraphID, edge, cns.uow); err != nil {
					cns.logger.Error("Failed to save sync edge in lazy mode",
						zap.Error(err),
						zap.String("edge_id", edge.ID),
					)
					continue
				}
			}

			d.CreatedEdgeIDs = append(d.CreatedEdgeIDs, edge.ID)
			d.EdgesCreated++
		}
	} else {
		// Regular mode - create edges in graph aggregate
		for _, candidate := range d.SyncEdges {
			edge, err := d.Graph.ConnectNodes(
				candidate.SourceID,
				candidate.TargetID,
				candidate.Type,
			)
			if err != nil {
				cns.logger.Error("Failed to create sync edge",
					zap.Error(err),
					zap.String("source", candidate.SourceID.String()),
					zap.String("target", candidate.TargetID.String()),
				)
				continue
			}

			edge.Weight = candidate.Similarity
			d.CreatedEdgeIDs = append(d.CreatedEdgeIDs, edge.ID)
			d.EdgesCreated++
		}
	}

	cns.logger.Info("Synchronous edges created",
		zap.Int("edges_created", d.EdgesCreated),
		zap.Bool("lazy_mode", d.IsLazyMode),
	)

	return d, nil
}

func (cns *CreateNodeSaga) compensateEdges(ctx context.Context, data interface{}) error {
	d := data.(*CreateNodeSagaData)
	
	if len(d.CreatedEdgeIDs) > 0 {
		cns.logger.Info("Compensating: Removing created edges",
			zap.Int("edge_count", len(d.CreatedEdgeIDs)),
		)
		// Edges will be removed with graph rollback
	}
	
	return nil
}

// Step 8: Update graph
func (cns *CreateNodeSaga) updateGraph(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	if d.IsLazyMode {
		// No need to save entire graph in lazy mode
		return d, nil
	}
	
	// Save graph with all nodes and edges
	if err := cns.saveGraphWithUoW(ctx, d.Graph); err != nil {
		return nil, fmt.Errorf("failed to update graph: %w", err)
	}
	
	cns.logger.Debug("Graph updated with new node and edges")
	return d, nil
}

func (cns *CreateNodeSaga) compensateGraphUpdate(ctx context.Context, data interface{}) error {
	// Graph update will be rolled back with transaction
	return nil
}

// Step 9: Commit transaction
func (cns *CreateNodeSaga) commitTransaction(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	if err := cns.uow.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	cns.logger.Info("Transaction committed successfully")
	return d, nil
}

// Step 10: Publish events (with retry)
func (cns *CreateNodeSaga) publishEvents(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	// Collect all domain events
	var domainEvents []events.DomainEvent
	
	if d.Node != nil {
		domainEvents = append(domainEvents, d.Node.GetUncommittedEvents()...)
	}
	
	if !d.IsLazyMode && d.Graph != nil {
		domainEvents = append(domainEvents, d.Graph.GetUncommittedEvents()...)
	}
	
	// Add enhanced event for async edge creation
	if len(d.AsyncCandidates) > 0 && cns.edgeConfig.AsyncEnabled {
		eventCandidates := make([]events.EdgeCandidate, len(d.AsyncCandidates))
		for i, candidate := range d.AsyncCandidates {
			eventCandidates[i] = events.EdgeCandidate{
				SourceID:   candidate.SourceID.String(),
				TargetID:   candidate.TargetID.String(),
				Type:       string(candidate.Type),
				Similarity: candidate.Similarity,
			}
		}
		
		// Extract keywords for the event
		nodeContent := d.Node.Content()
		textAnalyzer := domainservices.NewDefaultTextAnalyzer()
		keywords := textAnalyzer.ExtractKeywords(nodeContent.Title() + " " + nodeContent.Body())
		
		enhancedEvent := events.NewNodeCreatedWithPendingEdges(
			d.Node.ID(),
			d.GraphID,
			d.UserID,
			nodeContent.Title(),
			keywords,
			d.Node.GetTags(),
			len(d.SyncEdges),
			eventCandidates,
		)
		domainEvents = append(domainEvents, enhancedEvent)
	}
	
	// Publish events
	if len(domainEvents) > 0 {
		if err := cns.eventPublisher.PublishBatch(ctx, domainEvents); err != nil {
			// Log but don't fail - events can be retried
			cns.logger.Error("Failed to publish domain events",
				zap.Error(err),
				zap.Int("event_count", len(domainEvents)),
			)
			return d, err // Return error to trigger retry
		}
		
		// Mark events as committed
		if d.Node != nil {
			d.Node.MarkEventsAsCommitted()
		}
		if !d.IsLazyMode && d.Graph != nil {
			d.Graph.MarkEventsAsCommitted()
		}
		
		d.EventsPublished = true
		cns.logger.Info("Domain events published",
			zap.Int("event_count", len(domainEvents)),
		)
	}
	
	return d, nil
}

// Step 11: Update metadata
func (cns *CreateNodeSaga) updateMetadata(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(*CreateNodeSagaData)
	
	// Update graph metadata
	if err := cns.graphRepo.UpdateGraphMetadata(ctx, d.GraphID); err != nil {
		cns.logger.Error("Failed to update graph metadata",
			zap.Error(err),
			zap.String("graph_id", d.GraphID),
		)
		// Don't fail the saga for metadata update
	} else {
		d.MetadataUpdated = true
	}
	
	// Log completion
	cns.logger.Info("Node creation saga completed successfully",
		zap.String("node_id", d.Node.ID().String()),
		zap.String("graph_id", d.GraphID),
		zap.String("user_id", d.UserID),
		zap.Int("edges_created", d.EdgesCreated),
		zap.Duration("duration", time.Since(d.StartTime)),
	)
	
	return d, nil
}

// Helper methods

func (cns *CreateNodeSaga) ensureGraphWithLock(ctx context.Context, userID string) (*aggregates.Graph, error) {
	// Try to get existing graph first
	graph, err := cns.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err == nil {
		return graph, nil
	}
	
	// Need to create with lock
	lockResource := fmt.Sprintf("default_graph_creation_%s", userID)
	lock, err := cns.distributedLock.TryAcquireLock(
		ctx, lockResource, userID, 30*time.Second, 5*time.Second,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Release(ctx)
	
	// Double-check after acquiring lock
	graph, err = cns.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err == nil {
		return graph, nil
	}
	
	// Create new graph
	return cns.createDefaultGraph(ctx, userID)
}

func (cns *CreateNodeSaga) createDefaultGraph(ctx context.Context, userID string) (*aggregates.Graph, error) {
	graph, err := aggregates.NewGraph(userID, "Default Graph")
	if err != nil {
		return nil, fmt.Errorf("failed to create graph: %w", err)
	}
	
	cns.logger.Info("Created default graph",
		zap.String("graph_id", graph.ID().String()),
		zap.String("user_id", userID),
	)
	
	return graph, nil
}

func (cns *CreateNodeSaga) saveNodeWithUoW(ctx context.Context, node *entities.Node) error {
	if repoWithUoW, ok := cns.nodeRepo.(interface {
		SaveWithUoW(context.Context, *entities.Node, interface{}) error
	}); ok {
		return repoWithUoW.SaveWithUoW(ctx, node, cns.uow)
	}
	return fmt.Errorf("repository does not support unit of work")
}

func (cns *CreateNodeSaga) saveGraphWithUoW(ctx context.Context, graph *aggregates.Graph) error {
	if repoWithUoW, ok := cns.graphRepo.(interface {
		SaveWithUoW(context.Context, *aggregates.Graph, interface{}) error
	}); ok {
		return repoWithUoW.SaveWithUoW(ctx, graph, cns.uow)
	}
	return fmt.Errorf("repository does not support unit of work")
}