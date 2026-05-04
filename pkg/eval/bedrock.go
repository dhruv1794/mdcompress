package eval

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
	BedrockBackendName     = "bedrock"
	DefaultBedrockEndpoint = "https://bedrock-runtime.us-east-1.amazonaws.com"
	DefaultBedrockKeyEnv   = "AWS_BEARER_TOKEN_BEDROCK"
)

// BedrockBackend invokes a Bedrock model. Auth uses a bearer token from
// APIKeyEnv (no SigV4). Only anthropic.* model IDs are supported, since
// Bedrock's request body is per-model-family.
type BedrockBackend struct {
	Endpoint  string
	Model     string
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
		Model:     strings.TrimSpace(model),
		APIKeyEnv: strings.TrimSpace(apiKeyEnv),
		Client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

func (b *BedrockBackend) Name() string {
	return BedrockBackendName
}

func (b *BedrockBackend) Complete(prompt string) (string, error) {
	if strings.TrimSpace(b.Model) == "" {
		return "", fmt.Errorf("bedrock eval backend requires a model")
	}
	if !strings.HasPrefix(b.Model, "anthropic.") {
		return "", fmt.Errorf("bedrock eval backend currently supports only anthropic.* models, got %q", b.Model)
	}
	apiKey := strings.TrimSpace(os.Getenv(b.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("bedrock eval backend requires %s", b.APIKeyEnv)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	body, err := json.Marshal(map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        2048,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
	})
	if err != nil {
		return "", err
	}

	invokeURL := b.Endpoint + "/model/" + url.PathEscape(b.Model) + "/invoke"
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
