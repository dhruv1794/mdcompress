package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type TableNormalize struct{}

func (r *TableNormalize) Name() string { return "compact-tables" }
func (r *TableNormalize) Tier() Tier   { return TierAggressive }

var tableDelimRe = regexp.MustCompile(`^\s*\|?[\s:-]+\|[\s|:-]+\|?\s*$`)
var tableRowRe = regexp.MustCompile(`\|`)

func (r *TableNormalize) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for i := 0; i < len(lines); i++ {
		if lines[i].InFence {
			continue
		}
		trimmed := strings.TrimSpace(lines[i].Text)
		if !tableRowRe.MatchString(trimmed) {
			continue
		}
		if heading, ok := markdownHeadingText(trimmed); ok && heading != "" {
			continue
		}

		tableStart := i
		headerLine := trimmed
		if !isTableRow(headerLine) {
			continue
		}

		delimIdx := i + 1
		if delimIdx >= len(lines) {
			continue
		}
		delimTrimmed := strings.TrimSpace(lines[delimIdx].Text)
		if !tableDelimRe.MatchString(delimTrimmed) {
			continue
		}

		bodyEnd := delimIdx + 1
		for bodyEnd < len(lines) {
			t := strings.TrimSpace(lines[bodyEnd].Text)
			if lines[bodyEnd].InFence || t == "" {
				bodyEnd++
				continue
			}
			if !isTableRow(t) {
				break
			}
			bodyEnd++
		}
		tableEnd := bodyEnd

		if tableEnd-tableStart < 3 {
			i = tableEnd - 1
			continue
		}

		compacted := compactTableLines(lines[tableStart:tableEnd])
		replacement := strings.Join(compacted, "\n") + "\n"
		addReplacement(&changes,
			lines[tableStart].Start, lines[tableEnd-1].End,
			replacement)

		i = tableEnd - 1
	}
	return changes, nil
}

func isTableRow(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, ">") &&
		!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "```")
}

func compactTableLines(lines []sourceLine) []string {
	var out []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if tableDelimRe.MatchString(trimmed) {
			continue
		}
		compact := compactTableRow(trimmed)
		if compact == "" {
			continue
		}
		out = append(out, compact)
		_ = i
	}
	return out
}

func compactTableRow(text string) string {
	hasLeadingPipe := strings.HasPrefix(strings.TrimSpace(text), "|")
	hasTrailingPipe := strings.HasSuffix(strings.TrimSpace(text), "|")
	text = strings.Trim(text, "| ")
	cells := strings.Split(text, "|")
	compacted := make([]string, len(cells))
	for i, cell := range cells {
		compacted[i] = strings.TrimSpace(cell)
	}
	result := strings.Join(compacted, " | ")
	if hasLeadingPipe {
		result = "| " + result
	}
	if hasTrailingPipe {
		result = result + " |"
	}
	return result
}
