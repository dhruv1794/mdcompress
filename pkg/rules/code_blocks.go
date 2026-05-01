package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type CodeBlocks struct{}

func (r *CodeBlocks) Name() string { return "compress-code-blocks" }
func (r *CodeBlocks) Tier() Tier   { return TierSafe }

var (
	shellPromptRe  = regexp.MustCompile(`^(\$\s*|>\s*|#\s+)(.*)`)
	shellShebangRe = regexp.MustCompile(`^#!/`)
	shellSetRe     = regexp.MustCompile(`^set\s+[-+][a-zA-Z]`)
	shellSourceRe  = regexp.MustCompile(`^(source\s+|\.\s+\S)`)

	goSingleImportRe   = regexp.MustCompile(`^import\s+"[^"]*"$`)
	goParenImportStart = regexp.MustCompile(`^import\s*\($`)
	goParenImportEnd   = regexp.MustCompile(`^\)$`)
	goErrCheckRe       = regexp.MustCompile(`^\s*if\s+err\s*!=\s*nil\s*\{?\s*$`)
	goErrReturnRe      = regexp.MustCompile(`^\s*return\s+(nil,\s*)?\w*err\w*\s*$`)
	goLogFatalRe       = regexp.MustCompile(`^\s*log\.\w*(Fatal|Panic)\(`)
	goPanicRe          = regexp.MustCompile(`^\s*panic\(`)

	pythonImportRe  = regexp.MustCompile(`^(import\s+\S|from\s+\S+\s+import)`)
	pythonMainRe    = regexp.MustCompile(`^if\s+__name__\s*==\s*['"]__main__['"]\s*:`)
	pythonShebangRe = regexp.MustCompile(`^#!/usr/bin/(env\s+)?python`)

	jsImportRe     = regexp.MustCompile(`^(import\s+|export\s+)`)
	jsRequireRe    = regexp.MustCompile(`^(const|var|let)\s+\w+\s*=\s*require\(`)
	jsModuleRe     = regexp.MustCompile(`^module\.exports\s*=`)
	tsTypeImportRe = regexp.MustCompile(`^import\s+type\s+`)

	javaImportRe = regexp.MustCompile(`^(import\s+|package\s+)\S`)

	csIncludeRe = regexp.MustCompile(`^#include\s*[<"]`)
	csUsingRe   = regexp.MustCompile(`^using\s+(namespace|static)\s+`)
	csPragmaRe  = regexp.MustCompile(`^#pragma\s`)

	cHeaderBlockStart = regexp.MustCompile(`^#ifndef\s`)
	cHeaderBlockEnd   = regexp.MustCompile(`^#endif`)
	cHeaderDefine     = regexp.MustCompile(`^#define\s`)

	rustUseRe    = regexp.MustCompile(`^use\s+\S`)
	rustExternRe = regexp.MustCompile(`^extern\s+crate\s+`)
	rustModRe    = regexp.MustCompile(`^mod\s+\S`)
	rustDeriveRe = regexp.MustCompile(`^#\[derive\(`)

	rubyRequireRe    = regexp.MustCompile(`^require\s+['"]`)
	rubyRequireRelRe = regexp.MustCompile(`^require_relative\s+['"]`)
	rubyIncludeRe    = regexp.MustCompile(`^include\s+\S`)
	rubyExtendRe     = regexp.MustCompile(`^extend\s+\S`)

	phpUseRe       = regexp.MustCompile(`^use\s+\S+\s*;?$`)
	phpNamespaceRe = regexp.MustCompile(`^namespace\s+\S+\s*;?$`)
	phpRequireRe   = regexp.MustCompile(`^(require|include)(_once)?\s+`)

	swiftImportRe = regexp.MustCompile(`^import\s+\S+$`)
	swiftDylibRe  = regexp.MustCompile(`^@_exported`)

	dockerCommentRe   = regexp.MustCompile(`^\s*#`)
	dockerLabelRe     = regexp.MustCompile(`^LABEL\s+`)
	dockerMaintainRe  = regexp.MustCompile(`^MAINTAINER\s+`)

	makeCommentRe = regexp.MustCompile(`^\s*#`)
	makeIfRe      = regexp.MustCompile(`^\s*(ifeq|ifneq|ifdef|ifndef|else|endif)`)

	yamlCommentRe = regexp.MustCompile(`^\s*#`)
	iniCommentRe  = regexp.MustCompile(`^\s*[#;]`)

	terraformCommentRe = regexp.MustCompile(`^\s*#`)
	terraformBlockRe   = regexp.MustCompile(`^\s*(terraform|provider)\s*\{`)

	protobufImportRe = regexp.MustCompile(`^import\s+"[^"]*"`)

	graphqlImportRe = regexp.MustCompile(`^#import\s+`)

	csLineCommentRe = regexp.MustCompile(`^\s*//`)
	hashCommentRe   = regexp.MustCompile(`^\s*#`)
	sqlCommentRe    = regexp.MustCompile(`^\s*--\s`)
	csBlockStartRe  = regexp.MustCompile(`/\*`)
	csBlockEndRe    = regexp.MustCompile(`\*/`)
	pyDocStringRe   = regexp.MustCompile(`^\s*(?:'''|""")`)

	sudoRe        = regexp.MustCompile(`^\s*sudo\s+`)
	npmSaveRe     = regexp.MustCompile(`\s+--save(-dev|-optional)?\b`)
	cmdContinuationRe = regexp.MustCompile(`\s+\\\s*$`)
)

func (r *CodeBlocks) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
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
	end := len(lines)
	if end > 10 {
		end = 10
	}
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
		if lang == "ini" || lang == "cfg" || lang == "conf" || lang == "properties" || lang == "editorconfig" || lang == "gitconfig" {
			if iniCommentRe.MatchString(line) {
				lines[i] = ""
				changed = true
			}
		} else if lang == "makefile" || lang == "mk" {
			if makeCommentRe.MatchString(line) || makeIfRe.MatchString(trimmed) {
				lines[i] = ""
				changed = true
			}
		} else {
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

func stripImports(lines []string, lang string) bool {
	switch lang {
	case "golang", "go":
		return stripGoImports(lines)
	case "python", "py", "py3":
		return stripPythonImports(lines)
	case "javascript", "js", "typescript", "ts", "jsx", "tsx", "mjs", "cjs":
		return stripJSImports(lines)
	case "java", "scala", "kotlin":
		return stripJavaImports(lines)
	case "c", "cpp", "c++", "h", "hpp", "cxx", "cc":
		return stripCImports(lines)
	case "rust", "rs":
		return stripRustImports(lines)
	case "ruby", "rb":
		return stripRubyImports(lines)
	case "php":
		return stripPHPImports(lines)
	case "swift":
		return stripSwiftImports(lines)
	case "proto", "protobuf":
		return stripProtoImports(lines)
	}
	return false
}

func stripGoImports(lines []string) bool {
	changed := false
	inParenImport := false
	for i, line := range lines {
		if inParenImport {
			changed = true
			if goParenImportEnd.MatchString(strings.TrimSpace(line)) {
				inParenImport = false
			}
			lines[i] = ""
			continue
		}
		if goParenImportStart.MatchString(strings.TrimSpace(line)) {
			lines[i] = ""
			inParenImport = true
			changed = true
			continue
		}
		if goSingleImportRe.MatchString(strings.TrimSpace(line)) {
			lines[i] = ""
			changed = true
			continue
		}
		if strings.TrimSpace(line) == "package main" || strings.HasPrefix(strings.TrimSpace(line), "package ") {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripPythonImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if pythonImportRe.MatchString(trimmed) || pythonMainRe.MatchString(trimmed) || pythonShebangRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripJSImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if jsImportRe.MatchString(trimmed) || jsRequireRe.MatchString(trimmed) || jsModuleRe.MatchString(trimmed) || tsTypeImportRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripJavaImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		if javaImportRe.MatchString(strings.TrimSpace(line)) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripCImports(lines []string) bool {
	changed := false
	inIfndef := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if csIncludeRe.MatchString(trimmed) || csUsingRe.MatchString(trimmed) || csPragmaRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
			continue
		}
		if cHeaderBlockStart.MatchString(trimmed) {
			inIfndef = true
			lines[i] = ""
			changed = true
			continue
		}
		if inIfndef {
			if trimmed == "" || cHeaderDefine.MatchString(trimmed) {
				lines[i] = ""
				changed = true
				continue
			}
			if cHeaderBlockEnd.MatchString(trimmed) {
				lines[i] = ""
				changed = true
				inIfndef = false
			}
		}
	}
	return changed
}

func stripRustImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if rustUseRe.MatchString(trimmed) || rustExternRe.MatchString(trimmed) || rustModRe.MatchString(trimmed) || rustDeriveRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripRubyImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if rubyRequireRe.MatchString(trimmed) || rubyRequireRelRe.MatchString(trimmed) || rubyIncludeRe.MatchString(trimmed) || rubyExtendRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripPHPImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if phpUseRe.MatchString(trimmed) || phpNamespaceRe.MatchString(trimmed) || phpRequireRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripSwiftImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if swiftImportRe.MatchString(trimmed) || swiftDylibRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripProtoImports(lines []string) bool {
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if protobufImportRe.MatchString(trimmed) {
			lines[i] = ""
			changed = true
		}
	}
	return changed
}

func stripErrorBoilerplate(lines []string, lang string) bool {
	if lang != "go" && lang != "golang" {
		return false
	}
	changed := false
	for i := 0; i < len(lines); i++ {
		if !goErrCheckRe.MatchString(strings.TrimSpace(lines[i])) {
			continue
		}
		changed = true
		lines[i] = ""
		if i+1 < len(lines) && goErrReturnRe.MatchString(strings.TrimSpace(lines[i+1])) {
			lines[i+1] = ""
			i++
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
