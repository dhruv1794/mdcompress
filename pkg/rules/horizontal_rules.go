package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type HorizontalRules struct{}

func (r *HorizontalRules) Name() string { return "strip-horizontal-rules" }
func (r *HorizontalRules) Tier() Tier   { return TierSafe }

var hrulePattern = regexp.MustCompile(`^\s*(?:[-]{3,}|[\*]{3,}|[_]{3,})\s*$`)

func (r *HorizontalRules) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for i, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || line.InFence {
			continue
		}

		if !hrulePattern.MatchString(trimmed) {
			continue
		}

		if i > 0 && strings.Contains(strings.TrimSpace(line.Text), "|") && isTableDelimiter(line.Text) {
			continue
		}

		if i == 0 && i+1 < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[i+1].Text)
			if nextTrimmed == "" {
				continue
			}
		}

		if i > 0 {
			prevTrimmed := strings.TrimSpace(lines[i-1].Text)
			if prevTrimmed != "" && stringIsOnly(prevTrimmed, "-= ") {
				continue
			}
		}

		addRange(&changes, fullLineRange(line))
	}

	return changes, nil
}

func stringIsOnly(s string, chars string) bool {
	for _, c := range s {
		if !strings.ContainsRune(chars, c) {
			return false
		}
	}
	return true
}
