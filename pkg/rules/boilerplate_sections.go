package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type BoilerplateSections struct{}

func (r *BoilerplateSections) Name() string { return "strip-boilerplate-sections" }
func (r *BoilerplateSections) Tier() Tier   { return TierAggressive }

var boilerplateHeadingPattern = regexp.MustCompile(`(?i)^(contributing|license|support|code of conduct|getting help|need help\??|questions\??|community|sponsors?|backers?|acknowledgements?|credits|thanks|maintainers?|authors?|copyright and license)$`)

func (r *BoilerplateSections) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for lineIndex, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" {
			continue
		}

		heading, ok := markdownHeadingText(trimmed)
		if !ok {
			continue
		}
		if !boilerplateHeadingPattern.MatchString(heading) {
			continue
		}

		sectionEnd := boilerplateSectionEnd(lines, lineIndex)
		if sectionEnd <= line.Start {
			continue
		}

		sectionText := string(ctx.Source[line.Start:sectionEnd])
		if !isShortBoilerplate(sectionText, heading) {
			continue
		}

		addRange(&changes, render.Range{Start: line.Start, End: sectionEnd})
	}
	return changes, nil
}

func boilerplateSectionEnd(lines []sourceLine, headingLine int) int {
	headingLevel := 0
	for _, ch := range lines[headingLine].Text {
		if ch == '#' {
			headingLevel++
		} else {
			break
		}
	}

	for index := headingLine + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index].Text)
		if trimmed == "" || lines[index].InFence {
			continue
		}
		if h, ok := markdownHeadingText(trimmed); ok {
			nextLevel := 0
			for _, ch := range lines[index].Text {
				if ch == '#' {
					nextLevel++
				} else {
					break
				}
			}
			if nextLevel <= headingLevel {
				return lines[index].Start
			}
			_ = h
		}
	}
	return len(lines)
}

func isShortBoilerplate(text, heading string) bool {
	wordCnt := wordCount(text)
	switch {
	case strings.Contains(strings.ToLower(heading), "contribut"):
		return hasContributingLink(text)
	case strings.Contains(strings.ToLower(heading), "license"):
		return hasLicenseMention(text)
	case strings.Contains(strings.ToLower(heading), "support") || strings.Contains(strings.ToLower(heading), "help") || strings.Contains(strings.ToLower(heading), "question"):
		return hasSupportLink(text)
	case strings.Contains(strings.ToLower(heading), "code of conduct"):
		return hasCOCLink(text)
	default:
		return wordCnt <= 60
	}
}

func hasContributingLink(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "contributing.md") || strings.Contains(lower, "contribute") &&
		(strings.Contains(lower, "welcome") || strings.Contains(lower, "please") || strings.Contains(lower, "read"))
}

func hasLicenseMention(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "license") &&
		(strings.Contains(lower, "mit") || strings.Contains(lower, "apache") || strings.Contains(lower, "gpl") ||
			strings.Contains(lower, "bsd") || strings.Contains(lower, "mpl") || strings.Contains(lower, "see") || strings.Contains(lower, "license.md"))
}

func hasSupportLink(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "slack") || strings.Contains(lower, "discord") || strings.Contains(lower, "issue") ||
		strings.Contains(lower, "discussion") || strings.Contains(lower, "forum") || strings.Contains(lower, "stack overflow") ||
		strings.Contains(lower, "file a") || strings.Contains(lower, "reach out") || strings.Contains(lower, "join our")
}

func hasCOCLink(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "code_of_conduct.md") || strings.Contains(lower, "code-of-conduct") ||
		strings.Contains(lower, "covenant") || strings.Contains(lower, "enforcement")
}
