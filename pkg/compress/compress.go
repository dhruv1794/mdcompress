// Package compress exposes the public markdown compression API.
package compress

import (
	"os"
	"time"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
)

// profileTokensEnv enables per-rule token measurement when set to "1" or
// "true". Costly on large corpora, so default-off.
const profileTokensEnv = "MDCOMPRESS_PROFILE_TOKENS"

// Compress runs the configured rule tier against markdown content.
// Built-in rules run first (byte-range edits), then discovered plugins
// (full-text pipe), followed by the LLM rewriter when tier=llm.
func Compress(content []byte, opts Options) (Result, error) {
	tier := opts.Tier
	if tier == 0 {
		tier = TierSafe
	}

	var crossFile *rules.CrossFileState
	if opts.CrossFile != nil {
		if cfs, ok := opts.CrossFile.(*rules.CrossFileState); ok {
			crossFile = cfs
		}
	}

	profileTokens := opts.ProfileTokens || profileTokensEnabled()
	var counter tokens.Counter
	if profileTokens {
		counter = tokens.DefaultCounter()
	}

	output := content
	rulesFired := make(map[string]int)
	ruleDurations := make(map[string]int64)
	ruleBytes := make(map[string]int)
	ruleErrors := make(map[string]string)
	var ruleTokens map[string]int
	if profileTokens {
		ruleTokens = make(map[string]int)
	}
	disabled := disabledRuleSet(opts.DisabledRules)
	enabled := enabledRuleSet(opts.EnabledRules)
	for _, rule := range rules.RulesForTier(rules.Tier(tier)) {
		if (disabled[rule.Name()] || rules.DefaultDisabled(rule.Name())) && !enabled[rule.Name()] {
			continue
		}
		ctx := &rules.Context{
			Source:    output,
			FilePath:  opts.FilePath,
			CrossFile: crossFile,
			Config: &rules.Config{
				Tier:              rules.Tier(tier),
				Disabled:          disabled,
				CodeBlockMaxLines: opts.CodeBlockMaxLines,
			},
		}

		var changeSet rules.ChangeSet
		var err error
		started := time.Now()
		lineRule, ok := rule.(rules.LineRule)
		if !ok {
			continue
		}
		changeSet, err = lineRule.Apply(ctx)
		ruleDurations[rule.Name()] += time.Since(started).Milliseconds()
		if err != nil {
			ruleErrors[rule.Name()] = err.Error()
			continue
		}
		if changeSet.Stats.NodesAffected > 0 {
			rulesFired[rule.Name()] = changeSet.Stats.NodesAffected
		}
		before := len(output)
		var tokensBefore int
		if profileTokens {
			tokensBefore = counter.Count(output)
		}
		output = render.ApplyEdits(output, changeSet.Edits)
		if delta := before - len(output); delta != 0 {
			ruleBytes[rule.Name()] += delta
		}
		if profileTokens {
			tokensAfter := counter.Count(output)
			if delta := tokensBefore - tokensAfter; delta != 0 {
				ruleTokens[rule.Name()] += delta
			}
		}
	}

	pluginRules := rules.PluginRulesForTier(rules.Tier(tier))
	for _, plugin := range pluginRules {
		name := plugin.Name()
		if (disabled[name] || rules.DefaultDisabled(name)) && !enabled[name] {
			continue
		}
		started := time.Now()
		transformed, stats, err := rules.PluginApply(plugin, output)
		ruleDurations[name] += time.Since(started).Milliseconds()
		if err != nil {
			ruleErrors[name] = err.Error()
			continue
		}
		if stats.NodesAffected > 0 {
			rulesFired[name] = stats.NodesAffected
		}
		before := len(output)
		var tokensBefore int
		if profileTokens {
			tokensBefore = counter.Count(output)
		}
		output = transformed
		if delta := before - len(output); delta != 0 {
			ruleBytes[name] += delta
		}
		if profileTokens {
			tokensAfter := counter.Count(output)
			if delta := tokensBefore - tokensAfter; delta != 0 {
				ruleTokens[name] += delta
			}
		}
	}

	var llmStats LLMRewriteStats
	if Tier(tier) == TierLLM && opts.LLMRewriter != nil {
		rewritten, stats, err := opts.LLMRewriter.Rewrite(output)
		if err != nil {
			ruleErrors["llm-rewrite"] = err.Error()
		} else {
			output = rewritten
		}
		llmStats = stats
	}

	return Result{
		Output:          output,
		TokensBefore:    tokens.Count(content),
		TokensAfter:     tokens.Count(output),
		BytesBefore:     len(content),
		BytesAfter:      len(output),
		RulesFired:      rulesFired,
		RuleDurationsMS: ruleDurations,
		RuleBytesSaved:  ruleBytes,
		RuleTokensSaved: ruleTokens,
		RuleErrors:      ruleErrors,
		LLM:             llmStats,
	}, nil
}

func profileTokensEnabled() bool {
	switch os.Getenv(profileTokensEnv) {
	case "1", "true", "TRUE", "True", "yes":
		return true
	}
	return false
}

func disabledRuleSet(names []string) map[string]bool {
	disabled := make(map[string]bool, len(names))
	for _, name := range names {
		disabled[name] = true
	}
	return disabled
}

func enabledRuleSet(names []string) map[string]bool {
	enabled := make(map[string]bool, len(names))
	for _, name := range names {
		enabled[name] = true
	}
	return enabled
}
