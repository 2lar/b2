package services

import (
	"math"
	"strings"

	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// SimilarityCalculator calculates similarity between nodes
type SimilarityCalculator interface {
	// Calculate returns a similarity score (0.0 to 1.0) between two nodes.
	Calculate(node1, node2 *entities.Node) float64

	// CalculateWithKeywords calculates similarity using pre-extracted keywords.
	CalculateWithKeywords(node *entities.Node, keywords, tags map[string]bool) float64

	// CalculateBatch calculates similarities between a source node and multiple candidates.
	CalculateBatch(source *entities.Node, candidates []*entities.Node) map[string]float64
}

// SimilarityResult holds a similarity score along with metadata about how it was computed.
type SimilarityResult struct {
	Score      float64
	Confidence float64 // Higher when both keyword and semantic signals are available
	Method     string  // "hybrid", "semantic", "keyword"
}

// HybridSimilarityConfig configures the hybrid similarity calculator.
type HybridSimilarityConfig struct {
	SemanticWeight float64 // Weight for embedding cosine similarity (0.0 to 1.0)
	KeywordWeight  float64 // Weight for keyword-based similarity (0.0 to 1.0)
	TagWeight      float64 // Weight for tag overlap within the keyword component
	MinWordLength  int
	UseStopWords   bool
}

// DefaultHybridSimilarityConfig returns defaults that favor semantic signal.
func DefaultHybridSimilarityConfig() *HybridSimilarityConfig {
	return &HybridSimilarityConfig{
		SemanticWeight: 0.6,
		KeywordWeight:  0.4,
		TagWeight:      0.3, // Within the keyword component, 30% tags, 70% keywords
		MinWordLength:  3,
		UseStopWords:   true,
	}
}

// HybridSimilarityCalculator blends semantic (embedding) and keyword similarity.
// When embeddings are unavailable, it falls back to keyword-only similarity.
type HybridSimilarityCalculator struct {
	config       *HybridSimilarityConfig
	textAnalyzer TextAnalyzer
}

func NewHybridSimilarityCalculator(config *HybridSimilarityConfig, textAnalyzer TextAnalyzer) *HybridSimilarityCalculator {
	if config == nil {
		config = DefaultHybridSimilarityConfig()
	}
	if textAnalyzer == nil {
		textAnalyzer = NewDefaultTextAnalyzer()
	}
	return &HybridSimilarityCalculator{
		config:       config,
		textAnalyzer: textAnalyzer,
	}
}

func (sc *HybridSimilarityCalculator) Calculate(node1, node2 *entities.Node) float64 {
	return sc.CalculateDetailed(node1, node2).Score
}

// CalculateDetailed returns the full similarity result with method and confidence.
func (sc *HybridSimilarityCalculator) CalculateDetailed(node1, node2 *entities.Node) SimilarityResult {
	if node1 == nil || node2 == nil {
		return SimilarityResult{}
	}

	keywordSim := sc.keywordSimilarity(node1, node2)
	hasBothEmbeddings := node1.HasEmbedding() && node2.HasEmbedding()

	if hasBothEmbeddings {
		semanticSim := sc.semanticSimilarity(node1.Embedding(), node2.Embedding())
		blended := (sc.config.SemanticWeight * semanticSim) + (sc.config.KeywordWeight * keywordSim)
		return SimilarityResult{
			Score:      math.Min(blended, 1.0),
			Confidence: 1.0, // Both signals available
			Method:     "hybrid",
		}
	}

	// Fallback: keyword-only
	return SimilarityResult{
		Score:      keywordSim,
		Confidence: 0.5, // Only one signal
		Method:     "keyword",
	}
}

func (sc *HybridSimilarityCalculator) CalculateWithKeywords(node *entities.Node, keywords, tags map[string]bool) float64 {
	if node == nil || (len(keywords) == 0 && len(tags) == 0) {
		return 0.0
	}

	nodeKeywords := sc.extractNodeKeywords(node)
	nodeTags := sc.extractNodeTags(node)

	kwSim := jaccardSimilarity(nodeKeywords, keywords)
	tagSim := jaccardSimilarity(nodeTags, tags)

	return math.Min((kwSim*(1-sc.config.TagWeight))+(tagSim*sc.config.TagWeight), 1.0)
}

func (sc *HybridSimilarityCalculator) CalculateBatch(source *entities.Node, candidates []*entities.Node) map[string]float64 {
	results := make(map[string]float64)
	if source == nil || len(candidates) == 0 {
		return results
	}

	for _, candidate := range candidates {
		if candidate == nil || candidate.ID() == source.ID() {
			continue
		}
		results[candidate.ID().String()] = sc.Calculate(source, candidate)
	}
	return results
}

// CalculateBatchDetailed returns full SimilarityResult for each candidate.
func (sc *HybridSimilarityCalculator) CalculateBatchDetailed(source *entities.Node, candidates []*entities.Node) map[string]SimilarityResult {
	results := make(map[string]SimilarityResult)
	if source == nil || len(candidates) == 0 {
		return results
	}

	for _, candidate := range candidates {
		if candidate == nil || candidate.ID() == source.ID() {
			continue
		}
		results[candidate.ID().String()] = sc.CalculateDetailed(source, candidate)
	}
	return results
}

func (sc *HybridSimilarityCalculator) semanticSimilarity(a, b valueobjects.Embedding) float64 {
	sim := a.CosineSimilarity(b)
	// Clamp to [0, 1] — cosine can technically be negative for opposing vectors,
	// but for text embeddings that's essentially "unrelated" rather than "opposite".
	if sim < 0 {
		return 0.0
	}
	return sim
}

func (sc *HybridSimilarityCalculator) keywordSimilarity(node1, node2 *entities.Node) float64 {
	kw1 := sc.extractNodeKeywords(node1)
	kw2 := sc.extractNodeKeywords(node2)
	tags1 := sc.extractNodeTags(node1)
	tags2 := sc.extractNodeTags(node2)

	kwSim := jaccardSimilarity(kw1, kw2)
	tagSim := jaccardSimilarity(tags1, tags2)

	return (kwSim * (1 - sc.config.TagWeight)) + (tagSim * sc.config.TagWeight)
}

func (sc *HybridSimilarityCalculator) extractNodeKeywords(node *entities.Node) map[string]bool {
	content := node.Content()
	text := content.Title() + " " + content.Body()

	if sc.config.UseStopWords {
		keywords := sc.textAnalyzer.ExtractKeywords(text)
		set := make(map[string]bool)
		for _, kw := range keywords {
			if len(kw) >= sc.config.MinWordLength {
				set[strings.ToLower(kw)] = true
			}
		}
		return set
	}

	return sc.textAnalyzer.TokenizeWords(text)
}

func (sc *HybridSimilarityCalculator) extractNodeTags(node *entities.Node) map[string]bool {
	tags := node.GetTags()
	set := make(map[string]bool)
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			set[normalized] = true
		}
	}
	return set
}

// jaccardSimilarity calculates |A ∩ B| / |A ∪ B|
func jaccardSimilarity(set1, set2 map[string]bool) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 0.0
	}

	intersection := 0
	for key := range set1 {
		if set2[key] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
