package main

import (
	"fmt"

	"github.com/dhruv1794/mdcompress/pkg/migrate"
	"github.com/spf13/cobra"
)

func migrateCommand() *cobra.Command {
	var suggest bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Detect doc-quality tools and suggest mdcompress equivalents",
		Long: `Detect existing markdown linting and quality tools (markdownlint, Vale,
Prettier, remark-lint) and suggest complementary mdcompress configuration.

Run without flags to see a report. Use --suggest to generate a suggested
config.yaml block that coexists with detected tools.`,
		Example: `  mdcompress migrate
  mdcompress migrate --suggest`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := migrate.Analyze(".")
			if err != nil {
				return err
			}
			found := 0
			for _, tool := range report.Tools {
				if !tool.Found {
					continue
				}
				found++
				fmt.Fprintf(cmd.OutOrStdout(), "Detected %s (%s):\n", tool.Name, tool.ConfigFile)
				for _, s := range tool.Suggestions {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", s)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if found == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recognized doc-quality tools detected.")
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if suggest {
				fmt.Fprintln(cmd.OutOrStdout(), "--- Suggested .mdcompress/config.yaml ---")
				fmt.Fprint(cmd.OutOrStdout(), migrate.GenerateConfig(report, ""))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&suggest, "suggest", false, "generate a suggested mdcompress config")
	return cmd
}

const defaultConfigYAML = `version: 1
tier: aggressive
rules:
  enabled: []
  disabled:
    - dedup-cross-section
    - collapse-example-output
code_blocks:
  max_lines: 20
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
