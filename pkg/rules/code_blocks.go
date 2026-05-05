package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

// CodeBlocks is the engine for compressing fenced code blocks. Per-language
// rules live in code_blocks_lang.go.
type CodeBlocks struct{}

func (r *CodeBlocks) Name() string { return "compress-code-blocks" }
func (r *CodeBlocks) Tier() Tier   { return TierSafe }

func (r *CodeBlocks) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	blocks := fencedBlocks(lines)
	if len(blocks) == 0 {
		return ChangeSet{}, nil
	}

	var changes ChangeSet
	seenHashes := make(map[string]bool)
	blockLangs := make([]string, len(blocks))
	blockHashes := make([]string, len(blocks))
	blockModified := make([]bool, len(blocks))

	for i, block := range blocks {
		if len(block.Content) == 0 {
			continue
		}

		lang := fenceLanguage(lines[block.StartLine].Text)
		blockLangs[i] = lang

		contentStrs := extractTexts(block.Content)
		origHash := contentHash(contentStrs)
		blockHashes[i] = origHash

		if ctx.Config.Tier >= TierAggressive && seenHashes[origHash] {
			blockModified[i] = true
			addReplacement(&changes,
				block.Content[0].Start, block.Content[len(block.Content)-1].End,
				"[duplicate — same as block above]\n")
			continue
		}

		modified := false
		modified = stripShellPrompts(contentStrs, lang) || modified
		modified = stripConfigComments(contentStrs, lang) || modified

		if ctx.Config.Tier >= TierAggressive {
			modified = stripImports(contentStrs, lang) || modified
			modified = stripErrorBoilerplate(contentStrs, lang) || modified
			modified = stripCodeComments(contentStrs, lang) || modified
		}

		if modified {
			blockModified[i] = true
			cleaned := compactLines(contentStrs)
			if len(cleaned) == 0 {
				addRange(&changes, render.Range{Start: block.Start, End: block.End})
			} else {
				replacement := strings.Join(cleaned, "\n") + "\n"
				addReplacement(&changes,
					block.Content[0].Start, block.Content[len(block.Content)-1].End,
					replacement)
			}
		}

		if ctx.Config.Tier >= TierAggressive {
			seenHashes[origHash] = true
		}
	}

	if ctx.Config.Tier >= TierAggressive && len(blocks) > 1 {
		dedupConsecutive(blocks, blockModified, blockLangs, blockHashes, &changes)
	}

	return changes, nil
}

func extractTexts(lines []sourceLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text
	}
	return out
}

func fenceLanguage(fenceText string) string {
	trimmed := strings.TrimSpace(fenceText)
	marker, _ := fencedCodeMarker([]byte(trimmed))
	if marker == 0 {
		return ""
	}
	rest := strings.TrimLeft(trimmed, string(marker))
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}

func contentHash(lines []string) string {
	h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(h[:8])
}

func compactLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func stripShellPrompts(lines []string, lang string) bool {
	if !isShellLang(lang) && !hasShellPrompts(lines) {
		return false
	}
	changed := false
	for i, line := range lines {
		if m := shellPromptRe.FindStringSubmatch(line); m != nil {
			lines[i] = m[2]
			changed = true
			continue
		}
		if lang == "" && !shellShebangRe.MatchString(line) {
			continue
		}
		if shellShebangRe.MatchString(line) || shellSetRe.MatchString(line) || shellSourceRe.MatchString(line) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func isShellLang(lang string) bool {
	switch lang {
	case "sh", "bash", "zsh", "fish", "ksh", "shell", "console", "terminal", "powershell", "pwsh":
		return true
	default:
		return false
	}
}

func hasShellPrompts(lines []string) bool {
	prompts := 0
	end := min(len(lines), 10)
	for i := 0; i < end; i++ {
		if shellPromptRe.MatchString(lines[i]) {
			prompts++
		}
	}
	return prompts >= 2
}

func stripConfigComments(lines []string, lang string) bool {
	switch lang {
	case "yaml", "yml", "toml", "ini", "cfg", "conf", "properties", "env", "editorconfig", "gitconfig",
		"dockerfile", "docker", "makefile", "mk", "tf", "hcl", "terraform":
	default:
		return false
	}
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch lang {
		case "ini", "cfg", "conf", "properties", "editorconfig", "gitconfig":
			if iniCommentRe.MatchString(line) {
				lines[i] = ""
				changed = true
			}
		case "makefile", "mk":
			if makeCommentRe.MatchString(line) || makeIfRe.MatchString(trimmed) {
				lines[i] = ""
				changed = true
			}
		default:
			if yamlCommentRe.MatchString(line) {
				lines[i] = ""
				changed = true
			}
		}
		if lang == "dockerfile" || lang == "docker" {
			if dockerLabelRe.MatchString(trimmed) || dockerMaintainRe.MatchString(trimmed) {
				lines[i] = ""
				changed = true
			}
		}
	}
	return changed
}

func dedupConsecutive(blocks []fencedBlock, modified []bool, langs []string, hashes []string, changes *ChangeSet) {
	for i := 1; i < len(blocks); i++ {
		if modified[i] || modified[i-1] {
			continue
		}
		if len(blocks[i].Content) == 0 || len(blocks[i-1].Content) == 0 {
			continue
		}
		if langs[i] == "" || langs[i-1] == "" {
			continue
		}
		if langs[i] == langs[i-1] {
			continue
		}
		if hashes[i] != hashes[i-1] {
			continue
		}
		prevLang := langs[i-1]
		replacement := "[identical to " + prevLang + " example above]\n"
		addReplacement(changes,
			blocks[i].Content[0].Start, blocks[i].Content[len(blocks[i].Content)-1].End,
			replacement)
		modified[i] = true
	}
}

func stripCodeComments(lines []string, lang string) bool {
	if lang == "" {
		return false
	}

	usesCComments := false
	usesHashComments := false
	usesSQLComments := false
	usesDocstrings := false

	switch lang {
	case "golang", "go", "rust", "rs", "java", "scala", "kotlin",
		"javascript", "js", "typescript", "ts", "jsx", "tsx",
		"c", "cpp", "c++", "h", "hpp", "swift", "php", "cs", "dart":
		usesCComments = true
	case "python", "py", "py3", "ruby", "rb",
		"sh", "bash", "zsh", "yaml", "yml",
		"r", "perl", "pl":
		usesHashComments = true
	case "sql", "lua", "haskell", "hs":
		usesSQLComments = true
	}

	if lang == "python" || lang == "py" || lang == "py3" {
		usesDocstrings = true
	}

	if !usesCComments && !usesHashComments && !usesSQLComments && !usesDocstrings {
		return false
	}

	changed := false
	inBlockComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if inBlockComment {
			changed = true
			lines[i] = ""
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
			}
			continue
		}

		if usesCComments {
			if csBlockStartRe.MatchString(trimmed) {
				if !csBlockEndRe.MatchString(trimmed) {
					inBlockComment = true
				}
				changed = true
				lines[i] = ""
				continue
			}
			if csLineCommentRe.MatchString(trimmed) {
				changed = true
				lines[i] = ""
				continue
			}
		}

		if usesHashComments && hashCommentRe.MatchString(trimmed) {
			changed = true
			lines[i] = ""
			continue
		}

		if usesSQLComments && sqlCommentRe.MatchString(trimmed) {
			changed = true
			lines[i] = ""
			continue
		}

		if usesDocstrings && pyDocStringRe.MatchString(trimmed) {
			inBlockComment = !inBlockComment
			changed = true
			lines[i] = ""
			continue
		}
	}

	return changed
}
