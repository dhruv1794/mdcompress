# Rules Impact and Promotion Policy

Compression rules in `mdcompress` are organized in two tiers:

- **Tier 1 (`safe`)**: structural cleanup that never removes user-authored prose (frontmatter, URL tracking params, HTML comments, decorative images, badges, etc.). Default tier.
- **Tier 2 (`aggressive`)**: prose-touching transforms that reduce content (hedging, SEO chaff, admonition prefixes, table compaction, cross-section dedup, etc.). Opt-in.

Tier 3 (`llm`) is the LLM-rewrite pipeline, gated separately.

A small set of rules ship default-disabled even at their target tier; see `pkg/rules/registry.go` `DefaultDisabled`.

## Promotion policy

Every rule change — new rule, behavior change, or tier promotion — must run the curated eval (`make eval`) and meet the bar below before merge.

| Action | Pass-rate floor on `never_commit/eval-corpus.jsonl` |
|---|---|
| Add a new tier-2 rule | ≥ 95% |
| Promote tier-2 → tier-1 | ≥ 99% |
| Modify an existing rule (any tier) | no regression vs. baseline; absolute ≥ 95% |
| Add a new tier-1 rule | ≥ 99% |

Pass-rate is the curated `pass / total` over the active corpus, excluding tuples flagged `suspect: true` (responder failed to extract the answer from the *original* document — those are mis-curated or beyond the responder's recall and do not measure compression).

The per-rule sweep (`make eval-per-rule`) is advisory. A rule is a **suspect for retirement** when:

- its removal *rescues* one or more failing tuples (`passΔ > 0`), AND
- it does not break any previously-passing tuple (`broke` is empty).

Conversely, a rule is **load-bearing** when removing it breaks ≥ 1 previously-passing tuple. Such rules are kept even if their net byte savings are small.

## Operational notes

- Eval runs are cached on disk under `.mdcompress/cache/eval/`. Cache keys: `sha256(doc, question, model)` for original-side answers (stable across rule configs); `sha256(compressed, question, model)` for compressed-side; `sha256(doc, compressed, question, expected, judge_model)` for verdicts. Per-rule sweeps re-run only the compressed-side answers and verdicts whose compressed bytes actually change vs. baseline.
- The curated harness runs on DeepSeek. Recent evals use `deepseek-v4-flash` for the responder and verdict, and `deepseek-v4-pro` as the Tier-3 per-section faithfulness judge — the legacy `deepseek-chat` id is retired after 2026-07-24. Self-judging within one model family is a known evaluator-bias risk; a cross-family judge (e.g. Claude) is a planned extension.
- Default threshold in `make eval` is 0.90 — an honest reflection of where DeepSeek-as-responder lands on the current 34-tuple corpus, not the promotion bar above. Promotion decisions read the markdown scoreboard, not the make exit code.

## Reports produced by `make eval`

- `never_commit/eval-corpus-report.{md,json}` — baseline pass rate, per-tuple verdicts, suspect flags.
- `never_commit/eval-rule-scoreboard.{md,json}` — per-rule sweep with rescued/broken tuple ids.
