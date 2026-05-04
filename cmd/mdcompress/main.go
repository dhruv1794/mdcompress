package main

import (
	"fmt"
	"os"

	"github.com/dhruv1794/mdcompress/pkg/tokens"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	var tokenizerFlag string
	root := &cobra.Command{
		Use:           "mdcompress",
		Short:         "Compress markdown for token-efficient agent context",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			t, err := tokens.ParseTokenizer(tokenizerFlag)
			if err != nil {
				return err
			}
			return tokens.SetDefault(t)
		},
	}
	root.PersistentFlags().StringVar(&tokenizerFlag, "tokenizer", "cl100k", "tokenizer for token counts: cl100k (GPT-3.5/4), o200k (GPT-4o), bytes")

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
	root.AddCommand(migrateCommand())
	root.AddCommand(webCommand())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
