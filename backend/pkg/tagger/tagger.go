// Package tagger provides interfaces and implementations for generating tags from content.
package tagger

import "context"

// Tagger defines the interface for generating tags from text content.
type Tagger interface {
	// GenerateTags generates relevant tags for the given content.
	// Returns a slice of lowercase, single-word tags.
	GenerateTags(ctx context.Context, content string) ([]string, error)
	
	// HealthCheck verifies that the tagger is functioning properly.
	// Returns nil if healthy, error if there are issues.
	HealthCheck(ctx context.Context) error
}

// TaggerType represents the different types of taggers available.
type TaggerType string

const (
	// KeywordTagger uses keyword extraction and semantic grouping
	KeywordTagger TaggerType = "keyword"
	// OpenAITagger uses OpenAI API for intelligent tagging
	OpenAITagger TaggerType = "openai"
	// LocalLLMTagger uses a local LLM service for tagging
	LocalLLMTagger TaggerType = "local_llm"
)

// Config holds configuration for tagger implementations.
type Config struct {
	// Type specifies which tagger implementation to use
	Type TaggerType
	
	// OpenAI configuration (used when Type is OpenAITagger)
	OpenAIAPIKey string
	OpenAIModel  string
	
	// Local LLM configuration (used when Type is LocalLLMTagger)
	LocalLLMURL string
	
	// Fallback configuration
	EnableFallback bool // If true, falls back to keyword tagger on errors
	MaxTags        int  // Maximum number of tags to return (0 = unlimited)
}

// DefaultConfig returns a default configuration using keyword tagger.
func DefaultConfig() Config {
	return Config{
		Type:           KeywordTagger,
		EnableFallback: true,
		MaxTags:        5,
		OpenAIModel:    "gpt-3.5-turbo",
	}
}