package rules

import (
	"strconv"
)

const (
	DefaultCodeBlockMaxLines = 20
	DefaultCodeBlockMaxBytes = 2048
)

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
		contentStart := block.Content[0].Start
		contentEnd := block.Content[len(block.Content)-1].End
		contentBytes := contentEnd - contentStart

		if len(block.Content) > maxLines {
			omitted := len(block.Content) - maxLines
			start := block.Content[maxLines].Start
			end := contentEnd
			replacement := "[... " + strconv.Itoa(omitted) + " more lines ...]\n"
			if len(replacement) < end-start {
				addReplacement(&changes, start, end, replacement)
			}
			continue
		}

		if contentBytes > DefaultCodeBlockMaxBytes && len(block.Content) <= maxLines {
			keep := max(DefaultCodeBlockMaxBytes/2, 200)
			if keep >= contentBytes {
				continue
			}
			cut := contentStart + keep
			for cut < contentEnd && ctx.Source[cut] != '\n' {
				cut++
			}
			if cut >= contentEnd {
				continue
			}
			omitted := contentEnd - cut
			replacement := "[... " + strconv.Itoa(omitted) + " more bytes ...]\n"
			if len(replacement) < contentEnd-cut {
				addReplacement(&changes, cut, contentEnd, replacement)
			}
		}
	}
	return changes, nil
}
