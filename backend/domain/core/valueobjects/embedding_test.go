package valueobjects

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmbedding(t *testing.T) {
	t.Run("valid embedding", func(t *testing.T) {
		vec := []float64{0.1, 0.2, 0.3}
		emb, err := NewEmbedding(vec)
		require.NoError(t, err)
		assert.Equal(t, 3, emb.Dimensions())
		assert.Equal(t, vec, emb.Vector())
	})

	t.Run("empty vector rejected", func(t *testing.T) {
		_, err := NewEmbedding([]float64{})
		assert.Error(t, err)
	})

	t.Run("NaN rejected", func(t *testing.T) {
		_, err := NewEmbedding([]float64{1.0, math.NaN(), 3.0})
		assert.Error(t, err)
	})

	t.Run("Inf rejected", func(t *testing.T) {
		_, err := NewEmbedding([]float64{1.0, math.Inf(1), 3.0})
		assert.Error(t, err)
	})

	t.Run("immutability - modifying source doesnt affect embedding", func(t *testing.T) {
		vec := []float64{1.0, 2.0, 3.0}
		emb, err := NewEmbedding(vec)
		require.NoError(t, err)
		vec[0] = 999.0
		assert.Equal(t, 1.0, emb.Vector()[0])
	})
}

func TestEmbedding_IsZero(t *testing.T) {
	assert.True(t, Embedding{}.IsZero())

	emb, _ := NewEmbedding([]float64{0.1})
	assert.False(t, emb.IsZero())
}

func TestEmbedding_CosineSimilarity(t *testing.T) {
	t.Run("identical vectors", func(t *testing.T) {
		emb, _ := NewEmbedding([]float64{1.0, 0.0, 0.0})
		assert.InDelta(t, 1.0, emb.CosineSimilarity(emb), 1e-9)
	})

	t.Run("orthogonal vectors", func(t *testing.T) {
		a, _ := NewEmbedding([]float64{1.0, 0.0})
		b, _ := NewEmbedding([]float64{0.0, 1.0})
		assert.InDelta(t, 0.0, a.CosineSimilarity(b), 1e-9)
	})

	t.Run("opposite vectors", func(t *testing.T) {
		a, _ := NewEmbedding([]float64{1.0, 0.0})
		b, _ := NewEmbedding([]float64{-1.0, 0.0})
		assert.InDelta(t, -1.0, a.CosineSimilarity(b), 1e-9)
	})

	t.Run("similar vectors", func(t *testing.T) {
		a, _ := NewEmbedding([]float64{1.0, 1.0, 0.0})
		b, _ := NewEmbedding([]float64{1.0, 0.0, 0.0})
		sim := a.CosineSimilarity(b)
		assert.Greater(t, sim, 0.5)
		assert.Less(t, sim, 1.0)
	})

	t.Run("dimension mismatch returns 0", func(t *testing.T) {
		a, _ := NewEmbedding([]float64{1.0, 2.0})
		b, _ := NewEmbedding([]float64{1.0, 2.0, 3.0})
		assert.Equal(t, 0.0, a.CosineSimilarity(b))
	})

	t.Run("zero embedding returns 0", func(t *testing.T) {
		a, _ := NewEmbedding([]float64{1.0, 2.0})
		assert.Equal(t, 0.0, a.CosineSimilarity(Embedding{}))
	})
}

func TestEmbedding_ByteRoundtrip(t *testing.T) {
	original, err := NewEmbedding([]float64{0.123456789, -0.987654321, 0.0, 1.0})
	require.NoError(t, err)

	data := original.ToBytes()
	assert.Len(t, data, 4*8)

	restored, err := NewEmbeddingFromBytes(data)
	require.NoError(t, err)

	assert.True(t, original.Equals(restored))
	assert.Equal(t, original.Dimensions(), restored.Dimensions())
}

func TestEmbedding_ByteEdgeCases(t *testing.T) {
	t.Run("empty bytes rejected", func(t *testing.T) {
		_, err := NewEmbeddingFromBytes([]byte{})
		assert.Error(t, err)
	})

	t.Run("non-multiple-of-8 rejected", func(t *testing.T) {
		_, err := NewEmbeddingFromBytes([]byte{1, 2, 3})
		assert.Error(t, err)
	})

	t.Run("nil bytes rejected", func(t *testing.T) {
		_, err := NewEmbeddingFromBytes(nil)
		assert.Error(t, err)
	})

	t.Run("zero embedding ToBytes returns nil", func(t *testing.T) {
		assert.Nil(t, Embedding{}.ToBytes())
	})
}

func TestEmbedding_Equals(t *testing.T) {
	a, _ := NewEmbedding([]float64{1.0, 2.0, 3.0})
	b, _ := NewEmbedding([]float64{1.0, 2.0, 3.0})
	c, _ := NewEmbedding([]float64{1.0, 2.0, 3.1})

	assert.True(t, a.Equals(b))
	assert.False(t, a.Equals(c))
}
