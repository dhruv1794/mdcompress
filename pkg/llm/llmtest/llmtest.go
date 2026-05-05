// Package llmtest provides a deterministic Backend fake for testing Tier-3
// rewriter behavior without making network calls.
package llmtest

import (
	"fmt"

	"github.com/dhruv1794/mdcompress/pkg/llm"
)

// Backend is a scripted llm.Backend: it returns the responses you queue, in
// order. After all queued responses are consumed, it returns an error.
type Backend struct {
	name      string
	model     string
	responses []string
	calls     []string
}

// New constructs a fake Backend. name and model are reported by the Backend
// interface; the rewriter requires the rewrite and judge identities to differ,
// so pass two Backends with distinct names when wiring a full rewriter.
func New(name, model string, responses ...string) *Backend {
	return &Backend{name: name, model: model, responses: append([]string(nil), responses...)}
}

func (b *Backend) Name() string  { return b.name }
func (b *Backend) Model() string { return b.model }

// Complete records the prompt and pops the next queued response.
func (b *Backend) Complete(prompt string) (string, error) {
	b.calls = append(b.calls, prompt)
	if len(b.calls) > len(b.responses) {
		return "", fmt.Errorf("llmtest: no response queued for call %d", len(b.calls))
	}
	return b.responses[len(b.calls)-1], nil
}

// Calls returns the prompts received in order. Useful for asserting that the
// rewriter sent the expected sections.
func (b *Backend) Calls() []string {
	out := make([]string, len(b.calls))
	copy(out, b.calls)
	return out
}

// Compile-time check that Backend satisfies llm.Backend.
var _ llm.Backend = (*Backend)(nil)
