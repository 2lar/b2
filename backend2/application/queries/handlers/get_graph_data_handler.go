package handlers

import (
	"context"
	"fmt"
	"time"

	"backend2/application/ports"
	"backend2/application/queries"
	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
	"go.uber.org/zap"
)

// GetGraphDataHandler handles graph data visualization queries
type GetGraphDataHandler struct {
	graphRepo ports.GraphRepository
	nodeRepo  ports.NodeRepository
	edgeRepo  ports.EdgeRepository
	logger    *zap.Logger
}

// NewGetGraphDataHandler creates a new graph data handler
func NewGetGraphDataHandler(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *GetGraphDataHandler {
	return &GetGraphDataHandler{
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		logger:    logger,
	}
}

// Handle executes the graph data query
func (h *GetGraphDataHandler) Handle(ctx context.Context, query queries.GetGraphDataQuery) (*queries.GetGraphDataResult, error) {
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Get the graph - either specific one or user's default
	var graph *aggregates.Graph
	var err error

	if query.GraphID != "" {
		graphID := aggregates.GraphID(query.GraphID)
		graph, err = h.graphRepo.GetByID(ctx, graphID)
		if err != nil {
			return nil, fmt.Errorf("failed to get graph: %w", err)
		}

		// Verify ownership
		if graph.UserID() != query.UserID {
			return nil, fmt.Errorf("graph does not belong to user")
		}
	} else {
		// Get user's default graph
		graph, err = h.graphRepo.GetUserDefaultGraph(ctx, query.UserID)
		if err != nil {
			// If no default graph exists, create an empty result
			h.logger.Warn("No default graph found, returning empty graph data",
				zap.String("userID", query.UserID),
			)
			return &queries.GetGraphDataResult{
				Nodes: []queries.GraphNode{},
				Edges: []queries.GraphEdge{},
				Stats: queries.GraphStats{
					NodeCount:    0,
					EdgeCount:    0,
					ClusterCount: 0,
					Density:      0,
				},
			}, nil
		}
	}

	// Get nodes for the specific graph
	nodes, err := h.nodeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		h.logger.Error("Failed to get nodes for graph data",
			zap.String("graphID", graph.ID().String()),
			zap.String("userID", query.UserID),
			zap.Error(err),
		)
		nodes = []*entities.Node{}
	}

	// Get edges from the edge repository
	edges, err := h.edgeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		h.logger.Error("Failed to get edges for graph data",
			zap.String("graphID", graph.ID().String()),
			zap.String("userID", query.UserID),
			zap.Error(err),
		)
		edges = []*aggregates.Edge{}
	}

	// Build the result
	result := &queries.GetGraphDataResult{
		Nodes: make([]queries.GraphNode, 0, len(nodes)),
		Edges: make([]queries.GraphEdge, 0, len(edges)),
		Stats: queries.GraphStats{
			NodeCount: len(nodes),
			EdgeCount: len(edges),
		},
	}

	// Convert nodes to graph nodes
	for _, node := range nodes {
		content := node.Content()
		position := node.Position()

		graphNode := queries.GraphNode{
			ID:      node.ID().String(),
			Title:   content.Title(),
			Content: content.Body(),
			Position: queries.Position{
				X: position.X(),
				Y: position.Y(),
				Z: position.Z(),
			},
			Tags: node.GetTags(),
			Metadata: map[string]string{
				"created_at": node.CreatedAt().Format(time.RFC3339),
				"updated_at": node.UpdatedAt().Format(time.RFC3339),
				"status":     string(node.Status()),
			},
		}
		result.Nodes = append(result.Nodes, graphNode)
	}

	// Convert edges from the edge repository
	for _, edge := range edges {
		graphEdge := queries.GraphEdge{
			ID:       edge.ID,
			SourceID: edge.SourceID.String(),
			TargetID: edge.TargetID.String(),
			Type:     string(edge.Type),
			Weight:   edge.Weight,
			Metadata: edge.Metadata,
		}
		result.Edges = append(result.Edges, graphEdge)
	}

	// Calculate graph density if we have nodes
	if len(result.Nodes) > 1 {
		maxPossibleEdges := len(result.Nodes) * (len(result.Nodes) - 1) / 2
		if maxPossibleEdges > 0 {
			result.Stats.Density = float64(len(result.Edges)) / float64(maxPossibleEdges)
		}
	}

	// Calculate clusters (simplified - count connected components)
	clusters := h.calculateClusters(graph)
	result.Stats.ClusterCount = len(clusters)

	h.logger.Debug("Graph data retrieved",
		zap.String("graphID", graph.ID().String()),
		zap.String("userID", query.UserID),
		zap.Int("nodeCount", result.Stats.NodeCount),
		zap.Int("edgeCount", result.Stats.EdgeCount),
	)

	return result, nil
}

// calculateClusters calculates the number of connected components in the graph
func (h *GetGraphDataHandler) calculateClusters(graph *aggregates.Graph) [][]string {
	// Use the graph's built-in GetClusters method
	clusters := graph.GetClusters()

	// Convert to string arrays
	result := make([][]string, len(clusters))
	for i, cluster := range clusters {
		strCluster := make([]string, len(cluster))
		for j, nodeID := range cluster {
			strCluster[j] = nodeID.String()
		}
		result[i] = strCluster
	}

	return result
}
