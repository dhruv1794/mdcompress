package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// CrossFileParagraphs detects prose paragraphs (not whole sections) that
// recur verbatim across files in a multi-file run. Once the same normalized
// paragraph has been seen ≥ paragraphRepeatThreshold times across the
// corpus, subsequent occurrences are replaced with a `[same as in <file>]`
// reference pointing at the first file that recorded it.
//
// The rule is conservative: it skips fenced code, ATX headings, list items,
// blockquotes, and short paragraphs (< paragraphMinWords). Tier 2 — only
// fires when the cross-file scaffolding is wired up.
type CrossFileParagraphs struct{}

func (r *CrossFileParagraphs) Name() string { return "factor-cross-file-paragraphs" }
func (r *CrossFileParagraphs) Tier() Tier   { return TierAggressive }

const (
	paragraphMinWords         = 30
	paragraphRepeatThreshold  = 3
)

func (r *CrossFileParagraphs) Apply(ctx *Context) (ChangeSet, error) {
	if ctx.CrossFile == nil || ctx.FilePath == "" {
		return ChangeSet{}, nil
	}
	lines := sourceLines(ctx.Source)
	paragraphs := extractProseParagraphs(lines, ctx.Source)
	if len(paragraphs) == 0 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet
	for _, para := range paragraphs {
		text := string(ctx.Source[para.Start:para.End])
		if wordCount(text) < paragraphMinWords {
			continue
		}
		normalized := normalizeParagraphText(text)
		if normalized == "" {
			continue
		}
		hash := paragraphHash(normalized)
		entry, count := ctx.CrossFile.RecordParagraph(hash, ctx.FilePath, len(text))
		if count < paragraphRepeatThreshold {
			continue
		}
		if entry.CanonicalFile == ctx.FilePath {
			continue
		}
		refText := "[same as in " + fileBasename(entry.CanonicalFile) + "]"
		if len(refText)+1 >= para.End-para.Start {
			continue
		}
		addReplacement(&changes, para.Start, para.End, refText)
	}
	return changes, nil
}

type paragraphSpan struct {
	Start int
	End   int
}

// extractProseParagraphs returns byte ranges of plain-prose paragraphs:
// runs of contiguous non-blank lines that are not inside a code fence, not
// headings, not list items, not blockquotes, not tables. The range covers
// the paragraph body (no trailing newline) so the surrounding blank-line
// scaffolding is preserved.
func extractProseParagraphs(lines []sourceLine, source []byte) []paragraphSpan {
	var spans []paragraphSpan
	i := 0
	for i < len(lines) {
		if !proseLineEligible(lines[i]) {
			i++
			continue
		}
		start := lines[i].Start
		end := lineContentEnd(lines[i], source)
		j := i + 1
		for j < len(lines) && proseLineContinuation(lines[j]) {
			end = lineContentEnd(lines[j], source)
			j++
		}
		spans = append(spans, paragraphSpan{Start: start, End: end})
		i = j
	}
	return spans
}

// proseLineEligible returns true for a line that can begin a prose paragraph.
func proseLineEligible(line sourceLine) bool {
	if line.InFence {
		return false
	}
	trimmed := strings.TrimSpace(line.Text)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '#', '>', '|', '-', '*', '+':
		return false
	}
	for k := 0; k < len(trimmed); k++ {
		c := trimmed[k]
		if c >= '0' && c <= '9' {
			continue
		}
		if k > 0 && c == '.' && k+1 < len(trimmed) && trimmed[k+1] == ' ' {
			return false
		}
		break
	}
	return true
}

// proseLineContinuation is like proseLineEligible but a single blank line
// terminates the paragraph.
func proseLineContinuation(line sourceLine) bool {
	if line.InFence {
		return false
	}
	if strings.TrimSpace(line.Text) == "" {
		return false
	}
	return proseLineEligible(line)
}

// lineContentEnd returns the offset of the newline that terminates a line
// (i.e. the index just past the content), so a paragraph replacement does
// not eat the line's trailing '\n'.
func lineContentEnd(line sourceLine, source []byte) int {
	end := line.End
	if end > line.Start && end <= len(source) && source[end-1] == '\n' {
		return end - 1
	}
	return end
}

func normalizeParagraphText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

func paragraphHash(normalized string) string {
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:16])
}
