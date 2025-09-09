package queries

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/domain/core/entities"
)

// FindSimilarNodesQuery represents a query to find similar nodes
type FindSimilarNodesQuery struct {
	NodeID         string `json:"node_id"`
	UserID         string `json:"user_id"`
	MaxResults     int    `json:"max_results"`
	SimilarityType string `json:"similarity_type"` // semantic, keyword, temporal
}

// Validate validates the query
func (q *FindSimilarNodesQuery) Validate() error {
	if q.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if q.MaxResults <= 0 {
		q.MaxResults = 10
	}
	if q.SimilarityType == "" {
		q.SimilarityType = "semantic"
	}
	return nil
}

// SimilarNode represents a node with similarity score
type SimilarNode struct {
	Node       *entities.Node `json:"node"`
	Similarity float64        `json:"similarity"`
	Reason     string         `json:"reason"`
}

// FindSimilarNodesResult represents the result of finding similar nodes
type FindSimilarNodesResult struct {
	Nodes      []SimilarNode `json:"nodes"`
	TotalFound int           `json:"total_found"`
}

// FindSimilarNodesHandler handles finding similar nodes
type FindSimilarNodesHandler struct {
	nodeRepo ports.NodeRepository
}

// NewFindSimilarNodesHandler creates a new handler
func NewFindSimilarNodesHandler(nodeRepo ports.NodeRepository) *FindSimilarNodesHandler {
	return &FindSimilarNodesHandler{
		nodeRepo: nodeRepo,
	}
}

// Handle executes the query
func (h *FindSimilarNodesHandler) Handle(ctx context.Context, query interface{}) (interface{}, error) {
	q, ok := query.(*FindSimilarNodesQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	// Get the reference node
	// Note: This is simplified - the actual nodeRepo.GetByID might need adjustment
	// to work with string IDs instead of NodeID value objects

	// Get all nodes for the user (simplified approach)
	// In a real implementation, this would use a specialized similarity search
	allNodes, err := h.nodeRepo.GetByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	var similarNodes []SimilarNode

	// Filter out the reference node and calculate similarity
	for _, node := range allNodes {
		// Skip the reference node itself
		// Note: Need to access node ID - this depends on the Node entity implementation
		// For now, skip this check
		// if node.ID.String() == q.NodeID {
		//     continue
		// }

		// Calculate similarity based on type
		similarity := h.calculateSimilarity(node, q.SimilarityType)

		if similarity > 0 {
			similarNodes = append(similarNodes, SimilarNode{
				Node:       node,
				Similarity: similarity,
				Reason:     fmt.Sprintf("%s similarity", q.SimilarityType),
			})
		}
	}

	// Sort by similarity (simplified - just return as is)
	// In production, would sort by similarity score descending

	// Limit results
	if len(similarNodes) > q.MaxResults {
		similarNodes = similarNodes[:q.MaxResults]
	}

	return &FindSimilarNodesResult{
		Nodes:      similarNodes,
		TotalFound: len(similarNodes),
	}, nil
}

// calculateSimilarity calculates similarity between nodes
func (h *FindSimilarNodesHandler) calculateSimilarity(node *entities.Node, similarityType string) float64 {
	// This is a placeholder implementation
	// Real implementation would use:
	// - Vector embeddings for semantic similarity
	// - Keyword matching for keyword similarity
	// - Time-based calculations for temporal similarity

	switch similarityType {
	case "semantic":
		// Would use embedding vectors here
		return 0.5
	case "keyword":
		// Would compare keywords/tags
		return 0.3
	case "temporal":
		// Would check creation time proximity
		return 0.2
	default:
		return 0.0
	}
}

// SearchNodesQuery represents a query to search for nodes
type SearchNodesQuery struct {
	UserID   string   `json:"user_id"`
	Keywords []string `json:"keywords"`
	Tags     []string `json:"tags"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
}

// Validate validates the query
func (q *SearchNodesQuery) Validate() error {
	if q.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if len(q.Keywords) == 0 && len(q.Tags) == 0 {
		return fmt.Errorf("at least one keyword or tag is required")
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	return nil
}

// SearchNodesResult represents the search result
type SearchNodesResult struct {
	Nodes      []*entities.Node `json:"nodes"`
	TotalCount int              `json:"total_count"`
	Offset     int              `json:"offset"`
}

// SearchNodesHandler handles node search queries
type SearchNodesHandler struct {
	nodeRepo ports.NodeRepository
}

// NewSearchNodesHandler creates a new search handler
func NewSearchNodesHandler(nodeRepo ports.NodeRepository) *SearchNodesHandler {
	return &SearchNodesHandler{
		nodeRepo: nodeRepo,
	}
}

// Handle executes the search query
func (h *SearchNodesHandler) Handle(ctx context.Context, query interface{}) (interface{}, error) {
	q, ok := query.(*SearchNodesQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type")
	}

	// Get all nodes for the user (simplified)
	// Real implementation would use a search index
	allNodes, err := h.nodeRepo.GetByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}

	// Filter by keywords and tags (simplified)
	var matchedNodes []*entities.Node
	for _, node := range allNodes {
		// Check if node matches search criteria
		// This is simplified - real implementation would check
		// node content, tags, etc.
		matchedNodes = append(matchedNodes, node)
	}

	// Apply pagination
	start := q.Offset
	end := q.Offset + q.Limit
	if start > len(matchedNodes) {
		start = len(matchedNodes)
	}
	if end > len(matchedNodes) {
		end = len(matchedNodes)
	}

	paginatedNodes := matchedNodes[start:end]

	return &SearchNodesResult{
		Nodes:      paginatedNodes,
		TotalCount: len(matchedNodes),
		Offset:     q.Offset,
	}, nil
}
