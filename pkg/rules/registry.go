package rules

var allRules = []Rule{
	&Frontmatter{},
	&SetextHeaders{},
	&HTMLComments{},
	&Badges{},
	&DecorativeImages{},
	&MetadataLines{},
	&TOC{},
	&TrailingCTA{},
	&MarketingProse{},
	&HedgingPhrases{},
	&DedupCrossSection{},
	&BenchmarkProse{},
	&AdmonitionPrefixes{},
	&CrossReferences{},
	&BoilerplateSections{},
	&VerificationBoilerplate{},
	&ExampleOutput{},
	&BlankLines{},
}

// DefaultDisabled reports whether a rule is opt-in even when its tier is active.
func DefaultDisabled(name string) bool {
	return name == "collapse-example-output" || name == "strip-boilerplate-sections"
}

// AllRules returns every rule in fixed execution order.
func AllRules() []Rule {
	return allRules
}

// RulesForTier returns rules at or below the requested tier.
func RulesForTier(t Tier) []Rule {
	out := make([]Rule, 0, len(allRules))
	for _, rule := range allRules {
		if rule.Tier() <= t {
			out = append(out, rule)
		}
	}
	return out
}
