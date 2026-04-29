package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestBenchmarkProseStripsParagraphAfterTable(t *testing.T) {
	input := []byte("# Project\n\n| Repo | Tokens | Reduction |\n| --- | ---: | ---: |\n| react | 4500 | 38% |\n| express | 2100 | 22% |\n\nThe benchmarks above show that react sees a 38% reduction while express sees 22%.\n\n## Usage\n\nRun it.\n")

	got := applyBenchmarkProse(t, input)
	want := []byte("# Project\n\n| Repo | Tokens | Reduction |\n| --- | ---: | ---: |\n| react | 4500 | 38% |\n| express | 2100 | 22% |\n\n## Usage\n\nRun it.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBenchmarkProseStripsParagraphBeforeTable(t *testing.T) {
	input := []byte("# Project\n\nReact drops from 4500 tokens to 38% less, and express drops from 2100 tokens to 22% less.\n\n| Repo | Tokens | Reduction |\n| --- | ---: | ---: |\n| react | 4500 | 38% |\n| express | 2100 | 22% |\n")

	got := applyBenchmarkProse(t, input)
	want := []byte("# Project\n\n| Repo | Tokens | Reduction |\n| --- | ---: | ---: |\n| react | 4500 | 38% |\n| express | 2100 | 22% |\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBenchmarkProseKeepsSubstantiveAdjacentParagraph(t *testing.T) {
	input := []byte("# Project\n\n| Repo | Tokens | Reduction |\n| --- | ---: | ---: |\n| react | 4500 | 38% |\n| express | 2100 | 22% |\n\nReact uses a custom compiler pipeline, while Express keeps a minimal routing layer for backwards compatibility.\n")

	got := applyBenchmarkProse(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestBenchmarkProseSkipsLongParagraphsAndFences(t *testing.T) {
	input := []byte("# Project\n\n```md\n| Repo | Tokens |\n| --- | ---: |\n| react | 4500 |\nThe benchmarks above show react at 4500 tokens.\n```\n\n| Repo | Tokens |\n| --- | ---: |\n| react | 4500 |\n\nThe first sentence mentions react. The second sentence mentions 4500. The third sentence repeats the table. The fourth sentence should keep this paragraph.\n")

	got := applyBenchmarkProse(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func applyBenchmarkProse(t *testing.T, input []byte) []byte {
	t.Helper()
	doc, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	rule := &rules.BenchmarkProse{}
	changes, err := rule.Apply(doc, &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.ApplyEdits(input, changes.Edits)
}
