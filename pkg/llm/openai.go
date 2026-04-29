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
	DefaultOpenAIEndpoint = "https://api.openai.com/v1"
	DefaultOpenAIKeyEnv   = "OPENAI_API_KEY"
)

type OpenAIBackend struct {
	Endpoint  string
	ModelID   string
	APIKeyEnv string
	Client    *http.Client
}

func NewOpenAIBackend(endpoint, model, apiKeyEnv string) *OpenAIBackend {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultOpenAIEndpoint
	}
	if strings.TrimSpace(apiKeyEnv) == "" {
		apiKeyEnv = DefaultOpenAIKeyEnv
	}
	return &OpenAIBackend{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		ModelID:   strings.TrimSpace(model),
		APIKeyEnv: strings.TrimSpace(apiKeyEnv),
		Client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *OpenAIBackend) Name() string  { return BackendOpenAI }
func (b *OpenAIBackend) Model() string { return b.ModelID }

func (b *OpenAIBackend) Complete(prompt string) (string, error) {
	if strings.TrimSpace(b.ModelID) == "" {
		return "", fmt.Errorf("openai backend requires a model")
	}
	apiKey := strings.TrimSpace(os.Getenv(b.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("openai backend requires %s", b.APIKeyEnv)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	body, err := json.Marshal(map[string]any{
		"model": b.ModelID,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
		"temperature": 0,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, b.Endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai returned %s", resp.Status)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error.Message != "" {
		return "", fmt.Errorf("openai: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
