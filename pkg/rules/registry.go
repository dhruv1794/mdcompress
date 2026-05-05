package rules

// allRules is the fixed execution order for compression rules. Order is not
// arbitrary — several rules depend on prior cleanup having run. The hard
// invariants are asserted by TestRegistryOrderInvariants; the high-level
// constraints are:
//
//   - Frontmatter must run before any rule that scans for "---" or HTML
//     wrappers, otherwise YAML metadata leaks into downstream regex.
//   - Code-block compression must run before code-block dedup so the dedup
//     hash is taken on cleaned content.
//   - BlankLines runs last as the cleanup pass that collapses gaps left by
//     earlier rules.
var allRules = []Rule{
	&Frontmatter{},
	&URLTracking{},
	&SetextHeaders{},
	&HTMLComments{},
	&CodeBlocks{},
	&Badges{},
	&DecorativeImages{},
	&MetadataLines{},
	&HorizontalRules{},
	&HTMLWrappers{},
	&TOC{},
	&TrailingCTA{},
	&CrossFileDupes{},
	&CrossFileCodeBlocks{},
	&CodeBlockTruncate{},
	&MultilangDedup{},
	&MarketingProse{},
	&HedgingPhrases{},
	&DedupCrossSection{},
	&BenchmarkProse{},
	&AdmonitionPrefixes{},
	&CrossReferences{},
	&MkdocsIncludes{},
	&EditPageFooters{},
	&APIParameterTrivia{},
	&BoilerplateSections{},
	&VerificationBoilerplate{},
	&SEOChaff{},
	&ChangelogCompress{},
	&ExampleOutput{},
	&TableNormalize{},
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
