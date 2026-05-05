package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

// ChangelogCompress is a lossless rule that strips tracking IDs and chrome
// from changelog files: PR/issue links, GitHub user-profile links, author
// attributions, and date suffixes in version headings. The actual change
// description on each bullet is preserved.
type ChangelogCompress struct{}

func (r *ChangelogCompress) Name() string { return "compress-changelogs" }
func (r *ChangelogCompress) Tier() Tier   { return TierSafe }

// File-level activation: a file is treated as a changelog if its path
// references "changelog" or "release", or if a top-of-file heading matches
// one of the changelog-section names.
var changelogPathPattern = regexp.MustCompile(`(?i)(^|/)(changelog|change[-_]log|release[-_]?notes|releases|history)([.-]|$)`)

var changelogTopHeadingPattern = regexp.MustCompile(`(?i)^#+\s*(changelog|change log|release notes|releases|history|version history|what'?s new)\s*$`)

// Patterns we strip per line. Each match removes a "tracking" token that an
// LLM gains nothing from. The order matters: more specific patterns run first
// so the broader fallbacks don't eat the wrong text.
var changelogStripPatterns = []*regexp.Regexp{
	// `(@user [#123](url), [#124](url))` — author + multiple PR refs
	regexp.MustCompile(`\s*\(@[\w-]+(?:\s+(?:in\s+)?\[#\d+\]\([^)]+\))+(?:\s*,\s*\[#\d+\]\([^)]+\))*\)`),
	// `([user](https://github.com/user) in [#123](url))` — link author + PR
	regexp.MustCompile(`\s*\(\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\)\s+in\s+\[#\d+\]\([^)]+\)\)`),
	// `([u1](url) & [u2](url) in [#123](url))` — co-authors + PR
	regexp.MustCompile(`\s*\(\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\)(?:\s*[&,]\s*\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\))+\s+in\s+\[#\d+\]\([^)]+\)\)`),
	// `([user1](url) & [user2](url))` or `([user1](url), [user2](url))`
	regexp.MustCompile(`\s*\(\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\)(?:\s*[&,]\s*\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\))+\)`),
	// `([user](https://github.com/user))`
	regexp.MustCompile(`\s*\(\[[\w][\w.-]*\]\(https?://github\.com/[\w-]+/?\)\)`),
	// `([#1234](url))` or `([#1234](url), [#1235](url))`
	regexp.MustCompile(`\s*\(\[#\d+\]\([^)]+\)(?:\s*,\s*\[#\d+\]\([^)]+\))*\)`),
	// `(thanks @user)` / `(thanks to @user)` / `(by @user)` / `(@user)`
	regexp.MustCompile(`\s*\((?:thanks(?:\s+to)?|by|h/t)\s+@[\w-]+(?:\s*,\s*@[\w-]+)*\)`),
	regexp.MustCompile(`\s*\(@[\w-]+(?:\s*,\s*@[\w-]+)*\)`),
	// Bare PR/issue/commit refs at end of line: ` [#1234](url)`
	regexp.MustCompile(`\s+\[#\d+\]\(https?://[^)]+/(?:pull|issues|commit|compare)/[^)]+\)\s*$`),
}

// Date-suffix strippers that run only on version-heading lines. These remove
// trailing "- 2025-01-15", "(Oct 2, 2025)", "(October 1st, 2025)" etc., while
// preserving the version number itself.
var changelogVersionDateStrippers = []*regexp.Regexp{
	// "## 1.2.3 (October 1st, 2025)" / "## 1.2.3 (Oct 2, 2025)"
	regexp.MustCompile(`\s*\(\s*(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)[^)]*\d{4}\s*\)\s*$`),
	// "## 1.2.3 - 2025-01-15" or "## [1.2.3] - 2025-01-15"
	regexp.MustCompile(`\s*[-–—]\s*\d{4}[-/]\d{1,2}[-/]\d{1,2}\s*$`),
	// "## 1.2.3 (2025-01-15)"
	regexp.MustCompile(`\s*\(\s*\d{4}[-/]\d{1,2}[-/]\d{1,2}\s*\)\s*$`),
}

// changelogVersionHeadingPattern matches "## 1.2.3", "## v1.2.3", "## [1.2.3]",
// "## Unreleased", etc. — version heading lines where the date stripper runs.
var changelogVersionHeadingPattern = regexp.MustCompile(`(?i)^#+\s*(\[?v?\d+\.\d+|unreleased|upcoming|next)\b`)

// changelogStandaloneDateLine matches a bare date line like "October 20, 2025"
// often emitted under a version heading.
var changelogStandaloneDateLine = regexp.MustCompile(`^(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)\s+\d{1,2}(?:st|nd|rd|th)?,?\s+\d{4}\s*$`)

func (r *ChangelogCompress) Apply(ctx *Context) (ChangeSet, error) {
	if !looksLikeChangelogFile(ctx) {
		return ChangeSet{}, nil
	}

	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		if line.InFence {
			continue
		}
		text := line.Text
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}

		// Strip trailing date suffix on version-heading lines.
		if changelogVersionHeadingPattern.MatchString(trimmed) {
			for _, pat := range changelogVersionDateStrippers {
				if loc := pat.FindStringIndex(text); loc != nil {
					addRange(&changes, render.Range{
						Start: line.Start + loc[0],
						End:   line.Start + loc[1],
					})
					break
				}
			}
			continue
		}

		// Drop standalone date lines like "October 20, 2025".
		if changelogStandaloneDateLine.MatchString(trimmed) {
			addRange(&changes, render.Range{
				Start: line.Start,
				End:   line.End,
			})
			continue
		}

		// Strip tracking-ID parens from any other line.
		for _, pat := range changelogStripPatterns {
			for _, loc := range pat.FindAllStringIndex(text, -1) {
				addRange(&changes, render.Range{
					Start: line.Start + loc[0],
					End:   line.Start + loc[1],
				})
			}
		}
	}

	return changes, nil
}

func looksLikeChangelogFile(ctx *Context) bool {
	if ctx.FilePath != "" && changelogPathPattern.MatchString(ctx.FilePath) {
		return true
	}
	// Fallback: first heading in the file matches a changelog-section name.
	lines := sourceLines(ctx.Source)
	for i, line := range lines {
		if i > 20 || line.InFence {
			continue
		}
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" {
			continue
		}
		if changelogTopHeadingPattern.MatchString(trimmed) {
			return true
		}
		if strings.HasPrefix(trimmed, "#") {
			// First heading wasn't a changelog heading; not a changelog file.
			return false
		}
	}
	return false
}
