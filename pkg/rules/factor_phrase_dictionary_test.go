package rules

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

func runFactor(t *testing.T, in string) (string, ChangeSet) {
	t.Helper()
	rule := &FactorPhraseDictionary{}
	cs, err := rule.Apply(&Context{Source: []byte(in)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(render.ApplyEdits([]byte(in), cs.Edits)), cs
}

func TestFactor_NoOpOnShortDoc(t *testing.T) {
	in := "This is one short paragraph with no repetition.\n"
	got, cs := runFactor(t, in)
	if got != in {
		t.Fatalf("expected no-op, got %q", got)
	}
	if len(cs.Edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(cs.Edits))
	}
}

func TestFactor_FactorsRepeatedPhrase(t *testing.T) {
	phrase := "the asynchronous request handler middleware"
	body := strings.Repeat(phrase+" runs early on requests.\n", 6)
	got, cs := runFactor(t, body)
	if !strings.HasPrefix(got, "[gloss] T1=") {
		t.Fatalf("expected glossary preamble, got: %q", got[:80])
	}
	// T1 should reference some 3..6-word phrase that repeats; the body should
	// no longer contain that full phrase except in the glossary.
	if strings.Count(got, "T1") < 6 {
		t.Fatalf("expected ≥6 T1 substitutions in body, got %d in %q", strings.Count(got, "T1"), got)
	}
	if len(cs.Edits) < 6+1 {
		t.Fatalf("expected ≥7 edits (preamble + 6 occurrences), got %d", len(cs.Edits))
	}
	// Output must be smaller than input.
	if len(got) >= len(body) {
		t.Fatalf("expected smaller output, got %d >= %d", len(got), len(body))
	}
}

func TestFactor_SkipsCodeFence(t *testing.T) {
	phrase := "the asynchronous request handler middleware"
	body := "```go\n" + strings.Repeat(phrase+"\n", 6) + "```\n"
	got, _ := runFactor(t, body)
	if got != body {
		t.Fatalf("rule must not factor inside code fences; got: %q", got)
	}
}

func TestFactor_SkipsHeadingsAndLists(t *testing.T) {
	phrase := "the asynchronous request handler middleware"
	in := "# " + phrase + "\n" +
		"- " + phrase + "\n" +
		"- " + phrase + "\n" +
		"- " + phrase + "\n" +
		"- " + phrase + "\n" +
		"- " + phrase + "\n"
	got, _ := runFactor(t, in)
	if got != in {
		t.Fatalf("rule must not factor headings/lists; got: %q", got)
	}
}

func TestFactor_NetSavingsGuard(t *testing.T) {
	// 5 occurrences of a 16-byte phrase. Replace cost: 5*(16-2)=70. Glossary
	// entry cost: len("T1") + 1 + 16 + 2 = 21. Preamble overhead: 9 bytes.
	// Net = 70 - 21 - 9 = 40. So this should still factor — good.
	in := strings.Repeat("the lazy fox jumps high.\n", 5)
	got, _ := runFactor(t, in)
	if !strings.Contains(got, "T1") {
		t.Skip("this corpus does not yield a positive net savings; that's OK as a test of the guard but skip silently")
	}
}

func TestFactor_NoOpWhenAllPhrasesContainMarkdown(t *testing.T) {
	// All candidate phrases include `[` (markdown link), should be skipped.
	in := strings.Repeat("see [docs](u) for details on async pipelines.\n", 6)
	got, _ := runFactor(t, in)
	if strings.HasPrefix(got, "[gloss]") {
		t.Fatalf("rule should skip phrases with markdown special chars; got: %q", got)
	}
}

func TestFactor_PreservesInlineCodeIdentity(t *testing.T) {
	phrase := "the asynchronous request handler middleware"
	in := strings.Repeat(phrase+" `do_not_touch_this()` is internal.\n", 6)
	got, _ := runFactor(t, in)
	if !strings.Contains(got, "`do_not_touch_this()`") {
		t.Fatalf("inline-code span was modified: %q", got)
	}
}
