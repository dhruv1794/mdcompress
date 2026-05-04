package rules

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dhruv1794/mdcompress/pkg/render"
)

func TestDiscoverPlugins_None(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	discoveredPlugins = nil
	discoveredPlugins = discoverPlugins()
	if len(discoveredPlugins) > 0 {
		t.Errorf("expected no plugins, got %d", len(discoveredPlugins))
	}
}

func TestPluginRule_NameAndTier(t *testing.T) {
	r := &PluginRule{Name_: "test-rule", Tier_: TierAggressive}
	if r.Name() != "test-rule" {
		t.Errorf("expected name test-rule, got %s", r.Name())
	}
	if r.Tier() != TierAggressive {
		t.Errorf("expected tier aggressive, got %v", r.Tier())
	}
}

func TestPluginRule_Apply_NoChange(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-identity")
	writePluginScript(t, pluginPath, `#!/bin/sh
if [ "$1" = "--plugin-info" ]; then
  echo '{"name":"identity","tier":"safe","description":"passthrough"}'
  exit 0
fi
cat
`)
	r := &PluginRule{Bin: pluginPath, Name_: "identity", Tier_: TierSafe}

	source := []byte("# hello\n")
	ctx := &Context{Source: source, Config: &Config{}}
	cs, err := r.Apply(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cs.Stats.NodesAffected != 0 {
		t.Errorf("expected NodesAffected=0 for identity (no-op), got %d", cs.Stats.NodesAffected)
	}
}

func TestPluginRule_PluginApply(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-stripper")
	writePluginScript(t, pluginPath, `#!/bin/sh
if [ "$1" = "--plugin-info" ]; then
  echo '{"name":"stripper","tier":"aggressive","description":"strips comments"}'
  exit 0
fi
sed 's/<!--.*-->//g'
`)
	r := &PluginRule{Bin: pluginPath, Name_: "stripper", Tier_: TierAggressive}

	source := []byte("Hello <!-- world -->\n")
	out, stats, err := PluginApply(r, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) == string(source) {
		t.Error("expected output to differ from source")
	}
	if stats.BytesSaved == 0 {
		t.Error("expected BytesSaved > 0")
	}
}

func TestDiscoverPlugins_Valid(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-myrule")
	writePluginScript(t, pluginPath, `#!/bin/sh
echo '{"name":"myrule","tier":"safe","description":"my rule"}'
exit 0
`)
	pathEnv := dir + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathEnv)
	discoveredPlugins = nil
	discoveredPlugins = discoverPlugins()
	found := false
	for _, p := range discoveredPlugins {
		if p.Name() == "myrule" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to discover myrule plugin, got %d plugins", len(discoveredPlugins))
	}
}

func TestDiscoverPlugins_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-badjson")
	writePluginScript(t, pluginPath, `#!/bin/sh
echo 'not json'
exit 0
`)
	pathEnv := dir + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathEnv)
	discoveredPlugins = nil
	discoveredPlugins = discoverPlugins()
	for _, p := range discoveredPlugins {
		if p.Name() == "badjson" {
			t.Errorf("should not discover plugin with invalid JSON")
		}
	}
}

func TestDiscoverPlugins_NoName(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-noname")
	writePluginScript(t, pluginPath, `#!/bin/sh
echo '{"tier":"safe"}'
exit 0
`)
	pathEnv := dir + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathEnv)
	discoveredPlugins = nil
	discoveredPlugins = discoverPlugins()
	for _, p := range discoveredPlugins {
		if p != nil && p.Name() == "" {
			t.Errorf("should not discover plugin without name")
		}
	}
}

func TestAllRulesWithPlugins_NoneFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	discoveredPlugins = nil
	discoveredPlugins = discoverPlugins()
	rules := AllRulesWithPlugins()
	if len(rules) != len(allRules) {
		t.Errorf("expected %d rules with no plugins, got %d", len(allRules), len(rules))
	}
}

func TestRenderNoOpPlugin(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-noop")
	writePluginScript(t, pluginPath, `#!/bin/sh
if [ "$1" = "--plugin-info" ]; then
  echo '{"name":"noop","tier":"safe","description":"no-op"}'
  exit 0
fi
cat
`)
	r := &PluginRule{Bin: pluginPath, Name_: "test", Tier_: TierSafe}
	cs, err := r.Apply(&Context{Source: []byte("x"), Config: &Config{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := render.ApplyEdits([]byte("x"), cs.Edits)
	if string(result) != "x" {
		t.Errorf("expected no-op for empty edits, got %q", string(result))
	}
}

func TestPluginApplyRejectsOversizedOutput(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-big")
	writePluginScript(t, pluginPath, `#!/bin/sh
printf 'abcdef'
`)
	r := &PluginRule{Bin: pluginPath, Name_: "big", Tier_: TierSafe}
	_, _, err := PluginApply(r, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "output size") {
		t.Fatalf("expected output size error, got %v", err)
	}
}

func TestPluginApplyRejectsInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-invalid")
	writePluginScript(t, pluginPath, `#!/bin/sh
printf '\377'
`)
	r := &PluginRule{Bin: pluginPath, Name_: "invalid", Tier_: TierSafe}
	_, _, err := PluginApply(r, []byte("xxxx"))
	if err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("expected UTF-8 error, got %v", err)
	}
}

func TestPluginApplyTimesOut(t *testing.T) {
	oldTimeout := pluginTimeout
	pluginTimeout = 10 * time.Millisecond
	defer func() { pluginTimeout = oldTimeout }()

	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "mdcompress-rule-slow")
	writePluginScript(t, pluginPath, `#!/bin/sh
sleep 1
cat
`)
	r := &PluginRule{Bin: pluginPath, Name_: "slow", Tier_: TierSafe}
	_, _, err := PluginApply(r, []byte("xxxx"))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestPluginRulesForTier(t *testing.T) {
	discoveredPlugins = []*PluginRule{
		{Bin: "/tmp/a", Name_: "plugin-safe", Tier_: TierSafe},
		{Bin: "/tmp/b", Name_: "plugin-agg", Tier_: TierAggressive},
	}
	defer func() { discoveredPlugins = nil }()

	safe := PluginRulesForTier(TierSafe)
	if len(safe) != 1 || safe[0].Name_ != "plugin-safe" {
		t.Errorf("expected 1 safe plugin, got %d", len(safe))
	}
	agg := PluginRulesForTier(TierAggressive)
	if len(agg) != 2 {
		t.Errorf("expected 2 aggressive-tier plugins, got %d", len(agg))
	}
}

func writePluginScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write plugin script: %v", err)
	}
	_ = exec.Command
	_ = filepath.Join
}
