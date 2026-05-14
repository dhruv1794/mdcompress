package rules

import (
	"strings"
	"testing"
)

func runCrossFileParagraphs(t *testing.T, cfs *CrossFileState, file, source string) string {
	t.Helper()
	r := &CrossFileParagraphs{}
	ctx := &Context{
		Source:    []byte(source),
		Config:    &Config{Tier: TierAggressive},
		FilePath:  file,
		CrossFile: cfs,
	}
	changes, err := r.Apply(ctx)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return applyChanges([]byte(source), changes)
}

func applyChanges(source []byte, changes ChangeSet) string {
	// Minimal stand-in for render.ApplyEdits to keep this test pkg-local.
	// We sort by Start descending so non-overlapping edits apply correctly.
	out := append([]byte(nil), source...)
	for i := len(changes.Edits) - 1; i >= 0; i-- {
		e := changes.Edits[i]
		out = append(out[:e.Start], append(append([]byte(nil), e.Replacement...), out[e.End:]...)...)
	}
	return string(out)
}

const longPara = "This paragraph is intentionally long enough to clear the minimum word threshold required by the cross file paragraph factoring rule which demands at least thirty words of substantive prose before it bothers fingerprinting and recording the content across files."

func TestCrossFileParagraphs_ReplacesAfterThreshold(t *testing.T) {
	cfs := &CrossFileState{}
	doc := "# Title\n\n" + longPara + "\n\n## Other\n\nshort one.\n"

	out1 := runCrossFileParagraphs(t, cfs, "a.md", doc)
	if out1 != doc {
		t.Fatalf("first file should be untouched, got:\n%s", out1)
	}
	out2 := runCrossFileParagraphs(t, cfs, "b.md", doc)
	if out2 != doc {
		t.Fatalf("second file should be untouched (count==2), got:\n%s", out2)
	}
	out3 := runCrossFileParagraphs(t, cfs, "c.md", doc)
	if !strings.Contains(out3, "[same as in a.md]") {
		t.Fatalf("third file should reference a.md, got:\n%s", out3)
	}
	if strings.Contains(out3, longPara) {
		t.Fatalf("third file should not still contain the paragraph verbatim")
	}
}

func TestCrossFileParagraphs_SkipsShortParagraphs(t *testing.T) {
	cfs := &CrossFileState{}
	doc := "Short paragraph.\n"
	for _, f := range []string{"a.md", "b.md", "c.md", "d.md"} {
		out := runCrossFileParagraphs(t, cfs, f, doc)
		if out != doc {
			t.Fatalf("short paragraphs must not be factored, got: %q", out)
		}
	}
}

func TestCrossFileParagraphs_SkipsCodeAndHeadings(t *testing.T) {
	cfs := &CrossFileState{}
	doc := "```\n" + longPara + "\n```\n\n# " + longPara + "\n"
	for _, f := range []string{"a.md", "b.md", "c.md"} {
		out := runCrossFileParagraphs(t, cfs, f, doc)
		if out != doc {
			t.Fatalf("fenced/heading content must be skipped, got:\n%s", out)
		}
	}
}

func TestCrossFileParagraphs_CanonicalFileIsFirstSeen(t *testing.T) {
	cfs := &CrossFileState{}
	doc := longPara + "\n"
	_ = runCrossFileParagraphs(t, cfs, "first.md", doc)
	_ = runCrossFileParagraphs(t, cfs, "second.md", doc)
	out3 := runCrossFileParagraphs(t, cfs, "third.md", doc)
	if !strings.Contains(out3, "[same as in first.md]") {
		t.Fatalf("expected reference to first.md, got:\n%s", out3)
	}
}
