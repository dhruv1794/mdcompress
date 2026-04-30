package rules

import (
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type SetextHeaders struct{}

func (r *SetextHeaders) Name() string { return "strip-setext-headers" }
func (r *SetextHeaders) Tier() Tier   { return TierSafe }

func (r *SetextHeaders) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for index := 1; index < len(lines); index++ {
		prev := lines[index-1]
		curr := lines[index]

		if prev.InFence || curr.InFence {
			continue
		}
		if strings.TrimSpace(prev.Text) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(prev.Text), "#") || strings.HasPrefix(strings.TrimSpace(prev.Text), "|") || strings.HasPrefix(strings.TrimSpace(prev.Text), ">") {
			continue
		}

		isH1 := isSetextLevel1(curr.Text)
		isH2 := isSetextLevel2(curr.Text)
		if !isH1 && !isH2 {
			continue
		}

		prefix := "# "
		if isH2 {
			prefix = "## "
		}

		headingText := strings.TrimSpace(prev.Text)
		replaceStart := prev.Start
		replaceEnd := curr.End
		replacement := prefix + headingText

		addRange(&changes, render.Range{Start: replaceStart, End: replaceEnd})
		changes.Edits[len(changes.Edits)-1].Replacement = []byte(replacement)
		changes.Stats.BytesSaved -= len(replacement)
	}

	return changes, nil
}

func isSetextLevel1(text string) bool {
	trimmed := strings.TrimRight(text, " \t\r\n")
	if len(trimmed) < 1 {
		return false
	}
	for _, value := range trimmed {
		if value != '=' {
			return false
		}
	}
	return true
}

func isSetextLevel2(text string) bool {
	trimmed := strings.TrimRight(text, " \t\r\n")
	if len(trimmed) < 1 {
		return false
	}
	for _, value := range trimmed {
		if value != '-' {
			return false
		}
	}
	return true
}
