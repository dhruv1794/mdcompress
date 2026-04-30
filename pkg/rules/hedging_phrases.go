package rules

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type HedgingPhrases struct{}

func (r *HedgingPhrases) Name() string { return "strip-hedging-phrases" }
func (r *HedgingPhrases) Tier() Tier   { return TierAggressive }

type hedgingReplacement struct {
	Pattern     *regexp.Regexp
	Replacement string
}

var hedgingReplacements = []hedgingReplacement{
	{regexp.MustCompile(`(?i)\bit is worth noting that\s+`), ""},
	{regexp.MustCompile(`(?i)\bit is worth noting\s+`), ""},
	{regexp.MustCompile(`(?i)\bplease note that\s+`), ""},
	{regexp.MustCompile(`(?i)\bplease note\s+`), ""},
	{regexp.MustCompile(`(?i)\bit should be noted that\s+`), ""},
	{regexp.MustCompile(`(?i)\bit is important to note that\s+`), ""},
	{regexp.MustCompile(`(?i)\bneedless to say,?\s+`), ""},
	{regexp.MustCompile(`(?i)\bit goes without saying that\s+`), ""},
	{regexp.MustCompile(`(?i)\bas a matter of fact,?\s+`), ""},
	{regexp.MustCompile(`(?i)\bit should be mentioned that\s+`), ""},
	{regexp.MustCompile(`(?i)\bin order to\b`), "to"},
	{regexp.MustCompile(`(?i)\bdue to the fact that\b`), "because"},
	{regexp.MustCompile(`(?i)\bat this point in time\b`), "now"},
	{regexp.MustCompile(`(?i)\bin the event that\b`), "if"},
	{regexp.MustCompile(`(?i)\bwith regard to\b`), "about"},
	{regexp.MustCompile(`(?i)\bfor the purpose of\b`), "for"},
	{regexp.MustCompile(`(?i)\ba number of\b`), "several"},
}

func (r *HedgingPhrases) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" || strings.HasPrefix(trimmed, "|") {
			continue
		}
		for _, edit := range hedgingLineEdits(line) {
			changes.Edits = append(changes.Edits, edit)
			changes.Stats.NodesAffected++
			changes.Stats.BytesSaved += edit.End - edit.Start - len(edit.Replacement)
		}
	}
	return changes, nil
}

func hedgingLineEdits(line sourceLine) []render.Edit {
	var candidates []render.Edit
	for _, replacement := range hedgingReplacements {
		for _, match := range replacement.Pattern.FindAllStringIndex(line.Text, -1) {
			if insideInlineCode(line.Text, match[0]) {
				continue
			}
			candidates = append(candidates, hedgingEdit(line.Text, match[0], match[1], replacement.Replacement))
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Start < candidates[j].Start })

	var edits []render.Edit
	occupiedUntil := -1
	for _, edit := range candidates {
		if edit.Start < occupiedUntil {
			continue
		}
		occupiedUntil = edit.End
		edit.Start += line.Start
		edit.End += line.Start
		edits = append(edits, edit)
	}
	return edits
}

func hedgingEdit(text string, start, end int, replacement string) render.Edit {
	if replacement == "" {
		end, replacement = maybeCapitalizeFollowing(text, start, end)
	} else if atSentenceStart(text, start) {
		replacement = capitalizeASCII(replacement)
	}
	return render.Edit{
		Start:       start,
		End:         end,
		Replacement: []byte(replacement),
	}
}

func maybeCapitalizeFollowing(text string, start, end int) (int, string) {
	if !atSentenceStart(text, start) || end >= len(text) {
		return end, ""
	}
	r, size := utf8.DecodeRuneInString(text[end:])
	if r == utf8.RuneError || !unicode.IsLower(r) {
		return end, ""
	}
	return end + size, string(unicode.ToUpper(r))
}

func atSentenceStart(text string, start int) bool {
	prefix := strings.TrimSpace(text[:start])
	if prefix == "" {
		return true
	}
	last, _ := utf8.DecodeLastRuneInString(prefix)
	return last == '.' || last == '!' || last == '?'
}

func capitalizeASCII(value string) string {
	if value == "" {
		return value
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError {
		return value
	}
	return string(unicode.ToUpper(r)) + value[size:]
}
