package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type VerificationBoilerplate struct{}

func (r *VerificationBoilerplate) Name() string { return "strip-verification-boilerplate" }
func (r *VerificationBoilerplate) Tier() Tier   { return TierAggressive }

var verificationLinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*if (valid|successful),?\s+the output (is|will be|should be|looks like)\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*if the (check|command|test) (fails|succeeds|passes),?\s+.*$`),
	regexp.MustCompile(`(?i)^\s*if (you do|it does) not (see|encounter|get) (an |a )?error,?\s+it means\s+.*$`),
	regexp.MustCompile(`(?i)^\s*if (no )?error(s)? occur(s)?,?\s+.*$`),
	regexp.MustCompile(`(?i)^\s*the (expected )?output (is|will be|should be|looks like)\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*(you should see|you will see|you should get|you will get)\s+.*$`),
	regexp.MustCompile(`(?i)^\s*if (everything|all) (is|goes|went) (well|correctly),?\s+.*$`),
	regexp.MustCompile(`(?i)^\s*if (the )?above (command|step)s? (succeeds|executed? successfully|completed? successfully|ran? without errors?),?\s+.*$`),
}

func (r *VerificationBoilerplate) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		for _, pattern := range verificationLinePatterns {
			if pattern.MatchString(trimmed) {
				addRange(&changes, fullLineRange(line))
				break
			}
		}
	}

	return changes, nil
}
