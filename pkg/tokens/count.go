// Package tokens counts markdown tokens for before/after comparisons.
package tokens

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

const fallbackCharsPerToken = 4

var (
	encOnce sync.Once
	enc     *tiktoken.Tiktoken
	encErr  error
)

func encoder() (*tiktoken.Tiktoken, error) {
	encOnce.Do(func() {
		enc, encErr = tiktoken.GetEncoding("cl100k_base")
	})
	return enc, encErr
}

// Count returns a cl100k_base token count. If tokenizer initialization fails,
// it falls back to a stable len/4 estimate.
func Count(content []byte) int {
	encoding, err := encoder()
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
