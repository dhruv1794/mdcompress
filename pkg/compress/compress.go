// Package compress exposes the public markdown compression API.
package compress

import (
	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
)

// Compress runs the configured rule tier against markdown content.
func Compress(content []byte, opts Options) (Result, error) {
	tier := opts.Tier
	if tier == 0 {
		tier = TierSafe
	}

	output := content
	rulesFired := make(map[string]int)
	disabled := disabledRuleSet(opts.DisabledRules)
	enabled := enabledRuleSet(opts.EnabledRules)
	doc, err := parser.Parse(output)
	if err != nil {
		return Result{}, err
	}
	for _, rule := range rules.RulesForTier(rules.Tier(tier)) {
		if (disabled[rule.Name()] || rules.DefaultDisabled(rule.Name())) && !enabled[rule.Name()] {
			continue
		}
		ctx := &rules.Context{
			Source: output,
			Config: &rules.Config{
				Tier:     rules.Tier(tier),
				Disabled: disabled,
			},
		}
		changeSet, err := rule.Apply(doc, ctx)
		if err != nil {
			continue
		}
		if changeSet.Stats.NodesAffected > 0 {
			rulesFired[rule.Name()] = changeSet.Stats.NodesAffected
		}
		output = render.ApplyEdits(output, changeSet.Edits)
	}

	var llmStats LLMRewriteStats
	if Tier(tier) == TierLLM && opts.LLMRewriter != nil {
		rewritten, stats, err := opts.LLMRewriter.Rewrite(output)
		if err == nil {
			output = rewritten
		}
		llmStats = stats
	}

	return Result{
		Output:       output,
		TokensBefore: tokens.Count(content),
		TokensAfter:  tokens.Count(output),
		BytesBefore:  len(content),
		BytesAfter:   len(output),
		RulesFired:   rulesFired,
		LLM:          llmStats,
	}, nil
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
