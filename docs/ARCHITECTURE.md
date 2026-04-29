# Architecture

## Overview

mdcompress is a Go CLI and library that produces token-optimized mirrors of markdown files. Source files are never modified; compressed copies are written to a hidden cache directory (`.mdcompress/cache/`) that agents and humans can read in place of the originals.

The core pipeline is:

```
source bytes
    │
    ▼
pkg/parser      — parse markdown into a goldmark AST
    │
    ▼
pkg/rules       — walk AST, collect byte-range edits per rule
    │
    ▼
pkg/render      — splice edits into source bytes → compressed bytes
    │
    ▼
pkg/tokens      — count tokens before/after (cl100k_base via tiktoken-go)
    │
    ▼
pkg/compress    — assemble Result, optionally run LLM rewriter (Tier 3)
```

## Repository layout

```
mdcompress/
├── cmd/mdcompress/
│   ├── main.go          — cobra root + all sub-commands
│   └── serve.go         — MCP server command + init-mcp command
├── pkg/
│   ├── compress/        — public Compress() API, tier parsing, LLM adapter hook
│   ├── rules/           — Rule interface, Tier enum, ordered registry, all rule files
│   ├── parser/          — goldmark parse wrapper
│   ├── render/          — byte-range splice (Edit/Range types + ApplyEdits)
│   ├── tokens/          — token counting via tiktoken-go cl100k_base
│   ├── manifest/        — read/write .mdcompress/manifest.json
│   ├── cache/           — read/write .mdcompress/cache/<rel-path>, SHA helpers
│   ├── eval/            — faithfulness eval harness (question gen, judge, report)
│   ├── llm/             — Tier-3 rewriter, Ollama/Anthropic/OpenAI backends, cache
│   └── server/          — MCP stdio server (read_markdown, compress_text, compress_url)
├── internal/
│   ├── assets.go        — go:embed for hook scripts and Claude Code skill
│   ├── hooks/           — pre-commit.sh, post-merge.sh (embedded)
│   ├── skill/           — SKILL.md for Claude Code (embedded)
│   └── testdata/        — corpus (real READMEs) + golden (expected outputs)
├── extensions/vscode/   — VS Code extension (shows token savings on save)
├── docs/                — project specs, roadmap, execution notes
└── go.mod               — module github.com/dhruv1794/mdcompress, go 1.22
```

## Compression tiers

| Tier | String | What runs |
|------|--------|-----------|
| 1 | `safe` | Deterministic, lossless-to-meaning rules only |
| 2 | `aggressive` | Tier-1 rules + prose-simplification rules (opt-in) |
| 3 | `llm` | Tier-2 rules + section-level LLM rewriting with faithfulness guard |

The active tier is set in `.mdcompress/config.yaml` (`tier: aggressive`) and can be overridden per command with `--tier`.

## Rule system

Every rule implements the `Rule` interface (`pkg/rules/rule.go`):

```go
type Rule interface {
    Name() string
    Tier() Tier
    Apply(doc ast.Node, ctx *Context) (ChangeSet, error)
}
```

`Apply` walks the goldmark AST, identifies content to remove, and returns a `ChangeSet` containing `[]render.Edit` (byte ranges to delete or replace). Rules never mutate source bytes directly — `render.ApplyEdits` performs all splicing after every rule runs.

### Registered rules (in execution order)

| Name | Tier | Removes |
|------|------|---------|
| `strip-html-comments` | safe | `<!-- ... -->` blocks |
| `strip-badges` | safe | Shield.io and similar badge images |
| `strip-decorative-images` | safe | Standalone images with no informational alt text |
| `strip-toc` | safe | Generated table-of-contents blocks |
| `strip-trailing-cta` | safe | Social/star/sponsor sections near document end |
| `strip-marketing-prose` | aggressive | "blazing fast", "production-ready", decoration phrases |
| `strip-hedging-phrases` | aggressive | "it is worth noting that", "in order to", etc. |
| `dedup-cross-section` | aggressive | Duplicate facts repeated across sections (opt-in) |
| `strip-benchmark-prose` | aggressive | Prose that just narrates an adjacent table |
| `collapse-example-output` | aggressive | `--help`-style command-output blocks (opt-in) |
| `collapse-blank-lines` | safe | Excessive blank lines outside fenced code blocks |

Rules that are `DefaultDisabled` (currently `collapse-example-output`) must be explicitly opted in via `--enable-rule` or config even when their tier is active.

## Cache and manifest

```
.mdcompress/
├── cache/
│   └── <same relative path as source>.md   — compressed mirror
├── manifest.json                            — source SHA, cache path, token counts per file
└── config.yaml                             — tier, rule overrides, eval/llm settings
```

Both `cache/` and `manifest.json` are gitignored per-clone. The manifest is the source of truth for `mdcompress status` — it tracks `TokensBefore`, `TokensAfter`, and a SHA-256 of the source so staleness is detected without re-reading every file.

## Git hook integration

`mdcompress init` installs two hooks (adapting to any existing hook manager):

- **pre-commit** — `mdcompress run --staged --quiet`: compresses markdown files in the git index before commit.
- **post-merge** — `mdcompress run --changed --quiet`: recompresses files touched by the last pull/merge.

Supported hook managers: direct `.git/hooks/`, Husky, pre-commit (Python), Lefthook.

## MCP server mode

`mdcompress serve` runs a JSON-RPC MCP server over stdio, exposing three tools:

| Tool | Description |
|------|-------------|
| `read_markdown` | Read a local file from disk and return its compressed form |
| `compress_text` | Compress arbitrary markdown text passed directly |
| `compress_url` | Fetch a URL and return the compressed markdown |

The server uses an in-memory LRU cache keyed by SHA-256 of source content. It shares the same `pkg/compress` pipeline as the CLI with no duplication.

## Tier-3 LLM rewriting

When `tier: llm` is configured, `pkg/llm.Rewriter` is attached to the compress pipeline after deterministic rules. It:

1. Walks the compressed AST and identifies prose sections above `MinSectionTokens`.
2. Sends each section to the configured backend (Ollama, Anthropic, OpenAI) with a prompt that requires preserving all facts, code identifiers, and numbers.
3. Runs the optional judge backend to score faithfulness. If the score falls below `Threshold`, the original section is kept.
4. Caches `(section-SHA, prompt-SHA) → rewritten-text` on disk so re-runs are free.

## Faithfulness evaluation

`mdcompress eval` verifies that compressed output still answers factual questions identically to the original. The harness:

1. Generates `QuestionsPerDoc` factual questions per file via the configured LLM backend.
2. Answers each question against both the original and compressed markdown.
3. Asks a judge (same or separate backend) to score answer equivalence.
4. Fails if the average score falls below `Threshold` (default 0.95).

Supported backends for eval: Ollama (default), Anthropic, OpenAI.

## Key dependencies

| Package | Role |
|---------|------|
| `github.com/yuin/goldmark` | Markdown parsing |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/pkoukk/tiktoken-go` | Token counting (cl100k_base) |
