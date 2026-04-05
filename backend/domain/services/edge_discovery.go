package services

import (
	"sort"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
)

type EdgeCandidate = aggregates.EdgeCandidate
type EdgeDiscoveryService = aggregates.EdgeDiscoveryService

// EdgeDiscoveryConfig configures edge discovery behavior.
type EdgeDiscoveryConfig struct {
	MinSimilarity         float64
	StrongEdgeThreshold   float64
	NormalEdgeThreshold   float64
	MaxEdgesPerNode       int
	ConsiderBidirectional bool
}

func DefaultEdgeDiscoveryConfig() *EdgeDiscoveryConfig {
	return &EdgeDiscoveryConfig{
		MinSimilarity:         0.3,
		StrongEdgeThreshold:   0.75,
		NormalEdgeThreshold:   0.5,
		MaxEdgesPerNode:       50,
		ConsiderBidirectional: true,
	}
}

// DefaultEdgeDiscoveryService discovers edges using the hybrid similarity calculator.
type DefaultEdgeDiscoveryService struct {
	config               *EdgeDiscoveryConfig
	similarityCalculator *HybridSimilarityCalculator
}

func NewDefaultEdgeDiscoveryService(
	config *EdgeDiscoveryConfig,
	similarityCalculator *HybridSimilarityCalculator,
) *DefaultEdgeDiscoveryService {
	if config == nil {
		config = DefaultEdgeDiscoveryConfig()
	}
	if similarityCalculator == nil {
		similarityCalculator = NewHybridSimilarityCalculator(nil, nil)
	}
	return &DefaultEdgeDiscoveryService{
		config:               config,
		similarityCalculator: similarityCalculator,
	}
}

func (eds *DefaultEdgeDiscoveryService) DiscoverPotentialEdges(
	node *entities.Node,
	graph *aggregates.Graph,
) []EdgeCandidate {
	if node == nil || graph == nil {
		return nil
	}

	existingNodes, err := graph.GetNodes()
	if err != nil || len(existingNodes) <= 1 {
		return nil
	}

	results := eds.similarityCalculator.CalculateBatchDetailed(node, existingNodes)
	candidates := make([]EdgeCandidate, 0)

	for _, targetNode := range existingNodes {
		if targetNode.ID() == node.ID() {
			continue
		}

		result, exists := results[targetNode.ID().String()]
		if !exists || result.Score < eds.config.MinSimilarity {
			continue
		}

		edgeType := eds.classifyEdgeType(result.Score)
		reason := eds.generateReason(result)

		candidate := aggregates.EdgeCandidate{
			SourceID:   node.ID(),
			TargetID:   targetNode.ID(),
			Type:       edgeType,
			Similarity: result.Score,
			Reason:     reason,
		}
		candidates = append(candidates, candidate)

		if eds.config.ConsiderBidirectional && edgeType == entities.EdgeTypeStrong {
			candidates = append(candidates, aggregates.EdgeCandidate{
				SourceID:   targetNode.ID(),
				TargetID:   node.ID(),
				Type:       edgeType,
				Similarity: result.Score,
				Reason:     "Bidirectional strong connection",
			})
		}
	}

	return candidates
}

func (eds *DefaultEdgeDiscoveryService) RankEdges(candidates []EdgeCandidate) []EdgeCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	ranked := make([]EdgeCandidate, len(candidates))
	copy(ranked, candidates)

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Similarity != ranked[j].Similarity {
			return ranked[i].Similarity > ranked[j].Similarity
		}
		return eds.edgeTypePriority(ranked[i].Type) > eds.edgeTypePriority(ranked[j].Type)
	})

	return ranked
}

func (eds *DefaultEdgeDiscoveryService) FilterEdges(
	candidates []EdgeCandidate,
	maxEdges int,
	minSimilarity float64,
) []EdgeCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	if maxEdges <= 0 {
		maxEdges = eds.config.MaxEdgesPerNode
	}
	if minSimilarity <= 0 {
		minSimilarity = eds.config.MinSimilarity
	}

	filtered := make([]EdgeCandidate, 0)
	edgesPerNode := make(map[string]int)

	for _, candidate := range candidates {
		if candidate.Similarity < minSimilarity {
			continue
		}
		sourceKey := candidate.SourceID.String()
		if edgesPerNode[sourceKey] >= maxEdges {
			continue
		}
		filtered = append(filtered, candidate)
		edgesPerNode[sourceKey]++
	}

	return filtered
}

func (eds *DefaultEdgeDiscoveryService) ClassifyEdgeType(similarity float64) entities.EdgeType {
	return eds.classifyEdgeType(similarity)
}

func (eds *DefaultEdgeDiscoveryService) classifyEdgeType(similarity float64) entities.EdgeType {
	switch {
	case similarity >= eds.config.StrongEdgeThreshold:
		return entities.EdgeTypeStrong
	case similarity >= eds.config.NormalEdgeThreshold:
		return entities.EdgeTypeNormal
	default:
		return entities.EdgeTypeWeak
	}
}

func (eds *DefaultEdgeDiscoveryService) generateReason(result SimilarityResult) string {
	methodLabel := ""
	switch result.Method {
	case "hybrid":
		methodLabel = " (semantic + keyword)"
	case "semantic":
		methodLabel = " (semantic)"
	case "keyword":
		methodLabel = " (keyword)"
	}

	switch {
	case result.Score >= 0.9:
		return "Very high content similarity" + methodLabel
	case result.Score >= eds.config.StrongEdgeThreshold:
		return "Strong content relationship" + methodLabel
	case result.Score >= eds.config.NormalEdgeThreshold:
		return "Moderate content similarity" + methodLabel
	default:
		return "Related content" + methodLabel
	}
}

func (eds *DefaultEdgeDiscoveryService) edgeTypePriority(edgeType entities.EdgeType) int {
	switch edgeType {
	case entities.EdgeTypeStrong:
		return 3
	case entities.EdgeTypeNormal:
		return 2
	case entities.EdgeTypeWeak:
		return 1
	default:
		return 0
	}
}
