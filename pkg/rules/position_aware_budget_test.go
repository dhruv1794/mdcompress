package rules

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

func runPAB(t *testing.T, in string) (string, ChangeSet) {
	t.Helper()
	rule := &PositionAwareBudget{}
	cs, err := rule.Apply(&Context{Source: []byte(in)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(render.ApplyEdits([]byte(in), cs.Edits)), cs
}

func codeBlock(contentLines int) string {
	return "```\n" + strings.Repeat("code line x\n", contentLines) + "```\n"
}

func TestPAB_NoOpOnShortDoc(t *testing.T) {
	// A 30-line code block, but the whole doc is well under the length gate.
	in := strings.Repeat("filler text line here\n", 20) +
		codeBlock(30) +
		strings.Repeat("filler text line here\n", 20)
	got, cs := runPAB(t, in)
	if got != in {
		t.Fatalf("short doc must be left alone, got %d bytes vs %d", len(got), len(in))
	}
	if len(cs.Edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(cs.Edits))
	}
}

func TestPAB_TruncatesMiddleBandBlock(t *testing.T) {
	in := strings.Repeat("filler text line here\n", 100) +
		codeBlock(30) +
		strings.Repeat("filler text line here\n", 100)
	got, cs := runPAB(t, in)
	if !strings.Contains(got, "[... 22 more lines ...]") {
		t.Fatalf("expected the middle block truncated to %d lines, got: %q", pabMiddleMaxLines, got)
	}
	if strings.Count(got, "code line x") != pabMiddleMaxLines {
		t.Fatalf("expected %d surviving code lines, got %d", pabMiddleMaxLines, strings.Count(got, "code line x"))
	}
	if len(cs.Edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(cs.Edits))
	}
	if len(got) >= len(in) {
		t.Fatalf("expected smaller output, got %d >= %d", len(got), len(in))
	}
}

func TestPAB_LeavesHeadAndTailBlocksAlone(t *testing.T) {
	in := codeBlock(12) +
		strings.Repeat("filler text line here\n", 200) +
		codeBlock(12)
	got, _ := runPAB(t, in)
	if got != in {
		t.Fatalf("head/tail code blocks must not be truncated, got: %q", got[:120])
	}
}

func TestPAB_NoOpOnSmallMiddleBlock(t *testing.T) {
	// A middle-band block at or under the tight cap is left intact.
	in := strings.Repeat("filler text line here\n", 100) +
		codeBlock(pabMiddleMaxLines) +
		strings.Repeat("filler text line here\n", 100)
	got, cs := runPAB(t, in)
	if got != in {
		t.Fatalf("a middle block within the cap must be untouched, got: %q", got)
	}
	if len(cs.Edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(cs.Edits))
	}
}
