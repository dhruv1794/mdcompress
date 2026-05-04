package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// structuralNormalize fingerprints prose for cross-file dedup, treating
// version strings, numbers, URLs, paths, and quoted literals as wildcards
// so near-duplicate sections collide on the same hash.
func structuralNormalize(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strNormURL.ReplaceAllString(text, " <url> ")
	text = strNormMDLink.ReplaceAllString(text, " $1 ")
	text = strNormMDImage.ReplaceAllString(text, " ")
	text = strNormCode.ReplaceAllString(text, " <code> ")
	text = strNormVersion.ReplaceAllString(text, " <ver> ")
	text = strNormHexLit.ReplaceAllString(text, " <hex> ")
	text = strNormNumber.ReplaceAllString(text, " <num> ")
	text = strNormQuoted.ReplaceAllString(text, " <str> ")
	text = strNormPath.ReplaceAllString(text, " <path> ")
	return strings.Join(strings.Fields(text), " ")
}

// structuralNormalizeCode fingerprints code blocks: drops string/number/hex
// literals and single-line comments, keeping the keyword-and-identifier skeleton.
func structuralNormalizeCode(text, lang string) string {
	text = strings.ToLower(text)
	if prefix := langCommentMap[lang]; prefix != "" {
		var lines []string
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, prefix) {
				continue
			}
			lines = append(lines, trimmed)
		}
		text = strings.Join(lines, "\n")
	}
	text = strNormQuoted.ReplaceAllString(text, " <s> ")
	text = strNormVersion.ReplaceAllString(text, " <v> ")
	text = strNormHexLit.ReplaceAllString(text, " <h> ")
	text = strNormNumber.ReplaceAllString(text, " <n> ")
	return strings.Join(strings.Fields(text), " ")
}

func structuralHash(prefix, normalized string) string {
	h := sha256.Sum256([]byte(prefix + "\x00" + normalized))
	return hex.EncodeToString(h[:16])
}

var (
	strNormURL     = regexp.MustCompile(`https?://[^\s)\]]+`)
	strNormMDLink  = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	strNormMDImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
	strNormCode    = regexp.MustCompile("`[^`]+`")
	strNormVersion = regexp.MustCompile(`\bv?\d+(?:\.\d+){1,3}(?:[.-][a-z0-9]+)*\b`)
	strNormHexLit  = regexp.MustCompile(`\b0x[0-9a-f]+\b`)
	strNormNumber  = regexp.MustCompile(`\b\d+\b`)
	strNormQuoted  = regexp.MustCompile(`"[^"\n]*"|'[^'\n]*'`)
	strNormPath    = regexp.MustCompile(`(?:[\w.-]+/){2,}[\w.-]*`)
)
