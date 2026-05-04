package rules

import (
	"regexp"
	"strings"
)

// MkdocsIncludes strips MkDocs/PyMdown snippet directives (--8<--, {!...!}).
type MkdocsIncludes struct{}

func (r *MkdocsIncludes) Name() string { return "strip-mkdocs-includes" }
func (r *MkdocsIncludes) Tier() Tier   { return TierAggressive }

var mkdocsIncludeRe = regexp.MustCompile(`^\s*(?:--8<--|;--8<--|--snippet--)[^\n]*$`)

func (r *MkdocsIncludes) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for _, line := range lines {
		if line.InFence {
			continue
		}
		if mkdocsIncludeRe.MatchString(line.Text) || strings.HasPrefix(strings.TrimSpace(line.Text), "{!") {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}

// EditPageFooters strips "edit this page" / "last updated" / "view source" trailers.
type EditPageFooters struct{}

func (r *EditPageFooters) Name() string { return "strip-edit-page-footers" }
func (r *EditPageFooters) Tier() Tier   { return TierAggressive }

var editPageRe = regexp.MustCompile(`(?i)^\s*(?:#{1,6}\s+|\*+\s*)?(?:edit (?:this )?page|edit on github|last updated|page last (?:modified|updated)|view source)\b.*$`)

func (r *EditPageFooters) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for _, line := range lines {
		if line.InFence {
			continue
		}
		if editPageRe.MatchString(line.Text) {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}

// APIParameterTrivia drops single-fact rows like `- **Default**: None` /
// `- **Required**: yes` that pad MkDocStrings-style API references.
type APIParameterTrivia struct{}

func (r *APIParameterTrivia) Name() string { return "compress-api-parameter-trivia" }
func (r *APIParameterTrivia) Tier() Tier   { return TierAggressive }

var apiTriviaRe = regexp.MustCompile(`(?i)^\s*[-*]\s+\*{0,2}(default|required|type|optional|nullable|since|deprecated)\*{0,2}\s*[:=]\s*(none|null|nil|true|false|yes|no|n/a|-)\s*\.?\s*$`)

func (r *APIParameterTrivia) Apply(ctx *Context) (ChangeSet, error) {
	lines := sourceLines(ctx.Source)
	var changes ChangeSet
	for _, line := range lines {
		if line.InFence {
			continue
		}
		if apiTriviaRe.MatchString(line.Text) {
			addRange(&changes, fullLineRange(line))
		}
	}
	return changes, nil
}
