package rules_test

import (
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func applyChangelog(t *testing.T, path string, input string) string {
	t.Helper()
	rule := &rules.ChangelogCompress{}
	cs, err := rule.Apply(&rules.Context{
		Source:   []byte(input),
		FilePath: path,
		Config:   &rules.Config{Tier: rules.TierSafe},
	})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	return string(render.ApplyEdits([]byte(input), cs.Edits))
}

func TestChangelogStripsTrackingIDs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"PR ref bracketed",
			"- fix: don't mark deriveds while an effect is updating ([#18124](https://github.com/sveltejs/svelte/pull/18124))\n",
			"- fix: don't mark deriveds while an effect is updating\n",
		},
		{
			"author + PR ref",
			"- fix something (@alice in [#34796](https://github.com/o/r/pull/34796))\n",
			"- fix something\n",
		},
		{
			"linked author + PR",
			"* devtools: fix ellipsis ([sophiebits](https://github.com/sophiebits) in [#34796](https://github.com/facebook/react/pull/34796))\n",
			"* devtools: fix ellipsis\n",
		},
		{
			"co-author block",
			"* Various UI improvements ([sebmarkbage](https://github.com/sebmarkbage) & [eps1lon](https://github.com/eps1lon))\n",
			"* Various UI improvements\n",
		},
		{
			"author + multiple PRs",
			"- Add Code Editor Sidebar (@sebmarkbage [#33968](https://github.com/o/r/pull/33968), [#33987](https://github.com/o/r/pull/33987))\n",
			"- Add Code Editor Sidebar\n",
		},
		{
			"version heading date suffix",
			"## 1.2.3 (October 1st, 2025)\n",
			"## 1.2.3\n",
		},
		{
			"version heading dash date",
			"## [1.2.3] - 2025-01-15\n",
			"## [1.2.3]\n",
		},
		{
			"standalone date line",
			"### 7.0.1\nOctober 20, 2025\n\n* something\n",
			"### 7.0.1\n\n* something\n",
		},
		{
			"description with code containing # is preserved",
			"- fix: handle `#frag` URLs correctly\n",
			"- fix: handle `#frag` URLs correctly\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyChangelog(t, "CHANGELOG.md", tc.in)
			if got != tc.want {
				t.Fatalf("got %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestChangelogSkipsNonChangelogFiles(t *testing.T) {
	in := "# Guide\n\nSee [#1234](https://github.com/o/r/pull/1234) for more info.\n"
	got := applyChangelog(t, "docs/guide.md", in)
	if got != in {
		t.Fatalf("rule fired on non-changelog file:\nin=%q\nout=%q", in, got)
	}
}

func TestChangelogActivatesOnFirstHeading(t *testing.T) {
	in := "# Release Notes\n\n## 1.0.0\n\n- did the thing ([#1](https://github.com/o/r/pull/1))\n"
	got := applyChangelog(t, "docs/notes.md", in)
	if strings.Contains(got, "#1") {
		t.Fatalf("expected fallback heading detection to activate rule:\n%s", got)
	}
}

func TestChangelogPreservesCodeFences(t *testing.T) {
	in := "# Changelog\n\n## 1.0.0\n\n```\n([#1234](https://github.com/o/r/pull/1234))\n```\n"
	got := applyChangelog(t, "CHANGELOG.md", in)
	if !strings.Contains(got, "[#1234]") {
		t.Fatalf("rule stripped tracking ID inside code fence: %s", got)
	}
}
