package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type HTMLWrappers struct{}

func (r *HTMLWrappers) Name() string { return "strip-html-wrappers" }
func (r *HTMLWrappers) Tier() Tier   { return TierAggressive }

var (
	htmlOpenTagRe  = regexp.MustCompile(`^\s*</?(p|div|details|summary|span|center|font|big|small|dfn|abbr)\b[^>]*>\s*$`)
	htmlCloseTagRe = regexp.MustCompile(`^\s*</(p|div|details|summary|span|center|font|big|small|dfn|abbr)>\s*$`)
	htmlSmallTagRe = regexp.MustCompile(`(?i)^\s*<small>`)
	htmlSmallCloseRe = regexp.MustCompile(`(?i)</small>\s*$`)
	htmlAlignRe    = regexp.MustCompile(`(?i)^\s*<p\s+align\s*=\s*"[^"]*"\s*>\s*$`)
)

func (r *HTMLWrappers) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || line.InFence {
			continue
		}

		if htmlAlignRe.MatchString(trimmed) || (htmlOpenTagRe.MatchString(trimmed) && !htmlSmallTagRe.MatchString(trimmed)) {
			closing := htmlCloseTagRe.FindStringSubmatch(trimmed)
			if closing != nil {
				addRange(&changes, fullLineRange(line))
				continue
			}

			tagName := extractTagName(trimmed)
			if tagName != "" {
				j := i + 1
				for j < len(lines) {
					nextLine := strings.TrimSpace(lines[j].Text)
					if nextLine == "" || lines[j].InFence {
						j++
						continue
					}
					if isClosingTag(nextLine, tagName) {
						addRange(&changes, fullLineRange(line))
						addRange(&changes, fullLineRange(lines[j]))
						i = j
						break
					}
					if htmlOpenTagRe.MatchString(nextLine) {
						break
					}
					break
				}
			}
			continue
		}

		if htmlCloseTagRe.MatchString(trimmed) {
			addRange(&changes, fullLineRange(line))
			continue
		}

		if htmlSmallTagRe.MatchString(trimmed) {
			end := line.End
			for j := i + 1; j < len(lines); j++ {
				end = lines[j].End
				if htmlSmallCloseRe.MatchString(strings.TrimSpace(lines[j].Text)) {
					addRange(&changes, render.Range{Start: line.Start, End: end})
					i = j
					break
				}
				if j-i > 5 {
					break
				}
			}
		}
	}

	return changes, nil
}

func extractTagName(trimmed string) string {
	trimmed = strings.TrimLeft(trimmed, "</")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimRight(parts[0], ">")
}

func isClosingTag(trimmed, tagName string) bool {
	return strings.HasPrefix(strings.TrimSpace(trimmed), "</"+tagName+">")
}
