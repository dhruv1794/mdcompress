package rules

import "testing"

func TestRegistryOrderInvariants(t *testing.T) {
	order := make(map[string]int, len(allRules))
	for index, rule := range allRules {
		name := rule.Name()
		if _, exists := order[name]; exists {
			t.Fatalf("duplicate rule registered: %s", name)
		}
		order[name] = index
	}

	assertBefore := func(first, second string) {
		t.Helper()
		firstIndex, ok := order[first]
		if !ok {
			t.Fatalf("rule %s is not registered", first)
		}
		secondIndex, ok := order[second]
		if !ok {
			t.Fatalf("rule %s is not registered", second)
		}
		if firstIndex >= secondIndex {
			t.Fatalf("rule order: %s at %d must run before %s at %d", first, firstIndex, second, secondIndex)
		}
	}

	assertBefore("strip-frontmatter", "strip-html-comments")
	assertBefore("strip-setext-headers", "strip-html-comments")
	assertBefore("compress-code-blocks", "dedup-cross-file-code-blocks")
	assertBefore("dedup-cross-file-code-blocks", "truncate-large-code-blocks")
	assertBefore("truncate-large-code-blocks", "dedup-multilang-examples")
	assertBefore("compact-tables", "collapse-blank-lines")

	if got := allRules[len(allRules)-1].Name(); got != "collapse-blank-lines" {
		t.Fatalf("collapse-blank-lines should remain the final cleanup rule, got final rule %s", got)
	}
}
