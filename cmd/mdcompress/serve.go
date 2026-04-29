package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dhruv1794/mdcompress/pkg/server"
	"github.com/spf13/cobra"
)

func serveCommand() *cobra.Command {
	var allowDomains []string
	var disableURL bool
	var cacheSize int
	var rootDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run mdcompress as an MCP server over stdio",
		Long: `Serve the three mdcompress MCP tools (read_markdown, compress_text, ` +
			`compress_url) over stdio. Configure your agent (Claude Code, Cursor, ` +
			`Windsurf) to launch "mdcompress serve" as an MCP server.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			defaults := compressionOptionsFromConfig(".mdcompress/config.yaml")
			opts := server.Options{
				DefaultOpts: defaults,
				CacheSize:   cacheSize,
				RootDir:     rootDir,
			}
			if !disableURL {
				opts.Fetcher = server.NewURLFetcher(server.FetcherOptions{
					Allowlist: allowDomains,
				})
			}
			s := server.New(opts)
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return s.Serve(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringSliceVar(&allowDomains, "allow-domain", nil, "host suffix permitted for compress_url; may be repeated. Empty = allow any host.")
	cmd.Flags().BoolVar(&disableURL, "disable-url", false, "disable the compress_url tool entirely")
	cmd.Flags().IntVar(&cacheSize, "cache-size", 256, "maximum entries in the in-memory LRU cache")
	cmd.Flags().StringVar(&rootDir, "root", "", "scope read_markdown to this directory (default: any path)")
	return cmd
}

func initMCPCommand() *cobra.Command {
	var agentsFlag string

	cmd := &cobra.Command{
		Use:   "init-mcp",
		Short: "Wire mdcompress serve into agent MCP config files",
		Long: `Idempotently add an "mdcompress serve" entry to the MCP config of ` +
			`each requested agent: Claude Code (.mcp.json), Cursor ` +
			`(.cursor/mcp.json), Windsurf (.codeium/windsurf/mcp_config.json).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			agents, err := parseAgents(agentsFlag)
			if err != nil {
				return err
			}
			written, err := installMCPConfigs(agents)
			if err != nil {
				return err
			}
			if len(written) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no MCP-capable agents selected")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated MCP config: %s\n", strings.Join(written, ", "))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "claude,cursor,windsurf", "comma-separated agents to wire (claude,cursor,windsurf)")
	return cmd
}

// upsertMCPJSON writes/merges {[key]: {"mdcompress": {"command":"mdcompress","args":["serve"]}}}
// into path while preserving any existing keys.
func upsertMCPJSON(path, key string) error {
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("%s: invalid JSON: %w", path, err)
		}
	}
	servers, _ := root[key].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["mdcompress"] = map[string]any{
		"command": "mdcompress",
		"args":    []string{"serve"},
	}
	root[key] = servers
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0o644)
}

// installMCPConfigs writes the mdcompress MCP entry for each selected agent.
// It returns the list of touched files (for the init summary).
func installMCPConfigs(agents map[agentName]bool) ([]string, error) {
	var written []string
	if agents[agentClaude] {
		if err := upsertMCPJSON(".mcp.json", "mcpServers"); err != nil {
			return written, err
		}
		written = append(written, ".mcp.json")
	}
	if agents[agentCursor] {
		if err := os.MkdirAll(".cursor", 0o755); err != nil {
			return written, err
		}
		if err := upsertMCPJSON(".cursor/mcp.json", "mcpServers"); err != nil {
			return written, err
		}
		written = append(written, ".cursor/mcp.json")
	}
	if agents[agentWindsurf] {
		if err := os.MkdirAll(".codeium/windsurf", 0o755); err != nil {
			return written, err
		}
		if err := upsertMCPJSON(".codeium/windsurf/mcp_config.json", "mcpServers"); err != nil {
			return written, err
		}
		written = append(written, ".codeium/windsurf/mcp_config.json")
	}
	return written, nil
}
