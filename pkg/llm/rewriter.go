package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/parser"
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
	"github.com/yuin/goldmark/ast"
)

// Stats reports per-run rewriter activity.
type Stats struct {
	SectionsConsidered int
	SectionsRewritten  int
	SectionsSkipped    int
	SectionsFailed     int
	TokensSaved        int
	CacheHits          int
	CacheMisses        int
}

// Rewriter walks the markdown AST, sends qualifying prose sections to an LLM
// backend, and replaces them with a faithfulness-gated rewrite. Code blocks,
// tables, blockquotes, headings, and links are never touched (#65).
type Rewriter struct {
	Backend          Backend
	Judge            Backend
	MinSectionTokens int
	Threshold        float64
	Cache            *Cache
}

// NewRewriter constructs a Rewriter with sane defaults.
func NewRewriter(backend Backend) *Rewriter {
	return &Rewriter{
		Backend:          backend,
		MinSectionTokens: DefaultMinSectionTokens,
		Threshold:        DefaultThreshold,
	}
}

// Rewrite returns source with qualifying prose sections rewritten in place.
// Sections that fail the faithfulness gate fall back to the original (#66).
func (r *Rewriter) Rewrite(source []byte) ([]byte, Stats, error) {
	if r == nil || r.Backend == nil {
		return source, Stats{}, nil
	}

	doc, err := parser.Parse(source)
	if err != nil {
		return source, Stats{}, err
	}

	min := r.MinSectionTokens
	if min <= 0 {
		min = DefaultMinSectionTokens
	}
	threshold := r.Threshold
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	var stats Stats
	var edits []render.Edit

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		para, ok := child.(*ast.Paragraph)
		if !ok {
			continue
		}
		paraRange, ok := paragraphRange(para, source)
		if !ok {
			continue
		}
		original := string(source[paraRange.Start:paraRange.End])
		if !isProseSection(original) {
			continue
		}
		if tokens.Count([]byte(original)) < min {
			continue
		}
		stats.SectionsConsidered++

		rewrite, score, hit, err := r.rewriteSection(original)
		if hit {
			stats.CacheHits++
		} else {
			stats.CacheMisses++
		}
		if err != nil {
			stats.SectionsFailed++
			continue
		}
		if rewrite == "" || rewrite == strings.TrimRight(original, "\n") {
			stats.SectionsSkipped++
			continue
		}
		if score < threshold {
			stats.SectionsSkipped++
			continue
		}

		replacement := preserveTrailingNewlines(original, rewrite)
		if len(replacement) >= len(original) {
			stats.SectionsSkipped++
			continue
		}
		edits = append(edits, render.Edit{
			Start:       paraRange.Start,
			End:         paraRange.End,
			Replacement: []byte(replacement),
		})
		stats.SectionsRewritten++
		stats.TokensSaved += tokens.Count([]byte(original)) - tokens.Count([]byte(replacement))
	}

	if len(edits) == 0 {
		return source, stats, nil
	}
	return render.ApplyEdits(source, edits), stats, nil
}

// rewriteSection looks up a cached rewrite for original or, on miss, calls
// the backend, runs the faithfulness judge, and persists the response.
func (r *Rewriter) rewriteSection(original string) (string, float64, bool, error) {
	sectionSHA := SectionHash(original)
	promptSHA := PromptHash()
	modelID := r.Backend.Name() + ":" + r.Backend.Model()
	key := CacheKey(sectionSHA, promptSHA, modelID)

	if r.Cache != nil {
		if entry, ok := r.Cache.Get(key); ok {
			return entry.Rewrite, entry.Score, true, nil
		}
	}

	raw, err := r.Backend.Complete(RewritePrompt(original))
	if err != nil {
		return "", 0, false, err
	}
	rewrite := stripCodeFence(raw)
	if rewrite == "" {
		return "", 0, false, fmt.Errorf("backend returned empty rewrite")
	}

	score, err := r.judgeRewrite(original, rewrite)
	if err != nil {
		return "", 0, false, err
	}

	if r.Cache != nil {
		_ = r.Cache.Put(key, CacheEntry{
			Model:    modelID,
			Prompt:   promptSHA,
			Section:  sectionSHA,
			Original: original,
			Rewrite:  rewrite,
			Score:    score,
		})
	}
	return rewrite, score, false, nil
}

// judgeRewrite runs the per-section faithfulness gate (#66). Falls back to
// the rewrite backend when no dedicated judge is configured.
func (r *Rewriter) judgeRewrite(original, rewrite string) (float64, error) {
	judge := r.Judge
	if judge == nil {
		judge = r.Backend
	}
	raw, err := judge.Complete(JudgePrompt(original, rewrite))
	if err != nil {
		return 0, err
	}
	score, err := parseJudgeScore(raw)
	if err != nil {
		return 0, err
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score, nil
}

// paragraphRange returns the source byte range covering all lines of the
// paragraph node, including the trailing newline of the last line when
// present in source.
func paragraphRange(para *ast.Paragraph, source []byte) (render.Range, bool) {
	lines := para.Lines()
	if lines.Len() == 0 {
		return render.Range{}, false
	}
	start := lines.At(0).Start
	end := lines.At(lines.Len() - 1).Stop
	if start < 0 || end <= start || end > len(source) {
		return render.Range{}, false
	}
	return render.Range{Start: start, End: end}, true
}

// isProseSection enforces the v6 hard rules (#65) by rejecting sections that
// look like tables, blockquotes, lists, headings, or fenced code. Goldmark's
// default parser leaves pipe tables as Paragraphs, so the check is textual.
func isProseSection(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, raw := range strings.Split(trimmed, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "#"):
			return false
		case strings.HasPrefix(line, ">"):
			return false
		case strings.HasPrefix(line, "```"), strings.HasPrefix(line, "~~~"):
			return false
		case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "), strings.HasPrefix(line, "+ "):
			return false
		case strings.HasPrefix(line, "|"):
			return false
		case strings.Count(line, "|") >= 2:
			return false
		case looksLikeNumberedListItem(line):
			return false
		}
	}
	return true
}

var numberedListPattern = regexp.MustCompile(`^[0-9]+\.[ \t]`)

func looksLikeNumberedListItem(line string) bool {
	return numberedListPattern.MatchString(line)
}

// preserveTrailingNewlines re-attaches the original trailing newline pattern
// to the rewrite so block-level whitespace structure stays intact.
func preserveTrailingNewlines(original, rewrite string) string {
	tail := ""
	for i := len(original) - 1; i >= 0 && original[i] == '\n'; i-- {
		tail = "\n" + tail
	}
	cleaned := strings.TrimRight(rewrite, "\n")
	return cleaned + tail
}

// stripCodeFence drops a leading/trailing markdown fence in case the model
// wrapped its output despite the prompt asking it not to.
func stripCodeFence(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
			raw = raw[newline+1:]
		} else {
			raw = strings.TrimPrefix(raw, "```")
		}
		raw = strings.TrimSuffix(raw, "```")
	}
	return strings.TrimSpace(raw)
}

type judgeResponse struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

func parseJudgeScore(raw string) (float64, error) {
	body := extractJSON(raw)
	var resp judgeResponse
	if err := json.Unmarshal([]byte(body), &resp); err == nil {
		return resp.Score, nil
	}
	re := regexp.MustCompile(`(?i)score[^0-9]*(0(?:\.\d+)?|1(?:\.0+)?)`)
	if match := re.FindStringSubmatch(raw); len(match) == 2 {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil {
			return value, nil
		}
	}
	return 0, fmt.Errorf("backend returned no parseable judgement: %q", raw)
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		return strings.TrimSpace(raw)
	}
	start := strings.IndexAny(raw, "{[")
	end := strings.LastIndexAny(raw, "}]")
	if start >= 0 && end >= start {
		return raw[start : end+1]
	}
	return raw
}
