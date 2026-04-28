package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

var standaloneImagePattern = regexp.MustCompile(`^\s*!\[([^\]]*)\]\([^)]+\)\s*$`)
var (
	standaloneHTMLImagePattern      = regexp.MustCompile(`(?is)^\s*(?:<a\b[^>]*>\s*)?<img\b[^>]*>(?:\s*</a>)?\s*$`)
	htmlDecorativeImageBlockPattern = regexp.MustCompile(`(?is)<p\b[^>]*>\s*(?:<a\b[^>]*>\s*)?<img\b[^>]*>(?:\s*</a>)?\s*</p>`)
	htmlAltPattern                  = regexp.MustCompile(`(?is)\balt=["']([^"']*)["']`)
	htmlSrcPattern                  = regexp.MustCompile(`(?is)\bsrc=["']([^"']*)["']`)
)

type DecorativeImages struct{}

func (r *DecorativeImages) Name() string { return "strip-decorative-images" }
func (r *DecorativeImages) Tier() Tier   { return TierSafe }

func (r *DecorativeImages) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	var changes ChangeSet
	for _, match := range htmlDecorativeImageBlockPattern.FindAllIndex(ctx.Source, -1) {
		if decorativeHTMLImage(ctx.Source[match[0]:match[1]]) {
			addRange(&changes, render.Range{Start: match[0], End: match[1]})
		}
	}
	for _, line := range sourceLines(ctx.Source) {
		if line.InFence {
			continue
		}
		match := standaloneImagePattern.FindStringSubmatch(line.Text)
		if match != nil && decorativeAlt(match[1]) {
			addRange(&changes, fullLineRange(line))
			continue
		}
		if standaloneHTMLImagePattern.MatchString(line.Text) && decorativeHTMLImage([]byte(line.Text)) {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}

func decorativeAlt(alt string) bool {
	trimmed := strings.TrimSpace(alt)
	lowerAlt := strings.ToLower(trimmed)
	if trimmed == "" || singleRune(trimmed) {
		return true
	}
	if wordCount(trimmed) > 3 {
		return false
	}
	for _, marker := range []string{"banner", "logo", "hero", "header", "divider", "image", "pic", "screenshot"} {
		if strings.Contains(lowerAlt, marker) {
			return true
		}
	}
	return false
}

func decorativeHTMLImage(markup []byte) bool {
	alt := htmlAttribute(htmlAltPattern, markup)
	src := htmlAttribute(htmlSrcPattern, markup)
	if decorativeAlt(alt) {
		return true
	}
	lowerSrc := strings.ToLower(src)
	for _, marker := range []string{"logo", "banner", "hero", "header", "divider"} {
		if strings.Contains(lowerSrc, marker) {
			return true
		}
	}
	return false
}

func htmlAttribute(pattern *regexp.Regexp, markup []byte) string {
	match := pattern.FindSubmatch(markup)
	if len(match) < 2 {
		return ""
	}
	return string(match[1])
}
