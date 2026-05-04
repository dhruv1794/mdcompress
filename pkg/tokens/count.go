// Package tokens counts markdown tokens for before/after comparisons.
//
// Token counts are inherently tokenizer-dependent. The default is OpenAI's
// cl100k_base (GPT-3.5/4); o200k_base (GPT-4o) and a raw byte estimator are
// also available. Anthropic does not publish a tokenizer, so cl100k_base is
// the closest publicly-available proxy for Claude tokens but is not exact.
package tokens

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkoukk/tiktoken-go"
)

const fallbackCharsPerToken = 4

// Tokenizer selects how Count() and CountWith() turn bytes into a token count.
type Tokenizer int

const (
	// CL100k is OpenAI's cl100k_base encoding (GPT-3.5/4). Default.
	CL100k Tokenizer = iota + 1
	// O200k is OpenAI's o200k_base encoding (GPT-4o).
	O200k
	// Bytes returns the raw byte length of the input. Honest about not knowing
	// the consumer's tokenizer; useful when targeting Claude or other models
	// without a public tokenizer.
	Bytes
)

// Name returns the canonical disclosure label for the tokenizer.
func (t Tokenizer) Name() string {
	switch t {
	case CL100k:
		return "cl100k_base"
	case O200k:
		return "o200k_base"
	case Bytes:
		return "bytes"
	default:
		return "unknown"
	}
}

// ParseTokenizer parses a CLI/config tokenizer name.
func ParseTokenizer(value string) (Tokenizer, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "cl100k", "cl100k_base":
		return CL100k, nil
	case "o200k", "o200k_base":
		return O200k, nil
	case "bytes", "byte":
		return Bytes, nil
	default:
		return 0, fmt.Errorf("unknown tokenizer %q (want cl100k, o200k, or bytes)", value)
	}
}

var defaultTokenizer atomic.Int32

func init() {
	defaultTokenizer.Store(int32(CL100k))
}

// SetDefault sets the process-wide tokenizer used by Count(). Returns an error
// only for an unknown Tokenizer value.
func SetDefault(t Tokenizer) error {
	switch t {
	case CL100k, O200k, Bytes:
		defaultTokenizer.Store(int32(t))
		return nil
	default:
		return fmt.Errorf("unknown tokenizer %d", t)
	}
}

// DefaultTokenizer returns the process-wide tokenizer used by Count().
func DefaultTokenizer() Tokenizer {
	return Tokenizer(defaultTokenizer.Load())
}

var (
	cl100kOnce sync.Once
	cl100kEnc  *tiktoken.Tiktoken
	cl100kErr  error

	o200kOnce sync.Once
	o200kEnc  *tiktoken.Tiktoken
	o200kErr  error
)

func encoder(t Tokenizer) (*tiktoken.Tiktoken, error) {
	switch t {
	case CL100k:
		cl100kOnce.Do(func() { cl100kEnc, cl100kErr = tiktoken.GetEncoding("cl100k_base") })
		return cl100kEnc, cl100kErr
	case O200k:
		o200kOnce.Do(func() { o200kEnc, o200kErr = tiktoken.GetEncoding("o200k_base") })
		return o200kEnc, o200kErr
	default:
		return nil, fmt.Errorf("tokenizer %s has no encoder", t.Name())
	}
}

// Count returns a token count using the process-wide default tokenizer.
// If tokenizer initialization fails, it falls back to a stable len/4 estimate.
func Count(content []byte) int {
	return CountWith(content, DefaultTokenizer())
}

// CountWith returns a token count using the explicitly-provided tokenizer.
func CountWith(content []byte, t Tokenizer) int {
	if t == Bytes {
		return len(content)
	}
	encoding, err := encoder(t)
	if err != nil {
		return fallbackCount(content)
	}
	return len(encoding.Encode(string(content), nil, nil))
}

func fallbackCount(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	count := len(content) / fallbackCharsPerToken
	if len(content)%fallbackCharsPerToken != 0 {
		count++
	}
	if count == 0 {
		return 1
	}
	return count
}
