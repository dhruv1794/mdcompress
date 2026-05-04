package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	mdeval "github.com/dhruv1794/mdcompress/pkg/eval"
	mdllm "github.com/dhruv1794/mdcompress/pkg/llm"
	"github.com/dhruv1794/mdcompress/pkg/rules"
	"github.com/spf13/cobra"
)

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
		Short: "Audit compressed markdown faithfulness",
		Long: `Audit whether compressed markdown preserves factual content.

The command generates factual questions from each markdown file, answers them
against the original and compressed versions, then asks the configured judge
backend to score answer equivalence. It writes a report and exits non-zero when
the average score is below the configured threshold; it does not change
compressed output or disable rules automatically. Ollama is the default local
backend.

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
	cmd.Flags().StringVar(&backendName, "backend", "", "LLM backend: ollama, anthropic, openai, deepseek, bedrock")
	cmd.Flags().StringVar(&model, "model", "", "LLM model name")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "LLM backend endpoint")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "environment variable containing the backend API key")
	cmd.Flags().IntVar(&questions, "questions", 0, "factual questions to generate per markdown file")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "minimum average faithfulness score before the audit exits non-zero")
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
	case mdeval.DeepSeekBackendName:
		return mdeval.NewDeepSeekBackend(endpoint, model, apiKeyEnv), nil
	case mdeval.BedrockBackendName:
		if strings.TrimSpace(model) == "" || strings.TrimSpace(model) == mdeval.DefaultModel {
			return nil, fmt.Errorf("bedrock eval backend requires --model or eval.model")
		}
		return mdeval.NewBedrockBackend(endpoint, model, apiKeyEnv), nil
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
	TierName          string
	EnabledRules      []string
	DisabledRules     []string
	CodeBlockMaxLines int
	Eval              evalConfig
	LLM               llmConfig
}

func defaultProjectConfig() projectConfig {
	return projectConfig{
		TierName:          compress.TierSafe.String(),
		DisabledRules:     defaultDisabledRules(),
		CodeBlockMaxLines: rules.DefaultCodeBlockMaxLines,
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
		case !indented && trimmed == "code_blocks:":
			section = "code_blocks"
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

		if section == "code_blocks" {
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				continue
			}
			value = trimConfigValue(value)
			switch strings.TrimSpace(key) {
			case "max_lines":
				if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
					cfg.CodeBlockMaxLines = parsed
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
		Tier:              parsedTier,
		EnabledRules:      enabled,
		DisabledRules:     disabled,
		CodeBlockMaxLines: cfg.CodeBlockMaxLines,
		LLMRewriter:       rewriter,
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
		Tier:              tier,
		EnabledRules:      cfg.EnabledRules,
		DisabledRules:     cfg.DisabledRules,
		CodeBlockMaxLines: cfg.CodeBlockMaxLines,
		LLMRewriter:       rewriter,
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
	if strings.TrimSpace(cfg.JudgeBackend) == "" {
		return nil, fmt.Errorf("tier-3 requires llm.judge_backend (and llm.judge_model) to avoid the rewrite backend judging its own output")
	}
	judge, err := mdllm.NewBackend(mdllm.Config{
		Backend:   cfg.JudgeBackend,
		Model:     cfg.JudgeModel,
		Endpoint:  cfg.Endpoint,
		APIKeyEnv: cfg.APIKeyEnv,
	})
	if err != nil {
		return nil, err
	}
	if mdllm.SameBackend(judge, backend) {
		return nil, fmt.Errorf("llm.judge_backend %s:%s must differ from the rewrite backend (evaluator-bias)", judge.Name(), judge.Model())
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
