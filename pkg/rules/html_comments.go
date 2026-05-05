package rules

import (
	"bytes"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

// HTMLComments removes HTML comments from markdown source.
//
// The rule operates on raw bytes (not the AST): it scans for "<!-- ... -->"
// pairs that fall outside fenced code blocks. Comments that span multiple
// lines are removed in full; an unterminated "<!--" is left alone.
type HTMLComments struct{}

func (r *HTMLComments) Name() string { return "strip-html-comments" }
func (r *HTMLComments) Tier() Tier   { return TierSafe }

func (r *HTMLComments) Apply(ctx *Context) (ChangeSet, error) {
	var changes ChangeSet
	source := ctx.Source
	lines := sourceLines(source)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line.InFence {
			continue
		}
		text := source[line.Start:line.End]
		searchOffset := line.Start
		searchSlice := text
		for {
			rel := bytes.Index(searchSlice, []byte("<!--"))
			if rel < 0 {
				break
			}
			startAbs := searchOffset + rel
			closeAbs, closeLine := findCommentClose(lines, source, i, startAbs+4)
			if closeAbs < 0 {
				// Unterminated comment — leave the rest of this line alone.
				break
			}
			endAbs := closeAbs + 3
			addCommentRemoval(source, render.Range{Start: startAbs, End: endAbs}, &changes)
			if closeLine == i {
				// Continue scanning the rest of the same line for more comments.
				rest := endAbs - line.Start
				searchOffset = line.Start + rest
				if rest >= len(text) {
					break
				}
				searchSlice = text[rest:]
				continue
			}
			// Multi-line comment — skip past the closing line.
			i = closeLine
			break
		}
	}
	return changes, nil
}

// findCommentClose locates the next "-->" starting at byte offset start within
// or after lines[startLine]. Returns (-1, -1) if there's no close before EOF.
func findCommentClose(lines []sourceLine, source []byte, startLine, start int) (int, int) {
	for j := startLine; j < len(lines); j++ {
		l := lines[j]
		if l.End <= start {
			continue
		}
		searchFrom := max(start, l.Start)
		rel := bytes.Index(source[searchFrom:l.End], []byte("-->"))
		if rel >= 0 {
			return searchFrom + rel, j
		}
	}
	return -1, -1
}

func addCommentRemoval(source []byte, commentRange render.Range, changes *ChangeSet) {
	removal := expandOwnLineRange(source, commentRange)
	changes.Edits = append(changes.Edits, render.Edit{Start: removal.Start, End: removal.End})
	changes.Ranges = append(changes.Ranges, removal)
	changes.Stats.NodesAffected++
	changes.Stats.BytesSaved += removal.End - removal.Start
}

func expandOwnLineRange(source []byte, r render.Range) render.Range {
	lineStart := bytes.LastIndexByte(source[:r.Start], '\n') + 1
	lineEnd := r.End
	if nextNewline := bytes.IndexByte(source[r.End:], '\n'); nextNewline >= 0 {
		lineEnd = r.End + nextNewline + 1
	} else {
		lineEnd = len(source)
	}

	before := bytes.TrimSpace(source[lineStart:r.Start])
	afterEnd := lineEnd
	if afterEnd > r.End && source[afterEnd-1] == '\n' {
		afterEnd--
	}
	after := bytes.TrimSpace(source[r.End:afterEnd])
	if len(before) == 0 && len(after) == 0 {
		if lineStart > 0 && lineEnd < len(source) && source[lineStart-1] == '\n' && source[lineEnd] == '\n' {
			lineEnd++
		}
		return render.Range{Start: lineStart, End: lineEnd}
	}
	return r
}
