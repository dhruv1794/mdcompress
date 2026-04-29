package llm

import (
	"fmt"
	"strings"
	"testing"
)

type stubBackend struct {
	name      string
	model     string
	responses []string
	calls     []string
	idx       int
}

func (b *stubBackend) Name() string  { return b.name }
func (b *stubBackend) Model() string { return b.model }
func (b *stubBackend) Complete(prompt string) (string, error) {
	b.calls = append(b.calls, prompt)
	if b.idx >= len(b.responses) {
		return "", fmt.Errorf("stub: no response for call %d", b.idx)
	}
	out := b.responses[b.idx]
	b.idx++
	return out, nil
}

func longProse() string {
	sentence := "This library provides a streaming markdown parser that exposes block-level events for downstream consumers, including headings, paragraphs, code spans, and link references defined elsewhere in the document. "
	return strings.Repeat(sentence, 6)
}

func TestRewriter_RewritesProseSection(t *testing.T) {
	original := longProse()
	rewritten := "Streams markdown block events: headings, paragraphs, code spans, and link references."

	rewrite := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{rewritten},
	}
	judge := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{`{"score":1.0,"reason":"ok"}`},
	}

	source := []byte(original + "\n")
	r := &Rewriter{
		Backend:          rewrite,
		Judge:            judge,
		MinSectionTokens: 20,
		Threshold:        0.95,
	}
	out, stats, err := r.Rewrite(source)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if stats.SectionsConsidered != 1 {
		t.Fatalf("expected 1 considered, got %d", stats.SectionsConsidered)
	}
	if stats.SectionsRewritten != 1 {
		t.Fatalf("expected 1 rewrite, got %d (stats=%+v)", stats.SectionsRewritten, stats)
	}
	if !strings.Contains(string(out), rewritten) {
		t.Fatalf("output missing rewrite. got=%q", string(out))
	}
	if len(out) >= len(source) {
		t.Fatalf("rewrite did not reduce bytes: in=%d out=%d", len(source), len(out))
	}
}

func TestRewriter_FaithfulnessGateRejects(t *testing.T) {
	original := longProse()
	rewrite := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{"loses facts"},
	}
	judge := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{`{"score":0.4,"reason":"loses detail"}`},
	}

	source := []byte(original + "\n")
	r := &Rewriter{
		Backend:          rewrite,
		Judge:            judge,
		MinSectionTokens: 20,
		Threshold:        0.95,
	}
	out, stats, err := r.Rewrite(source)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if stats.SectionsRewritten != 0 {
		t.Fatalf("expected 0 rewrites due to gate, got %d", stats.SectionsRewritten)
	}
	if stats.SectionsSkipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", stats.SectionsSkipped)
	}
	if string(out) != string(source) {
		t.Fatalf("source should be unchanged when gate rejects")
	}
}

func TestRewriter_SkipsCodeTableQuoteHeading(t *testing.T) {
	source := []byte("# Heading\n\n> a long blockquote that contains many words and should never be rewritten by tier-3 because the rule is non-negotiable per spec\n\n```\nfunc main() { println(\"this is some code that is intentionally long enough to exceed any token threshold\") }\n```\n\n| col1 | col2 |\n|------|------|\n| this | that |\n| more | data |\n| even | more |\n")

	rewrite := &stubBackend{name: "ollama", model: "llama3.1:8b"}
	judge := &stubBackend{name: "ollama", model: "llama3.1:8b"}

	r := &Rewriter{
		Backend:          rewrite,
		Judge:            judge,
		MinSectionTokens: 5,
		Threshold:        0.95,
	}
	out, stats, err := r.Rewrite(source)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if stats.SectionsConsidered != 0 {
		t.Fatalf("expected 0 considered, got %d (stats=%+v)", stats.SectionsConsidered, stats)
	}
	if string(out) != string(source) {
		t.Fatalf("non-prose source should remain byte-identical")
	}
	if len(rewrite.calls) != 0 || len(judge.calls) != 0 {
		t.Fatalf("backend should not be called for non-prose sections")
	}
}

func TestRewriter_CacheHitsAvoidBackend(t *testing.T) {
	original := longProse()
	rewritten := "Streams markdown block events; preserves all referenced anchors and link targets verbatim."

	rewrite := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{rewritten},
	}
	judge := &stubBackend{
		name:      "ollama",
		model:     "llama3.1:8b",
		responses: []string{`{"score":0.99,"reason":"ok"}`},
	}

	cacheDir := t.TempDir()
	cache := NewCache(cacheDir)

	source := []byte(original + "\n")
	r := &Rewriter{
		Backend:          rewrite,
		Judge:            judge,
		MinSectionTokens: 20,
		Threshold:        0.95,
		Cache:            cache,
	}

	first, _, err := r.Rewrite(source)
	if err != nil {
		t.Fatalf("first rewrite: %v", err)
	}

	rewriteCallsAfterFirst := len(rewrite.calls)
	judgeCallsAfterFirst := len(judge.calls)

	second, stats, err := r.Rewrite(source)
	if err != nil {
		t.Fatalf("second rewrite: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("cache should produce identical output")
	}
	if len(rewrite.calls) != rewriteCallsAfterFirst {
		t.Fatalf("rewrite backend called again on cache hit (calls=%d)", len(rewrite.calls))
	}
	if len(judge.calls) != judgeCallsAfterFirst {
		t.Fatalf("judge backend called again on cache hit (calls=%d)", len(judge.calls))
	}
	if stats.CacheHits != 1 {
		t.Fatalf("expected 1 cache hit, got %d", stats.CacheHits)
	}
}
