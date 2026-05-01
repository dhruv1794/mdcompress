// Package rules contains deterministic markdown compression rules.
package rules

import (
	"sync"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

// Tier identifies a rule's risk level.
type Tier int

const (
	TierSafe Tier = iota + 1
	TierAggressive
	TierLLM
)

func (t Tier) String() string {
	switch t {
	case TierSafe:
		return "safe"
	case TierAggressive:
		return "aggressive"
	case TierLLM:
		return "llm"
	default:
		return "unknown"
	}
}

// CrossFileState carries shared state across compression runs on multiple
// files, enabling cross-file deduplication and reference tracking.
type CrossFileState struct {
	mu sync.Mutex

	SectionHashes map[string]string
	SeenSections  map[string]CrossFileSection
}

type CrossFileSection struct {
	CanonicalFile   string
	SectionHeading  string
	ContentHash     string
	ByteLength      int
}

func (cfs *CrossFileState) RecordSection(hash, file, heading string, length int) bool {
	if cfs == nil {
		return false
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.SectionHashes == nil {
		cfs.SectionHashes = make(map[string]string)
		cfs.SeenSections = make(map[string]CrossFileSection)
	}
	if existing, ok := cfs.SectionHashes[hash]; ok && existing != file {
		return false
	}
	cfs.SectionHashes[hash] = file
	cfs.SeenSections[hash] = CrossFileSection{
		CanonicalFile:  file,
		ContentHash:    hash,
		ByteLength:     length,
	}
	return true
}

func (cfs *CrossFileSection) ReferenceFile() string {
	if cfs == nil {
		return ""
	}
	return cfs.CanonicalFile
}

// Context is passed to every rule. Rules read Source and return byte ranges
// to remove; they should not mutate the source bytes.
type Context struct {
	Source     []byte
	Config     *Config
	FilePath   string
	CrossFile  *CrossFileState
}

// Config contains the active rule configuration.
type Config struct {
	Tier     Tier
	Disabled map[string]bool
}

// Stats reports a rule's effect.
type Stats struct {
	NodesAffected int
	BytesSaved    int
}

// ChangeSet describes source edits made by a rule.
type ChangeSet struct {
	Edits  []render.Edit
	Ranges []render.Range
	Stats  Stats
}

// Rule is the contract implemented by every compression rule.
type Rule interface {
	Name() string
	Tier() Tier
	Apply(doc ast.Node, ctx *Context) (ChangeSet, error)
}
