// Package rules contains deterministic markdown compression rules.
package rules

import (
	"sync"

	"github.com/dhruv1794/mdcompress/pkg/render"
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

	SectionHashes        map[string]string
	SeenSections         map[string]CrossFileSection
	StructuralSections   map[string]CrossFileSection
	CodeBlocks           map[string]CrossFileCodeBlock
	StructuralCodeBlocks map[string]CrossFileCodeBlock
}

type CrossFileSection struct {
	CanonicalFile  string
	SectionHeading string
	ContentHash    string
	ByteLength     int
}

type CrossFileCodeBlock struct {
	CanonicalFile string
	ContentHash   string
	StartLine     int
	ByteLength    int
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
		CanonicalFile: file,
		ContentHash:   hash,
		ByteLength:    length,
	}
	return true
}

func (cfs *CrossFileState) RecordStructuralSection(structuralHash, file, heading string, length int) (CrossFileSection, bool) {
	if cfs == nil {
		return CrossFileSection{}, false
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.StructuralSections == nil {
		cfs.StructuralSections = make(map[string]CrossFileSection)
	}
	if existing, ok := cfs.StructuralSections[structuralHash]; ok {
		return existing, existing.CanonicalFile != file
	}
	cfs.StructuralSections[structuralHash] = CrossFileSection{
		CanonicalFile:  file,
		SectionHeading: heading,
		ContentHash:    structuralHash,
		ByteLength:     length,
	}
	return CrossFileSection{}, false
}

func (cfs *CrossFileState) RecordStructuralCodeBlock(structuralHash, file string, startLine, length int) (CrossFileCodeBlock, bool) {
	if cfs == nil {
		return CrossFileCodeBlock{}, false
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.StructuralCodeBlocks == nil {
		cfs.StructuralCodeBlocks = make(map[string]CrossFileCodeBlock)
	}
	if existing, ok := cfs.StructuralCodeBlocks[structuralHash]; ok {
		return existing, existing.CanonicalFile != file
	}
	cfs.StructuralCodeBlocks[structuralHash] = CrossFileCodeBlock{
		CanonicalFile: file,
		ContentHash:   structuralHash,
		StartLine:     startLine,
		ByteLength:    length,
	}
	return CrossFileCodeBlock{}, false
}

func (cfs *CrossFileState) RecordCodeBlock(hash, file string, startLine, length int) (CrossFileCodeBlock, bool) {
	if cfs == nil {
		return CrossFileCodeBlock{}, true
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.CodeBlocks == nil {
		cfs.CodeBlocks = make(map[string]CrossFileCodeBlock)
	}
	if existing, ok := cfs.CodeBlocks[hash]; ok {
		return existing, existing.CanonicalFile == file
	}
	block := CrossFileCodeBlock{
		CanonicalFile: file,
		ContentHash:   hash,
		StartLine:     startLine,
		ByteLength:    length,
	}
	cfs.CodeBlocks[hash] = block
	return block, true
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
	Source    []byte
	Config    *Config
	FilePath  string
	CrossFile *CrossFileState
}

// Config contains the active rule configuration.
type Config struct {
	Tier              Tier
	Disabled          map[string]bool
	CodeBlockMaxLines int
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

// Rule is the common metadata contract implemented by every compression rule.
type Rule interface {
	Name() string
	Tier() Tier
}

// LineRule reads source bytes and emits byte-range edits. All rules in this
// package are LineRules; the engine no longer parses to an AST.
type LineRule interface {
	Rule
	Apply(ctx *Context) (ChangeSet, error)
}
