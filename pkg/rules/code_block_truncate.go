package rules

import (
	"strconv"
)

const DefaultCodeBlockMaxLines = 30

type CodeBlockTruncate struct{}

func (r *CodeBlockTruncate) Name() string { return "truncate-large-code-blocks" }
func (r *CodeBlockTruncate) Tier() Tier   { return TierAggressive }

func (r *CodeBlockTruncate) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	blocks := fencedBlocks(lines)
	if len(blocks) == 0 {
		return ChangeSet{}, nil
	}

	maxLines := DefaultCodeBlockMaxLines
	if ctx.Config != nil && ctx.Config.CodeBlockMaxLines > 0 {
		maxLines = ctx.Config.CodeBlockMaxLines
	}

	var changes ChangeSet
	for _, block := range blocks {
		if len(block.Content) <= maxLines {
			continue
		}

		omitted := len(block.Content) - maxLines
		start := block.Content[maxLines].Start
		end := block.Content[len(block.Content)-1].End
		replacement := "[... " + strconv.Itoa(omitted) + " more lines ...]\n"
		if len(replacement) >= end-start {
			continue
		}
		addReplacement(&changes, start, end, replacement)
	}
	return changes, nil
}
