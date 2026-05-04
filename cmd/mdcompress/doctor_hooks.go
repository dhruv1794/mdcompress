package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	assets "github.com/dhruv1794/mdcompress/internal"
	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/manifest"
)

const (
	doctorOK   = "OK"
	doctorWarn = "WARN"
	doctorFail = "FAIL"
)

type doctorCheck struct {
	Status string
	Name   string
	Detail string
	Fix    string
}

func diagnoseRepo() []doctorCheck {
	checks := []doctorCheck{
		checkConfig(),
		checkHooks(),
		checkAgentHints(),
		checkCacheFreshness(),
		checkManifestConsistency(),
		checkPath(),
		checkGitignoredMarkdown(),
	}
	return checks
}

func fixDoctor() ([]string, error) {
	var fixes []string

	if !fileExists(".mdcompress/config.yaml") {
		if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
			return fixes, err
		}
		if err := writeFileIfMissing(".mdcompress/config.yaml", []byte(defaultConfigYAML)); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "wrote .mdcompress/config.yaml")
	}

	hooks := checkHooks()
	if hooks.Status != doctorOK {
		if err := installHooks(); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "installed mdcompress hooks")
	}

	agents := checkAgentHints()
	if agents.Status != doctorOK {
		if err := appendAgentHint("AGENTS.md", true); err != nil {
			return fixes, err
		}
		if err := appendAgentHint("CLAUDE.md", false); err != nil {
			return fixes, err
		}
		if err := installSkill(); err != nil {
			return fixes, err
		}
		if err := appendCursorHints(); err != nil {
			return fixes, err
		}
		if err := appendWindsurfHints(); err != nil {
			return fixes, err
		}
		if err := appendContinueHint(); err != nil {
			return fixes, err
		}
		if err := appendAiderHint(); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "restored agent hints")
	}

	cacheFresh := checkCacheFreshness()
	manifestConsistent := checkManifestConsistency()
	if cacheFresh.Status != doctorOK || manifestConsistent.Status != doctorOK {
		if _, err := runMarkdown(runOptions{
			Args:     []string{"."},
			All:      true,
			Compress: compressionOptionsFromConfig(".mdcompress/config.yaml"),
		}); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "rebuilt markdown cache and manifest")
	}

	return fixes, nil
}

func checkConfig() doctorCheck {
	const path = ".mdcompress/config.yaml"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: ".mdcompress/config.yaml is missing", Fix: "run `mdcompress init` or `mdcompress doctor --fix`"}
	}
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: err.Error(), Fix: "inspect file permissions and rerun doctor"}
	}

	text := string(data)
	if !strings.Contains(text, "version: 1") {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: "missing or unsupported version", Fix: "rewrite .mdcompress/config.yaml from `mdcompress init` defaults"}
	}
	tier := configTier(text)
	if _, err := compress.ParseTier(tier); err != nil {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: err.Error(), Fix: "set tier to safe, aggressive, or llm"}
	}
	return doctorCheck{Status: doctorOK, Name: "config", Detail: "valid .mdcompress/config.yaml"}
}

func configTier(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "tier:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "tier:"))
			return strings.Trim(value, `"'`)
		}
	}
	return "safe"
}

func checkHooks() doctorCheck {
	switch {
	case dirExists(".husky"):
		return checkHookFile("hooks", ".husky/pre-commit", ".husky/post-merge")
	case fileExists(".pre-commit-config.yaml"):
		text, err := readText(".pre-commit-config.yaml")
		if err != nil {
			return doctorCheck{Status: doctorFail, Name: "hooks", Detail: err.Error(), Fix: "inspect .pre-commit-config.yaml permissions"}
		}
		if strings.Contains(text, "# mdcompress") && strings.Contains(text, "mdcompress-staged") && strings.Contains(text, "mdcompress-post-merge") {
			return doctorCheck{Status: doctorOK, Name: "hooks", Detail: "pre-commit config includes mdcompress hooks"}
		}
		return doctorCheck{Status: doctorFail, Name: "hooks", Detail: "pre-commit config missing mdcompress hook entries", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	case fileExists("lefthook.yml"):
		return checkLefthookFile("lefthook.yml")
	case fileExists("lefthook.yaml"):
		return checkLefthookFile("lefthook.yaml")
	default:
		return checkHookFile("hooks", filepath.Join(".git", "hooks", "pre-commit"), filepath.Join(".git", "hooks", "post-merge"))
	}
}

func checkHookFile(name, preCommitPath, postMergePath string) doctorCheck {
	preCommit, err := readText(preCommitPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: name, Detail: preCommitPath + " is missing", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	}
	postMerge, err := readText(postMergePath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: name, Detail: postMergePath + " is missing", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	}
	if strings.Contains(preCommit, "# mdcompress") && strings.Contains(preCommit, "mdcompress run --staged --quiet") &&
		strings.Contains(postMerge, "# mdcompress") && strings.Contains(postMerge, "mdcompress run --changed --quiet") {
		return doctorCheck{Status: doctorOK, Name: name, Detail: "mdcompress pre-commit and post-merge hooks installed"}
	}
	return doctorCheck{Status: doctorFail, Name: name, Detail: "mdcompress hook block is missing or outdated", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
}

func checkLefthookFile(path string) doctorCheck {
	text, err := readText(path)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "hooks", Detail: err.Error(), Fix: "inspect lefthook config permissions"}
	}
	if strings.Contains(text, "# mdcompress pre-commit") && strings.Contains(text, "# mdcompress post-merge") &&
		strings.Contains(text, "mdcompress run --staged --quiet") && strings.Contains(text, "mdcompress run --changed --quiet") {
		return doctorCheck{Status: doctorOK, Name: "hooks", Detail: path + " includes mdcompress commands"}
	}
	return doctorCheck{Status: doctorFail, Name: "hooks", Detail: path + " missing mdcompress commands", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
}

func checkAgentHints() doctorCheck {
	files := agentHintFiles()
	if len(files) == 0 {
		return doctorCheck{Status: doctorWarn, Name: "agent hints", Detail: "no mdcompress agent hints found", Fix: "run `mdcompress init --agents=...` or `mdcompress doctor --fix`"}
	}
	sort.Strings(files)
	return doctorCheck{Status: doctorOK, Name: "agent hints", Detail: "found in " + strings.Join(files, ", ")}
}

func agentHintFiles() []string {
	var files []string
	for _, path := range []string{"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules", ".aider.conf.yml"} {
		text, err := readText(path)
		if err == nil && (strings.Contains(text, "## mdcompress") || strings.Contains(text, "# mdcompress")) {
			files = append(files, path)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(".cursor", "rules", "*.mdc"))
	for _, path := range matches {
		text, err := readText(path)
		if err == nil && strings.Contains(text, "## mdcompress") {
			files = append(files, filepath.ToSlash(path))
		}
	}
	continueConfig := filepath.Join(".continue", "config.json")
	text, err := readText(continueConfig)
	if err == nil && strings.Contains(text, continueHintMarker) {
		files = append(files, filepath.ToSlash(continueConfig))
	}
	return files
}

func checkCacheFreshness() doctorCheck {
	sources, err := markdownPaths([]string{"."})
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: err.Error(), Fix: "inspect markdown source paths"}
	}
	m, err := manifest.Read(manifest.DefaultPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: "manifest cannot be read: " + err.Error(), Fix: "run `mdcompress run --all` or `mdcompress doctor --fix`"}
	}
	var missing []string
	var stale []string
	for _, path := range sources {
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: err.Error(), Fix: "inspect markdown source paths"}
		}
		rel = filepath.ToSlash(rel)
		entry, ok := m.Entries[rel]
		if !ok || !mdcache.Exists(entry.Cache) {
			missing = append(missing, rel)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			stale = append(stale, rel)
			continue
		}
		if !freshManifestEntry(path, entry, mdcache.SourceSHA(content), true) {
			stale = append(stale, rel)
		}
	}
	if len(missing) == 0 && len(stale) == 0 {
		return doctorCheck{Status: doctorOK, Name: "cache freshness", Detail: fmt.Sprintf("%d markdown file(s) have fresh cache mirrors", len(sources))}
	}
	return doctorCheck{
		Status: doctorFail,
		Name:   "cache freshness",
		Detail: fmt.Sprintf("%d missing, %d stale cache mirror(s)", len(missing), len(stale)),
		Fix:    "run `mdcompress run --all` or `mdcompress doctor --fix`",
	}
}

func checkManifestConsistency() doctorCheck {
	m, err := manifest.Read(manifest.DefaultPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "manifest", Detail: err.Error(), Fix: "run `mdcompress run --all` or `mdcompress doctor --fix`"}
	}
	var problems []string
	for source, entry := range m.Entries {
		if entry.Source != source {
			problems = append(problems, source+" source mismatch")
		}
		if entry.Cache == "" || !mdcache.Exists(entry.Cache) {
			problems = append(problems, source+" cache missing")
		}
		if _, err := os.Stat(source); err != nil {
			problems = append(problems, source+" source missing")
		}
	}
	if len(problems) == 0 {
		return doctorCheck{Status: doctorOK, Name: "manifest", Detail: fmt.Sprintf("%d manifest entries consistent with cache", len(m.Entries))}
	}
	return doctorCheck{
		Status: doctorFail,
		Name:   "manifest",
		Detail: fmt.Sprintf("%d inconsistent manifest entrie(s)", len(problems)),
		Fix:    "run `mdcompress run --all` or `mdcompress doctor --fix`",
	}
}

func checkPath() doctorCheck {
	if _, err := exec.LookPath("mdcompress"); err == nil {
		return doctorCheck{Status: doctorOK, Name: "PATH", Detail: "mdcompress is available to hooks"}
	}
	return doctorCheck{Status: doctorWarn, Name: "PATH", Detail: "mdcompress was not found on PATH", Fix: "install mdcompress somewhere on PATH before relying on hooks"}
}

func checkGitignoredMarkdown() doctorCheck {
	paths, err := markdownPaths([]string{"."})
	if err != nil {
		return doctorCheck{Status: doctorWarn, Name: "gitignored markdown", Detail: err.Error(), Fix: "inspect markdown source paths"}
	}
	var ignored []string
	for _, path := range paths {
		if gitCheckIgnore(path) {
			ignored = append(ignored, filepath.ToSlash(path))
		}
	}
	if len(ignored) == 0 {
		return doctorCheck{Status: doctorOK, Name: "gitignored markdown", Detail: "no source markdown files are gitignored"}
	}
	sort.Strings(ignored)
	return doctorCheck{
		Status: doctorWarn,
		Name:   "gitignored markdown",
		Detail: fmt.Sprintf("%d source markdown file(s) are gitignored", len(ignored)),
		Fix:    "remove intentional source docs from .gitignore or run mdcompress on them manually when needed",
	}
}

func gitCheckIgnore(path string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", path)
	err := cmd.Run()
	return err == nil
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func installHooks() error {
	switch {
	case dirExists(".husky"):
		return installHuskyHooks()
	case fileExists(".pre-commit-config.yaml"):
		return appendMarkedText(".pre-commit-config.yaml", "# mdcompress", preCommitFrameworkBlock)
	case fileExists("lefthook.yml"):
		return installLefthookHooks("lefthook.yml")
	case fileExists("lefthook.yaml"):
		return installLefthookHooks("lefthook.yaml")
	default:
		return installDirectGitHooks()
	}
}

func installDirectGitHooks() error {
	if info, err := os.Stat(".git"); err != nil || !info.IsDir() {
		return fmt.Errorf("not a git repository: .git directory not found")
	}
	if err := os.MkdirAll(filepath.Join(".git", "hooks"), 0o755); err != nil {
		return err
	}
	if err := appendMarkedBlock(filepath.Join(".git", "hooks", "pre-commit"), assets.PreCommitHook); err != nil {
		return err
	}
	return appendMarkedBlock(filepath.Join(".git", "hooks", "post-merge"), assets.PostMergeHook)
}

func installHuskyHooks() error {
	if err := appendMarkedBlock(filepath.Join(".husky", "pre-commit"), huskyPreCommitBlock); err != nil {
		return err
	}
	return appendMarkedBlock(filepath.Join(".husky", "post-merge"), huskyPostMergeBlock)
}

func installLefthookHooks(path string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(current)
	text = upsertLefthookCommand(text, "pre-commit", "# mdcompress pre-commit", "mdcompress run --staged --quiet")
	text = upsertLefthookCommand(text, "post-merge", "# mdcompress post-merge", "mdcompress run --changed --quiet")
	if text == string(current) {
		return nil
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func upsertLefthookCommand(text, section, marker, run string) string {
	if strings.Contains(text, marker) {
		return text
	}

	lines := strings.Split(text, "\n")
	sectionStart := -1
	sectionEnd := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == section+":" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			sectionStart = i
			break
		}
	}

	block := []string{
		section + ":",
		"  commands:",
		"    " + marker,
		"    mdcompress:",
		"      run: " + run,
	}
	if sectionStart == -1 {
		return appendYAMLBlock(text, block)
	}

	for i := sectionStart + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			sectionEnd = i
			break
		}
	}

	commandsIndex := -1
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if strings.TrimSpace(lines[i]) == "commands:" && strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "    ") {
			commandsIndex = i
			break
		}
	}

	var insert []string
	if commandsIndex == -1 {
		insert = []string{
			"  commands:",
			"    " + marker,
			"    mdcompress:",
			"      run: " + run,
		}
	} else {
		insert = []string{
			"    " + marker,
			"    mdcompress:",
			"      run: " + run,
		}
	}

	next := make([]string, 0, len(lines)+len(insert))
	next = append(next, lines[:sectionEnd]...)
	next = append(next, insert...)
	next = append(next, lines[sectionEnd:]...)
	return strings.Join(next, "\n")
}

func appendYAMLBlock(text string, block []string) string {
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	return text + strings.Join(block, "\n") + "\n"
}

func installSkill() error {
	path := filepath.Join(".claude", "skills", "mdcompress", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(assets.Skill), 0o644)
}

func appendMarkedBlock(path, block string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(current), "# mdcompress") {
		return os.Chmod(path, 0o755)
	}

	var next []byte
	if len(current) > 0 {
		next = append(next, current...)
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(strings.TrimRight(block, "\n"))...)
	next = append(next, '\n')
	if err := os.WriteFile(path, next, 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

func appendMarkedText(path, marker, block string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(current), marker) {
		return nil
	}

	var next []byte
	if len(current) > 0 {
		next = append(next, current...)
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(strings.TrimRight(block, "\n"))...)
	next = append(next, '\n')
	return os.WriteFile(path, next, 0o644)
}

func writeFileIfMissing(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func appendLinesOnce(path string, lines []string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := make(map[string]bool)
	for _, line := range strings.Split(string(current), "\n") {
		existing[strings.TrimSpace(line)] = true
	}

	var additions []string
	for _, line := range lines {
		if !existing[line] {
			additions = append(additions, line)
		}
	}
	if len(additions) == 0 {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 && !strings.HasSuffix(string(next), "\n") {
		next = append(next, '\n')
	}
	for _, line := range additions {
		next = append(next, []byte(line)...)
		next = append(next, '\n')
	}
	return os.WriteFile(path, next, 0o644)
}

func appendAgentHint(path string, create bool) error {
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if !create {
			return nil
		}
		current = nil
	} else if err != nil {
		return err
	}
	if strings.Contains(string(current), "## mdcompress") {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 {
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(agentHint)...)
	return os.WriteFile(path, next, 0o644)
}

func appendCursorHints() error {
	var paths []string
	if fileExists(".cursorrules") {
		paths = append(paths, ".cursorrules")
	}

	matches, err := filepath.Glob(filepath.Join(".cursor", "rules", "*.mdc"))
	if err != nil {
		return err
	}
	sort.Strings(matches)
	paths = append(paths, matches...)

	for _, path := range paths {
		if err := appendExistingHint(path, "## mdcompress", cursorAgentHint); err != nil {
			return err
		}
	}
	return nil
}

func appendWindsurfHints() error {
	if !fileExists(".windsurfrules") {
		return nil
	}
	return appendExistingHint(".windsurfrules", "## mdcompress", windsurfAgentHint)
}

func appendAiderHint() error {
	if !fileExists(".aider.conf.yml") {
		return nil
	}
	return appendExistingHint(".aider.conf.yml", "# mdcompress", aiderAgentHint)
}

// appendContinueHint merges the mdcompress hint into a Continue config.json.
// JSON-merge: parse, update only the systemMessage field, preserve other keys.
// Idempotent via a marker substring inside systemMessage.
func appendContinueHint() error {
	path := filepath.Join(".continue", "config.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	config := make(map[string]json.RawMessage)
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}

	var systemMessage string
	if existing, ok := config["systemMessage"]; ok && len(existing) > 0 {
		// systemMessage may legitimately be a string or null; ignore decode
		// errors for non-string types (e.g. number/object), treating them as
		// "unset" so we don't clobber unrelated user data.
		_ = json.Unmarshal(existing, &systemMessage)
	}

	if strings.Contains(systemMessage, continueHintMarker) {
		return nil
	}

	if systemMessage == "" {
		systemMessage = continueAgentHint
	} else {
		trimmed := strings.TrimRight(systemMessage, "\n")
		systemMessage = trimmed + "\n\n" + continueAgentHint
	}

	encoded, err := json.Marshal(systemMessage)
	if err != nil {
		return err
	}
	config["systemMessage"] = encoded

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func appendExistingHint(path, marker, hint string) error {
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.Contains(string(current), marker) {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 {
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(hint)...)
	return os.WriteFile(path, next, 0o644)
}

type agentName string

const (
	agentClaude   agentName = "claude"
	agentCodex    agentName = "codex"
	agentCursor   agentName = "cursor"
	agentWindsurf agentName = "windsurf"
	agentContinue agentName = "continue"
	agentAider    agentName = "aider"
)

var allAgents = []agentName{
	agentClaude, agentCodex, agentCursor, agentWindsurf, agentContinue, agentAider,
}

func parseAgents(input string) (map[agentName]bool, error) {
	valid := make(map[agentName]bool, len(allAgents))
	for _, a := range allAgents {
		valid[a] = true
	}

	if strings.TrimSpace(input) == "" {
		return valid, nil
	}

	selection := make(map[agentName]bool)
	for _, raw := range strings.Split(input, ",") {
		name := agentName(strings.ToLower(strings.TrimSpace(raw)))
		if name == "" {
			continue
		}
		if !valid[name] {
			return nil, fmt.Errorf("unknown agent %q (valid: claude, codex, cursor, windsurf, continue, aider)", name)
		}
		selection[name] = true
	}
	return selection, nil
}

const agentHint = `## mdcompress
This repo uses mdcompress. For any *.md file you read, prefer the version
at .mdcompress/cache/<same-relative-path> if it exists because it is a
token-optimized mirror of the original maintained automatically.
`

const cursorAgentHint = `## mdcompress
When reading any *.md file in this repo, prefer
.mdcompress/cache/<same-relative-path> when it exists. It is a
token-optimized mirror of the original maintained automatically.
`

const windsurfAgentHint = `## mdcompress
When reading any *.md file in this repo, prefer
.mdcompress/cache/<same-relative-path> when it exists. It is a
token-optimized mirror of the original maintained automatically.
`

const aiderAgentHint = `# mdcompress
# When reading any *.md file in this repo, prefer
# .mdcompress/cache/<same-relative-path> when it exists. It is a
# token-optimized mirror of the original maintained automatically.
`

const continueHintMarker = "mdcompress: prefer .mdcompress/cache"

const continueAgentHint = "mdcompress: prefer .mdcompress/cache/<same-relative-path> over the original *.md when it exists. It is a token-optimized mirror of the original maintained automatically."

const huskyPreCommitBlock = `# mdcompress
if command -v mdcompress >/dev/null 2>&1; then
  mdcompress run --staged --quiet
else
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
fi
`

const huskyPostMergeBlock = `# mdcompress
if command -v mdcompress >/dev/null 2>&1; then
  mdcompress run --changed --quiet
else
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
fi
`

const preCommitFrameworkBlock = `# mdcompress
- repo: local
  hooks:
    - id: mdcompress-staged
      name: mdcompress staged markdown cache
      entry: mdcompress run --staged --quiet
      language: system
      pass_filenames: false
      stages: [pre-commit]
    - id: mdcompress-post-merge
      name: mdcompress changed markdown cache
      entry: mdcompress run --changed --quiet
      language: system
      pass_filenames: false
      stages: [post-merge]
`
