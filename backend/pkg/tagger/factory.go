package tagger

import (
	"fmt"
)

// NewTagger creates a new tagger instance based on the provided configuration.
func NewTagger(config Config) (Tagger, error) {
	switch config.Type {
	case KeywordTagger:
		return NewKeywordTagger(config), nil
	case OpenAITagger:
		if config.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for OpenAI tagger")
		}
		return NewOpenAITagger(config)
	case LocalLLMTagger:
		if config.LocalLLMURL == "" {
			return nil, fmt.Errorf("Local LLM URL is required for local LLM tagger")
		}
		return NewLocalLLMTagger(config)
	default:
		return nil, fmt.Errorf("unknown tagger type: %s", config.Type)
	}
}

// NewWithFallback creates a tagger with automatic fallback to keyword tagger on errors.
func NewWithFallback(config Config) Tagger {
	primaryTagger, err := NewTagger(config)
	if err != nil {
		// If primary tagger fails to initialize, use keyword tagger
		return NewKeywordTagger(config)
	}
	
	if config.EnableFallback && config.Type != KeywordTagger {
		return NewFallbackTagger(primaryTagger, NewKeywordTagger(config))
	}
	
	return primaryTagger
}