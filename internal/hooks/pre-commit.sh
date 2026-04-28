#!/bin/sh
# mdcompress: refresh cache for staged markdown files
if ! command -v mdcompress >/dev/null 2>&1; then
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
  exit 0
fi

mdcompress run --staged --quiet
