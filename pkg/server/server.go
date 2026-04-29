// Package server hosts the mdcompress MCP server.
//
// MCP messages are exchanged as line-delimited JSON-RPC 2.0 frames over
// stdio. The server is a thin wrapper around pkg/compress with an LRU
// cache keyed by the SHA-256 of the source markdown.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

// Version reports the server's protocol/server version string. Bumping it
// is part of the MCP handshake.
const Version = "0.1.0"

const protocolVersion = "2024-11-05"

// CompressedResult is the payload returned by every tool.
type CompressedResult struct {
	Content      string         `json:"content"`
	TokensBefore int            `json:"tokens_before"`
	TokensAfter  int            `json:"tokens_after"`
	BytesBefore  int            `json:"bytes_before"`
	BytesAfter   int            `json:"bytes_after"`
	RulesFired   map[string]int `json:"rules_fired"`
}

// Options configures a Server.
type Options struct {
	// DefaultOpts is used when a tool call doesn't override Tier/rules.
	DefaultOpts compress.Options
	// CacheSize bounds the LRU. Defaults to 256 entries when zero.
	CacheSize int
	// Fetcher handles compress_url. nil disables that tool.
	Fetcher *URLFetcher
	// RootDir scopes read_markdown to a directory. Empty allows any path.
	RootDir string
}

// Server holds the runtime state for the mdcompress MCP endpoint.
type Server struct {
	opts  Options
	cache *LRU[CompressedResult]
}

// New builds a Server with the supplied options.
func New(opts Options) *Server {
	size := opts.CacheSize
	if size <= 0 {
		size = 256
	}
	return &Server{
		opts:  opts,
		cache: NewLRU[CompressedResult](size),
	}
}

// CacheStats returns LRU hit/miss counts. Useful for the >50% hit-rate
// acceptance criterion.
func (s *Server) CacheStats() (hits, misses int) { return s.cache.Stats() }

// Serve reads JSON-RPC frames from r and writes responses to w until r is
// closed or ctx is cancelled. Each frame is a single line.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode: %w", err)
		}
		resp, ok := s.dispatch(ctx, raw)
		if !ok {
			continue // notification — no response
		}
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode: %w", err)
		}
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

func (s *Server) dispatch(ctx context.Context, raw json.RawMessage) (rpcResponse, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: codeParseError, Message: err.Error()},
		}, true
	}
	// Notifications carry no id.
	isNotification := len(req.ID) == 0
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}

	result, rerr := s.handle(ctx, req.Method, req.Params)
	if rerr != nil {
		if isNotification {
			return rpcResponse{}, false
		}
		resp.Error = rerr
		return resp, true
	}
	if isNotification {
		return rpcResponse{}, false
	}
	resp.Result = result
	return resp, true
}

func (s *Server) handle(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.handleInitialize(params)
	case "notifications/initialized", "initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return s.handleToolsList(), nil
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + method}
	}
}

func (s *Server) handleInitialize(_ json.RawMessage) (any, *rpcError) {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "mdcompress",
			"version": Version,
		},
	}, nil
}

type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) handleToolsList() any {
	tools := []toolDescriptor{
		{
			Name:        "read_markdown",
			Description: "Read a markdown file from disk and return its compressed form.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Filesystem path to a markdown file."},
					"tier": map[string]any{"type": "string", "enum": []string{"safe", "aggressive", "llm"}},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "compress_text",
			Description: "Compress a markdown document supplied inline.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{"type": "string"},
					"tier":    map[string]any{"type": "string", "enum": []string{"safe", "aggressive", "llm"}},
				},
				"required": []string{"content"},
			},
		},
	}
	if s.opts.Fetcher != nil {
		tools = append(tools, toolDescriptor{
			Name:        "compress_url",
			Description: "Fetch markdown from a URL (allowlist + robots.txt enforced) and return its compressed form.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url":  map[string]any{"type": "string"},
					"tier": map[string]any{"type": "string", "enum": []string{"safe", "aggressive", "llm"}},
				},
				"required": []string{"url"},
			},
		})
	}
	return map[string]any{"tools": tools}
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var p toolCallParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
		}
	}
	res, err := s.callTool(ctx, p.Name, p.Arguments)
	if err != nil {
		return toolErrorResult(err), nil
	}
	body, err := json.Marshal(res)
	if err != nil {
		return nil, &rpcError{Code: codeInternalError, Message: err.Error()}
	}
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
		"structuredContent": res,
		"isError":           false,
	}, nil
}

func toolErrorResult(err error) any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": err.Error()},
		},
		"isError": true,
	}
}

func (s *Server) callTool(ctx context.Context, name string, raw json.RawMessage) (CompressedResult, error) {
	switch name {
	case "read_markdown":
		return s.toolReadMarkdown(raw)
	case "compress_text":
		return s.toolCompressText(raw)
	case "compress_url":
		return s.toolCompressURL(ctx, raw)
	default:
		return CompressedResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

type readMarkdownArgs struct {
	Path string `json:"path"`
	Tier string `json:"tier,omitempty"`
}

func (s *Server) toolReadMarkdown(raw json.RawMessage) (CompressedResult, error) {
	var args readMarkdownArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return CompressedResult{}, err
	}
	if args.Path == "" {
		return CompressedResult{}, errors.New("path is required")
	}
	abs, err := s.resolvePath(args.Path)
	if err != nil {
		return CompressedResult{}, err
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return CompressedResult{}, err
	}
	opts, err := s.optsForTier(args.Tier)
	if err != nil {
		return CompressedResult{}, err
	}
	return s.compressCached(content, opts)
}

type compressTextArgs struct {
	Content string `json:"content"`
	Tier    string `json:"tier,omitempty"`
}

func (s *Server) toolCompressText(raw json.RawMessage) (CompressedResult, error) {
	var args compressTextArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return CompressedResult{}, err
	}
	if args.Content == "" {
		return CompressedResult{}, errors.New("content is required")
	}
	opts, err := s.optsForTier(args.Tier)
	if err != nil {
		return CompressedResult{}, err
	}
	return s.compressCached([]byte(args.Content), opts)
}

type compressURLArgs struct {
	URL  string `json:"url"`
	Tier string `json:"tier,omitempty"`
}

func (s *Server) toolCompressURL(ctx context.Context, raw json.RawMessage) (CompressedResult, error) {
	if s.opts.Fetcher == nil {
		return CompressedResult{}, errors.New("compress_url is disabled (no fetcher configured)")
	}
	var args compressURLArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return CompressedResult{}, err
	}
	if args.URL == "" {
		return CompressedResult{}, errors.New("url is required")
	}
	if _, err := url.Parse(args.URL); err != nil {
		return CompressedResult{}, fmt.Errorf("invalid url: %w", err)
	}
	body, err := s.opts.Fetcher.Fetch(ctx, args.URL)
	if err != nil {
		return CompressedResult{}, err
	}
	opts, err := s.optsForTier(args.Tier)
	if err != nil {
		return CompressedResult{}, err
	}
	return s.compressCached(body, opts)
}

func (s *Server) compressCached(content []byte, opts compress.Options) (CompressedResult, error) {
	key := cacheKey(content, opts)
	if cached, ok := s.cache.Get(key); ok {
		return cached, nil
	}
	res, err := compress.Compress(content, opts)
	if err != nil {
		return CompressedResult{}, err
	}
	out := CompressedResult{
		Content:      string(res.Output),
		TokensBefore: res.TokensBefore,
		TokensAfter:  res.TokensAfter,
		BytesBefore:  res.BytesBefore,
		BytesAfter:   res.BytesAfter,
		RulesFired:   res.RulesFired,
	}
	s.cache.Put(key, out)
	return out, nil
}

func (s *Server) optsForTier(tier string) (compress.Options, error) {
	opts := s.opts.DefaultOpts
	if tier == "" {
		if opts.Tier == 0 {
			opts.Tier = compress.TierSafe
		}
		return opts, nil
	}
	parsed, err := compress.ParseTier(tier)
	if err != nil {
		return compress.Options{}, err
	}
	opts.Tier = parsed
	return opts, nil
}

func (s *Server) resolvePath(p string) (string, error) {
	if s.opts.RootDir == "" {
		return p, nil
	}
	root, err := filepath.Abs(s.opts.RootDir)
	if err != nil {
		return "", err
	}
	target := p
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", p)
	}
	return abs, nil
}

func cacheKey(content []byte, opts compress.Options) string {
	h := sha256.New()
	fmt.Fprintf(h, "tier=%d\n", opts.Tier)
	for _, n := range opts.EnabledRules {
		fmt.Fprintf(h, "+%s\n", n)
	}
	for _, n := range opts.DisabledRules {
		fmt.Fprintf(h, "-%s\n", n)
	}
	h.Write([]byte{0})
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}
