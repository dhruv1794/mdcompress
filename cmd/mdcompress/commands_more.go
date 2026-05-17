package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	mdcache "github.com/dhruv1794/mdcompress/pkg/cache"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/manifest"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
	"github.com/spf13/cobra"
)

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
			fmt.Fprintf(cmd.ErrOrStderr(), "tokens (%s): %d -> %d (%d saved)\n", tokens.DefaultTokenizer().Name(), result.TokensBefore, result.TokensAfter, result.TokensSaved())
			fmt.Fprintf(cmd.ErrOrStderr(), "bytes: %d -> %d (%d saved)\n", result.BytesBefore, result.BytesAfter, result.BytesSaved())
			if l := result.LLM; l.Active() {
				fmt.Fprintf(cmd.ErrOrStderr(), "tier-3 rewriter: %d considered, %d rewritten, %d skipped, %d failed, %d tokens saved (cache %dH/%dM)\n",
					l.SectionsConsidered, l.SectionsRewritten, l.SectionsSkipped, l.SectionsFailed, l.TokensSaved, l.CacheHits, l.CacheMisses)
			}
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
