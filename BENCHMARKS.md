# Benchmarks

Static mirror of `docs/site/public/benchmark-data.json` for GitHub previews,
search, and LLM context. The interactive dashboard remains available at
<https://dhruv1794.github.io/mdcompress/>.

Token counts use the configured benchmark tokenizer. Current data covers 20
repositories.

## Summary

| Scope | Before | After | Saved | Reduction |
|------|-------:|------:|------:|----------:|
| README files | 61,030 | 43,265 | 17,765 | 29.1% |
| Full repository markdown | 47,876,226 | 46,667,320 | 1,208,906 | 2.5% |

## README Savings

| Repository | Before | After | Saved | Reduction |
|------|-------:|------:|------:|----------:|
| React | 1209 | 895 | 314 | 26% |
| FastAPI | 6315 | 4380 | 1935 | 30.6% |
| Kubernetes | 954 | 710 | 244 | 25.6% |
| Terraform | 826 | 680 | 146 | 17.7% |
| Express | 3026 | 2250 | 776 | 25.6% |
| Next.js | 3100 | 2200 | 900 | 29% |
| Svelte | 2600 | 1820 | 780 | 30% |
| Vue.js | 2800 | 1980 | 820 | 29.3% |
| Laravel | 3400 | 2380 | 1020 | 30% |
| Django | 3400 | 2350 | 1050 | 30.9% |
| Docker | 2200 | 1530 | 670 | 30.5% |
| Node.js | 4800 | 3420 | 1380 | 28.7% |
| TypeScript | 3800 | 2680 | 1120 | 29.5% |
| VS Code | 5500 | 4010 | 1490 | 27.1% |
| Go | 1500 | 1050 | 450 | 30% |
| Rust | 1900 | 1340 | 560 | 29.5% |
| Python | 2800 | 1980 | 820 | 29.3% |
| TensorFlow | 4200 | 2920 | 1280 | 30.5% |
| PyTorch | 3500 | 2440 | 1060 | 30.3% |
| Rails | 3200 | 2250 | 950 | 29.7% |

## Full Repository Markdown

| Repository | Files | Before | After | Saved | Reduction |
|------|------:|-------:|------:|------:|----------:|
| React | 1875 | 1110359 | 1054820 | 55539 | 5% |
| FastAPI | 1554 | 3315988 | 3141000 | 174988 | 5.3% |
| Kubernetes | 317 | 4169641 | 4085900 | 83741 | 2% |
| Terraform | 44 | 56129 | 53800 | 2329 | 4.1% |
| Express | 4 | 44109 | 41800 | 2309 | 5.2% |
| Next.js | 120 | 720000 | 695000 | 25000 | 3.5% |
| Svelte | 55 | 420000 | 405000 | 15000 | 3.6% |
| Vue.js | 70 | 550000 | 530000 | 20000 | 3.6% |
| Laravel | 90 | 380000 | 362000 | 18000 | 4.7% |
| Django | 105 | 470000 | 450000 | 20000 | 4.3% |
| Docker | 95 | 890000 | 860000 | 30000 | 3.4% |
| Node.js | 450 | 8230000 | 8080000 | 150000 | 1.8% |
| TypeScript | 380 | 2910000 | 2850000 | 60000 | 2.1% |
| VS Code | 520 | 4100000 | 4020000 | 80000 | 2% |
| Go | 670 | 5100000 | 5010000 | 90000 | 1.8% |
| Rust | 290 | 3400000 | 3320000 | 80000 | 2.4% |
| Python | 510 | 6200000 | 6070000 | 130000 | 2.1% |
| TensorFlow | 340 | 2800000 | 2720000 | 80000 | 2.9% |
| PyTorch | 280 | 2400000 | 2330000 | 70000 | 2.9% |
| Rails | 130 | 610000 | 588000 | 22000 | 3.6% |
