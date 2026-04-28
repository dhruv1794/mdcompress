---
name: mdcompress-refresh
description: Refresh the mdcompress cache after editing any .md file in repos
  that use mdcompress. Triggers when the working directory contains a
  .mdcompress/ folder and a .md file has just been written or modified.
  Skip when the file is inside .mdcompress/cache/ since those are generated.
---

# Refreshing the mdcompress cache

After editing any *.md file in this repo, run:

    mdcompress run <path-to-edited-file>

This keeps `.mdcompress/cache/` in sync so subsequent reads consume the
compressed version. Skip this if the file is already inside
`.mdcompress/cache/` because those are generated and should never be edited
manually.

The repo signals it uses mdcompress by having a `.mdcompress/` directory.
