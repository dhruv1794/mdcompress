package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
		"models":              []any{map[string]any{"title": "Local", "provider": "ollama"}},
		"tabAutocompleteModel": map[string]any{"title": "Auto", "provider": "ollama"},
		"customCommands":      []any{map[string]any{"name": "test", "prompt": "hi"}},
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
