package rag

import (
	"context"
	"math"
)

type EmbeddingBackend interface {
	Name() string
	Embed(ctx context.Context, text string, dims int) ([]float64, error)
}

type HashEmbeddingBackend struct{}

func (h *HashEmbeddingBackend) Name() string {
	return "hash"
}

func (h *HashEmbeddingBackend) Embed(_ context.Context, text string, dims int) ([]float64, error) {
	return EmbedText(text, dims), nil
}

func normalizeEmbedding(vec []float64) []float64 {
	norm := l2Norm(vec)
	if norm == 0 {
		return vec
	}
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func clipEmbedding(vec []float64, dims int) []float64 {
	if dims <= 0 || len(vec) == dims {
		return vec
	}
	if len(vec) > dims {
		return normalizeEmbedding(vec[:dims])
	}
	padded := make([]float64, dims)
	copy(padded, vec)
	return normalizeEmbedding(padded)
}

func vectorDistance(a []float64, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 1
	}
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	var sum float64
	for i := 0; i < limit; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
