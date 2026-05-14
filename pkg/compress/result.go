package compress

// Result contains compressed markdown and before/after metrics.
type Result struct {
	Output       []byte
	TokensBefore int
	TokensAfter  int
	BytesBefore  int
	BytesAfter   int
	RulesFired   map[string]int
	// RuleDurationsMS records wall-clock time per rule invocation in
	// milliseconds. Includes rules that fired with no edits, so it is the
	// canonical signal for cost-vs-savings analysis.
	RuleDurationsMS map[string]int64
	RuleBytesSaved  map[string]int
	// RuleTokensSaved is populated only when MDCOMPRESS_PROFILE_TOKENS=1 is
	// set in the environment (or compress.Options.ProfileTokens=true). The
	// fast path leaves it nil to avoid the per-rule tokenizer cost.
	RuleTokensSaved map[string]int
	// RuleErrors records rules and plugins that returned an error during
	// compression. The compression continues without that rule's edits, but
	// callers can surface the failure (verbose run, doctor, MCP).
	RuleErrors map[string]string
	// LLM is populated when Tier == TierLLM and an LLMRewriter ran. A zero
	// value means Tier-3 was not invoked.
	LLM LLMRewriteStats
}

// TokensSaved returns the before/after token delta.
func (r Result) TokensSaved() int {
	return r.TokensBefore - r.TokensAfter
}

// BytesSaved returns the before/after byte delta.
func (r Result) BytesSaved() int {
	return r.BytesBefore - r.BytesAfter
}
