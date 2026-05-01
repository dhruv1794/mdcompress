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
│   ├── serve.go         — MCP server command + init-mcp command
│   └── web.go           — HTTP server with interactive test page
├── pkg/
│   ├── compress/        — public Compress() API, tier parsing, LLM adapter hook
│   ├── rules/           — Rule interface, Tier enum, ordered registry, 26 rule files + plugin system
│   ├── parser/          — goldmark parse wrapper
│   ├── render/          — byte-range splice (Edit/Range types + ApplyEdits)
│   ├── tokens/          — token counting via tiktoken-go cl100k_base
│   ├── manifest/        — read/write .mdcompress/manifest.json
│   ├── cache/           — read/write .mdcompress/cache/<rel-path>, SHA helpers
│   ├── eval/            — faithfulness eval harness (question gen, judge, report)
│   ├── llm/             — Tier-3 rewriter, Ollama/Anthropic/OpenAI backends, cache
│   ├── server/          — MCP stdio server (read_markdown, compress_text, compress_url)
│   └── migrate/         — config migration from markdownlint, Vale, Prettier
├── internal/
│   ├── assets.go        — go:embed for hook scripts, Claude Code skill, test page
│   ├── hooks/           — pre-commit.sh, post-merge.sh (embedded)
│   ├── skill/           — SKILL.md for Claude Code (embedded)
│   ├── test.html        — interactive test page served by `mdcompress web` (embedded)
│   └── testdata/        — corpus (real READMEs) + golden (expected outputs)
├── docs/
│   ├── site/            — Vite + React SPA (benchmarks + interactive test)
│   │   └── src/pages/{Benchmarks,Test}.jsx
│   ├── _site/           — built static output (deployed to GitHub Pages)
│   └── *.md             — project specs, roadmap, execution notes
├── extensions/vscode/   — VS Code extension (shows token savings on save)
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
| `strip-frontmatter` | safe | YAML/TOML frontmatter at document start |
| `strip-setext-headers` | safe | Convert setext-style headings to ATX |
| `strip-html-comments` | safe | `<!-- ... -->` blocks |
| `compress-code-blocks` | safe | Shell prompts, config comments, imports from fenced code blocks |
| `strip-badges` | safe | Shield.io and similar badge images |
| `strip-decorative-images` | safe | Standalone images with no informational alt text |
| `strip-metadata-lines` | safe | `**Last updated:**`, `**Version:**`, etc. |
| `strip-horizontal-rules` | safe | `---` / `***` / `___` decorative horizontal rules |
| `strip-html-wrappers` | aggressive | Decorative `<div>`, `<p align>`, `<small>`, `<details>` wrappers |
| `strip-toc` | safe | Generated table-of-contents blocks |
| `strip-trailing-cta` | safe | Social/star/sponsor sections near document end |
| `strip-cross-file-dupes` | aggressive | Boilerplate sections (Contributing/License/Support) duplicated across files |
| `dedup-multilang-examples` | aggressive | Multi-language code examples that are semantically identical |
| `strip-marketing-prose` | aggressive | "blazing fast", "production-ready", decoration phrases |
| `strip-hedging-phrases` | aggressive | "it is worth noting that", "in order to", etc. |
| `dedup-cross-section` | aggressive | Duplicate facts repeated across sections |
| `strip-benchmark-prose` | aggressive | Prose that just narrates an adjacent table |
| `strip-admonition-prefixes` | aggressive | `**Note:**`, `**Warning:**`, `**Tip:**` prefixes |
| `strip-cross-references` | aggressive | "See the [X] section for details" type phrases |
| `strip-boilerplate-sections` | aggressive | Contributing/License/Support sections redirecting to dedicated files **(opt-in)** |
| `strip-verification-boilerplate` | aggressive | "If valid, the output is:" type verification chitchat |
| `strip-seo-chaff` | aggressive | Breadcrumbs, prev/next links, "Was this helpful?", "Edit on GitHub" |
| `compress-changelogs` | aggressive | Changelog/release-note sections → bullet summaries |
| `collapse-example-output` | aggressive | `--help`-style command-output blocks **(opt-in)** |
| `compact-tables` | aggressive | Pipe table delimiter rows and extra whitespace |
| `collapse-blank-lines` | safe | Excessive blank lines outside fenced code blocks |

Rules that are `DefaultDisabled` (currently `collapse-example-output` and `strip-boilerplate-sections`) must be explicitly opted in via `--enable-rule` or config even when their tier is active.

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

## Web test page

`mdcompress web` starts a local HTTP server serving an interactive test page (embedded via `go:embed`). A separate **React SPA** in `docs/site/` provides the same test page at the [public benchmark site](https://dhruv1794.github.io/mdcompress/) with client-side JavaScript compression mirroring all 16 Go rules (no server needed). Both share the same dark theme and navigation.

**API endpoints** (`mdcompress web` only):
- `GET /` — embedded interactive test page
- `GET /api/rules` — list of all rules with name, tier, default status
- `POST /api/compress` — accepts `{content, tier, disabled[], enabled[]}`, returns `{output, tokens_before, tokens_after, bytes_before, bytes_after, rules_fired}`

The React SPA (`docs/site/`) builds to `docs/_site/` and is deployed to GitHub Pages via the benchmark CI workflow. It uses HashRouter for client-side routing (`/` for benchmarks, `#/test` for interactive test).

## Plugin API

`mdcompress` discovers external binaries matching `mdcompress-rule-*` on `PATH` and loads them as rules. Two protocols:

- **`--plugin-info`** — queried once at startup, returns JSON `{"name","tier","description"}`. Plugins are filtered by the active tier.
- **stdin→stdout transform** — full source bytes in, transformed bytes out. Plugins run after built-in AST rules and before the LLM rewriter.

This allows custom rules to be written in any language without modifying the core codebase. See `pkg/rules/plugin.go` for the implementation.

## Key dependencies

| Package | Role |
|---------|------|
| `github.com/yuin/goldmark` | Markdown parsing |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/pkoukk/tiktoken-go` | Token counting (cl100k_base) |
