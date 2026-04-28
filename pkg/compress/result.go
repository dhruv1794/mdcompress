package compress

// Result contains compressed markdown and before/after metrics.
type Result struct {
	Output       []byte
	TokensBefore int
	TokensAfter  int
	BytesBefore  int
	BytesAfter   int
	RulesFired   map[string]int
}

// TokensSaved returns the before/after token delta.
func (r Result) TokensSaved() int {
	return r.TokensBefore - r.TokensAfter
}

// BytesSaved returns the before/after byte delta.
func (r Result) BytesSaved() int {
	return r.BytesBefore - r.BytesAfter
}
