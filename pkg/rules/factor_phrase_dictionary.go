package rules

import (
	"fmt"
	"sort"
	"strings"
)

// FactorPhraseDictionary mines per-document phrases (3–6 whitespace-split words)
// that repeat ≥ minOccurrences times and replaces each with a short token
// (T1, T2, …), emitting a single `[gloss] T1=phrase; T2=phrase; ...` preamble
// at the top of the document. Net-byte-savings guard: if the rewrite would
// not save bytes, the rule no-ops.
//
// The rule is conservative: it skips fenced code blocks, ATX headings,
// list markers, and phrases that contain markdown special chars. Words must
// be ≥3 ASCII letters to avoid factoring out junk like "the cat ate".
type FactorPhraseDictionary struct{}

func (r *FactorPhraseDictionary) Name() string { return "factor-phrase-dictionary" }
func (r *FactorPhraseDictionary) Tier() Tier   { return TierAggressive }

const (
	phraseMinWords      = 3
	phraseMaxWords      = 6
	phraseMinOccurs     = 5
	phraseMinByteLength = 15
	phraseMaxAbbrevs    = 64
)

// markdownReservedChars are characters that, if they appear in a candidate
// phrase, disqualify it. Keeps the rule away from links, images, fences,
// emphasis, autolinks, and HTML.
const markdownReservedChars = "[](){}*_`<>|#~"

func (r *FactorPhraseDictionary) Apply(ctx *Context) (ChangeSet, error) {
	source := ctx.Source
	lines := sourceLines(source)

	// 1. Collect eligible spans of running words across the doc, with their
	//    absolute byte offsets. Each "run" is a contiguous sequence of words
	//    on a single eligible line.
	runs := collectPhraseRuns(source, lines)
	if len(runs) == 0 {
		return ChangeSet{}, nil
	}

	// Tree-wide modes (when CrossFile state is present):
	//   - Mine mode: record corpus-wide n-gram observations; emit no edits.
	//   - Apply mode: substitute using a pre-built glossary instead of
	//     mining per-file. The single glossary preamble is emitted in the
	//     anchor file designated by the caller.
	if ctx.CrossFile != nil {
		if ctx.CrossFile.PhraseMineMode {
			return r.minePhrases(ctx.CrossFile, ctx.Source, runs)
		}
		if len(ctx.CrossFile.PhraseGlossary) > 0 {
			return r.applyTreeGlossary(ctx, runs)
		}
	}

	// 2. Mine n-grams of length 3..6 across all runs. Track count and
	//    occurrence positions.
	type occurrence struct {
		startByte int
		endByte   int
	}
	type ngram struct {
		text        string
		occurrences []occurrence
	}
	ngrams := make(map[string]*ngram)
	for _, run := range runs {
		for n := phraseMinWords; n <= phraseMaxWords; n++ {
			if len(run.words) < n {
				break
			}
			for i := 0; i+n <= len(run.words); i++ {
				slice := run.words[i : i+n]
				phrase := joinWords(slice, source)
				if !eligiblePhrase(phrase) {
					continue
				}
				start := slice[0].start
				end := slice[n-1].end
				ng, ok := ngrams[phrase]
				if !ok {
					ng = &ngram{text: phrase}
					ngrams[phrase] = ng
				}
				ng.occurrences = append(ng.occurrences, occurrence{start, end})
			}
		}
	}

	// 3. Filter to phrases meeting min occurrences. Sort by gross byte savings
	//    descending so longer/more-frequent phrases are picked first.
	type candidate struct {
		text   string
		occs   []occurrence
		bytes  int // gross byte savings if abbreviated to "T1" (2-byte abbrev)
	}
	candidates := make([]candidate, 0, len(ngrams))
	for text, ng := range ngrams {
		if len(ng.occurrences) < phraseMinOccurs {
			continue
		}
		// Approximate gross savings using a 2-byte abbreviation. Real abbrev
		// length depends on index (T1..T9 = 2 bytes, T10..T99 = 3). We use 2
		// as the optimistic bound for ranking; the net-savings guard at the
		// end catches under-shoots.
		gross := len(ng.occurrences) * (len(text) - 2)
		candidates = append(candidates, candidate{text: text, occs: ng.occurrences, bytes: gross})
	}
	if len(candidates) == 0 {
		return ChangeSet{}, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].bytes != candidates[j].bytes {
			return candidates[i].bytes > candidates[j].bytes
		}
		// Stable secondary sort for determinism across runs.
		return candidates[i].text < candidates[j].text
	})

	// 4. Greedy selection with non-overlap. Track byte ranges already claimed
	//    by a higher-priority phrase; skip occurrences that overlap.
	var (
		picks     []claimedRange
		glossary  []string
		netBytes  int // byte savings less the glossary cost
		abbrevIdx = 1
	)
	for _, cand := range candidates {
		if abbrevIdx > phraseMaxAbbrevs {
			break
		}
		// Collect occurrences that don't overlap any previously picked range.
		var keep []occurrence
		for _, occ := range cand.occs {
			if rangeOverlapsAny(occ.startByte, occ.endByte, picks) {
				continue
			}
			keep = append(keep, occ)
		}
		if len(keep) < phraseMinOccurs {
			continue
		}
		abbrev := fmt.Sprintf("T%d", abbrevIdx)
		// Per-phrase savings net of glossary entry cost.
		// Replacement saves len(text)-len(abbrev) per occurrence; glossary
		// entry costs len(abbrev) + 1 ('=') + len(text) + 2 ('; ') bytes.
		perOccSave := len(cand.text) - len(abbrev)
		entryCost := len(abbrev) + 1 + len(cand.text) + 2
		net := len(keep)*perOccSave - entryCost
		if net <= 0 {
			continue
		}
		for _, occ := range keep {
			picks = append(picks, claimedRange{start: occ.startByte, end: occ.endByte, abbrev: abbrev})
		}
		glossary = append(glossary, fmt.Sprintf("%s=%s", abbrev, cand.text))
		netBytes += net
		abbrevIdx++
	}

	// 5. Net-savings guard. The preamble itself adds "[gloss] " (8 bytes) plus
	//    a trailing newline. Bail if the rewrite is a wash or worse.
	preambleOverhead := len("[gloss] ") + 1 // newline
	if len(picks) == 0 || netBytes-preambleOverhead <= 0 {
		return ChangeSet{}, nil
	}

	// 6. Emit edits: a glossary insert at byte 0 plus one replacement per
	//    claimed range.
	var changes ChangeSet
	preamble := "[gloss] " + strings.Join(glossary, "; ") + "\n"
	addReplacement(&changes, 0, 0, preamble)
	for _, p := range picks {
		addReplacement(&changes, p.start, p.end, p.abbrev)
	}
	return changes, nil
}

// collectPhraseRuns walks the source and returns one run per eligible line:
// not in a fence, not an ATX heading, not a list marker line. Each run holds
// the byte offsets of words on that line.
type wordRef struct {
	start, end int
}
type phraseRun struct {
	words []wordRef
}

func collectPhraseRuns(source []byte, lines []sourceLine) []phraseRun {
	runs := make([]phraseRun, 0, len(lines))
	for _, line := range lines {
		if line.InFence {
			continue
		}
		if !lineEligibleForPhraseFactoring(line.Text) {
			continue
		}
		words := tokenizeWords(source, line)
		if len(words) < phraseMinWords {
			continue
		}
		runs = append(runs, phraseRun{words: words})
	}
	return runs
}

// lineEligibleForPhraseFactoring returns true for plain-prose lines.
// Excludes ATX headings (#), list markers (-, *, +, ordered), blockquotes (>),
// table rows (|), and lines that are entirely inline-code or links.
func lineEligibleForPhraseFactoring(text string) bool {
	trimmed := strings.TrimLeft(text, " \t")
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '#', '>', '|':
		return false
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return false
	}
	// Ordered list: digits then ". ".
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && c == '.' && i+1 < len(trimmed) && trimmed[i+1] == ' ' {
			return false
		}
		break
	}
	// Skip lines that look like a single link or are mostly URL.
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return false
	}
	return true
}

// tokenizeWords splits a line into whitespace-separated word tokens, returning
// absolute byte offsets into source. Skips inline-code spans (between
// backticks) so we don't factor over code identity.
func tokenizeWords(source []byte, line sourceLine) []wordRef {
	var out []wordRef
	inCode := false
	i := line.Start
	for i < line.End {
		c := source[i]
		if c == '`' {
			inCode = !inCode
			i++
			continue
		}
		if inCode {
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' {
			i++
			continue
		}
		start := i
		for i < line.End && source[i] != ' ' && source[i] != '\t' && source[i] != '\n' && source[i] != '`' {
			i++
		}
		out = append(out, wordRef{start: start, end: i})
	}
	return out
}

// joinWords concatenates the source bytes spanning the given word refs, using
// a single ASCII space as separator. We rebuild the canonical phrase text
// rather than slicing source[first.start:last.end] so phrases that share
// content but differ only in inter-word whitespace count as the same phrase.
func joinWords(words []wordRef, source []byte) string {
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.Write(source[w.start:w.end])
	}
	return b.String()
}

func eligiblePhrase(phrase string) bool {
	if len(phrase) < phraseMinByteLength {
		return false
	}
	if strings.ContainsAny(phrase, markdownReservedChars) {
		return false
	}
	// Each whitespace-split word must be ≥3 ASCII letters (allowing trailing
	// punctuation). Filters out "of the file" and similar function-word soup.
	for _, w := range strings.Fields(phrase) {
		letters := 0
		for j := 0; j < len(w); j++ {
			c := w[j]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				letters++
			}
		}
		if letters < 3 {
			return false
		}
	}
	return true
}

type claimedRange struct {
	start, end int
	abbrev     string
}

func rangeOverlapsAny(start, end int, picks []claimedRange) bool {
	for _, p := range picks {
		if start < p.end && end > p.start {
			return true
		}
	}
	return false
}

// minePhrases is pass 1 of the tree-wide phrase dictionary build. Records
// every eligible n-gram occurrence into CrossFile.PhraseObservations.
// Emits no edits — substitution happens during pass 2.
func (r *FactorPhraseDictionary) minePhrases(cfs *CrossFileState, source []byte, runs []phraseRun) (ChangeSet, error) {
	for _, run := range runs {
		for n := phraseMinWords; n <= phraseMaxWords; n++ {
			if len(run.words) < n {
				break
			}
			for i := 0; i+n <= len(run.words); i++ {
				slice := run.words[i : i+n]
				phrase := joinWords(slice, source)
				if !eligiblePhrase(phrase) {
					continue
				}
				cfs.RecordPhraseObservation(phrase)
			}
		}
	}
	return ChangeSet{}, nil
}

// applyTreeGlossary substitutes pre-mined glossary phrases. On the anchor
// file (designated by the caller) the rule additionally inserts a single
// [gloss] preamble listing every assigned abbreviation.
func (r *FactorPhraseDictionary) applyTreeGlossary(ctx *Context, runs []phraseRun) (ChangeSet, error) {
	cfs := ctx.CrossFile
	if cfs == nil || len(cfs.PhraseGlossary) == 0 {
		return ChangeSet{}, nil
	}
	source := ctx.Source
	var picks []claimedRange
	for _, run := range runs {
		// Try longer phrases first within each run so a 6-gram win is not
		// preempted by a 3-gram subset.
		for n := phraseMaxWords; n >= phraseMinWords; n-- {
			if len(run.words) < n {
				continue
			}
			for i := 0; i+n <= len(run.words); i++ {
				slice := run.words[i : i+n]
				phrase := joinWords(slice, source)
				abbrev, ok := cfs.PhraseGlossary[phrase]
				if !ok {
					continue
				}
				start := slice[0].start
				end := slice[n-1].end
				if rangeOverlapsAny(start, end, picks) {
					continue
				}
				picks = append(picks, claimedRange{start: start, end: end, abbrev: abbrev})
			}
		}
	}
	if len(picks) == 0 && ctx.FilePath != cfs.PhraseGlossaryAnchor {
		return ChangeSet{}, nil
	}
	var changes ChangeSet
	if ctx.FilePath == cfs.PhraseGlossaryAnchor {
		var b strings.Builder
		b.WriteString("[gloss] ")
		for i, phrase := range cfs.PhraseGlossaryOrder {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(cfs.PhraseGlossary[phrase])
			b.WriteByte('=')
			b.WriteString(phrase)
		}
		b.WriteByte('\n')
		addReplacement(&changes, 0, 0, b.String())
	}
	for _, p := range picks {
		addReplacement(&changes, p.start, p.end, p.abbrev)
	}
	return changes, nil
}
