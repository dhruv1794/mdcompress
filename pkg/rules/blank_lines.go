package rules

import (
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type BlankLines struct{}

func (r *BlankLines) Name() string { return "collapse-blank-lines" }
func (r *BlankLines) Tier() Tier   { return TierSafe }

func (r *BlankLines) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	blankRun := 0
	seenContent := false

	for _, line := range lines {
		if line.InFence {
			blankRun = 0
			seenContent = true
			continue
		}
		if strings.TrimSpace(line.Text) == "" {
			if !seenContent {
				addRange(&changes, fullLineRange(line))
				continue
			}
			blankRun++
			if blankRun > 1 {
				addRange(&changes, fullLineRange(line))
			}
			continue
		}
		seenContent = true
		blankRun = 0
	}

	for lineIndex := len(lines) - 1; lineIndex >= 0; lineIndex-- {
		if strings.TrimSpace(lines[lineIndex].Text) != "" {
			break
		}
		addRange(&changes, render.Range{Start: lines[lineIndex].Start, End: lines[lineIndex].End})
	}

	return changes, nil
}
