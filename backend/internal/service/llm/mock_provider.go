package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"brain2-backend/internal/domain"
)

// MockProvider provides a simple mock implementation for testing and development
type MockProvider struct {
	available bool
}

// NewMockProvider creates a new mock LLM provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		available: true,
	}
}

// IsAvailable returns whether the mock provider is available
func (m *MockProvider) IsAvailable() bool {
	return m.available
}

// Complete provides mock completions based on simple pattern matching
func (m *MockProvider) Complete(ctx context.Context, prompt string, options CompletionOptions) (string, error) {
	if !m.available {
		return "", fmt.Errorf("mock provider is not available")
	}

	// Detect what type of request this is based on the prompt
	if strings.Contains(prompt, "suggest 1-3 hierarchical categories") {
		return m.mockCategorization(prompt)
	}
	
	if strings.Contains(prompt, "parent-child relationships") {
		return m.mockHierarchy(prompt)
	}
	
	if strings.Contains(prompt, "similar and might be duplicates") {
		return m.mockSimilarity(prompt)
	}

	return "", fmt.Errorf("unsupported prompt type")
}

// mockCategorization provides mock category suggestions
func (m *MockProvider) mockCategorization(prompt string) (string, error) {
	// Extract content from prompt
	lines := strings.Split(prompt, "\n")
	var content string
	for i, line := range lines {
		if strings.Contains(line, "Text to categorize:") && i+1 < len(lines) {
			content = strings.TrimSpace(lines[i+1])
			break
		}
	}

	suggestions := m.generateMockSuggestions(content)
	
	jsonData, err := json.Marshal(suggestions)
	if err != nil {
		return "", err
	}
	
	return string(jsonData), nil
}

// mockHierarchy provides mock hierarchy suggestions
func (m *MockProvider) mockHierarchy(prompt string) (string, error) {
	// Simple mock: return empty hierarchy for now
	return "{}", nil
}

// mockSimilarity provides mock similarity detection
func (m *MockProvider) mockSimilarity(prompt string) (string, error) {
	// Simple mock: return empty array for now
	return "[]", nil
}

// generateMockSuggestions creates category suggestions based on content keywords
func (m *MockProvider) generateMockSuggestions(content string) []domain.CategorySuggestion {
	content = strings.ToLower(content)
	var suggestions []domain.CategorySuggestion

	// Technology-related content
	if strings.Contains(content, "ai") || strings.Contains(content, "machine learning") || 
	   strings.Contains(content, "algorithm") || strings.Contains(content, "neural") {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Technology",
			Level:      0,
			Confidence: 0.9,
			Reason:     "Content discusses artificial intelligence and technology concepts",
		})
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Machine Learning",
			Level:      1,
			Confidence: 0.85,
			Reason:     "Specific focus on ML/AI algorithms and techniques",
		})
	}

	// Programming/Software
	if strings.Contains(content, "code") || strings.Contains(content, "programming") || 
	   strings.Contains(content, "software") || strings.Contains(content, "api") ||
	   strings.Contains(content, "javascript") || strings.Contains(content, "python") ||
	   strings.Contains(content, "go") || strings.Contains(content, "react") {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Programming",
			Level:      0,
			Confidence: 0.88,
			Reason:     "Content relates to software development and programming",
		})
	}

	// Business/Work
	if strings.Contains(content, "business") || strings.Contains(content, "work") || 
	   strings.Contains(content, "project") || strings.Contains(content, "team") ||
	   strings.Contains(content, "meeting") || strings.Contains(content, "strategy") {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Work",
			Level:      0,
			Confidence: 0.82,
			Reason:     "Content appears to be work or business related",
		})
	}

	// Learning/Education
	if strings.Contains(content, "learn") || strings.Contains(content, "study") || 
	   strings.Contains(content, "course") || strings.Contains(content, "tutorial") ||
	   strings.Contains(content, "book") || strings.Contains(content, "research") {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Learning",
			Level:      0,
			Confidence: 0.8,
			Reason:     "Content relates to learning and education",
		})
	}

	// Personal/Life
	if strings.Contains(content, "personal") || strings.Contains(content, "life") || 
	   strings.Contains(content, "health") || strings.Contains(content, "family") ||
	   strings.Contains(content, "hobby") || strings.Contains(content, "travel") {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "Personal",
			Level:      0,
			Confidence: 0.75,
			Reason:     "Content appears to be personal or lifestyle related",
		})
	}

	// If no specific patterns match, suggest a general category
	if len(suggestions) == 0 {
		suggestions = append(suggestions, domain.CategorySuggestion{
			Name:       "General",
			Level:      0,
			Confidence: 0.7,
			Reason:     "Content doesn't match specific patterns, using general category",
		})
	}

	return suggestions
}

// SetAvailable controls whether the mock provider is available (for testing)
func (m *MockProvider) SetAvailable(available bool) {
	m.available = available
}