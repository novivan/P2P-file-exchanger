package embedder

import "context"

type NoopEmbedder struct{}

func (n *NoopEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 1024), nil
}

func (n *NoopEmbedder) Len() int {
	return 1024
}
