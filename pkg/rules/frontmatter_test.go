package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func TestFrontmatter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "yaml frontmatter is stripped",
			in:   "---\ntitle: Hello\nauthor: A\n---\n# Body\n",
			want: "# Body\n",
		},
		{
			name: "toml frontmatter is stripped",
			in:   "+++\ntitle = \"Hello\"\n+++\n# Body\n",
			want: "# Body\n",
		},
		{
			name: "crlf line endings",
			in:   "---\r\ntitle: Hello\r\n---\r\nBody\r\n",
			want: "Body\r\n",
		},
		{
			name: "no frontmatter is unchanged",
			in:   "# Body\n\nText\n",
			want: "# Body\n\nText\n",
		},
		{
			name: "missing closing delimiter is left alone",
			in:   "---\ntitle: oops\n# Body\n",
			want: "---\ntitle: oops\n# Body\n",
		},
		{
			name: "horizontal rule is not frontmatter (mid-doc ---)",
			in:   "# Title\n\n---\n\nBody\n",
			want: "# Title\n\n---\n\nBody\n",
		},
		{
			name: "empty frontmatter still stripped",
			in:   "---\n---\nBody\n",
			want: "Body\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := &rules.Frontmatter{}
			ctx := &rules.Context{Source: []byte(tc.in), Config: &rules.Config{Tier: rules.TierSafe}}
			cs, err := rule.Apply(ctx)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got := render.Splice([]byte(tc.in), cs.Ranges)
			if !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
