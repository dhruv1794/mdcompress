package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/cache"
)

func TestWriteMirrorsSourcePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".mdcompress", "cache")

	cachePath, err := cache.Write(dir, "docs/README.md", []byte("# Docs\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !cache.Exists(cachePath) {
		t.Fatalf("cache file %q does not exist", cachePath)
	}
	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != "# Docs\n" {
		t.Fatalf("content = %q, want %q", content, "# Docs\n")
	}
}
