package llmtest_test

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/llm"
	"github.com/dhruv1794/mdcompress/pkg/llm/llmtest"
)

func TestBackendRewriteAndJudgeFlow(t *testing.T) {
	rewriter := llmtest.New("ollama", "llama3.1:8b",
		"Streams markdown block events; preserves all referenced anchors and link targets verbatim.",
	)
	judge := llmtest.New("anthropic", "claude-3-haiku",
		`{"score":0.99,"reason":"ok"}`,
	)

	long := strings.Repeat("This library streams markdown block events for downstream consumers, including headings, paragraphs, code spans, and link references. ", 6)
	r := &llm.Rewriter{
		Backend:          rewriter,
		Judge:            judge,
		MinSectionTokens: 20,
		Threshold:        0.95,
	}

	out, stats, err := r.Rewrite([]byte(long + "\n"))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if stats.SectionsRewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %+v", stats)
	}
	if !strings.Contains(string(out), "preserves all referenced anchors") {
		t.Fatalf("rewrite not applied: %q", out)
	}
	if len(rewriter.Calls()) != 1 {
		t.Fatalf("expected 1 rewriter call, got %d", len(rewriter.Calls()))
	}
}

func TestBackendErrorsWhenOutOfResponses(t *testing.T) {
	b := llmtest.New("anthropic", "claude", "first")
	if _, err := b.Complete("p1"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := b.Complete("p2"); err == nil {
		t.Fatalf("expected error on second call without queued response")
	}
}
