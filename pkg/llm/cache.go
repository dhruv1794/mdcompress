package llm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// DefaultCacheDir is the on-disk location for cached LLM responses, relative
// to the project root. Stored under .mdcompress/cache/.llm-cache/ so it is
// gitignored alongside the rest of the cache by default.
const DefaultCacheDir = ".mdcompress/cache/.llm-cache"

// CacheEntry is the persisted form of a cached rewrite.
type CacheEntry struct {
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	Section  string  `json:"section_sha"`
	Original string  `json:"original"`
	Rewrite  string  `json:"rewrite"`
	Score    float64 `json:"score"`
}

// Cache stores rewrite responses on disk plus a small in-memory hot map.
type Cache struct {
	dir string
	mu  sync.Mutex
	mem map[string]CacheEntry

	hits   int
	misses int
	writes int
}

// NewCache creates a cache rooted at dir. Pass an empty dir to use the
// DefaultCacheDir.
func NewCache(dir string) *Cache {
	if dir == "" {
		dir = DefaultCacheDir
	}
	return &Cache{dir: dir, mem: make(map[string]CacheEntry)}
}

// Get returns the cached entry for key. Disk is read on the first miss for
// each key.
func (c *Cache) Get(key string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.mem[key]; ok {
		c.hits++
		return entry, true
	}
	path := filepath.Join(c.dir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		c.misses++
		return CacheEntry{}, false
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		c.misses++
		return CacheEntry{}, false
	}
	c.mem[key] = entry
	c.hits++
	return entry, true
}

// Put writes an entry to disk and the in-memory hot map.
func (c *Cache) Put(key string, entry CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem[key] = entry
	if c.dir == "" {
		return errors.New("llm cache: empty dir")
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	c.writes++
	return os.WriteFile(filepath.Join(c.dir, key+".json"), data, 0o644)
}

// Stats returns the cache hit/miss counters since the cache was created.
func (c *Cache) Stats() (hits, misses, writes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, c.writes
}

// Dir returns the cache root directory.
func (c *Cache) Dir() string { return c.dir }
