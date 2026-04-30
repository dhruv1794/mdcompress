import { useState, useCallback } from 'react';
import { compress, ruleList } from '../lib/compress';

function fmt(n) { return n.toLocaleString(); }

const SAMPLE = `---
title: Getting Started
description: Learn how to use the API
author: Developer
version: 2.0
---

# Getting Started

<!-- This is an HTML comment that should be stripped -->
<!-- Another one -->

[![Build Status](https://img.shields.io/github/workflow/status/org/repo/CI)](https://github.com/org/repo/actions)
[![NPM Version](https://img.shields.io/npm/v/package)](https://npmjs.com/package)
![Logo](./logo.png)

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [API](#api)
- [Contributing](#contributing)

## Overview

Welcome to our blazing fast, production-ready library! It is worth noting that this
library is loved by developers and battle-tested in production environments. We offer
a feature-rich, cutting-edge solution that is both elegant and delightful to use.

For more information, see the [API Reference](#api). Check out the [documentation](https://docs.example.com).

## Installation

\`\`\`bash
npm install my-library
\`\`\`

**Note:** This library requires Node.js 18 or later.
**Important:** Make sure to set up your API key before starting.

> **Warning:** This is a breaking change.
> **Tip:** Use the \`--debug\` flag for verbose output.

## Usage

Here is a simple example:

\`\`\`js
const client = new MyClient({ apiKey: 'xxx' });
const result = await client.query('SELECT * FROM data');
console.log(result);
\`\`\`

## Benchmark

| Library | Ops/sec | Latency (ms) |
|---------|---------|-------------|
| ours    | 50000   | 0.02        |
| theirs  | 30000   | 0.03        |

Our library is significantly faster than the competition across all benchmarks.

## Contributing
Contributions are welcome! Please read our [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License
This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Support
Need help? Join our [Slack](https://slack.example.com) or file an [issue](https://github.com/org/repo/issues).

**Last updated:** April 2026
**Version:** 2.0.0

---

*Star us on GitHub! Follow us on Twitter!*
`;

export default function Test() {
  const [input, setInput] = useState('');
  const [tier, setTier] = useState('aggressive');
  const [result, setResult] = useState(null);
  const [activeTab, setActiveTab] = useState('summary');
  const [loading, setLoading] = useState(false);

  const handleCompress = useCallback(() => {
    if (!input.trim()) return;
    setLoading(true);
    setTimeout(() => {
      const res = compress(input, tier);
      setResult(res);
      setLoading(false);
    }, 0);
  }, [input, tier]);

  const handleFile = useCallback((e) => {
    const file = e.target.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => setInput(ev.target.result);
    reader.readAsText(file);
  }, []);

  const loadSample = () => setInput(SAMPLE);

  const tabs = ['summary', 'rules', 'output', 'diff'];

  const saved = result ? result.tokensBefore - result.tokensAfter : 0;
  const pct = result && result.tokensBefore > 0 ? (saved / result.tokensBefore * 100) : 0;
  const byteSaved = result ? result.bytesBefore - result.bytesAfter : 0;
  const bytePct = result && result.bytesBefore > 0 ? (byteSaved / result.bytesBefore * 100) : 0;
  const cost = (saved / 1_000_000 * 3).toFixed(3);

  const pctClass = (p) => p >= 20 ? 'green' : p >= 10 ? 'teal' : p >= 5 ? 'amber' : 'red';

  const diffLines = [];
  if (result) {
    const origLines = input.split('\n');
    const outSet = new Set((result.output || '').split('\n').map(l => l.trim()).filter(Boolean));
    origLines.forEach((l, i) => {
      const t = l.trim();
      if (!t || outSet.has(t)) return;
      diffLines.push({ line: i + 1, text: l });
    });
  }

  const ruleEntries = result ? Object.entries(result.rulesFired || {}).sort((a, b) => b[1] - a[1]) : [];
  const totalNodes = ruleEntries.reduce((s, e) => s + e[1], 0);

  const tiers = { safe: 1, aggressive: 2, llm: 3 };
  const maxTier = tiers[tier] || 2;
  const rules = ruleList();

  return (
    <div className="section" style={{ marginTop: 0 }}>
      <div className="test-layout">
        <div className="card">
          <div className="panel-header">
            <h2>Input</h2>
            <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
              <label htmlFor="file-upload" className="btn-ghost" style={{ cursor: 'pointer', position: 'relative' }}>
                Upload .md
                <input id="file-upload" type="file" accept=".md,.markdown,.txt" onChange={handleFile} style={{ position: 'absolute', opacity: 0, width: '100%', height: '100%', left: 0, cursor: 'pointer' }} />
              </label>
              <button className="btn-ghost" onClick={loadSample}>Load sample</button>
              <button onClick={handleCompress} disabled={loading || !input.trim()}>
                {loading ? 'Compressing...' : 'Compress'}
              </button>
            </div>
          </div>
          <div className="controls" style={{ marginBottom: '.5rem' }}>
            <label>Tier</label>
            <select value={tier} onChange={e => setTier(e.target.value)}>
              <option value="safe">Safe (Tier 1)</option>
              <option value="aggressive">Aggressive (Tier 2)</option>
              <option value="llm">LLM (Tier 3)</option>
            </select>
          </div>
          {tier === 'llm' && (
            <div className="llm-notice">
              LLM (Tier 3) uses AI-assisted compression via Ollama, Anthropic, or OpenAI.
              This browser demo falls back to Aggressive (Tier 2) rules.
              For real LLM compression, run the CLI:<br />
              <code>mdcompress compress file.md --tier=llm</code>
            </div>
          )}
          <textarea
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder="Paste your markdown here, or upload a .md file..."
          />
          <div className="rule-toggles">
            {rules.map(r => {
              const rt = tiers[r.tier] || 1;
              return (
                <div key={r.name} className="rule-toggle">
                  <input type="checkbox" checked={r.default && rt <= maxTier} disabled={rt > maxTier} readOnly />
                  <label style={{ opacity: rt > maxTier ? 0.3 : 1, color: result?.rulesFired?.[r.name] ? 'var(--purple)' : undefined }}>
                    <span className={`badge ${r.tier}`}>{r.tier}</span>
                    {r.name}
                  </label>
                </div>
              );
            })}
          </div>
        </div>

        <div className="card">
          <div className="panel-header">
            <h2>Results</h2>
          </div>

          {!result && (
            <div className="empty-state">Paste markdown and click <strong>Compress</strong> to see results.</div>
          )}

          {result && (
            <>
              <div className="tab-bar">
                {tabs.map(t => (
                  <button key={t} className={`tab ${activeTab === t ? 'active' : ''}`} onClick={() => setActiveTab(t)}>
                    {t.charAt(0).toUpperCase() + t.slice(1)}
                  </button>
                ))}
              </div>

              {activeTab === 'summary' && (
                <div className="tab-content active">
                  <div className="stats" style={{ marginBottom: '1rem' }}>
                    {[
                      { cls: 'purple', val: fmt(result.tokensBefore), lbl: 'Tokens Before' },
                      { cls: 'teal', val: fmt(result.tokensAfter), lbl: 'Tokens After' },
                      { cls: 'green', val: `${fmt(saved)} (${pct.toFixed(1)}%)`, lbl: 'Tokens Saved' },
                      { cls: 'amber', val: `${fmt(byteSaved)} (${bytePct.toFixed(1)}%)`, lbl: 'Bytes Saved' },
                    ].map(s => (
                      <div key={s.lbl} className={`stat-card ${s.cls}`} style={{ padding: '1rem' }}>
                        <div className="stat-label">{s.lbl}</div>
                        <div className="stat-value" style={{ fontSize: '1.4rem' }}>{s.val}</div>
                      </div>
                    ))}
                  </div>
                  <div style={{ fontSize: '.85rem', color: 'var(--muted)' }}>
                    <div className="bar-wrap" style={{ marginBottom: '.5rem' }}>
                      <span>Reduction {pct.toFixed(1)}%</span>
                      <span className="bar-track"><span className={`bar-fill ${pctClass(pct)}`} style={{ width: Math.min(pct * 3, 100) }} /></span>
                    </div>
                    <div>{ruleEntries.length} rules fired</div>
                    <div style={{ marginTop: '.25rem' }}>Estimated cost savings: ~${cost} (Sonnet $3/MTok input)</div>
                  </div>
                </div>
              )}

              {activeTab === 'rules' && (
                <div className="tab-content active">
                  {ruleEntries.length === 0 ? (
                    <div className="empty-state">No rules fired — content unchanged.</div>
                  ) : (
                    <div className="rule-list">
                      {ruleEntries.map(([name, count]) => {
                        const rpct = totalNodes > 0 ? (count / totalNodes * 100).toFixed(1) : '0.0';
                        const rule = rules.find(r => r.name === name);
                        return (
                          <div key={name} className="rule-row">
                            <span className="rule-name">
                              {name}
                              <span className={`badge ${rule?.tier || 'safe'}`}>{rule?.tier || 'safe'}</span>
                            </span>
                            <span className="rule-stats">{count} nodes ({rpct}%)</span>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}

              {activeTab === 'output' && (
                <div className="tab-content active">
                  <div className="output-box">{result.output || '(empty output)'}</div>
                </div>
              )}

              {activeTab === 'diff' && (
                <div className="tab-content active">
                  {diffLines.length === 0 ? (
                    <div className="empty-state">No removed content detected (or content was shortened rather than removed).</div>
                  ) : (
                    <div className="output-box">
                      <div style={{ marginBottom: '.5rem', color: 'var(--muted)', fontSize: '.7rem' }}>{diffLines.length} lines potentially removed/modified:</div>
                      {diffLines.map(r => (
                        <div key={r.line} className="diff-line">
                          <span className="diff-del">-</span> <span style={{ color: 'var(--muted)' }}>L{r.line}:</span> <span style={{ color: 'var(--text)' }}>{r.text}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
