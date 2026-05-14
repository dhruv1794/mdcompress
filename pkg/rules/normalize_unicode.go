package rules

import (
	"strings"
	"unicode/utf8"
)

// NormalizeUnicode replaces typographic Unicode characters with ASCII
// equivalents that tokenize more cheaply for LLMs:
//
//   - Smart quotes (curly, low-9, guillemets) → ASCII " or '
//   - Em/en/figure/horizontal-bar dashes / minus → -- or -
//   - Non-ASCII space variants (NBSP, narrow no-break, en/em/thin/hair/figure
//     spaces, ideographic space) → ASCII space
//   - Invisible characters (soft hyphen, zero-width space/joiner/non-joiner,
//     BOM) → stripped
//   - Bullet variants → "*"
//   - Ellipsis → "..."
//
// Code fences are skipped — code identity must be byte-preserved. Inline-code
// spans are also skipped to avoid rewriting backticked snippets.
type NormalizeUnicode struct{}

func (r *NormalizeUnicode) Name() string { return "normalize-unicode" }
func (r *NormalizeUnicode) Tier() Tier   { return TierSafe }

func (r *NormalizeUnicode) Apply(ctx *Context) (ChangeSet, error) {
	source := ctx.Source
	lines := sourceLines(source)
	var changes ChangeSet

	for _, line := range lines {
		if line.InFence {
			continue
		}
		text := source[line.Start:line.End]
		if !hasNonASCIIByte(text) {
			continue
		}
		normalized := normalizeRunesPreservingInlineCode(string(text))
		if normalized == string(text) {
			continue
		}
		addReplacement(&changes, line.Start, line.End, normalized)
	}
	return changes, nil
}

// hasNonASCIIByte runs in bytes (no UTF-8 decode) — most lines are pure
// ASCII and skipping them avoids the cost of normalizing.
func hasNonASCIIByte(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] >= 0x80 {
			return true
		}
	}
	return false
}

// normalizeRunesPreservingInlineCode applies the rune map, except inside
// inline-code spans (between unescaped backticks on a single line).
func normalizeRunesPreservingInlineCode(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	inCode := false
	for i := 0; i < len(text); {
		c := text[i]
		if c == '`' {
			b.WriteByte(c)
			inCode = !inCode
			i++
			continue
		}
		if inCode || c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		if size == 0 {
			b.WriteByte(c)
			i++
			continue
		}
		if repl, ok := unicodeReplacement(r); ok {
			b.WriteString(repl)
		} else {
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// unicodeReplacement returns the ASCII replacement for r and ok=true when r
// should be normalized. ok=false leaves r in place.
//
// Code points are written as \uXXXX escapes throughout — these characters
// are visually identical or invisible in source, so escapes are the only
// honest representation.
func unicodeReplacement(r rune) (string, bool) {
	switch r {
	// Smart double quotes.
	case '“', '”', '„', '‟', '«', '»':
		return `"`, true
	// Smart single quotes / apostrophes.
	case '‘', '’', '‚', '‛', '‹', '›':
		return "'", true
	// Em-dash, horizontal bar.
	case '—', '―':
		return "--", true
	// En-dash, figure dash, minus sign.
	case '–', '‒', '−':
		return "-", true
	// Ellipsis.
	case '…':
		return "...", true
	// Bullet variants. Middle-dot U+00B7 is intentionally NOT included —
	// it's used as a real character in many languages and as a separator in
	// technical writing.
	case '•', '∙', '◦', '⁃':
		return "*", true
	// Space variants → ASCII space:
	// NBSP, en quad..hair space, NNBSP, medium math space, ideographic space.
	case ' ',
		' ', ' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', '　':
		return " ", true
	// Invisible characters → strip:
	// soft hyphen, ZWSP, ZWNJ, ZWJ, BOM/ZWNBSP.
	case '\u00ad', '\u200b', '\u200c', '\u200d', '\ufeff':
		return "", true
	}
	return "", false
}
