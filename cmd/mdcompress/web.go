package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	assets "github.com/dhruv1794/mdcompress/internal"
	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/rules"
	"github.com/spf13/cobra"
)

func webCommand() *cobra.Command {
	var port int
	var openBrowser bool

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start a local web UI for testing markdown compression",
		Long: `Starts a local HTTP server with an interactive test page. Paste or upload 
any markdown file to see real-time compression results with per-rule analysis.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			defaults := compressionOptionsFromConfig(".mdcompress/config.yaml")
			mux := http.NewServeMux()

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(assets.TestPage))
			})

			allRules := rules.AllRules()
			ruleList := make([]jsonRule, len(allRules))
			for i, rule := range allRules {
				ruleList[i] = jsonRule{
					Name:    rule.Name(),
					Tier:    rule.Tier().String(),
					Default: !rules.DefaultDisabled(rule.Name()),
				}
			}

			mux.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
				json.NewEncoder(w).Encode(ruleList)
			})

			mux.HandleFunc("/api/compress", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")
				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.Method != "POST" {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				var req struct {
					Content  string   `json:"content"`
					Tier     string   `json:"tier"`
					Disabled []string `json:"disabled"`
					Enabled  []string `json:"enabled"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				opts := defaults
				opts.DisabledRules = req.Disabled
				opts.EnabledRules = req.Enabled
				if req.Tier != "" {
					opts.Tier = parseTier(req.Tier)
				}

				result, err := compress.Compress([]byte(req.Content), opts)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				json.NewEncoder(w).Encode(apiCompressResponse{
					Output:       string(result.Output),
					TokensBefore: result.TokensBefore,
					TokensAfter:  result.TokensAfter,
					BytesBefore:  result.BytesBefore,
					BytesAfter:   result.BytesAfter,
					RulesFired:   result.RulesFired,
				})
			})

			addr := fmt.Sprintf("127.0.0.1:%d", port)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("failed to listen on %s: %w", addr, err)
			}

			url := fmt.Sprintf("http://%s", addr)
			fmt.Fprintf(cmd.OutOrStdout(), "mdcompress test page: %s\n", url)
			fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to stop.\n")

			if openBrowser {
				openURL(url)
			}

			srv := &http.Server{Handler: mux}
			return srv.Serve(listener)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8765, "port to listen on")
	cmd.Flags().BoolVar(&openBrowser, "open", false, "open the browser automatically")
	return cmd
}

func openURL(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "linux":
		cmd, args = "xdg-open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	default:
		return
	}
	_ = exec.Command(cmd, args...).Start()
}

func parseTier(t string) compress.Tier {
	switch t {
	case "safe", "tier1", "1":
		return compress.TierSafe
	case "aggressive", "tier2", "2":
		return compress.TierAggressive
	case "llm", "tier3", "3":
		return compress.TierLLM
	default:
		return compress.TierAggressive
	}
}

type jsonRule struct {
	Name    string `json:"name"`
	Tier    string `json:"tier"`
	Default bool   `json:"default"`
}

type apiCompressResponse struct {
	Output       string         `json:"output"`
	TokensBefore int            `json:"tokens_before"`
	TokensAfter  int            `json:"tokens_after"`
	BytesBefore  int            `json:"bytes_before"`
	BytesAfter   int            `json:"bytes_after"`
	RulesFired   map[string]int `json:"rules_fired"`
}
