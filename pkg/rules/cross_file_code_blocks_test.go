package rules_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func applyCrossFileCB(t *testing.T, src []byte, file string, state *rules.CrossFileState) []byte {
	t.Helper()
	rule := &rules.CrossFileCodeBlocks{}
	cs, err := rule.Apply(&rules.Context{
		Source:    src,
		FilePath:  file,
		CrossFile: state,
		Config:    &rules.Config{Tier: rules.TierAggressive},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return render.ApplyEdits(src, cs.Edits)
}

// First file should be left alone; second identical block should be replaced.
func TestCrossFileCodeBlocksDedupesIdenticalBlockAcrossFiles(t *testing.T) {
	state := &rules.CrossFileState{}
	body := "```go\n" + strings.Repeat("fmt.Println(\"hello world from compressor\")\n", 5) + "```\n"

	a := applyCrossFileCB(t, []byte(body), "docs/a.md", state)
	if !bytes.Equal(a, []byte(body)) {
		t.Fatalf("first file should be unchanged: %q", a)
	}

	b := applyCrossFileCB(t, []byte(body), "docs/b.md", state)
	if !bytes.Contains(b, []byte("[same as a.md")) {
		t.Fatalf("expected reference to canonical file, got %q", b)
	}
}

// No CrossFile state → rule must be a no-op.
func TestCrossFileCodeBlocksNoStateIsNoop(t *testing.T) {
	body := "```go\n" + strings.Repeat("fmt.Println(\"x\")\n", 5) + "```\n"
	rule := &rules.CrossFileCodeBlocks{}
	cs, err := rule.Apply(&rules.Context{Source: []byte(body), FilePath: "a.md", Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(cs.Edits) != 0 {
		t.Fatalf("expected no edits without CrossFile state, got %d", len(cs.Edits))
	}
}

// Tiny block (< 20 normalized chars) is below threshold and must NOT be replaced.
func TestCrossFileCodeBlocksSkipsTinyBlocks(t *testing.T) {
	state := &rules.CrossFileState{}
	body := "```go\nx := 1\n```\n"
	_ = applyCrossFileCB(t, []byte(body), "a.md", state)
	got := applyCrossFileCB(t, []byte(body), "b.md", state)
	if !bytes.Equal(got, []byte(body)) {
		t.Fatalf("tiny block should not be deduped: %q", got)
	}
}

// Structurally different code in two files: neither should be replaced.
func TestCrossFileCodeBlocksKeepsStructurallyDistinctBlocks(t *testing.T) {
	state := &rules.CrossFileState{}
	a := "```go\nfunc Add(x, y int) int { return x + y }\nfunc Sub(x, y int) int { return x - y }\nfunc Mul(x, y int) int { return x * y }\n```\n"
	b := "```python\nclass Server:\n    def start(self): pass\n    def stop(self): pass\n    def restart(self): self.stop(); self.start()\n```\n"

	_ = applyCrossFileCB(t, []byte(a), "a.md", state)
	gotB := applyCrossFileCB(t, []byte(b), "b.md", state)
	if !bytes.Equal(gotB, []byte(b)) {
		t.Fatalf("distinct block should not be deduped: %q", gotB)
	}
}

// Reference replacement should never make the document larger.
func TestCrossFileCodeBlocksDoesNotInflateOutput(t *testing.T) {
	state := &rules.CrossFileState{}
	// Block is just barely over the 20-char threshold but small enough that
	// "[same as ...:N]" reference might be longer than original.
	body := "```\n" + strings.Repeat("a", 20) + "\n```\n"
	_ = applyCrossFileCB(t, []byte(body), "a.md", state)
	gotB := applyCrossFileCB(t, []byte(body), "b.md", state)
	if len(gotB) > len(body) {
		t.Fatalf("replacement inflated output: %d > %d", len(gotB), len(body))
	}
}
