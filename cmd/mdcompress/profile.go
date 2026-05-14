package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/rules"
	"github.com/dhruv1794/mdcompress/pkg/tokens"
	"github.com/spf13/cobra"
)

type ruleProfile struct {
	Name        string `json:"name"`
	Fires       int    `json:"fires"`
	BytesSaved  int    `json:"bytes_saved"`
	TokensSaved int    `json:"tokens_saved"`
	DurationMS  int64  `json:"duration_ms"`
	Errors      int    `json:"errors,omitempty"`
}

type profileReport struct {
	Repo            string        `json:"repo"`
	Files           int           `json:"files"`
	BytesBefore     int           `json:"bytes_before"`
	BytesAfter      int           `json:"bytes_after"`
	TokensBefore    int           `json:"tokens_before"`
	TokensAfter     int           `json:"tokens_after"`
	Tier            string        `json:"tier"`
	Tokenizer       string        `json:"tokenizer"`
	Rules           []ruleProfile `json:"rules"`
	NegativeOrZero  []string      `json:"flagged_negative_or_zero,omitempty"`
}

func profileCommand() *cobra.Command {
	var tier string
	var enabled []string
	var disabled []string
	var jsonOut string
	var repoLabelFlag string

	cmd := &cobra.Command{
		Use:   "profile [path...]",
		Short: "Per-rule fires/bytes_saved/tokens_saved/ms across markdown files",
		Long: `Walks markdown files, compresses each with per-rule token measurement
enabled, and prints a per-rule scoreboard sorted by tokens_saved desc.

Implicitly sets MDCOMPRESS_PROFILE_TOKENS=1 for the run; expect the profile
run to be ~2-4x slower than a normal compress because each rule's edit is
re-tokenized.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			compressOpts, err := compressionOptionsFromFlags(cmd, tier, enabled, disabled)
			if err != nil {
				return err
			}
			compressOpts.ProfileTokens = true

			report, err := runProfile(args, compressOpts, repoLabelFlag)
			if err != nil {
				return err
			}
			if jsonOut != "" {
				if err := writeProfileJSON(jsonOut, report); err != nil {
					return err
				}
			}
			return printProfile(cmd.OutOrStdout(), report)
		},
	}

	cmd.Flags().StringVar(&tier, "tier", "", "compression tier: safe, aggressive, llm (default: config tier or safe)")
	cmd.Flags().StringSliceVar(&enabled, "enable-rule", nil, "opt-in rule to enable; may be repeated")
	cmd.Flags().StringSliceVar(&disabled, "disable-rule", nil, "rule to disable; may be repeated")
	cmd.Flags().StringVar(&jsonOut, "json-out", "", "write JSON report to this path (in addition to stdout)")
	cmd.Flags().StringVar(&repoLabelFlag, "label", "", "repo label to embed in the JSON report (default: cwd basename)")
	return cmd
}

func runProfile(args []string, compressOpts compress.Options, label string) (profileReport, error) {
	paths, err := markdownPaths(args)
	if err != nil {
		return profileReport{}, err
	}

	crossFile := &rules.CrossFileState{}

	report := profileReport{
		Repo:      label,
		Tier:      compressOpts.Tier.String(),
		Tokenizer: defaultTokenizerName(),
	}
	if report.Repo == "" {
		report.Repo = repoLabel()
	}
	if compressOpts.Tier == 0 {
		report.Tier = compress.TierSafe.String()
	}

	totalFires := make(map[string]int)
	totalBytes := make(map[string]int)
	totalTokens := make(map[string]int)
	totalMS := make(map[string]int64)
	totalErr := make(map[string]int)

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return report, err
		}
		opts := compressOpts
		opts.FilePath = path
		opts.CrossFile = crossFile
		result, err := compress.Compress(content, opts)
		if err != nil {
			return report, fmt.Errorf("%s: %w", path, err)
		}
		report.Files++
		report.BytesBefore += result.BytesBefore
		report.BytesAfter += result.BytesAfter
		report.TokensBefore += result.TokensBefore
		report.TokensAfter += result.TokensAfter

		for name, fires := range result.RulesFired {
			totalFires[name] += fires
		}
		for name, bytes := range result.RuleBytesSaved {
			totalBytes[name] += bytes
		}
		for name, tok := range result.RuleTokensSaved {
			totalTokens[name] += tok
		}
		for name, ms := range result.RuleDurationsMS {
			totalMS[name] += ms
		}
		for name := range result.RuleErrors {
			totalErr[name]++
		}
	}

	names := make(map[string]bool)
	for n := range totalFires {
		names[n] = true
	}
	for n := range totalBytes {
		names[n] = true
	}
	for n := range totalTokens {
		names[n] = true
	}
	for n := range totalMS {
		names[n] = true
	}
	for n := range totalErr {
		names[n] = true
	}

	for name := range names {
		report.Rules = append(report.Rules, ruleProfile{
			Name:        name,
			Fires:       totalFires[name],
			BytesSaved:  totalBytes[name],
			TokensSaved: totalTokens[name],
			DurationMS:  totalMS[name],
			Errors:      totalErr[name],
		})
	}
	sort.Slice(report.Rules, func(i, j int) bool {
		if report.Rules[i].TokensSaved != report.Rules[j].TokensSaved {
			return report.Rules[i].TokensSaved > report.Rules[j].TokensSaved
		}
		return report.Rules[i].Name < report.Rules[j].Name
	})
	for _, r := range report.Rules {
		if r.Fires > 0 && r.TokensSaved <= 0 {
			report.NegativeOrZero = append(report.NegativeOrZero, r.Name)
		}
	}
	return report, nil
}

func printProfile(out io.Writer, report profileReport) error {
	fmt.Fprintf(out, "Repo: %s\n", report.Repo)
	fmt.Fprintf(out, "Tier: %s\n", report.Tier)
	fmt.Fprintf(out, "Tokenizer: %s\n", report.Tokenizer)
	fmt.Fprintf(out, "Files: %d\n", report.Files)
	fmt.Fprintf(out, "Bytes:  %s -> %s (saved %s)\n", formatInt(report.BytesBefore), formatInt(report.BytesAfter), formatInt(report.BytesBefore-report.BytesAfter))
	fmt.Fprintf(out, "Tokens: %s -> %s (saved %s)\n\n", formatInt(report.TokensBefore), formatInt(report.TokensAfter), formatInt(report.TokensBefore-report.TokensAfter))

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "RULE\tFIRES\tBYTES_SAVED\tTOKENS_SAVED\tMS\tERRORS")
	for _, r := range report.Rules {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\n", r.Name, r.Fires, r.BytesSaved, r.TokensSaved, r.DurationMS, r.Errors)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(report.NegativeOrZero) > 0 {
		fmt.Fprintf(out, "\nFlagged (fires>0 but tokens_saved<=0): %s\n", strings.Join(report.NegativeOrZero, ", "))
	}
	return nil
}

func writeProfileJSON(path string, report profileReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func defaultTokenizerName() string {
	return tokens.DefaultTokenizer().Name()
}
