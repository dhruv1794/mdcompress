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

func TestApplyEditsReplacesRange(t *testing.T) {
	src := []byte("Run in order to test.")
	got := render.ApplyEdits(src, []render.Edit{{Start: 4, End: 15, Replacement: []byte("to")}})
	want := []byte("Run to test.")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEditsAdjacentReplacements(t *testing.T) {
	// Two replacements sharing a boundary (one starts where the other ends)
	// must both apply. The previous `<=` overlap check silently dropped the
	// second.
	src := []byte("AAAABBBB")
	got := render.ApplyEdits(src, []render.Edit{
		{Start: 0, End: 4, Replacement: []byte("xx")},
		{Start: 4, End: 8, Replacement: []byte("yy")},
	})
	want := []byte("xxyy")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEditsPureInsert(t *testing.T) {
	// A pure-insert edit (Start==End, non-empty Replacement) inserts before
	// the byte at Start without consuming any source.
	src := []byte("hello world")
	got := render.ApplyEdits(src, []render.Edit{
		{Start: 6, End: 6, Replacement: []byte("brave ")},
	})
	want := []byte("hello brave world")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEditsInsertBeforeReplacementAtSameOffset(t *testing.T) {
	// A pure insert at offset X and a replacement [X, Y) both apply; the
	// insert text lands before the replacement output.
	src := []byte("ABCDE")
	got := render.ApplyEdits(src, []render.Edit{
		{Start: 0, End: 0, Replacement: []byte("[")},
		{Start: 0, End: 3, Replacement: []byte("xxx")},
	})
	want := []byte("[xxxDE")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyEditsMergesAdjacentDeletions(t *testing.T) {
	// Adjacent pure deletions should still produce the same end result as
	// merging them — both forms drop the byte range [0, 8).
	src := []byte("AAAABBBBC")
	got := render.ApplyEdits(src, []render.Edit{
		{Start: 0, End: 4},
		{Start: 4, End: 8},
	})
	want := []byte("C")
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
