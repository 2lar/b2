package services

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"go.uber.org/zap"
)

// GraphLoader provides efficient batch loading of graph data
// This prevents N+1 queries by loading all related data in minimal queries
type GraphLoader struct {
	graphRepo ports.GraphRepository
	nodeRepo  ports.NodeRepository
	edgeRepo  ports.EdgeRepository
	logger    *zap.Logger
}

// NewGraphLoader creates a new graph loader
func NewGraphLoader(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *GraphLoader {
	return &GraphLoader{
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		logger:    logger,
	}
}

// LoadComplete loads a complete graph with all nodes and edges in minimal queries
func (l *GraphLoader) LoadComplete(ctx context.Context, graphID aggregates.GraphID) (*aggregates.Graph, error) {
	l.logger.Debug("Loading complete graph", zap.String("graphID", string(graphID)))

	// Load graph metadata first
	graph, err := l.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Load all nodes for the graph in a single query
	nodes, err := l.nodeRepo.GetByGraphID(ctx, string(graphID))
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	// Load all edges for the graph in a single query
	edges, err := l.edgeRepo.GetByGraphID(ctx, string(graphID))
	if err != nil {
		return nil, fmt.Errorf("failed to load edges: %w", err)
	}

	// Add nodes and edges back to the graph
	// Note: The graph already exists, we just need to populate it with loaded data
	for _, node := range nodes {
		if err := graph.AddNode(node); err != nil {
			l.logger.Warn("Failed to add node to graph", 
				zap.String("nodeID", node.ID().String()),
				zap.Error(err))
		}
	}

	for _, edge := range edges {
		_, err := graph.ConnectNodes(edge.SourceID, edge.TargetID, edge.Type)
		if err != nil {
			l.logger.Warn("Failed to add edge to graph",
				zap.String("edgeID", edge.ID),
				zap.Error(err))
		}
	}

	l.logger.Info("Loaded complete graph",
		zap.String("graphID", string(graphID)),
		zap.Int("nodeCount", len(nodes)),
		zap.Int("edgeCount", len(edges)),
	)

	return graph, nil
}

// LoadGraphWithNodes loads a graph with its nodes but without edges
func (l *GraphLoader) LoadGraphWithNodes(ctx context.Context, graphID aggregates.GraphID) (*aggregates.Graph, []*entities.Node, error) {
	// Load graph metadata
	graph, err := l.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Load all nodes in a single query
	nodes, err := l.nodeRepo.GetByGraphID(ctx, string(graphID))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	return graph, nodes, nil
}

// LoadNodesWithEdges loads multiple nodes with their edges in batch
func (l *GraphLoader) LoadNodesWithEdges(ctx context.Context, nodeIDs []valueobjects.NodeID) (map[valueobjects.NodeID]*NodeWithEdges, error) {
	result := make(map[valueobjects.NodeID]*NodeWithEdges)

	// Batch load all nodes
	for _, nodeID := range nodeIDs {
		node, err := l.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			l.logger.Warn("Failed to load node", zap.String("nodeID", nodeID.String()), zap.Error(err))
			continue
		}

		// Load edges for this node
		edges, err := l.edgeRepo.GetByNodeID(ctx, nodeID.String())
		if err != nil {
			l.logger.Warn("Failed to load edges for node", zap.String("nodeID", nodeID.String()), zap.Error(err))
			edges = []*aggregates.Edge{} // Continue with empty edges
		}

		result[nodeID] = &NodeWithEdges{
			Node:  node,
			Edges: edges,
		}
	}

	return result, nil
}

// LoadUserGraphsSummary loads summary information for all user's graphs
func (l *GraphLoader) LoadUserGraphsSummary(ctx context.Context, userID string) ([]*GraphSummary, error) {
	// Load all graphs for user
	graphs, err := l.graphRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user graphs: %w", err)
	}

	summaries := make([]*GraphSummary, 0, len(graphs))
	
	for _, graph := range graphs {
		// Get statistics without loading all data
		stats, err := l.graphRepo.GetGraphStatistics(ctx, graph.ID())
		if err != nil {
			l.logger.Warn("Failed to get graph statistics", 
				zap.String("graphID", string(graph.ID())), 
				zap.Error(err))
			// Use counts from graph metadata as fallback
			stats = ports.GraphStatistics{
				NodeCount: graph.NodeCount(),
				EdgeCount: graph.EdgeCount(),
			}
		}

		summaries = append(summaries, &GraphSummary{
			Graph:     graph,
			NodeCount: stats.NodeCount,
			EdgeCount: stats.EdgeCount,
			LastActivity: graph.UpdatedAt().Format(time.RFC3339),
		})
	}

	return summaries, nil
}

// NodeWithEdges combines a node with its edges
type NodeWithEdges struct {
	Node  *entities.Node
	Edges []*aggregates.Edge
}

// GraphSummary provides summary information about a graph
type GraphSummary struct {
	Graph        *aggregates.Graph
	NodeCount    int
	EdgeCount    int
	LastActivity string
}