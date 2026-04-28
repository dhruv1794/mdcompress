package compress_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

func TestCorpusGoldens(t *testing.T) {
	corpusPaths, err := filepath.Glob("../../internal/testdata/corpus/*.md")
	if err != nil {
		t.Fatalf("glob corpus: %v", err)
	}
	if len(corpusPaths) == 0 {
		t.Fatal("no corpus files found")
	}

	for _, corpusPath := range corpusPaths {
		name := filepath.Base(corpusPath)
		goldenPath := filepath.Join("../../internal/testdata/golden", strings.TrimSuffix(name, ".md")+".golden.md")
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(corpusPath)
			if err != nil {
				t.Fatalf("read corpus: %v", err)
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			result, err := compress.Compress(input, compress.Options{Tier: compress.TierSafe})
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}
			if !bytes.Equal(result.Output, want) {
				t.Fatalf("golden mismatch for %s\n got:\n%s\nwant:\n%s", name, result.Output, want)
			}
		})
	}
}
