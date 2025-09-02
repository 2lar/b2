// Package search provides search service implementations
package search

import (
	"context"
	"strings"
	"time"
	
	"brain2-backend/internal/core/application/ports"
)

// SimpleSearchService provides basic in-memory search capabilities
type SimpleSearchService struct {
	documents map[string]ports.SearchDocument
	logger    ports.Logger
}

// NewSimpleSearchService creates a new search service
func NewSimpleSearchService(logger ports.Logger) *SimpleSearchService {
	return &SimpleSearchService{
		documents: make(map[string]ports.SearchDocument),
		logger:    logger,
	}
}

// Index adds or updates a document in the search index
func (s *SimpleSearchService) Index(ctx context.Context, doc ports.SearchDocument) error {
	s.documents[doc.ID] = doc
	
	s.logger.Debug("Document indexed",
		ports.Field{Key: "doc_id", Value: doc.ID},
		ports.Field{Key: "user_id", Value: doc.UserID})
	
	return nil
}

// Delete removes a document from the search index
func (s *SimpleSearchService) Delete(ctx context.Context, id string) error {
	delete(s.documents, id)
	
	s.logger.Debug("Document deleted from index",
		ports.Field{Key: "doc_id", Value: id})
	
	return nil
}

// Search performs a search query
func (s *SimpleSearchService) Search(ctx context.Context, query ports.SearchQuery) (*ports.SearchResult, error) {
	startTime := time.Now()
	
	var hits []ports.SearchHit
	queryLower := strings.ToLower(query.Query)
	
	// Simple substring search
	for _, doc := range s.documents {
		// Check user filter
		if query.UserID != "" && doc.UserID != query.UserID {
			continue
		}
		
		// Check if query matches content or title
		contentLower := strings.ToLower(doc.Content)
		titleLower := strings.ToLower(doc.Title)
		
		score := 0.0
		highlights := make(map[string][]string)
		
		if strings.Contains(titleLower, queryLower) {
			score += 2.0 // Title matches are weighted higher
			highlights["title"] = s.extractHighlights(doc.Title, query.Query)
		}
		
		if strings.Contains(contentLower, queryLower) {
			score += 1.0
			highlights["content"] = s.extractHighlights(doc.Content, query.Query)
		}
		
		// Check tags
		for _, tag := range doc.Tags {
			if strings.Contains(strings.ToLower(tag), queryLower) {
				score += 0.5
				if _, exists := highlights["tags"]; !exists {
					highlights["tags"] = []string{}
				}
				highlights["tags"] = append(highlights["tags"], tag)
			}
		}
		
		// Check keywords
		for _, keyword := range doc.Keywords {
			if strings.Contains(strings.ToLower(keyword), queryLower) {
				score += 0.3
				if _, exists := highlights["keywords"]; !exists {
					highlights["keywords"] = []string{}
				}
				highlights["keywords"] = append(highlights["keywords"], keyword)
			}
		}
		
		if score > 0 {
			hits = append(hits, ports.SearchHit{
				ID:         doc.ID,
				Score:      score,
				Document:   doc,
				Highlights: highlights,
			})
		}
	}
	
	// Sort by score (simple bubble sort for now)
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Score > hits[i].Score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	
	// Apply pagination
	totalCount := len(hits)
	
	if query.Offset > 0 && query.Offset < len(hits) {
		hits = hits[query.Offset:]
	}
	
	if query.Limit > 0 && query.Limit < len(hits) {
		hits = hits[:query.Limit]
	}
	
	// Convert hits to items for backward compatibility
	items := make([]ports.SearchDocument, len(hits))
	for i, hit := range hits {
		items[i] = hit.Document
	}
	
	result := &ports.SearchResult{
		Items:      items,
		Hits:       hits,
		TotalCount: totalCount,
		Duration:   time.Since(startTime),
	}
	
	s.logger.Debug("Search completed",
		ports.Field{Key: "query", Value: query.Query},
		ports.Field{Key: "hits", Value: len(hits)},
		ports.Field{Key: "duration_ms", Value: result.Duration.Milliseconds()})
	
	return result, nil
}

// BatchIndex indexes multiple documents
func (s *SimpleSearchService) BatchIndex(ctx context.Context, docs []ports.SearchDocument) error {
	for _, doc := range docs {
		if err := s.Index(ctx, doc); err != nil {
			return err
		}
	}
	
	s.logger.Debug("Batch index completed",
		ports.Field{Key: "doc_count", Value: len(docs)})
	
	return nil
}

// extractHighlights extracts snippet highlights around matches
func (s *SimpleSearchService) extractHighlights(text, query string) []string {
	var highlights []string
	
	// Simple implementation: extract 50 chars before and after match
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	
	index := strings.Index(lowerText, lowerQuery)
	if index == -1 {
		return highlights
	}
	
	start := index - 50
	if start < 0 {
		start = 0
	}
	
	end := index + len(query) + 50
	if end > len(text) {
		end = len(text)
	}
	
	highlight := text[start:end]
	if start > 0 {
		highlight = "..." + highlight
	}
	if end < len(text) {
		highlight = highlight + "..."
	}
	
	highlights = append(highlights, highlight)
	return highlights
}