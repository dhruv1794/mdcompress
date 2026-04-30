package rules

import (
	"regexp"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

var (
	linkedImagePattern          = regexp.MustCompile(`\[!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)\]\([^)]+\)`)
	imagePattern                = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	referenceLinkedImagePattern = regexp.MustCompile(`\[!\[([^\]]*)\]\[([^\]]+)\]\]\[[^\]]+\]`)
	referenceImagePattern       = regexp.MustCompile(`!\[([^\]]*)\]\[([^\]]+)\]`)
	referenceDefinitionPattern  = regexp.MustCompile(`^\s*\[([^\]]+)\]:\s*(\S+)`)
	htmlBadgeBlockPattern       = regexp.MustCompile(`(?is)<p\b[^>]*>\s*(?:<a\b[^>]*>\s*<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>\s*</a>\s*)+</p>`)
	htmlLinkedImagePattern      = regexp.MustCompile(`(?is)<a\b[^>]*>\s*<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>\s*</a>`)
	htmlImageSrcPattern         = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>`)
)

type Badges struct{}

func (r *Badges) Name() string { return "strip-badges" }
func (r *Badges) Tier() Tier   { return TierSafe }

func (r *Badges) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc

	var changes ChangeSet
	refURLs := badgeReferenceDefinitions(ctx.Source)
	stripRefs := make(map[string]bool)

	for _, match := range htmlBadgeBlockPattern.FindAllSubmatchIndex(ctx.Source, -1) {
		if htmlBlockContainsOnlyBadges(ctx.Source[match[0]:match[1]]) {
			addRange(&changes, render.Range{Start: match[0], End: match[1]})
		}
	}
	for _, match := range htmlLinkedImagePattern.FindAllSubmatchIndex(ctx.Source, -1) {
		url := string(ctx.Source[match[2]:match[3]])
		if isBadgeURL(url) {
			addRange(&changes, render.Range{Start: match[0], End: match[1]})
		}
	}

	lines := sourceLines(ctx.Source)
	for _, line := range lines {
		if line.InFence {
			continue
		}
		var lineRanges []render.Range
		for _, match := range linkedImagePattern.FindAllStringSubmatchIndex(line.Text, -1) {
			alt := line.Text[match[2]:match[3]]
			url := line.Text[match[4]:match[5]]
			if shouldStripBadge(alt, url) {
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		for _, match := range imagePattern.FindAllStringSubmatchIndex(line.Text, -1) {
			if insideExistingRange(line.Start+match[0], line.Start+match[1], lineRanges) {
				continue
			}
			alt := line.Text[match[2]:match[3]]
			url := line.Text[match[4]:match[5]]
			if shouldStripBadge(alt, url) {
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		for _, match := range referenceLinkedImagePattern.FindAllStringSubmatchIndex(line.Text, -1) {
			alt := line.Text[match[2]:match[3]]
			ref := normalizeReferenceLabel(line.Text[match[4]:match[5]])
			if shouldStripBadge(alt, refURLs[ref]) {
				stripRefs[ref] = true
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		for _, match := range referenceImagePattern.FindAllStringSubmatchIndex(line.Text, -1) {
			if insideExistingRange(line.Start+match[0], line.Start+match[1], lineRanges) {
				continue
			}
			alt := line.Text[match[2]:match[3]]
			ref := normalizeReferenceLabel(line.Text[match[4]:match[5]])
			if shouldStripBadge(alt, refURLs[ref]) {
				stripRefs[ref] = true
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		for _, match := range htmlLinkedImagePattern.FindAllStringSubmatchIndex(line.Text, -1) {
			url := line.Text[match[2]:match[3]]
			if isBadgeURL(url) {
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		for _, match := range htmlImageSrcPattern.FindAllStringSubmatchIndex(line.Text, -1) {
			if insideExistingRange(line.Start+match[0], line.Start+match[1], lineRanges) {
				continue
			}
			url := line.Text[match[2]:match[3]]
			if isBadgeURL(url) {
				lineRanges = append(lineRanges, render.Range{Start: line.Start + match[0], End: line.Start + match[1]})
			}
		}
		if len(lineRanges) == 0 {
			continue
		}
		if rangesCoverOnlyDecorativeMarkup(line.Text, line.Start, lineRanges) {
			addRange(&changes, fullLineRange(line))
			continue
		}
		for _, removal := range lineRanges {
			addRange(&changes, removal)
		}
	}
	for _, line := range lines {
		if line.InFence {
			continue
		}
		match := referenceDefinitionPattern.FindStringSubmatch(line.Text)
		if match == nil {
			continue
		}
		ref := normalizeReferenceLabel(match[1])
		if stripRefs[ref] || isBadgeURL(match[2]) {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}

func badgeReferenceDefinitions(source []byte) map[string]string {
	defs := make(map[string]string)
	for _, line := range sourceLines(source) {
		if line.InFence {
			continue
		}
		match := referenceDefinitionPattern.FindStringSubmatch(line.Text)
		if match == nil {
			continue
		}
		defs[normalizeReferenceLabel(match[1])] = match[2]
	}
	return defs
}

func normalizeReferenceLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

func htmlBlockContainsOnlyBadges(block []byte) bool {
	found := false
	for _, match := range htmlImageSrcPattern.FindAllSubmatch(block, -1) {
		if len(match) < 2 || !isBadgeURL(string(match[1])) {
			return false
		}
		found = true
	}
	return found
}

func shouldStripBadge(alt string, url string) bool {
	if meaningfulBadgeAlt(alt) {
		return false
	}
	return isBadgeURL(url)
}

func meaningfulBadgeAlt(alt string) bool {
	lowerAlt := strings.ToLower(strings.TrimSpace(alt))
	if wordCount(lowerAlt) <= 5 {
		return false
	}
	for _, marker := range []string{"badge", "build", "status", "license", "coverage", "download", "version", "ci"} {
		if strings.Contains(lowerAlt, marker) {
			return false
		}
	}
	return true
}

func isBadgeURL(url string) bool {
	lowerURL := strings.ToLower(url)
	for _, marker := range []string{
		"img.shields.io",
		"shields.io",
		"pepy.tech",
		"badge.fury.io",
		"travis-ci.com",
		"travis-ci.org",
		"circleci.com/gh/",
		"coverage-badge.",
		"coveralls.io",
		"codecov.io",
		"camo.githubusercontent.com",
		"bestpractices.coreinfrastructure.org/projects/",
		"goreportcard.com/badge/",
		"api.scorecard.dev/projects/",
		"/actions/workflows/",
		"badge.svg",
		"app.codacy.com/project/badge",
		"snyk.io/test/",
		"sonarcloud.io/api/project_badges",
		"app.wercker.com",
		"ci.appveyor.com",
		"api.cirrus-ci.com",
		"semaphoreci.com",
		"drone.io",
		"app.buddy.works",
		"app.saucelabs.com",
		"app.fossa.com",
		"bettercodehub.com",
		"david-dm.org",
		"requires.io",
		"inch-ci.org",
		"lgtm.com",
	} {
		if strings.Contains(lowerURL, marker) {
			return true
		}
	}
	return false
}

func insideExistingRange(start int, end int, ranges []render.Range) bool {
	for _, existing := range ranges {
		if start >= existing.Start && end <= existing.End {
			return true
		}
	}
	return false
}

func rangesCoverOnlyDecorativeMarkup(line string, lineStart int, ranges []render.Range) bool {
	remaining := []byte(line)
	for _, removal := range ranges {
		start := removal.Start - lineStart
		end := removal.End - lineStart
		if start < 0 || end > len(remaining) || start >= end {
			continue
		}
		for index := start; index < end; index++ {
			remaining[index] = ' '
		}
	}
	return strings.TrimSpace(string(remaining)) == ""
}
