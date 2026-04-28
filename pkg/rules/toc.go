package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

var (
	headingPattern    = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$`)
	listItemPattern   = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s+`)
	anchorLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(#[^)]+\)`)
)

type TOC struct{}

func (r *TOC) Name() string { return "strip-toc" }
func (r *TOC) Tier() Tier   { return TierSafe }

func (r *TOC) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		line := lines[lineIndex]
		if line.InFence || !isTOCHeading(line.Text) {
			continue
		}
		listStart := nextNonBlankLine(lines, lineIndex+1)
		if listStart == -1 || !listItemPattern.MatchString(lines[listStart].Text) {
			continue
		}
		listEnd, totalItems, anchorItems := tocListEnd(lines, listStart)
		if totalItems == 0 || float64(anchorItems)/float64(totalItems) < 0.7 {
			continue
		}
		addRange(&changes, sectionRange(lines, lineIndex, listEnd))
		lineIndex = listEnd
	}
	return changes, nil
}

func isTOCHeading(line string) bool {
	match := headingPattern.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(match[2]))
	return text == "table of contents" || text == "table of content" || text == "contents" || text == "toc"
}

func nextNonBlankLine(lines []sourceLine, start int) int {
	for lineIndex := start; lineIndex < len(lines); lineIndex++ {
		if strings.TrimSpace(lines[lineIndex].Text) != "" {
			return lineIndex
		}
	}
	return -1
}

func tocListEnd(lines []sourceLine, start int) (int, int, int) {
	end := start
	totalItems := 0
	anchorItems := 0
	for end < len(lines) {
		text := lines[end].Text
		if strings.TrimSpace(text) == "" {
			end++
			continue
		}
		if !listItemPattern.MatchString(text) {
			break
		}
		totalItems++
		if anchorLinkPattern.MatchString(text) {
			anchorItems++
		}
		end++
	}
	return end - 1, totalItems, anchorItems
}

func sectionRange(lines []sourceLine, start int, end int) render.Range {
	return render.Range{Start: lines[start].Start, End: lines[end].End}
}
