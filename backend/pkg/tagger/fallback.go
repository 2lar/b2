package tagger

import (
	"context"
	"fmt"
	"log"
)

// fallbackTagger wraps a primary tagger with a fallback tagger for resilience.
type fallbackTagger struct {
	primary   Tagger
	fallback  Tagger
}

// NewFallbackTagger creates a new fallback tagger that tries the primary tagger first,
// then falls back to the fallback tagger if the primary fails.
func NewFallbackTagger(primary, fallback Tagger) Tagger {
	return &fallbackTagger{
		primary:  primary,
		fallback: fallback,
	}
}

// GenerateTags attempts to generate tags using the primary tagger first.
// If the primary tagger fails, it falls back to the fallback tagger.
func (f *fallbackTagger) GenerateTags(ctx context.Context, content string) ([]string, error) {
	// Try primary tagger first
	tags, err := f.primary.GenerateTags(ctx, content)
	if err == nil {
		return tags, nil
	}
	
	// Log the primary tagger failure and try fallback
	log.Printf("Primary tagger failed, falling back: %v", err)
	
	fallbackTags, fallbackErr := f.fallback.GenerateTags(ctx, content)
	if fallbackErr != nil {
		// Return the original primary error if fallback also fails
		log.Printf("Fallback tagger also failed: %v", fallbackErr)
		return nil, err
	}
	
	return fallbackTags, nil
}

// HealthCheck performs health checks on both primary and fallback taggers.
func (f *fallbackTagger) HealthCheck(ctx context.Context) error {
	// Check primary tagger health
	primaryErr := f.primary.HealthCheck(ctx)
	
	// Check fallback tagger health
	fallbackErr := f.fallback.HealthCheck(ctx)
	
	// If both fail, return the primary error
	if primaryErr != nil && fallbackErr != nil {
		return fmt.Errorf("both primary and fallback taggers failed health checks - primary: %v, fallback: %v", primaryErr, fallbackErr)
	}
	
	// If only primary fails but fallback is healthy, log warning but return success
	if primaryErr != nil && fallbackErr == nil {
		log.Printf("Warning: Primary tagger failed health check, but fallback is healthy: %v", primaryErr)
		return nil
	}
	
	// If primary is healthy (regardless of fallback), return success
	return nil
}