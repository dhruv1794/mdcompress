package compress_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
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
