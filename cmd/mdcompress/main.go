package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	assets "github.com/dhruv1794/mdcompress/internal"
	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/manifest"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "mdcompress",
		Short:         "Compress markdown for token-efficient agent context",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(versionCommand())
	root.AddCommand(compressCommand())
	root.AddCommand(runCommand())
	root.AddCommand(statusCommand())
	root.AddCommand(cleanCommand())
	root.AddCommand(initCommand())
	root.AddCommand(installHooksCommand())
	root.AddCommand(installSkillCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand() *cobra.Command {
	var all bool
	var staged bool
	var changed bool
	var quiet bool
	var disabledRules []string
	var tier string

	cmd := &cobra.Command{
		Use:   "run [path...]",
		Short: "Compress markdown files into the hidden cache mirror",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedTier, err := compress.ParseTier(tier)
			if err != nil {
				return err
			}
			summary, err := runMarkdown(runOptions{
				Args:    args,
				All:     all,
				Staged:  staged,
				Changed: changed,
				Compress: compress.Options{
					Tier:          parsedTier,
					DisabledRules: disabledRules,
				},
			})
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "compressed %d file(s), skipped %d unchanged\n", summary.Compressed, summary.Skipped)
				fmt.Fprintf(cmd.OutOrStdout(), "tokens saved: %d\n", summary.TokensSaved)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "force rebuild of all selected markdown files")
	cmd.Flags().BoolVar(&staged, "staged", false, "compress staged markdown files from the git index")
	cmd.Flags().BoolVar(&changed, "changed", false, "compress markdown files changed by the last merge")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	cmd.Flags().StringVar(&tier, "tier", compress.TierSafe.String(), "compression tier: safe, aggressive, llm")
	cmd.Flags().StringSliceVar(&disabledRules, "disable-rule", nil, "rule to disable; may be repeated")
	return cmd
}

func statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cached markdown token savings",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manifest.Read(manifest.DefaultPath)
			if err != nil {
				return err
			}

			stale := staleEntries(m)
			fmt.Fprintf(cmd.OutOrStdout(), "files tracked: %d\n", m.Totals.Files)
			fmt.Fprintf(cmd.OutOrStdout(), "tokens before: %d\n", m.Totals.TokensBefore)
			fmt.Fprintf(cmd.OutOrStdout(), "tokens after: %d\n", m.Totals.TokensAfter)
			fmt.Fprintf(cmd.OutOrStdout(), "tokens saved: %d\n", m.Totals.TokensSaved)
			fmt.Fprintf(cmd.OutOrStdout(), "stale entries: %d\n", len(stale))
			if len(stale) > 0 {
				sort.Strings(stale)
				for _, source := range stale {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", source)
				}
			}
			return nil
		},
	}
}

func cleanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Delete the cache and reset the manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.RemoveAll(mdcache.DefaultDir); err != nil {
				return err
			}
			if err := manifest.Write(manifest.DefaultPath, manifest.New()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "cache removed and manifest reset")
			return nil
		},
	}
}

func initCommand() *cobra.Command {
	var agentsFlag string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize mdcompress in this repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			agents, err := parseAgents(agentsFlag)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
				return err
			}
			if err := writeFileIfMissing(".mdcompress/config.yaml", []byte(defaultConfigYAML)); err != nil {
				return err
			}
			if err := appendLinesOnce(".gitignore", []string{".mdcompress/cache/", ".mdcompress/manifest.json"}); err != nil {
				return err
			}
			if err := installHooks(); err != nil {
				return err
			}
			if agents[agentCodex] {
				if err := appendAgentHint("AGENTS.md", true); err != nil {
					return err
				}
			}
			if agents[agentClaude] {
				if err := appendAgentHint("CLAUDE.md", false); err != nil {
					return err
				}
				if err := installSkill(); err != nil {
					return err
				}
			}
			if agents[agentCursor] {
				if err := appendCursorHints(); err != nil {
					return err
				}
			}
			if agents[agentWindsurf] {
				if err := appendWindsurfHints(); err != nil {
					return err
				}
			}
			if agents[agentContinue] {
				if err := appendContinueHint(); err != nil {
					return err
				}
			}
			if agents[agentAider] {
				if err := appendAiderHint(); err != nil {
					return err
				}
			}
			summary, err := runMarkdown(runOptions{
				Args: []string{"."},
				All:  true,
				Compress: compress.Options{
					Tier: compress.TierSafe,
				},
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized mdcompress in this repo. Cache: %d files, saved %d tokens.\n", summary.Compressed+summary.Skipped, summary.TokensSaved)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "comma-separated agents to integrate with (claude,codex,cursor,windsurf,continue,aider). Default: all.")
	return cmd
}

func installHooksCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install-hooks",
		Short: "Install mdcompress git hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := installHooks(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "mdcompress git hooks installed")
			return nil
		},
	}
}

func installSkillCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install-skill",
		Short: "Install the Claude Code mdcompress skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := installSkill(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "mdcompress Claude Code skill installed")
			return nil
		},
	}
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mdcompress version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version)
		},
	}
}

func compressCommand() *cobra.Command {
	var disabledRules []string
	var tier string

	cmd := &cobra.Command{
		Use:   "compress [file]",
		Short: "Compress markdown from a file or stdin",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := readInput(args)
			if err != nil {
				return err
			}

			parsedTier, err := compress.ParseTier(tier)
			if err != nil {
				return err
			}

			result, err := compress.Compress(content, compress.Options{
				Tier:          parsedTier,
				DisabledRules: disabledRules,
			})
			if err != nil {
				return err
			}

			if _, err := cmd.OutOrStdout().Write(result.Output); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "tokens: %d -> %d (%d saved)\n", result.TokensBefore, result.TokensAfter, result.TokensSaved())
			fmt.Fprintf(cmd.ErrOrStderr(), "bytes: %d -> %d (%d saved)\n", result.BytesBefore, result.BytesAfter, result.BytesSaved())
			return nil
		},
	}

	cmd.Flags().StringVar(&tier, "tier", compress.TierSafe.String(), "compression tier: safe, aggressive, llm")
	cmd.Flags().StringSliceVar(&disabledRules, "disable-rule", nil, "rule to disable; may be repeated")
	return cmd
}

func readInput(args []string) ([]byte, error) {
	if len(args) == 0 || args[0] == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(args[0])
}

type runOptions struct {
	Args     []string
	All      bool
	Staged   bool
	Changed  bool
	Compress compress.Options
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

	var summary runSummary
	for _, input := range inputs {
		sha := mdcache.SourceSHA(input.Content)
		entry, ok := m.Entries[input.Rel]
		if !opts.All && ok && entry.SHA256 == sha && mdcache.Exists(entry.Cache) {
			summary.Skipped++
			continue
		}

		result, err := compress.Compress(input.Content, opts.Compress)
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

	if err := manifest.Write(manifest.DefaultPath, m); err != nil {
		return summary, err
	}
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
		if mdcache.SourceSHA(content) != entry.SHA256 || !mdcache.Exists(entry.Cache) {
			stale = append(stale, source)
		}
	}
	return stale
}

func installHooks() error {
	if info, err := os.Stat(".git"); err != nil || !info.IsDir() {
		return fmt.Errorf("not a git repository: .git directory not found")
	}
	if err := os.MkdirAll(filepath.Join(".git", "hooks"), 0o755); err != nil {
		return err
	}
	if err := appendMarkedBlock(filepath.Join(".git", "hooks", "pre-commit"), assets.PreCommitHook); err != nil {
		return err
	}
	if err := appendMarkedBlock(filepath.Join(".git", "hooks", "post-merge"), assets.PostMergeHook); err != nil {
		return err
	}
	return nil
}

func installSkill() error {
	path := filepath.Join(".claude", "skills", "mdcompress", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(assets.Skill), 0o644)
}

func appendMarkedBlock(path, block string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(current), "# mdcompress") {
		return os.Chmod(path, 0o755)
	}

	var next []byte
	if len(current) > 0 {
		next = append(next, current...)
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(strings.TrimRight(block, "\n"))...)
	next = append(next, '\n')
	if err := os.WriteFile(path, next, 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

func writeFileIfMissing(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func appendLinesOnce(path string, lines []string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := make(map[string]bool)
	for _, line := range strings.Split(string(current), "\n") {
		existing[strings.TrimSpace(line)] = true
	}

	var additions []string
	for _, line := range lines {
		if !existing[line] {
			additions = append(additions, line)
		}
	}
	if len(additions) == 0 {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 && !strings.HasSuffix(string(next), "\n") {
		next = append(next, '\n')
	}
	for _, line := range additions {
		next = append(next, []byte(line)...)
		next = append(next, '\n')
	}
	return os.WriteFile(path, next, 0o644)
}

func appendAgentHint(path string, create bool) error {
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if !create {
			return nil
		}
		current = nil
	} else if err != nil {
		return err
	}
	if strings.Contains(string(current), "## mdcompress") {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 {
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(agentHint)...)
	return os.WriteFile(path, next, 0o644)
}

func appendCursorHints() error {
	var paths []string
	if fileExists(".cursorrules") {
		paths = append(paths, ".cursorrules")
	}

	matches, err := filepath.Glob(filepath.Join(".cursor", "rules", "*.mdc"))
	if err != nil {
		return err
	}
	sort.Strings(matches)
	paths = append(paths, matches...)

	for _, path := range paths {
		if err := appendExistingHint(path, "## mdcompress", cursorAgentHint); err != nil {
			return err
		}
	}
	return nil
}

func appendWindsurfHints() error {
	if !fileExists(".windsurfrules") {
		return nil
	}
	return appendExistingHint(".windsurfrules", "## mdcompress", windsurfAgentHint)
}

func appendAiderHint() error {
	if !fileExists(".aider.conf.yml") {
		return nil
	}
	return appendExistingHint(".aider.conf.yml", "# mdcompress", aiderAgentHint)
}

// appendContinueHint merges the mdcompress hint into a Continue config.json.
// JSON-merge: parse, update only the systemMessage field, preserve other keys.
// Idempotent via a marker substring inside systemMessage.
func appendContinueHint() error {
	path := filepath.Join(".continue", "config.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	config := make(map[string]json.RawMessage)
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}

	var systemMessage string
	if existing, ok := config["systemMessage"]; ok && len(existing) > 0 {
		// systemMessage may legitimately be a string or null; ignore decode
		// errors for non-string types (e.g. number/object), treating them as
		// "unset" so we don't clobber unrelated user data.
		_ = json.Unmarshal(existing, &systemMessage)
	}

	if strings.Contains(systemMessage, continueHintMarker) {
		return nil
	}

	if systemMessage == "" {
		systemMessage = continueAgentHint
	} else {
		trimmed := strings.TrimRight(systemMessage, "\n")
		systemMessage = trimmed + "\n\n" + continueAgentHint
	}

	encoded, err := json.Marshal(systemMessage)
	if err != nil {
		return err
	}
	config["systemMessage"] = encoded

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func appendExistingHint(path, marker, hint string) error {
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.Contains(string(current), marker) {
		return nil
	}

	next := append([]byte(nil), current...)
	if len(next) > 0 {
		if !strings.HasSuffix(string(next), "\n") {
			next = append(next, '\n')
		}
		next = append(next, '\n')
	}
	next = append(next, []byte(hint)...)
	return os.WriteFile(path, next, 0o644)
}

type agentName string

const (
	agentClaude   agentName = "claude"
	agentCodex    agentName = "codex"
	agentCursor   agentName = "cursor"
	agentWindsurf agentName = "windsurf"
	agentContinue agentName = "continue"
	agentAider    agentName = "aider"
)

var allAgents = []agentName{
	agentClaude, agentCodex, agentCursor, agentWindsurf, agentContinue, agentAider,
}

func parseAgents(input string) (map[agentName]bool, error) {
	valid := make(map[agentName]bool, len(allAgents))
	for _, a := range allAgents {
		valid[a] = true
	}

	if strings.TrimSpace(input) == "" {
		return valid, nil
	}

	selection := make(map[agentName]bool)
	for _, raw := range strings.Split(input, ",") {
		name := agentName(strings.ToLower(strings.TrimSpace(raw)))
		if name == "" {
			continue
		}
		if !valid[name] {
			return nil, fmt.Errorf("unknown agent %q (valid: claude, codex, cursor, windsurf, continue, aider)", name)
		}
		selection[name] = true
	}
	return selection, nil
}

const agentHint = `## mdcompress
This repo uses mdcompress. For any *.md file you read, prefer the version
at .mdcompress/cache/<same-relative-path> if it exists because it is a
token-optimized mirror of the original maintained automatically.
`

const cursorAgentHint = `## mdcompress
When reading any *.md file in this repo, prefer
.mdcompress/cache/<same-relative-path> when it exists. It is a
token-optimized mirror of the original maintained automatically.
`

const windsurfAgentHint = `## mdcompress
When reading any *.md file in this repo, prefer
.mdcompress/cache/<same-relative-path> when it exists. It is a
token-optimized mirror of the original maintained automatically.
`

const aiderAgentHint = `# mdcompress
# When reading any *.md file in this repo, prefer
# .mdcompress/cache/<same-relative-path> when it exists. It is a
# token-optimized mirror of the original maintained automatically.
`

const continueHintMarker = "mdcompress: prefer .mdcompress/cache"

const continueAgentHint = "mdcompress: prefer .mdcompress/cache/<same-relative-path> over the original *.md when it exists. It is a token-optimized mirror of the original maintained automatically."

const defaultConfigYAML = `version: 1
tier: safe
rules:
  enabled: []
  disabled: []
patterns:
  include:
    - "**/*.md"
  exclude:
    - ".mdcompress/cache/**"
    - "node_modules/**"
    - "vendor/**"
    - ".git/**"
output:
  mode: hidden-mirror
  cache_dir: .mdcompress/cache
tokens:
  encoding: cl100k_base
hooks:
  pre_commit: true
  post_merge: true
`
