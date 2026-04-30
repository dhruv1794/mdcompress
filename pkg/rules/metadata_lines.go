package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type MetadataLines struct{}

func (r *MetadataLines) Name() string { return "strip-metadata-lines" }
func (r *MetadataLines) Tier() Tier   { return TierSafe }

var metadataLinePattern = regexp.MustCompile(`(?i)^\s*\*{0,2}(last\s*updated|updated|version|since|available\s*(since|in|from)|added\s*(in|since)|deprecated\s*(since|in)|removed\s*(in|since)|status|author|date|created|modified|published|contributors?|repository|repo|requires|compatible|support(?:s|ed)?\s*(on|by|with)?|tested\s*(on|with)|target(?:s|ing)|minimum\s*(version)?|toolchain)\s*\*{0,2}\s*:?\s*.*$`)

func (r *MetadataLines) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if line.InFence || trimmed == "" || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, ">") {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !metadataLinePattern.MatchString(trimmed) {
			continue
		}
		if isLicenseOrCopyrightLine(trimmed) {
			continue
		}
		if trimmedMetadataLine(trimmed) {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}

func trimmedMetadataLine(trimmed string) bool {
	return strings.Contains(trimmed, ":")
}

func isLicenseOrCopyrightLine(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "copyright") || strings.Contains(lower, "license") ||
		strings.Contains(lower, "spdx") || strings.Contains(lower, "all rights reserved")
}
