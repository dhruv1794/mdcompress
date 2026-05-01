package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type ChangelogCompress struct{}

func (r *ChangelogCompress) Name() string { return "compress-changelogs" }
func (r *ChangelogCompress) Tier() Tier   { return TierAggressive }

var changelogVersionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(?:#+\s*)?\[?v?(\d+\.\d+[\.\d]*.*?)\]?\s*(?:[-–—]\s*(?:\d{4}[-/]\d{2}[-/]\d{2}|\[\d{4}[-/]\d{2}[-/]\d{2}\]|unreleased|latest)\s*)?$`),
	regexp.MustCompile(`(?i)^(?:#+\s*)?(unreleased|upcoming)\s*$`),
}

var changelogHeadingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(changelog|change log|release notes|releases|history|version history|what's new|whats new)$`),
}

var changelogBulletPrefix = regexp.MustCompile(`^\s*[-*+]\s+`)

func (r *ChangelogCompress) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)

	isChangelogFile := ctx.FilePath != "" && strings.Contains(strings.ToLower(ctx.FilePath), "changelog")
	isReleaseNoteFile := ctx.FilePath != "" && strings.Contains(strings.ToLower(ctx.FilePath), "release")

	var sections []changelogSection
	inChangelog := isChangelogFile || isReleaseNoteFile

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i].Text)
		if lines[i].InFence {
			continue
		}
		if heading, _, ok := detectHeading(trimmed); ok {
			if !inChangelog {
				for _, pattern := range changelogHeadingPatterns {
					if pattern.MatchString(heading) {
						inChangelog = true
						break
					}
				}
			}
			if isVersionHeading(trimmed) {
				sections = append(sections, changelogSection{
					headingLine: i,
					level:       headingLevel(trimmed),
				})
			}
		}
	}

	if len(sections) == 0 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet
	for idx, sectionInfo := range sections {
		bodyStart := sections[idx].headingLine + 1
		bodyEnd := len(lines)
		if idx+1 < len(sections) {
			bodyEnd = sections[idx+1].headingLine
		}

		summary := compactChangelogBody(lines[bodyStart:bodyEnd])
		if summary == "" || summary == "no changes" {
			continue
		}

		if bodyStart < bodyEnd {
			replacement := summary + "\n"
			addReplacement(&changes,
				lines[bodyStart].Start,
				lines[bodyEnd-1].End,
				replacement)
		}

		_ = sectionInfo
	}

	return changes, nil
}

func compactChangelogBody(lines []sourceLine) string {
	var bullets []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || line.InFence {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			break
		}
		if matches := changelogBulletPrefix.FindString(trimmed); matches != "" {
			content := strings.TrimSpace(trimmed[len(matches):])
			if content == "" {
				continue
			}
			content = strings.Map(func(r rune) rune {
				if r == '\n' {
					return ' '
				}
				return r
			}, content)
			content = strings.TrimSpace(content)
			if wordCount(content) >= 1 {
				bullets = append(bullets, content)
			}
		}
	}

	if len(bullets) == 0 {
		return "no changes"
	}

	out := make([]string, 0)
	for i, b := range bullets {
		if len(out) >= 5 {
			out = append(out, "...")
			break
		}
		if len(b) > 100 {
			b = b[:97] + "..."
		}
		out = append(out, b)
		_ = i
	}
	return strings.Join(out, "; ")
}

func isVersionHeading(trimmed string) bool {
	for _, pattern := range changelogVersionPatterns {
		if pattern.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func headingLevel(trimmed string) int {
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	return level
}

type changelogSection struct {
	headingLine int
	level       int
}
