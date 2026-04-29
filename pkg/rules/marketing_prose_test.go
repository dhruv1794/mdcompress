package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestMarketingProseStripsIntroPhrases(t *testing.T) {
	input := []byte("# Project\n\nA production-ready, feature-rich Go library for processing markdown.\nIt is a delightful CLI for agents.\n\n## Usage\n\nRun it.\n")

	got := applyMarketingProse(t, input)
	want := []byte("# Project\n\nA Go library for processing markdown.\nIt is a CLI for agents.\n\n## Usage\n\nRun it.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarketingProseStripsFeatureListPhrases(t *testing.T) {
	input := []byte("# Project\n\n## Features\n\n- Blazing fast parsing\n- Developer-friendly config\n\n## Benchmarks\n\nRock-solid uptime under load.\n")

	got := applyMarketingProse(t, input)
	want := []byte("# Project\n\n## Features\n\n- parsing\n- config\n\n## Benchmarks\n\nRock-solid uptime under load.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMarketingProseSkipsCodeAndTechnicalContext(t *testing.T) {
	input := []byte("# Project\n\nUse `production-ready` as a label.\n\n```md\nA blazing fast parser.\n```\n\n## Features\n\n- Highly performant p99 latency under load\n")

	got := applyMarketingProse(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func applyMarketingProse(t *testing.T, input []byte) []byte {
	t.Helper()
	doc, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	rule := &rules.MarketingProse{}
	changes, err := rule.Apply(doc, &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.Splice(input, changes.Ranges)
}
