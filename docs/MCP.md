# MCP server mode

`mdcompress serve` exposes the compression engine as a [Model Context
Protocol](https://modelcontextprotocol.io/) server over stdio. Agents that
speak MCP can call `read_markdown`, `compress_text`, and `compress_url`
without writing cache files to disk.

This is the same `pkg/compress` engine that powers the CLI. The server is
a thin wrapper plus an in-memory LRU keyed by `sha256(content + tier +
rules)`.

## Running it

```sh
mdcompress serve
```

Reads JSON-RPC 2.0 frames (newline-delimited) from stdin and writes
responses to stdout. Standard MCP transport.

Flags:

| Flag | Meaning |
|------|---------|
| `--cache-size N` | LRU capacity. Default 256 entries. |
| `--root DIR` | Restrict `read_markdown` paths to `DIR`. Default: any path. |
| `--allow-domain HOST` | Allowed host suffix for `compress_url`. Repeatable. Empty = allow any. |
| `--disable-url` | Disable the `compress_url` tool entirely. |

The default tier and disabled rules are read from `.mdcompress/config.yaml`
when present.

## Tools

### `read_markdown`

```json
{"name": "read_markdown", "arguments": {"path": "docs/architecture.md"}}
```

Reads the file from disk and returns:

```json
{
  "content": "<compressed markdown>",
  "tokens_before": 4820,
  "tokens_after": 3010,
  "bytes_before": 19340,
  "bytes_after": 12120,
  "rules_fired": {"strip-badges": 4, "strip-toc": 1}
}
```

### `compress_text`

```json
{"name": "compress_text", "arguments": {"content": "# Hello\n\n<!-- ... -->\n", "tier": "aggressive"}}
```

For RAG pipelines that already have markdown in memory.

### `compress_url`

```json
{"name": "compress_url", "arguments": {"url": "https://raw.githubusercontent.com/foo/bar/main/README.md"}}
```

Honors `--allow-domain` and `robots.txt`. Off when `--disable-url` is
passed.

## Wiring it into an agent

Run `mdcompress init --mcp` (optionally `--agents=claude,cursor,windsurf`)
to add the entry idempotently to each agent's MCP config file:

| Agent | File written |
|-------|--------------|
| Claude Code | `.mcp.json` |
| Cursor | `.cursor/mcp.json` |
| Windsurf | `.codeium/windsurf/mcp_config.json` |

Each entry looks like:

```json
{
  "mcpServers": {
    "mdcompress": {
      "command": "mdcompress",
      "args": ["serve"]
    }
  }
}
```

Existing keys in those files are preserved. Re-running `init --mcp` is
safe — the `mdcompress` entry is overwritten in place, others untouched.

To install MCP without re-running the full `init`, use:

```sh
mdcompress init-mcp --agents=claude,cursor,windsurf
```

## Example agent prompts

Once installed in Claude Code, asking the agent to "read the architecture
doc" will route through `read_markdown` and return the compressed
version. You don't need to mention mdcompress explicitly; the tool
description is enough.

For RAG retrieval pipelines, prompt the LLM to call `compress_text` on
each chunk before reasoning over it:

> Before answering, compress every retrieved markdown chunk with the
> `compress_text` tool and reason over the compressed result.

## RAG pipeline integration

### LangChain (Python) — replace `MarkdownLoader`

```python
import json
import subprocess

def compress_markdown(text: str, tier: str = "aggressive") -> str:
    """Drop-in replacement for the markdown step of a LangChain loader."""
    request = {
        "jsonrpc": "2.0", "id": 1, "method": "tools/call",
        "params": {
            "name": "compress_text",
            "arguments": {"content": text, "tier": tier},
        },
    }
    proc = subprocess.run(
        ["mdcompress", "serve"],
        input=json.dumps(request),
        capture_output=True, text=True, check=True,
    )
    response = json.loads(proc.stdout.splitlines()[0])
    return response["result"]["structuredContent"]["content"]
```

For high-throughput pipelines, prefer keeping a long-lived `mdcompress
serve` subprocess and writing/reading frames continuously rather than
spawning one per call.

### LlamaIndex — node post-processor

Wrap `compress_markdown` above in a `BaseNodePostprocessor` and apply it
to each retrieved `TextNode.text` before passing nodes to the LLM. The
LRU keeps repeat retrievals cheap.

## Talking to the server from Python (`mcp` client)

```python
import asyncio
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

async def main():
    params = StdioServerParameters(command="mdcompress", args=["serve"])
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            tools = await session.list_tools()
            print([t.name for t in tools.tools])

            result = await session.call_tool(
                "compress_text",
                {"content": "# Hello\n\n<!-- decorative -->\n\nbody\n"},
            )
            print(result.structuredContent)

asyncio.run(main())
```

## Cache behaviour

- Keyed by `sha256(tier ‖ enabled-rules ‖ disabled-rules ‖ source)`.
- LRU; size set with `--cache-size`. Default 256.
- Stats are exposed in-process (`server.CacheStats()`); useful when
  embedding the package directly.
- Acceptance target: hit rate >50% in steady-state usage. With a
  default of 256 entries that holds for any agent re-reading the same
  doc multiple times in a session.

## Hardening notes

- `--root DIR` rejects any `read_markdown` call whose resolved path
  escapes `DIR`. Recommended whenever the agent runs in a context where
  the user can supply the path.
- `compress_url` enforces `robots.txt` per RFC 9309 and the host
  allowlist. Missing `robots.txt` is treated as "allow"; explicit
  `Disallow` entries (under `User-agent: *` or our agent token) are
  honored.
- Response bodies are capped at 5 MiB.
