package services

import (
	"context"
	"math"

	"backend/application/ports"
	"backend/domain/core/entities"
	domainservices "backend/domain/services"
)

const (
	// rrfK is the Reciprocal Rank Fusion constant. K=60 is standard in the literature.
	rrfK = 60
)

// SearchResult represents a single search result with scoring metadata.
type SearchResult struct {
	Node          *entities.Node
	Score         float64  // Combined RRF score
	BM25Score     float64  // Raw BM25 score (0 if not found by BM25)
	SemanticScore float64  // Raw cosine similarity (0 if not found by semantic)
	Sources       []string // Which methods found this node: "bm25", "semantic"
}

// SearchConfig configures the hybrid search service.
type SearchConfig struct {
	MaxResults int // Maximum results to return (default 20)
}

// DefaultSearchConfig returns reasonable defaults.
func DefaultSearchConfig() *SearchConfig {
	return &SearchConfig{
		MaxResults: 20,
	}
}

// HybridSearchService orchestrates BM25 keyword search and semantic vector search,
// combining results with Reciprocal Rank Fusion.
type HybridSearchService struct {
	bm25             *domainservices.BM25Scorer
	embeddingService domainservices.EmbeddingService
	textAnalyzer     domainservices.TextAnalyzer
	nodeRepo         ports.NodeRepository
	config           *SearchConfig
}

// NewHybridSearchService creates a new hybrid search service.
// embeddingService may be nil if embeddings are not available — search falls back to BM25-only.
func NewHybridSearchService(
	bm25 *domainservices.BM25Scorer,
	embeddingService domainservices.EmbeddingService,
	textAnalyzer domainservices.TextAnalyzer,
	nodeRepo ports.NodeRepository,
	config *SearchConfig,
) *HybridSearchService {
	if config == nil {
		config = DefaultSearchConfig()
	}
	if textAnalyzer == nil {
		textAnalyzer = domainservices.NewDefaultTextAnalyzer()
	}
	if bm25 == nil {
		bm25 = domainservices.NewBM25Scorer(textAnalyzer)
	}
	return &HybridSearchService{
		bm25:             bm25,
		embeddingService: embeddingService,
		textAnalyzer:     textAnalyzer,
		nodeRepo:         nodeRepo,
		config:           config,
	}
}

// Search performs a hybrid search across the user's nodes.
func (s *HybridSearchService) Search(ctx context.Context, userID string, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = s.config.MaxResults
	}

	// Load all user nodes — at personal scale (<10K) this is fine
	nodes, err := s.nodeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	// Build node index for quick lookup
	nodeIndex := make(map[string]*entities.Node, len(nodes))
	for _, n := range nodes {
		nodeIndex[n.ID().String()] = n
	}

	// --- BM25 ---
	queryTerms := s.textAnalyzer.ExtractKeywords(query)
	docs := make([]domainservices.DocumentRecord, len(nodes))
	for i, n := range nodes {
		content := n.Content()
		docs[i] = domainservices.DocumentRecord{
			ID:   n.ID().String(),
			Text: content.Title() + " " + content.Body(),
		}
	}
	bm25Results := s.bm25.Score(queryTerms, docs)

	// --- Semantic ---
	var semanticResults []domainservices.ScoredDocument
	if s.embeddingService != nil {
		semanticResults = s.semanticSearch(ctx, query, nodes)
	}

	// --- RRF Fusion ---
	merged := s.mergeWithRRF(bm25Results, semanticResults, nodeIndex)

	// Limit results
	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged, nil
}

// semanticSearch embeds the query and computes cosine similarity against all nodes with embeddings.
func (s *HybridSearchService) semanticSearch(ctx context.Context, query string, nodes []*entities.Node) []domainservices.ScoredDocument {
	queryEmbedding, err := s.embeddingService.GenerateEmbedding(ctx, query)
	if err != nil {
		// Degrade gracefully — BM25 results still work
		return nil
	}

	results := make([]domainservices.ScoredDocument, 0)
	for _, n := range nodes {
		if !n.HasEmbedding() {
			continue
		}
		sim := queryEmbedding.CosineSimilarity(n.Embedding())
		if sim > 0 {
			results = append(results, domainservices.ScoredDocument{
				ID:    n.ID().String(),
				Score: sim,
			})
		}
	}

	domainservices.SortScoredDocuments(results)
	return results
}

// mergeWithRRF combines BM25 and semantic rankings using Reciprocal Rank Fusion.
func (s *HybridSearchService) mergeWithRRF(
	bm25Results []domainservices.ScoredDocument,
	semanticResults []domainservices.ScoredDocument,
	nodeIndex map[string]*entities.Node,
) []SearchResult {
	type mergedEntry struct {
		rrfScore      float64
		bm25Score     float64
		semanticScore float64
		sources       []string
	}

	merged := make(map[string]*mergedEntry)

	for i, doc := range bm25Results {
		rrfScore := 1.0 / float64(rrfK+i+1)
		merged[doc.ID] = &mergedEntry{
			rrfScore:  rrfScore,
			bm25Score: doc.Score,
			sources:   []string{"bm25"},
		}
	}

	for i, doc := range semanticResults {
		rrfScore := 1.0 / float64(rrfK+i+1)
		if existing, ok := merged[doc.ID]; ok {
			existing.rrfScore += rrfScore
			existing.semanticScore = doc.Score
			existing.sources = append(existing.sources, "semantic")
		} else {
			merged[doc.ID] = &mergedEntry{
				rrfScore:      rrfScore,
				semanticScore: doc.Score,
				sources:       []string{"semantic"},
			}
		}
	}

	results := make([]SearchResult, 0, len(merged))
	for id, entry := range merged {
		node, ok := nodeIndex[id]
		if !ok {
			continue
		}
		results = append(results, SearchResult{
			Node:          node,
			Score:         math.Round(entry.rrfScore*10000) / 10000,
			BM25Score:     entry.bm25Score,
			SemanticScore: entry.semanticScore,
			Sources:       entry.sources,
		})
	}

	// Sort by RRF score descending
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && results[j].Score < key.Score {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}

	return results
}
