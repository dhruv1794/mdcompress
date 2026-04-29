package rules

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

type sourceLine struct {
	Start   int
	End     int
	Text    string
	InFence bool
}

func sourceLines(source []byte) []sourceLine {
	lines := make([]sourceLine, 0)
	inFence := false
	fenceMarker := byte(0)

	for lineStart := 0; lineStart < len(source); {
		lineEnd := bytes.IndexByte(source[lineStart:], '\n')
		contentEnd := 0
		if lineEnd == -1 {
			contentEnd = len(source)
			lineEnd = len(source)
		} else {
			contentEnd = lineStart + lineEnd
			lineEnd = contentEnd + 1
		}

		text := string(source[lineStart:contentEnd])
		line := sourceLine{
			Start:   lineStart,
			End:     lineEnd,
			Text:    text,
			InFence: inFence,
		}
		lines = append(lines, line)

		if marker, ok := fencedCodeMarker([]byte(text)); ok {
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
			}
		}

		if lineEnd == len(source) {
			break
		}
		lineStart = lineEnd
	}

	return lines
}

func fencedCodeMarker(line []byte) (byte, bool) {
	trimmed := bytes.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return 0, false
	}
	if trimmed[0] != '`' && trimmed[0] != '~' {
		return 0, false
	}
	marker := trimmed[0]
	count := 0
	for count < len(trimmed) && trimmed[count] == marker {
		count++
	}
	return marker, count >= 3
}

func fullLineRange(line sourceLine) render.Range {
	return render.Range{Start: line.Start, End: line.End}
}

func addRange(changes *ChangeSet, removal render.Range) {
	if removal.Start >= removal.End {
		return
	}
	changes.Edits = append(changes.Edits, render.Edit{Start: removal.Start, End: removal.End})
	changes.Ranges = append(changes.Ranges, removal)
	changes.Stats.NodesAffected++
	changes.Stats.BytesSaved += removal.End - removal.Start
}

func addReplacement(changes *ChangeSet, start, end int, replacement string) {
	if start >= end {
		return
	}
	changes.Edits = append(changes.Edits, render.Edit{
		Start:       start,
		End:         end,
		Replacement: []byte(replacement),
	})
	changes.Stats.NodesAffected++
	changes.Stats.BytesSaved += end - start - len(replacement)
}

func wordCount(text string) int {
	return len(strings.FieldsFunc(text, func(value rune) bool {
		return !(unicode.IsLetter(value) || unicode.IsDigit(value))
	}))
}

func singleRune(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	_, size := utf8.DecodeRuneInString(trimmed)
	return size == len(trimmed)
}
