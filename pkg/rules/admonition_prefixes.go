package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type AdmonitionPrefixes struct{}

func (r *AdmonitionPrefixes) Name() string { return "strip-admonition-prefixes" }
func (r *AdmonitionPrefixes) Tier() Tier   { return TierAggressive }

var admonitionPrefixPattern = regexp.MustCompile(`(?i)^(\*{1,2}|_{1,2})\s*((?:[⚠️💡📖⚡🔍💥🔥✅❌🚧🚨🎉💬📣📢🤖🔧])?\s*)?(NOTE|WARNING|IMPORTANT|TIP|INFO|CAUTION|DANGER|HINT|REMINDER)\s*:?\s*(\*{1,2}|_{1,2})?\s*`)

func (r *AdmonitionPrefixes) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" || strings.HasPrefix(trimmed, "|") {
			continue
		}

		inner, _ := stripBlockquotePrefix(trimmed)
		loc := admonitionPrefixPattern.FindStringIndex(inner)
		if loc == nil || loc[0] != 0 {
			continue
		}

		matchLen := loc[1]
		if loc[1] < len(inner) && inner[loc[1]] == ' ' {
			matchLen++
		}

		prefixLen := len(line.Text) - len(inner) + matchLen
		start := line.Start + (len(line.Text) - len(trimmed))

		addReplacement(&changes, start, start+prefixLen, "")
	}
	return changes, nil
}

func stripBlockquotePrefix(text string) (string, int) {
	depth := 0
	rest := text
	for strings.HasPrefix(rest, ">") {
		rest = strings.TrimPrefix(rest, ">")
		depth++
	}
	for strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}
	for strings.HasPrefix(rest, ">") {
		rest = strings.TrimPrefix(rest, ">")
		depth++
		for strings.HasPrefix(rest, " ") {
			rest = rest[1:]
		}
	}
	return rest, depth
}
