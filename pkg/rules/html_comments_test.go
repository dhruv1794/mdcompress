package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestHTMLCommentsStripsOwnLineComment(t *testing.T) {
	got := applyHTMLComments(t, []byte("# Title\n\n<!-- hidden -->\n\nText.\n"))
	want := []byte("# Title\n\nText.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHTMLCommentsStripsInlineCommentOnly(t *testing.T) {
	got := applyHTMLComments(t, []byte("Keep <!-- hidden --> text.\n"))
	want := []byte("Keep  text.\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHTMLCommentsSkipsFencedCodeBlocks(t *testing.T) {
	input := []byte("```md\n<!-- keep -->\n```\n\n<!-- remove -->\n")
	got := applyHTMLComments(t, input)
	want := []byte("```md\n<!-- keep -->\n```\n\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHTMLCommentsIgnoresBrokenComment(t *testing.T) {
	input := []byte("Text\n<!-- broken\n")
	got := applyHTMLComments(t, input)
	if !bytes.Equal(got, input) {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func applyHTMLComments(t *testing.T, input []byte) []byte {
	t.Helper()
	rule := &rules.HTMLComments{}
	changes, err := rule.Apply(&rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierSafe}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	return render.Splice(input, changes.Ranges)
}
