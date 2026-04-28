package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
)

func TestSpliceNoRangesIsIdentity(t *testing.T) {
	src := []byte("# Hello\n\nWorld.\n")
	if got := render.Splice(src, nil); !bytes.Equal(got, src) {
		t.Fatalf("identity mismatch: %q vs %q", got, src)
	}
	if got := render.Splice(src, []render.Range{}); !bytes.Equal(got, src) {
		t.Fatalf("identity mismatch (empty slice): %q vs %q", got, src)
	}
}

func TestSpliceRemovesSingleRange(t *testing.T) {
	src := []byte("AAAxxxBBB")
	got := render.Splice(src, []render.Range{{Start: 3, End: 6}})
	want := []byte("AAABBB")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSpliceMergesOverlappingRanges(t *testing.T) {
	src := []byte("0123456789")
	got := render.Splice(src, []render.Range{
		{Start: 2, End: 5},
		{Start: 4, End: 7},
	})
	want := []byte("01789")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSpliceUnsortedRanges(t *testing.T) {
	src := []byte("0123456789")
	got := render.Splice(src, []render.Range{
		{Start: 7, End: 9},
		{Start: 1, End: 3},
	})
	want := []byte("034569")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSpliceClampsOutOfRange(t *testing.T) {
	src := []byte("hello")
	got := render.Splice(src, []render.Range{{Start: -5, End: 2}})
	want := []byte("llo")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	got = render.Splice(src, []render.Range{{Start: 3, End: 999}})
	want = []byte("hel")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestRoundtripCorpus exercises the parser on real documents and asserts
// that splicing with no ranges returns the source byte-for-byte.
func TestRoundtripCorpus(t *testing.T) {
	matches, err := filepath.Glob("../../internal/testdata/corpus/*.md")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Skip("no corpus files in internal/testdata/corpus")
	}
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if _, err := parser.Parse(src); err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := render.Splice(src, nil)
			if !bytes.Equal(got, src) {
				t.Fatalf("roundtrip mismatch for %s (len src=%d, len out=%d)", path, len(src), len(got))
			}
		})
	}
}

func TestRoundtripProjectDocs(t *testing.T) {
	paths := []string{
		"../../README.md",
		"../../../INDEX.md",
		"../../../ROADMAP.md",
		"../../../RULES.md",
		"../../../V1-SPEC.md",
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				t.Skipf("not present in this checkout: %s", path)
			}
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if _, err := parser.Parse(src); err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := render.Splice(src, nil)
			if !bytes.Equal(got, src) {
				t.Fatalf("roundtrip mismatch for %s (len src=%d, len out=%d)", path, len(src), len(got))
			}
		})
	}
}
