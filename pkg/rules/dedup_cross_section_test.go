package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
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

func applyDedupCrossSection(t *testing.T, input []byte) []byte {
	t.Helper()
	doc, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	rule := &rules.DedupCrossSection{}
	changes, err := rule.Apply(doc, &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.ApplyEdits(input, changes.Edits)
}
