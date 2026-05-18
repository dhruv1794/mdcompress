//go:build js && wasm

// Command mdcompress-wasm exposes the markdown compressor to the browser.
//
// It is compiled with GOOS=js GOARCH=wasm (see `make wasm`) and installs a
// single global JavaScript function:
//
//	mdcompressCompress(text, tier) -> { ok, output, tokensBefore, ... }
//
// Token counts are a deterministic offline len/4 estimate (see offlineLoader):
// the build embeds no tokenizer tables and makes no network calls, so input
// text never leaves the browser.
package main

import (
	"errors"
	"syscall/js"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

// offlineLoader makes tiktoken initialisation fail immediately instead of
// downloading multi-MB BPE data. compress.Compress then falls back to a
// deterministic len/4 token estimate, keeping the WASM build small and fully
// offline. Real tokenizer counts belong to the native CLI.
type offlineLoader struct{}

func (offlineLoader) LoadTiktokenBpe(string) (map[string]int, error) {
	return nil, errors.New("mdcompress-wasm: tokenizer runs offline (estimated counts)")
}

// compressFn is the JS-facing entrypoint. It accepts (text, tier?) and returns
// a plain object; any failure is reported as { ok: false, error }.
func compressFn(_ js.Value, args []js.Value) (result any) {
	defer func() {
		if r := recover(); r != nil {
			result = errResult("compression failed unexpectedly")
		}
	}()

	if len(args) < 1 || args[0].Type() != js.TypeString {
		return errResult("mdcompressCompress(text, tier): text must be a string")
	}

	tierName := "aggressive"
	if len(args) > 1 && args[1].Type() == js.TypeString && args[1].String() != "" {
		tierName = args[1].String()
	}
	tier, err := compress.ParseTier(tierName)
	if err != nil {
		return errResult(err.Error())
	}

	res, err := compress.Compress([]byte(args[0].String()), compress.Options{Tier: tier})
	if err != nil {
		return errResult(err.Error())
	}

	rulesFired := make(map[string]any, len(res.RulesFired))
	for name, n := range res.RulesFired {
		rulesFired[name] = n
	}
	ruleErrors := make(map[string]any, len(res.RuleErrors))
	for name, msg := range res.RuleErrors {
		ruleErrors[name] = msg
	}

	return map[string]any{
		"ok":           true,
		"output":       string(res.Output),
		"tier":         tier.String(),
		"tokensBefore": res.TokensBefore,
		"tokensAfter":  res.TokensAfter,
		"bytesBefore":  res.BytesBefore,
		"bytesAfter":   res.BytesAfter,
		"rulesFired":   rulesFired,
		"ruleErrors":   ruleErrors,
	}
}

func errResult(msg string) map[string]any {
	return map[string]any{"ok": false, "error": msg}
}

func main() {
	tiktoken.SetBpeLoader(offlineLoader{})
	js.Global().Set("mdcompressCompress", js.FuncOf(compressFn))
	select {} // keep the Go runtime alive so the exported function stays callable
}
