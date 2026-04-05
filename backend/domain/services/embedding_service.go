package services

import (
	"context"

	"backend/domain/core/valueobjects"
)

// EmbeddingService generates vector embeddings from text content.
// Implementations may call external APIs (Bedrock, OpenAI) or run local models.
type EmbeddingService interface {
	// GenerateEmbedding produces a vector embedding for a single text input.
	GenerateEmbedding(ctx context.Context, text string) (valueobjects.Embedding, error)

	// GenerateEmbeddings produces vector embeddings for multiple texts in a single batch call.
	GenerateEmbeddings(ctx context.Context, texts []string) ([]valueobjects.Embedding, error)

	// Dimensions returns the number of dimensions produced by this service's model.
	Dimensions() int
}
