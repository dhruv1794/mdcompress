package eval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIBackendComplete(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "test-model" {
			t.Fatalf("model = %#v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" answer "}}]}`))
	}))
	defer server.Close()

	backend := NewOpenAIBackend(server.URL+"/v1", "test-model", "TEST_OPENAI_KEY")
	got, err := backend.Complete("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "answer" {
		t.Fatalf("Complete = %q, want answer", got)
	}
}

func TestAnthropicBackendComplete(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatalf("missing anthropic-version header")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "test-model" {
			t.Fatalf("model = %#v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":" answer "}]}`))
	}))
	defer server.Close()

	backend := NewAnthropicBackend(server.URL+"/v1", "test-model", "TEST_ANTHROPIC_KEY")
	got, err := backend.Complete("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "answer" {
		t.Fatalf("Complete = %q, want answer", got)
	}
}

func TestOptionalBackendRequiresModelAndKey(t *testing.T) {
	if _, err := NewOpenAIBackend("", "", "MISSING_KEY").Complete("prompt"); err == nil || !strings.Contains(err.Error(), "requires a model") {
		t.Fatalf("openai missing model error = %v", err)
	}
	if _, err := NewAnthropicBackend("", "model", "MISSING_KEY").Complete("prompt"); err == nil || !strings.Contains(err.Error(), "MISSING_KEY") {
		t.Fatalf("anthropic missing key error = %v", err)
	}
}
