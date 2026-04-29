package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Tool struct {
	Name        string
	ConfigFile  string
	Found       bool
	Suggestions []string
}

type Report struct {
	Tools []Tool
}

func Analyze(root string) (Report, error) {
	var report Report
	abs, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	report.Tools = append(report.Tools, detectMarkdownlint(abs))
	report.Tools = append(report.Tools, detectVale(abs))
	report.Tools = append(report.Tools, detectPrettier(abs))
	report.Tools = append(report.Tools, detectRemark(abs))
	return report, nil
}

func detectMarkdownlint(root string) Tool {
	t := Tool{Name: "markdownlint", ConfigFile: ".markdownlint-cli2.yaml"}
	for _, f := range []string{".markdownlint-cli2.yaml", ".markdownlint-cli2.jsonc", ".markdownlint.json", ".markdownlint.yaml", ".markdownlint.yml"} {
		if fileExists(filepath.Join(root, f)) {
			t.ConfigFile = f
			t.Found = true
			break
		}
	}
	if !t.Found {
		return t
	}
	hasLint := execCommand("which", "markdownlint")
	t.Suggestions = append(t.Suggestions,
		"mdcompress strip-html-comments (~ markdownlint MD033 no-inline-html)",
	)
	if !hasLint {
		t.Suggestions = append(t.Suggestions,
			"mdcompress does not enforce Markdown style rules (MD013, MD041, etc.); it strips fluff. Keep markdownlint for style enforcement.",
		)
	}
	return t
}

func detectVale(root string) Tool {
	t := Tool{Name: "vale", ConfigFile: ".vale.ini"}
	for _, f := range []string{".vale.ini"} {
		if fileExists(filepath.Join(root, f)) {
			t.Found = true
			break
		}
	}
	if !t.Found {
		valeDir := filepath.Join(root, ".vale")
		if info, err := os.Stat(valeDir); err == nil && info.IsDir() {
			t.Found = true
		}
	}
	if !t.Found {
		valeDir := filepath.Join(root, "_vale")
		if info, err := os.Stat(valeDir); err == nil && info.IsDir() {
			t.Found = true
		}
	}
	if !t.Found {
		return t
	}
	t.Suggestions = append(t.Suggestions,
		"mdcompress strip-hedging-phrases (~ Vale prose lint rules for wordiness)",
		"mdcompress strip-marketing-prose (~ Vale brand/style rules for superlatives)",
		"mdcompress is complementary to Vale: Vale enforces style/grammar; mdcompress strips fluff pre-compression.",
		"Run Vale first for style, then mdcompress for token reduction.",
	)
	return t
}

func detectPrettier(root string) Tool {
	t := Tool{Name: "prettier", ConfigFile: ".prettierrc"}
	for _, f := range []string{".prettierrc", ".prettierrc.json", ".prettierrc.yaml", ".prettierrc.yml", ".prettierrc.js", "prettier.config.js", "prettier.config.cjs"} {
		if fileExists(filepath.Join(root, f)) {
			t.ConfigFile = f
			t.Found = true
			break
		}
	}
	if !t.Found {
		pkgJSON := filepath.Join(root, "package.json")
		if data, err := os.ReadFile(pkgJSON); err == nil && strings.Contains(string(data), `"prettier"`) {
			t.ConfigFile = "package.json"
			t.Found = true
		}
	}
	if !t.Found {
		return t
	}
	t.Suggestions = append(t.Suggestions,
		"mdcompress and Prettier serve different purposes: Prettier formats; mdcompress strips fluff.",
		"Safe to run both. Run Prettier first (format), then mdcompress (compress).",
	)
	return t
}

func detectRemark(root string) Tool {
	t := Tool{Name: "remark-lint", ConfigFile: ".remarkrc"}
	for _, f := range []string{".remarkrc", ".remarkrc.json", ".remarkrc.yaml", ".remarkrc.yml", ".remarkrc.js"} {
		if fileExists(filepath.Join(root, f)) {
			t.ConfigFile = f
			t.Found = true
			break
		}
	}
	if !t.Found {
		pkgJSON := filepath.Join(root, "package.json")
		if data, err := os.ReadFile(pkgJSON); err == nil && strings.Contains(string(data), `"remark"`) {
			t.ConfigFile = "package.json"
			t.Found = true
		}
	}
	if !t.Found {
		return t
	}
	t.Suggestions = append(t.Suggestions,
		"remark-lint and mdcompress can coexist: remark enforces rules; mdcompress strips fluff.",
		"Consider disabling remark lint rules that conflict with mdcompress (e.g., no-html when strip-html-comments renders the rule moot).",
	)
	return t
}

func GenerateConfig(report Report, tier string) string {
	if tier == "" {
		tier = "aggressive"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# mdcompress config (auto-suggested by `mdcompress migrate`)\n"))
	b.WriteString("version: 1\n")
	b.WriteString(fmt.Sprintf("tier: %s\n", tier))
	b.WriteString("rules:\n")
	b.WriteString("  disabled:\n")
	b.WriteString("    - dedup-cross-section\n")
	b.WriteString("    - collapse-example-output\n")
	return b.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func execCommand(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	return cmd.Run() == nil
}
