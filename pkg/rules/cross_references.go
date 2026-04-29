package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type CrossReferences struct{}

func (r *CrossReferences) Name() string { return "strip-cross-references" }
func (r *CrossReferences) Tier() Tier   { return TierAggressive }

var crossRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)see the \[[^\]]+\]\(?[^\)]*\)?\s*(section|guide|page|doc|document|chapter|tutorial|reference|manual|below|above)?[\.;]?`),
	regexp.MustCompile(`(?i)for more (information|details|info),? see (the )?\[[^\]]+\]\(?[^\)]*\)?[\.;]?`),
	regexp.MustCompile(`(?i)refer to (the )?\[[^\]]+\]\(?[^\)]*\)?( for (more |further )?(information|details|guidelines?))?[\.;]?`),
	regexp.MustCompile(`(?i)check out (the )?\[[^\]]+\]\(?[^\)]*\)?( for (more |further )?(information|details))?[\.;]?`),
	regexp.MustCompile(`(?i)read (more |further )?(about it )?in (the )?\[[^\]]+\]\(?[^\)]*\)?[\.;]?`),
	regexp.MustCompile(`(?i)see also\s*:?\s*\[[^\]]+\]\(?[^\)]*\)?[\.;]?`),
	regexp.MustCompile(`(?i)(please |kindly )?(refer|see|check|consult) \[[^\]]+\]\(?[^\)]*\)?( for (more |further )?(information|details|guidance))?\s*[\.;]?`),
	regexp.MustCompile(`(?i)(head|go|navigate) (over |back )?to (the )?\[[^\]]+\]\(?[^\)]*\)?\s*(section|page|doc|guide)?[\.;]?`),
	regexp.MustCompile(`(?i)more (details|information) (can be found|are available) (in|at) \[[^\]]+\]\(?[^\)]*\)?[\.;]?`),
	regexp.MustCompile(`(?i)(please |feel free to )?(visit|check) (out )?\[[^\]]+\]\(?[^\)]*\)?( for (more |further )?(information|details))?[\.;]?`),
	regexp.MustCompile(`(?i)(continue |keep )?reading (in|at) \[[^\]]+\]\(?[^\)]*\)?[\.;]?`),
}

func (r *CrossReferences) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, ">") {
			continue
		}
		if !isCrossRefLine(trimmed) {
			continue
		}
		for _, phrase := range crossRefPatterns {
			for _, match := range phrase.FindAllStringIndex(line.Text, -1) {
				rng := crossRefRemovalRange(line.Text, match[0], match[1])
				addRange(&changes, render.Range{
					Start: line.Start + rng.Start,
					End:   line.Start + rng.End,
				})
			}
		}
	}
	return changes, nil
}

func isCrossRefLine(trimmed string) bool {
	return strings.Contains(strings.ToLower(trimmed), "see") ||
		strings.Contains(strings.ToLower(trimmed), "refer") ||
		strings.Contains(strings.ToLower(trimmed), "check") ||
		strings.Contains(strings.ToLower(trimmed), "read") ||
		strings.Contains(strings.ToLower(trimmed), "visit") ||
		strings.Contains(strings.ToLower(trimmed), "navigate") ||
		strings.Contains(strings.ToLower(trimmed), "head") ||
		strings.Contains(strings.ToLower(trimmed), "more detail") ||
		strings.Contains(strings.ToLower(trimmed), "more information")
}

func crossRefRemovalRange(text string, start, end int) render.Range {
	spaceBefore := start
	for spaceBefore > 0 && text[spaceBefore-1] == ' ' {
		spaceBefore--
	}
	commaBefore := spaceBefore
	if commaBefore > 0 && text[commaBefore-1] == ',' {
		commaBefore--
		for commaBefore > 0 && text[commaBefore-1] == ' ' {
			commaBefore--
		}
		return render.Range{Start: commaBefore, End: end}
	}
	return render.Range{Start: spaceBefore, End: end}
}
