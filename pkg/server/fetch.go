package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// URLFetcher downloads remote markdown subject to an optional domain allowlist
// and robots.txt rules. The zero value is not usable; build with NewURLFetcher.
type URLFetcher struct {
	allowlist map[string]bool
	client    *http.Client
	userAgent string
	maxBytes  int64

	mu        sync.Mutex
	robotsCtx map[string]*robotsRules
}

type robotsRules struct {
	disallow []string
}

// FetcherOptions configures a URLFetcher.
type FetcherOptions struct {
	// Allowlist of host suffixes (e.g. "raw.githubusercontent.com",
	// "github.com"). Nil/empty means no allowlist enforcement.
	Allowlist []string
	// Timeout per HTTP request. Zero uses 15s.
	Timeout time.Duration
	// MaxBytes caps the response body size. Zero uses 5 MiB.
	MaxBytes int64
	// UserAgent sent on every request. Zero uses "mdcompress-mcp/1.0".
	UserAgent string
}

// NewURLFetcher returns a fetcher with sensible defaults applied.
func NewURLFetcher(opts FetcherOptions) *URLFetcher {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	max := opts.MaxBytes
	if max == 0 {
		max = 5 * 1024 * 1024
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "mdcompress-mcp/1.0"
	}
	allow := make(map[string]bool, len(opts.Allowlist))
	for _, host := range opts.Allowlist {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allow[host] = true
		}
	}
	return &URLFetcher{
		allowlist: allow,
		client:    &http.Client{Timeout: timeout},
		userAgent: ua,
		maxBytes:  max,
		robotsCtx: make(map[string]*robotsRules),
	}
}

// Fetch downloads rawURL after checking allowlist and robots.txt.
func (f *URLFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("url has no host")
	}
	if !f.hostAllowed(u.Host) {
		return nil, fmt.Errorf("host %q not in allowlist", u.Host)
	}

	rules, err := f.getRobots(ctx, u)
	if err == nil && !robotsAllows(rules, u.Path) {
		return nil, fmt.Errorf("robots.txt disallows %s", u.Path)
	}

	return f.do(ctx, u.String())
}

func (f *URLFetcher) hostAllowed(host string) bool {
	if len(f.allowlist) == 0 {
		return true
	}
	host = strings.ToLower(host)
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}
	for entry := range f.allowlist {
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

func (f *URLFetcher) do(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/markdown, text/plain, */*")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, target)
	}
	return io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
}

func (f *URLFetcher) getRobots(ctx context.Context, u *url.URL) (*robotsRules, error) {
	host := strings.ToLower(u.Host)
	f.mu.Lock()
	if cached, ok := f.robotsCtx[host]; ok {
		f.mu.Unlock()
		return cached, nil
	}
	f.mu.Unlock()

	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	body, err := f.do(ctx, robotsURL)
	rules := &robotsRules{}
	if err == nil {
		rules = parseRobots(string(body), f.userAgent)
	}
	f.mu.Lock()
	f.robotsCtx[host] = rules
	f.mu.Unlock()
	if err != nil {
		// Missing robots.txt = allow everything (RFC 9309 §2.3.1.3).
		return rules, nil
	}
	return rules, nil
}

// parseRobots applies the most permissive rules between User-agent: * and the
// supplied agent token. Only Disallow lines are honored.
func parseRobots(text, agent string) *robotsRules {
	agentTokens := []string{"*"}
	if t := strings.ToLower(strings.SplitN(agent, "/", 2)[0]); t != "" {
		agentTokens = append(agentTokens, t)
	}

	var current []string
	groups := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i != -1 {
			line = strings.TrimSpace(line[:i])
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "user-agent":
			current = append(current, strings.ToLower(value))
		case "disallow":
			for _, ua := range current {
				groups[ua] = append(groups[ua], value)
			}
		case "allow":
			// Ignore Allow lines for our minimal parser; treated as no-op.
		default:
			if key != "" && len(current) > 0 && !isContinuation(key) {
				current = nil
			}
		}
	}

	rules := &robotsRules{}
	seen := map[string]bool{}
	for _, ua := range agentTokens {
		for _, dis := range groups[ua] {
			if !seen[dis] {
				rules.disallow = append(rules.disallow, dis)
				seen[dis] = true
			}
		}
	}
	return rules
}

func isContinuation(key string) bool {
	switch key {
	case "crawl-delay", "sitemap", "host":
		return true
	}
	return false
}

func robotsAllows(rules *robotsRules, path string) bool {
	if rules == nil {
		return true
	}
	if path == "" {
		path = "/"
	}
	for _, dis := range rules.disallow {
		if dis == "" {
			continue
		}
		if strings.HasPrefix(path, dis) {
			return false
		}
	}
	return true
}
