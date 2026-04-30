package rules

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type MarketingProse struct{}

func (r *MarketingProse) Name() string { return "strip-marketing-prose" }
func (r *MarketingProse) Tier() Tier   { return TierAggressive }

var marketingPhrases = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bblazing(?:ly)? fast\b`),
	regexp.MustCompile(`(?i)\blightning fast\b`),
	regexp.MustCompile(`(?i)\bsuper-?fast\b`),
	regexp.MustCompile(`(?i)\bincredibly fast\b`),
	regexp.MustCompile(`(?i)\bproduction-ready\b`),
	regexp.MustCompile(`(?i)\bproduction-grade\b`),
	regexp.MustCompile(`(?i)\bbattle-tested\b`),
	regexp.MustCompile(`(?i)\bloved by developers\b`),
	regexp.MustCompile(`(?i)\bdeveloper-first\b`),
	regexp.MustCompile(`(?i)\bdeveloper-friendly\b`),
	regexp.MustCompile(`(?i)\bfeature-rich\b`),
	regexp.MustCompile(`(?i)\bfully-featured\b`),
	regexp.MustCompile(`(?i)\bcutting-edge\b`),
	regexp.MustCompile(`(?i)\bstate-of-the-art\b`),
	regexp.MustCompile(`(?i)\brock-solid\b`),
	regexp.MustCompile(`(?i)\brobust\b`),
	regexp.MustCompile(`(?i)\bhighly performant\b`),
	regexp.MustCompile(`(?i)\belegant\b`),
	regexp.MustCompile(`(?i)\bbeautiful\b`),
	regexp.MustCompile(`(?i)\bdelightful\b`),
	regexp.MustCompile(`(?i)\bworld-class\b`),
	regexp.MustCompile(`(?i)\bindustry-leading\b`),
	regexp.MustCompile(`(?i)\benterprise-grade\b`),
	regexp.MustCompile(`(?i)\bbest-in-class\b`),
	regexp.MustCompile(`(?i)\bnext-generation\b`),
	regexp.MustCompile(`(?i)\bseamless(?:ly)?\b`),
	regexp.MustCompile(`(?i)\bintuitive\b`),
	regexp.MustCompile(`(?i)\bunparalleled\b`),
	regexp.MustCompile(`(?i)\bground-?breaking\b`),
	regexp.MustCompile(`(?i)\brevolutionary\b`),
}

var marketingFeatureHeadingPattern = regexp.MustCompile(`(?i)\b(features?|highlights?|benefits?|why|overview)\b`)

var technicalContextPattern = regexp.MustCompile(`(?i)\b(uptime|latency|throughput|benchmark|benchmarks|load|memory|cpu|concurrency|concurrent|failure|failover|retry|retries|error rate|p95|p99|rps|qps|ops/sec)\b`)

func (r *MarketingProse) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	inIntro := false
	seenTitle := false
	currentHeading := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" {
			continue
		}
		if headingText, ok := markdownHeadingText(trimmed); ok {
			if !seenTitle {
				seenTitle = true
				inIntro = true
			} else {
				inIntro = false
			}
			currentHeading = headingText
			continue
		}
		if !marketingContext(line.Text, inIntro, currentHeading) {
			continue
		}
		for _, removal := range marketingPhraseRanges(line) {
			addRange(&changes, render.Range{
				Start: line.Start + removal.Start,
				End:   line.Start + removal.End,
			})
		}
	}

	return changes, nil
}

func markdownHeadingText(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[level+1:]), true
}

func marketingContext(text string, inIntro bool, currentHeading string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, ">") {
		return false
	}
	if inIntro && !looksStructural(trimmed) {
		return true
	}
	return isListItem(trimmed) && marketingFeatureHeadingPattern.MatchString(currentHeading)
}

func looksStructural(trimmed string) bool {
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		return true
	}
	return isListItem(trimmed)
}

func isListItem(trimmed string) bool {
	if len(trimmed) < 2 {
		return false
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	dot := strings.IndexByte(trimmed, '.')
	if dot <= 0 || dot+1 >= len(trimmed) || trimmed[dot+1] != ' ' {
		return false
	}
	for _, r := range trimmed[:dot] {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func marketingPhraseRanges(line sourceLine) []render.Range {
	if technicalContextPattern.MatchString(line.Text) {
		return nil
	}

	var ranges []render.Range
	for _, phrase := range marketingPhrases {
		for _, match := range phrase.FindAllStringIndex(line.Text, -1) {
			if insideInlineCode(line.Text, match[0]) {
				continue
			}
			start, end := trimMarketingPhraseSpace(line.Text, match[0], match[1])
			ranges = append(ranges, render.Range{Start: start, End: end})
		}
	}
	return ranges
}

func trimMarketingPhraseSpace(text string, start, end int) (int, int) {
	if end < len(text) && text[end] == ',' {
		end++
		for end < len(text) && text[end] == ' ' {
			end++
		}
		return start, end
	}
	if start >= 2 && text[start-2:start] == ", " {
		start -= 2
		if end < len(text) && text[end] == ' ' {
			end++
		}
		return start, end
	}
	if end < len(text) && text[end] == ' ' {
		end++
		return start, end
	}
	if start > 0 && text[start-1] == ' ' {
		start--
	}
	return start, end
}

func insideInlineCode(text string, offset int) bool {
	return strings.Count(text[:offset], "`")%2 == 1
}
