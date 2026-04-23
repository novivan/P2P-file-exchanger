package llm

import (
	"context"
	"errors"
)

type MockGenerator struct {
	Response string
	Err      error
}

func (m *MockGenerator) Chat(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

type ErrorGenerator struct{}

func (ErrorGenerator) Chat(ctx context.Context, system, user string, jsonMode bool) (string, error) {
	return "", errors.New("llm: generator is disabled")
}
