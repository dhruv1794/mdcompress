package server

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

func newTestServer(t *testing.T, root string) *Server {
	t.Helper()
	return New(Options{
		DefaultOpts: compress.Options{Tier: compress.TierSafe},
		CacheSize:   8,
		RootDir:     root,
	})
}

func roundtrip(t *testing.T, s *Server, frame string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	in := strings.NewReader(frame)
	if err := s.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() == 0 {
		return nil
	}
	var resp map[string]any
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\n%s", err, out.String())
	}
	return resp
}

func TestInitializeAndToolsList(t *testing.T) {
	s := newTestServer(t, "")
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	resp := roundtrip(t, s, init)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion=%v", result["protocolVersion"])
	}

	resp = roundtrip(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := resp["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools without fetcher, got %d", len(tools))
	}
}

func TestNotificationDoesNotRespond(t *testing.T) {
	s := newTestServer(t, "")
	resp := roundtrip(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if resp != nil {
		t.Fatalf("notification should not produce response, got %v", resp)
	}
}

func TestCompressTextHitsCache(t *testing.T) {
	s := newTestServer(t, "")
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"compress_text","arguments":{"content":"# Hi\n\n<!-- a -->\n\nbody\n"}}}`
	roundtrip(t, s, body)
	roundtrip(t, s, body)
	hits, misses := s.CacheStats()
	if hits != 1 || misses != 1 {
		t.Fatalf("expected hits=1 misses=1, got %d/%d", hits, misses)
	}
}

func TestReadMarkdownRespectsRoot(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(target, []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestServer(t, dir)

	good := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_markdown","arguments":{"path":"doc.md"}}}`
	resp := roundtrip(t, s, good)
	result := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("expected success, got %v", result)
	}

	escape := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_markdown","arguments":{"path":"../secret.md"}}}`
	resp = roundtrip(t, s, escape)
	result = resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected escape to be rejected, got %v", result)
	}
}

func TestUnknownToolReturnsToolError(t *testing.T) {
	s := newTestServer(t, "")
	resp := roundtrip(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	result := resp["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError=true, got %v", result)
	}
}

func TestRobotsAllowsBlocksDisallowed(t *testing.T) {
	rules := parseRobots("User-agent: *\nDisallow: /private/\n", "mdcompress-mcp/1.0")
	if !robotsAllows(rules, "/public/foo") {
		t.Fatalf("expected /public/foo to be allowed")
	}
	if robotsAllows(rules, "/private/foo") {
		t.Fatalf("expected /private/foo to be blocked")
	}
}

func TestHostAllowlist(t *testing.T) {
	f := NewURLFetcher(FetcherOptions{Allowlist: []string{"example.com"}})
	if !f.hostAllowed("example.com") {
		t.Fatalf("example.com should be allowed")
	}
	if !f.hostAllowed("docs.example.com") {
		t.Fatalf("subdomain should be allowed")
	}
	if f.hostAllowed("evil.com") {
		t.Fatalf("evil.com should be blocked")
	}
}
