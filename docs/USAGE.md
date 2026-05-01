# Usage

## Installation

**Homebrew:**
```sh
brew install dhruv1794/tap/mdcompress
```

**One-line installer:**
```sh
curl -fsSL https://dhruv1794.dev/install.sh | sh
```

**Go install:**
```sh
go install github.com/dhruv1794/mdcompress/cmd/mdcompress@latest
```

**Docker:**
```sh
docker pull ghcr.io/dhruv1794/mdcompress
```

---

## Quick start

```sh
mdcompress init       # one-time setup in any repo
mdcompress run --all  # compress every .md in the repository
mdcompress status     # show token savings summary
```

`init` creates `.mdcompress/config.yaml`, installs git hooks, appends an agent hint to `CLAUDE.md` / `AGENTS.md`, installs the Claude Code skill, and runs an initial full compression. Source markdown is never modified.

---

## Commands

### `init`

Initialize mdcompress in the current repository.

```sh
mdcompress init
mdcompress init --agents=claude,cursor          # limit which agents get hints
mdcompress init --mcp                           # also wire the MCP server into agent configs
```

What it does:
- Creates `.mdcompress/config.yaml` with defaults.
- Appends `.mdcompress/cache/` and `.mdcompress/manifest.json` to `.gitignore`.
- Installs pre-commit and post-merge git hooks.
- Appends an agent hint to `CLAUDE.md`, `AGENTS.md`, `.cursorrules`, `.windsurfrules`, `.aider.conf.yml`, and `.continue/config.json` where they exist.
- Installs the Claude Code skill to `.claude/skills/mdcompress/SKILL.md`.
- Runs `mdcompress run --all` to populate the cache.

### `run`

Compress markdown files and update the hidden cache mirror.

```sh
mdcompress run                        # refresh any stale cache entries
mdcompress run --all                  # force-rebuild every tracked file
mdcompress run README.md docs/        # compress specific files or directories
mdcompress run --staged               # compress staged files (used by pre-commit hook)
mdcompress run --changed              # compress files changed by last merge (used by post-merge hook)
mdcompress run --tier=aggressive      # override the configured tier for this run
mdcompress run --enable-rule=dedup-cross-section
mdcompress run --disable-rule=strip-toc
mdcompress run --quiet                # suppress non-error output
```

Output goes to `.mdcompress/cache/<same relative path>`. The manifest at `.mdcompress/manifest.json` is updated with SHA, token counts, and rules that fired.

### `compress`

Compress a single file or stdin and write to stdout. Useful for one-off inspection without touching the cache.

```sh
mdcompress compress README.md
cat README.md | mdcompress compress
mdcompress compress README.md --tier=aggressive
```

Token and byte counts are printed to stderr.

### `status`

Show cumulative token savings tracked in the manifest.

```sh
mdcompress status
```

Example output:
```
Repo: github.com/dhruv1794/mdcompress
Tier: aggressive
Files tracked: 42
Cache: 40 fresh, 1 stale, 1 missing

Tokens before: 284,901
Tokens after:  241,732
Saved:         43,169 (15.2%)

ROI estimate (Claude Sonnet input pricing, $3.00/MTok):
  Per full-cache read: ~$0.0001
  Local manifest total: ~$0.0001

Top savings:
  docs/kubernetes.md               8,342 saved (23.4%)
  README.md                        2,109 saved (18.1%)
```

Manifest totals are per-clone and not shared team-wide (the manifest is gitignored).

### `eval`

Evaluate whether compressed markdown preserves factual content.

```sh
mdcompress eval --repo=.
mdcompress eval --repo=docs --rule=strip-toc          # isolate one rule
mdcompress eval --backend=ollama --model=llama3.1:8b
mdcompress eval --backend=anthropic --model=claude-3-5-haiku-latest --api-key-env=ANTHROPIC_API_KEY
mdcompress eval --backend=openai --model=gpt-4o-mini --api-key-env=OPENAI_API_KEY
mdcompress eval --json-out=.mdcompress/eval.json --markdown-out=.mdcompress/eval.md
```

The harness generates factual questions per document, answers them against both the original and compressed versions, then scores equivalence. The command exits non-zero if the average score falls below `--threshold` (default 0.95).

### `doctor`

Diagnose the repository setup and report any problems.

```sh
mdcompress doctor
mdcompress doctor --fix    # attempt automatic fixes
```

Checks: config file validity, hook installation, agent hints, cache freshness, manifest consistency, PATH availability, and gitignored source markdown.

### `clean`

Delete the cache directory and reset the manifest to empty.

```sh
mdcompress clean
```

### `install-hooks`

Install (or re-install) the pre-commit and post-merge hooks.

```sh
mdcompress install-hooks
```

Supports Husky, pre-commit (Python), Lefthook, and direct `.git/hooks/`.

### `install-skill`

Install the Claude Code mdcompress skill.

```sh
mdcompress install-skill
```

Writes `.claude/skills/mdcompress/SKILL.md`, which tells Claude Code to run `mdcompress run <path>` after editing any `.md` file.

### `serve`

Run mdcompress as an MCP server over stdio.

```sh
mdcompress serve
mdcompress serve --allow-domain=docs.example.com --allow-domain=pkg.go.dev
mdcompress serve --disable-url          # disable the compress_url tool
mdcompress serve --cache-size=512       # in-memory LRU entry limit (default 256)
mdcompress serve --root=/workspace      # scope read_markdown to a directory
```

MCP tools exposed:
- `read_markdown(path)` — read a local file and return its compressed form.
- `compress_text(content, tier?)` — compress markdown text passed inline.
- `compress_url(url)` — fetch a URL and return compressed markdown.

### `init-mcp`

Wire `mdcompress serve` into agent MCP config files.

```sh
mdcompress init-mcp
mdcompress init-mcp --agents=claude,cursor,windsurf
```

Writes/merges into `.mcp.json` (Claude Code), `.cursor/mcp.json` (Cursor), and `.codeium/windsurf/mcp_config.json` (Windsurf).

### `version`

Print the installed version.

```sh
mdcompress version
```

---

## Configuration

`.mdcompress/config.yaml` is created by `mdcompress init`. All fields are optional; shown here with their defaults.

```yaml
version: 1
tier: aggressive          # safe | aggressive | llm

rules:
  enabled: []             # opt-in rules to enable (e.g. strip-boilerplate-sections)
  disabled:               # rules to disable even when tier is active
    - dedup-cross-section
    - collapse-example-output
    - strip-boilerplate-sections

eval:
  backend: ollama
  model: llama3.1:8b
  threshold: 0.95
  questions_per_doc: 10
  seeds: 1

llm:                      # Tier-3 settings
  backend: ollama
  model: llama3.1:8b
  endpoint: http://localhost:11434
  api_key_env: ANTHROPIC_API_KEY
  cache: true
  threshold: 0.95
  min_section_tokens: 200
```

### Compression tiers

| Tier | Token savings | Risk |
|------|--------------|------|
| `safe` | Low–moderate (5–20% on typical READMEs) | None — removes decoration only |
| `aggressive` | Moderate–high | Low — prose simplification, opt-in |
| `llm` | High | Medium — non-deterministic; faithfulness-guarded |

### Rule overrides

Disable a rule that is on by default:

```yaml
rules:
  disabled:
    - strip-toc
```

Enable a rule that is opt-in (off by default even when its tier is active):

```yaml
rules:
  enabled:
    - dedup-cross-section
    - collapse-example-output
```

---

## How the cache works

After `mdcompress run`, every source `.md` file has a mirror under `.mdcompress/cache/` at the same relative path. Agents configured with the mdcompress hint will automatically read from the cache instead of the source.

The cache and manifest are gitignored. Each developer's clone has its own local token-savings tracking. Run `mdcompress run --all` after a fresh clone to populate the cache, or let the post-merge hook handle it after pulls.

---

## Agent integration

The `init` command appends a hint to agent configuration files so they prefer cached mirrors automatically:

**Claude Code / CLAUDE.md / AGENTS.md:**
```
## mdcompress
This repo uses mdcompress. For any *.md file you read, prefer the version
at .mdcompress/cache/<same-relative-path> if it exists because it is a
token-optimized mirror of the original maintained automatically.
```

Similar hints are appended to `.cursorrules`, `.windsurfrules`, and `.aider.conf.yml` if those files exist.

---

## VS Code extension

The VS Code extension shows token savings in the status bar and runs `mdcompress run <file>` automatically on save for `.md` files in repos that have a `.mdcompress/` directory.

Install from the `extensions/vscode/` directory or from the VS Code Marketplace.
