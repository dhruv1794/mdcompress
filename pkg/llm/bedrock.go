package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultBedrockEndpoint = "https://bedrock-runtime.us-east-1.amazonaws.com"
	DefaultBedrockKeyEnv   = "AWS_BEARER_TOKEN_BEDROCK"
)

// BedrockBackend invokes a model on AWS Bedrock via the InvokeModel HTTP API.
//
// Authentication uses a Bedrock API key from APIKeyEnv (default
// AWS_BEARER_TOKEN_BEDROCK) sent as a bearer token. SigV4 is intentionally not
// supported here — to keep the dependency footprint small, callers who need
// SigV4 should generate a short-term API key and export it as an env var.
//
// Currently only Anthropic Claude models on Bedrock are supported, since
// Bedrock's request body is per-model-family. The model ID is the Bedrock
// model identifier, e.g. "anthropic.claude-3-5-sonnet-20241022-v2:0".
type BedrockBackend struct {
	Endpoint  string
	ModelID   string
	APIKeyEnv string
	Client    *http.Client
}

func NewBedrockBackend(endpoint, model, apiKeyEnv string) *BedrockBackend {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultBedrockEndpoint
	}
	if strings.TrimSpace(apiKeyEnv) == "" {
		apiKeyEnv = DefaultBedrockKeyEnv
	}
	return &BedrockBackend{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		ModelID:   strings.TrimSpace(model),
		APIKeyEnv: strings.TrimSpace(apiKeyEnv),
		Client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *BedrockBackend) Name() string  { return BackendBedrock }
func (b *BedrockBackend) Model() string { return b.ModelID }

func (b *BedrockBackend) Complete(prompt string) (string, error) {
	if strings.TrimSpace(b.ModelID) == "" {
		return "", fmt.Errorf("bedrock backend requires a model")
	}
	if !strings.HasPrefix(b.ModelID, "anthropic.") {
		return "", fmt.Errorf("bedrock backend currently supports only anthropic.* models, got %q", b.ModelID)
	}
	apiKey := strings.TrimSpace(os.Getenv(b.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("bedrock backend requires %s", b.APIKeyEnv)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	body, err := json.Marshal(map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        4096,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
	})
	if err != nil {
		return "", err
	}
	invokeURL := b.Endpoint + "/model/" + url.PathEscape(b.ModelID) + "/invoke"
	req, err := http.NewRequest(http.MethodPost, invokeURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("bedrock returned %s", resp.Status)
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Message != "" {
		return "", fmt.Errorf("bedrock: %s", out.Message)
	}
	for _, block := range out.Content {
		if block.Type == "text" || block.Type == "" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("bedrock returned no text content")
}
