package rules

import (
	"strings"
	"unicode"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type BenchmarkProse struct{}

func (r *BenchmarkProse) Name() string { return "strip-benchmark-prose" }
func (r *BenchmarkProse) Tier() Tier   { return TierAggressive }

type benchmarkBlock struct {
	Kind      string
	Start     int
	End       int
	StartLine int
	EndLine   int
	Text      string
	Cells     []string
}

func (r *BenchmarkProse) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	blocks := benchmarkBlocks(lines)
	var changes ChangeSet

	for index, block := range blocks {
		if block.Kind != "paragraph" || sentenceCount(block.Text) > 3 {
			continue
		}
		if table, ok := adjacentBenchmarkTable(blocks, index); ok && paragraphNarratesTable(block.Text, table.Cells) {
			addRange(&changes, benchmarkParagraphRemoval(lines, block, table))
		}
	}

	return changes, nil
}

func benchmarkBlocks(lines []sourceLine) []benchmarkBlock {
	var blocks []benchmarkBlock
	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" {
			index++
			continue
		}
		if isTableStart(lines, index) {
			start := index
			index += 2
			for index < len(lines) && !lines[index].InFence && looksTableRow(lines[index].Text) {
				index++
			}
			blocks = append(blocks, benchmarkBlock{
				Kind:      "table",
				Start:     lines[start].Start,
				End:       lines[index-1].End,
				StartLine: start,
				EndLine:   index,
				Cells:     benchmarkTableCells(lines[start:index]),
			})
			continue
		}
		if !startsParagraph(trimmed) {
			index++
			continue
		}
		start := index
		var text []string
		for index < len(lines) {
			current := lines[index]
			currentTrimmed := strings.TrimSpace(current.Text)
			if current.InFence || currentTrimmed == "" || isTableStart(lines, index) || !startsParagraph(currentTrimmed) {
				break
			}
			text = append(text, currentTrimmed)
			index++
		}
		blocks = append(blocks, benchmarkBlock{
			Kind:      "paragraph",
			Start:     lines[start].Start,
			End:       lines[index-1].End,
			StartLine: start,
			EndLine:   index,
			Text:      strings.Join(text, " "),
		})
	}
	return blocks
}

func adjacentBenchmarkTable(blocks []benchmarkBlock, index int) (benchmarkBlock, bool) {
	if index > 0 && blocks[index-1].Kind == "table" {
		return blocks[index-1], true
	}
	if index+1 < len(blocks) && blocks[index+1].Kind == "table" {
		return blocks[index+1], true
	}
	return benchmarkBlock{}, false
}

func benchmarkParagraphRemoval(lines []sourceLine, paragraph benchmarkBlock, table benchmarkBlock) render.Range {
	removal := render.Range{Start: paragraph.Start, End: paragraph.End}
	if paragraph.EndLine <= table.StartLine {
		removal.End = lines[table.StartLine].Start
		return removal
	}
	if table.EndLine <= paragraph.StartLine {
		removal.Start = lines[table.EndLine-1].End
	}
	return removal
}

func isTableStart(lines []sourceLine, index int) bool {
	if index+1 >= len(lines) || lines[index].InFence || lines[index+1].InFence {
		return false
	}
	return looksTableRow(lines[index].Text) && isTableDelimiter(lines[index+1].Text)
}

func looksTableRow(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.Contains(trimmed, "|") && !strings.HasPrefix(trimmed, ">")
}

func isTableDelimiter(text string) bool {
	trimmed := strings.Trim(strings.TrimSpace(text), "|")
	if trimmed == "" {
		return false
	}
	for _, cell := range strings.Split(trimmed, "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false
		}
		hyphen := false
		for _, value := range cell {
			switch value {
			case '-', ':', ' ':
				if value == '-' {
					hyphen = true
				}
			default:
				return false
			}
		}
		if !hyphen {
			return false
		}
	}
	return true
}

func benchmarkTableCells(lines []sourceLine) []string {
	var cells []string
	for index, line := range lines {
		if index == 1 {
			continue
		}
		for _, cell := range strings.Split(strings.Trim(strings.TrimSpace(line.Text), "|"), "|") {
			normalized := normalizeBenchmarkCell(cell)
			if normalized == "" || genericBenchmarkCell(normalized) {
				continue
			}
			cells = append(cells, normalized)
		}
	}
	return cells
}

func paragraphNarratesTable(text string, cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	normalizedText := normalizeBenchmarkCell(text)
	matched := 0
	for _, cell := range cells {
		if strings.Contains(normalizedText, cell) {
			matched++
		}
	}
	return float64(matched)/float64(len(cells)) >= 0.60
}

func normalizeBenchmarkCell(text string) string {
	fields := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(value rune) bool {
		return !(unicode.IsLetter(value) || unicode.IsDigit(value) || value == '.' || value == '%' || value == '-')
	})
	return strings.Join(fields, " ")
}

func genericBenchmarkCell(cell string) bool {
	switch cell {
	case "repo", "project", "name", "tokens", "token", "before", "after", "reduction", "saved", "saving", "savings", "total", "totals":
		return true
	default:
		return false
	}
}

func startsParagraph(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, ">") {
		return false
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return false
	}
	return true
}

func sentenceCount(text string) int {
	count := 0
	for _, value := range text {
		if value == '.' || value == '!' || value == '?' {
			count++
		}
	}
	if count == 0 && strings.TrimSpace(text) != "" {
		return 1
	}
	return count
}
