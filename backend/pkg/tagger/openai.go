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

// openaiTagger implements the Tagger interface using OpenAI's API.
type openaiTagger struct {
	apiKey  string
	model   string
	maxTags int
	client  *http.Client
}

// NewOpenAITagger creates a new OpenAI-based tagger.
func NewOpenAITagger(config Config) (Tagger, error) {
	if config.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	return &openaiTagger{
		apiKey:  config.OpenAIAPIKey,
		model:   config.OpenAIModel,
		maxTags: config.MaxTags,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

type openaiRequest struct {
	Model       string                 `json:"model"`
	Messages    []openaiMessage        `json:"messages"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// GenerateTags generates tags using OpenAI's API.
func (o *openaiTagger) GenerateTags(ctx context.Context, content string) ([]string, error) {
	maxTagsText := "up to 5"
	if o.maxTags > 0 {
		maxTagsText = fmt.Sprintf("up to %d", o.maxTags)
	}

	prompt := fmt.Sprintf(`Analyze the following text and generate %s relevant, single-word, lowercase tags that categorize the content. Return only the tags as a comma-separated list with no additional text.

Text: "%s"

Tags:`, maxTagsText, content)

	reqBody := openaiRequest{
		Model: o.model,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   100,
		Temperature: 0.3,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if openaiResp.Error.Message != "" {
		return nil, fmt.Errorf("OpenAI API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI API")
	}

	// Parse the response content
	content = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	if content == "" {
		return []string{}, nil
	}

	// Split by comma and clean up tags
	rawTags := strings.Split(content, ",")
	var tags []string
	for _, tag := range rawTags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		// Remove any quotes or extra characters
		tag = strings.Trim(tag, "\"'")
		if tag != "" && isValidTag(tag) {
			tags = append(tags, tag)
		}
	}

	// Respect max tags limit
	if o.maxTags > 0 && len(tags) > o.maxTags {
		tags = tags[:o.maxTags]
	}

	return tags, nil
}

// HealthCheck verifies that the OpenAI tagger is functioning properly.
func (o *openaiTagger) HealthCheck(ctx context.Context) error {
	// Simple health check by testing a minimal API call
	_, err := o.GenerateTags(ctx, "test")
	if err != nil {
		return fmt.Errorf("OpenAI tagger health check failed: %w", err)
	}
	return nil
}

// isValidTag checks if a tag is valid (single word, only letters/numbers).
func isValidTag(tag string) bool {
	if len(tag) < 2 || len(tag) > 20 {
		return false
	}
	
	for _, r := range tag {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	
	return true
}