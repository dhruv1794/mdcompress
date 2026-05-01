package rules

import (
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
)

type MultilangDedup struct{}

func (r *MultilangDedup) Name() string { return "dedup-multilang-examples" }
func (r *MultilangDedup) Tier() Tier   { return TierAggressive }

var langCommentMap = map[string]string{
	"go":         "//",
	"golang":     "//",
	"python":     "#",
	"py":         "#",
	"ruby":       "#",
	"rb":         "#",
	"javascript": "//",
	"js":         "//",
	"typescript": "//",
	"ts":         "//",
	"java":       "//",
	"scala":      "//",
	"kotlin":     "//",
	"rust":       "//",
	"rs":         "//",
	"c":          "//",
	"cpp":        "//",
	"c++":        "//",
	"h":          "//",
	"hpp":        "//",
	"swift":      "//",
	"php":        "//",
	"cs":         "//",
	"dart":       "//",
	"svelte":     "//",
	"vue":        "//",
}

func (r *MultilangDedup) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	blocks := fencedBlocks(lines)
	if len(blocks) < 2 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet

	for i := 1; i < len(blocks); i++ {
		prev := blocks[i-1]
		curr := blocks[i]
		if len(prev.Content) == 0 || len(curr.Content) == 0 {
			continue
		}
		prevLang := fenceLanguage(lines[prev.StartLine].Text)
		currLang := fenceLanguage(lines[curr.StartLine].Text)
		if prevLang == "" || currLang == "" || prevLang == currLang {
			continue
		}

		prevNorm := normalizeCode(extractTexts(prev.Content), prevLang)
		currNorm := normalizeCode(extractTexts(curr.Content), currLang)

		if !areCodesSimilar(prevNorm, currNorm) {
			continue
		}

		totalLines := len(curr.Content)
		firstContent := curr.Content[0]

		if totalLines < 2 {
			continue
		}

		lastContent := curr.Content[totalLines-1]
		replacement := "[equivalent to " + prevLang + " example above]\n"
		addReplacement(&changes,
			firstContent.Start, lastContent.End,
			replacement)
	}

	return changes, nil
}

func normalizeCode(lines []string, lang string) string {
	var normalized []string
	commentPrefix := langCommentMap[lang]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if commentPrefix != "" && strings.HasPrefix(line, commentPrefix) {
			continue
		}
		line = strings.ToLower(line)
		line = strings.Trim(line, ";{}")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func areCodesSimilar(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")

	if len(aLines) < 2 || len(bLines) < 2 {
		return false
	}

	lenRatio := float64(len(aLines)) / float64(len(bLines))
	if lenRatio < 0.4 || lenRatio > 2.5 {
		return false
	}

	similarity := textWordSimilarity(a, b)
	return similarity >= 0.50
}

func textWordSimilarity(a, b string) float64 {
	aWords := strings.FieldsFunc(strings.ToLower(a), splitNonAlpha)
	bWords := strings.FieldsFunc(strings.ToLower(b), splitNonAlpha)
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}

	aSet := make(map[string]int, len(aWords))
	for _, w := range aWords {
		aSet[w]++
	}

	matches := 0
	seen := make(map[string]bool)
	for _, w := range bWords {
		if aSet[w] > 0 && !seen[w] {
			matches++
			seen[w] = true
		}
	}

	total := len(aWords)
	if len(bWords) > total {
		total = len(bWords)
	}
	return float64(matches) / float64(total)
}

func splitNonAlpha(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
