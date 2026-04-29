// Package llm exposes the Tier-3 LLM-assisted rewrite pipeline.
//
// Tier-3 walks the markdown AST, sends each prose section to an LLM with a
// strict preserve-facts prompt, and replaces the original section with the
// rewrite when a faithfulness gate accepts it. The package is opt-in: nothing
// here runs unless the caller selects tier llm and provides a configured
// backend.
package llm

import (
	"fmt"
	"strings"
)

const (
	BackendOllama    = "ollama"
	BackendAnthropic = "anthropic"
	BackendOpenAI    = "openai"

	DefaultBackend  = BackendOllama
	DefaultModel    = "llama3.1:8b"
	DefaultEndpoint = "http://localhost:11434"

	DefaultMinSectionTokens = 200
	DefaultThreshold        = 0.95
)

// Backend completes a prompt to a string. Implementations live in
// ollama.go / anthropic.go / openai.go.
type Backend interface {
	Name() string
	Model() string
	Complete(prompt string) (string, error)
}

// Config holds the resolved Tier-3 settings, typically built from the
// project's config.yaml plus CLI flags.
type Config struct {
	Backend          string
	Model            string
	Endpoint         string
	APIKeyEnv        string
	Cache            bool
	CacheDir         string
	Threshold        float64
	MinSectionTokens int
}

// NewBackend constructs a backend from the resolved Config.
func NewBackend(cfg Config) (Backend, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if name == "" {
		name = DefaultBackend
	}
	switch name {
	case BackendOllama:
		return NewOllamaBackend(cfg.Endpoint, cfg.Model), nil
	case BackendAnthropic:
		return NewAnthropicBackend(cfg.Endpoint, cfg.Model, cfg.APIKeyEnv), nil
	case BackendOpenAI:
		return NewOpenAIBackend(cfg.Endpoint, cfg.Model, cfg.APIKeyEnv), nil
	default:
		return nil, fmt.Errorf("unknown llm backend %q", cfg.Backend)
	}
}
