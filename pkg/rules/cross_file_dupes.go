package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type CrossFileDupes struct{}

func (r *CrossFileDupes) Name() string { return "strip-cross-file-dupes" }
func (r *CrossFileDupes) Tier() Tier   { return TierAggressive }

func (r *CrossFileDupes) Apply(ctx *Context) (ChangeSet, error) {
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
		sectionText := string(ctx.Source[sec.BodyStart:sec.BodyEnd])
		if wordCount(sectionText) < 15 || wordCount(sectionText) > 600 {
			continue
		}
		normalized := normalizeSectionText(sectionText)
		hash := sectionHash(normalized)

		if ctx.CrossFile.RecordSection(hash, ctx.FilePath, sec.Heading, len(sectionText)) {
			if len(sectionText) < 256 {
				continue
			}
			structHash := structuralHash("section", structuralNormalize(sectionText))
			if canonical, dup := ctx.CrossFile.RecordStructuralSection(structHash, ctx.FilePath, sec.Heading, len(sectionText)); dup {
				refText := "[same as in " + fileBasename(canonical.CanonicalFile) + "]"
				if len(refText)+1 < sec.BodyEnd-sec.BodyStart {
					addReplacement(&changes, sec.BodyStart, sec.BodyEnd, refText+"\n")
				}
			}
			continue
		}

		canonical := ctx.CrossFile.SeenSections[hash]
		refText := "[same as in " + fileBasename(canonical.CanonicalFile) + "]"
		addReplacement(&changes, sec.BodyStart, sec.BodyEnd, refText+"\n")
	}
	return changes, nil
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
