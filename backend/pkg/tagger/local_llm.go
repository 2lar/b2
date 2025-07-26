package tagger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// localLLMTagger implements the Tagger interface using a local LLM service.
type localLLMTagger struct {
	baseURL string
	maxTags int
	client  *http.Client
}

// NewLocalLLMTagger creates a new local LLM-based tagger.
func NewLocalLLMTagger(config Config) (Tagger, error) {
	if config.LocalLLMURL == "" {
		return nil, fmt.Errorf("Local LLM URL is required")
	}

	return &localLLMTagger{
		baseURL: config.LocalLLMURL,
		maxTags: config.MaxTags,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type localLLMRequest struct {
	Content string `json:"content"`
}

type localLLMResponse struct {
	Tags []string `json:"tags"`
}

// GenerateTags generates tags using a local LLM service.
func (l *localLLMTagger) GenerateTags(ctx context.Context, content string) ([]string, error) {
	reqBody := localLLMRequest{
		Content: content,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimSuffix(l.baseURL, "/") + "/generate-tags"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call local LLM service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("local LLM service returned status %d", resp.StatusCode)
	}

	var llmResp localLLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Clean and validate tags
	var tags []string
	for _, tag := range llmResp.Tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" && isValidTag(tag) {
			tags = append(tags, tag)
		}
	}

	// Respect max tags limit
	if l.maxTags > 0 && len(tags) > l.maxTags {
		tags = tags[:l.maxTags]
	}

	return tags, nil
}

// HealthCheck verifies that the local LLM tagger is functioning properly.
func (l *localLLMTagger) HealthCheck(ctx context.Context) error {
	// Test basic connectivity to the local LLM service
	url := strings.TrimSuffix(l.baseURL, "/") + "/health"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("local LLM service is unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("local LLM service health check failed with status %d", resp.StatusCode)
	}

	return nil
}