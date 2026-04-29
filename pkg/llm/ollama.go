package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OllamaBackend struct {
	Endpoint string
	ModelID  string
	Client   *http.Client
}

func NewOllamaBackend(endpoint, model string) *OllamaBackend {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultEndpoint
	}
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	return &OllamaBackend{
		Endpoint: strings.TrimRight(endpoint, "/"),
		ModelID:  model,
		Client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *OllamaBackend) Name() string  { return BackendOllama }
func (b *OllamaBackend) Model() string { return b.ModelID }

func (b *OllamaBackend) Complete(prompt string) (string, error) {
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	body, err := json.Marshal(map[string]any{
		"model":  b.ModelID,
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return "", err
	}
	resp, err := client.Post(b.Endpoint+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama returned %s", resp.Status)
	}
	var out struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: %s", out.Error)
	}
	return strings.TrimSpace(out.Response), nil
}
