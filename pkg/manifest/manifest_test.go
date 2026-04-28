package manifest_test

import (
	"path/filepath"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/manifest"
)

func TestReadMissingManifestReturnsEmptyManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mdcompress", "manifest.json")

	m, err := manifest.Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if m.Version != manifest.Version {
		t.Fatalf("Version = %d, want %d", m.Version, manifest.Version)
	}
	if len(m.Entries) != 0 {
		t.Fatalf("Entries length = %d, want 0", len(m.Entries))
	}
}

func TestWriteRecalculatesTotals(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mdcompress", "manifest.json")
	m := manifest.New()
	m.Entries["README.md"] = manifest.Entry{
		Source:       "README.md",
		Cache:        ".mdcompress/cache/README.md",
		SHA256:       "abc",
		TokensBefore: 10,
		TokensAfter:  6,
	}

	if err := manifest.Write(path, m); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got, err := manifest.Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.Totals.Files != 1 || got.Totals.TokensSaved != 4 {
		t.Fatalf("Totals = %+v, want files=1 tokens_saved=4", got.Totals)
	}
}
