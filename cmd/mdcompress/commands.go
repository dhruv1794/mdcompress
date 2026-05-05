package main

import (
	"fmt"

	"github.com/dhruv1794/mdcompress/pkg/manifest"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
	"github.com/spf13/cobra"
)

func runCommand() *cobra.Command {
	var all bool
	var staged bool
	var changed bool
	var noStaleCheck bool
	var quiet bool
	var verbose bool
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
				Verbose:      verbose,
				Compress:     compressOpts,
			})
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "compressed %d file(s), skipped %d unchanged\n", summary.Compressed, summary.Skipped)
				fmt.Fprintf(cmd.OutOrStdout(), "tokens saved (%s): %d\n", tokens.DefaultTokenizer().Name(), summary.TokensSaved)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "force rebuild of all selected markdown files")
	cmd.Flags().BoolVar(&staged, "staged", false, "compress staged markdown files from the git index")
	cmd.Flags().BoolVar(&changed, "changed", false, "compress markdown files changed by the last merge")
	cmd.Flags().BoolVar(&noStaleCheck, "no-stale-check", false, "when run without paths, skip automatic stale cache refresh")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print warnings for rules that errored during compression")
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
			fmt.Fprintf(cmd.OutOrStdout(), "Tokenizer: %s\n", tokens.DefaultTokenizer().Name())
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
