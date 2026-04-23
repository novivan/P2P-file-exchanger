package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaGenerator_ChatJSON(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("bad request JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"{\"ok\":true}"}}`))
	}))
	defer server.Close()

	g := NewOllamaGenerator(server.URL, "qwen2.5:3b")

	got, err := g.Chat(context.Background(), "sys-prompt", "user-prompt", true)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if got != `{"ok":true}` {
		t.Errorf("unexpected content: %q", got)
	}

	if capturedBody["model"] != "qwen2.5:3b" {
		t.Errorf("model mismatch: %v", capturedBody["model"])
	}
	if capturedBody["format"] != "json" {
		t.Errorf("format should be json, got: %v", capturedBody["format"])
	}
	if capturedBody["stream"] != false {
		t.Errorf("stream should be false, got: %v", capturedBody["stream"])
	}
	msgs, ok := capturedBody["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got: %v", capturedBody["messages"])
	}
	m0 := msgs[0].(map[string]any)
	m1 := msgs[1].(map[string]any)
	if m0["role"] != "system" || m0["content"] != "sys-prompt" {
		t.Errorf("bad system msg: %v", m0)
	}
	if m1["role"] != "user" || m1["content"] != "user-prompt" {
		t.Errorf("bad user msg: %v", m1)
	}
}

func TestOllamaGenerator_NoSystem(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"ok"}}`))
	}))
	defer server.Close()

	g := NewOllamaGenerator(server.URL, "m")
	if _, err := g.Chat(context.Background(), "", "hi", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msgs := capturedBody["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if _, ok := capturedBody["format"]; ok {
		t.Errorf("format must be omitted when jsonMode=false")
	}
}

func TestOllamaGenerator_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found: foo", http.StatusNotFound)
	}))
	defer server.Close()

	g := NewOllamaGenerator(server.URL, "foo")
	_, err := g.Chat(context.Background(), "", "x", false)
	if err == nil {
		t.Fatal("expected error on non-200")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error should include status and body, got: %v", err)
	}
}

func TestMockGenerator_Response(t *testing.T) {
	m := &MockGenerator{Response: "hello"}
	got, err := m.Chat(context.Background(), "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestMockGenerator_Error(t *testing.T) {
	m := &MockGenerator{Err: io.EOF}
	_, err := m.Chat(context.Background(), "", "", false)
	if err != io.EOF {
		t.Errorf("got %v, want EOF", err)
	}
}
