package eval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekBackendComplete(t *testing.T) {
	t.Setenv("TEST_DEEPSEEK_KEY", "secret")
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
		if body["model"] != "deepseek-reasoner" {
			t.Fatalf("model = %#v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" answer "}}]}`))
	}))
	defer server.Close()

	backend := NewDeepSeekBackend(server.URL+"/v1", "deepseek-reasoner", "TEST_DEEPSEEK_KEY")
	got, err := backend.Complete("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "answer" {
		t.Fatalf("Complete = %q, want answer", got)
	}
}

func TestDeepSeekBackendDefaultsModel(t *testing.T) {
	b := NewDeepSeekBackend("", "", "DEEPSEEK_API_KEY")
	if b.Model != DefaultDeepSeekModel {
		t.Fatalf("Model = %q, want %q", b.Model, DefaultDeepSeekModel)
	}
	if b.Endpoint != strings.TrimRight(DefaultDeepSeekEndpoint, "/") {
		t.Fatalf("Endpoint = %q, want %q", b.Endpoint, DefaultDeepSeekEndpoint)
	}
}

func TestBedrockBackendComplete(t *testing.T) {
	t.Setenv("TEST_BEDROCK_KEY", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/invoke"
		if r.URL.Path != wantPath {
			t.Fatalf("path = %s, want %s", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["anthropic_version"] != "bedrock-2023-05-31" {
			t.Fatalf("anthropic_version = %#v", body["anthropic_version"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":" answer "}]}`))
	}))
	defer server.Close()

	backend := NewBedrockBackend(server.URL, "anthropic.claude-3-5-sonnet-20241022-v2:0", "TEST_BEDROCK_KEY")
	got, err := backend.Complete("prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "answer" {
		t.Fatalf("Complete = %q, want answer", got)
	}
}

func TestBedrockBackendRejectsNonAnthropicModel(t *testing.T) {
	t.Setenv("TEST_BEDROCK_KEY", "secret")
	b := NewBedrockBackend("https://example.invalid", "amazon.titan-text-express-v1", "TEST_BEDROCK_KEY")
	_, err := b.Complete("prompt")
	if err == nil || !strings.Contains(err.Error(), "anthropic.*") {
		t.Fatalf("expected non-anthropic rejection, got %v", err)
	}
}
