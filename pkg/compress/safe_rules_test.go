package compress_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

func TestCompressSafeTierRules(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "badges",
			input: "# Project\n\n[![CI](https://github.com/acme/tool/actions/workflows/ci.yml/badge.svg)](https://github.com/acme/tool/actions)\nText with ![build](https://img.shields.io/badge/build-passing-green.svg) badge.\n",
			want:  "# Project\n\nText with  badge.\n",
		},
		{
			name:  "reference badges",
			input: "# Project\n\n[![NPM Version][npm-version-image]][npm-url]\n[![Coverage][coveralls-image]][coveralls-url]\n\nInstall it.\n\n[npm-version-image]: https://img.shields.io/npm/v/tool\n[npm-url]: https://npmjs.org/package/tool\n[coveralls-image]: https://img.shields.io/coverallsCoverage/github/acme/tool\n[coveralls-url]: https://coveralls.io/r/acme/tool\n",
			want:  "# Project\n\nInstall it.\n\n[npm-url]: https://npmjs.org/package/tool\n",
		},
		{
			name:  "html badge paragraph",
			input: "# Project\n\n<p align=\"center\">\n<a href=\"https://github.com/acme/tool/actions\"><img src=\"https://github.com/acme/tool/actions/workflows/ci.yml/badge.svg\" alt=\"CI\"></a>\n<a href=\"https://pypi.org/project/tool\"><img src=\"https://img.shields.io/pypi/v/tool\" alt=\"Package version\"></a>\n</p>\n\nUse it.\n",
			want:  "# Project\n\nUse it.\n",
		},
		{
			name:  "multiline html badge anchors",
			input: "# Project\n\n<p align=\"center\">\n<a href=\"https://github.com/acme/tool/actions\">\n    <img src=\"https://github.com/acme/tool/actions/workflows/ci.yml/badge.svg\" alt=\"CI\">\n</a>\n<a href=\"https://coverage-badge.example/redirect/acme/tool\">\n    <img src=\"https://coverage-badge.example/acme/tool.svg\" alt=\"Coverage\">\n</a>\n</p>\n\nUse it.\n",
			want:  "# Project\n\nUse it.\n",
		},
		{
			name:  "decorative images",
			input: "# Project\n\n![logo](logo.png)\n\n![Architecture diagram showing requests flowing through the queue](architecture.png)\n",
			want:  "# Project\n\n![Architecture diagram showing requests flowing through the queue](architecture.png)\n",
		},
		{
			name:  "decorative html image block",
			input: "# Project\n\n<p align=\"center\">\n  <a href=\"https://example.com\"><img src=\"https://example.com/logo.png\" alt=\"Project\"></a>\n</p>\n\nUse it.\n",
			want:  "# Project\n\nUse it.\n",
		},
		{
			name:  "toc",
			input: "# Project\n\n## Table of Contents\n\n- [Install](#install)\n- [Usage](#usage)\n\n## Install\n\nUse it.\n",
			want:  "# Project\n\n## Install\n\nUse it.\n",
		},
		{
			name:  "trailing cta",
			input: "# Project\n\nUse it.\n\n## Usage\n\nRun the command.\n\nAdditional technical notes explain configuration, files, and runtime behavior.\n\n## Star This Repo\n\nPlease star it.\n",
			want:  "# Project\n\nUse it.\n\n## Usage\n\nRun the command.\n\nAdditional technical notes explain configuration, files, and runtime behavior.\n",
		},
		{
			name:  "blank lines outside fences",
			input: "\n\n# Project\n\n\nText.\n\n```text\none\n\n\ntwo\n```\n\n\n",
			want:  "# Project\n\nText.\n\n```text\none\n\n\ntwo\n```\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := compress.Compress([]byte(test.input), compress.Options{Tier: compress.TierSafe})
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			if !bytes.Equal(result.Output, []byte(test.want)) {
				t.Fatalf("output = %q, want %q", result.Output, test.want)
			}
		})
	}
}

func TestCompressAggressiveTierStripsMarketingProse(t *testing.T) {
	input := []byte("# Project\n\nA production-ready Go library for markdown.\n\n## Usage\n\nRun it.\n")

	safe, err := compress.Compress(input, compress.Options{Tier: compress.TierSafe})
	if err != nil {
		t.Fatalf("safe Compress() error = %v", err)
	}
	if !bytes.Equal(safe.Output, input) {
		t.Fatalf("safe output = %q, want %q", safe.Output, input)
	}

	aggressive, err := compress.Compress(input, compress.Options{Tier: compress.TierAggressive})
	if err != nil {
		t.Fatalf("aggressive Compress() error = %v", err)
	}
	want := []byte("# Project\n\nA Go library for markdown.\n\n## Usage\n\nRun it.\n")
	if !bytes.Equal(aggressive.Output, want) {
		t.Fatalf("aggressive output = %q, want %q", aggressive.Output, want)
	}
	if aggressive.RulesFired["strip-marketing-prose"] != 1 {
		t.Fatalf("strip-marketing-prose fired %d times", aggressive.RulesFired["strip-marketing-prose"])
	}
}

func TestCompressAggressiveTierStripsHedgingPhrases(t *testing.T) {
	input := []byte("# Project\n\nPlease note that users run mdcompress in order to refresh docs.\n")

	safe, err := compress.Compress(input, compress.Options{Tier: compress.TierSafe})
	if err != nil {
		t.Fatalf("safe Compress() error = %v", err)
	}
	if !bytes.Equal(safe.Output, input) {
		t.Fatalf("safe output = %q, want %q", safe.Output, input)
	}

	aggressive, err := compress.Compress(input, compress.Options{Tier: compress.TierAggressive})
	if err != nil {
		t.Fatalf("aggressive Compress() error = %v", err)
	}
	want := []byte("# Project\n\nUsers run mdcompress to refresh docs.\n")
	if !bytes.Equal(aggressive.Output, want) {
		t.Fatalf("aggressive output = %q, want %q", aggressive.Output, want)
	}
	if aggressive.RulesFired["strip-hedging-phrases"] != 2 {
		t.Fatalf("strip-hedging-phrases fired %d times", aggressive.RulesFired["strip-hedging-phrases"])
	}
}
