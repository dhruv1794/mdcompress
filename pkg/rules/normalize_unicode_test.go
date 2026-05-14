package rules

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

func runNormalizeUnicode(t *testing.T, input string) string {
	t.Helper()
	rule := &NormalizeUnicode{}
	ctx := &Context{Source: []byte(input)}
	cs, err := rule.Apply(ctx)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(render.ApplyEdits([]byte(input), cs.Edits))
}

func TestNormalizeUnicode_SmartQuotes(t *testing.T) {
	in := "He said “hello” and ‘world’."
	want := `He said "hello" and 'world'.`
	if got := runNormalizeUnicode(t, in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeUnicode_Dashes(t *testing.T) {
	in := "range –1—2 ―end −x"
	want := "range -1--2 --end -x"
	if got := runNormalizeUnicode(t, in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeUnicode_NBSPAndEllipsis(t *testing.T) {
	in := "wait a moment…"
	want := "wait a moment..."
	if got := runNormalizeUnicode(t, in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeUnicode_BulletAndInvisible(t *testing.T) {
	in := "\u2022 item one\u00ad\nzero\u200bwidth\ufeffbom"
	want := "* item one\nzerowidthbom"
	if got := runNormalizeUnicode(t, in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeUnicode_PreservesCodeFence(t *testing.T) {
	in := "```go\nfunc f() { println(\"“hi”\") }\n```\n"
	got := runNormalizeUnicode(t, in)
	if !strings.Contains(got, "“hi”") {
		t.Fatalf("smart quotes inside fence were rewritten: %q", got)
	}
}

func TestNormalizeUnicode_PreservesInlineCode(t *testing.T) {
	in := "Use `“hi”` in code; outside the fence “hi” is plain."
	got := runNormalizeUnicode(t, in)
	if !strings.Contains(got, "`“hi”`") {
		t.Fatalf("inline-code was rewritten: %q", got)
	}
	if !strings.Contains(got, `"hi"`) {
		t.Fatalf("non-inline smart quotes were not normalized: %q", got)
	}
}

func TestNormalizeUnicode_NoOpOnPlainASCII(t *testing.T) {
	in := "Plain ASCII line with `code` and \"quotes\".\n"
	if got := runNormalizeUnicode(t, in); got != in {
		t.Fatalf("ASCII-only line was modified: %q -> %q", in, got)
	}
}

func TestNormalizeUnicode_PreservesNonTargetUnicode(t *testing.T) {
	in := "naïve café — résumé\n"
	want := "naïve café -- résumé\n"
	if got := runNormalizeUnicode(t, in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
