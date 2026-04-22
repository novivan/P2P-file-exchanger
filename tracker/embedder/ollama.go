package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
	mu      sync.Mutex
	dim     int
}

type OllamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type OllamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := OllamaEmbedRequest{
		Model: o.model,
		Input: text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/embed", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama returned empty embeddings (check model (\"%q\") and input)", o.model)
	}

	embedding := result.Embeddings[0]

	if o.dim == 0 {
		o.mu.Lock()
		o.dim = len(embedding)
		o.mu.Unlock()
	}

	return embedding, nil
}

func (o *OllamaEmbedder) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.dim
}
