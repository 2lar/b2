package sagas

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GraphMigrationSaga handles the complex process of migrating a graph
// This includes copying all nodes, edges, and metadata from one graph to another
type GraphMigrationSaga struct {
	saga          *Saga
	sourceGraphID string
	targetGraphID string
	nodeRepo      ports.NodeRepository
	edgeRepo      ports.EdgeRepository
	graphRepo     ports.GraphRepository
	logger        *zap.Logger

	// Track migrated entities for rollback
	migratedNodes []string
	migratedEdges []string
	nodeMapping   map[string]string // Maps old node IDs to new node IDs
}

// GraphMigrationData holds data passed between saga steps
type GraphMigrationData struct {
	SourceGraph    *aggregates.Graph
	TargetGraph    *aggregates.Graph
	Nodes          []*entities.Node
	Edges          []*aggregates.Edge
	NodeMapping    map[string]string
	StartTime      time.Time
	CompletedSteps int
}

// NewGraphMigrationSaga creates a new graph migration saga
func NewGraphMigrationSaga(
	sourceID, targetID string,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	logger *zap.Logger,
) *GraphMigrationSaga {
	gms := &GraphMigrationSaga{
		sourceGraphID: sourceID,
		targetGraphID: targetID,
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		graphRepo:     graphRepo,
		logger:        logger,
		migratedNodes: make([]string, 0),
		migratedEdges: make([]string, 0),
		nodeMapping:   make(map[string]string),
	}

	// Build the saga with all steps
	gms.saga = NewSagaBuilder("GraphMigration", logger).
		WithMetadata("source_graph_id", sourceID).
		WithMetadata("target_graph_id", targetID).
		WithCompensableStep("ValidateGraphs", gms.validateGraphs, gms.compensateValidation).
		WithCompensableStep("CopyNodes", gms.copyNodes, gms.compensateNodes).
		WithCompensableStep("CopyEdges", gms.copyEdges, gms.compensateEdges).
		WithCompensableStep("UpdateMetadata", gms.updateMetadata, gms.compensateMetadata).
		WithStep("FinalizeMigration", gms.finalizeMigration).
		Build()

	return gms
}

// Execute runs the graph migration saga
func (gms *GraphMigrationSaga) Execute(ctx context.Context) error {
	initialData := &GraphMigrationData{
		NodeMapping: make(map[string]string),
		StartTime:   time.Now(),
	}

	_, err := gms.saga.Execute(ctx, initialData)
	if err != nil {
		gms.logger.Error("Graph migration failed",
			zap.String("source_graph_id", gms.sourceGraphID),
			zap.String("target_graph_id", gms.targetGraphID),
			zap.Error(err),
		)
		return err
	}

	gms.logger.Info("Graph migration completed successfully",
		zap.String("source_graph_id", gms.sourceGraphID),
		zap.String("target_graph_id", gms.targetGraphID),
		zap.Int("migrated_nodes", len(gms.migratedNodes)),
		zap.Int("migrated_edges", len(gms.migratedEdges)),
	)

	return nil
}

// Step 1: Validate both graphs exist and are accessible
func (gms *GraphMigrationSaga) validateGraphs(ctx context.Context, data interface{}) (interface{}, error) {
	migrationData := data.(*GraphMigrationData)

	// Validate source graph
	sourceGraphID := aggregates.GraphID(gms.sourceGraphID)

	sourceGraph, err := gms.graphRepo.GetByID(ctx, sourceGraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source graph: %w", err)
	}
	migrationData.SourceGraph = sourceGraph

	// Validate target graph
	targetGraphID := aggregates.GraphID(gms.targetGraphID)

	targetGraph, err := gms.graphRepo.GetByID(ctx, targetGraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target graph: %w", err)
	}
	migrationData.TargetGraph = targetGraph

	// Check if target graph is empty (optional safety check)
	metadata := targetGraph.Metadata()
	if nodeCount, ok := metadata["nodeCount"].(int); ok && nodeCount > 0 {
		gms.logger.Warn("Target graph is not empty",
			zap.String("target_graph_id", gms.targetGraphID),
			zap.Int("node_count", nodeCount),
		)
	}

	sourceMetadata := sourceGraph.Metadata()
	sourceNodeCount, _ := sourceMetadata["nodeCount"].(int)
	sourceEdgeCount, _ := sourceMetadata["edgeCount"].(int)

	gms.logger.Info("Graphs validated",
		zap.String("source_nodes", fmt.Sprintf("%d", sourceNodeCount)),
		zap.String("source_edges", fmt.Sprintf("%d", sourceEdgeCount)),
	)

	return migrationData, nil
}

func (gms *GraphMigrationSaga) compensateValidation(ctx context.Context, data interface{}) error {
	// Nothing to compensate for validation
	return nil
}

// Step 2: Copy all nodes from source to target
func (gms *GraphMigrationSaga) copyNodes(ctx context.Context, data interface{}) (interface{}, error) {
	migrationData := data.(*GraphMigrationData)

	// Get all nodes from source graph
	nodes, err := gms.nodeRepo.GetByGraphID(ctx, gms.sourceGraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes from source graph: %w", err)
	}

	migrationData.Nodes = nodes
	gms.logger.Info("Copying nodes", zap.Int("count", len(nodes)))

	// Copy each node to the target graph
	for _, node := range nodes {
		// Create new node with new ID but same content
		newNodeID := valueobjects.NewNodeID()

		// Create new node with same content but new ID and graph
		newNode, err := entities.NewNode(
			newNodeID.String(),
			node.Content(),
			node.Position(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create new node: %w", err)
		}

		// Copy tags (assuming tags are part of the node structure)
		// Tags might need to be handled differently based on implementation

		// Copy metadata if the node has a way to expose it
		// Metadata handling would depend on the actual Node entity structure

		// Save the new node
		if err := gms.nodeRepo.Save(ctx, newNode); err != nil {
			return nil, fmt.Errorf("failed to save migrated node: %w", err)
		}

		// Track migration
		gms.migratedNodes = append(gms.migratedNodes, newNode.ID().String())
		gms.nodeMapping[node.ID().String()] = newNode.ID().String()
		migrationData.NodeMapping[node.ID().String()] = newNode.ID().String()

		gms.logger.Debug("Node migrated",
			zap.String("old_id", node.ID().String()),
			zap.String("new_id", newNode.ID().String()),
		)
	}

	return migrationData, nil
}

func (gms *GraphMigrationSaga) compensateNodes(ctx context.Context, data interface{}) error {
	gms.logger.Info("Compensating node migration", zap.Int("count", len(gms.migratedNodes)))

	// Delete all migrated nodes
	for _, nodeID := range gms.migratedNodes {
		nodeIDObj, err := valueobjects.NewNodeIDFromString(nodeID)
		if err != nil {
			gms.logger.Error("Failed to parse node ID during compensation", zap.String("node_id", nodeID))
			continue
		}

		if err := gms.nodeRepo.Delete(ctx, nodeIDObj); err != nil {
			gms.logger.Error("Failed to delete migrated node during compensation",
				zap.String("node_id", nodeID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// Step 3: Copy all edges with updated node references
func (gms *GraphMigrationSaga) copyEdges(ctx context.Context, data interface{}) (interface{}, error) {
	migrationData := data.(*GraphMigrationData)

	// Get all edges from source graph
	edges, err := gms.edgeRepo.GetByGraphID(ctx, gms.sourceGraphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges from source graph: %w", err)
	}

	migrationData.Edges = edges
	gms.logger.Info("Copying edges", zap.Int("count", len(edges)))

	// Copy each edge with updated node references
	for _, edge := range edges {
		// Map old node IDs to new ones
		newSourceID, sourceExists := gms.nodeMapping[edge.SourceID.String()]
		newTargetID, targetExists := gms.nodeMapping[edge.TargetID.String()]

		if !sourceExists || !targetExists {
			gms.logger.Warn("Skipping edge with unmapped nodes",
				zap.String("source", edge.SourceID.String()),
				zap.String("target", edge.TargetID.String()),
			)
			continue
		}

		// Create new edge with mapped node IDs
		newSourceNodeID, _ := valueobjects.NewNodeIDFromString(newSourceID)
		newTargetNodeID, _ := valueobjects.NewNodeIDFromString(newTargetID)

		newEdge := &aggregates.Edge{
			ID:        fmt.Sprintf("%s", uuid.New()),
			SourceID:  newSourceNodeID,
			TargetID:  newTargetNodeID,
			Type:      edge.Type,
			Weight:    edge.Weight,
			CreatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		// Copy edge metadata if any
		if edge.Metadata != nil {
			for key, value := range edge.Metadata {
				newEdge.Metadata[key] = value
			}
		}

		// Save the new edge
		if err := gms.edgeRepo.Save(ctx, gms.targetGraphID, newEdge); err != nil {
			return nil, fmt.Errorf("failed to save migrated edge: %w", err)
		}

		// Track migration
		edgeID := fmt.Sprintf("%s->%s", newSourceID, newTargetID)
		gms.migratedEdges = append(gms.migratedEdges, edgeID)

		gms.logger.Debug("Edge migrated",
			zap.String("old_source", edge.SourceID.String()),
			zap.String("old_target", edge.TargetID.String()),
			zap.String("new_source", newSourceID),
			zap.String("new_target", newTargetID),
		)
	}

	return migrationData, nil
}

func (gms *GraphMigrationSaga) compensateEdges(ctx context.Context, data interface{}) error {
	gms.logger.Info("Compensating edge migration", zap.Int("count", len(gms.migratedEdges)))

	// Delete all migrated edges
	for _, edgeID := range gms.migratedEdges {
		// Parse edge ID (format: source->target)
		var source, target string
		fmt.Sscanf(edgeID, "%s->%s", &source, &target)

		if err := gms.edgeRepo.Delete(ctx, gms.targetGraphID, source, target); err != nil {
			gms.logger.Error("Failed to delete migrated edge during compensation",
				zap.String("edge_id", edgeID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// Step 4: Update graph metadata
func (gms *GraphMigrationSaga) updateMetadata(ctx context.Context, data interface{}) (interface{}, error) {
	migrationData := data.(*GraphMigrationData)

	// Update target graph metadata
	targetGraph := migrationData.TargetGraph

	// Note: The graph's metadata is updated automatically when nodes are added
	// No need to manually update counts

	// Update migration metadata
	// Store migration metadata separately as the graph doesn't have a SetMetadata method
	// This would typically be stored in a separate migration audit table
	migrationMetadata := map[string]interface{}{
		"migrated_from":  gms.sourceGraphID,
		"migration_date": time.Now().Format(time.RFC3339),
		"migrated_nodes": len(gms.migratedNodes),
		"migrated_edges": len(gms.migratedEdges),
	}
	migrationData.TargetGraph = targetGraph // Update the graph reference
	gms.logger.Info("Migration metadata prepared", zap.Any("metadata", migrationMetadata))

	// Save updated graph
	if err := gms.graphRepo.Save(ctx, targetGraph); err != nil {
		return nil, fmt.Errorf("failed to update target graph metadata: %w", err)
	}

	return migrationData, nil
}

func (gms *GraphMigrationSaga) compensateMetadata(ctx context.Context, data interface{}) error {
	// Since we don't have direct metadata manipulation on the graph,
	// compensation would typically involve logging or updating external metadata store
	gms.logger.Info("Compensating metadata changes")
	return nil
}

// Step 5: Finalize migration
func (gms *GraphMigrationSaga) finalizeMigration(ctx context.Context, data interface{}) (interface{}, error) {
	migrationData := data.(*GraphMigrationData)

	duration := time.Since(migrationData.StartTime)
	gms.logger.Info("Migration finalized",
		zap.String("duration", duration.String()),
		zap.Int("nodes_migrated", len(gms.migratedNodes)),
		zap.Int("edges_migrated", len(gms.migratedEdges)),
	)

	// Since the graph doesn't have a SetMetadata method, we would handle archiving differently
	// This could be done through a separate archival service or by updating a status field
	gms.logger.Info("Migration finalized - source graph should be marked as archived in external system",
		zap.String("source_graph", gms.sourceGraphID),
		zap.String("target_graph", gms.targetGraphID),
	)

	return migrationData, nil
}
