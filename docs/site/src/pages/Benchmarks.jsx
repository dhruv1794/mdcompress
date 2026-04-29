import { useState, useEffect } from 'react';

function fmt(n) { return n.toLocaleString(); }
function pctClass(p) { return p >= 25 ? 'high' : p >= 10 ? 'mid' : 'low'; }
function bar(pct) {
  const cls = pct >= 25 ? 'green' : pct >= 10 ? 'teal' : pct >= 3 ? 'amber' : 'red';
  const w = Math.min(pct * 2.5, 100);
  return (
    <span className="bar-wrap">
      <span className={`reduction ${pctClass(pct)}`}>{pct.toFixed(1)}%</span>
      <span className="bar-track"><span className={`bar-fill ${cls}`} style={{ width: w }} /></span>
    </span>
  );
}

export default function Benchmarks() {
  const [repos, setRepos] = useState([]);

  useEffect(() => {
    fetch('/mdcompress/benchmark-data.json')
      .then(r => r.json())
      .then(setRepos)
      .catch(() => setRepos([]));
  }, []);

  const data = repos;

  const allT1B = data.reduce((s, r) => s + r.tier1_before, 0);
  const allT1A = data.reduce((s, r) => s + r.tier1_after, 0);
  const t1Saved = allT1B - allT1A;
  const allT2B = data.reduce((s, r) => s + r.tier2_before, 0);
  const allT2A = data.reduce((s, r) => s + r.tier2_after, 0);
  const t2Saved = allT2B - allT2A;
  const rmB = data.reduce((s, r) => s + r.readme_before, 0);
  const rmA = data.reduce((s, r) => s + r.readme_after, 0);
  const rmSaved = rmB - rmA;
  const avgRMPct = rmB > 0 ? (rmSaved / rmB * 100) : 0;
  const costSaved = (t2Saved / 1_000_000 * 3).toFixed(2);

  const cards = [
    { label: 'Repos Tracked', value: data.length, sub: '', cls: 'purple' },
    { label: 'README Reduction', value: avgRMPct.toFixed(1) + '%', sub: 'aggressive tier, avg across repos', cls: 'teal' },
    { label: 'Tier-2 Tokens Saved', value: fmt(t2Saved), sub: `$${costSaved} at Sonnet pricing`, cls: 'green' },
    { label: 'Per-session ROI', value: '~$' + ((rmSaved / rmB * 6315 * 3) / 1e6 * 3 * 5).toFixed(2), sub: '5-doc agent context, aggressive tier', cls: 'amber' },
  ];

  const readmeSort = data.filter(r => r.readme_before > 0).sort((a, b) => ((b.readme_before - b.readme_after) / b.readme_before) - ((a.readme_before - a.readme_after) / a.readme_before));
  const repoSort = [...data].sort((a, b) => b.tier1_before - a.tier1_before);

  const ruleTotals = {};
  data.forEach(r => {
    if (r.tier2_rules) {
      Object.entries(r.tier2_rules).forEach(([name, count]) => {
        ruleTotals[name] = (ruleTotals[name] || 0) + count;
      });
    }
  });
  const totalNodes = Object.values(ruleTotals).reduce((a, b) => a + b, 0);

  return (
    <>
      <div className="stats">
        {cards.map(c => (
          <div key={c.label} className={`stat-card ${c.cls}`}>
            <div className="stat-label">{c.label}</div>
            <div className="stat-value">{c.value}</div>
            {c.sub && <div className="stat-sub">{c.sub}</div>}
          </div>
        ))}
      </div>

      <div className="section">
        <h2>README Savings</h2>
        <p className="note">Top-level README files with the <code>aggressive</code> tier.</p>
        <div className="table-wrap" style={{ maxHeight: 550, overflowY: 'auto' }}>
          <table>
            <thead><tr><th>Repository</th><th>Before</th><th>After</th><th>Saved</th><th>Reduction</th></tr></thead>
            <tbody>
              {readmeSort.map(r => {
                const s = r.readme_before - r.readme_after;
                const pct = (s / r.readme_before * 100);
                return (
                  <tr key={r.name}>
                    <td>{r.name}</td>
                    <td>{fmt(r.readme_before)}</td>
                    <td>{fmt(r.readme_after)}</td>
                    <td>{fmt(s)}</td>
                    <td>{bar(pct)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className="section">
        <h2>Full-Repo Compression</h2>
        <p className="note">Every tracked <code>*.md</code> file in the repository. Full docs trees are mostly technical content, so expect smaller percentages.</p>
        <div className="table-wrap" style={{ maxHeight: 550, overflowY: 'auto' }}>
          <table>
            <thead><tr><th>Repository</th><th>Files</th><th>Tier-1 Before</th><th>Tier-1 After</th><th>Tier-1 %</th><th>Tier-2 Before</th><th>Tier-2 After</th><th>Tier-2 %</th><th>Top Rules</th></tr></thead>
            <tbody>
              {repoSort.map(r => {
                const t1s = r.tier1_before - r.tier1_after;
                const t2s = r.tier2_before - r.tier2_after;
                const top = Object.entries(r.tier2_rules || {}).sort((a, b) => b[1] - a[1]).slice(0, 3)
                  .map(e => <span key={e[0]} className="badge teal">{e[0]}&nbsp;{e[1]}</span>);
                return (
                  <tr key={r.name}>
                    <td>{r.name}</td>
                    <td>{r.tier1_files}</td>
                    <td>{fmt(r.tier1_before)}</td>
                    <td>{fmt(r.tier1_after)}</td>
                    <td>{bar(t1s / r.tier1_before * 100)}</td>
                    <td>{fmt(r.tier2_before)}</td>
                    <td>{fmt(r.tier2_after)}</td>
                    <td>{bar(t2s / r.tier2_before * 100)}</td>
                    <td>{top.length ? top : <>&mdash;</>}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className="section">
        <h2>Per-Rule Contribution</h2>
        <p className="note">How many nodes each rule removed across all repositories (Tier-2 aggressive).</p>
        <div className="table-wrap">
          <table>
            <thead><tr><th>Rule</th><th>Tier</th><th>Nodes Removed</th><th>% of Total</th></tr></thead>
            <tbody>
              {Object.entries(ruleTotals).sort((a, b) => b[1] - a[1]).map(([name, count]) => {
                const pct = (count / totalNodes * 100);
                const tier = name.includes('marketing') || name.includes('hedging') || name.includes('benchmark') || name.includes('dedup') ? 'Aggressive' : 'Safe';
                return (
                  <tr key={name}>
                    <td>{name}</td>
                    <td><span className={`badge ${tier === 'Safe' ? 'green' : 'amber'}`}>{tier}</span></td>
                    <td>{fmt(count)}</td>
                    <td>{pct.toFixed(1)}%</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
