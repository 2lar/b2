package valueobjects

import (
	"encoding/binary"
	"fmt"
	"math"

	pkgerrors "backend/pkg/errors"
)

// Embedding is a value object representing a vector embedding for semantic similarity.
// It wraps a float64 slice and provides cosine similarity computation.
type Embedding struct {
	vector     []float64
	dimensions int
}

// NewEmbedding creates an Embedding from a float64 slice with dimension validation.
func NewEmbedding(vector []float64) (Embedding, error) {
	if len(vector) == 0 {
		return Embedding{}, pkgerrors.NewValidationError("embedding vector cannot be empty")
	}

	// Validate no NaN or Inf values
	for i, v := range vector {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Embedding{}, fmt.Errorf("embedding vector contains invalid value at index %d", i)
		}
	}

	// Copy to ensure immutability
	vec := make([]float64, len(vector))
	copy(vec, vector)

	return Embedding{
		vector:     vec,
		dimensions: len(vector),
	}, nil
}

// NewEmbeddingFromBytes reconstructs an Embedding from a binary representation.
// Each float64 is stored as 8 bytes in little-endian order.
func NewEmbeddingFromBytes(data []byte) (Embedding, error) {
	if len(data) == 0 {
		return Embedding{}, pkgerrors.NewValidationError("embedding data cannot be empty")
	}
	if len(data)%8 != 0 {
		return Embedding{}, pkgerrors.NewValidationError("embedding data must be a multiple of 8 bytes")
	}

	dims := len(data) / 8
	vector := make([]float64, dims)
	for i := 0; i < dims; i++ {
		bits := binary.LittleEndian.Uint64(data[i*8 : (i+1)*8])
		vector[i] = math.Float64frombits(bits)
	}

	return NewEmbedding(vector)
}

// Vector returns a copy of the embedding vector.
func (e Embedding) Vector() []float64 {
	if e.vector == nil {
		return nil
	}
	vec := make([]float64, len(e.vector))
	copy(vec, e.vector)
	return vec
}

// Dimensions returns the number of dimensions.
func (e Embedding) Dimensions() int {
	return e.dimensions
}

// IsZero returns true if the embedding has no vector data.
func (e Embedding) IsZero() bool {
	return e.dimensions == 0
}

// ToBytes serializes the embedding to a binary representation.
// Each float64 is stored as 8 bytes in little-endian order.
func (e Embedding) ToBytes() []byte {
	if e.vector == nil {
		return nil
	}
	data := make([]byte, len(e.vector)*8)
	for i, v := range e.vector {
		bits := math.Float64bits(v)
		binary.LittleEndian.PutUint64(data[i*8:(i+1)*8], bits)
	}
	return data
}

// CosineSimilarity computes the cosine similarity between this embedding and another.
// Returns a value between -1.0 and 1.0, where 1.0 means identical direction.
// Returns 0.0 if either embedding is zero or dimensions don't match.
func (e Embedding) CosineSimilarity(other Embedding) float64 {
	if e.IsZero() || other.IsZero() {
		return 0.0
	}
	if e.dimensions != other.dimensions {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < e.dimensions; i++ {
		dotProduct += e.vector[i] * other.vector[i]
		normA += e.vector[i] * e.vector[i]
		normB += other.vector[i] * other.vector[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Equals checks if two embeddings are equal.
func (e Embedding) Equals(other Embedding) bool {
	if e.dimensions != other.dimensions {
		return false
	}
	for i := 0; i < e.dimensions; i++ {
		if math.Abs(e.vector[i]-other.vector[i]) > 1e-9 {
			return false
		}
	}
	return true
}
