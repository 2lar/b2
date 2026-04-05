package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"backend/domain/core/valueobjects"
	"backend/domain/services"

	"go.uber.org/zap"
)

var _ services.EmbeddingService = (*OpenAICompatibleService)(nil)

// OpenAICompatibleConfig configures the embedding service.
type OpenAICompatibleConfig struct {
	BaseURL    string // e.g. "https://api.openai.com/v1" or "http://localhost:11434/v1"
	APIKey     string // empty for local endpoints like Ollama
	Model      string // e.g. "text-embedding-3-small"
	Dimensions int    // expected output dimensions
	BatchSize  int    // max texts per request (OpenAI supports up to 2048)
	Timeout    time.Duration
}

func DefaultOpenAICompatibleConfig() *OpenAICompatibleConfig {
	return &OpenAICompatibleConfig{
		BaseURL:    "https://api.openai.com/v1",
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		BatchSize:  64,
		Timeout:    30 * time.Second,
	}
}

// OpenAICompatibleService calls any OpenAI-compatible /v1/embeddings endpoint.
// Works with OpenAI, Ollama, vLLM, LiteLLM, Azure OpenAI, etc.
type OpenAICompatibleService struct {
	client *http.Client
	config *OpenAICompatibleConfig
	logger *zap.Logger
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

func NewOpenAICompatibleService(config *OpenAICompatibleConfig, logger *zap.Logger) *OpenAICompatibleService {
	if config == nil {
		config = DefaultOpenAICompatibleConfig()
	}
	return &OpenAICompatibleService{
		client: &http.Client{Timeout: config.Timeout},
		config: config,
		logger: logger,
	}
}

func (s *OpenAICompatibleService) GenerateEmbedding(ctx context.Context, text string) (valueobjects.Embedding, error) {
	embeddings, err := s.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return valueobjects.Embedding{}, err
	}
	return embeddings[0], nil
}

func (s *OpenAICompatibleService) GenerateEmbeddings(ctx context.Context, texts []string) ([]valueobjects.Embedding, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Process in batches
	results := make([]valueobjects.Embedding, 0, len(texts))
	batchSize := s.config.BatchSize
	if batchSize <= 0 {
		batchSize = 64
	}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := s.callAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}
		results = append(results, embeddings...)
	}

	return results, nil
}

func (s *OpenAICompatibleService) Dimensions() int {
	return s.config.Dimensions
}

func (s *OpenAICompatibleService) callAPI(ctx context.Context, texts []string) ([]valueobjects.Embedding, error) {
	reqBody := embeddingRequest{
		Input: texts,
		Model: s.config.Model,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := s.config.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(embResp.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(embResp.Data))
	}

	results := make([]valueobjects.Embedding, len(texts))
	for _, d := range embResp.Data {
		emb, err := valueobjects.NewEmbedding(d.Embedding)
		if err != nil {
			return nil, fmt.Errorf("invalid embedding at index %d: %w", d.Index, err)
		}
		if d.Index >= len(results) {
			return nil, fmt.Errorf("embedding index %d out of range", d.Index)
		}
		results[d.Index] = emb
	}

	s.logger.Debug("Generated embeddings batch",
		zap.Int("count", len(texts)),
		zap.Int("dimensions", results[0].Dimensions()),
	)

	return results, nil
}
