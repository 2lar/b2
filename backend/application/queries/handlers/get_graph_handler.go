package handlers

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"go.uber.org/zap"
)

// GetGraphHandler handles get graph queries
type GetGraphHandler struct {
	graphRepo ports.GraphRepository
	nodeRepo  ports.NodeRepository
	logger    *zap.Logger
}

// NewGetGraphHandler creates a new get graph handler
func NewGetGraphHandler(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	logger *zap.Logger,
) *GetGraphHandler {
	return &GetGraphHandler{
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		logger:    logger,
	}
}

// Handle executes the get graph query
func (h *GetGraphHandler) Handle(ctx context.Context, query queries.GetGraphByIDQuery) (*queries.GetGraphByIDResult, error) {
	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Get graph from repository
	graphID := aggregates.GraphID(query.GraphID)
	graph, err := h.graphRepo.GetByID(ctx, graphID)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph: %w", err)
	}

	// Verify ownership
	if graph.UserID() != query.UserID {
		return nil, fmt.Errorf("graph does not belong to user")
	}

	// Get all nodes for this graph
	nodes, err := h.nodeRepo.GetByUserID(ctx, query.UserID)
	if err != nil {
		h.logger.Warn("Failed to get nodes for graph", zap.Error(err))
		nodes = []*entities.Node{}
	}

	// Map to result
	result := &queries.GetGraphByIDResult{
		ID:          graph.ID().String(),
		UserID:      graph.UserID(),
		Name:        graph.Name(),
		Description: graph.Description(),
		NodeCount:   len(nodes),
		EdgeCount:   len(graph.Edges()),
		Nodes:       make([]queries.GraphNode, 0, len(nodes)),
		Edges:       make([]queries.GraphEdge, 0),
		Metadata:    graph.Metadata(),
		CreatedAt:   graph.CreatedAt().Format(time.RFC3339),
		UpdatedAt:   graph.UpdatedAt().Format(time.RFC3339),
	}

	// Convert nodes
	for _, node := range nodes {
		content := node.Content()
		position := node.Position()

		result.Nodes = append(result.Nodes, queries.GraphNode{
			ID:      node.ID().String(),
			Title:   content.Title(),
			Content: content.Body(),
			Position: queries.Position{
				X: position.X(),
				Y: position.Y(),
				Z: position.Z(),
			},
			Tags:     node.GetTags(),
			Metadata: make(map[string]string),
		})
	}

	// Convert edges
	for _, edge := range graph.Edges() {
		result.Edges = append(result.Edges, queries.GraphEdge{
			ID:       edge.ID,
			SourceID: edge.SourceID.String(),
			TargetID: edge.TargetID.String(),
			Type:     string(edge.Type),
			Weight:   edge.Weight,
			Metadata: edge.Metadata,
		})
	}

	h.logger.Debug("Graph retrieved",
		zap.String("graphID", query.GraphID),
		zap.String("userID", query.UserID),
		zap.Int("nodeCount", result.NodeCount),
		zap.Int("edgeCount", result.EdgeCount),
	)

	return result, nil
}
