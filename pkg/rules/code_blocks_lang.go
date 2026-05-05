package rules

import (
	"regexp"
	"strings"
)

// Per-language regex bundles for compress-code-blocks. Each language adds a
// stripImports* helper; new languages should land here so the engine stays
// generic.

var (
	// Shell.
	shellPromptRe  = regexp.MustCompile(`^(\$\s*|>\s*|#\s+)(.*)`)
	shellShebangRe = regexp.MustCompile(`^#!/`)
	shellSetRe     = regexp.MustCompile(`^set\s+[-+][a-zA-Z]`)
	shellSourceRe  = regexp.MustCompile(`^(source\s+|\.\s+\S)`)

	// Go.
	goSingleImportRe   = regexp.MustCompile(`^import\s+"[^"]*"$`)
	goParenImportStart = regexp.MustCompile(`^import\s*\($`)
	goParenImportEnd   = regexp.MustCompile(`^\)$`)
	goErrCheckRe       = regexp.MustCompile(`^\s*if\s+err\s*!=\s*nil\s*\{?\s*$`)
	goErrReturnRe      = regexp.MustCompile(`^\s*return\s+(nil,\s*)?\w*err\w*\s*$`)

	// Python.
	pythonImportRe  = regexp.MustCompile(`^(import\s+\S|from\s+\S+\s+import)`)
	pythonMainRe    = regexp.MustCompile(`^if\s+__name__\s*==\s*['"]__main__['"]\s*:`)
	pythonShebangRe = regexp.MustCompile(`^#!/usr/bin/(env\s+)?python`)

	// JS / TS.
	jsImportRe     = regexp.MustCompile(`^(import\s+|export\s+)`)
	jsRequireRe    = regexp.MustCompile(`^(const|var|let)\s+\w+\s*=\s*require\(`)
	jsModuleRe     = regexp.MustCompile(`^module\.exports\s*=`)
	tsTypeImportRe = regexp.MustCompile(`^import\s+type\s+`)

	// Java / Scala / Kotlin.
	javaImportRe = regexp.MustCompile(`^(import\s+|package\s+)\S`)

	// C / C++.
	csIncludeRe       = regexp.MustCompile(`^#include\s*[<"]`)
	csUsingRe         = regexp.MustCompile(`^using\s+(namespace|static)\s+`)
	csPragmaRe        = regexp.MustCompile(`^#pragma\s`)
	cHeaderBlockStart = regexp.MustCompile(`^#ifndef\s`)
	cHeaderBlockEnd   = regexp.MustCompile(`^#endif`)
	cHeaderDefine     = regexp.MustCompile(`^#define\s`)

	// Rust.
	rustUseRe    = regexp.MustCompile(`^use\s+\S`)
	rustExternRe = regexp.MustCompile(`^extern\s+crate\s+`)
	rustModRe    = regexp.MustCompile(`^mod\s+\S`)
	rustDeriveRe = regexp.MustCompile(`^#\[derive\(`)

	// Ruby.
	rubyRequireRe    = regexp.MustCompile(`^require\s+['"]`)
	rubyRequireRelRe = regexp.MustCompile(`^require_relative\s+['"]`)
	rubyIncludeRe    = regexp.MustCompile(`^include\s+\S`)
	rubyExtendRe     = regexp.MustCompile(`^extend\s+\S`)

	// PHP.
	phpUseRe       = regexp.MustCompile(`^use\s+\S+\s*;?$`)
	phpNamespaceRe = regexp.MustCompile(`^namespace\s+\S+\s*;?$`)
	phpRequireRe   = regexp.MustCompile(`^(require|include)(_once)?\s+`)

	// Swift.
	swiftImportRe = regexp.MustCompile(`^import\s+\S+$`)
	swiftDylibRe  = regexp.MustCompile(`^@_exported`)

	// Dockerfile / config.
	dockerLabelRe    = regexp.MustCompile(`^LABEL\s+`)
	dockerMaintainRe = regexp.MustCompile(`^MAINTAINER\s+`)

	// Makefile.
	makeCommentRe = regexp.MustCompile(`^\s*#`)
	makeIfRe      = regexp.MustCompile(`^\s*(ifeq|ifneq|ifdef|ifndef|else|endif)`)

	// Generic config comments.
	yamlCommentRe = regexp.MustCompile(`^\s*#`)
	iniCommentRe  = regexp.MustCompile(`^\s*[#;]`)

	// Protobuf.
	protobufImportRe = regexp.MustCompile(`^import\s+"[^"]*"`)

	// Generic comment shapes used by stripCodeComments.
	csLineCommentRe = regexp.MustCompile(`^\s*//`)
	hashCommentRe   = regexp.MustCompile(`^\s*#`)
	sqlCommentRe    = regexp.MustCompile(`^\s*--\s`)
	csBlockStartRe  = regexp.MustCompile(`/\*`)
	csBlockEndRe    = regexp.MustCompile(`\*/`)
	pyDocStringRe   = regexp.MustCompile(`^\s*(?:'''|""")`)
)

// stripImports dispatches to a language-specific stripper. Returns true when
// any line was modified.
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
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
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
