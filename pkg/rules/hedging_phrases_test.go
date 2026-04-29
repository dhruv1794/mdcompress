package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestHedgingPhrasesReplacesExactPatterns(t *testing.T) {
	input := []byte("# Project\n\nPlease note that the cache is local. Use hooks in order to refresh docs due to the fact that agents read markdown.\n")

	got := applyHedgingPhrases(t, input)
	want := []byte("# Project\n\nThe cache is local. Use hooks to refresh docs because agents read markdown.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHedgingPhrasesHandlesSentenceStartReplacement(t *testing.T) {
	input := []byte("# Project\n\nIn the event that cache files are stale, run mdcompress. At this point in time, status reports totals.\n")

	got := applyHedgingPhrases(t, input)
	want := []byte("# Project\n\nIf cache files are stale, run mdcompress. Now, status reports totals.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHedgingPhrasesSkipsCodeAndTables(t *testing.T) {
	input := []byte("# Project\n\nUse `in order to` as a literal.\n\n```md\nplease note that code stays unchanged\n```\n\n| phrase | meaning |\n| --- | --- |\n| in order to | literal |\n")

	got := applyHedgingPhrases(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func applyHedgingPhrases(t *testing.T, input []byte) []byte {
	t.Helper()
	doc, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	rule := &rules.HedgingPhrases{}
	changes, err := rule.Apply(doc, &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.ApplyEdits(input, changes.Edits)
}
