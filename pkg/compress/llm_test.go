package compress_test

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

type fakeRewriter struct {
	called bool
	out    []byte
	stats  compress.LLMRewriteStats
}

func (f *fakeRewriter) Rewrite(source []byte) ([]byte, compress.LLMRewriteStats, error) {
	f.called = true
	if f.out != nil {
		return f.out, f.stats, nil
	}
	return source, f.stats, nil
}

func TestCompressInvokesLLMRewriterAtTier3(t *testing.T) {
	input := []byte("# Project\n\nUseful text.\n")
	rewritten := []byte("# Project\n\nText.\n")

	rewriter := &fakeRewriter{
		out: rewritten,
		stats: compress.LLMRewriteStats{
			SectionsConsidered: 1,
			SectionsRewritten:  1,
			TokensSaved:        2,
		},
	}

	result, err := compress.Compress(input, compress.Options{
		Tier:        compress.TierLLM,
		LLMRewriter: rewriter,
	})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if !rewriter.called {
		t.Fatalf("expected LLMRewriter to be invoked at TierLLM")
	}
	if string(result.Output) != string(rewritten) {
		t.Fatalf("expected rewritten output, got %q", string(result.Output))
	}
	if result.LLM.SectionsRewritten != 1 {
		t.Fatalf("expected stats to propagate, got %+v", result.LLM)
	}
}

func TestCompressSkipsRewriterBelowTier3(t *testing.T) {
	rewriter := &fakeRewriter{out: []byte("should not be used")}

	_, err := compress.Compress([]byte("# x\n\nbody\n"), compress.Options{
		Tier:        compress.TierAggressive,
		LLMRewriter: rewriter,
	})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if rewriter.called {
		t.Fatalf("LLMRewriter must not be invoked when Tier < TierLLM")
	}
}

func TestCompressTier3WithoutRewriterDegrades(t *testing.T) {
	input := []byte("# Project\n\n<!-- comment -->\n\nBody.\n")
	result, err := compress.Compress(input, compress.Options{Tier: compress.TierLLM})
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if strings.Contains(string(result.Output), "comment") {
		t.Fatalf("Tier-3 without rewriter should still apply lower-tier rules: %q", string(result.Output))
	}
}
