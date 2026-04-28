// Package cache writes compressed markdown mirrors under .mdcompress/cache.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

const DefaultDir = ".mdcompress/cache"

func SourceSHA(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func Path(cacheDir, sourceRel string) string {
	return filepath.Join(cacheDir, filepath.FromSlash(sourceRel))
}

func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func Write(cacheDir, sourceRel string, content []byte) (string, error) {
	cachePath := Path(cacheDir, sourceRel)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(cachePath, content, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(cachePath), nil
}
