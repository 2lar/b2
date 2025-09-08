package queries

import (
	"context"
	"errors"

	"backend2/application/ports"
	"backend2/domain/core/aggregates"
)

// GetGraphQuery represents a query to retrieve a graph
type GetGraphQuery struct {
	GraphID string `json:"graph_id"`
	UserID  string `json:"user_id"`
}

// GetGraphResult represents the query result
type GetGraphResult struct {
	Graph     *aggregates.Graph       `json:"graph"`
	Nodes     []NodeDTO              `json:"nodes"`
	Edges     []EdgeDTO              `json:"edges"`
	Metadata  GraphMetadataDTO       `json:"metadata"`
}

// NodeDTO is a data transfer object for nodes
type NodeDTO struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Position  PositionDTO            `json:"position"`
	Status    string                 `json:"status"`
	Tags      []string              `json:"tags"`
	CreatedAt string                `json:"created_at"`
	UpdatedAt string                `json:"updated_at"`
}

// EdgeDTO is a data transfer object for edges
type EdgeDTO struct {
	ID       string  `json:"id"`
	SourceID string  `json:"source_id"`
	TargetID string  `json:"target_id"`
	Type     string  `json:"type"`
	Weight   float64 `json:"weight"`
}

// PositionDTO represents node position
type PositionDTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// GraphMetadataDTO represents graph metadata
type GraphMetadataDTO struct {
	NodeCount int      `json:"node_count"`
	EdgeCount int      `json:"edge_count"`
	IsPublic  bool     `json:"is_public"`
	Tags      []string `json:"tags"`
}

// GetGraphHandler handles the GetGraphQuery
type GetGraphHandler struct {
	graphRepo ports.GraphRepository
	cache     ports.Cache
}

// NewGetGraphHandler creates a new handler instance
func NewGetGraphHandler(graphRepo ports.GraphRepository, cache ports.Cache) *GetGraphHandler {
	return &GetGraphHandler{
		graphRepo: graphRepo,
		cache:     cache,
	}
}

// Handle executes the get graph query
func (h *GetGraphHandler) Handle(ctx context.Context, query GetGraphQuery) (*GetGraphResult, error) {
	// Check cache first
	cacheKey := "graph:" + query.GraphID
	if cached, found := h.cache.Get(ctx, cacheKey); found {
		if result, ok := cached.(*GetGraphResult); ok {
			return result, nil
		}
	}
	
	// Load graph from repository
	graph, err := h.graphRepo.GetByID(ctx, aggregates.GraphID(query.GraphID))
	if err != nil {
		return nil, err
	}
	
	// Verify user has access
	if graph.UserID() != query.UserID {
		// Check if graph is public
		// For now, we'll return an error
		return nil, errors.New("access denied")
	}
	
	// Get nodes safely
	nodes, err := graph.GetNodes()
	if err != nil {
		return nil, err
	}

	// Convert to DTOs
	result := &GetGraphResult{
		Graph: graph,
		Nodes: make([]NodeDTO, 0),
		Edges: make([]EdgeDTO, 0),
		Metadata: GraphMetadataDTO{
			NodeCount: len(nodes),
			EdgeCount: len(graph.GetEdges()),
			IsPublic:  false, // TODO: Get from graph metadata
			Tags:      []string{},
		},
	}
	
	// Convert nodes to DTOs
	for _, node := range nodes {
		dto := NodeDTO{
			ID:      node.ID().String(),
			Title:   node.Content().Title(),
			Content: node.Content().Body(),
			Position: PositionDTO{
				X: node.Position().X(),
				Y: node.Position().Y(),
				Z: node.Position().Z(),
			},
			Status:    string(node.Status()),
			Tags:      node.GetTags(),
			CreatedAt: node.CreatedAt().Format("2006-01-02T15:04:05Z"),
			UpdatedAt: node.UpdatedAt().Format("2006-01-02T15:04:05Z"),
		}
		result.Nodes = append(result.Nodes, dto)
	}
	
	// Convert edges to DTOs
	for _, edge := range graph.GetEdges() {
		dto := EdgeDTO{
			ID:       edge.ID,
			SourceID: edge.SourceID.String(),
			TargetID: edge.TargetID.String(),
			Type:     string(edge.Type),
			Weight:   edge.Weight,
		}
		result.Edges = append(result.Edges, dto)
	}
	
	// Cache the result for 5 minutes
	h.cache.Set(ctx, cacheKey, result, 300)
	
	return result, nil
}

// Validate validates the query
func (q GetGraphQuery) Validate() error {
	if q.GraphID == "" {
		return errors.New("graph ID is required")
	}
	if q.UserID == "" {
		return errors.New("user ID is required")
	}
	return nil
}