package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultAnthropicEndpoint = "https://api.anthropic.com/v1"
	DefaultAnthropicKeyEnv   = "ANTHROPIC_API_KEY"
)

type AnthropicBackend struct {
	Endpoint  string
	ModelID   string
	APIKeyEnv string
	Client    *http.Client
}

func NewAnthropicBackend(endpoint, model, apiKeyEnv string) *AnthropicBackend {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultAnthropicEndpoint
	}
	if strings.TrimSpace(apiKeyEnv) == "" {
		apiKeyEnv = DefaultAnthropicKeyEnv
	}
	return &AnthropicBackend{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		ModelID:   strings.TrimSpace(model),
		APIKeyEnv: strings.TrimSpace(apiKeyEnv),
		Client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *AnthropicBackend) Name() string  { return BackendAnthropic }
func (b *AnthropicBackend) Model() string { return b.ModelID }

func (b *AnthropicBackend) Complete(prompt string) (string, error) {
	if strings.TrimSpace(b.ModelID) == "" {
		return "", fmt.Errorf("anthropic backend requires a model")
	}
	apiKey := strings.TrimSpace(os.Getenv(b.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("anthropic backend requires %s", b.APIKeyEnv)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	body, err := json.Marshal(map[string]any{
		"model":      b.ModelID,
		"max_tokens": 4096,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, b.Endpoint+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic returned %s", resp.Status)
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error.Message != "" {
		return "", fmt.Errorf("anthropic: %s", out.Error.Message)
	}
	for _, block := range out.Content {
		if block.Type == "text" || block.Type == "" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("anthropic returned no text content")
}
