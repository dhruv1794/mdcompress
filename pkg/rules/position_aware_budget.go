package rules

// PositionAwareBudget truncates fenced code blocks that sit in the middle of a
// long document harder than the global truncator does. The premise is the
// "lost in the middle" effect: a reader (or an LLM consuming the compressed
// doc) leans on the head and tail of a long document, so a long code block
// buried in the middle is the cheapest place to spend a tighter line budget.
//
// It runs after truncate-large-code-blocks, so it only sees blocks already
// capped at the global limit and further tightens the middle band. Opt-in
// (DefaultDisabled): it is genuinely lossy on code, so it ships behind an
// explicit enable.
type PositionAwareBudget struct{}

func (r *PositionAwareBudget) Name() string { return "position-aware-budget" }
func (r *PositionAwareBudget) Tier() Tier   { return TierAggressive }

const (
	// pabMinDocLines is the length-aware gate: only documents at least this
	// long have a meaningful "middle" worth budgeting differently.
	pabMinDocLines = 200
	// pabMiddleMaxLines is the tight per-block line cap applied inside the
	// middle band — well below the global truncate-large-code-blocks cap.
	pabMiddleMaxLines = 8
)

func (r *PositionAwareBudget) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	if len(lines) < pabMinDocLines {
		return ChangeSet{}, nil
	}

	// Middle band is the inner half of the document: the first and last
	// quarters (head and tail) are left to the global truncator alone.
	bandLo := len(lines) / 4
	bandHi := len(lines) * 3 / 4

	var changes ChangeSet
	for _, block := range fencedBlocks(lines) {
		if block.StartLine < bandLo || block.EndLine >= bandHi {
			continue // not fully inside the middle band
		}
		truncateBlockLines(&changes, block.Content, pabMiddleMaxLines)
	}
	return changes, nil
}
