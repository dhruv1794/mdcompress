# mdcompress (VS Code)

Thin wrapper around the [mdcompress](https://github.com/dhruv1794/mdcompress) CLI. After you save a tracked `.md` file, the extension refreshes the `.mdcompress/cache/` mirror and surfaces token savings in the status bar.

## What it does

- **On save** — runs `mdcompress run <path>` for any markdown file inside a workspace that has a `.mdcompress/` directory. Files inside `.mdcompress/cache/` are skipped.
- **Status bar** — shows `mdcompress: -<saved> tok (<pct>%)` for the active markdown file, sourced from `.mdcompress/manifest.json`.
- **`mdcompress: View compressed mirror`** — opens a diff between the working copy and `.mdcompress/cache/<path>` so you can see exactly what was stripped.
- **`mdcompress: Refresh cache for current file`** — runs `mdcompress run` on demand without saving.

## Requirements

- The [`mdcompress`](https://github.com/dhruv1794/mdcompress/releases) CLI on `PATH` (or set `mdcompress.binaryPath`).
- A workspace with a `.mdcompress/` directory (created by `mdcompress init`).

## Settings

| Key | Default | Description |
|---|---|---|
| `mdcompress.runOnSave` | `true` | Compress markdown files automatically on save. |
| `mdcompress.binaryPath` | `mdcompress` | Path to the `mdcompress` binary. |
| `mdcompress.showStatusBar` | `true` | Show per-file token savings in the status bar. |

## Notes

- The status bar reads `.mdcompress/manifest.json`. The manifest is gitignored, so numbers reflect this clone only — see the upstream README for the rationale.
- The extension never edits source markdown. All output goes into `.mdcompress/cache/`.
