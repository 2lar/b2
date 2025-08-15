// Package llm provides AI-powered text processing capabilities for categorization and analysis.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"brain2-backend/internal/domain"
)

// Provider defines the interface for LLM providers (OpenAI, Anthropic, etc.)
type Provider interface {
	Complete(ctx context.Context, prompt string, options CompletionOptions) (string, error)
	IsAvailable() bool
}

// CompletionOptions configures LLM completion requests
type CompletionOptions struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Format      string  `json:"format"` // "json" or "text"
}

// Service provides LLM-powered categorization and analysis
type Service struct {
	provider Provider
}

// NewService creates a new LLM service with the specified provider
func NewService(provider Provider) *Service {
	return &Service{
		provider: provider,
	}
}

// IsAvailable returns true if the LLM service is available
func (s *Service) IsAvailable() bool {
	return s.provider != nil && s.provider.IsAvailable()
}

// SuggestCategories analyzes content and suggests appropriate categories
func (s *Service) SuggestCategories(
	ctx context.Context,
	content string,
	existingCategories []domain.Category,
) ([]domain.CategorySuggestion, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("LLM service is not available")
	}

	prompt := s.buildCategorizationPrompt(content, existingCategories)

	response, err := s.provider.Complete(ctx, prompt, CompletionOptions{
		Temperature: 0.5, // Lower for more consistent categorization
		MaxTokens:   300,
		Format:      "json",
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get LLM response: %w", err)
	}

	suggestions, err := s.parseCategorizationResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return suggestions, nil
}

// AnalyzeCategoryHierarchy suggests parent-child relationships for categories
func (s *Service) AnalyzeCategoryHierarchy(
	ctx context.Context,
	categories []domain.Category,
) (map[string]string, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("LLM service is not available")
	}

	prompt := s.buildHierarchyPrompt(categories)

	response, err := s.provider.Complete(ctx, prompt, CompletionOptions{
		Temperature: 0.3,
		MaxTokens:   500,
		Format:      "json",
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get hierarchy analysis: %w", err)
	}

	return s.parseHierarchyResponse(response)
}

// DetectSimilarCategories finds categories that might be duplicates or very similar
func (s *Service) DetectSimilarCategories(
	ctx context.Context,
	categories []domain.Category,
	threshold float64,
) ([]domain.CategoryConnection, error) {
	if !s.IsAvailable() {
		return nil, fmt.Errorf("LLM service is not available")
	}

	prompt := s.buildSimilarityPrompt(categories, threshold)

	response, err := s.provider.Complete(ctx, prompt, CompletionOptions{
		Temperature: 0.2,
		MaxTokens:   400,
		Format:      "json",
	})

	if err != nil {
		return nil, fmt.Errorf("failed to detect similar categories: %w", err)
	}

	return s.parseSimilarityResponse(response)
}

// buildCategorizationPrompt creates a prompt for content categorization
func (s *Service) buildCategorizationPrompt(content string, existing []domain.Category) string {
	existingStr := s.formatExistingCategories(existing)

	return fmt.Sprintf(`You are an expert content categorizer. Analyze the following text and suggest 1-3 hierarchical categories.

Existing categories in the system:
%s

Text to categorize:
%s

Return a JSON array with this structure:
[
  {"name": "General Category", "level": 0, "confidence": 0.9, "reason": "why this category", "parent_id": null},
  {"name": "Specific Category", "level": 1, "confidence": 0.8, "reason": "why this category", "parent_id": "parent_category_id"},
  {"name": "More Specific", "level": 2, "confidence": 0.7, "reason": "why this category", "parent_id": "specific_category_id"}
]

Rules:
1. Prefer existing categories when appropriate
2. Suggest new categories only when content doesn't fit existing ones
3. Keep category names concise (2-3 words)
4. Ensure hierarchy makes sense (general → specific)
5. Confidence should be 0.0-1.0
6. Only suggest categories with confidence > 0.6
7. For existing categories, use their exact name and ID
`, existingStr, content)
}

// buildHierarchyPrompt creates a prompt for analyzing category relationships
func (s *Service) buildHierarchyPrompt(categories []domain.Category) string {
	categoryList := make([]string, len(categories))
	for i, cat := range categories {
		categoryList[i] = fmt.Sprintf(`{"id": "%s", "name": "%s", "description": "%s"}`,
			cat.ID, cat.Name, cat.Description)
	}

	return fmt.Sprintf(`Analyze these categories and suggest parent-child relationships:

Categories:
%s

Return JSON object mapping child category IDs to parent category IDs:
{
  "child_category_id": "parent_category_id",
  "another_child_id": "another_parent_id"
}

Rules:
1. Only suggest relationships that make semantic sense
2. Avoid circular dependencies
3. Keep hierarchy depth reasonable (max 3 levels)
4. Parent categories should be more general than children
`, strings.Join(categoryList, ",\n"))
}

// buildSimilarityPrompt creates a prompt for detecting similar categories
func (s *Service) buildSimilarityPrompt(categories []domain.Category, threshold float64) string {
	categoryList := make([]string, len(categories))
	for i, cat := range categories {
		categoryList[i] = fmt.Sprintf(`{"id": "%s", "name": "%s", "description": "%s"}`,
			cat.ID, cat.Name, cat.Description)
	}

	return fmt.Sprintf(`Find categories that are very similar and might be duplicates or should be merged:

Categories:
%s

Similarity threshold: %.2f (only return pairs with similarity >= this value)

Return JSON array of similar category pairs:
[
  {
    "category1_id": "id1",
    "category1_name": "name1", 
    "category2_id": "id2",
    "category2_name": "name2",
    "strength": 0.85,
    "reason": "explanation of why they're similar"
  }
]

Rules:
1. Strength should be 0.0-1.0 (semantic similarity)
2. Only include pairs with strength >= %.2f
3. Explain why categories are similar
4. Consider synonyms, overlapping concepts, etc.
`, strings.Join(categoryList, ",\n"), threshold, threshold)
}

// formatExistingCategories formats existing categories for the prompt
func (s *Service) formatExistingCategories(categories []domain.Category) string {
	if len(categories) == 0 {
		return "No existing categories."
	}

	var formatted []string
	for _, cat := range categories {
		desc := cat.Description
		if desc == "" {
			desc = "No description"
		}
		formatted = append(formatted, fmt.Sprintf("- %s (ID: %s): %s", cat.Name, cat.ID, desc))
	}

	return strings.Join(formatted, "\n")
}

// parseCategorizationResponse parses the LLM response into category suggestions
func (s *Service) parseCategorizationResponse(response string) ([]domain.CategorySuggestion, error) {
	var suggestions []domain.CategorySuggestion

	// Clean up the response (remove any markdown formatting)
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	err := json.Unmarshal([]byte(response), &suggestions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate and filter suggestions
	var validSuggestions []domain.CategorySuggestion
	for _, suggestion := range suggestions {
		if suggestion.Confidence >= 0.6 && suggestion.Name != "" {
			validSuggestions = append(validSuggestions, suggestion)
		}
	}

	return validSuggestions, nil
}

// parseHierarchyResponse parses hierarchy suggestions from LLM
func (s *Service) parseHierarchyResponse(response string) (map[string]string, error) {
	var hierarchy map[string]string

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	err := json.Unmarshal([]byte(response), &hierarchy)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hierarchy JSON: %w", err)
	}

	return hierarchy, nil
}

// parseSimilarityResponse parses similarity analysis from LLM
func (s *Service) parseSimilarityResponse(response string) ([]domain.CategoryConnection, error) {
	var connections []domain.CategoryConnection

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	err := json.Unmarshal([]byte(response), &connections)
	if err != nil {
		return nil, fmt.Errorf("failed to parse similarity JSON: %w", err)
	}

	return connections, nil
}
