package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type ExampleOutput struct{}

func (r *ExampleOutput) Name() string { return "collapse-example-output" }
func (r *ExampleOutput) Tier() Tier   { return TierAggressive }

type fencedBlock struct {
	StartLine int
	EndLine   int
	Start     int
	End       int
	Content   []sourceLine
}

var (
	exampleCommandPattern = regexp.MustCompile(`(?i)(--help|-h|--version)(?:\s*[` + "`" + `"'”’)]*)?\s*[:.]?$`)
	flagOutputPattern     = regexp.MustCompile(`^\s*(?:-{1,2}[A-Za-z0-9][A-Za-z0-9-]*|[A-Za-z0-9][A-Za-z0-9-]*,\s*--[A-Za-z0-9][A-Za-z0-9-]*)(?:\s|,|$)`)
	usageOutputPattern    = regexp.MustCompile(`(?i)^\s*(usage|options|flags|commands|available commands):\s*`)
)

func (r *ExampleOutput) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for _, block := range fencedBlocks(lines) {
		command, ok := precedingExampleCommand(lines, block.StartLine)
		if !ok {
			continue
		}
		if exampleOutputBlock(command, block.Content) {
			addRange(&changes, render.Range{Start: block.Start, End: block.End})
		}
	}
	return changes, nil
}

func fencedBlocks(lines []sourceLine) []fencedBlock {
	var blocks []fencedBlock
	for index := 0; index < len(lines); index++ {
		if lines[index].InFence {
			continue
		}
		marker, ok := fencedCodeMarker([]byte(lines[index].Text))
		if !ok {
			continue
		}
		end := index + 1
		for end < len(lines) {
			if lines[end].InFence {
				if closing, ok := fencedCodeMarker([]byte(lines[end].Text)); ok && closing == marker {
					blocks = append(blocks, fencedBlock{
						StartLine: index,
						EndLine:   end,
						Start:     lines[index].Start,
						End:       lines[end].End,
						Content:   lines[index+1 : end],
					})
					index = end
					break
				}
			}
			end++
		}
	}
	return blocks
}

func precedingExampleCommand(lines []sourceLine, blockStart int) (string, bool) {
	index := blockStart - 1
	for index >= 0 && strings.TrimSpace(lines[index].Text) == "" {
		index--
	}
	if index < 0 {
		return "", false
	}
	if commandLine := strings.TrimSpace(lines[index].Text); exampleCommandPattern.MatchString(commandLine) {
		return commandLine, true
	}
	if lines[index].InFence {
		for index >= 0 && lines[index].InFence {
			index--
		}
		if index >= 0 {
			for cursor := blockStart - 1; cursor > index; cursor-- {
				commandLine := strings.TrimSpace(lines[cursor].Text)
				if commandLine == "" {
					continue
				}
				if _, ok := fencedCodeMarker([]byte(commandLine)); ok {
					continue
				}
				if exampleCommandPattern.MatchString(commandLine) {
					return commandLine, true
				}
				return "", false
			}
		}
	}
	return "", false
}

func exampleOutputBlock(command string, content []sourceLine) bool {
	nonblank := 0
	flagLike := 0
	usageLike := 0
	for _, line := range content {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" {
			continue
		}
		nonblank++
		if nonblank > 50 {
			return false
		}
		if usageOutputPattern.MatchString(trimmed) {
			usageLike++
		}
		if flagOutputPattern.MatchString(trimmed) {
			flagLike++
		}
	}
	if nonblank == 0 {
		return false
	}
	if strings.Contains(strings.ToLower(command), "--version") {
		return nonblank <= 3
	}
	if usageLike > 0 && flagLike >= 1 {
		return true
	}
	return float64(flagLike)/float64(nonblank) >= 0.50
}
