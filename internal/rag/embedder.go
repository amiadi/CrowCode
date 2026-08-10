package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	ModelLabel() string
	Dimensions() int
}

type hashEmbedder struct {
	dimensions int
	label      string
}

func newHashEmbedder(dimensions int, label string) Embedder {
	if dimensions <= 0 {
		dimensions = 128
	}
	if strings.TrimSpace(label) == "" {
		label = fmt.Sprintf("crowcode-hash-%d", dimensions)
	}
	return &hashEmbedder{dimensions: dimensions, label: label}
}

func (h *hashEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	return hashEmbedText(text, h.dimensions), nil
}

func (h *hashEmbedder) ModelLabel() string {
	return h.label
}

func (h *hashEmbedder) Dimensions() int {
	return h.dimensions
}

type ollamaEmbedder struct {
	httpClient *http.Client
	baseURL    string
	model      string
	dimensions int
}

func newOllamaEmbedder(baseURL, model string, timeout time.Duration, dimensions int) Embedder {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if strings.TrimSpace(model) == "" {
		model = "nomic-embed-text"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if dimensions <= 0 {
		dimensions = 768
	}

	return &ollamaEmbedder{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
	}
}

func (o *ollamaEmbedder) ModelLabel() string {
	return o.model
}

func (o *ollamaEmbedder) Dimensions() int {
	return o.dimensions
}

func (o *ollamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	payload := map[string]any{
		"model": o.model,
		"input": text,
	}

	attempts := []string{"/api/embed", "/api/embeddings", "/v1/embeddings"}
	var lastErr error
	for _, path := range attempts {
		vector, err := o.requestEmbedding(ctx, path, payload)
		if err == nil {
			if o.dimensions > 0 && len(vector) > o.dimensions {
				vector = vector[:o.dimensions]
			}
			if o.dimensions > 0 && len(vector) < o.dimensions {
				padded := make([]float64, o.dimensions)
				copy(padded, vector)
				vector = padded
			}
			return vector, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("embedding request failed")
	}
	return nil, lastErr
}

func (o *ollamaEmbedder) requestEmbedding(ctx context.Context, endpointPath string, payload map[string]any) ([]float64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolveEndpoint(o.baseURL, endpointPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("embedding endpoint %s returned %d: %s", endpointPath, resp.StatusCode, string(raw))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiEmbed struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(raw, &apiEmbed); err == nil && len(apiEmbed.Embedding) > 0 {
		return apiEmbed.Embedding, nil
	}

	var apiEmbeds struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.Unmarshal(raw, &apiEmbeds); err == nil && len(apiEmbeds.Embeddings) > 0 && len(apiEmbeds.Embeddings[0]) > 0 {
		return apiEmbeds.Embeddings[0], nil
	}

	var openAIEmbeds struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &openAIEmbeds); err == nil && len(openAIEmbeds.Data) > 0 && len(openAIEmbeds.Data[0].Embedding) > 0 {
		return openAIEmbeds.Data[0].Embedding, nil
	}

	return nil, fmt.Errorf("unsupported embedding response format")
}

func resolveEndpoint(base, endpointPath string) string {
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + endpointPath
	}
	u.Path = endpointPath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func newEmbedderFromConfig(cfg config.RAGConfig) Embedder {
	provider := strings.ToLower(strings.TrimSpace(cfg.Embeddings.Provider))
	dimensions := cfg.Embeddings.Dimensions
	model := cfg.Embeddings.Model
	timeout := time.Duration(cfg.Embeddings.TimeoutMs) * time.Millisecond

	switch provider {
	case "", "hash":
		return newHashEmbedder(dimensions, model)
	case "ollama", "local":
		baseURL := os.Getenv("LOCAL_ENDPOINT")
		if loaded := config.Get(); loaded != nil && strings.TrimSpace(loaded.LocalEndpoint) != "" {
			baseURL = loaded.LocalEndpoint
		}
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "http://127.0.0.1:11434"
		}
		return newOllamaEmbedder(baseURL, model, timeout, dimensions)
	default:
		return newHashEmbedder(dimensions, model)
	}
}
