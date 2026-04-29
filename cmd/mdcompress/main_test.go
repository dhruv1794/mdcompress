package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/manifest"
)

func TestAppendCursorHintsNoFiles(t *testing.T) {
	chdirTemp(t)

	if err := appendCursorHints(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(".cursorrules"); !os.IsNotExist(err) {
		t.Fatalf("appendCursorHints created .cursorrules unexpectedly: %v", err)
	}
}

func TestAppendCursorHintsAppendsCursorrulesOnce(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(".cursorrules", []byte("Always answer concisely.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendCursorHints(); err != nil {
		t.Fatal(err)
	}
	if err := appendCursorHints(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".cursorrules")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Always answer concisely.") {
		t.Fatalf("existing content was not preserved:\n%s", text)
	}
	if count := strings.Count(text, "## mdcompress"); count != 1 {
		t.Fatalf("mdcompress hint count = %d, want 1:\n%s", count, text)
	}
}

func TestAppendCursorHintsAppendsMDCFiles(t *testing.T) {
	chdirTemp(t)
	rulesDir := filepath.Join(".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(rulesDir, "docs.mdc"),
		filepath.Join(rulesDir, "go.mdc"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("existing cursor rule\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := appendCursorHints(); err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "existing cursor rule") {
			t.Fatalf("existing content was not preserved in %s:\n%s", path, text)
		}
		if count := strings.Count(text, "## mdcompress"); count != 1 {
			t.Fatalf("mdcompress hint count in %s = %d, want 1:\n%s", path, count, text)
		}
	}
}

func TestAppendWindsurfHintsNoFile(t *testing.T) {
	chdirTemp(t)

	if err := appendWindsurfHints(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(".windsurfrules"); !os.IsNotExist(err) {
		t.Fatalf("appendWindsurfHints created .windsurfrules unexpectedly: %v", err)
	}
}

func TestAppendWindsurfHintsAppendsOnce(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(".windsurfrules", []byte("Use project conventions.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendWindsurfHints(); err != nil {
		t.Fatal(err)
	}
	if err := appendWindsurfHints(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".windsurfrules")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Use project conventions.") {
		t.Fatalf("existing content was not preserved:\n%s", text)
	}
	if count := strings.Count(text, "## mdcompress"); count != 1 {
		t.Fatalf("mdcompress hint count = %d, want 1:\n%s", count, text)
	}
}

func TestAppendAiderHintNoFile(t *testing.T) {
	chdirTemp(t)

	if err := appendAiderHint(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(".aider.conf.yml"); !os.IsNotExist(err) {
		t.Fatalf("appendAiderHint created .aider.conf.yml unexpectedly: %v", err)
	}
}

func TestAppendAiderHintAppendsOnce(t *testing.T) {
	chdirTemp(t)
	original := "model: gpt-4\nedit-format: diff\n"
	if err := os.WriteFile(".aider.conf.yml", []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendAiderHint(); err != nil {
		t.Fatal(err)
	}
	if err := appendAiderHint(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".aider.conf.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "model: gpt-4") {
		t.Fatalf("existing config keys were not preserved:\n%s", text)
	}
	if count := strings.Count(text, "# mdcompress\n"); count != 1 {
		t.Fatalf("aider hint count = %d, want 1:\n%s", count, text)
	}
}

func TestAppendContinueHintNoFile(t *testing.T) {
	chdirTemp(t)

	if err := appendContinueHint(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(".continue", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("appendContinueHint created config.json unexpectedly: %v", err)
	}
}

func TestAppendContinueHintAddsSystemMessageWhenAbsent(t *testing.T) {
	chdirTemp(t)
	writeContinueConfig(t, map[string]any{
		"models": []any{map[string]any{"title": "GPT-4", "provider": "openai"}},
	})

	if err := appendContinueHint(); err != nil {
		t.Fatal(err)
	}

	got := readContinueConfig(t)
	msg, _ := got["systemMessage"].(string)
	if !strings.Contains(msg, continueHintMarker) {
		t.Fatalf("systemMessage missing mdcompress hint: %q", msg)
	}
	if _, ok := got["models"]; !ok {
		t.Fatalf("user key 'models' was dropped: %#v", got)
	}
}

func TestAppendContinueHintAppendsToExistingSystemMessage(t *testing.T) {
	chdirTemp(t)
	writeContinueConfig(t, map[string]any{
		"systemMessage": "You are concise.",
		"models":        []any{map[string]any{"title": "GPT-4"}},
	})

	if err := appendContinueHint(); err != nil {
		t.Fatal(err)
	}

	got := readContinueConfig(t)
	msg, _ := got["systemMessage"].(string)
	if !strings.HasPrefix(msg, "You are concise.") {
		t.Fatalf("existing systemMessage not preserved: %q", msg)
	}
	if !strings.Contains(msg, continueHintMarker) {
		t.Fatalf("mdcompress hint not appended: %q", msg)
	}
}

func TestAppendContinueHintIsIdempotent(t *testing.T) {
	chdirTemp(t)
	writeContinueConfig(t, map[string]any{
		"systemMessage": "You are concise.",
	})

	for i := range 3 {
		if err := appendContinueHint(); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	got := readContinueConfig(t)
	msg, _ := got["systemMessage"].(string)
	if count := strings.Count(msg, continueHintMarker); count != 1 {
		t.Fatalf("hint appears %d times, want 1: %q", count, msg)
	}
}

func TestAppendContinueHintPreservesUserKeys(t *testing.T) {
	chdirTemp(t)
	original := map[string]any{
		"models":               []any{map[string]any{"title": "Local", "provider": "ollama"}},
		"tabAutocompleteModel": map[string]any{"title": "Auto", "provider": "ollama"},
		"customCommands":       []any{map[string]any{"name": "test", "prompt": "hi"}},
	}
	writeContinueConfig(t, original)

	if err := appendContinueHint(); err != nil {
		t.Fatal(err)
	}

	got := readContinueConfig(t)
	for key, want := range original {
		gotVal, ok := got[key]
		if !ok {
			t.Errorf("user key %q dropped", key)
			continue
		}
		if !reflect.DeepEqual(gotVal, want) {
			t.Errorf("user key %q changed: got %#v, want %#v", key, gotVal, want)
		}
	}
}

func TestAppendContinueHintMalformedJSONErrors(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".continue", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".continue", "config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendContinueHint(); err == nil {
		t.Fatalf("expected error parsing malformed JSON, got nil")
	}
}

func TestInstallHooksUsesHuskyWhenPresent(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".husky", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".husky", "pre-commit"), []byte("#!/bin/sh\nnpm test\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(); err != nil {
		t.Fatal(err)
	}

	preCommit := readFile(t, filepath.Join(".husky", "pre-commit"))
	if !strings.Contains(preCommit, "npm test") {
		t.Fatalf("existing Husky pre-commit content was not preserved:\n%s", preCommit)
	}
	if count := strings.Count(preCommit, "# mdcompress"); count != 1 {
		t.Fatalf("Husky pre-commit marker count = %d, want 1:\n%s", count, preCommit)
	}
	if !strings.Contains(preCommit, "mdcompress run --staged --quiet") {
		t.Fatalf("Husky pre-commit missing staged command:\n%s", preCommit)
	}

	postMerge := readFile(t, filepath.Join(".husky", "post-merge"))
	if count := strings.Count(postMerge, "# mdcompress"); count != 1 {
		t.Fatalf("Husky post-merge marker count = %d, want 1:\n%s", count, postMerge)
	}
	if !strings.Contains(postMerge, "mdcompress run --changed --quiet") {
		t.Fatalf("Husky post-merge missing changed command:\n%s", postMerge)
	}
}

func TestInstallHooksUsesPreCommitConfigWhenPresent(t *testing.T) {
	chdirTemp(t)
	original := "repos:\n  - repo: https://github.com/pre-commit/pre-commit-hooks\n"
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(); err != nil {
		t.Fatal(err)
	}

	text := readFile(t, ".pre-commit-config.yaml")
	if !strings.Contains(text, "pre-commit/pre-commit-hooks") {
		t.Fatalf("existing pre-commit config was not preserved:\n%s", text)
	}
	if count := strings.Count(text, "# mdcompress"); count != 1 {
		t.Fatalf("pre-commit marker count = %d, want 1:\n%s", count, text)
	}
	for _, want := range []string{"mdcompress-staged", "mdcompress-post-merge", "stages: [pre-commit]", "stages: [post-merge]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("pre-commit config missing %q:\n%s", want, text)
		}
	}
}

func TestInstallHooksUsesLefthookWhenPresent(t *testing.T) {
	chdirTemp(t)
	original := "pre-commit:\n  commands:\n    test:\n      run: go test ./...\n"
	if err := os.WriteFile("lefthook.yml", []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(); err != nil {
		t.Fatal(err)
	}

	text := readFile(t, "lefthook.yml")
	if !strings.Contains(text, "test:\n      run: go test ./...") {
		t.Fatalf("existing lefthook command was not preserved:\n%s", text)
	}
	if count := strings.Count(text, "# mdcompress pre-commit"); count != 1 {
		t.Fatalf("lefthook pre-commit marker count = %d, want 1:\n%s", count, text)
	}
	if count := strings.Count(text, "# mdcompress post-merge"); count != 1 {
		t.Fatalf("lefthook post-merge marker count = %d, want 1:\n%s", count, text)
	}
	if !strings.Contains(text, "mdcompress run --staged --quiet") || !strings.Contains(text, "mdcompress run --changed --quiet") {
		t.Fatalf("lefthook config missing mdcompress commands:\n%s", text)
	}
}

func TestInstallHooksFallsBackToDirectGitHooks(t *testing.T) {
	chdirTemp(t)
	if _, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(); err != nil {
		t.Fatal(err)
	}

	preCommit := readFile(t, filepath.Join(".git", "hooks", "pre-commit"))
	if count := strings.Count(preCommit, "# mdcompress"); count != 1 {
		t.Fatalf("direct pre-commit marker count = %d, want 1:\n%s", count, preCommit)
	}
	postMerge := readFile(t, filepath.Join(".git", "hooks", "post-merge"))
	if count := strings.Count(postMerge, "# mdcompress"); count != 1 {
		t.Fatalf("direct post-merge marker count = %d, want 1:\n%s", count, postMerge)
	}
}

func TestCheckConfigReportsInvalidTier(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".mdcompress", "config.yaml"), []byte("version: 1\ntier: bogus\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	check := checkConfig()
	if check.Status != doctorFail {
		t.Fatalf("Status = %s, want %s: %#v", check.Status, doctorFail, check)
	}
	if !strings.Contains(check.Detail, "unknown tier") {
		t.Fatalf("Detail missing tier error: %#v", check)
	}
}

func TestReadEvalConfig(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".mdcompress", "config.yaml"), []byte(`version: 1
tier: safe
eval:
  backend: openai
  model: eval-model
  endpoint: http://127.0.0.1:8080/v1
  api_key_env: TEST_OPENAI_KEY
  threshold: 0.91
  questions_per_doc: 7
  seeds: 3
hooks:
  pre_commit: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := readEvalConfig(filepath.Join(".mdcompress", "config.yaml"))
	if cfg.Backend != "openai" || cfg.Model != "eval-model" || cfg.Endpoint != "http://127.0.0.1:8080/v1" || cfg.APIKeyEnv != "TEST_OPENAI_KEY" {
		t.Fatalf("unexpected string config: %#v", cfg)
	}
	if cfg.Threshold != 0.91 || cfg.QuestionsPerDoc != 7 || cfg.Seeds != 3 {
		t.Fatalf("unexpected numeric config: %#v", cfg)
	}
}

func TestReadProjectConfigRulesAndTier(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".mdcompress", "config.yaml"), []byte(`version: 1
tier: aggressive
rules:
  enabled:
    - collapse-example-output
  disabled:
    - dedup-cross-section
eval:
  backend: ollama
  model: llama3.1:8b
  threshold: 0.95
  questions_per_doc: 10
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := readProjectConfig(filepath.Join(".mdcompress", "config.yaml"))
	if cfg.TierName != "aggressive" {
		t.Fatalf("TierName = %q, want aggressive", cfg.TierName)
	}
	if !reflect.DeepEqual(cfg.EnabledRules, []string{"collapse-example-output"}) {
		t.Fatalf("EnabledRules = %#v", cfg.EnabledRules)
	}
	if !reflect.DeepEqual(cfg.DisabledRules, []string{"dedup-cross-section"}) {
		t.Fatalf("DisabledRules = %#v", cfg.DisabledRules)
	}
	if cfg.Eval.Backend != "ollama" || cfg.Eval.Model != "llama3.1:8b" || cfg.Eval.Threshold != 0.95 || cfg.Eval.QuestionsPerDoc != 10 {
		t.Fatalf("Eval = %#v", cfg.Eval)
	}
}

func TestDefaultConfigYAMLUsesAggressiveWithRiskyRulesDisabled(t *testing.T) {
	for _, want := range []string{
		"tier: aggressive",
		"    - dedup-cross-section",
		"    - collapse-example-output",
		"  threshold: 0.95",
		"  questions_per_doc: 10",
	} {
		if !strings.Contains(defaultConfigYAML, want) {
			t.Fatalf("defaultConfigYAML missing %q:\n%s", want, defaultConfigYAML)
		}
	}
}

func TestCompressionOptionsFromConfigUsesAggressiveTierAndDisabledDefaults(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".mdcompress", "config.yaml"), []byte(defaultConfigYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := compressionOptionsFromConfig(filepath.Join(".mdcompress", "config.yaml"))
	if opts.Tier != compress.TierAggressive {
		t.Fatalf("Tier = %s, want aggressive", opts.Tier)
	}
	if !reflect.DeepEqual(opts.DisabledRules, []string{"dedup-cross-section", "collapse-example-output"}) {
		t.Fatalf("DisabledRules = %#v", opts.DisabledRules)
	}
}

func TestEvalBackendRequiresExplicitModelForHostedProviders(t *testing.T) {
	for _, backend := range []string{"openai", "anthropic"} {
		if _, err := evalBackend(backend, "", "", "TEST_KEY"); err == nil || !strings.Contains(err.Error(), "requires --model") {
			t.Fatalf("%s missing model error = %v", backend, err)
		}
	}
}

func TestEvalCommandHelpDocumentsRepoAndRule(t *testing.T) {
	var out strings.Builder
	cmd := evalCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"eval [--repo=<path>] [--rule=<name>]",
		"mdcompress eval --repo=.",
		"mdcompress eval --repo=docs --rule=strip-toc",
		"--backend=openai --model=gpt-4o-mini",
		"Use --rule to isolate one registered rule",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("help output missing %q:\n%s", want, text)
		}
	}
}

func TestStatusCommandShowsV2Summary(t *testing.T) {
	chdirTemp(t)
	if _, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("git", "remote", "add", "origin", "https://example.com/acme/widgets.git").CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".mdcompress", "config.yaml"), []byte("version: 1\ntier: safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}

	sources := map[string][]byte{
		"README.md":         []byte("# Project\n\n<!-- hidden -->\n\nContent.\n"),
		"docs/guide.md":     []byte("# Guide\n\nContent.\n"),
		"docs/unchanged.md": []byte("# Unchanged\n\nNo savings.\n"),
	}
	for path, content := range sources {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range sources {
		if _, err := mdcache.Write(mdcache.DefaultDir, path, content); err != nil {
			t.Fatal(err)
		}
	}

	m := manifest.New()
	m.Entries["README.md"] = manifest.Entry{
		Source:       "README.md",
		Cache:        filepath.ToSlash(filepath.Join(".mdcompress", "cache", "README.md")),
		SHA256:       mdcache.SourceSHA(sources["README.md"]),
		TokensBefore: 7000,
		TokensAfter:  4500,
	}
	m.Entries["docs/guide.md"] = manifest.Entry{
		Source:       "docs/guide.md",
		Cache:        filepath.ToSlash(filepath.Join(".mdcompress", "cache", "docs", "guide.md")),
		SHA256:       mdcache.SourceSHA(sources["docs/guide.md"]),
		TokensBefore: 3000,
		TokensAfter:  2500,
	}
	m.Entries["docs/unchanged.md"] = manifest.Entry{
		Source:       "docs/unchanged.md",
		Cache:        filepath.ToSlash(filepath.Join(".mdcompress", "cache", "docs", "unchanged.md")),
		SHA256:       mdcache.SourceSHA(sources["docs/unchanged.md"]),
		TokensBefore: 1000,
		TokensAfter:  1000,
	}
	if err := manifest.Write(manifest.DefaultPath, m); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	cmd := statusCommand()
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Repo: https://example.com/acme/widgets.git",
		"Tier: safe",
		"Files tracked: 3",
		"Cache: 3 fresh, 0 stale, 0 missing",
		"Tokens before: 11,000",
		"Tokens after:  8,000",
		"Saved:         3,000 (27.3%)",
		"ROI estimate (Claude Sonnet input pricing, $3.00/MTok):",
		"Per full-cache read: ~$0.0090",
		"README.md                        2,500 saved (35.7%)",
		"docs/guide.md                    500 saved (16.7%)",
		"per clone, not shared team-wide",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "docs/unchanged.md") {
		t.Fatalf("top savings included zero-savings file:\n%s", text)
	}
}

func TestRunMarkdownBareRunRefreshesStaleMTime(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("README.md", []byte("# Project\n\n<!-- hidden -->\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	cachePath, err := mdcache.Write(mdcache.DefaultDir, "README.md", source)
	if err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(cachePath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes("README.md", newTime, newTime); err != nil {
		t.Fatal(err)
	}

	m := manifest.New()
	m.Entries["README.md"] = manifest.Entry{
		Source:       "README.md",
		Cache:        cachePath,
		SHA256:       mdcache.SourceSHA(source),
		TokensBefore: 10,
		TokensAfter:  10,
	}
	if err := manifest.Write(manifest.DefaultPath, m); err != nil {
		t.Fatal(err)
	}

	summary, err := runMarkdown(runOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Compressed != 1 || summary.Skipped != 0 {
		t.Fatalf("summary = %#v, want 1 compressed and 0 skipped", summary)
	}
	cacheData := readFile(t, cachePath)
	if strings.Contains(cacheData, "hidden") {
		t.Fatalf("stale cache was not refreshed:\n%s", cacheData)
	}
}

func TestRunMarkdownNoStaleCheckSkipsBareAutoRefresh(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("README.md", []byte("# Project\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := runMarkdown(runOptions{NoStaleCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Compressed != 0 || summary.Skipped != 0 {
		t.Fatalf("summary = %#v, want no work", summary)
	}
	if fileExists(manifest.DefaultPath) {
		t.Fatalf("--no-stale-check bare run wrote %s", manifest.DefaultPath)
	}
	if fileExists(filepath.Join(".mdcompress", "cache", "README.md")) {
		t.Fatalf("--no-stale-check bare run wrote cache")
	}
}

func TestFixDoctorRepairsFreshGitRepo(t *testing.T) {
	chdirTemp(t)
	if _, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("README.md", []byte("# Project\n\n<!-- hidden -->\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixes, err := fixDoctor()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"wrote .mdcompress/config.yaml",
		"installed mdcompress hooks",
		"restored agent hints",
		"rebuilt markdown cache and manifest",
	} {
		if !containsString(fixes, want) {
			t.Fatalf("fixes missing %q: %#v", want, fixes)
		}
	}

	for _, path := range []string{
		".mdcompress/config.yaml",
		filepath.Join(".git", "hooks", "pre-commit"),
		filepath.Join(".git", "hooks", "post-merge"),
		"AGENTS.md",
		filepath.Join(".mdcompress", "cache", "README.md"),
		filepath.Join(".mdcompress", "manifest.json"),
	} {
		if !fileExists(path) {
			t.Fatalf("expected %s to exist", path)
		}
	}

	statusByName := make(map[string]string)
	for _, check := range diagnoseRepo() {
		statusByName[check.Name] = check.Status
	}
	for _, name := range []string{"config", "hooks", "agent hints", "cache freshness", "manifest"} {
		if statusByName[name] != doctorOK {
			t.Fatalf("%s status = %s, want %s; all statuses: %#v", name, statusByName[name], doctorOK, statusByName)
		}
	}
}

func TestParseAgentsDefaultIsAll(t *testing.T) {
	got, err := parseAgents("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range allAgents {
		if !got[a] {
			t.Errorf("default selection missing %q", a)
		}
	}
}

func TestParseAgentsExplicitSubset(t *testing.T) {
	got, err := parseAgents("claude, cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !got[agentClaude] || !got[agentCursor] {
		t.Errorf("missing requested agents: %#v", got)
	}
	if got[agentWindsurf] || got[agentContinue] || got[agentAider] || got[agentCodex] {
		t.Errorf("unrequested agents present: %#v", got)
	}
}

func TestParseAgentsUnknownReturnsError(t *testing.T) {
	if _, err := parseAgents("claude,bogus"); err == nil {
		t.Fatalf("expected error for unknown agent")
	}
}

func writeContinueConfig(t *testing.T, value map[string]any) {
	t.Helper()
	if err := os.MkdirAll(".continue", 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".continue", "config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readContinueConfig(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".continue", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("config.json is not valid JSON after merge: %v\n%s", err, data)
	}
	return got
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func chdirTemp(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatal(err)
		}
	})
}
