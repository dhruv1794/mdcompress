# Bravo

![](banner.png)

## Overview

Stable fact: bravo reads configuration from `.bravo/config.yaml`.

## Configuration

Set `mode: strict` to reject invalid markdown before compression.

## Benchmarks

| Target | Before | After |
| --- | ---: | ---: |
| macOS | 1200 | 780 |
| Linux | 1100 | 715 |

The benchmark above shows macOS moving from 1200 tokens to 780 tokens and Linux moving from 1100 tokens to 715 tokens.

## License

MIT
