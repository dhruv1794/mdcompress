package compress_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestCompressStripsHTMLComments(t *testing.T) {
	input := []byte("# Project\n\n<!-- remove me -->\n\nUseful text.\n")

	result, err := compress.Compress(input, compress.Options{Tier: compress.TierSafe})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	want := []byte("# Project\n\nUseful text.\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	if result.RulesFired["strip-html-comments"] != 1 {
		t.Fatalf("strip-html-comments fired %d times", result.RulesFired["strip-html-comments"])
	}
}

func TestCompressReparsesASTAfterEarlierEdits(t *testing.T) {
	input := []byte("---\ntitle: Project\n---\n# Project\n\n<!-- remove me -->\n\nUseful text.\n")

	result, err := compress.Compress(input, compress.Options{Tier: compress.TierSafe})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	want := []byte("# Project\n\nUseful text.\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	if result.RulesFired["strip-frontmatter"] != 1 {
		t.Fatalf("strip-frontmatter fired %d times", result.RulesFired["strip-frontmatter"])
	}
	if result.RulesFired["strip-html-comments"] != 1 {
		t.Fatalf("strip-html-comments fired %d times", result.RulesFired["strip-html-comments"])
	}
}

func TestCompressCanDisableRule(t *testing.T) {
	input := []byte("# Project\n\n<!-- keep me -->\n\nUseful text.\n")

	result, err := compress.Compress(input, compress.Options{
		Tier:          compress.TierSafe,
		DisabledRules: []string{"strip-html-comments"},
	})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if !bytes.Equal(result.Output, input) {
		t.Fatalf("output = %q, want %q", result.Output, input)
	}
}

func TestCompressDedupsCrossFileCodeBlocks(t *testing.T) {
	crossFile := &rules.CrossFileState{}
	first := []byte("# Guide\n\n```go\nclient := NewClient(config)\nresult, err := client.Do(ctx, request)\nif err != nil {\n    return err\n}\nreturn result\n```\n")
	second := []byte("# Tutorial\n\n```go\nclient := NewClient(config)\nresult, err := client.Do(ctx, request)\nif err != nil {\n    return err\n}\nreturn result\n```\n")

	firstResult, err := compress.Compress(first, compress.Options{
		Tier:      compress.TierAggressive,
		FilePath:  "docs/guide.md",
		CrossFile: crossFile,
	})
	if err != nil {
		t.Fatalf("first Compress() error = %v", err)
	}
	if firstResult.RulesFired["dedup-cross-file-code-blocks"] != 0 {
		t.Fatalf("canonical file should not be deduped: %q", firstResult.Output)
	}

	secondResult, err := compress.Compress(second, compress.Options{
		Tier:      compress.TierAggressive,
		FilePath:  "docs/tutorial.md",
		CrossFile: crossFile,
	})
	if err != nil {
		t.Fatalf("second Compress() error = %v", err)
	}

	want := []byte("# Tutorial\n\n```go\n[same as guide.md:3]\n```\n")
	if !bytes.Equal(secondResult.Output, want) {
		t.Fatalf("output = %q, want %q", secondResult.Output, want)
	}
	if secondResult.RulesFired["dedup-cross-file-code-blocks"] != 1 {
		t.Fatalf("dedup-cross-file-code-blocks fired %d times", secondResult.RulesFired["dedup-cross-file-code-blocks"])
	}
}

func TestCompressDedupsExactCrossFileSectionsUnderAnyHeading(t *testing.T) {
	crossFile := &rules.CrossFileState{}
	body := "The client reads configuration from the project config file, creates a local cache directory, validates token settings, and writes a manifest entry after each successful compression run.\n"
	first := []byte("# Guide\n\n## Runtime Behavior\n\n" + body)
	second := []byte("# Tutorial\n\n## How It Works\n\n" + body)

	if _, err := compress.Compress(first, compress.Options{
		Tier:      compress.TierAggressive,
		FilePath:  "docs/guide.md",
		CrossFile: crossFile,
	}); err != nil {
		t.Fatalf("first Compress() error = %v", err)
	}
	secondResult, err := compress.Compress(second, compress.Options{
		Tier:      compress.TierAggressive,
		FilePath:  "docs/tutorial.md",
		CrossFile: crossFile,
	})
	if err != nil {
		t.Fatalf("second Compress() error = %v", err)
	}

	want := []byte("# Tutorial\n\n## How It Works\n[same as in guide.md]\n")
	if !bytes.Equal(secondResult.Output, want) {
		t.Fatalf("output = %q, want %q", secondResult.Output, want)
	}
	if secondResult.RulesFired["strip-cross-file-dupes"] != 1 {
		t.Fatalf("strip-cross-file-dupes fired %d times", secondResult.RulesFired["strip-cross-file-dupes"])
	}
}

func TestCompressTruncatesLargeCodeBlocks(t *testing.T) {
	input := "# Log\n\n```text\n" + strings.Join([]string{
		"line 01: keep this setup line",
		"line 02: keep this setup line",
		"line 03: keep this setup line",
		"line 04: omit this verbose generated output line",
		"line 05: omit this verbose generated output line",
	}, "\n") + "\n```\n"

	result, err := compress.Compress([]byte(input), compress.Options{
		Tier:              compress.TierAggressive,
		CodeBlockMaxLines: 3,
	})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}

	want := []byte("# Log\n\n```text\nline 01: keep this setup line\nline 02: keep this setup line\nline 03: keep this setup line\n[... 2 more lines ...]\n```\n")
	if !bytes.Equal(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
	if result.RulesFired["truncate-large-code-blocks"] != 1 {
		t.Fatalf("truncate-large-code-blocks fired %d times", result.RulesFired["truncate-large-code-blocks"])
	}
}

func TestCompressEmptyFencedBlockDoesNotPanic(t *testing.T) {
	input := []byte("# Doc\n\n```\n```\n\n```text\n```\n\nafter\n")
	if _, err := compress.Compress(input, compress.Options{Tier: compress.TierAggressive}); err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
}
