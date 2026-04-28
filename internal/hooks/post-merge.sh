#!/bin/sh
# mdcompress: refresh cache after pull/merge
if ! command -v mdcompress >/dev/null 2>&1; then
  echo "mdcompress: command not found; skipping markdown cache refresh" >&2
  exit 0
fi

mdcompress run --changed --quiet
