package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type pluginInfo struct {
	Name        string `json:"name"`
	Tier        string `json:"tier"`
	Description string `json:"description"`
}

// PluginRule is a Rule backed by an external binary discovered on PATH.
type PluginRule struct {
	Bin  string
	Name_ string
	Tier_ Tier
}

func (r *PluginRule) Name() string { return r.Name_ }
func (r *PluginRule) Tier() Tier   { return r.Tier_ }

func (r *PluginRule) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	cmd := exec.Command(r.Bin)
	cmd.Stdin = bytes.NewReader(ctx.Source)
	cmd.Stderr = nil

	out, err := cmd.Output()
	if err != nil {
		return ChangeSet{}, fmt.Errorf("plugin %s: %w", r.Name_, err)
	}
	if bytes.Equal(out, ctx.Source) {
		return ChangeSet{}, nil
	}
	stats := Stats{
		NodesAffected: 1,
		BytesSaved:    len(ctx.Source) - len(out),
	}
	if stats.BytesSaved < 0 {
		stats.BytesSaved = 0
	}
	return ChangeSet{
		Edits: []render.Edit{{Start: 0, End: len(ctx.Source), Replacement: out}},
		Stats: stats,
	}, nil
}

// PluginApply runs a plugin binary on the full source and returns the
// transformed output. Called by the pipeline after AST-level rules finish.
func PluginApply(rule *PluginRule, source []byte) ([]byte, Stats, error) {
	cmd := exec.Command(rule.Bin)
	cmd.Stdin = bytes.NewReader(source)
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return nil, Stats{}, fmt.Errorf("plugin %s: %w", rule.Name_, err)
	}
	stats := Stats{NodesAffected: 1}
	if len(source) > len(out) {
		stats.BytesSaved = len(source) - len(out)
	}
	return out, stats, nil
}

var discoveredPlugins []*PluginRule

func init() {
	discoveredPlugins = discoverPlugins()
}

func discoverPlugins() []*PluginRule {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}
	seen := make(map[string]bool)
	var found []*PluginRule
	for _, dir := range filepath.SplitList(pathEnv) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, "mdcompress-rule-") {
				continue
			}
			fullPath := filepath.Join(dir, name)
			if seen[fullPath] {
				continue
			}
			seen[fullPath] = true
			info, err := queryPlugin(fullPath)
			if err != nil {
				continue
			}
			tier := parsePluginTier(info.Tier)
			found = append(found, &PluginRule{
				Bin:   fullPath,
				Name_: info.Name,
				Tier_: tier,
			})
		}
	}
	return found
}

func queryPlugin(path string) (pluginInfo, error) {
	cmd := exec.Command(path, "--plugin-info")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return pluginInfo{}, fmt.Errorf("%s --plugin-info failed: %w", path, err)
	}
	var info pluginInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return pluginInfo{}, fmt.Errorf("%s --plugin-info: invalid JSON: %w", path, err)
	}
	if strings.TrimSpace(info.Name) == "" {
		return pluginInfo{}, fmt.Errorf("%s --plugin-info: missing name", path)
	}
	return info, nil
}

func parsePluginTier(t string) Tier {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "aggressive":
		return TierAggressive
	case "llm":
		return TierLLM
	default:
		return TierSafe
	}
}

// DiscoveredPlugins returns plugins found on PATH at process start.
func DiscoveredPlugins() []*PluginRule {
	return discoveredPlugins
}

// PluginRulesForTier returns discovered plugins at or below the given tier.
func PluginRulesForTier(t Tier) []*PluginRule {
	var out []*PluginRule
	for _, p := range discoveredPlugins {
		if p.Tier() <= t {
			out = append(out, p)
		}
	}
	return out
}

// AllRulesWithPlugins returns built-in rules plus discovered plugins.
func AllRulesWithPlugins() []Rule {
	builtin := allRules
	if len(discoveredPlugins) == 0 {
		return builtin
	}
	out := make([]Rule, len(builtin), len(builtin)+len(discoveredPlugins))
	copy(out, builtin)
	for _, p := range discoveredPlugins {
		out = append(out, p)
	}
	return out
}
