package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Generator interface {
	Chat(ctx context.Context, system, user string, jsonMode bool) (string, error)
}

type OllamaGenerator struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaGenerator(baseURL, model string) *OllamaGenerator {
	return &OllamaGenerator{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
}

func (g *OllamaGenerator) Chat(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	msgs := make([]chatMessage, 0, 2)
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: user})

	reqBody := chatRequest{
		Model:    g.model,
		Messages: msgs,
		Stream:   false,
	}
	if jsonMode {
		reqBody.Format = "json"
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("llm_api: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("llm: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}

	return result.Message.Content, nil
}
