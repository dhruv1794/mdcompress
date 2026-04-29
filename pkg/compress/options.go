package compress

import (
	"fmt"
	"strings"
)

// Tier selects the rule set to run.
type Tier int

const (
	TierSafe Tier = iota + 1
	TierAggressive
	TierLLM
)

func (t Tier) String() string {
	switch t {
	case TierSafe:
		return "safe"
	case TierAggressive:
		return "aggressive"
	case TierLLM:
		return "llm"
	default:
		return "unknown"
	}
}

// ParseTier parses a CLI/config tier name.
func ParseTier(value string) (Tier, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "safe":
		return TierSafe, nil
	case "aggressive":
		return TierAggressive, nil
	case "llm":
		return TierLLM, nil
	default:
		return 0, fmt.Errorf("unknown tier %q", value)
	}
}

// LLMRewriter is the contract Tier-3 callers implement. It walks the
// (already Tier-2-compressed) source and returns a further-compressed copy
// where qualifying prose sections have been rewritten.
//
// The interface lives here, not in pkg/llm, so pkg/compress does not depend
// on the llm package. Callers that want Tier-3 build a *llm.Rewriter and
// pass it via Options.LLMRewriter.
type LLMRewriter interface {
	Rewrite(source []byte) ([]byte, LLMRewriteStats, error)
}

// LLMRewriteStats reports per-section activity from a Tier-3 run.
type LLMRewriteStats struct {
	SectionsConsidered int
	SectionsRewritten  int
	SectionsSkipped    int
	SectionsFailed     int
	TokensSaved        int
	CacheHits          int
	CacheMisses        int
}

// Options controls a compression run.
type Options struct {
	Tier          Tier
	EnabledRules  []string
	DisabledRules []string
	// LLMRewriter, when non-nil and Tier == TierLLM, runs after the rule
	// pipeline. A nil rewriter at TierLLM degrades to Tier-2 output.
	LLMRewriter LLMRewriter
}
