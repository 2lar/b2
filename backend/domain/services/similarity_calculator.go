package services

import (
	"math"
	"strings"

	"backend/domain/core/entities"
)

// SimilarityCalculator calculates similarity between nodes
// This is a domain service that encapsulates similarity algorithms
type SimilarityCalculator interface {
	// Calculate calculates similarity between two nodes (0.0 to 1.0)
	Calculate(node1, node2 *entities.Node) float64
	
	// CalculateWithKeywords calculates similarity using pre-extracted keywords
	CalculateWithKeywords(node *entities.Node, keywords, tags map[string]bool) float64
	
	// CalculateBatch calculates similarities between a node and multiple candidates
	CalculateBatch(source *entities.Node, candidates []*entities.Node) map[string]float64
}

// SimilarityAlgorithm defines the algorithm to use
type SimilarityAlgorithm string

const (
	AlgorithmJaccard SimilarityAlgorithm = "jaccard"
	AlgorithmCosine  SimilarityAlgorithm = "cosine"
	AlgorithmHybrid  SimilarityAlgorithm = "hybrid"
)

// SimilarityConfig configures the similarity calculation
type SimilarityConfig struct {
	Algorithm       SimilarityAlgorithm
	TagWeight       float64 // Weight given to tag matches (0.0 to 1.0)
	KeywordWeight   float64 // Weight given to keyword matches (0.0 to 1.0)
	MinWordLength   int     // Minimum word length to consider
	UseStopWords    bool    // Whether to filter stop words
}

// DefaultSimilarityConfig returns a balanced default configuration
func DefaultSimilarityConfig() *SimilarityConfig {
	return &SimilarityConfig{
		Algorithm:     AlgorithmHybrid,
		TagWeight:     0.3,
		KeywordWeight: 0.7,
		MinWordLength: 3,
		UseStopWords:  true,
	}
}

// DefaultSimilarityCalculator provides similarity calculation using configurable algorithms
type DefaultSimilarityCalculator struct {
	config       *SimilarityConfig
	textAnalyzer TextAnalyzer
}

// NewDefaultSimilarityCalculator creates a new similarity calculator
func NewDefaultSimilarityCalculator(config *SimilarityConfig, textAnalyzer TextAnalyzer) *DefaultSimilarityCalculator {
	if config == nil {
		config = DefaultSimilarityConfig()
	}
	if textAnalyzer == nil {
		textAnalyzer = NewDefaultTextAnalyzer()
	}
	
	return &DefaultSimilarityCalculator{
		config:       config,
		textAnalyzer: textAnalyzer,
	}
}

// Calculate calculates similarity between two nodes
func (sc *DefaultSimilarityCalculator) Calculate(node1, node2 *entities.Node) float64 {
	if node1 == nil || node2 == nil {
		return 0.0
	}
	
	// Extract features from both nodes
	keywords1 := sc.extractNodeKeywords(node1)
	keywords2 := sc.extractNodeKeywords(node2)
	
	tags1 := sc.extractNodeTags(node1)
	tags2 := sc.extractNodeTags(node2)
	
	// Calculate keyword similarity
	keywordSim := sc.calculateSetSimilarity(keywords1, keywords2)
	
	// Calculate tag similarity
	tagSim := sc.calculateSetSimilarity(tags1, tags2)
	
	// Combine with weights
	totalSim := (keywordSim * sc.config.KeywordWeight) + (tagSim * sc.config.TagWeight)
	
	// Normalize to 0-1 range
	return math.Min(totalSim, 1.0)
}

// CalculateWithKeywords calculates similarity using pre-extracted keywords
func (sc *DefaultSimilarityCalculator) CalculateWithKeywords(node *entities.Node, keywords, tags map[string]bool) float64 {
	if node == nil || (len(keywords) == 0 && len(tags) == 0) {
		return 0.0
	}
	
	// Extract features from the node
	nodeKeywords := sc.extractNodeKeywords(node)
	nodeTags := sc.extractNodeTags(node)
	
	// Calculate similarities
	keywordSim := sc.calculateSetSimilarity(nodeKeywords, keywords)
	tagSim := sc.calculateSetSimilarity(nodeTags, tags)
	
	// Combine with weights
	totalSim := (keywordSim * sc.config.KeywordWeight) + (tagSim * sc.config.TagWeight)
	
	return math.Min(totalSim, 1.0)
}

// CalculateBatch calculates similarities between a node and multiple candidates
func (sc *DefaultSimilarityCalculator) CalculateBatch(source *entities.Node, candidates []*entities.Node) map[string]float64 {
	results := make(map[string]float64)
	
	if source == nil || len(candidates) == 0 {
		return results
	}
	
	// Pre-extract source features for efficiency
	sourceKeywords := sc.extractNodeKeywords(source)
	sourceTags := sc.extractNodeTags(source)
	
	// Calculate similarity for each candidate
	for _, candidate := range candidates {
		if candidate == nil || candidate.ID() == source.ID() {
			continue
		}
		
		sim := sc.CalculateWithKeywords(candidate, sourceKeywords, sourceTags)
		results[candidate.ID().String()] = sim
	}
	
	return results
}

// extractNodeKeywords extracts keywords from node content
func (sc *DefaultSimilarityCalculator) extractNodeKeywords(node *entities.Node) map[string]bool {
	content := node.Content()
	text := content.Title() + " " + content.Body()
	
	if sc.config.UseStopWords {
		keywords := sc.textAnalyzer.ExtractKeywords(text)
		keywordSet := make(map[string]bool)
		for _, kw := range keywords {
			if len(kw) >= sc.config.MinWordLength {
				keywordSet[strings.ToLower(kw)] = true
			}
		}
		return keywordSet
	}
	
	return sc.textAnalyzer.TokenizeWords(text)
}

// extractNodeTags extracts and normalizes node tags
func (sc *DefaultSimilarityCalculator) extractNodeTags(node *entities.Node) map[string]bool {
	tags := node.GetTags()
	tagSet := make(map[string]bool)
	
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			tagSet[normalized] = true
		}
	}
	
	return tagSet
}

// calculateSetSimilarity calculates Jaccard similarity between two sets
func (sc *DefaultSimilarityCalculator) calculateSetSimilarity(set1, set2 map[string]bool) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 0.0
	}
	
	switch sc.config.Algorithm {
	case AlgorithmJaccard:
		return sc.jaccardSimilarity(set1, set2)
	case AlgorithmCosine:
		return sc.cosineSimilarity(set1, set2)
	case AlgorithmHybrid:
		// Combine both algorithms
		jaccard := sc.jaccardSimilarity(set1, set2)
		cosine := sc.cosineSimilarity(set1, set2)
		return (jaccard + cosine) / 2.0
	default:
		return sc.jaccardSimilarity(set1, set2)
	}
}

// jaccardSimilarity calculates Jaccard index: |A ∩ B| / |A ∪ B|
func (sc *DefaultSimilarityCalculator) jaccardSimilarity(set1, set2 map[string]bool) float64 {
	intersection := 0
	union := make(map[string]bool)
	
	// Count intersection and build union
	for key := range set1 {
		union[key] = true
		if set2[key] {
			intersection++
		}
	}
	
	for key := range set2 {
		union[key] = true
	}
	
	if len(union) == 0 {
		return 0.0
	}
	
	return float64(intersection) / float64(len(union))
}

// cosineSimilarity calculates cosine similarity between two sets (treating as binary vectors)
func (sc *DefaultSimilarityCalculator) cosineSimilarity(set1, set2 map[string]bool) float64 {
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}
	
	// Calculate dot product (intersection count)
	dotProduct := 0
	for key := range set1 {
		if set2[key] {
			dotProduct++
		}
	}
	
	// Calculate magnitudes (sqrt of set sizes for binary vectors)
	magnitude1 := math.Sqrt(float64(len(set1)))
	magnitude2 := math.Sqrt(float64(len(set2)))
	
	if magnitude1 == 0 || magnitude2 == 0 {
		return 0.0
	}
	
	return float64(dotProduct) / (magnitude1 * magnitude2)
}