package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
	dim     int
}

type OllamaEmbedRequest struct {
	Model string `json:"model"`
	Text  string `json:"text"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
		dim:     0,
	}
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := OllamaEmbedRequest{
		Model: o.model,
		Text:  text,
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
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if o.dim == 0 && len(result.Embedding) > 0 {
		o.dim = len(result.Embedding)
	}

	return result.Embedding, nil
}

func (o *OllamaEmbedder) Len() int {
	return o.dim
}
