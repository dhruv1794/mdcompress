package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DeepSeekBackendName     = "deepseek"
	DefaultDeepSeekEndpoint = "https://api.deepseek.com/v1"
	DefaultDeepSeekKeyEnv   = "DEEPSEEK_API_KEY"
	DefaultDeepSeekModel    = "deepseek-chat"
)

type DeepSeekBackend struct {
	Endpoint  string
	Model     string
	APIKeyEnv string
	Client    *http.Client
}

func NewDeepSeekBackend(endpoint, model, apiKeyEnv string) *DeepSeekBackend {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultDeepSeekEndpoint
	}
	if strings.TrimSpace(apiKeyEnv) == "" {
		apiKeyEnv = DefaultDeepSeekKeyEnv
	}
	if strings.TrimSpace(model) == "" {
		model = DefaultDeepSeekModel
	}
	return &DeepSeekBackend{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		Model:     strings.TrimSpace(model),
		APIKeyEnv: strings.TrimSpace(apiKeyEnv),
		Client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

func (b *DeepSeekBackend) Name() string {
	return DeepSeekBackendName
}

func (b *DeepSeekBackend) Complete(prompt string) (string, error) {
	if strings.TrimSpace(b.Model) == "" {
		return "", fmt.Errorf("deepseek eval backend requires a model")
	}
	apiKey := strings.TrimSpace(os.Getenv(b.APIKeyEnv))
	if apiKey == "" {
		return "", fmt.Errorf("deepseek eval backend requires %s", b.APIKeyEnv)
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	body, err := json.Marshal(map[string]any{
		"model": b.Model,
		"messages": []map[string]string{{
			"role":    "user",
			"content": prompt,
		}},
		"temperature": 0,
		"stream":      false,
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
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("deepseek returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
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
		return "", fmt.Errorf("deepseek: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("deepseek returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
