// Package rules contains deterministic markdown compression rules.
package rules

import (
	"fmt"
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
	SeenParagraphs       map[string]CrossFileParagraph

	// Tree-wide phrase dictionary. Populated across two passes:
	//   pass 1: factor-phrase-dictionary, when PhraseMineMode is true,
	//           calls RecordPhraseObservation for each candidate phrase
	//           instead of substituting.
	//   between passes: caller invokes BuildPhraseGlossary to pick winners.
	//   pass 2: factor-phrase-dictionary, when PhraseGlossary is non-empty,
	//           applies the pre-built glossary instead of mining per-file.
	PhraseMineMode      bool
	PhraseObservations  map[string]int    // phrase → corpus-wide count
	PhraseGlossary      map[string]string // phrase → abbreviation ("T1"…)
	PhraseGlossaryOrder []string          // phrases ordered by selection priority
	PhraseGlossaryAnchor string           // file that receives the [gloss] preamble
}

type CrossFileParagraph struct {
	CanonicalFile string
	ContentHash   string
	ByteLength    int
	Count         int
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

// RecordParagraph counts occurrences of a normalized-paragraph hash across
// files. Returns the canonical entry (the first file that recorded the
// paragraph) and the post-call count. Callers replace the paragraph with a
// reference once count >= 3 — until then we don't yet know whether the
// paragraph repeats enough to be worth factoring.
func (cfs *CrossFileState) RecordParagraph(hash, file string, length int) (CrossFileParagraph, int) {
	if cfs == nil {
		return CrossFileParagraph{}, 0
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.SeenParagraphs == nil {
		cfs.SeenParagraphs = make(map[string]CrossFileParagraph)
	}
	if existing, ok := cfs.SeenParagraphs[hash]; ok {
		existing.Count++
		cfs.SeenParagraphs[hash] = existing
		return existing, existing.Count
	}
	p := CrossFileParagraph{
		CanonicalFile: file,
		ContentHash:   hash,
		ByteLength:    length,
		Count:         1,
	}
	cfs.SeenParagraphs[hash] = p
	return p, 1
}

// RecordPhraseObservation increments the corpus-wide occurrence count of
// the given phrase. Used during pass 1 of the tree-wide phrase dictionary
// build.
func (cfs *CrossFileState) RecordPhraseObservation(phrase string) {
	if cfs == nil || phrase == "" {
		return
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if cfs.PhraseObservations == nil {
		cfs.PhraseObservations = make(map[string]int)
	}
	cfs.PhraseObservations[phrase]++
}

// BuildPhraseGlossary picks the highest-value phrases from the corpus-wide
// observations and assigns abbreviations T1..Tn. Greedy by gross byte
// savings, capped at maxAbbrevs and gated on minOccurs. Returns the
// number of abbreviations assigned. Anchor is the file path that will
// receive the glossary preamble during pass 2.
func (cfs *CrossFileState) BuildPhraseGlossary(minOccurs, maxAbbrevs int, anchor string) int {
	if cfs == nil {
		return 0
	}
	cfs.mu.Lock()
	defer cfs.mu.Unlock()
	if len(cfs.PhraseObservations) == 0 {
		return 0
	}
	type cand struct {
		text  string
		count int
		gross int
	}
	cands := make([]cand, 0, len(cfs.PhraseObservations))
	for text, count := range cfs.PhraseObservations {
		if count < minOccurs {
			continue
		}
		// Approximate savings: each occurrence saves (len(text)-2) when
		// replaced with a 2-byte abbreviation. We refine in selection below
		// using actual abbreviation length and net-savings math.
		cands = append(cands, cand{text: text, count: count, gross: count * (len(text) - 2)})
	}
	if len(cands) == 0 {
		return 0
	}
	// Stable sort: gross savings desc, then text asc.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && lessCand(cands[j], cands[j-1]); j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	cfs.PhraseGlossary = make(map[string]string, maxAbbrevs)
	cfs.PhraseGlossaryOrder = make([]string, 0, maxAbbrevs)
	cfs.PhraseGlossaryAnchor = anchor
	for _, c := range cands {
		if len(cfs.PhraseGlossary) >= maxAbbrevs {
			break
		}
		idx := len(cfs.PhraseGlossary) + 1
		abbrev := fmt.Sprintf("T%d", idx)
		perOccSave := len(c.text) - len(abbrev)
		entryCost := len(abbrev) + 1 + len(c.text) + 2 // "T1=phrase; "
		net := c.count*perOccSave - entryCost
		if net <= 0 {
			continue
		}
		cfs.PhraseGlossary[c.text] = abbrev
		cfs.PhraseGlossaryOrder = append(cfs.PhraseGlossaryOrder, c.text)
	}
	return len(cfs.PhraseGlossary)
}

// lessCand orders candidates: higher gross savings first, then alphabetical
// for determinism.
func lessCand(a, b struct {
	text  string
	count int
	gross int
}) bool {
	if a.gross != b.gross {
		return a.gross > b.gross
	}
	return a.text < b.text
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
