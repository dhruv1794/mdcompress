package rules_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestDedupCrossSectionStripsIntroSentenceWhenBodyHasLongerClaim(t *testing.T) {
	input := []byte("# Project\n\nProject stores cache files under `.project/cache`.\n\n## Cache\n\nProject stores cache files under `.project/cache` so repeated runs can reuse compressed mirrors across commands.\n")

	got := applyDedupCrossSection(t, input)
	want := []byte("# Project\n\n\n\n## Cache\n\nProject stores cache files under `.project/cache` so repeated runs can reuse compressed mirrors across commands.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDedupCrossSectionStripsStableFactCorpusShape(t *testing.T) {
	input := []byte("# Charlie\n\nStable fact: Charlie listens on port `4318` by default.\n\n## Server\n\nStable fact: Charlie listens on port `4318` by default and accepts OTLP traces from local collectors.\n")

	got := applyDedupCrossSection(t, input)
	want := []byte("# Charlie\n\n\n\n## Server\n\nStable fact: Charlie listens on port `4318` by default and accepts OTLP traces from local collectors.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDedupCrossSectionKeepsIntroWhenBodyIsNotMoreDetailed(t *testing.T) {
	input := []byte("# Project\n\nProject stores cache files under `.project/cache`.\n\n## Cache\n\nProject stores cache files under `.project/cache`.\n")

	got := applyDedupCrossSection(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestDedupCrossSectionKeepsDistinctClaims(t *testing.T) {
	input := []byte("# Project\n\nProject stores cache files under `.project/cache`.\n\n## Runtime\n\nProject reads configuration from `.project/config.yaml` before running compression.\n")

	got := applyDedupCrossSection(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestDedupCrossSectionKeepsClaimsWithMissingSpecialTokens(t *testing.T) {
	input := []byte("# Project\n\nProject listens on port `4318` by default.\n\n## Server\n\nProject listens on a configurable port by default when the server starts.\n")

	got := applyDedupCrossSection(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestDedupCrossSectionSkipsLargeFilesQuickly(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Big\n\nIntro paragraph claim about token budgets.\n\n## Body\n\n")
	line := "Sentence about token budgets and aggregate compression behavior across many files. "
	for b.Len() < 512*1024 {
		b.WriteString(line)
	}
	input := []byte(b.String())

	start := time.Now()
	got := applyDedupCrossSection(t, input)
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("dedup-cross-section took %s on a large input; size guard not effective", elapsed)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("expected oversized input to be returned unchanged")
	}
}

// Regression: a paragraph line containing inline-code with `|` (e.g. the
// regex character class below) used to wedge dedupParagraphs into an infinite
// loop, ballooning memory until the process was OOM-killed.
func TestDedupCrossSectionTerminatesWithInlineCodePipe(t *testing.T) {
	input := []byte("# Doc\n\nIntro paragraph mentions tokens.\n\n## Body\n\n" +
		"While the first part up to the first `|` contains a list of single control characters at which the string should be split, the rest is irrelevant.\n\n" +
		"Body sentence about tokens with extra detail and concrete numbers like 4318 and 5318 and additional context for the reader.\n")

	done := make(chan struct{})
	go func() {
		applyDedupCrossSection(t, input)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dedup-cross-section did not terminate on a paragraph with inline-code |")
	}
}

func BenchmarkDedupCrossSectionMediumDoc(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("# Doc\n\nIntro: project uses port `4318` by default for OTLP traces.\n\n## Body\n\n")
	for i := range 200 {
		sb.WriteString("Body sentence describing detail ")
		sb.WriteString(strings.Repeat("a ", i%30+5))
		sb.WriteString(" with reference to port `4318` and OTLP traces and other context.\n\n")
	}
	input := []byte(sb.String())
	rule := &rules.DedupCrossSection{}
	ctx := &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}}
	b.ResetTimer()
	for range b.N {
		if _, err := rule.Apply(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func applyDedupCrossSection(t *testing.T, input []byte) []byte {
	t.Helper()
	rule := &rules.DedupCrossSection{}
	changes, err := rule.Apply(&rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.ApplyEdits(input, changes.Edits)
}
