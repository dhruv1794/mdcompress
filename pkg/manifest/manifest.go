// Package manifest reads and writes the mdcompress manifest file.
package manifest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	Version     = 1
	DefaultPath = ".mdcompress/manifest.json"
)

type Manifest struct {
	Version     int              `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	Totals      Totals           `json:"totals"`
	Entries     map[string]Entry `json:"entries"`
}

type Totals struct {
	Files        int `json:"files"`
	TokensBefore int `json:"tokens_before"`
	TokensAfter  int `json:"tokens_after"`
	TokensSaved  int `json:"tokens_saved"`
}

type Entry struct {
	Source       string         `json:"source"`
	Cache        string         `json:"cache"`
	SHA256       string         `json:"sha256"`
	TokensBefore int            `json:"tokens_before"`
	TokensAfter  int            `json:"tokens_after"`
	CompressedAt time.Time      `json:"compressed_at"`
	RulesFired   map[string]int `json:"rules_fired"`
}

func New() *Manifest {
	return &Manifest{
		Version:     Version,
		GeneratedAt: time.Now().UTC(),
		Entries:     make(map[string]Entry),
	}
}

func Read(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Entries == nil {
		m.Entries = make(map[string]Entry)
	}
	if m.Version == 0 {
		m.Version = Version
	}
	m.RecalculateTotals()
	return &m, nil
}

func Write(path string, m *Manifest) error {
	if m == nil {
		m = New()
	}
	m.Version = Version
	m.GeneratedAt = time.Now().UTC()
	m.RecalculateTotals()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (m *Manifest) RecalculateTotals() {
	var totals Totals
	for _, entry := range m.Entries {
		totals.Files++
		totals.TokensBefore += entry.TokensBefore
		totals.TokensAfter += entry.TokensAfter
	}
	totals.TokensSaved = totals.TokensBefore - totals.TokensAfter
	m.Totals = totals
}
