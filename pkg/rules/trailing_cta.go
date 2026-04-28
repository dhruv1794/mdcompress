package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

var (
	ctaHeadingPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^star( this( repo)?)?$`),
		regexp.MustCompile(`(?i)^(please )?star us$`),
		regexp.MustCompile(`(?i)^follow( me| us)?( on .+)?$`),
		regexp.MustCompile(`(?i)^sponsor( this project)?$`),
		regexp.MustCompile(`(?i)^(buy me a )?coffee$`),
		regexp.MustCompile(`(?i)^support( me| us| this project)?$`),
		regexp.MustCompile(`(?i)^connect with( me| us)$`),
		regexp.MustCompile(`(?i)^social( media)?$`),
	}
	aboutHeadingPattern = regexp.MustCompile(`(?i)^about( the author| us)?$`)
)

type TrailingCTA struct{}

func (r *TrailingCTA) Name() string { return "strip-trailing-cta" }
func (r *TrailingCTA) Tier() Tier   { return TierSafe }

func (r *TrailingCTA) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	startOffset := trailingStartOffset(len(ctx.Source))
	var changes ChangeSet
	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		level, text, ok := parseHeading(lines[lineIndex].Text)
		if !ok || level > 2 || lines[lineIndex].Start < startOffset {
			continue
		}
		sectionEnd := headingSectionEnd(lines, lineIndex, level)
		content := sectionContent(lines, lineIndex+1, sectionEnd)
		if shouldStripCTASection(text, content) {
			addRange(&changes, render.Range{Start: lines[lineIndex].Start, End: lines[sectionEnd].End})
			lineIndex = sectionEnd
		}
	}
	return changes, nil
}

func parseHeading(line string) (int, string, bool) {
	match := headingPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, "", false
	}
	return len(match[1]), strings.TrimSpace(match[2]), true
}

func trailingStartOffset(sourceLength int) int {
	return int(float64(sourceLength) * 0.7)
}

func headingSectionEnd(lines []sourceLine, start int, level int) int {
	end := start
	for lineIndex := start + 1; lineIndex < len(lines); lineIndex++ {
		nextLevel, _, ok := parseHeading(lines[lineIndex].Text)
		if ok && nextLevel <= level {
			break
		}
		end = lineIndex
	}
	return end
}

func sectionContent(lines []sourceLine, start int, end int) string {
	var builder strings.Builder
	for lineIndex := start; lineIndex <= end && lineIndex < len(lines); lineIndex++ {
		builder.WriteString(lines[lineIndex].Text)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func shouldStripCTASection(heading string, content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(heading))
	for _, pattern := range ctaHeadingPatterns {
		if pattern.MatchString(normalized) {
			return true
		}
	}
	if aboutHeadingPattern.MatchString(normalized) {
		return wordCount(content) < 50
	}
	if normalized == "license" {
		return wordCount(content) < 30 && !strings.Contains(strings.ToLower(content), "apache license")
	}
	return false
}
