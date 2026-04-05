package services

import (
	"math"
	"strings"
)

// BM25 parameters (standard Okapi BM25)
const (
	bm25K1 = 1.2  // Term frequency saturation
	bm25B  = 0.75 // Document length normalization
)

// BM25Scorer computes BM25 relevance scores for a set of documents against a query.
// It builds an ephemeral corpus (IDF, average doc length) from the provided documents
// and scores each one. Designed for in-memory use at personal-tool scale (<10K docs).
type BM25Scorer struct {
	textAnalyzer TextAnalyzer
}

// NewBM25Scorer creates a new scorer that uses the given text analyzer for tokenization.
func NewBM25Scorer(textAnalyzer TextAnalyzer) *BM25Scorer {
	if textAnalyzer == nil {
		textAnalyzer = NewDefaultTextAnalyzer()
	}
	return &BM25Scorer{textAnalyzer: textAnalyzer}
}

// ScoredDocument holds a document ID and its BM25 score.
type ScoredDocument struct {
	ID    string
	Score float64
}

// Score computes BM25 scores for each document against the query terms.
// Returns documents sorted by score descending, excluding zero-score documents.
func (s *BM25Scorer) Score(queryTerms []string, documents []DocumentRecord) []ScoredDocument {
	if len(queryTerms) == 0 || len(documents) == 0 {
		return nil
	}

	// Normalize query terms
	normalizedQuery := make([]string, 0, len(queryTerms))
	for _, t := range queryTerms {
		lower := strings.ToLower(t)
		if lower != "" {
			normalizedQuery = append(normalizedQuery, lower)
		}
	}
	if len(normalizedQuery) == 0 {
		return nil
	}

	// Build corpus stats: document frequency per term and average doc length
	n := float64(len(documents))
	totalLength := 0.0
	termDFs := make(map[string]int) // number of docs containing each term
	docTermFreqs := make([]map[string]int, len(documents))
	docLengths := make([]float64, len(documents))

	for i, doc := range documents {
		tokens := s.tokenize(doc.Text)
		docLengths[i] = float64(len(tokens))
		totalLength += docLengths[i]

		tf := make(map[string]int)
		seen := make(map[string]bool)
		for _, token := range tokens {
			tf[token]++
			if !seen[token] {
				termDFs[token]++
				seen[token] = true
			}
		}
		docTermFreqs[i] = tf
	}

	avgDL := totalLength / n

	// Score each document
	results := make([]ScoredDocument, 0, len(documents))
	for i, doc := range documents {
		score := 0.0
		tf := docTermFreqs[i]
		dl := docLengths[i]

		for _, term := range normalizedQuery {
			freq := float64(tf[term])
			if freq == 0 {
				continue
			}

			df := float64(termDFs[term])
			// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
			idf := math.Log((n-df+0.5)/(df+0.5) + 1)
			// TF component with length normalization
			tfNorm := (freq * (bm25K1 + 1)) / (freq + bm25K1*(1-bm25B+bm25B*(dl/avgDL)))
			score += idf * tfNorm
		}

		if score > 0 {
			results = append(results, ScoredDocument{ID: doc.ID, Score: score})
		}
	}

	// Sort by score descending
	SortScoredDocuments(results)
	return results
}

// DocumentRecord is a minimal struct for BM25 input.
type DocumentRecord struct {
	ID   string
	Text string
}

// tokenize splits text into lowercase tokens, preserving duplicates for TF counting.
func (s *BM25Scorer) tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			word := current.String()
			if len(word) > 1 {
				tokens = append(tokens, word)
			}
			current.Reset()
		}
	}
	if current.Len() > 0 {
		word := current.String()
		if len(word) > 1 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// SortScoredDocuments sorts in-place by score descending.
func SortScoredDocuments(docs []ScoredDocument) {
	// Simple insertion sort — fine for <10K items
	for i := 1; i < len(docs); i++ {
		key := docs[i]
		j := i - 1
		for j >= 0 && docs[j].Score < key.Score {
			docs[j+1] = docs[j]
			j--
		}
		docs[j+1] = key
	}
}
