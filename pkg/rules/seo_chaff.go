package rules

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type SEOChaff struct{}

func (r *SEOChaff) Name() string { return "strip-seo-chaff" }
func (r *SEOChaff) Tier() Tier   { return TierAggressive }

var seoChaffPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*on this page\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*in this article\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*in this (guide|document|section|tutorial)\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*(edit (this page|on github)( on github)?|suggest (edits|changes)|found a typo\??|report a bug)\s*:?\s*$`),
	regexp.MustCompile(`(?i)^\s*was this (page |article |document )?(helpful|useful)\??\s*$`),
	regexp.MustCompile(`(?i)^\s*(last |page )?(updated|modified|reviewed)( on |:).*$`),
	regexp.MustCompile(`(?i)^\s*all content on this page .+(license|copyright|creative commons).+$`),
	regexp.MustCompile(`(?i)^\s*(?:\[?edit\]?)(?:\s*on\s+github)?$`),
	regexp.MustCompile(`(?i)^\s*(help us improve|feedback|rate this|share this)\s*:?\s*$`),
}

var seoBreadcrumbPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*([a-z0-9._-]+|\[[^\]]+\](?:\([^)]+\))?)\s*>\s*([a-z0-9._-]+|\[[^\]]+\](?:\([^)]+\))?)\s*>\s*.+$`),
	regexp.MustCompile(`(?i)^\s*\[?(home|docs|guide|reference|api|cookbook|examples?)\]?\s*(/|>|\|)\s*\[?(.+)\s*(/|>|\|)\s*.+$`),
}

var seoPrevNextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*(previous|prev|next|back|continue)\s*:?\s*(section|page|chapter|article|guide)?\s*(:|\|)?\s*(\[?[^\]]+\]?)?\s*$`),
	regexp.MustCompile(`(?i)^\s*\[?(previous|prev|next|back)\s*(section|page|chapter|article|guide)?\s*\]?\s*[:|]?\s*\[?[^\]]+\]?\s*$`),
}

func (r *SEOChaff) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	lines := sourceLines(ctx.Source)
	var changes ChangeSet

	for _, line := range lines {
		trimmed := strings.TrimSpace(line.Text)
		if trimmed == "" || line.InFence {
			continue
		}
		if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") {
			continue
		}

		matched := false
		for _, pattern := range seoChaffPatterns {
			if pattern.MatchString(trimmed) {
				matched = true
				break
			}
		}
		if !matched {
			for _, pattern := range seoBreadcrumbPatterns {
				if pattern.MatchString(trimmed) && wordCount(trimmed) <= 12 {
					matched = true
					break
				}
			}
		}
		if !matched {
			for _, pattern := range seoPrevNextPatterns {
				if pattern.MatchString(trimmed) && wordCount(trimmed) <= 10 {
					matched = true
					break
				}
			}
		}

		if matched {
			addRange(&changes, fullLineRange(line))
		}
	}

	return changes, nil
}
