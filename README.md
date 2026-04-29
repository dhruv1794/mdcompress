# mdcompress

A Go library + CLI that strips fluff from markdown files and emits a
token-optimized version, reducing LLM input cost on docs-heavy contexts
(READMEs, runbooks, RFCs, AGENTS.md/CLAUDE.md, scraped docs).

Status: **v3.1 (Tier-2 aggressive)** with 16 rules, faithfulness eval, MCP server,
LLM rewriter, and interactive web test page.

Live benchmarks + test page: **[dhruv1794.github.io/mdcompress](https://dhruv1794.github.io/mdcompress/)**

## Quick start

```sh
mdcompress init      # one-time setup in any repo
mdcompress run --all # compress every tracked .md
mdcompress status    # show cumulative token savings
mdcompress web       # launch interactive test UI (local)
```

## Compression tiers

| Tier | String | What runs |
|------|--------|-----------|
| 1 | `safe` | Safe rules — deterministic, lossless-to-meaning |
| 2 | `aggressive` (default) | Tier-1 + prose-simplification rules (opt-in portions) |
| 3 | `llm` | Tier-2 + section-level LLM rewriting with faithfulness gate |

## Rules

16 deterministic rules run in fixed order. `strip-boilerplate-sections` and
`collapse-example-output` are opt-in even at Tier-2.

| Rule | Tier | What it does |
|------|------|-------------|
| `strip-frontmatter` | safe | Remove YAML/TOML frontmatter (`---` / `+++` blocks) |
| `strip-html-comments` | safe | Remove `<!-- ... -->` blocks |
| `strip-badges` | safe | Remove shield.io and similar badge images/links |
| `strip-decorative-images` | safe | Remove standalone decorative images |
| `strip-metadata-lines` | safe | Remove `**Last updated:**`, `**Version:**`, etc. |
| `strip-toc` | safe | Remove auto-generated table-of-contents blocks |
| `strip-trailing-cta` | safe | Remove star/follow/sponsor sections at doc end |
| `strip-marketing-prose` | aggressive | Remove "blazing fast", "battle-tested", etc. |
| `strip-hedging-phrases` | aggressive | Replace "it is worth noting that", "in order to", etc. |
| `strip-cross-references` | aggressive | Remove "See the [X] section for details" type phrases |
| `strip-admonition-prefixes` | aggressive | Remove `**Note:**`, `**Warning:**`, `**Tip:**` prefixes |
| `strip-benchmark-prose` | aggressive | Remove prose that only narrates an adjacent table |
| `dedup-cross-section` | aggressive | Remove intro sentences duplicated in body sections |
| `strip-boilerplate-sections` | aggressive | Remove "Contributing"/"License"/"Support" sections that just link to a dedicated file **(opt-in)** |
| `collapse-example-output` | aggressive | Remove `--help` output blocks **(opt-in)** |
| `collapse-blank-lines` | safe | Collapse 3+ blank lines to 2 |

## Web UI

```sh
mdcompress web --open   # starts local test page at http://127.0.0.1:8765
```

Paste or upload any `.md` file, select a tier, and see real-time compression
results with per-rule breakdown, token/byte stats, cost estimate, and a diff
view of what was removed. Also available as a client-side React SPA at the
[public benchmark site](https://dhruv1794.github.io/mdcompress/#/test).

## Config

```yaml
# .mdcompress/config.yaml
version: 1
tier: aggressive
rules:
  enabled: []
  disabled:
    - dedup-cross-section
    - collapse-example-output
    - strip-boilerplate-sections
eval:
  backend: ollama
  model: llama3.1:8b
  threshold: 0.95
  questions_per_doc: 10
```

## Faithfulness eval

`mdcompress eval` verifies compressed markdown answers factual questions
identically to the original.

```sh
mdcompress eval --repo=.                       # evaluate all markdown
mdcompress eval --repo=docs --rule=strip-toc   # isolate one rule
```

Supports Ollama (default), Anthropic, and OpenAI judge backends.

## MCP server

```sh
mdcompress serve                # JSON-RPC MCP server over stdio
mdcompress init --mcp           # add to .mcp.json (Claude Code, Cursor, Windsurf)
```

Three tools: `read_markdown(path)`, `compress_text(content, tier?)`,
`compress_url(url)`. In-memory LRU cache. See [`docs/MCP.md`](docs/MCP.md).

## Build from source

```sh
go build ./cmd/mdcompress
```

## License

MIT — see [LICENSE](./LICENSE).
