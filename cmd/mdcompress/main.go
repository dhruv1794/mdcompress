package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	assets "github.com/dhruv1794/mdcompress/internal"
	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	mdeval "github.com/dhruv1794/mdcompress/pkg/eval"
	mdllm "github.com/dhruv1794/mdcompress/pkg/llm"
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
	root.AddCommand(evalCommand())
	root.AddCommand(doctorCommand())
	root.AddCommand(cleanCommand())
	root.AddCommand(initCommand())
	root.AddCommand(installHooksCommand())
	root.AddCommand(installSkillCommand())
	root.AddCommand(serveCommand())
	root.AddCommand(initMCPCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand() *cobra.Command {
	var all bool
	var staged bool
	var changed bool
	var noStaleCheck bool
	var quiet bool
	var enabledRules []string
	var disabledRules []string
	var tier string

	cmd := &cobra.Command{
		Use:   "run [path...]",
		Short: "Compress markdown files into the hidden cache mirror",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			compressOpts, err := compressionOptionsFromFlags(cmd, tier, enabledRules, disabledRules)
			if err != nil {
				return err
			}
			summary, err := runMarkdown(runOptions{
				Args:         args,
				All:          all,
				Staged:       staged,
				Changed:      changed,
				NoStaleCheck: noStaleCheck,
				Compress:     compressOpts,
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
	cmd.Flags().BoolVar(&noStaleCheck, "no-stale-check", false, "when run without paths, skip automatic stale cache refresh")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	cmd.Flags().StringVar(&tier, "tier", "", "compression tier: safe, aggressive, llm (default: config tier or safe)")
	cmd.Flags().StringSliceVar(&enabledRules, "enable-rule", nil, "opt-in rule to enable; may be repeated")
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

			tier := currentTier()
			freshness := cacheFreshness(m)
			savedPercent := percentSaved(m.Totals.TokensBefore, m.Totals.TokensSaved)
			savedCost := sonnetInputSavingsUSD(m.Totals.TokensSaved)

			fmt.Fprintf(cmd.OutOrStdout(), "Repo: %s\n", repoLabel())
			fmt.Fprintf(cmd.OutOrStdout(), "Tier: %s\n", tier)
			fmt.Fprintf(cmd.OutOrStdout(), "Files tracked: %s\n", formatInt(m.Totals.Files))
			fmt.Fprintf(cmd.OutOrStdout(), "Cache: %s fresh, %s stale, %s missing\n", formatInt(freshness.Fresh), formatInt(freshness.Stale), formatInt(freshness.Missing))
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Tokens before: %s\n", formatInt(m.Totals.TokensBefore))
			fmt.Fprintf(cmd.OutOrStdout(), "Tokens after:  %s\n", formatInt(m.Totals.TokensAfter))
			fmt.Fprintf(cmd.OutOrStdout(), "Saved:         %s (%.1f%%)\n", formatInt(m.Totals.TokensSaved), savedPercent)
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "ROI estimate (Claude Sonnet input pricing, $3.00/MTok):")
			fmt.Fprintf(cmd.OutOrStdout(), "  Per full-cache read: ~$%.4f\n", savedCost)
			fmt.Fprintf(cmd.OutOrStdout(), "  Local manifest total: ~$%.4f\n", savedCost)

			top := topSavings(m, 10)
			if len(top) > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "Top savings:")
				for _, entry := range top {
					saved := entry.TokensBefore - entry.TokensAfter
					fmt.Fprintf(cmd.OutOrStdout(), "  %-32s %s saved (%.1f%%)\n", entry.Source, formatInt(saved), percentSaved(entry.TokensBefore, saved))
				}
			}

			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Note: .mdcompress/manifest.json is gitignored, so local manifest totals are per clone, not shared team-wide.")
			return nil
		},
	}
}

func evalCommand() *cobra.Command {
	var repo string
	var rule string
	var tier string
	var backendName string
	var model string
	var endpoint string
	var apiKeyEnv string
	var questions int
	var threshold float64
	var seeds int
	var jsonOut string
	var markdownOut string

	cmd := &cobra.Command{
		Use:   "eval [--repo=<path>] [--rule=<name>]",
		Short: "Evaluate compressed markdown faithfulness",
		Long: `Evaluate whether compressed markdown preserves factual content.

The command generates factual questions from each markdown file, answers them
against the original and compressed versions, then asks the configured judge
backend to score answer equivalence. Ollama is the default local backend.

Use --rule to isolate one registered rule by disabling all others for the run.`,
		Example: `  mdcompress eval --repo=.
  mdcompress eval --repo=docs --rule=strip-toc
  mdcompress eval --backend=ollama --model=llama3.1:8b --questions=10 --seeds=3
  mdcompress eval --backend=openai --model=gpt-4o-mini --api-key-env=OPENAI_API_KEY`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			evalCfg := readEvalConfig(".mdcompress/config.yaml")
			flags := cmd.Flags()
			if !flags.Changed("backend") {
				backendName = evalCfg.Backend
			}
			if !flags.Changed("model") {
				model = evalCfg.Model
			}
			if !flags.Changed("endpoint") {
				endpoint = evalCfg.Endpoint
			}
			if !flags.Changed("api-key-env") {
				apiKeyEnv = evalCfg.APIKeyEnv
			}
			if !flags.Changed("questions") {
				questions = evalCfg.QuestionsPerDoc
			}
			if !flags.Changed("threshold") {
				threshold = evalCfg.Threshold
			}
			if !flags.Changed("seeds") {
				seeds = evalCfg.Seeds
			}
			projectCfg := readProjectConfig(".mdcompress/config.yaml")
			tierValue := tier
			if !flags.Changed("tier") {
				tierValue = projectCfg.TierName
			}
			parsedTier, err := compress.ParseTier(tierValue)
			if err != nil {
				return err
			}
			backend, err := evalBackend(backendName, endpoint, model, apiKeyEnv)
			if err != nil {
				return err
			}
			model = resolvedEvalModel(backendName, model)
			report, err := mdeval.Run(mdeval.Options{
				Repo:            repo,
				Rule:            rule,
				Tier:            parsedTier,
				QuestionsPerDoc: questions,
				Threshold:       threshold,
				Seeds:           seeds,
				Backend:         backend,
				Model:           model,
			})
			if err != nil {
				return err
			}
			if markdownOut != "" {
				if err := writeEvalMarkdown(markdownOut, report); err != nil {
					return err
				}
			}
			if jsonOut != "" {
				if err := writeEvalJSON(jsonOut, report); err != nil {
					return err
				}
			}
			if markdownOut == "" && jsonOut == "" {
				if err := mdeval.WriteMarkdown(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			}
			if !report.Passed {
				return fmt.Errorf("faithfulness score %.3f is below threshold %.3f", report.AverageScore, report.Threshold)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", ".", "repository or directory to evaluate")
	cmd.Flags().StringVar(&rule, "rule", "", "evaluate a single rule by disabling all other registered rules")
	cmd.Flags().StringVar(&tier, "tier", "", "compression tier: safe, aggressive, llm (default: config tier or safe)")
	cmd.Flags().StringVar(&backendName, "backend", "", "LLM backend: ollama, anthropic, openai")
	cmd.Flags().StringVar(&model, "model", "", "LLM model name")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "LLM backend endpoint")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "environment variable containing the backend API key")
	cmd.Flags().IntVar(&questions, "questions", 0, "factual questions to generate per markdown file")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "minimum passing average faithfulness score")
	cmd.Flags().IntVar(&seeds, "seeds", 0, "number of question-generation passes per file")
	cmd.Flags().StringVar(&jsonOut, "json-out", "", "write JSON report to this path")
	cmd.Flags().StringVar(&markdownOut, "markdown-out", "", "write markdown report to this path")
	return cmd
}

func evalBackend(name, endpoint, model, apiKeyEnv string) (mdeval.Backend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", mdeval.DefaultBackend:
		return mdeval.NewOllamaBackend(endpoint, model), nil
	case mdeval.AnthropicBackendName:
		if strings.TrimSpace(model) == "" || strings.TrimSpace(model) == mdeval.DefaultModel {
			return nil, fmt.Errorf("anthropic eval backend requires --model or eval.model")
		}
		return mdeval.NewAnthropicBackend(endpoint, model, apiKeyEnv), nil
	case mdeval.OpenAIBackendName:
		if strings.TrimSpace(model) == "" || strings.TrimSpace(model) == mdeval.DefaultModel {
			return nil, fmt.Errorf("openai eval backend requires --model or eval.model")
		}
		return mdeval.NewOpenAIBackend(endpoint, model, apiKeyEnv), nil
	default:
		return nil, fmt.Errorf("unsupported eval backend %q", name)
	}
}

func resolvedEvalModel(backendName, model string) string {
	if strings.TrimSpace(model) != "" {
		return strings.TrimSpace(model)
	}
	if strings.TrimSpace(backendName) == "" || strings.EqualFold(strings.TrimSpace(backendName), mdeval.DefaultBackend) {
		return mdeval.DefaultModel
	}
	return ""
}

type evalConfig struct {
	Backend         string
	Model           string
	Endpoint        string
	APIKeyEnv       string
	Threshold       float64
	QuestionsPerDoc int
	Seeds           int
}

type llmConfig struct {
	Backend          string
	Model            string
	Endpoint         string
	APIKeyEnv        string
	Cache            bool
	Threshold        float64
	MinSectionTokens int
	JudgeBackend     string
	JudgeModel       string
}

type projectConfig struct {
	TierName      string
	EnabledRules  []string
	DisabledRules []string
	Eval          evalConfig
	LLM           llmConfig
}

func defaultProjectConfig() projectConfig {
	return projectConfig{
		TierName:      compress.TierSafe.String(),
		DisabledRules: defaultDisabledRules(),
		Eval: evalConfig{
			Backend:         mdeval.DefaultBackend,
			Threshold:       mdeval.DefaultThreshold,
			QuestionsPerDoc: mdeval.DefaultQuestionsPerDoc,
			Seeds:           mdeval.DefaultSeeds,
		},
		LLM: llmConfig{
			Backend:          mdllm.DefaultBackend,
			Model:            mdllm.DefaultModel,
			Cache:            true,
			Threshold:        mdllm.DefaultThreshold,
			MinSectionTokens: mdllm.DefaultMinSectionTokens,
		},
	}
}

func readProjectConfig(path string) projectConfig {
	cfg := defaultProjectConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	section := ""
	ruleList := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if !indented {
			section = ""
			ruleList = ""
		}

		switch {
		case !indented && strings.HasPrefix(trimmed, "tier:"):
			cfg.TierName = trimConfigValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "tier:")))
			continue
		case !indented && trimmed == "rules:":
			section = "rules"
			continue
		case !indented && trimmed == "eval:":
			section = "eval"
			continue
		case !indented && trimmed == "llm:":
			section = "llm"
			continue
		}

		if section == "rules" {
			if strings.HasPrefix(trimmed, "enabled:") {
				ruleList = "enabled"
				cfg.EnabledRules = parseInlineConfigList(strings.TrimSpace(strings.TrimPrefix(trimmed, "enabled:")))
				continue
			}
			if strings.HasPrefix(trimmed, "disabled:") {
				ruleList = "disabled"
				values := parseInlineConfigList(strings.TrimSpace(strings.TrimPrefix(trimmed, "disabled:")))
				cfg.DisabledRules = values
				continue
			}
			if strings.HasPrefix(trimmed, "- ") {
				value := trimConfigValue(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				if value == "" {
					continue
				}
				switch ruleList {
				case "enabled":
					cfg.EnabledRules = append(cfg.EnabledRules, value)
				case "disabled":
					cfg.DisabledRules = append(cfg.DisabledRules, value)
				}
			}
			continue
		}

		if section == "eval" {
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				continue
			}
			value = trimConfigValue(value)
			switch strings.TrimSpace(key) {
			case "backend":
				cfg.Eval.Backend = value
			case "model":
				cfg.Eval.Model = value
			case "endpoint":
				cfg.Eval.Endpoint = value
			case "api_key_env":
				cfg.Eval.APIKeyEnv = value
			case "threshold":
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.Eval.Threshold = parsed
				}
			case "questions_per_doc":
				if parsed, err := strconv.Atoi(value); err == nil {
					cfg.Eval.QuestionsPerDoc = parsed
				}
			case "seeds":
				if parsed, err := strconv.Atoi(value); err == nil {
					cfg.Eval.Seeds = parsed
				}
			}
			continue
		}

		if section == "llm" {
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				continue
			}
			value = trimConfigValue(value)
			switch strings.TrimSpace(key) {
			case "backend":
				cfg.LLM.Backend = value
			case "model":
				cfg.LLM.Model = value
			case "endpoint":
				cfg.LLM.Endpoint = value
			case "api_key_env":
				cfg.LLM.APIKeyEnv = value
			case "cache":
				cfg.LLM.Cache = strings.EqualFold(value, "true")
			case "threshold":
				if parsed, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.LLM.Threshold = parsed
				}
			case "min_section_tokens":
				if parsed, err := strconv.Atoi(value); err == nil {
					cfg.LLM.MinSectionTokens = parsed
				}
			case "judge_backend":
				cfg.LLM.JudgeBackend = value
			case "judge_model":
				cfg.LLM.JudgeModel = value
			}
		}
	}
	return cfg
}

func readEvalConfig(path string) evalConfig {
	return readProjectConfig(path).Eval
}

func compressionOptionsFromFlags(cmd *cobra.Command, tier string, enabledRules, disabledRules []string) (compress.Options, error) {
	cfg := readProjectConfig(".mdcompress/config.yaml")
	tierValue := cfg.TierName
	if cmd.Flags().Changed("tier") {
		tierValue = tier
	}
	parsedTier, err := compress.ParseTier(tierValue)
	if err != nil {
		return compress.Options{}, err
	}
	enabled := cfg.EnabledRules
	if cmd.Flags().Changed("enable-rule") {
		enabled = enabledRules
	}
	disabled := cfg.DisabledRules
	if cmd.Flags().Changed("disable-rule") {
		disabled = disabledRules
	}
	rewriter, err := buildLLMRewriter(parsedTier, cfg.LLM)
	if err != nil {
		return compress.Options{}, err
	}
	return compress.Options{
		Tier:          parsedTier,
		EnabledRules:  enabled,
		DisabledRules: disabled,
		LLMRewriter:   rewriter,
	}, nil
}

func compressionOptionsFromConfig(path string) compress.Options {
	cfg := readProjectConfig(path)
	tier, err := compress.ParseTier(cfg.TierName)
	if err != nil {
		tier = compress.TierSafe
	}
	rewriter, err := buildLLMRewriter(tier, cfg.LLM)
	if err != nil {
		// fall back to Tier-2 silently — rewriter construction errors are
		// surfaced by user-facing commands; non-interactive call sites keep
		// the project usable when an env var is missing.
		rewriter = nil
	}
	return compress.Options{
		Tier:          tier,
		EnabledRules:  cfg.EnabledRules,
		DisabledRules: cfg.DisabledRules,
		LLMRewriter:   rewriter,
	}
}

// buildLLMRewriter returns a Tier-3 rewriter wired to the configured backend
// and on-disk cache, or nil when tier is below TierLLM.
func buildLLMRewriter(tier compress.Tier, cfg llmConfig) (compress.LLMRewriter, error) {
	if tier != compress.TierLLM {
		return nil, nil
	}
	backend, err := mdllm.NewBackend(mdllm.Config{
		Backend:   cfg.Backend,
		Model:     cfg.Model,
		Endpoint:  cfg.Endpoint,
		APIKeyEnv: cfg.APIKeyEnv,
	})
	if err != nil {
		return nil, err
	}
	var judge mdllm.Backend
	if strings.TrimSpace(cfg.JudgeBackend) != "" {
		judge, err = mdllm.NewBackend(mdllm.Config{
			Backend:   cfg.JudgeBackend,
			Model:     cfg.JudgeModel,
			Endpoint:  cfg.Endpoint,
			APIKeyEnv: cfg.APIKeyEnv,
		})
		if err != nil {
			return nil, err
		}
	}
	rewriter := mdllm.NewRewriter(backend)
	rewriter.Judge = judge
	if cfg.MinSectionTokens > 0 {
		rewriter.MinSectionTokens = cfg.MinSectionTokens
	}
	if cfg.Threshold > 0 {
		rewriter.Threshold = cfg.Threshold
	}
	if cfg.Cache {
		rewriter.Cache = mdllm.NewCache(mdllm.DefaultCacheDir)
	}
	return mdllm.NewCompressAdapter(rewriter), nil
}

func defaultDisabledRules() []string {
	return []string{"dedup-cross-section", "collapse-example-output"}
}

func parseInlineConfigList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if value == "[]" {
		return []string{}
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		if strings.TrimSpace(value) == "" {
			return []string{}
		}
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if item := trimConfigValue(part); item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	return nil
}

func trimConfigValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func writeEvalJSON(path string, report mdeval.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return mdeval.WriteJSON(file, report)
}

func writeEvalMarkdown(path string, report mdeval.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return mdeval.WriteMarkdown(file, report)
}

func doctorCommand() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose mdcompress repository setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var fixes []string
			if fix {
				var err error
				fixes, err = fixDoctor()
				if err != nil {
					return err
				}
			}

			checks := diagnoseRepo()
			for _, check := range checks {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", check.Status, check.Name, check.Detail)
				if check.Fix != "" && check.Status != doctorOK {
					fmt.Fprintf(cmd.OutOrStdout(), "  fix: %s\n", check.Fix)
				}
			}
			for _, applied := range fixes {
				fmt.Fprintf(cmd.OutOrStdout(), "[OK] fix applied: %s\n", applied)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "attempt automatic fixes for common setup problems")
	return cmd
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
	var mcpFlag bool

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
				Args:     []string{"."},
				All:      true,
				Compress: compressionOptionsFromConfig(".mdcompress/config.yaml"),
			})
			if err != nil {
				return err
			}
			var mcpFiles []string
			if mcpFlag {
				mcpFiles, err = installMCPConfigs(agents)
				if err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized mdcompress in this repo. Cache: %d files, saved %d tokens.\n", summary.Compressed+summary.Skipped, summary.TokensSaved)
			if len(mcpFiles) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "MCP wired into: %s\n", strings.Join(mcpFiles, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "comma-separated agents to integrate with (claude,codex,cursor,windsurf,continue,aider). Default: all.")
	cmd.Flags().BoolVar(&mcpFlag, "mcp", false, "also wire 'mdcompress serve' into agent MCP config files")
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
	var enabledRules []string
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

			compressOpts, err := compressionOptionsFromFlags(cmd, tier, enabledRules, disabledRules)
			if err != nil {
				return err
			}

			result, err := compress.Compress(content, compressOpts)
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

	cmd.Flags().StringVar(&tier, "tier", "", "compression tier: safe, aggressive, llm (default: config tier or safe)")
	cmd.Flags().StringSliceVar(&enabledRules, "enable-rule", nil, "opt-in rule to enable; may be repeated")
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

	var summary runSummary
	for _, input := range inputs {
		sha := mdcache.SourceSHA(input.Content)
		entry, ok := m.Entries[input.Rel]
		if !opts.All && ok && freshManifestEntry(input.Path, entry, sha, !opts.Staged) {
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

const (
	doctorOK   = "OK"
	doctorWarn = "WARN"
	doctorFail = "FAIL"
)

type doctorCheck struct {
	Status string
	Name   string
	Detail string
	Fix    string
}

func diagnoseRepo() []doctorCheck {
	checks := []doctorCheck{
		checkConfig(),
		checkHooks(),
		checkAgentHints(),
		checkCacheFreshness(),
		checkManifestConsistency(),
		checkPath(),
		checkGitignoredMarkdown(),
	}
	return checks
}

func fixDoctor() ([]string, error) {
	var fixes []string

	if !fileExists(".mdcompress/config.yaml") {
		if err := os.MkdirAll(".mdcompress", 0o755); err != nil {
			return fixes, err
		}
		if err := writeFileIfMissing(".mdcompress/config.yaml", []byte(defaultConfigYAML)); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "wrote .mdcompress/config.yaml")
	}

	hooks := checkHooks()
	if hooks.Status != doctorOK {
		if err := installHooks(); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "installed mdcompress hooks")
	}

	agents := checkAgentHints()
	if agents.Status != doctorOK {
		if err := appendAgentHint("AGENTS.md", true); err != nil {
			return fixes, err
		}
		if err := appendAgentHint("CLAUDE.md", false); err != nil {
			return fixes, err
		}
		if err := installSkill(); err != nil {
			return fixes, err
		}
		if err := appendCursorHints(); err != nil {
			return fixes, err
		}
		if err := appendWindsurfHints(); err != nil {
			return fixes, err
		}
		if err := appendContinueHint(); err != nil {
			return fixes, err
		}
		if err := appendAiderHint(); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "restored agent hints")
	}

	cacheFresh := checkCacheFreshness()
	manifestConsistent := checkManifestConsistency()
	if cacheFresh.Status != doctorOK || manifestConsistent.Status != doctorOK {
		if _, err := runMarkdown(runOptions{
			Args:     []string{"."},
			All:      true,
			Compress: compressionOptionsFromConfig(".mdcompress/config.yaml"),
		}); err != nil {
			return fixes, err
		}
		fixes = append(fixes, "rebuilt markdown cache and manifest")
	}

	return fixes, nil
}

func checkConfig() doctorCheck {
	const path = ".mdcompress/config.yaml"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: ".mdcompress/config.yaml is missing", Fix: "run `mdcompress init` or `mdcompress doctor --fix`"}
	}
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: err.Error(), Fix: "inspect file permissions and rerun doctor"}
	}

	text := string(data)
	if !strings.Contains(text, "version: 1") {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: "missing or unsupported version", Fix: "rewrite .mdcompress/config.yaml from `mdcompress init` defaults"}
	}
	tier := configTier(text)
	if _, err := compress.ParseTier(tier); err != nil {
		return doctorCheck{Status: doctorFail, Name: "config", Detail: err.Error(), Fix: "set tier to safe, aggressive, or llm"}
	}
	return doctorCheck{Status: doctorOK, Name: "config", Detail: "valid .mdcompress/config.yaml"}
}

func configTier(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "tier:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "tier:"))
			return strings.Trim(value, `"'`)
		}
	}
	return "safe"
}

func checkHooks() doctorCheck {
	switch {
	case dirExists(".husky"):
		return checkHookFile("hooks", ".husky/pre-commit", ".husky/post-merge")
	case fileExists(".pre-commit-config.yaml"):
		text, err := readText(".pre-commit-config.yaml")
		if err != nil {
			return doctorCheck{Status: doctorFail, Name: "hooks", Detail: err.Error(), Fix: "inspect .pre-commit-config.yaml permissions"}
		}
		if strings.Contains(text, "# mdcompress") && strings.Contains(text, "mdcompress-staged") && strings.Contains(text, "mdcompress-post-merge") {
			return doctorCheck{Status: doctorOK, Name: "hooks", Detail: "pre-commit config includes mdcompress hooks"}
		}
		return doctorCheck{Status: doctorFail, Name: "hooks", Detail: "pre-commit config missing mdcompress hook entries", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	case fileExists("lefthook.yml"):
		return checkLefthookFile("lefthook.yml")
	case fileExists("lefthook.yaml"):
		return checkLefthookFile("lefthook.yaml")
	default:
		return checkHookFile("hooks", filepath.Join(".git", "hooks", "pre-commit"), filepath.Join(".git", "hooks", "post-merge"))
	}
}

func checkHookFile(name, preCommitPath, postMergePath string) doctorCheck {
	preCommit, err := readText(preCommitPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: name, Detail: preCommitPath + " is missing", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	}
	postMerge, err := readText(postMergePath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: name, Detail: postMergePath + " is missing", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
	}
	if strings.Contains(preCommit, "# mdcompress") && strings.Contains(preCommit, "mdcompress run --staged --quiet") &&
		strings.Contains(postMerge, "# mdcompress") && strings.Contains(postMerge, "mdcompress run --changed --quiet") {
		return doctorCheck{Status: doctorOK, Name: name, Detail: "mdcompress pre-commit and post-merge hooks installed"}
	}
	return doctorCheck{Status: doctorFail, Name: name, Detail: "mdcompress hook block is missing or outdated", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
}

func checkLefthookFile(path string) doctorCheck {
	text, err := readText(path)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "hooks", Detail: err.Error(), Fix: "inspect lefthook config permissions"}
	}
	if strings.Contains(text, "# mdcompress pre-commit") && strings.Contains(text, "# mdcompress post-merge") &&
		strings.Contains(text, "mdcompress run --staged --quiet") && strings.Contains(text, "mdcompress run --changed --quiet") {
		return doctorCheck{Status: doctorOK, Name: "hooks", Detail: path + " includes mdcompress commands"}
	}
	return doctorCheck{Status: doctorFail, Name: "hooks", Detail: path + " missing mdcompress commands", Fix: "run `mdcompress install-hooks` or `mdcompress doctor --fix`"}
}

func checkAgentHints() doctorCheck {
	files := agentHintFiles()
	if len(files) == 0 {
		return doctorCheck{Status: doctorWarn, Name: "agent hints", Detail: "no mdcompress agent hints found", Fix: "run `mdcompress init --agents=...` or `mdcompress doctor --fix`"}
	}
	sort.Strings(files)
	return doctorCheck{Status: doctorOK, Name: "agent hints", Detail: "found in " + strings.Join(files, ", ")}
}

func agentHintFiles() []string {
	var files []string
	for _, path := range []string{"AGENTS.md", "CLAUDE.md", ".cursorrules", ".windsurfrules", ".aider.conf.yml"} {
		text, err := readText(path)
		if err == nil && (strings.Contains(text, "## mdcompress") || strings.Contains(text, "# mdcompress")) {
			files = append(files, path)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(".cursor", "rules", "*.mdc"))
	for _, path := range matches {
		text, err := readText(path)
		if err == nil && strings.Contains(text, "## mdcompress") {
			files = append(files, filepath.ToSlash(path))
		}
	}
	continueConfig := filepath.Join(".continue", "config.json")
	text, err := readText(continueConfig)
	if err == nil && strings.Contains(text, continueHintMarker) {
		files = append(files, filepath.ToSlash(continueConfig))
	}
	return files
}

func checkCacheFreshness() doctorCheck {
	sources, err := markdownPaths([]string{"."})
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: err.Error(), Fix: "inspect markdown source paths"}
	}
	m, err := manifest.Read(manifest.DefaultPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: "manifest cannot be read: " + err.Error(), Fix: "run `mdcompress run --all` or `mdcompress doctor --fix`"}
	}
	var missing []string
	var stale []string
	for _, path := range sources {
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return doctorCheck{Status: doctorFail, Name: "cache freshness", Detail: err.Error(), Fix: "inspect markdown source paths"}
		}
		rel = filepath.ToSlash(rel)
		entry, ok := m.Entries[rel]
		if !ok || !mdcache.Exists(entry.Cache) {
			missing = append(missing, rel)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			stale = append(stale, rel)
			continue
		}
		if !freshManifestEntry(path, entry, mdcache.SourceSHA(content), true) {
			stale = append(stale, rel)
		}
	}
	if len(missing) == 0 && len(stale) == 0 {
		return doctorCheck{Status: doctorOK, Name: "cache freshness", Detail: fmt.Sprintf("%d markdown file(s) have fresh cache mirrors", len(sources))}
	}
	return doctorCheck{
		Status: doctorFail,
		Name:   "cache freshness",
		Detail: fmt.Sprintf("%d missing, %d stale cache mirror(s)", len(missing), len(stale)),
		Fix:    "run `mdcompress run --all` or `mdcompress doctor --fix`",
	}
}

func checkManifestConsistency() doctorCheck {
	m, err := manifest.Read(manifest.DefaultPath)
	if err != nil {
		return doctorCheck{Status: doctorFail, Name: "manifest", Detail: err.Error(), Fix: "run `mdcompress run --all` or `mdcompress doctor --fix`"}
	}
	var problems []string
	for source, entry := range m.Entries {
		if entry.Source != source {
			problems = append(problems, source+" source mismatch")
		}
		if entry.Cache == "" || !mdcache.Exists(entry.Cache) {
			problems = append(problems, source+" cache missing")
		}
		if _, err := os.Stat(source); err != nil {
			problems = append(problems, source+" source missing")
		}
	}
	if len(problems) == 0 {
		return doctorCheck{Status: doctorOK, Name: "manifest", Detail: fmt.Sprintf("%d manifest entries consistent with cache", len(m.Entries))}
	}
	return doctorCheck{
		Status: doctorFail,
		Name:   "manifest",
		Detail: fmt.Sprintf("%d inconsistent manifest entrie(s)", len(problems)),
		Fix:    "run `mdcompress run --all` or `mdcompress doctor --fix`",
	}
}

func checkPath() doctorCheck {
	if _, err := exec.LookPath("mdcompress"); err == nil {
		return doctorCheck{Status: doctorOK, Name: "PATH", Detail: "mdcompress is available to hooks"}
	}
	return doctorCheck{Status: doctorWarn, Name: "PATH", Detail: "mdcompress was not found on PATH", Fix: "install mdcompress somewhere on PATH before relying on hooks"}
}

func checkGitignoredMarkdown() doctorCheck {
	paths, err := markdownPaths([]string{"."})
	if err != nil {
		return doctorCheck{Status: doctorWarn, Name: "gitignored markdown", Detail: err.Error(), Fix: "inspect markdown source paths"}
	}
	var ignored []string
	for _, path := range paths {
		if gitCheckIgnore(path) {
			ignored = append(ignored, filepath.ToSlash(path))
		}
	}
	if len(ignored) == 0 {
		return doctorCheck{Status: doctorOK, Name: "gitignored markdown", Detail: "no source markdown files are gitignored"}
	}
	sort.Strings(ignored)
	return doctorCheck{
		Status: doctorWarn,
		Name:   "gitignored markdown",
		Detail: fmt.Sprintf("%d source markdown file(s) are gitignored", len(ignored)),
		Fix:    "remove intentional source docs from .gitignore or run mdcompress on them manually when needed",
	}
}

func gitCheckIgnore(path string) bool {
	cmd := exec.Command("git", "check-ignore", "--quiet", path)
	err := cmd.Run()
	return err == nil
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func installHooks() error {
	switch {
	case dirExists(".husky"):
		return installHuskyHooks()
	case fileExists(".pre-commit-config.yaml"):
		return appendMarkedText(".pre-commit-config.yaml", "# mdcompress", preCommitFrameworkBlock)
	case fileExists("lefthook.yml"):
		return installLefthookHooks("lefthook.yml")
	case fileExists("lefthook.yaml"):
		return installLefthookHooks("lefthook.yaml")
	default:
		return installDirectGitHooks()
	}
}

func installDirectGitHooks() error {
	if info, err := os.Stat(".git"); err != nil || !info.IsDir() {
		return fmt.Errorf("not a git repository: .git directory not found")
	}
	if err := os.MkdirAll(filepath.Join(".git", "hooks"), 0o755); err != nil {
		return err
	}
	if err := appendMarkedBlock(filepath.Join(".git", "hooks", "pre-commit"), assets.PreCommitHook); err != nil {
		return err
	}
	return appendMarkedBlock(filepath.Join(".git", "hooks", "post-merge"), assets.PostMergeHook)
}

func installHuskyHooks() error {
	if err := appendMarkedBlock(filepath.Join(".husky", "pre-commit"), huskyPreCommitBlock); err != nil {
		return err
	}
	return appendMarkedBlock(filepath.Join(".husky", "post-merge"), huskyPostMergeBlock)
}

func installLefthookHooks(path string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(current)
	text = upsertLefthookCommand(text, "pre-commit", "# mdcompress pre-commit", "mdcompress run --staged --quiet")
	text = upsertLefthookCommand(text, "post-merge", "# mdcompress post-merge", "mdcompress run --changed --quiet")
	if text == string(current) {
		return nil
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func upsertLefthookCommand(text, section, marker, run string) string {
	if strings.Contains(text, marker) {
		return text
	}

	lines := strings.Split(text, "\n")
	sectionStart := -1
	sectionEnd := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == section+":" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			sectionStart = i
			break
		}
	}

	block := []string{
		section + ":",
		"  commands:",
		"    " + marker,
		"    mdcompress:",
		"      run: " + run,
	}
	if sectionStart == -1 {
		return appendYAMLBlock(text, block)
	}

	for i := sectionStart + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			sectionEnd = i
			break
		}
	}

	commandsIndex := -1
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if strings.TrimSpace(lines[i]) == "commands:" && strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "    ") {
			commandsIndex = i
			break
		}
	}

	var insert []string
	if commandsIndex == -1 {
		insert = []string{
			"  commands:",
			"    " + marker,
			"    mdcompress:",
			"      run: " + run,
		}
	} else {
		insert = []string{
			"    " + marker,
			"    mdcompress:",
			"      run: " + run,
		}
	}

	next := make([]string, 0, len(lines)+len(insert))
	next = append(next, lines[:sectionEnd]...)
	next = append(next, insert...)
	next = append(next, lines[sectionEnd:]...)
	return strings.Join(next, "\n")
}

func appendYAMLBlock(text string, block []string) string {
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	return text + strings.Join(block, "\n") + "\n"
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

func appendMarkedText(path, marker, block string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(current), marker) {
		return nil
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
	return os.WriteFile(path, next, 0o644)
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

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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

const huskyPreCommitBlock = `# mdcompress
if command -v mdcompress >/dev/null 2>&1; then
  mdcompress run --staged --quiet
else
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
fi
`

const huskyPostMergeBlock = `# mdcompress
if command -v mdcompress >/dev/null 2>&1; then
  mdcompress run --changed --quiet
else
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
fi
`

const preCommitFrameworkBlock = `# mdcompress
- repo: local
  hooks:
    - id: mdcompress-staged
      name: mdcompress staged markdown cache
      entry: mdcompress run --staged --quiet
      language: system
      pass_filenames: false
      stages: [pre-commit]
    - id: mdcompress-post-merge
      name: mdcompress changed markdown cache
      entry: mdcompress run --changed --quiet
      language: system
      pass_filenames: false
      stages: [post-merge]
`

const defaultConfigYAML = `version: 1
tier: aggressive
rules:
  enabled: []
  disabled:
    - dedup-cross-section
    - collapse-example-output
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
eval:
  backend: ollama
  model: llama3.1:8b
  threshold: 0.95
  questions_per_doc: 10
  seeds: 1
llm:
  backend: ollama
  model: llama3.1:8b
  endpoint: http://localhost:11434
  api_key_env: ANTHROPIC_API_KEY
  cache: true
  threshold: 0.95
  min_section_tokens: 200
`
