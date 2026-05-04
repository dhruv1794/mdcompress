package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

type CrossFileCodeBlocks struct{}

func (r *CrossFileCodeBlocks) Name() string { return "dedup-cross-file-code-blocks" }
func (r *CrossFileCodeBlocks) Tier() Tier   { return TierAggressive }

func (r *CrossFileCodeBlocks) Apply(ctx *Context) (ChangeSet, error) {
	if ctx.CrossFile == nil || ctx.FilePath == "" {
		return ChangeSet{}, nil
	}

	lines := sourceLines(ctx.Source)
	blocks := fencedBlocks(lines)
	if len(blocks) == 0 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet
	for _, block := range blocks {
		if len(block.Content) == 0 {
			continue
		}
		lang := fenceLanguage(lines[block.StartLine].Text)
		normalized := normalizeCrossFileCode(extractTexts(block.Content), lang)
		if len(normalized) < 20 {
			continue
		}

		contentStart := block.Content[0].Start
		contentEnd := block.Content[len(block.Content)-1].End
		contentLength := contentEnd - contentStart
		hash := crossFileCodeHash(lang, normalized)
		canonical, firstForFile := ctx.CrossFile.RecordCodeBlock(hash, ctx.FilePath, block.StartLine+1, contentLength)
		if firstForFile {
			if len(normalized) < 80 {
				continue
			}
			structHash := structuralHash("code:"+lang, structuralNormalizeCode(normalized, lang))
			if structCanon, dup := ctx.CrossFile.RecordStructuralCodeBlock(structHash, ctx.FilePath, block.StartLine+1, contentLength); dup {
				replacement := "[same as " + fileBasename(structCanon.CanonicalFile) + ":" + strconv.Itoa(structCanon.StartLine) + "]\n"
				if len(replacement) < contentLength {
					addReplacement(&changes, contentStart, contentEnd, replacement)
				}
			}
			continue
		}

		replacement := "[same as " + fileBasename(canonical.CanonicalFile) + ":" + strconv.Itoa(canonical.StartLine) + "]\n"
		if len(replacement) >= contentLength {
			continue
		}
		addReplacement(&changes, contentStart, contentEnd, replacement)
	}
	return changes, nil
}

func normalizeCrossFileCode(lines []string, lang string) string {
	commentPrefix := langCommentMap[lang]
	var normalized []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if commentPrefix != "" && strings.HasPrefix(line, commentPrefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		normalized = append(normalized, strings.Join(fields, " "))
	}
	return strings.Join(normalized, "\n")
}

func crossFileCodeHash(lang, normalized string) string {
	h := sha256.Sum256([]byte(lang + "\n" + normalized))
	return hex.EncodeToString(h[:16])
}
