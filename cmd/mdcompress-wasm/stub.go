//go:build !(js && wasm)

// This command only does anything when built for GOOS=js GOARCH=wasm; the real
// entrypoint is main.go, build-constrained to js/wasm. This stub keeps the
// package buildable on every other platform so `go build ./...` and
// `go test ./...` don't fail on an otherwise all-excluded directory.
package main

func main() {}
