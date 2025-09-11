package services

import (
	"sort"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
)

// EdgeCandidate is now defined in aggregates package to avoid circular dependency
type EdgeCandidate = aggregates.EdgeCandidate

// EdgeDiscoveryService discovers potential edges between nodes
// This interface is also defined in aggregates package to avoid circular dependency
// This concrete implementation satisfies the aggregates.EdgeDiscoveryService interface
type EdgeDiscoveryService = aggregates.EdgeDiscoveryService

// EdgeDiscoveryConfig configures edge discovery behavior
type EdgeDiscoveryConfig struct {
	MinSimilarity       float64 // Minimum similarity threshold for edge creation
	StrongEdgeThreshold float64 // Similarity threshold for strong edges
	MaxEdgesPerNode     int     // Maximum edges to create per node
	ConsiderBidirectional bool  // Whether to create bidirectional edges
}

// DefaultEdgeDiscoveryConfig returns default configuration
func DefaultEdgeDiscoveryConfig() *EdgeDiscoveryConfig {
	return &EdgeDiscoveryConfig{
		MinSimilarity:       0.3,
		StrongEdgeThreshold: 0.7,
		MaxEdgesPerNode:     50,
		ConsiderBidirectional: true,
	}
}

// DefaultEdgeDiscoveryService provides edge discovery using similarity calculation
type DefaultEdgeDiscoveryService struct {
	config               *EdgeDiscoveryConfig
	similarityCalculator SimilarityCalculator
}

// NewDefaultEdgeDiscoveryService creates a new edge discovery service
func NewDefaultEdgeDiscoveryService(
	config *EdgeDiscoveryConfig,
	similarityCalculator SimilarityCalculator,
) *DefaultEdgeDiscoveryService {
	if config == nil {
		config = DefaultEdgeDiscoveryConfig()
	}
	if similarityCalculator == nil {
		similarityCalculator = NewDefaultSimilarityCalculator(nil, nil)
	}
	
	return &DefaultEdgeDiscoveryService{
		config:               config,
		similarityCalculator: similarityCalculator,
	}
}

// DiscoverPotentialEdges finds all potential edges for a node within a graph
func (eds *DefaultEdgeDiscoveryService) DiscoverPotentialEdges(
	node *entities.Node,
	graph *aggregates.Graph,
) []EdgeCandidate {
	if node == nil || graph == nil {
		return nil
	}
	
	candidates := make([]EdgeCandidate, 0)
	
	// Get all nodes in the graph
	existingNodes, err := graph.GetNodes()
	if err != nil || len(existingNodes) <= 1 {
		return candidates
	}
	
	// Calculate similarity with each node
	similarities := eds.similarityCalculator.CalculateBatch(node, existingNodes)
	
	// Create edge candidates for nodes above threshold
	for _, targetNode := range existingNodes {
		// Skip self and nodes without sufficient similarity
		if targetNode.ID() == node.ID() {
			continue
		}
		
		similarity, exists := similarities[targetNode.ID().String()]
		if !exists || similarity < eds.config.MinSimilarity {
			continue
		}
		
		// Determine edge type based on similarity strength
		edgeType := eds.ClassifyEdgeType(similarity)
		
		// Create candidate  
		candidate := aggregates.EdgeCandidate{
			SourceID:   node.ID(),
			TargetID:   targetNode.ID(),
			Type:       edgeType,
			Similarity: similarity,
			Reason:     eds.generateReason(similarity, edgeType),
		}
		
		candidates = append(candidates, candidate)
		
		// Add reverse edge if bidirectional
		if eds.config.ConsiderBidirectional && edgeType == entities.EdgeTypeStrong {
			reverseCandidate := aggregates.EdgeCandidate{
				SourceID:   targetNode.ID(),
				TargetID:   node.ID(),
				Type:       edgeType,
				Similarity: similarity,
				Reason:     "Bidirectional strong connection",
			}
			candidates = append(candidates, reverseCandidate)
		}
	}
	
	return candidates
}

// RankEdges sorts edges by relevance/importance
func (eds *DefaultEdgeDiscoveryService) RankEdges(candidates []EdgeCandidate) []EdgeCandidate {
	if len(candidates) <= 1 {
		return candidates
	}
	
	// Create a copy to avoid modifying the original
	ranked := make([]EdgeCandidate, len(candidates))
	copy(ranked, candidates)
	
	// Sort by similarity (highest first), then by edge type strength
	sort.Slice(ranked, func(i, j int) bool {
		// First compare by similarity
		if ranked[i].Similarity != ranked[j].Similarity {
			return ranked[i].Similarity > ranked[j].Similarity
		}
		
		// Then by edge type (strong edges first)
		return eds.edgeTypePriority(ranked[i].Type) > eds.edgeTypePriority(ranked[j].Type)
	})
	
	return ranked
}

// FilterEdges applies business rules to filter edge candidates
func (eds *DefaultEdgeDiscoveryService) FilterEdges(
	candidates []EdgeCandidate,
	maxEdges int,
	minSimilarity float64,
) []EdgeCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	
	// Use config values if not specified
	if maxEdges <= 0 {
		maxEdges = eds.config.MaxEdgesPerNode
	}
	if minSimilarity <= 0 {
		minSimilarity = eds.config.MinSimilarity
	}
	
	filtered := make([]EdgeCandidate, 0)
	
	// Track edges per source node to enforce limits
	edgesPerNode := make(map[string]int)
	
	for _, candidate := range candidates {
		// Skip if below minimum similarity
		if candidate.Similarity < minSimilarity {
			continue
		}
		
		// Check if source node has reached edge limit
		sourceKey := candidate.SourceID.String()
		if edgesPerNode[sourceKey] >= maxEdges {
			continue
		}
		
		filtered = append(filtered, candidate)
		edgesPerNode[sourceKey]++
	}
	
	return filtered
}

// ClassifyEdgeType determines the appropriate edge type based on similarity
func (eds *DefaultEdgeDiscoveryService) ClassifyEdgeType(similarity float64) entities.EdgeType {
	if similarity >= eds.config.StrongEdgeThreshold {
		return entities.EdgeTypeStrong
	}
	return entities.EdgeTypeWeak
}

// generateReason creates a human-readable reason for the edge
func (eds *DefaultEdgeDiscoveryService) generateReason(similarity float64, edgeType entities.EdgeType) string {
	switch {
	case similarity >= 0.9:
		return "Very high content similarity"
	case similarity >= eds.config.StrongEdgeThreshold:
		return "Strong content relationship"
	case similarity >= 0.5:
		return "Moderate content similarity"
	default:
		return "Related content"
	}
}

// edgeTypePriority returns priority value for edge types (higher = more important)
func (eds *DefaultEdgeDiscoveryService) edgeTypePriority(edgeType entities.EdgeType) int {
	switch edgeType {
	case entities.EdgeTypeStrong:
		return 3
	case entities.EdgeTypeWeak:
		return 2
	case entities.EdgeTypeReference:
		return 1
	default:
		return 0
	}
}