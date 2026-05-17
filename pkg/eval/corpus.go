package eval

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

// Tuple is one curated (doc, question, expected_answer) entry. Authored by
// hand or sampled from canonical docs; loaded from a JSONL file.
type Tuple struct {
	ID             string `json:"id"`
	Repo           string `json:"repo"`
	File           string `json:"file"`
	Question       string `json:"question"`
	ExpectedAnswer string `json:"expected_answer"`
}

// Verdict is the judge's classification of a compressed answer relative to
// the original-document answer and the curator's expected_answer.
type Verdict string

const (
	VerdictPass     Verdict = "pass"
	VerdictDegraded Verdict = "degraded"
	VerdictFail     Verdict = "fail"
)

// CorpusOptions controls a curated eval run.
type CorpusOptions struct {
	CorpusPath string
	RepoRoot   string
	Filter     string
	Compress   compress.Options
	Backend    Backend
	JudgeModel string

	CacheDir string
	NoCache  bool
}

// CorpusTupleResult holds the per-tuple outcome.
type CorpusTupleResult struct {
	Tuple            Tuple   `json:"tuple"`
	Verdict          Verdict `json:"verdict"`
	Reason           string  `json:"reason,omitempty"`
	OriginalAnswer   string  `json:"original_answer"`
	CompressedAnswer string  `json:"compressed_answer"`
	BytesBefore      int     `json:"bytes_before"`
	BytesAfter       int     `json:"bytes_after"`
	TokensBefore     int     `json:"tokens_before"`
	TokensAfter      int     `json:"tokens_after"`
	JudgeFromCache   bool    `json:"judge_from_cache"`
	OriginalCached   bool    `json:"original_cached"`
	CompressedCached bool    `json:"compressed_cached"`
	// Suspect is true when the responder failed to find the answer in the
	// ORIGINAL document. Such tuples are not measuring compression damage —
	// either the curator's question is misphrased for this responder, or the
	// expected_answer is wrong. Separate from the pass/degraded/fail axis.
	Suspect       bool   `json:"suspect"`
	SuspectReason string `json:"suspect_reason,omitempty"`
	// LLM holds the Tier-3 rewriter stats for this tuple's compression.
	// Zero unless the run is tier=llm with a rewriter attached.
	LLM compress.LLMRewriteStats `json:"llm"`
}

// CorpusReport aggregates verdicts across the run.
type CorpusReport struct {
	GeneratedAt    time.Time               `json:"generated_at"`
	CorpusPath     string                  `json:"corpus_path"`
	Backend        string                  `json:"backend"`
	ResponderModel string                  `json:"responder_model"`
	JudgeModel     string                  `json:"judge_model"`
	Tier           string                  `json:"tier"`
	Tuples         []CorpusTupleResult     `json:"tuples"`
	ByRepo         map[string]VerdictTally `json:"by_repo"`
	Totals         VerdictTally            `json:"totals"`
	DurationMS     int64                   `json:"duration_ms"`
	APICallCount   int                     `json:"api_call_count"`
	// LLM aggregates the Tier-3 rewriter activity across all tuples.
	LLM compress.LLMRewriteStats `json:"llm"`
}

// VerdictTally counts pass/degraded/fail.
type VerdictTally struct {
	Pass     int `json:"pass"`
	Degraded int `json:"degraded"`
	Fail     int `json:"fail"`
	Total    int `json:"total"`
}

// PassRate returns pass/total as a fraction in [0,1].
func (v VerdictTally) PassRate() float64 {
	if v.Total == 0 {
		return 0
	}
	return float64(v.Pass) / float64(v.Total)
}

// LoadCorpus reads a JSONL corpus file.
func LoadCorpus(path string) ([]Tuple, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tuples []Tuple
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var t Tuple
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			return nil, fmt.Errorf("parse tuple: %w", err)
		}
		if t.ID == "" || t.File == "" || t.Question == "" {
			return nil, fmt.Errorf("tuple missing required fields: %s", line)
		}
		tuples = append(tuples, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return tuples, nil
}

// RunCorpus runs the curated eval and returns a report.
func RunCorpus(opts CorpusOptions) (CorpusReport, error) {
	if opts.Backend == nil {
		return CorpusReport{}, fmt.Errorf("backend is required")
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot = "/tmp/bench"
	}
	if opts.CacheDir == "" {
		opts.CacheDir = ".mdcompress/cache/eval"
	}
	tuples, err := LoadCorpus(opts.CorpusPath)
	if err != nil {
		return CorpusReport{}, err
	}
	if opts.Filter != "" {
		tuples = filterTuples(tuples, opts.Filter)
	}

	cache := newCorpusCache(opts.CacheDir, opts.NoCache)
	responderModel := backendModel(opts.Backend)

	report := CorpusReport{
		GeneratedAt:    time.Now().UTC(),
		CorpusPath:     opts.CorpusPath,
		Backend:        opts.Backend.Name(),
		ResponderModel: responderModel,
		JudgeModel:     opts.JudgeModel,
		Tier:           opts.Compress.Tier.String(),
		ByRepo:         make(map[string]VerdictTally),
	}
	if opts.Compress.Tier == 0 {
		report.Tier = compress.TierAggressive.String()
	}

	start := time.Now()
	for _, t := range tuples {
		fileResult, calls, err := runTuple(t, opts, cache, responderModel)
		report.APICallCount += calls
		if err != nil {
			return report, fmt.Errorf("tuple %s: %w", t.ID, err)
		}
		report.Tuples = append(report.Tuples, fileResult)
		report.LLM.Add(fileResult.LLM)
		tally := report.ByRepo[t.Repo]
		tally.Total++
		switch fileResult.Verdict {
		case VerdictPass:
			tally.Pass++
		case VerdictDegraded:
			tally.Degraded++
		case VerdictFail:
			tally.Fail++
		}
		report.ByRepo[t.Repo] = tally
		report.Totals.Total++
		switch fileResult.Verdict {
		case VerdictPass:
			report.Totals.Pass++
		case VerdictDegraded:
			report.Totals.Degraded++
		case VerdictFail:
			report.Totals.Fail++
		}
	}
	report.DurationMS = time.Since(start).Milliseconds()
	return report, nil
}

func runTuple(t Tuple, opts CorpusOptions, cache *corpusCache, responderModel string) (CorpusTupleResult, int, error) {
	var calls int
	path := filepath.Join(opts.RepoRoot, t.Repo, t.File)
	original, err := os.ReadFile(path)
	if err != nil {
		return CorpusTupleResult{Tuple: t}, 0, fmt.Errorf("read %s: %w", path, err)
	}
	compressOpts := opts.Compress
	compressOpts.FilePath = t.File
	compressed, err := compress.Compress(original, compressOpts)
	if err != nil {
		return CorpusTupleResult{Tuple: t}, 0, fmt.Errorf("compress: %w", err)
	}

	result := CorpusTupleResult{
		Tuple:        t,
		BytesBefore:  compressed.BytesBefore,
		BytesAfter:   compressed.BytesAfter,
		TokensBefore: compressed.TokensBefore,
		TokensAfter:  compressed.TokensAfter,
		LLM:          compressed.LLM,
	}

	originalAnswer, originalCached, originalCalls, err := cachedAnswer(opts.Backend, original, t.Question, responderModel, "original", cache)
	if err != nil {
		return result, originalCalls, fmt.Errorf("answer original: %w", err)
	}
	calls += originalCalls
	result.OriginalAnswer = originalAnswer
	result.OriginalCached = originalCached
	if isNotFoundAnswer(originalAnswer) {
		result.Suspect = true
		result.SuspectReason = "responder said 'not found' on original document — tuple is not measuring compression"
	}

	compressedAnswer, compressedCached, compressedCalls, err := cachedAnswer(opts.Backend, compressed.Output, t.Question, responderModel, "compressed", cache)
	if err != nil {
		return result, calls + compressedCalls, fmt.Errorf("answer compressed: %w", err)
	}
	calls += compressedCalls
	result.CompressedAnswer = compressedAnswer
	result.CompressedCached = compressedCached

	verdict, reason, fromCache, judgeCalls, err := cachedVerdict(opts.Backend, opts.JudgeModel, t, original, compressed.Output, originalAnswer, compressedAnswer, cache)
	calls += judgeCalls
	if err != nil {
		return result, calls, fmt.Errorf("judge: %w", err)
	}
	result.Verdict = verdict
	result.Reason = reason
	result.JudgeFromCache = fromCache
	return result, calls, nil
}

func cachedAnswer(backend Backend, doc []byte, question, model, side string, cache *corpusCache) (string, bool, int, error) {
	key := hashKey("answer-"+side, doc, []byte(question), []byte(model))
	if cached, ok := cache.GetString(key); ok {
		return cached, true, 0, nil
	}
	prompt := answerPrompt(string(doc), question)
	raw, err := backend.Complete(prompt)
	if err != nil {
		return "", false, 1, err
	}
	answer := strings.TrimSpace(raw)
	if err := cache.PutString(key, answer); err != nil {
		return answer, false, 1, err
	}
	return answer, false, 1, nil
}

func answerPrompt(doc, question string) string {
	return fmt.Sprintf(`Answer the question using only the document below.
If the document does not contain the answer, say "not found".
Keep the answer concise and factual.

Question: %s

Document:
%s
%s
%s`, question, "```markdown", doc, "```")
}

func cachedVerdict(backend Backend, judgeModel string, t Tuple, original, compressed []byte, originalAnswer, compressedAnswer string, cache *corpusCache) (Verdict, string, bool, int, error) {
	// promptVersion is part of the cache key so that prompt revisions invalidate
	// stale verdicts without manual cache clears.
	const promptVersion = "v2"
	key := hashKey("verdict", original, compressed, []byte(t.Question), []byte(t.ExpectedAnswer), []byte(judgeModel), []byte(promptVersion))
	type entry struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	var ent entry
	if cache.GetJSON(key, &ent) {
		return Verdict(ent.Verdict), ent.Reason, true, 0, nil
	}
	// Short-circuit: when the responder's answer is character-identical on both
	// sides (after whitespace trim), compression cannot be at fault. Skip the
	// judge call and emit pass directly. Saves API spend and removes the
	// false-degraded signal that arises when the judge compares against the
	// curator's expected_answer rather than the original answer.
	if strings.TrimSpace(originalAnswer) == strings.TrimSpace(compressedAnswer) && strings.TrimSpace(originalAnswer) != "" {
		ent = entry{Verdict: string(VerdictPass), Reason: "compressed answer is identical to original answer (short-circuit, no judge call)"}
		if err := cache.PutJSON(key, ent); err != nil {
			return VerdictPass, ent.Reason, false, 0, err
		}
		return VerdictPass, ent.Reason, false, 0, nil
	}
	prompt := verdictPrompt(t, originalAnswer, compressedAnswer)
	raw, err := backend.Complete(prompt)
	if err != nil {
		return "", "", false, 1, err
	}
	verdict, reason, err := parseVerdict(raw)
	if err != nil {
		return "", "", false, 1, fmt.Errorf("parse verdict: %w (raw=%q)", err, truncate(raw, 400))
	}
	ent = entry{Verdict: string(verdict), Reason: reason}
	if err := cache.PutJSON(key, ent); err != nil {
		return verdict, reason, false, 1, err
	}
	return verdict, reason, false, 1, nil
}

func verdictPrompt(t Tuple, originalAnswer, compressedAnswer string) string {
	return fmt.Sprintf(`You are judging COMPRESSION FAITHFULNESS — whether markdown compression preserved enough information for a downstream reader to answer a question as well as they could from the original document.

The reference is the ORIGINAL-side answer, not the curator's expected answer.
The curator's expected answer is provided only as a tiebreaker when the original-side answer is itself ambiguous.

Question: %s

Answer the model gave when reading the ORIGINAL document:
%s

Answer the model gave when reading the COMPRESSED document:
%s

Curator's expected answer (tiebreaker only):
%s

Classify the COMPRESSED answer relative to the ORIGINAL answer:
- "pass": the compressed answer carries the same factual content as the original answer. Different wording, different sentence structure, or extra/fewer words are fine as long as the core facts match.
- "degraded": the compressed answer is missing a specific fact, named entity, or qualifier that the original answer contained. Still partially correct, but lossy.
- "fail": the compressed answer contradicts the original, says "not found" while the original found it, or omits the core requested fact entirely.

Special rules:
- If the ORIGINAL answer is itself "not found", the tuple is mis-curated and you should return "pass" (compression cannot make recall worse than the original).
- Treat phrasing differences (active/passive, synonyms, paraphrase) as "pass" as long as the facts match. Penalize only missing or contradictory facts.

Return JSON only, in this exact shape, with no commentary outside the JSON:
{"verdict":"pass|degraded|fail","reason":"one short sentence explaining the call"}`, t.Question, originalAnswer, compressedAnswer, t.ExpectedAnswer)
}

func parseVerdict(raw string) (Verdict, string, error) {
	var ent struct {
		Verdict string `json:"verdict"`
		Reason  string `json:"reason"`
	}
	candidate := extractJSON(raw)
	if err := json.Unmarshal([]byte(candidate), &ent); err == nil {
		v := strings.ToLower(strings.TrimSpace(ent.Verdict))
		switch v {
		case "pass", "degraded", "fail":
			return Verdict(v), strings.TrimSpace(ent.Reason), nil
		}
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, `"verdict":"pass"`) || strings.Contains(lower, "verdict: pass"):
		return VerdictPass, strings.TrimSpace(raw), nil
	case strings.Contains(lower, `"verdict":"degraded"`) || strings.Contains(lower, "verdict: degraded"):
		return VerdictDegraded, strings.TrimSpace(raw), nil
	case strings.Contains(lower, `"verdict":"fail"`) || strings.Contains(lower, "verdict: fail"):
		return VerdictFail, strings.TrimSpace(raw), nil
	}
	return "", "", fmt.Errorf("no parseable verdict")
}

// isNotFoundAnswer reports whether a responder's reply is essentially the
// "not found" sentinel — case-insensitive, allowing trailing punctuation or a
// short qualifier like "Not found." or "not found in the document".
func isNotFoundAnswer(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.Trim(t, ".!?\"' ")
	if t == "not found" {
		return true
	}
	return strings.HasPrefix(t, "not found ") || strings.HasPrefix(t, "not found,") || strings.HasPrefix(t, "not found:")
}

func filterTuples(tuples []Tuple, filter string) []Tuple {
	parts := strings.Split(filter, ",")
	wanted := make(map[string]bool)
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			wanted[p] = true
		}
	}
	if len(wanted) == 0 {
		return tuples
	}
	var out []Tuple
	for _, t := range tuples {
		if wanted[t.ID] || wanted[t.Repo] {
			out = append(out, t)
		}
	}
	return out
}

func backendModel(b Backend) string {
	if m, ok := b.(interface{ ModelName() string }); ok {
		return m.ModelName()
	}
	switch v := b.(type) {
	case *AnthropicBackend:
		return v.Model
	case *OpenAIBackend:
		return v.Model
	case *OllamaBackend:
		return v.Model
	case *DeepSeekBackend:
		return v.Model
	}
	return ""
}

// WriteCorpusJSON writes a JSON report.
func WriteCorpusJSON(w io.Writer, report CorpusReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteCorpusMarkdown writes a human-readable markdown summary.
func WriteCorpusMarkdown(w io.Writer, report CorpusReport) error {
	fmt.Fprintf(w, "# Curated eval — %s\n\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Corpus: %s\nTier: %s\nResponder: %s/%s\nJudge model: %s\nTuples: %d  •  Pass rate: %.1f%%  •  API calls: %d  •  Duration: %dms\n\n",
		report.CorpusPath, report.Tier, report.Backend, report.ResponderModel, report.JudgeModel,
		report.Totals.Total, report.Totals.PassRate()*100, report.APICallCount, report.DurationMS)
	if l := report.LLM; l.Active() {
		fmt.Fprintf(w, "Tier-3 rewriter: %d considered · %d rewritten · %d skipped · %d failed · %d tokens saved · cache %dH/%dM\n\n",
			l.SectionsConsidered, l.SectionsRewritten, l.SectionsSkipped, l.SectionsFailed, l.TokensSaved, l.CacheHits, l.CacheMisses)
	}
	fmt.Fprintln(w, "## Aggregate")
	suspectCount := 0
	for _, t := range report.Tuples {
		if t.Suspect {
			suspectCount++
		}
	}
	fmt.Fprintf(w, "- pass: %d\n- degraded: %d\n- fail: %d\n- suspect: %d (responder said \"not found\" on the original — not measuring compression)\n\n",
		report.Totals.Pass, report.Totals.Degraded, report.Totals.Fail, suspectCount)
	if suspectCount > 0 {
		fmt.Fprintln(w, "### Suspect tuples")
		for _, t := range report.Tuples {
			if t.Suspect {
				fmt.Fprintf(w, "- `%s` — %s\n", t.Tuple.ID, t.SuspectReason)
			}
		}
		fmt.Fprintln(w)
	}

	repos := make([]string, 0, len(report.ByRepo))
	for k := range report.ByRepo {
		repos = append(repos, k)
	}
	sort.Strings(repos)
	fmt.Fprintln(w, "## By repo")
	fmt.Fprintln(w, "| repo | pass | degraded | fail | total | pass-rate |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|")
	for _, r := range repos {
		t := report.ByRepo[r]
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %.1f%% |\n", r, t.Pass, t.Degraded, t.Fail, t.Total, t.PassRate()*100)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Per-tuple")
	fmt.Fprintln(w, "| id | verdict | tokens before→after | reason |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, t := range report.Tuples {
		fmt.Fprintf(w, "| %s | %s | %d→%d | %s |\n", t.Tuple.ID, t.Verdict, t.TokensBefore, t.TokensAfter, sanitizePipe(t.Reason))
	}
	return nil
}

func sanitizePipe(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", "/")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// PerRuleReport aggregates a baseline corpus run plus one corpus run per
// candidate rule disabled in turn. Used to identify which rule's removal
// rescues failing tuples (or breaks passing ones).
type PerRuleReport struct {
	GeneratedAt    time.Time        `json:"generated_at"`
	CorpusPath     string           `json:"corpus_path"`
	Backend        string           `json:"backend"`
	ResponderModel string           `json:"responder_model"`
	JudgeModel     string           `json:"judge_model"`
	Tier           string           `json:"tier"`
	Baseline       CorpusReport     `json:"baseline"`
	PerRule        []RuleSweepEntry `json:"per_rule"`
	DurationMS     int64            `json:"duration_ms"`
	APICallCount   int              `json:"api_call_count"`
}

// RuleSweepEntry holds one rule's sweep outcome relative to baseline.
type RuleSweepEntry struct {
	Rule         string       `json:"rule"`
	Tally        VerdictTally `json:"tally"`
	PassDelta    int          `json:"pass_delta"`
	FailDelta    int          `json:"fail_delta"`
	Rescued      []string     `json:"rescued_tuple_ids"`
	Broke        []string     `json:"broke_tuple_ids"`
	DurationMS   int64        `json:"duration_ms"`
	APICallCount int          `json:"api_call_count"`
}

// CandidateRules returns the rule names eligible for the per-rule sweep at the
// given compress tier: rules at-or-below the tier, excluding default-disabled
// (since they wouldn't be active in baseline either).
func CandidateRules(tier compress.Tier) []string {
	var names []string
	for _, r := range rules.RulesForTier(rules.Tier(tier)) {
		if rules.DefaultDisabled(r.Name()) {
			continue
		}
		names = append(names, r.Name())
	}
	return names
}

// RunCorpusPerRule runs the corpus once with the baseline rule set, then
// once per candidate rule with that rule appended to DisabledRules. Caches
// answer-original across all sweeps (key independent of rule config) and
// reuses answer-compressed + verdict whenever disabling the rule did not
// change the compressed bytes.
func RunCorpusPerRule(opts CorpusOptions) (PerRuleReport, error) {
	start := time.Now()
	baseline, err := RunCorpus(opts)
	if err != nil {
		return PerRuleReport{}, fmt.Errorf("baseline: %w", err)
	}

	tier := opts.Compress.Tier
	if tier == 0 {
		tier = compress.TierAggressive
	}
	candidates := CandidateRules(tier)

	baselineByID := make(map[string]Verdict, len(baseline.Tuples))
	for _, t := range baseline.Tuples {
		baselineByID[t.Tuple.ID] = t.Verdict
	}

	report := PerRuleReport{
		GeneratedAt:    time.Now().UTC(),
		CorpusPath:     opts.CorpusPath,
		Backend:        baseline.Backend,
		ResponderModel: baseline.ResponderModel,
		JudgeModel:     baseline.JudgeModel,
		Tier:           baseline.Tier,
		Baseline:       baseline,
		APICallCount:   baseline.APICallCount,
	}

	for _, name := range candidates {
		ruleStart := time.Now()
		sweepOpts := opts
		sweepOpts.Compress = opts.Compress
		sweepOpts.Compress.DisabledRules = append([]string{}, opts.Compress.DisabledRules...)
		sweepOpts.Compress.DisabledRules = append(sweepOpts.Compress.DisabledRules, name)

		sweep, err := RunCorpus(sweepOpts)
		if err != nil {
			return report, fmt.Errorf("sweep rule=%s: %w", name, err)
		}

		entry := RuleSweepEntry{
			Rule:         name,
			Tally:        sweep.Totals,
			PassDelta:    sweep.Totals.Pass - baseline.Totals.Pass,
			FailDelta:    sweep.Totals.Fail - baseline.Totals.Fail,
			DurationMS:   time.Since(ruleStart).Milliseconds(),
			APICallCount: sweep.APICallCount,
		}
		for _, t := range sweep.Tuples {
			before := baselineByID[t.Tuple.ID]
			after := t.Verdict
			if before != VerdictPass && after == VerdictPass {
				entry.Rescued = append(entry.Rescued, t.Tuple.ID)
			}
			if before == VerdictPass && after != VerdictPass {
				entry.Broke = append(entry.Broke, t.Tuple.ID)
			}
		}
		sort.Strings(entry.Rescued)
		sort.Strings(entry.Broke)
		report.PerRule = append(report.PerRule, entry)
		report.APICallCount += sweep.APICallCount
	}

	sort.Slice(report.PerRule, func(i, j int) bool {
		if report.PerRule[i].PassDelta != report.PerRule[j].PassDelta {
			return report.PerRule[i].PassDelta > report.PerRule[j].PassDelta
		}
		return report.PerRule[i].Rule < report.PerRule[j].Rule
	})
	report.DurationMS = time.Since(start).Milliseconds()
	return report, nil
}

// WritePerRuleJSON writes a JSON sweep report.
func WritePerRuleJSON(w io.Writer, report PerRuleReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WritePerRuleMarkdown writes a human-readable scoreboard.
func WritePerRuleMarkdown(w io.Writer, report PerRuleReport) error {
	fmt.Fprintf(w, "# Per-rule eval scoreboard — %s\n\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Corpus: %s\nTier: %s\nResponder: %s/%s\nJudge model: %s\nDuration: %dms  •  API calls: %d\n\n",
		report.CorpusPath, report.Tier, report.Backend, report.ResponderModel, report.JudgeModel,
		report.DurationMS, report.APICallCount)
	b := report.Baseline.Totals
	fmt.Fprintf(w, "## Baseline (all default rules)\npass=%d  degraded=%d  fail=%d  total=%d  pass-rate=%.1f%%\n\n",
		b.Pass, b.Degraded, b.Fail, b.Total, b.PassRate()*100)

	fmt.Fprintln(w, "## Per-rule sweep")
	fmt.Fprintln(w, "Each row reports the corpus result with that rule disabled (everything else as default). PassΔ > 0 means disabling the rule rescued tuples; PassΔ < 0 means disabling the rule broke tuples.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| rule | pass | degraded | fail | passΔ | failΔ | rescued | broke | ms |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---|---|---:|")
	for _, e := range report.PerRule {
		fmt.Fprintf(w, "| %s | %d | %d | %d | %+d | %+d | %s | %s | %d |\n",
			e.Rule, e.Tally.Pass, e.Tally.Degraded, e.Tally.Fail, e.PassDelta, e.FailDelta,
			joinIDs(e.Rescued), joinIDs(e.Broke), e.DurationMS)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Suspects")
	any := false
	for _, e := range report.PerRule {
		if len(e.Rescued) > 0 {
			any = true
			fmt.Fprintf(w, "- **%s** rescues: %s\n", e.Rule, strings.Join(e.Rescued, ", "))
		}
	}
	if !any {
		fmt.Fprintln(w, "_No single-rule rescues. Failures may be from rule interaction or non-rule-related (e.g., judge variance)._")
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Collateral damage")
	any = false
	for _, e := range report.PerRule {
		if len(e.Broke) > 0 {
			any = true
			fmt.Fprintf(w, "- **%s** disabling breaks: %s\n", e.Rule, strings.Join(e.Broke, ", "))
		}
	}
	if !any {
		fmt.Fprintln(w, "_No rule's removal broke a previously-passing tuple._")
	}
	return nil
}

func joinIDs(ids []string) string {
	if len(ids) == 0 {
		return "—"
	}
	return strings.Join(ids, ", ")
}

// hashKey returns a hex sha256 of the concatenated parts, separated by NUL.
func hashKey(prefix string, parts ...[]byte) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write(p)
	}
	return prefix + "-" + hex.EncodeToString(h.Sum(nil))
}

// corpusCache is a tiny disk-backed key/value for answer + verdict caching.
type corpusCache struct {
	dir     string
	disable bool
}

func newCorpusCache(dir string, disable bool) *corpusCache {
	return &corpusCache{dir: dir, disable: disable}
}

func (c *corpusCache) path(key string) string { return filepath.Join(c.dir, key+".json") }

func (c *corpusCache) GetString(key string) (string, bool) {
	if c.disable {
		return "", false
	}
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return "", false
	}
	var v struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return "", false
	}
	return v.Value, true
}

func (c *corpusCache) PutString(key, value string) error {
	if c.disable {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Value string `json:"value"`
	}{Value: value})
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(key), data, 0o644)
}

func (c *corpusCache) GetJSON(key string, dest any) bool {
	if c.disable {
		return false
	}
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return false
	}
	return json.Unmarshal(data, dest) == nil
}

func (c *corpusCache) PutJSON(key string, value any) error {
	if c.disable {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(key), data, 0o644)
}
