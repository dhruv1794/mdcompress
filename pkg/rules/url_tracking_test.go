package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func applyURLTracking(t *testing.T, input []byte) []byte {
	t.Helper()
	rule := &rules.URLTracking{}
	cs, err := rule.Apply(&rules.Context{Source: input, Config: &rules.Config{Tier: rules.TierSafe}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return render.ApplyEdits(input, cs.Edits)
}

func TestURLTracking(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single utm param removed",
			in:   "Visit [docs](https://example.com/page?utm_source=newsletter).\n",
			want: "Visit [docs](https://example.com/page).\n",
		},
		{
			name: "multiple utm params removed",
			in:   "[docs](https://example.com/page?utm_source=x&utm_medium=email&utm_campaign=launch)\n",
			want: "[docs](https://example.com/page)\n",
		},
		{
			name: "tracking param mixed with semantic param keeps semantic",
			in:   "[docs](https://example.com/page?id=42&utm_source=x)\n",
			want: "[docs](https://example.com/page?id=42)\n",
		},
		{
			name: "tracking param first, semantic second",
			in:   "[docs](https://example.com/page?utm_source=x&id=42)\n",
			want: "[docs](https://example.com/page?id=42)\n",
		},
		{
			name: "fbclid + gclid removed",
			in:   "[x](https://example.com/?fbclid=AAA&gclid=BBB&keep=yes)\n",
			want: "[x](https://example.com/?keep=yes)\n",
		},
		{
			name: "URLs inside fenced code blocks are left alone",
			in:   "```\nhttps://example.com/?utm_source=x\n```\n",
			want: "```\nhttps://example.com/?utm_source=x\n```\n",
		},
		{
			name: "no tracking params is a no-op",
			in:   "[docs](https://example.com/path?id=1#section)\n",
			want: "[docs](https://example.com/path?id=1#section)\n",
		},
		{
			name: "bare URL in prose",
			in:   "Visit https://example.com/?utm_source=newsletter for more.\n",
			want: "Visit https://example.com/ for more.\n",
		},
		{
			name: "fragment after tracking is preserved",
			in:   "[docs](https://example.com/page?utm_source=x#section)\n",
			want: "[docs](https://example.com/page#section)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyURLTracking(t, []byte(tc.in))
			if !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
