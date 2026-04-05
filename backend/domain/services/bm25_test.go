package services

import (
	"fmt"
	"testing"
)

func TestBM25Scorer_EmptyInputs(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	if results := scorer.Score(nil, nil); results != nil {
		t.Errorf("expected nil for nil inputs, got %v", results)
	}
	if results := scorer.Score([]string{"test"}, nil); results != nil {
		t.Errorf("expected nil for nil documents, got %v", results)
	}
	if results := scorer.Score(nil, []DocumentRecord{{ID: "1", Text: "test"}}); results != nil {
		t.Errorf("expected nil for nil query, got %v", results)
	}
}

func TestBM25Scorer_SingleDocument(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	docs := []DocumentRecord{
		{ID: "1", Text: "machine learning neural networks deep learning"},
	}

	results := scorer.Score([]string{"machine", "learning"}, docs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "1" {
		t.Errorf("expected ID '1', got %s", results[0].ID)
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", results[0].Score)
	}
}

func TestBM25Scorer_Ranking(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	docs := []DocumentRecord{
		{ID: "irrelevant", Text: "the weather today is sunny and warm"},
		{ID: "partial", Text: "introduction to machine learning concepts"},
		{ID: "best", Text: "machine learning machine learning deep neural networks machine learning algorithms"},
	}

	results := scorer.Score([]string{"machine", "learning"}, docs)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// "best" should rank first (highest TF for query terms)
	if results[0].ID != "best" {
		t.Errorf("expected 'best' to rank first, got %s", results[0].ID)
	}

	// Scores should be in descending order
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: score[%d]=%f > score[%d]=%f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestBM25Scorer_NoMatch(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	docs := []DocumentRecord{
		{ID: "1", Text: "the quick brown fox"},
		{ID: "2", Text: "jumped over the lazy dog"},
	}

	results := scorer.Score([]string{"quantum", "physics"}, docs)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestBM25Scorer_IDFEffect(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	// "common" appears in all docs, "rare" only in one
	docs := []DocumentRecord{
		{ID: "1", Text: "common word common word"},
		{ID: "2", Text: "common word another thing"},
		{ID: "3", Text: "common rare special unique"},
	}

	results := scorer.Score([]string{"rare"}, docs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'rare', got %d", len(results))
	}
	if results[0].ID != "3" {
		t.Errorf("expected doc '3' with rare term, got %s", results[0].ID)
	}

	// "rare" should have higher IDF than "common"
	rareScore := results[0].Score

	commonResults := scorer.Score([]string{"common"}, docs)
	commonBestScore := 0.0
	for _, r := range commonResults {
		if r.Score > commonBestScore {
			commonBestScore = r.Score
		}
	}

	if rareScore <= commonBestScore {
		t.Errorf("rare term score (%f) should be higher than common term score (%f)", rareScore, commonBestScore)
	}
}

func TestBM25Scorer_CaseInsensitive(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	docs := []DocumentRecord{
		{ID: "1", Text: "Machine Learning Neural Networks"},
	}

	results := scorer.Score([]string{"MACHINE", "learning"}, docs)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (case insensitive), got %d", len(results))
	}
}

func TestSortScoredDocuments(t *testing.T) {
	docs := []ScoredDocument{
		{ID: "low", Score: 0.1},
		{ID: "high", Score: 0.9},
		{ID: "mid", Score: 0.5},
	}
	SortScoredDocuments(docs)

	if docs[0].ID != "high" || docs[1].ID != "mid" || docs[2].ID != "low" {
		t.Errorf("unexpected sort order: %v", docs)
	}
}

func TestBM25Scorer_LargeCorpus(t *testing.T) {
	scorer := NewBM25Scorer(NewDefaultTextAnalyzer())

	docs := make([]DocumentRecord, 1000)
	for i := 0; i < 1000; i++ {
		docs[i] = DocumentRecord{
			ID:   fmt.Sprintf("doc-%d", i),
			Text: fmt.Sprintf("document number %d about various topics including technology science art", i),
		}
	}
	// Add a highly relevant document
	docs[500] = DocumentRecord{
		ID:   "target",
		Text: "quantum physics quantum mechanics quantum entanglement quantum computing",
	}

	results := scorer.Score([]string{"quantum", "physics"}, docs)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].ID != "target" {
		t.Errorf("expected 'target' to rank first, got %s", results[0].ID)
	}
}
