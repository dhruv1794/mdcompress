package llm

import "github.com/dhruv1794/mdcompress/pkg/compress"

// CompressAdapter wraps a Rewriter so it satisfies compress.LLMRewriter
// without forcing pkg/compress to import this package.
type CompressAdapter struct {
	Rewriter *Rewriter
}

// NewCompressAdapter returns a CompressAdapter for the given Rewriter.
func NewCompressAdapter(r *Rewriter) *CompressAdapter {
	return &CompressAdapter{Rewriter: r}
}

// Rewrite runs the underlying rewriter and converts its stats into the
// compress.LLMRewriteStats shape.
func (a *CompressAdapter) Rewrite(source []byte) ([]byte, compress.LLMRewriteStats, error) {
	if a == nil || a.Rewriter == nil {
		return source, compress.LLMRewriteStats{}, nil
	}
	out, stats, err := a.Rewriter.Rewrite(source)
	return out, compress.LLMRewriteStats{
		SectionsConsidered: stats.SectionsConsidered,
		SectionsRewritten:  stats.SectionsRewritten,
		SectionsSkipped:    stats.SectionsSkipped,
		SectionsFailed:     stats.SectionsFailed,
		TokensSaved:        stats.TokensSaved,
		CacheHits:          stats.CacheHits,
		CacheMisses:        stats.CacheMisses,
	}, err
}
