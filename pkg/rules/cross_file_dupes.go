package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type CrossFileDupes struct{}

func (r *CrossFileDupes) Name() string { return "strip-cross-file-dupes" }
func (r *CrossFileDupes) Tier() Tier   { return TierAggressive }

var boilerplateHeadingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(contribut(?:ing|e|ors?)|how to (?:contribute|help))$`),
	regexp.MustCompile(`(?i)^(license|copyright and license|license information)$`),
	regexp.MustCompile(`(?i)^(support|getting help|need help\??|questions\??|getting support|help and support)$`),
	regexp.MustCompile(`(?i)^(code of conduct|code of conduct and guidelines|coc)$`),
	regexp.MustCompile(`(?i)^(installation|installing|getting started|quick start|setup)$`),
	regexp.MustCompile(`(?i)^(acknowledg(?:ements?|ments?)|credits|thanks|thank you)$`),
	regexp.MustCompile(`(?i)^(security|security policy|reporting (?:a )?(?:security )?(?:vulnerabilit(?:y|ies)|issues?|bugs?)|responsible disclosure)$`),
	regexp.MustCompile(`(?i)^(community|join (?:the )?community|our community)$`),
	regexp.MustCompile(`(?i)^(sponsors?|backers?|funding|financial support)$`),
}

func (r *CrossFileDupes) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	if ctx.CrossFile == nil || ctx.FilePath == "" {
		return ChangeSet{}, nil
	}

	lines := sourceLines(ctx.Source)
	sections := extractSections(lines)
	if len(sections) == 0 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet
	for _, sec := range sections {
		if !isBoilerplateHeading(sec.Heading) {
			continue
		}
		sectionText := string(ctx.Source[sec.BodyStart:sec.BodyEnd])
		if wordCount(sectionText) < 15 || wordCount(sectionText) > 600 {
			continue
		}
		normalized := normalizeSectionText(sectionText)
		hash := sectionHash(normalized)

		if ctx.CrossFile.RecordSection(hash, ctx.FilePath, sec.Heading, len(sectionText)) {
			continue
		}

		canonical := ctx.CrossFile.SeenSections[hash]
		refText := "[same as in " + fileBasename(canonical.CanonicalFile) + "]"
		addReplacement(&changes, sec.BodyStart, sec.BodyEnd, refText+"\n")
	}
	return changes, nil
}

func isBoilerplateHeading(heading string) bool {
	for _, pattern := range boilerplateHeadingPatterns {
		if pattern.MatchString(heading) {
			return true
		}
	}
	return false
}

type section struct {
	Heading   string
	Level     int
	BodyStart int
	BodyEnd   int
}

func extractSections(lines []sourceLine) []section {
	var sections []section
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i].Text)
		heading, level, ok := detectHeading(trimmed)
		if !ok || lines[i].InFence {
			continue
		}
		bodyStart := lines[i].End
		bodyEnd := sectionEndAt(lines, i, level)
		if bodyEnd <= bodyStart {
			continue
		}
		sections = append(sections, section{
			Heading:   heading,
			Level:     level,
			BodyStart: bodyStart,
			BodyEnd:   bodyEnd,
		})
	}
	return sections
}

func detectHeading(trimmed string) (string, int, bool) {
	if !strings.HasPrefix(trimmed, "#") {
		return "", 0, false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
		return "", 0, false
	}
	return strings.TrimSpace(trimmed[level+1:]), level, true
}

func sectionEndAt(lines []sourceLine, headingIdx, headingLevel int) int {
	for i := headingIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i].Text)
		if trimmed == "" || lines[i].InFence {
			continue
		}
		if _, level, ok := detectHeading(trimmed); ok {
			if level <= headingLevel {
				return lines[i].Start
			}
		}
	}
	if headingIdx+1 < len(lines) {
		lastLine := lines[len(lines)-1]
		if lastLine.End > lastLine.Start {
			return lastLine.End
		}
	}
	return len(lines)
}

func normalizeSectionText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

func sectionHash(normalized string) string {
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:16])
}

func fileBasename(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}
