package rules

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type DedupCrossSection struct{}

func (r *DedupCrossSection) Name() string { return "dedup-cross-section" }
func (r *DedupCrossSection) Tier() Tier   { return TierAggressive }

type dedupParagraph struct {
	Start     int
	End       int
	Text      string
	InIntro   bool
	Sentences []dedupSentence
}

type dedupSentence struct {
	Start int
	End   int
	Text  string
}

var dedupTokenPattern = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._/-]*`)

func (r *DedupCrossSection) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	paragraphs := dedupParagraphs(sourceLines(ctx.Source), ctx.Source)
	intro := dedupIntroSentences(paragraphs)
	body := dedupBodySentences(paragraphs)
	var changes ChangeSet

	for _, sentence := range intro {
		if !dedupSentenceIsClaim(sentence.Text) {
			continue
		}
		for _, candidate := range body {
			if dedupBodySupersedesIntro(sentence.Text, candidate.Text) {
				addRange(&changes, dedupRemovalRange(ctx.Source, sentence))
				break
			}
		}
	}

	return changes, nil
}

func dedupParagraphs(lines []sourceLine, source []byte) []dedupParagraph {
	var paragraphs []dedupParagraph
	seenFirstHeading := false
	inIntro := true

	for index := 0; index < len(lines); {
		line := lines[index]
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" {
			index++
			continue
		}
		if _, ok := markdownHeadingText(trimmed); ok {
			if !seenFirstHeading {
				seenFirstHeading = true
			} else {
				inIntro = false
			}
			index++
			continue
		}
		if !startsParagraph(trimmed) {
			index++
			continue
		}

		start := index
		for index < len(lines) {
			current := lines[index]
			currentTrimmed := strings.TrimSpace(current.Text)
			if current.InFence || currentTrimmed == "" || !startsParagraph(currentTrimmed) || looksTableRow(current.Text) {
				break
			}
			index++
		}
		startOffset := lines[start].Start
		endOffset := lines[index-1].End
		text := string(source[startOffset:endOffset])
		paragraphs = append(paragraphs, dedupParagraph{
			Start:     startOffset,
			End:       endOffset,
			Text:      text,
			InIntro:   inIntro,
			Sentences: dedupSentences(text, startOffset),
		})
	}
	return paragraphs
}

func dedupSentences(text string, base int) []dedupSentence {
	var sentences []dedupSentence
	start := 0
	for index, value := range text {
		if value != '.' && value != '!' && value != '?' {
			continue
		}
		end := index + len(string(value))
		if end < len(text) {
			next, _ := nextRune(text[end:])
			if next != 0 && !unicode.IsSpace(next) {
				continue
			}
		}
		if sentence := strings.TrimSpace(text[start:end]); sentence != "" {
			trimmedStart := start + leadingSpaceLen(text[start:end])
			sentences = append(sentences, dedupSentence{
				Start: base + trimmedStart,
				End:   base + end,
				Text:  sentence,
			})
		}
		start = end
		for start < len(text) {
			value, size := nextRune(text[start:])
			if value == 0 || !unicode.IsSpace(value) {
				break
			}
			start += size
		}
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		trimmedStart := start + leadingSpaceLen(text[start:])
		sentences = append(sentences, dedupSentence{
			Start: base + trimmedStart,
			End:   base + len(strings.TrimRightFunc(text, unicode.IsSpace)),
			Text:  tail,
		})
	}
	return sentences
}

func nextRune(text string) (rune, int) {
	for _, value := range text {
		return value, len(string(value))
	}
	return 0, 0
}

func leadingSpaceLen(text string) int {
	for index, value := range text {
		if !unicode.IsSpace(value) {
			return index
		}
	}
	return len(text)
}

func dedupIntroSentences(paragraphs []dedupParagraph) []dedupSentence {
	var sentences []dedupSentence
	for _, paragraph := range paragraphs {
		if paragraph.InIntro {
			sentences = append(sentences, paragraph.Sentences...)
		}
	}
	return sentences
}

func dedupBodySentences(paragraphs []dedupParagraph) []dedupSentence {
	var sentences []dedupSentence
	for _, paragraph := range paragraphs {
		if !paragraph.InIntro {
			sentences = append(sentences, paragraph.Sentences...)
		}
	}
	return sentences
}

func dedupSentenceIsClaim(text string) bool {
	trimmed := strings.TrimSpace(text)
	words := wordCount(trimmed)
	if words < 6 || words > 45 {
		return false
	}
	if strings.Contains(trimmed, "](") || strings.HasPrefix(trimmed, ">") {
		return false
	}
	return true
}

func dedupBodySupersedesIntro(intro, body string) bool {
	introTokens := dedupMeaningfulTokens(intro)
	bodyTokens := dedupMeaningfulTokens(body)
	if len(introTokens) < 5 || len(bodyTokens) <= len(introTokens) {
		return false
	}
	if wordCount(body) < wordCount(intro)+3 {
		return false
	}
	if !dedupSpecialTokensPreserved(intro, body) {
		return false
	}
	coverage := dedupCoverage(introTokens, bodyTokens)
	return coverage >= 0.85
}

func dedupMeaningfulTokens(text string) map[string]bool {
	tokens := make(map[string]bool)
	for _, token := range dedupTokenPattern.FindAllString(strings.ToLower(text), -1) {
		token = strings.Trim(token, "._/-")
		if token == "" || dedupStopToken(token) {
			continue
		}
		tokens[token] = true
	}
	return tokens
}

func dedupStopToken(token string) bool {
	switch token {
	case "a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "in", "into", "is", "it", "of", "on", "or", "that", "the", "this", "to", "with":
		return true
	default:
		return false
	}
}

func dedupSpecialTokensPreserved(intro, body string) bool {
	bodyLower := strings.ToLower(body)
	for _, token := range dedupTokenPattern.FindAllString(intro, -1) {
		token = strings.Trim(token, "._/-")
		if !dedupSpecialToken(token) {
			continue
		}
		if !strings.Contains(bodyLower, strings.ToLower(token)) {
			return false
		}
	}
	return true
}

func dedupSpecialToken(token string) bool {
	if strings.ContainsAny(token, "0123456789`./_-") {
		return true
	}
	for _, value := range token {
		return unicode.IsUpper(value)
	}
	return false
}

func dedupCoverage(intro, body map[string]bool) float64 {
	matched := 0
	for token := range intro {
		if body[token] {
			matched++
		}
	}
	return float64(matched) / float64(len(intro))
}

func dedupRemovalRange(source []byte, sentence dedupSentence) render.Range {
	removal := render.Range{Start: sentence.Start, End: sentence.End}
	for removal.End < len(source) {
		value := source[removal.End]
		if value != ' ' && value != '\t' {
			break
		}
		removal.End++
	}
	for removal.Start > 0 {
		value := source[removal.Start-1]
		if value != ' ' && value != '\t' {
			break
		}
		removal.Start--
	}
	return removal
}
