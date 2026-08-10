package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OllamaEmbeddingBackend struct {
	endpoint string
	model    string
	client   *http.Client
}

type ollamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

type openAIEmbeddingEnvelope struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func NewOllamaEmbeddingBackend(endpoint string, model string, timeout time.Duration) (*OllamaEmbeddingBackend, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("LOCAL_ENDPOINT"))
	}
	if endpoint == "" {
		return nil, fmt.Errorf("LOCAL_ENDPOINT is required for ollama embeddings")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("embedding model is required for ollama embeddings")
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &OllamaEmbeddingBackend{
		endpoint: strings.TrimRight(endpoint, "/"),
		model:    model,
		client:   &http.Client{Timeout: timeout},
	}, nil
}

func (o *OllamaEmbeddingBackend) Name() string {
	return "ollama"
}

func (o *OllamaEmbeddingBackend) Embed(ctx context.Context, text string, dims int) ([]float64, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("text is required for embeddings")
	}
	vec, err := o.embedWithOllamaAPI(ctx, text)
	if err != nil {
		vec, err = o.embedWithOpenAICompatibleAPI(ctx, text)
		if err != nil {
			return nil, err
		}
	}
	vec = clipEmbedding(vec, dims)
	if len(vec) == 0 {
		return nil, fmt.Errorf("empty embedding vector returned")
	}
	return vec, nil
}

func (o *OllamaEmbeddingBackend) embedWithOllamaAPI(ctx context.Context, text string) ([]float64, error) {
	payload := map[string]any{
		"model":  o.model,
		"prompt": text,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embeddings api returned status %d", resp.StatusCode)
	}
	var parsed ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embeddings api returned empty embedding")
	}
	return normalizeEmbedding(parsed.Embedding), nil
}

func (o *OllamaEmbeddingBackend) embedWithOpenAICompatibleAPI(ctx context.Context, text string) ([]float64, error) {
	payload := map[string]any{
		"model": o.model,
		"input": text,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai-compatible embeddings api returned status %d", resp.StatusCode)
	}
	var parsed openAIEmbeddingEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai-compatible embeddings api returned empty embedding")
	}
	return normalizeEmbedding(parsed.Data[0].Embedding), nil
}
