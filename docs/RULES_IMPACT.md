# Rule impact

Per-rule bytes saved across the 20-repo benchmark corpus
(`docs/benchmark-repos.json`). Generated from
`docs/site/public/benchmark-data.json`.

This is a coarse signal: it shows where compression savings actually come from
in the wild, sorted by total bytes removed. "Repos affected" tells you how
broadly a rule fires — high-impact-but-narrow rules (e.g. `strip-cross-file-dupes`)
are concentrated in repos with heavy duplication, while broad-but-shallow rules
(e.g. `collapse-blank-lines`) fire almost everywhere for tiny gains.

Rules with low byte savings are still worth keeping if they're cheap and
catch user-visible noise, but this table is the place to start when deciding
where to spend new optimization effort.

## Tier 1 (safe)

| Rule | Bytes saved | Repos affected |
|---|---:|---:|
| `compact-tables` | 348.5 KB | 17/20 |
| `strip-badges` | 8.3 KB | 17/20 |
| `strip-toc` | 5.8 KB | 8/20 |
| `strip-setext-headers` | 5.6 KB | 12/20 |
| `strip-decorative-images` | 2.9 KB | 17/20 |
| `strip-frontmatter` | 2.7 KB | 17/20 |
| `strip-html-comments` | 2.2 KB | 18/20 |
| `strip-trailing-cta` | 2.2 KB | 9/20 |
| `strip-horizontal-rules` | 2.1 KB | 15/20 |
| `strip-metadata-lines` | 2 KB | 15/20 |
| `compress-code-blocks` | 214 B | 16/20 |
| `collapse-blank-lines` | 95 B | 19/20 |

## Tier 2 (aggressive)

| Rule | Bytes saved | Repos affected |
|---|---:|---:|
| `strip-cross-file-dupes` | 373 KB | 17/20 |
| `compact-tables` | 347.9 KB | 17/20 |
| `compress-changelogs` | 221.3 KB | 16/20 |
| `truncate-large-code-blocks` | 59.2 KB | 16/20 |
| `strip-cross-references` | 11.6 KB | 19/20 |
| `strip-badges` | 8.3 KB | 17/20 |
| `strip-toc` | 5.8 KB | 8/20 |
| `strip-setext-headers` | 5.6 KB | 12/20 |
| `dedup-cross-file-code-blocks` | 4.2 KB | 16/20 |
| `dedup-multilang-examples` | 3.7 KB | 8/20 |
| `strip-decorative-images` | 2.9 KB | 17/20 |
| `strip-boilerplate-sections` | 2.9 KB | 11/20 |
| `strip-frontmatter` | 2.7 KB | 17/20 |
| `compress-code-blocks` | 2.3 KB | 19/20 |
| `strip-html-comments` | 2.2 KB | 18/20 |
| `strip-trailing-cta` | 2.2 KB | 9/20 |
| `strip-horizontal-rules` | 2.1 KB | 15/20 |
| `strip-metadata-lines` | 2 KB | 15/20 |
| `strip-verification-boilerplate` | 1.1 KB | 10/20 |
| `strip-html-wrappers` | 802 B | 14/20 |
| `strip-seo-chaff` | 766 B | 15/20 |
| `strip-benchmark-prose` | 762 B | 3/20 |
| `collapse-example-output` | 742 B | 3/20 |
| `compress-api-parameter-trivia` | 659 B | 1/20 |
| `strip-hedging-phrases` | 332 B | 19/20 |
| `strip-admonition-prefixes` | 239 B | 17/20 |
| `strip-edit-page-footers` | 172 B | 2/20 |
| `strip-marketing-prose` | 113 B | 10/20 |
| `collapse-blank-lines` | 85 B | 19/20 |
| `strip-mkdocs-includes` | 26 B | 1/20 |
