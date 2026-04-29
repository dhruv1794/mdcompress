package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestExampleOutputStripsHelpOutputAfterCommandLine(t *testing.T) {
	input := []byte("# Project\n\nRun `mdcompress --help`:\n\n```text\nUsage: mdcompress [command]\n\nFlags:\n  -h, --help      help for mdcompress\n      --version   version for mdcompress\n```\n\n## Usage\n\nRun it.\n")

	got := applyExampleOutput(t, input)
	want := []byte("# Project\n\nRun `mdcompress --help`:\n\n\n## Usage\n\nRun it.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExampleOutputStripsHelpOutputAfterCommandFence(t *testing.T) {
	input := []byte("# Project\n\n```sh\nmdcompress run --help\n```\n\n```text\nUsage: mdcompress run [path...]\n\nFlags:\n  --all       rebuild all files\n  --staged    read staged files\n```\n")

	got := applyExampleOutput(t, input)
	want := []byte("# Project\n\n```sh\nmdcompress run --help\n```\n\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExampleOutputStripsShortVersionOutput(t *testing.T) {
	input := []byte("# Project\n\nRun `mdcompress --version`:\n\n```text\nmdcompress v0.1.0\n```\n")

	got := applyExampleOutput(t, input)
	want := []byte("# Project\n\nRun `mdcompress --version`:\n\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExampleOutputKeepsSubstantiveCodeBlock(t *testing.T) {
	input := []byte("# Project\n\nRun `mdcompress --help`:\n\n```text\nThis example explains how cache mirrors are generated after markdown files change.\nIt includes operational context that is not present in the command itself.\n```\n")

	got := applyExampleOutput(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestExampleOutputKeepsLongHelpOutput(t *testing.T) {
	var input []byte
	input = append(input, []byte("# Project\n\nRun `mdcompress --help`:\n\n```text\nUsage: mdcompress\n")...)
	for i := 0; i < 51; i++ {
		input = append(input, []byte("  --flag     value\n")...)
	}
	input = append(input, []byte("```\n")...)

	got := applyExampleOutput(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func applyExampleOutput(t *testing.T, input []byte) []byte {
	t.Helper()
	doc, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	rule := &rules.ExampleOutput{}
	changes, err := rule.Apply(doc, &rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierAggressive}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.ApplyEdits(input, changes.Edits)
}
