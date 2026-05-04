# mdcompress

mdcompress strips fluff from README files and agent-context bundles
(AGENTS.md, CLAUDE.md, scraped docs) into a hidden token-optimized mirror.
On the current public benchmark corpus, top-level READMEs shrink 17-31%
(28.5% average) while full docs trees shrink 1.8-5.3% (3.3% average).

Status: **v3.2 (Tier-2 aggressive)** with 28 rules, faithfulness audit, MCP server,
LLM rewriter, plugin API, and interactive web test page.

Live benchmarks + test page: **[dhruv1794.github.io/mdcompress](https://dhruv1794.github.io/mdcompress/)**

## When it helps

mdcompress is strongest on marketing-heavy READMEs, repeated agent context,
generated command output, repeated code examples, and docs with badges,
wrappers, tables of contents, CTAs, and boilerplate prose.

It helps less on dense technical reference docs where most tokens are code,
API names, tables, or unique procedural detail. For those repos, expect smaller
single-digit full-tree savings and use per-rule diffs before enabling
aggressive rules broadly.

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
| 3 | `llm` | Tier-2 + section-level LLM rewriting with a per-section faithfulness guard |

## Rules

28 deterministic rules run in fixed order. `strip-boilerplate-sections` and
`collapse-example-output` are opt-in even at Tier-2.

### Safe (Tier-1)

| Rule | What it does |
|------|-------------|
| `strip-frontmatter` | Remove YAML/TOML frontmatter (`---` / `+++` blocks) |
| `strip-setext-headers` | Convert setext-style headings (`====`/`----`) to ATX (`#`/`##`) |
| `strip-html-comments` | Remove `<!-- ... -->` blocks |
| `compress-code-blocks` | Strip shell prompts, config comments from fenced code blocks |
| `strip-badges` | Remove shield.io and similar badge images/links |
| `strip-decorative-images` | Remove standalone decorative images |
| `strip-metadata-lines` | Remove `**Last updated:**`, `**Version:**`, etc. |
| `strip-horizontal-rules` | Remove `---` / `***` / `___` horizontal rule lines |
| `strip-toc` | Remove auto-generated table-of-contents blocks |
| `strip-trailing-cta` | Remove star/follow/sponsor sections at doc end |
| `collapse-blank-lines` | Collapse 3+ blank lines to 2 |

### Aggressive (Tier-2)

| Rule | What it does |
|------|-------------|
| `strip-html-wrappers` | Remove decorative `<div>`, `<p align>`, `<small>`, `<details>` wrappers |
| `strip-cross-file-dupes` | Replace exact duplicate sections shared across repo files |
| `dedup-cross-file-code-blocks` | Replace repeated fenced code blocks shared across repo files |
| `truncate-large-code-blocks` | Truncate oversized fenced code blocks after a configurable line limit |
| `dedup-multilang-examples` | Collapse multi-language code examples that are semantically identical |
| `strip-marketing-prose` | Remove "blazing fast", "battle-tested", etc. |
| `strip-hedging-phrases` | Replace "it is worth noting that", "in order to", etc. |
| `dedup-cross-section` | Remove intro sentences duplicated in body sections |
| `strip-benchmark-prose` | Remove prose that only narrates an adjacent table |
| `strip-admonition-prefixes` | Remove `**Note:**`, `**Warning:**`, `**Tip:**` prefixes |
| `strip-cross-references` | Remove "See the [X] section for details" type phrases |
| `strip-verification-boilerplate` | Remove "If valid, the output is:" type verification chitchat |
| `strip-seo-chaff` | Remove breadcrumbs, prev/next links, "Was this helpful?", "Edit on GitHub" |
| `compress-changelogs` | Compact changelog/release-note sections to bullet summaries |
| `compact-tables` | Compact pipe tables by removing delimiter rows and extra whitespace |
| `strip-boilerplate-sections` | Remove "Contributing"/"License"/"Support" sections that just link to a dedicated file **(opt-in)** |
| `collapse-example-output` | Remove `--help` output blocks **(opt-in)** |

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

## Faithfulness audit

`mdcompress eval` audits whether compressed markdown answers factual questions
identically to the original. It writes a report and exits non-zero when the
average score is below the configured threshold; it does not change compressed
output or disable rules automatically.

```sh
mdcompress eval --repo=.                       # evaluate all markdown
mdcompress eval --repo=docs --rule=strip-toc   # isolate one rule
```

Supports Ollama (default), Anthropic, and OpenAI judge backends.

## Plugin API

External binaries matching `mdcompress-rule-*` on PATH are auto-discovered.
Write custom rules in any language using the stdio protocol:

```sh
# Custom rule responds to --plugin-info with JSON
mdcompress-rule-mine --plugin-info
# {"name":"mine","tier":"aggressive","description":"My custom rule"}

# Transform markdown via stdin→stdout
echo "# Title" | mdcompress-rule-mine
```

Plugins have a 5-second timeout; transform output must be valid UTF-8 and no larger than 2x the input.

See [`docs/RULES.md`](docs/RULES.md) for the full protocol specification.

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
