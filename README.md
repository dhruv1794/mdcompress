# mdcompress

A Go library + CLI that strips fluff from markdown files and emits a
token-optimized version, reducing LLM input cost on docs-heavy contexts
(READMEs, runbooks, RFCs, AGENTS.md/CLAUDE.md, scraped docs).

Status: **v0.1.0-ready v1 safe tier**. See the project plan in the parent
directory (`../INDEX.md`) for the full roadmap.

## Benchmark baseline

These numbers were generated with the v1 safe tier against fresh shallow clones
of five public repositories. Token counts use `cl100k_base` as a proxy; real
Claude/GPT counts may differ by about +/-10%.

Tier-1 is conservative: it removes decoration and generated scaffolding, not
technical prose. Expect larger savings on badge-heavy, TOC-heavy, media-heavy,
or CTA-heavy README files, and small savings on full documentation trees where
most markdown is real content.

| Repo | Scope | Files | Tokens before | Tokens after | Saved | Reduction |
|---|---:|---:|---:|---:|---:|---:|
| React | full repo | 1,875 | 1,110,359 | 1,107,390 | 2,969 | 0.3% |
| FastAPI | full repo | 1,554 | 3,315,988 | 3,290,404 | 25,584 | 0.8% |
| Kubernetes | full repo | 317 | 4,169,641 | 4,167,795 | 1,846 | 0.0% |
| Terraform | full repo | 44 | 56,129 | 56,024 | 105 | 0.2% |
| Express | full repo | 4 | 44,109 | 43,692 | 417 | 0.9% |

README-only checks show the practical Tier-1 range on common OSS landing pages:

| Repo README | Tokens before | Tokens after | Saved | Reduction |
|---|---:|---:|---:|---:|
| React | 1,209 | 1,019 | 190 | 15.7% |
| FastAPI | 6,315 | 4,954 | 1,361 | 21.6% |
| Kubernetes | 954 | 839 | 115 | 12.1% |
| Terraform | 826 | 801 | 25 | 3.0% |
| Express | 3,026 | 2,610 | 416 | 13.7% |

These are measured release baselines, not a blanket 25-40% claim. Tier-1 usually
lands around 5-20% on mixed real READMEs, can reach 25-40% on decoration-heavy
README classes, and is often below 1-5% on content-dense full documentation
trees.

## Quick start

```sh
mdcompress init      # one-time setup in any repo
mdcompress run --all # compress every tracked .md
mdcompress status    # show cumulative token savings
```

`mdcompress init` creates `.mdcompress/config.yaml`, installs git hooks, adds an
agent hint, installs the Claude Code skill, and populates the hidden cache once.
Source markdown remains untouched; compressed mirrors are written under
`.mdcompress/cache/<same-relative-path>`.

## How it works

The v1 safe tier applies deterministic markdown rules in a fixed order:

1. Strip HTML comments.
2. Strip known badge images and badge links.
3. Strip standalone decorative images.
4. Strip generated table-of-contents blocks.
5. Strip trailing social/CTA sections near the end of a document.
6. Collapse excessive blank lines outside fenced code blocks.

The cache manifest is stored at `.mdcompress/manifest.json` and is gitignored.
That means cumulative `status` numbers are per clone, not shared team-wide.

## Faithfulness eval

`mdcompress eval` verifies that compressed markdown still answers factual
questions the same way as the original. It uses Ollama by default, with optional
Anthropic and OpenAI judges.

```sh
mdcompress eval --repo=.                       # evaluate all markdown
mdcompress eval --repo=docs --rule=strip-toc   # isolate one rule
mdcompress eval --json-out=.mdcompress/eval.json --markdown-out=.mdcompress/eval.md
```

Configuration can live in `.mdcompress/config.yaml`:

```yaml
eval:
  backend: ollama
  model: llama3.1:8b
  threshold: 0.95
  questions_per_doc: 10
  seeds: 1
```

For hosted judges, pass an explicit model and API key environment variable:

```sh
mdcompress eval --backend=openai --model=gpt-4o-mini --api-key-env=OPENAI_API_KEY
mdcompress eval --backend=anthropic --model=claude-3-5-haiku-latest --api-key-env=ANTHROPIC_API_KEY
```

## v1 acceptance

v1 is considered ready when:

- `mdcompress init && mdcompress run --all` populates `.mdcompress/cache/` in a
  real repository.
- The safe-tier benchmark table is segmented by repo scope and README scope.
- The pre-commit hook refreshes staged markdown cache entries.
- The post-merge hook refreshes markdown files changed by the last merge.
- Agent hints tell Claude Code and other agents to prefer cached mirrors.
- `mdcompress status` reports tracked files and cumulative local token savings.
- Roundtrip tests prove parse plus no-op splice rendering is bytes-identical.

## Build from source

```sh
go build ./cmd/mdcompress
```

## License

MIT — see [LICENSE](./LICENSE).
