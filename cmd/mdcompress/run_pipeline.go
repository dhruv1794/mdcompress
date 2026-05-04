package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/manifest"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

type runOptions struct {
	Args         []string
	All          bool
	Staged       bool
	Changed      bool
	NoStaleCheck bool
	Compress     compress.Options
}

type runSummary struct {
	Compressed  int
	Skipped     int
	TokensSaved int
}

type markdownInput struct {
	Path    string
	Rel     string
	Content []byte
}

func runMarkdown(opts runOptions) (runSummary, error) {
	inputs, err := markdownInputs(opts)
	if err != nil {
		return runSummary{}, err
	}
	m, err := manifest.Read(manifest.DefaultPath)
	if err != nil {
		return runSummary{}, err
	}

	crossFile := &rules.CrossFileState{}

	var summary runSummary
	for _, input := range inputs {
		sha := mdcache.SourceSHA(input.Content)
		entry, ok := m.Entries[input.Rel]
		if !opts.All && ok && freshManifestEntry(input.Path, entry, sha, !opts.Staged) {
			summary.Skipped++
			continue
		}

		fileOpts := opts.Compress
		fileOpts.FilePath = input.Rel
		fileOpts.CrossFile = crossFile
		result, err := compress.Compress(input.Content, fileOpts)
		if err != nil {
			return summary, err
		}
		cachePath, err := mdcache.Write(mdcache.DefaultDir, input.Rel, result.Output)
		if err != nil {
			return summary, err
		}
		m.Entries[input.Rel] = manifest.Entry{
			Source:       input.Rel,
			Cache:        cachePath,
			SHA256:       sha,
			TokensBefore: result.TokensBefore,
			TokensAfter:  result.TokensAfter,
			CompressedAt: time.Now().UTC(),
			RulesFired:   result.RulesFired,
		}
		summary.Compressed++
	}

	if summary.Compressed > 0 {
		if err := manifest.Write(manifest.DefaultPath, m); err != nil {
			return summary, err
		}
	}
	m.RecalculateTotals()
	summary.TokensSaved = m.Totals.TokensSaved
	return summary, nil
}

func markdownInputs(opts runOptions) ([]markdownInput, error) {
	if opts.Staged && opts.Changed {
		return nil, fmt.Errorf("--staged and --changed cannot be used together")
	}
	if (opts.Staged || opts.Changed) && len(opts.Args) > 0 {
		return nil, fmt.Errorf("--staged and --changed do not accept path arguments")
	}
	if opts.NoStaleCheck && !opts.All && !opts.Staged && !opts.Changed && len(opts.Args) == 0 {
		return nil, nil
	}
	if opts.Staged {
		paths, err := gitMarkdownPaths("diff", "--cached", "--name-only", "--diff-filter=ACM")
		if err != nil {
			return nil, err
		}
		inputs := make([]markdownInput, 0, len(paths))
		for _, path := range paths {
			content, err := gitOutput("show", ":"+path)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, markdownInput{Path: path, Rel: path, Content: content})
		}
		return inputs, nil
	}
	if opts.Changed {
		paths, err := gitMarkdownPaths("diff", "--name-only", "HEAD@{1}", "HEAD")
		if err != nil {
			return nil, err
		}
		return readMarkdownInputs(paths)
	}
	paths, err := markdownPaths(opts.Args)
	if err != nil {
		return nil, err
	}
	return readMarkdownInputs(paths)
}

func readMarkdownInputs(paths []string) ([]markdownInput, error) {
	inputs := make([]markdownInput, 0, len(paths))
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, markdownInput{
			Path:    path,
			Rel:     filepath.ToSlash(rel),
			Content: source,
		})
	}
	return inputs, nil
}

func gitMarkdownPaths(args ...string) ([]string, error) {
	output, err := gitOutput(args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		path := strings.TrimSpace(line)
		if path == "" || !isMarkdownPath(path) || excludedPath(path) || seen[path] {
			continue
		}
		paths = append(paths, path)
		seen[path] = true
	}
	sort.Strings(paths)
	return paths, nil
}

func gitOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func markdownPaths(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}

	seen := make(map[string]bool)
	var paths []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isMarkdownPath(arg) && !excludedPath(arg) {
				clean := filepath.Clean(arg)
				if !seen[clean] {
					paths = append(paths, clean)
					seen[clean] = true
				}
			}
			continue
		}

		if err := filepath.WalkDir(arg, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if excludedPath(path) && path != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if isMarkdownPath(path) && !excludedPath(path) {
				clean := filepath.Clean(path)
				if !seen[clean] {
					paths = append(paths, clean)
					seen[clean] = true
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func isMarkdownPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

func freshManifestEntry(sourcePath string, entry manifest.Entry, sourceSHA string, checkMTime bool) bool {
	if entry.SHA256 != sourceSHA || !mdcache.Exists(entry.Cache) {
		return false
	}
	if !checkMTime {
		return true
	}
	sourceInfo, sourceErr := os.Stat(sourcePath)
	cacheInfo, cacheErr := os.Stat(entry.Cache)
	if sourceErr != nil || cacheErr != nil {
		return false
	}
	return !sourceInfo.ModTime().After(cacheInfo.ModTime())
}

func excludedPath(path string) bool {
	slash := filepath.ToSlash(filepath.Clean(path))
	if slash == "." {
		return false
	}
	excludedPrefixes := []string{
		".git/",
		".mdcompress/cache/",
		"node_modules/",
		"vendor/",
	}
	for _, prefix := range excludedPrefixes {
		if slash == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(slash, prefix) {
			return true
		}
	}
	return false
}

func staleEntries(m *manifest.Manifest) []string {
	var stale []string
	for source, entry := range m.Entries {
		content, err := os.ReadFile(source)
		if err != nil {
			stale = append(stale, source)
			continue
		}
		if !freshManifestEntry(source, entry, mdcache.SourceSHA(content), true) {
			stale = append(stale, source)
		}
	}
	return stale
}

type freshnessSummary struct {
	Fresh   int
	Stale   int
	Missing int
}

const claudeSonnetInputUSDPerMTok = 3.0

func cacheFreshness(m *manifest.Manifest) freshnessSummary {
	paths, err := markdownPaths([]string{"."})
	if err != nil {
		return freshnessSummary{Stale: len(m.Entries)}
	}

	var summary freshnessSummary
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(".", path)
		if err != nil {
			summary.Stale++
			continue
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true

		entry, ok := m.Entries[rel]
		if !ok || !mdcache.Exists(entry.Cache) {
			summary.Missing++
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			summary.Stale++
			continue
		}
		if !freshManifestEntry(path, entry, mdcache.SourceSHA(content), true) {
			summary.Stale++
			continue
		}
		summary.Fresh++
	}

	for source := range m.Entries {
		if !seen[source] {
			summary.Stale++
		}
	}
	return summary
}

func topSavings(m *manifest.Manifest, limit int) []manifest.Entry {
	entries := make([]manifest.Entry, 0, len(m.Entries))
	for _, entry := range m.Entries {
		if entry.TokensBefore <= entry.TokensAfter {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i].TokensBefore - entries[i].TokensAfter
		right := entries[j].TokensBefore - entries[j].TokensAfter
		if left == right {
			return entries[i].Source < entries[j].Source
		}
		return left > right
	})
	if limit > 0 && len(entries) > limit {
		return entries[:limit]
	}
	return entries
}

func percentSaved(before, saved int) float64 {
	if before <= 0 {
		return 0
	}
	return float64(saved) / float64(before) * 100
}

func sonnetInputSavingsUSD(tokensSaved int) float64 {
	return float64(tokensSaved) / 1_000_000 * claudeSonnetInputUSDPerMTok
}

func formatInt(value int) string {
	if value == 0 {
		return "0"
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	digits := fmt.Sprintf("%d", value)
	firstGroup := len(digits) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	var b strings.Builder
	b.WriteString(sign)
	b.WriteString(digits[:firstGroup])
	for i := firstGroup; i < len(digits); i += 3 {
		b.WriteByte(',')
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

func currentTier() string {
	data, err := os.ReadFile(".mdcompress/config.yaml")
	if err != nil {
		return compress.TierSafe.String()
	}
	tier := configTier(string(data))
	if _, err := compress.ParseTier(tier); err != nil {
		return "invalid (" + tier + ")"
	}
	return tier
}

func repoLabel() string {
	if output, err := gitOutput("remote", "get-url", "origin"); err == nil {
		if remote := strings.TrimSpace(string(output)); remote != "" {
			return remote
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(dir)
}
